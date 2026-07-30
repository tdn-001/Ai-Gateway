package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	log        *zap.Logger
	currentLog string
)

func Init() {
	os.MkdirAll("./data/logs", 0755)

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)

	currentLog = filepath.Join("data", "logs", fmt.Sprintf("gateway_%s.log", time.Now().Format("2006-01-02")))
	logFile, _ := os.OpenFile(currentLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	fileEncoder := zapcore.NewJSONEncoder(encoderConfig)

	core := zapcore.NewTee(
		zapcore.NewCore(fileEncoder, zapcore.AddSync(logFile), zapcore.InfoLevel),
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), zapcore.InfoLevel),
	)

	log = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
}

func Sync() {
	if log != nil {
		log.Sync()
	}
}

func Info(msg string, fields ...zap.Field) {
	if log != nil {
		log.Info(msg, fields...)
	}
}

func Error(msg string, fields ...zap.Field) {
	if log != nil {
		log.Error(msg, fields...)
	}
}

func Debug(msg string, fields ...zap.Field) {
	if log != nil {
		log.Debug(msg, fields...)
	}
}

func Warn(msg string, fields ...zap.Field) {
	if log != nil {
		log.Warn(msg, fields...)
	}
}

func Fatal(msg string, fields ...zap.Field) {
	if log != nil {
		log.Fatal(msg, fields...)
	}
}