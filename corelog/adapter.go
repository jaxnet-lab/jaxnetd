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

type LogAdapter struct {
	Logger *zap.Logger
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
	res := &LogAdapter{
		Logger: logger,
	}
	return res
}

func (l *LogAdapter) Tracef(format string, params ...interface{}) {
	if params != nil {
		l.Logger.Debug(fmt.Sprintf(format, params...))
	} else {
		l.Logger.Debug(format)
	}
}

func (l *LogAdapter) Debugf(format string, params ...interface{}) {
	if params != nil {
		l.Logger.Debug(fmt.Sprintf(format, params...))
	} else {
		l.Logger.Debug(format)
	}

}

func (l *LogAdapter) Infof(format string, params ...interface{}) {
	if params != nil {
		l.Logger.Info(fmt.Sprintf(format, params...))
	} else {
		l.Logger.Info(format)
	}
}

func (l *LogAdapter) Warnf(format string, params ...interface{}) {
	if params != nil {
		l.Logger.Warn(fmt.Sprintf(format, params...))
	} else {
		l.Logger.Warn(format)
	}
}

func (l *LogAdapter) Errorf(format string, params ...interface{}) {
	if params != nil {
		l.Logger.Error(fmt.Sprintf(format, params...))
	} else {
		l.Logger.Error(format)
	}
}

func (l *LogAdapter) Trace(format string) {
	l.Logger.Debug("TRACE: " + format)
}

func (l *LogAdapter) Debug(format string) {
	l.Logger.Debug(format)
}

func (l *LogAdapter) Info(format string) {
	l.Logger.Info(format)
}

func (l *LogAdapter) Warn(format string) {
	l.Logger.Warn(format)
}

func (l *LogAdapter) Error(format string) {
	l.Logger.Error(format)
}
