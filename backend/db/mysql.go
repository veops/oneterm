package mysql

import (
	"fmt"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/veops/oneterm/conf"
	"github.com/veops/oneterm/logger"
)

var (
	DB *gorm.DB
)

func init() {
	var err error
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/oneterm?charset=utf8mb4&parseTime=True&loc=Local",
		conf.Cfg.Mysql.User, conf.Cfg.Mysql.Password, conf.Cfg.Mysql.Host, conf.Cfg.Mysql.Port)
	if DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{}); err != nil {
		logger.L().Fatal("init mysql failed", zap.Error(err))
	}
}
