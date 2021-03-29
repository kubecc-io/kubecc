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

package host

import (
	"context"
	"fmt"
	"runtime"

	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/meta/mdkeys"
	"github.com/kubecc-io/kubecc/pkg/types"
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
