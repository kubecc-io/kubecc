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

func (r *RunRequest) MarshalLogObject(enc zapcore.ObjectEncoder) error {
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
	enc.AddInt("dataLen", len(r.CompiledSource))
	return nil
}

func (r *CompileStatus) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("status", r.GetCompileStatus().String())
	enc.AddInt("dataLen", len(r.CompiledSource))
	return nil
}
