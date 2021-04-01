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

// +kubebuilder:validation:Required
package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BuildClusterSpec struct {
	Components ComponentsSpec `json:"components"`
	Tracing    TracingSpec    `json:"tracing,omitempty"` // +optional
}

type ComponentsSpec struct {
	Image           string        `json:"image"`
	ImagePullPolicy v1.PullPolicy `json:"imagePullPolicy,omitempty"` // +optional
	Agents          AgentSpec     `json:"agents"`
	Scheduler       SchedulerSpec `json:"scheduler,omitempty"` // +optional
	Monitor         MonitorSpec   `json:"monitor,omitempty"`   // +optional
	Cache           CacheSpec     `json:"cache,omitempty"`     // +optional
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
	NodeAffinity     *v1.NodeAffinity        `json:"nodeAffinity"`
	Image            string                  `json:"image"`
	ImagePullPolicy  v1.PullPolicy           `json:"imagePullPolicy,omitempty"`  // +optional
	Resources        v1.ResourceRequirements `json:"resources,omitempty"`        // +optional
	AdditionalLabels map[string]string       `json:"additionalLabels,omitempty"` // +optional
}

type SchedulerSpec struct {
	NodeAffinity     *v1.NodeAffinity        `json:"nodeAffinity,omitempty"`     // +optional
	Resources        v1.ResourceRequirements `json:"resources,omitempty"`        // +optional
	AdditionalLabels map[string]string       `json:"additionalLabels,omitempty"` // +optional
}

type MonitorSpec struct {
	NodeAffinity     *v1.NodeAffinity        `json:"nodeAffinity,omitempty"`     // +optional
	Resources        v1.ResourceRequirements `json:"resources,omitempty"`        // +optional
	AdditionalLabels map[string]string       `json:"additionalLabels,omitempty"` // +optional
}

type CacheSpec struct {
	NodeAffinity     *v1.NodeAffinity        `json:"nodeAffinity,omitempty"`     // +optional
	Resources        v1.ResourceRequirements `json:"resources,omitempty"`        // +optional
	AdditionalLabels map[string]string       `json:"additionalLabels,omitempty"` // +optional
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
