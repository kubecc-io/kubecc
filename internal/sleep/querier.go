package sleep

import (
	"runtime"
	"time"

	"github.com/cobalt77/kubecc/pkg/toolchains"
	"github.com/cobalt77/kubecc/pkg/types"
)

type SleepQuerier struct{}

func (q SleepQuerier) IsPicDefault(compiler string) (bool, error) {
	return true, nil
}

func (q SleepQuerier) TargetArch(compiler string) (string, error) {
	return runtime.GOARCH, nil
}

func (q SleepQuerier) Version(compiler string) (string, error) {
	return "1.0", nil
}

func (q SleepQuerier) Kind(compiler string) (types.ToolchainKind, error) {
	return types.Sleep, nil
}

func (q SleepQuerier) Lang(compiler string) (types.ToolchainLang, error) {
	return types.CXX, nil
}

func (q SleepQuerier) ModTime(compiler string) (time.Time, error) {
	return toolchains.ExecQuerier{}.ModTime(compiler)
}
