package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ToolchainSpec defines the desired state of Toolchain
type ToolchainSpec struct {
	Kind     string `json:"kind"`
	Triple   string `json:"triple"`
	Host     string `json:"host"`
	Target   string `json:"target"`
	Location string `json:"location,omitempty"`
}

// ToolchainStatus defines the observed state of Toolchain
type ToolchainStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Toolchain is the Schema for the toolchains API
type Toolchain struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ToolchainSpec   `json:"spec,omitempty"`
	Status ToolchainStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ToolchainList contains a list of Toolchain
type ToolchainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Toolchain `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Toolchain{}, &ToolchainList{})
}
