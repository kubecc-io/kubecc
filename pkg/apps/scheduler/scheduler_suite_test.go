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
