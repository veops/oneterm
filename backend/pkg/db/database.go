package db

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"

	"github.com/veops/oneterm/pkg/config"
	"github.com/veops/oneterm/pkg/logger"
)

var (
	DB     *gorm.DB
	dbOnce sync.Once
)

type DBType string

const (
	MySQL    DBType = "mysql"
	Postgres DBType = "postgres"
	TiDB     DBType = "tidb"
	TDSQL    DBType = "tdsql"
)

type Config struct {
	Type            DBType
	Host            string
	Port            string
	User            string
	Password        string
	Database        string
	Charset         string
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	SSLMode         string
}

func ConfigFromGlobal() Config {
	dbType := DBType(config.Cfg.Database.Type)
	if dbType == "" {
		dbType = MySQL
	}

	return Config{
		Type:            dbType,
		Host:            config.Cfg.Database.Host,
		Port:            config.Cfg.Database.Port,
		User:            config.Cfg.Database.User,
		Password:        config.Cfg.Database.Password,
		Database:        config.Cfg.Database.Database,
		Charset:         config.Cfg.Database.Charset,
		MaxIdleConns:    config.Cfg.Database.MaxIdleConns,
		MaxOpenConns:    config.Cfg.Database.MaxOpenConns,
		ConnMaxLifetime: time.Duration(config.Cfg.Database.ConnMaxLife) * time.Second,
		ConnMaxIdleTime: time.Duration(config.Cfg.Database.ConnMaxIdle) * time.Second,
		SSLMode:         config.Cfg.Database.SSLMode,
	}
}

func (c *Config) DSN() string {
	switch c.Type {
	case Postgres:
		return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode)
	default: // MySQL, TiDB, TDSQL
		return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=True&loc=Local",
			c.User, c.Password, c.Host, c.Port, c.Database, c.Charset)
	}
}

func Open(cfg Config) (*gorm.DB, error) {
	var dialector gorm.Dialector

	switch cfg.Type {
	case Postgres:
		dialector = postgres.Open(cfg.DSN())
	default: // MySQL, TiDB, TDSQL
		dialector = mysql.Open(cfg.DSN())
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   gormLogger.Default.LogMode(gormLogger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open database failed: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB failed: %w", err)
	}

	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	return db, nil
}

func Init(cfg Config, models ...interface{}) error {
	var err error

	dbOnce.Do(func() {
		DB, err = Open(cfg)
		if err != nil {
			err = fmt.Errorf("init database failed: %w", err)
			return
		}

		if len(models) > 0 {
			if err = DB.AutoMigrate(models...); err != nil {
				err = fmt.Errorf("auto migrate failed: %w", err)
				return
			}
		}
	})

	return err
}

func GetDB() *gorm.DB {
	if DB == nil {
		panic("database not initialized, call Init() first")
	}
	return DB
}

func WithContext(ctx context.Context) *gorm.DB {
	return GetDB().WithContext(ctx)
}

func Transaction(fn func(tx *gorm.DB) error) error {
	return GetDB().Transaction(fn)
}

func Close() error {
	if DB == nil {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}

func DropIndex(value interface{}, indexName string) error {
	db := GetDB()

	if !db.Migrator().HasIndex(value, indexName) {
		return nil
	}

	err := db.Migrator().DropIndex(value, indexName)
	if err != nil && !strings.Contains(err.Error(), "1091") {
		return fmt.Errorf("drop index %s failed: %w", indexName, err)
	}

	return nil
}

// Initialize (backward compatibility)
func init() {
	if config.Cfg == nil {
		return
	}

	// Compatibility with old configurations
	if config.Cfg.Database.Host == "" && config.Cfg.Mysql.Host != "" {
		// Use old MySQL configuration
		cfg := Config{
			Type:            MySQL,
			Host:            config.Cfg.Mysql.Host,
			Port:            config.Cfg.Mysql.Port,
			User:            config.Cfg.Mysql.User,
			Password:        config.Cfg.Mysql.Password,
			Database:        "oneterm",
			Charset:         "utf8mb4",
			MaxIdleConns:    10,
			MaxOpenConns:    100,
			ConnMaxLifetime: time.Hour,
			ConnMaxIdleTime: time.Minute * 10,
		}

		if err := Init(cfg); err != nil {
			logger.L().Fatal("init database failed", zap.Error(err))
		}
		return
	}

	// Use new configuration
	if config.Cfg.Database.Host != "" {
		cfg := ConfigFromGlobal()

		if err := Init(cfg); err != nil {
			logger.L().Fatal("init database failed", zap.Error(err))
		}
	}
}
