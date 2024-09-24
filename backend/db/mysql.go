package mysql

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/veops/oneterm/conf"
	"github.com/veops/oneterm/logger"
	"github.com/veops/oneterm/model"
)

var (
	DB *gorm.DB
)

func init() {
	var err error
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/oneterm?charset=utf8mb4&parseTime=True&loc=Local",
		conf.Cfg.Mysql.User, conf.Cfg.Mysql.Password, conf.Cfg.Mysql.Host, conf.Cfg.Mysql.Port)
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		logger.L().Fatal("init mysql failed", zap.Error(err))
	}

	err = DB.AutoMigrate(
		&model.Account{}, &model.Asset{}, &model.Authorization{}, &model.Command{},
		&model.Config{}, &model.FileHistory{}, &model.Gateway{}, &model.History{},
		&model.Node{}, &model.PublicKey{}, &model.Session{}, &model.Share{},
	)
	if err != nil {
		logger.L().Fatal("auto migrate mysql failed", zap.Error(err))
	}

	dropIndexs := map[string]any{
		"asset_account_id_del": &model.Authorization{},
	}
	for k, v := range dropIndexs {
		if !DB.Migrator().HasIndex(v, k) {
			continue
		}
		if err = DB.Migrator().DropIndex(v, k); err != nil && !strings.Contains(err.Error(), "1091") {
			logger.L().Fatal("drop index failed", zap.Error(err))
		}
	}
}
