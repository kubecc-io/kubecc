package host

import (
	"context"
	"fmt"
	"runtime"

	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/meta/mdkeys"
	"github.com/cobalt77/kubecc/pkg/types"
	"google.golang.org/protobuf/encoding/protojson"
)

func GetSystemInfo() *types.SystemInfo {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return &types.SystemInfo{
		Arch:         runtime.GOARCH,
		CpuThreads:   int32(runtime.NumCPU()),
		SystemMemory: memStats.Sys,
	}
}

type systemInfoProvider struct{}

var SystemInfo systemInfoProvider

func (systemInfoProvider) Key() meta.MetadataKey {
	return mdkeys.SystemInfoKey
}

func (systemInfoProvider) InitialValue(context.Context) interface{} {
	return GetSystemInfo()
}

func (systemInfoProvider) Marshal(i interface{}) string {
	data, err := protojson.Marshal(i.(*types.SystemInfo))
	if err != nil {
		panic(fmt.Sprintf("Could not marshal SystemInfo: %s", err.Error()))
	}
	return string(data)
}

func (systemInfoProvider) Unmarshal(s string) (interface{}, error) {
	info := &types.SystemInfo{}
	err := protojson.Unmarshal([]byte(s), info)
	if err != nil {
		return nil, err
	}
	return info, nil
}
