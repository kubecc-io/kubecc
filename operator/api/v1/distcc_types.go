package v1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type NodeConfig struct {
	NodeAffinity v1.NodeAffinity         `json:"nodeAffinity"`
	Resources    v1.ResourceRequirements `json:"resources"`
}

// DistccSpec defines the desired state of Distcc
type DistccSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Nodes   NodeConfig         `json:"nodes"`
	Image   string             `json:"image"`
	Command []string           `json:"command"`
	Ports   []v1.ContainerPort `json:"ports"`

	// +kubebuilder:validation:Enum:=traefik
	IngressStrategy string `json:"ingressStrategy"`
	TLSStrategy     string `json:"tlsStrategy"`
	Hostname        string `json:"hostname"`
}

// DistccStatus defines the observed state of Distcc
type DistccStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Distcc is the Schema for the distccs API
type Distcc struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DistccSpec   `json:"spec,omitempty"`
	Status DistccStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DistccList contains a list of Distcc
type DistccList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Distcc `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Distcc{}, &DistccList{})
}
