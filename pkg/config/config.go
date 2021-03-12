package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"reflect"
	"strings"

	"github.com/cobalt77/kubecc/pkg/types"
	"k8s.io/client-go/util/homedir"
	"sigs.k8s.io/yaml"
)

type ConfigProvider interface {
	Setup(types.Component)
}

type ConfigMapProvider struct{}

// applyGlobals walks the config structure of KubeccSpec (depth 1), finds any
// structs that contain a *GlobalSpec field, and syncs non-overridden global
// fields from the top level KubeccSpec globals.
func applyGlobals(cfg *KubeccSpec) {
	cfgValue := reflect.ValueOf(cfg).Elem()
	for i := 0; i < cfgValue.NumField(); i++ {
		component := cfgValue.Field(i)
		if component.Type() == reflect.TypeOf(cfg.Global) {
			continue
		}
		for j := 0; j < component.NumField(); j++ {
			if f := component.Field(j); f.Type() == reflect.TypeOf(cfg.Global) {
				f.Interface().(GlobalSpec).LoadIfUnset(cfg.Global)
			}
		}
	}
	cfg.Agent.GlobalSpec.LoadIfUnset(cfg.Global)
}

func loadConfigOrDie(path string) *KubeccSpec {
	contents, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("Error reading config file %s: %s", path, err.Error()))
	}
	cfg := &KubeccSpec{}
	if strings.HasSuffix(path, ".json") {
		err = json.Unmarshal(contents, cfg)
	} else {
		err = yaml.Unmarshal(contents, cfg, yaml.DisallowUnknownFields)
	}
	if err != nil {
		panic(fmt.Sprintf("Error parsing config file %s: %s", path, err.Error()))
	}
	applyGlobals(cfg)
	return cfg
}

func (cmp *ConfigMapProvider) Load() *KubeccSpec {
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
			return loadConfigOrDie(abs)
		}
	}
	panic("Could not find config file")
}
