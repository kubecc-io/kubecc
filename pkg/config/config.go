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

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"reflect"
	"strings"

	"github.com/kubecc-io/kubecc/internal/zapkc"
	"k8s.io/client-go/util/homedir"
	"sigs.k8s.io/yaml"
)

type ConfigLoader interface {
	Load() *KubeccSpec
}

type configMapProvider struct{}

var ConfigMapProvider configMapProvider

// ApplyGlobals walks the config structure of KubeccSpec (depth 1), finds any
// structs that contain a *GlobalSpec field, and syncs non-overridden global
// fields from the top level KubeccSpec globals.
func ApplyGlobals(cfg *KubeccSpec) {
	cfgValue := reflect.ValueOf(cfg).Elem()
	for i := 0; i < cfgValue.NumField(); i++ {
		component := cfgValue.Field(i)
		if component.Type() == reflect.TypeOf(cfg.Global) {
			continue
		}
		for j := 0; j < component.NumField(); j++ {
			if f := component.Field(j); f.Type() == reflect.TypeOf(cfg.Global) {
				override := f.Interface().(GlobalSpec)
				override.Merge(cfg.Global)
				f.Set(reflect.ValueOf(override))
			}
		}
	}
}

func LoadFile(path string) *KubeccSpec {
	contents, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, zapkc.Red.Add(fmt.Sprintf("Error reading config file %s: %s", path, err.Error())))
	}
	cfg := &KubeccSpec{}
	if strings.HasSuffix(path, ".json") {
		err = json.Unmarshal(contents, cfg)
	} else {
		err = yaml.Unmarshal(contents, cfg, yaml.DisallowUnknownFields)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, zapkc.Red.Add(fmt.Sprintf("Error parsing config file %s: %s", path, err.Error())))
	}
	ApplyGlobals(cfg)
	return cfg
}

func (cmp configMapProvider) Load() *KubeccSpec {
	paths := []string{
		"/etc/kubecc",
		path.Join(homedir.HomeDir(), ".kubecc"),
	}
	filenames := []string{
		"config.yaml",
		"config.yml",
		"config.json",
	}
	for _, p := range paths {
		for _, f := range filenames {
			abs := path.Join(p, f)
			if _, err := os.Stat(abs); err != nil {
				continue
			}
			return LoadFile(abs)
		}
	}
	fmt.Fprintln(os.Stderr, zapkc.Red.Add("Could not find config file (try running `kubecc setup`)"))
	os.Exit(1)
	return nil
}
