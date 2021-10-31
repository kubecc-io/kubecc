/*
Copyright 2021 The Kubecc Authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package util

import (
	"github.com/go-logr/logr"
	"go.uber.org/zap"
)

// ZapfLogShim implements the logr.Logger interface.
type ZapfLogShim struct {
	ZapLogger *zap.SugaredLogger
}

func (lg ZapfLogShim) Enabled() bool {
	return true
}

func translateKeyValuePairs(keysAndValues ...interface{}) []interface{} {
	args := []interface{}{}
	for i := 0; i < len(keysAndValues); i += 2 {
		args = append(args, zap.Any(keysAndValues[i].(string), keysAndValues[i+1]))
	}
	return args
}

func (lg ZapfLogShim) Info(msg string, keysAndValues ...interface{}) {
	lg.ZapLogger.With(translateKeyValuePairs(keysAndValues...)...).Info(msg)
}

func (lg ZapfLogShim) Error(err error, msg string, keysAndValues ...interface{}) {
	lg.ZapLogger.With(
		append([]interface{}{zap.Error(err)}, translateKeyValuePairs(keysAndValues...)...),
	).Error(msg)
}

func (lg ZapfLogShim) V(level int) logr.Logger {
	return lg
}

func (lg ZapfLogShim) WithValues(keysAndValues ...interface{}) logr.Logger {
	return ZapfLogShim{
		ZapLogger: lg.ZapLogger.With(translateKeyValuePairs(keysAndValues...)...),
	}
}

func (lg ZapfLogShim) WithName(name string) logr.Logger {
	return ZapfLogShim{
		ZapLogger: lg.ZapLogger.Named(name),
	}
}
