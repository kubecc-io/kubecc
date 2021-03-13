// +kubebuilder:validation:Required
package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BuildClusterSpec struct {
	Components ComponentsSpec `json:"components"`
	Ingress    IngressSpec    `json:"ingress,omitempty"` // +optional
	Tracing    TracingSpec    `json:"tracing,omitempty"` // +optional
}

type ComponentsSpec struct {
	Agent     AgentSpec     `json:"agent"`
	Scheduler SchedulerSpec `json:"scheduler,omitempty"` // +optional
	Monitor   MonitorSpec   `json:"monitor,omitempty"`   // +optional
	Cache     CacheSpec     `json:"cache,omitempty"`     // +optional
}

type IngressSpec struct {
	Kind          string `json:"kind,omitempty"`
	Host          string `json:"host,omitempty"`
	TLSSecretName string `json:"tlsSecretName,omitempty"` // +optional
}

type TracingSpec struct {
	Jaeger JaegerSpec `json:"jaeger,omitempty"` // +optional
}

type JaegerSpec struct {
	Collector CollectorSpec `json:"collector,omitempty"`
	Sampler   SamplerSpec   `json:"sampler,omitempty"`
}

type CollectorSpec struct {
	Endpoint         string `json:"endpoint"`
	InternalEndpoint string `json:"internalEndpoint,omitempty"` // +optional
	User             string `json:"user,omitempty"`             // +optional
	Password         string `json:"password,omitempty"`         // +optional
}

type SamplerSpec struct {
	Server string `json:"server,omitempty"` // +optional
	Type   string `json:"type"`
	Param  string `json:"param,omitempty"` // +optional
}

type AgentSpec struct {
	NodeAffinity *v1.NodeAffinity        `json:"nodeAffinity"`
	Resources    v1.ResourceRequirements `json:"resources,omitempty"` // +optional
	// +kubebuilder:default:="gcr.io/kubecc/agent:latest"
	Image            string            `json:"image"`                      // +optional
	AdditionalLabels map[string]string `json:"additionalLabels,omitempty"` // +optional
	// +kubebuilder:default:=Always
	ImagePullPolicy v1.PullPolicy `json:"imagePullPolicy"` // +optional
}

type SchedulerSpec struct {
	NodeAffinity *v1.NodeAffinity        `json:"nodeAffinity,omitempty"` // +optional
	Resources    v1.ResourceRequirements `json:"resources,omitempty"`    // +optional
	// +kubebuilder:default:="gcr.io/kubecc/scheduler:latest"
	Image            string            `json:"image"`                      // +optional
	AdditionalLabels map[string]string `json:"additionalLabels,omitempty"` // +optional
	// +kubebuilder:default:=Always
	ImagePullPolicy v1.PullPolicy `json:"imagePullPolicy"` // +optional
}

type MonitorSpec struct {
	NodeAffinity *v1.NodeAffinity        `json:"nodeAffinity,omitempty"` // +optional
	Resources    v1.ResourceRequirements `json:"resources,omitempty"`    // +optional
	// +kubebuilder:default:="gcr.io/kubecc/monitor:latest"
	Image            string            `json:"image"`                      // +optional
	AdditionalLabels map[string]string `json:"additionalLabels,omitempty"` // +optional
	// +kubebuilder:default:=Always
	ImagePullPolicy v1.PullPolicy `json:"imagePullPolicy"` // +optional
}

type CacheSpec struct {
	NodeAffinity *v1.NodeAffinity        `json:"nodeAffinity,omitempty"` // +optional
	Resources    v1.ResourceRequirements `json:"resources,omitempty"`    // +optional
	// +kubebuilder:default:="gcr.io/kubecc/monitor:latest"
	Image            string            `json:"image"`                      // +optional
	AdditionalLabels map[string]string `json:"additionalLabels,omitempty"` // +optional
	// +kubebuilder:default:=Always
	ImagePullPolicy v1.PullPolicy `json:"imagePullPolicy"` // +optional
}

type TLSSpec struct {
	// +kubebuilder:validation:MinItems:=1
	Hosts      []string `json:"hosts,omitempty"`
	SecretName string   `json:"secretName,omitempty"`
}

// BuildClusterStatus defines the observed state of BuildCluster.
type BuildClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// BuildCluster is the Schema for the buildclusters API.
type BuildCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BuildClusterSpec   `json:"spec,omitempty"`
	Status BuildClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BuildClusterList contains a list of BuildCluster.
type BuildClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BuildCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BuildCluster{}, &BuildClusterList{})
}
