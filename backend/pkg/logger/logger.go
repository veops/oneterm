package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/veops/oneterm/pkg/config"
)

func init() {
	level := zapcore.DebugLevel
	switch config.Cfg.Log.Level {
	case "error":
		level = zapcore.ErrorLevel
	case "warn":
		level = zapcore.WarnLevel
	case "info":
		level = zapcore.InfoLevel
	case "debug":
		level = zapcore.DebugLevel
	}
	fw := &lumberjack.Logger{
		Filename:   "logs/oneterm.log",
		MaxSize:    0,
		MaxBackups: 0,
		MaxAge:     15,
		LocalTime:  false,
		Compress:   false,
	}
	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000")
	encoder := zapcore.NewConsoleEncoder(cfg)
	cores := []zapcore.Core{zapcore.NewCore(
		encoder,
		zapcore.AddSync(fw),
		level,
	)}
	if config.Cfg.Log.ConsoleEnable {
		cores = append(cores, zapcore.NewCore(
			encoder,
			zapcore.AddSync(zapcore.Lock(os.Stderr)),
			level,
		))
	}
	zap.ReplaceGlobals(zap.New(zapcore.NewTee(cores...)))
}

func L() *zap.Logger {
	return zap.L()
}
