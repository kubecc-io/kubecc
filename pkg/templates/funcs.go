package templates

import (
	"fmt"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
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
	return template.FuncMap{
		"toYaml": toYAML,
		"toHost": toHost,
	}
}
