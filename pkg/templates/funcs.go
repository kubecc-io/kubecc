package templates

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
	"sigs.k8s.io/yaml"
)

func toYAML(i interface{}) (string, error) {
	out, err := yaml.Marshal(i)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func toHost(i interface{}) string {
	return fmt.Sprintf("Host(`%s`)", i)
}

func Funcs() template.FuncMap {
	m := sprig.TxtFuncMap()
	m["toYaml"] = toYAML
	m["toHost"] = toHost
	return m
}
