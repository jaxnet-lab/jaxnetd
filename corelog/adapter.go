package corelog

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	Disabled    ILogger
	DisabledZap *zap.Logger

	DefaultLevel   = zap.InfoLevel
	DefaultLogFile = "shard.core.log"
)

func init() {
	Disabled = Adapter(zap.NewNop())
	DisabledZap = zap.NewNop()
}

type ILogger interface {
	Trace(format string)
	Debug(format string)
	Info(format string)
	Warn(format string)
	Error(format string)
	Tracef(format string, params ...interface{})
	Debugf(format string, params ...interface{})
	Infof(format string, params ...interface{})
	Warnf(format string, params ...interface{})
	Errorf(format string, params ...interface{})
}

type logAdapter struct {
	logger *zap.Logger
}

func New(logLevel zapcore.Level, file string, disableStdOut bool) *zap.Logger {
	w := zapcore.AddSync(&lumberjack.Logger{
		Filename:   file,
		MaxSize:    100, // megabytes
		MaxBackups: 3,
		MaxAge:     28, // days
	})

	core := zapcore.NewTee(
		zapcore.NewCore(zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()), os.Stdout, logLevel),
		zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), w, logLevel),
	)

	if disableStdOut {
		core = zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), w, logLevel)
	}

	log := zap.New(core).With(zap.String("app", "shard.core"))
	return log
}

func Adapter(logger *zap.Logger) ILogger {
	res := &logAdapter{
		logger: logger,
	}
	return res
}

func (l *logAdapter) Tracef(format string, params ...interface{}) {
	if params != nil {
		l.logger.Debug(fmt.Sprintf(format, params...))
	} else {
		l.logger.Debug(format)
	}
}

func (l *logAdapter) Debugf(format string, params ...interface{}) {
	if params != nil {
		l.logger.Debug(fmt.Sprintf(format, params...))
	} else {
		l.logger.Debug(format)
	}

}

func (l *logAdapter) Infof(format string, params ...interface{}) {
	if params != nil {
		l.logger.Info(fmt.Sprintf(format, params...))
	} else {
		l.logger.Info(format)
	}
}

func (l *logAdapter) Warnf(format string, params ...interface{}) {
	if params != nil {
		l.logger.Warn(fmt.Sprintf(format, params...))
	} else {
		l.logger.Warn(format)
	}
}

func (l *logAdapter) Errorf(format string, params ...interface{}) {
	if params != nil {
		l.logger.Error(fmt.Sprintf(format, params...))
	} else {
		l.logger.Error(format)
	}
}

func (l *logAdapter) Trace(format string) {
	l.logger.Debug(format)
}

func (l *logAdapter) Debug(format string) {
	l.logger.Debug(format)
}

func (l *logAdapter) Info(format string) {
	l.logger.Info(format)
}

func (l *logAdapter) Warn(format string) {
	l.logger.Warn(format)
}

func (l *logAdapter) Error(format string) {
	l.logger.Error(format)
}
