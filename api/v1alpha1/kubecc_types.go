package v1alpha1

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

// KubeccSpec defines the desired state of Kubecc
type KubeccSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Nodes      NodeConfig `json:"nodes"`
	AgentImage string     `json:"agentImage"`
	MgrImage   string     `json:"mgrImage"`

	Hostname string `json:"hostname"`
}

// KubeccStatus defines the observed state of Kubecc
type KubeccStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Kubecc is the Schema for the kubeccs API
type Kubecc struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubeccSpec   `json:"spec,omitempty"`
	Status KubeccStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KubeccList contains a list of Kubecc
type KubeccList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Kubecc `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Kubecc{}, &KubeccList{})
}
