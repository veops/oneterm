package mysql

import (
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/veops/oneterm/pkg/conf"
)

var (
	DB *gorm.DB
)

func Init(cfg *conf.MysqlConfig) (err error) {
	if cfg == nil {
		return
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/oneterm?charset=utf8mb4&parseTime=True&loc=Local", cfg.User, cfg.Password, cfg.Ip, cfg.Port)
	if DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{}); err != nil {
		return
	}

	return
}
