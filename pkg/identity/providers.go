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

package identity

import (
	"context"

	"github.com/google/uuid"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/meta/mdkeys"
	"github.com/kubecc-io/kubecc/pkg/types"
)

type componentMDP struct{}

var Component componentMDP

func (componentMDP) Key() meta.MetadataKey {
	return mdkeys.ComponentKey
}

func (componentMDP) Marshal(i interface{}) string {
	return types.Component_name[int32(i.(types.Component))]
}

func (componentMDP) Unmarshal(s string) (interface{}, error) {
	return types.Component(types.Component_value[s]), nil
}

type uuidMDP struct{}

var UUID uuidMDP

func (uuidMDP) Key() meta.MetadataKey {
	return mdkeys.UUIDKey
}

func (uuidMDP) InitialValue(context.Context) interface{} {
	return uuid.NewString()
}

func (uuidMDP) Marshal(i interface{}) string {
	return i.(string)
}

func (uuidMDP) Unmarshal(s string) (interface{}, error) {
	return s, nil
}
