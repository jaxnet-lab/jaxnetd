package network

import (
	"fmt"

	"go.uber.org/zap"
)

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

func LogAdapter(logger *zap.Logger) ILogger {
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
		fmt.Println("warn format...", format, l.logger)
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
