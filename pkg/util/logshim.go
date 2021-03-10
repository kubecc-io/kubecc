package util

import (
	"github.com/go-logr/logr"
	"go.uber.org/zap"
)

type ZapfLogShim struct {
	ZapLogger *zap.SugaredLogger
}

func (lg ZapfLogShim) Enabled() bool {
	return true
}

func (lg ZapfLogShim) Info(msg string, keysAndValues ...interface{}) {
	lg.ZapLogger.With(keysAndValues...).Info(msg)
}

func (lg ZapfLogShim) Error(err error, msg string, keysAndValues ...interface{}) {
	lg.ZapLogger.With(
		append([]interface{}{zap.Error(err)}, keysAndValues...),
	).Error(msg)
}

func (lg ZapfLogShim) V(level int) logr.Logger {
	return lg
}

func (lg ZapfLogShim) WithValues(keysAndValues ...interface{}) logr.Logger {
	return ZapfLogShim{
		ZapLogger: lg.ZapLogger.With(keysAndValues...),
	}
}

func (lg ZapfLogShim) WithName(name string) logr.Logger {
	return ZapfLogShim{
		ZapLogger: lg.ZapLogger.Named(name),
	}
}
