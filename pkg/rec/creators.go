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

package rec

import (
	"embed"
	_ "embed"

	"github.com/kubecc-io/kubecc/pkg/templates"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//go:embed objects
var objectsFS embed.FS

type mustExist struct {
	error
}

func (e mustExist) Error() string {
	return "Object must already exist"
}

var (
	MustExist ObjectCreator = func(ResolveContext) (client.Object, error) {
		return nil, &mustExist{}
	}
)

func FromTemplate(templateName string) ObjectCreator {
	return func(rc ResolveContext) (client.Object, error) {
		tmplData, err := templates.Load(objectsFS, templateName, templates.LoadSpec{
			Spec: rc.Object,
			Root: rc.RootObject,
		})
		if err != nil {
			return nil, err
		}
		obj, err := templates.Unmarshal(tmplData, rc.Client.Scheme())
		if err != nil {
			return nil, err
		}
		obj.SetNamespace(rc.RootObject.GetNamespace())
		return obj, nil
	}
}
