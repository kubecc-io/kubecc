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

package mdkeys

type componentKeyType struct{}
type uuidKeyType struct{}
type logKeyType struct{}
type tracingKeyType struct{}
type systemInfoKeyType struct{}

func (componentKeyType) String() string {
	return "kubecc-component"
}

func (uuidKeyType) String() string {
	return "kubecc-uuid"
}

func (logKeyType) String() string {
	return "kubecc-log"
}

func (tracingKeyType) String() string {
	return "kubecc-tracing"
}

func (systemInfoKeyType) String() string {
	return "kubecc-systeminfo"
}

var (
	ComponentKey  componentKeyType
	UUIDKey       uuidKeyType
	LogKey        logKeyType
	TracingKey    tracingKeyType
	SystemInfoKey systemInfoKeyType
)
