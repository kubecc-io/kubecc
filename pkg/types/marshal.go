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

package types

import (
	"go.uber.org/zap/zapcore"
)

type stringSliceEncoder struct {
	zapcore.ArrayMarshaler
	data []string
}

func (s stringSliceEncoder) MarshalLogArray(enc zapcore.ArrayEncoder) error {
	for _, s := range s.data {
		enc.AppendString(s)
	}
	return nil
}

func NewStringSliceEncoder(slice []string) stringSliceEncoder {
	return stringSliceEncoder{
		data: slice,
	}
}

func (r *RunRequest) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("dir", r.GetWorkDir())
	enc.AddUint32("uid", r.GetUID())
	enc.AddUint32("gid", r.GetGID())
	_ = enc.AddArray("args", NewStringSliceEncoder(r.Args))
	return nil
}

func (r *RunResponse) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddInt("code", int(r.GetReturnCode()))
	return nil
}

func (r *CompileResponse) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	switch data := r.GetData().(type) {
	case *CompileResponse_Error:
		enc.AddString("error", data.Error)
	case *CompileResponse_CompiledSource:
		enc.AddInt("dataLen", len(data.CompiledSource))
	}
	return nil
}
