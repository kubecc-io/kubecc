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

package tools

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/config"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var min, max, granularity time.Duration
var numTasks int64

var SleepCmd = &cobra.Command{
	Use:   "sleep",
	Short: "Run `kubecc help sleep` for more info.",
	Long: `The sleep tool will sleep for the given duration bounded by the provided 
min and max, then exit with a 0 status code (unless given invalid arguments).

Sleep is a special command that Kubecc is aware of and can treat as a compiler 
toolchain. This can be used to test if a Kubecc cluster is working. It is 
primarily used as a debugging and development tool.`,
	Run: func(cmd *cobra.Command, args []string) {
		time.Sleep(time.Duration(rand.Int63n(int64(max-min)) + int64(min)).
			Round(granularity))
	},
}

func worker(
	ctx context.Context,
	conf config.ConsumerSpec,
	lg *zap.SugaredLogger,
	queue <-chan struct{},
) {
	lg.Infof("Starting worker")
	defer lg.Info("Worker finished")

	cc, err := servers.Dial(ctx, conf.ConsumerdAddress)
	if err != nil {
		lg.With(zap.Error(err)).Fatal("Error connecting to consumerd")
	}
	consumerd := types.NewConsumerdClient(cc)
	wd, err := os.Getwd()
	if err != nil {
		lg.Fatal(err.Error())
	}
	executable, err := os.Executable()
	if err != nil {
		lg.With(zap.Error(err)).Fatal("Could not locate the current executable")
	}
	counterLen := int(math.Floor(math.Log10(float64(numTasks)))) + 1
	for {
		select {
		case <-queue:
		default:
			return
		}
		lg.Infof("[%0*d/%0*d] Running task",
			counterLen, numTasks-int64(len(queue)),
			counterLen, numTasks)
		_, err = consumerd.Run(ctx, &types.RunRequest{
			Compiler: &types.RunRequest_Path{
				Path: executable,
			},
			Args:    append([]string{"sleep"}, strings.Split(sleepArgs, " ")...),
			Env:     []string{},
			UID:     uint32(os.Getuid()),
			GID:     uint32(os.Getgid()),
			Stdin:   []byte{},
			WorkDir: wd,
		}, grpc.WaitForReady(true))
		if err != nil {
			lg.With(zap.Error(err)).Error("Dispatch error")
			return
		}
	}
}

var jobs int
var sleepArgs string
var arch string

var MakeSleepCmd = &cobra.Command{
	Use: "makesleep numOperations",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return cobra.ExactArgs(1)(cmd, args)
		}
		value, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil || value <= 0 {
			return fmt.Errorf("Argument must be a positive integer greater than zero.")
		}
		return nil
	},
	Short: "Run `kubecc help makesleep` for more info.",
	Long: `The makesleep tool is a command that will simulate a make operation using the 
builtin sleep toolchain. It will spawn consumers which will interact with the 
running Kubecc cluster. This can be used to test if a Kubecc cluster is working. 
It is primarily used as a debugging and development tool.


Controlling the makesleep toolchain:

An important factor when running makesleep is the granularity argument
(-g | --granularity). This controls the number of 'buckets' the random number 
generator will use when computing a random duration.

For example, if the minimum duration is 1 second, the maximum duration is 10 
seconds, and the granularity is 1 second, there would only be 10 possible 
durations the random duration could be (1s, 2s, ..., 9s, 10s). If the 
granularity is 100ms, there would then be 100 possible durations the random 
duration could be (100ms, 200ms, ... 9900ms, 10000ms). 

This is important because the duration is what identifies the sleep command as
unique for caching purposes. When testing the cache server, a very low 
granularity value with a large duration range will result in very little cache
usage as there would be a large number of 'unique' commands that could run.
On the other hand, a small duration range and/or a large granularity value will
result in more cache usage, due to the smaller number of possible 'unique'
commands that could be issued. Tune this value according to the testing 
scenario or the desired cache usage.

The duration of the cache entries generated by the sleep toolchain can be tuned
as to not pollute the cache with long-lasting entries. If the cache duration
is too long, any subsequent makesleep commands could complete faster than 
expected due to cache entries from previous runs. If testing cache usage,
this duration should be set long enough to allow multiple full runs before
the cache entries expire. Note that the expiry duration for a cache entry is 
refreshed on each cache hit of that entry.

The host architecture name can be changed if desired using the --host flag. 
This defaults to the host's actual architecture, but can be used to augment 
the pool of available agents during the run. Only agents matching the given
architecture will be available.
`,
	Run: func(cmd *cobra.Command, args []string) {
		conf := (&config.ConfigMapProvider{}).Load().Consumer
		ctx := meta.NewContext(
			meta.WithProvider(identity.Component, meta.WithValue(types.Consumer)),
			meta.WithProvider(identity.UUID),
			meta.WithProvider(logkc.Logger),
			meta.WithProvider(tracing.Tracer),
		)
		lg := meta.Log(ctx)
		numTasks, _ = strconv.ParseInt(args[0], 10, 64) // already checked in Args
		jobQueue := make(chan struct{}, numTasks)
		for i := int64(0); i < numTasks; i++ {
			jobQueue <- struct{}{}
		}
		nameLen := int(math.Floor(math.Log10(float64(jobs)))) + 1
		wg := sync.WaitGroup{}
		wg.Add(jobs)
		for i := 0; i < jobs; i++ {
			go func(i int) {
				defer wg.Done()
				worker(ctx, conf, lg.Named(fmt.Sprintf("%0*d", nameLen, i)), jobQueue)
			}(i)
		}
		wg.Wait()
	},
}

func init() {
	SleepCmd.Flags().DurationVar(&max, "max", 8*time.Second,
		"Maximum duration to sleep for (exclusive)")
	SleepCmd.Flags().DurationVar(&min, "min", 2*time.Second,
		"Minimum duration to sleep for (inclusive)")
	SleepCmd.Flags().DurationVarP(&granularity, "granularity", "g", 100*time.Millisecond,
		"Granularity of the random duration generated by the sleep tool")

	MakeSleepCmd.Flags().IntVarP(&jobs, "jobs", "j", runtime.NumCPU(),
		"Number of local jobs to run in parallel. Equivalent to make's -j flag.")
	MakeSleepCmd.Flags().StringVar(&arch, "arch", runtime.GOARCH,
		"The architecture of the host")
	MakeSleepCmd.Flags().StringVarP(&sleepArgs, "sleep-args", "a",
		"", "The arguments that will be passed to each sleep command")
}
