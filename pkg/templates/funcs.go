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
