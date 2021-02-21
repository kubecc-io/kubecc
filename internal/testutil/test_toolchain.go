package testutil

import (
	"context"
	"flag"
	"time"

	"github.com/cobalt77/kubecc/pkg/toolchains"
	"github.com/cobalt77/kubecc/pkg/types"
)

var TestToolchainExecutable = "/path/to/test-toolchain"

type TestQuerier struct{}

func (q TestQuerier) Version(compiler string) (string, error) {
	return "0", nil
}

func (q TestQuerier) TargetArch(compiler string) (string, error) {
	return "testarch", nil
}

func (q TestQuerier) IsPicDefault(compiler string) (bool, error) {
	return true, nil
}

func (q TestQuerier) Kind(compiler string) (types.ToolchainKind, error) {
	return types.TestToolchain, nil
}

func (q TestQuerier) Lang(compiler string) (types.ToolchainLang, error) {
	return types.CXX, nil
}

func (q TestQuerier) ModTime(compiler string) (time.Time, error) {
	return time.Unix(0, 0), nil
}

type TestToolchainFinder struct{}

func (f TestToolchainFinder) FindToolchains(
	ctx context.Context,
	opts ...toolchains.FindOption,
) *toolchains.Store {
	store := toolchains.NewStore()
	_, _ = store.Add(TestToolchainExecutable, TestQuerier{})
	return store
}

// SleepRunner will sleep for (probably slightly more than) the given duration.
// This runner will "pause time" to a granularity of 1/100th the total duration
// while its goroutine is paused (i.e. while debugging at a breakpoint) by
// chaining multiple small timers in sequence instead of using one timer.
type SleepRunner struct {
	Duration time.Duration
}

func (r *SleepRunner) Run(_ context.Context, _ *types.Toolchain) error {
	for i := 0; i < 100; i++ {
		time.Sleep(r.Duration / 100)
	}
	return nil
}

type TestArgParser struct {
	Args     []string
	Duration time.Duration
}

func (ap *TestArgParser) Parse() {
	var duration string
	set := flag.NewFlagSet("test", flag.PanicOnError)
	set.StringVar(&duration, "duration", "1s", "")
	set.Parse(ap.Args)

	d, err := time.ParseDuration(duration)
	if err != nil {
		panic(err)
	}
	ap.Duration = d
}

func (TestArgParser) CanRunRemote() bool {
	return true
}
