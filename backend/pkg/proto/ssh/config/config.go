package config

import (
	"sync"
)

type Config struct {
	Api   string `yaml:"api"`
	Token string `yaml:"token"`

	Ip   string `yaml:"ip"`
	Port int    `yaml:"port"`

	WebUser  string `yaml:"webUser"`
	WebToken string `yaml:"webToken"`

	RecordFilePath string `yaml:"recordFilePath"`
	PrivateKeyPath string `yaml:"privateKeyPath"`

	PlainMode bool `yaml:"plainMode"`
}

var (
	SSHConfig        Config
	TotalMonitors    = sync.Map{}
	TotalHostSession = sync.Map{}
	Assets           = sync.Map{}
)
