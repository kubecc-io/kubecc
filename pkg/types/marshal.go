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

func (a *AgentInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("node", a.GetNode())
	enc.AddString("pod", a.GetPod())
	enc.AddString("ns", a.GetNamespace())

	enc.AddInt("cpus", int(a.GetNumCpus()))
	enc.AddString("arch", a.GetArch())
	return nil
}

func (r *DispatchRequest) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("dir", r.GetWorkDir())
	enc.AddString("cmd", r.GetCommand())
	enc.AddArray("args", NewStringSliceEncoder(r.Args))
	return nil
}

func (r *CompileRequest) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("cmd", r.GetCommand())
	enc.AddArray("args", NewStringSliceEncoder(r.Args))
	enc.AddInt("dataLen", len(r.PreprocessedSource))
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

func (r *CompileStatus) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	switch data := r.GetData().(type) {
	case *CompileStatus_Info:
		enc.AddObject("info", data.Info)
	case *CompileStatus_Error:
		enc.AddString("error", data.Error)
	case *CompileStatus_CompiledSource:
		enc.AddInt("dataLen", len(data.CompiledSource))
	}
	return nil
}
