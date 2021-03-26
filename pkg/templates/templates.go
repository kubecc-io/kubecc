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

// Package templates contains functions to load Kubernetes resources from
// YAML templates, similar to those used in Helm charts.
package templates

import (
	"bytes"
	"io/fs"
	"path/filepath"
	"text/template"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type wrapper struct {
	Spec interface{}
}

// Load parses a template file and returns the processed text. The file is
// searched for in the given filesystem. A spec can be passed to the template
// loader which will be applied when processing the template.
func Load(fsys fs.FS, name string, spec interface{}) ([]byte, error) {
	tmpl := template.New(filepath.Base(name)).Funcs(Funcs())
	tmpl, err := tmpl.ParseFS(fsys, name)
	if err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, wrapper{Spec: spec})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Unmarshal decodes text data (JSON/YAML) into a Kubernetes object.
func Unmarshal(data []byte, scheme *runtime.Scheme) (client.Object, error) {
	ds := serializer.NewCodecFactory(scheme).UniversalDeserializer()
	out, _, err := ds.Decode(data, nil, nil)
	if err != nil {
		return nil, err
	}
	return out.(client.Object), nil
}
