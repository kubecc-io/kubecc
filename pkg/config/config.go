package config

import (
	"context"
	"encoding/json"
	"os"
	"path"
	"strings"

	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"k8s.io/client-go/util/homedir"
	"sigs.k8s.io/yaml"
)

type ConfigProvider interface {
	Setup(types.Component)
}

type ConfigMapProvider struct{}

func loadConfigOrDie(lg *zap.SugaredLogger, path string) *KubeccSpec {
	contents, err := os.ReadFile(path)
	if err != nil {
		lg.With(
			zap.Error(err),
			zap.String("path", path),
		).Fatal("Error reading config file")
	}
	cfg := &KubeccSpec{}
	if strings.HasSuffix(path, ".json") {
		err = json.Unmarshal(contents, cfg)
	} else {
		err = yaml.Unmarshal(contents, cfg, yaml.DisallowUnknownFields)
	}
	if err != nil {
		lg.With(
			zap.Error(err),
			zap.String("path", path),
		).Fatal("Error parsing config file")
	}
	return cfg
}

func (cmp *ConfigMapProvider) Load(ctx context.Context) *KubeccSpec {
	lg := meta.Log(ctx)
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
			return loadConfigOrDie(lg, abs)
		}
	}
	lg.Fatal("Could not find config file")
	return nil
}
