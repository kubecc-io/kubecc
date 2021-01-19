package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BuildClusterSpec struct {
	Components ComponentsSpec `json:"components"`
	Ingress    IngressSpec    `json:"ingress"`
	Tracing    TracingSpec    `json:"tracing"`
}

type ComponentsSpec struct {
	Agent     AgentSpec     `json:"agent"`
	Scheduler SchedulerSpec `json:"scheduler"`
}

type IngressSpec struct {
	Kind string  `json:"kind"`
	TLS  TLSSpec `json:"tls"`
}

type TracingSpec struct {
	Jaeger JaegerSpec `json:"jaeger"`
}

type JaegerSpec struct {
	Collector CollectorSpec `json:"collector"`
	Sampler   SamplerSpec   `json:"sampler"`
}

type CollectorSpec struct {
	Endpoint         string `json:"endpoint"`
	InternalEndpoint string `json:"internalEndpoint"`
	User             string `json:"user"`
	Password         string `json:"password"`
}

type SamplerSpec struct {
	Server string `json:"server"`
	Type   string `json:"type"`
	Param  string `json:"param"`
}

type AgentSpec struct {
	Placement        *v1.NodeSelector         `json:"placement"`
	Resources        *v1.ResourceRequirements `json:"resources"`
	Image            string                   `json:"image"`
	AdditionalLabels map[string]string        `json:"additionalLabels"`
	LogLevel         string                   `json:"logLevel"`
	ImagePullPolicy  string                   `json:"imagePullPolicy"`
}

type SchedulerSpec struct {
	Placement        *v1.NodeSelector         `json:"placement"`
	Resources        *v1.ResourceRequirements `json:"resources"`
	Image            string                   `json:"image"`
	AdditionalLabels map[string]string        `json:"additionalLabels"`
	LogLevel         string                   `json:"logLevel"`
	ImagePullPolicy  string                   `json:"imagePullPolicy"`
}

type TLSSpec struct {
	Hosts      []string `json:"hosts"`
	SecretName string   `json:"secretName"`
}

// BuildClusterStatus defines the observed state of BuildCluster
type BuildClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// BuildCluster is the Schema for the buildclusters API
type BuildCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BuildClusterSpec   `json:"spec,omitempty"`
	Status BuildClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BuildClusterList contains a list of BuildCluster
type BuildClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BuildCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BuildCluster{}, &BuildClusterList{})
}
