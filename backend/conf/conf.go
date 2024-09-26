package conf

import (
	"fmt"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	RESOURCE_NODE          = "node"
	RESOURCE_ACCOUNT       = "account"
	RESOURCE_ASSET         = "asset"
	RESOURCE_COMMAND       = "command"
	RESOURCE_GATEWAY       = "gateway"
	RESOURCE_AUTHORIZATION = "authorization"
)

var (
	PermResource = []string{RESOURCE_NODE, RESOURCE_ACCOUNT, RESOURCE_ASSET, RESOURCE_COMMAND, RESOURCE_GATEWAY}

	Cfg = &ConfigYaml{
		Mode: "debug",
		Http: HttpConfig{
			Host: "0.0.0.0",
			Port: 80,
		},
		Log: LogConfig{
			Level:         "info",
			MaxSize:       100, // megabytes
			MaxBackups:    5,
			MaxAge:        15, // 15 days
			Compress:      true,
			Path:          "app.log",
			ConsoleEnable: true,
		},
	}
)

func init() {
	path := pflag.StringP("config", "c", "config.yaml", "config path")
	pflag.Parse()

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(*path)
	viper.AddConfigPath(".")

	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error config file: %s", err))
	}

	if err = viper.Unmarshal(Cfg); err != nil {
		panic(fmt.Sprintf("parse config from config.yaml failed:%s", err))
	}

}

type HttpConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
}

type MysqlConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

type KV struct {
	Key   string
	Value string
}

type AclConfig struct {
	Url       string `yaml:"url"`
	AppId     string `yaml:"appId"`
	SecretKey string `yaml:"secretKey"`
}

type AesConfig struct {
	Key string `yaml:"key"`
	Iv  string `yaml:"iv"`
}

type LogConfig struct {
	Level string `yaml:"level"`
	Path  string `yaml:"path"`
	// MaxSize max size of single file, unit is MB
	MaxSize int `yaml:"maxSize"`
	// MaxBackups max number of backup files
	MaxBackups int `yaml:"maxBackups"`
	// MaxAge max days of backup files, unit is day
	MaxAge int `yaml:"maxAge"`
	// Compress whether compress backup file
	Compress bool `yaml:"compress"`
	// Format
	Format string `yaml:"format"`
	// Console output
	ConsoleEnable bool `yaml:"consoleEnable"`
}

type Auth struct {
	Acl AclConfig `yaml:"acl"`
	Aes AesConfig `yaml:"aes"`
}

type SshConfig struct {
	Host       string `yaml:"host"`
	Port       int    `yaml:"port"`
	PrivateKey string `yaml:"privateKey"`
}

type GuacdConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type ConfigYaml struct {
	Mode      string      `yaml:"mode"`
	I18nDir   string      `yaml:"i18nDir"`
	Log       LogConfig   `yaml:"log"`
	Redis     RedisConfig `yaml:"redis"`
	Mysql     MysqlConfig `yaml:"mysql"`
	Guacd     GuacdConfig `yaml:"guacd"`
	Http      HttpConfig  `yaml:"http"`
	Ssh       SshConfig   `yaml:"ssh"`
	Auth      Auth        `yaml:"auth"`
	SecretKey string      `yaml:"secretKey"`
}
