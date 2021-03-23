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

package cluster

import (
	"os"

	v1 "k8s.io/api/core/v1"
)

// GetPodName returns the current pod name from the downward API.
func GetPodName() string {
	value, ok := os.LookupEnv("KUBECC_POD_NAME")
	if !ok {
		panic("KUBECC_POD_NAME not defined")
	}
	return value
}

// GetNamespace returns the current namespace from the downward API.
func GetNamespace() string {
	value, ok := os.LookupEnv("KUBECC_NAMESPACE")
	if !ok {
		panic("KUBECC_NAMESPACE not defined")
	}
	return value
}

// GetNode returns the current node from the downward API.
func GetNode() string {
	value, ok := os.LookupEnv("KUBECC_NODE")
	if !ok {
		panic("KUBECC_NODE not defined")
	}
	return value
}

func MakeDownwardApi() []v1.EnvVar {
	return []v1.EnvVar{
		{
			Name: "KUBECC_POD_NAME",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: "KUBECC_NAMESPACE",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
		{
			Name: "KUBECC_NODE",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "spec.nodeName",
				},
			},
		},
	}
}

func InCluster() bool {
	host := os.Getenv("KUBERNETES_SERVICE_HOST")
	port := os.Getenv("KUBERNETES_SERVICE_PORT")
	return len(host) != 0 && len(port) != 0
}
