package sleep

import (
	"context"
	"fmt"
	"os"

	"github.com/cobalt77/kubecc/pkg/toolchains"
)

type SleepToolchainFinder struct{}

func (f SleepToolchainFinder) FindToolchains(
	ctx context.Context,
	_ ...toolchains.FindOption,
) *toolchains.Store {
	executable, err := os.Executable()
	if err != nil {
		panic(fmt.Sprintf("Could not find the current executable: %s", err.Error()))
	}
	store := toolchains.NewStore()
	_, _ = store.Add(executable, SleepQuerier{})
	return store
}
