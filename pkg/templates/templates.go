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

func Unmarshal(data []byte, scheme *runtime.Scheme) (client.Object, error) {
	ds := serializer.NewCodecFactory(scheme).UniversalDeserializer()
	out, _, err := ds.Decode(data, nil, nil)
	if err != nil {
		return nil, err
	}
	return out.(client.Object), nil
}
