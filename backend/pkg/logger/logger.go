package logger

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/veops/oneterm/pkg/conf"
)

var (
	L           *zap.Logger
	AtomicLevel = zap.NewAtomicLevel()
)

func Init(ctx context.Context, cfg *conf.LogConfig) (err error) {
	err = initLogger(cfg)
	if err != nil {
		return
	}

	L = zap.L()

	go func() {
		<-ctx.Done()
		err = L.Sync()
		if err != nil {
			fmt.Println(err)
		}
	}()

	return nil
}

func getEncoder(format string) zapcore.Encoder {

	encodeConfig := zap.NewProductionEncoderConfig()
	encodeConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encodeConfig.TimeKey = "time"
	encodeConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encodeConfig.EncodeCaller = zapcore.ShortCallerEncoder

	if strings.ToUpper(format) == "JSON" {
		return zapcore.NewJSONEncoder(encodeConfig)
	} else {
		encodeConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		return zapcore.NewConsoleEncoder(encodeConfig)
	}
}

func getLogWriter(cfg *conf.LogConfig) zapcore.Core {
	var cores []zapcore.Core

	if cfg.Path != "" {
		logRotate := &lumberjack.Logger{
			Filename:   cfg.Path,
			MaxSize:    cfg.MaxSize,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,
			Compress:   cfg.Compress,
		}
		fileEncoder := getEncoder(cfg.Format)
		cores = append(cores, zapcore.NewCore(fileEncoder, zapcore.AddSync(logRotate), AtomicLevel))
	}

	if cfg.ConsoleEnable {
		consoleEncoder := getEncoder("console")
		cores = append(cores, zapcore.NewCore(consoleEncoder, zapcore.Lock(os.Stdout), AtomicLevel))
	}

	return zapcore.NewTee(cores...)
}

func initLogger(cfg *conf.LogConfig) (err error) {

	level, err := zap.ParseAtomicLevel(cfg.Level)
	if err != nil {
		return err
	}
	AtomicLevel.SetLevel(level.Level())

	core := getLogWriter(cfg)

	logger := zap.New(core, zap.AddCaller())
	zap.ReplaceGlobals(logger)

	return
}
