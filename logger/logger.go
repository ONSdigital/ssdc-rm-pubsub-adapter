package logger

import (
	"github.com/ONSdigital/ssdc-rm-pubsub-adapter/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.SugaredLogger

func getZapLevel(textLevel string) (zap.AtomicLevel, error) {
	level := zap.AtomicLevel{}
	err := level.UnmarshalText([]byte(textLevel))
	return level, err
}

func init() {
	// Initialise a default dev logger
	initLogger, _ := zap.NewDevelopment()
	Logger = initLogger.Sugar()
}

func ConfigureLogger(cfg *config.Configuration) error {
	logLevel, err := getZapLevel(cfg.LogLevel)
	if err != nil {
		return err
	}
	initLogger, err := zap.Config{
		Encoding:         "json",
		Level:            logLevel,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stdout"},
		EncoderConfig: zapcore.EncoderConfig{
			MessageKey: "message",

			LevelKey:    "severity",
			EncodeLevel: zapcore.CapitalLevelEncoder,

			TimeKey:    "timestamp",
			EncodeTime: zapcore.RFC3339NanoTimeEncoder,

			CallerKey:    "caller",
			EncodeCaller: zapcore.ShortCallerEncoder,
		}}.Build()
	if err != nil {
		return err
	}
	defer Logger.Sync()
	Logger = initLogger.Sugar()
	return nil
}
