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
