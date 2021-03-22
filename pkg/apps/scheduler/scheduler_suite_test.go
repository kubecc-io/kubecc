package scheduler

import (
	"testing"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestScheduler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Scheduler Suite")
}

var (
	testCtx = meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.TestComponent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger),
		meta.WithProvider(tracing.Tracer),
	)
	clang_c = &types.Toolchain{
		Kind:       types.Clang,
		Lang:       types.C,
		Executable: "clang-c",
		TargetArch: "amd64",
		Version:    "1.0",
		PicDefault: true,
	}
	gnu_c = &types.Toolchain{
		Kind:       types.Gnu,
		Lang:       types.C,
		Executable: "gnu-c",
		TargetArch: "amd64",
		Version:    "1.0",
		PicDefault: true,
	}
	sample_req1 = &types.CompileRequest{
		RequestID:          uuid.NewString(),
		Toolchain:          clang_c,
		Args:               []string{},
		PreprocessedSource: []byte("test"),
	}
	sample_req2 = &types.CompileRequest{
		RequestID:          uuid.NewString(),
		Toolchain:          gnu_c,
		Args:               []string{},
		PreprocessedSource: []byte("test2"),
	}
	sample_resp1 = &types.CompileResponse{
		RequestID:     sample_req1.RequestID,
		CompileResult: types.CompileResponse_Success,
		Data: &types.CompileResponse_CompiledSource{
			CompiledSource: []byte("test"),
		},
	}
	sample_resp2 = &types.CompileResponse{
		RequestID:     sample_req2.RequestID,
		CompileResult: types.CompileResponse_Success,
		Data: &types.CompileResponse_CompiledSource{
			CompiledSource: []byte("test2"),
		},
	}
)

type mockTcWatcher struct {
	C map[string]chan *metrics.Toolchains
}

func (b mockTcWatcher) WatchToolchains(uuid string) chan *metrics.Toolchains {
	return b.C[uuid]
}
