package cluster

import (
	"os"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

// GetPodName returns the current pod name from the downward API
func GetPodName() string {
	value, ok := os.LookupEnv("KUBECC_POD_NAME")
	if !ok {
		log.Fatal("KUBECC_POD_NAME not defined")
	}
	return value
}

// GetNamespace returns the current namespace from the downward API
func GetNamespace() string {
	value, ok := os.LookupEnv("KUBECC_NAMESPACE")
	if !ok {
		log.Fatal("KUBECC_NAMESPACE not defined")
	}
	return value
}

// GetNode returns the current node from the downward API
func GetNode() string {
	value, ok := os.LookupEnv("KUBECC_NODE")
	if !ok {
		log.Fatal("KUBECC_NODE not defined")
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
	}
}
