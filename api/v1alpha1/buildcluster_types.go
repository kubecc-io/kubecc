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

// +kubebuilder:validation:Optional
package v1alpha1

import (
	"github.com/kubecc-io/kubecc/pkg/config"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BuildClusterSpec struct {
	// Image to use for Kubecc components. Optional, will default to the same
	// image used by the Operator.
	Image           string        `json:"image"`
	ImagePullPolicy v1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// Configuration for the Kubecc components
	Components ComponentsSpec `json:"components"`
	// List of toolchains, each of which will deploy an Agent DaemonSet.
	Toolchains []ToolchainSpec `json:"toolchains"`
	// Whether to deploy the toolbox pod, which will provide a configured
	// environment in which to use the Kubecc CLI.
	DeployToolbox bool         `json:"deployToolbox,omitempty"`
	Tracing       *TracingSpec `json:"tracing,omitempty"`
}

type ToolchainSpec struct {
	// Optional, defaults to "kind-version"
	Name *string `json:"name"`
	// Kind is one of ["gcc", "custom"] (default is "gcc")
	// +kubebuilder:validation:Enum=gcc;custom
	// +kubebuilder:default=gcc
	Kind string `json:"kind,omitempty"`
	// Version is the version of the toolchain image (default is "latest")
	// +kubebuilder:default=latest
	Version string `json:"version,omitempty"`
	// Arch is one of ["amd64", "arm/v5", "arm/v7", "arm64/v8", "ppc64le", "s390x"] (default is "amd64")
	// +kubebuilder:validation:Enum=amd64;arm/v5;arm/v7;arm64/v8;ppc64le;s390x
	// +kubebuilder:default=amd64
	Arch string `json:"arch,omitempty"`
	// CustomImage is the base image for the toolchain (without the tag)
	// If Kind is gcc, this defaults to "docker.io/gcc", but can be overridden.
	// If Kind is custom, this field is required.
	CustomImage *string `json:"image,omitempty"`
}

type ComponentsSpec struct {
	Agent     AgentSpec     `json:"agent,omitempty"`
	Scheduler SchedulerSpec `json:"scheduler,omitempty"`
	Monitor   MonitorSpec   `json:"monitor,omitempty"`
	Cache     CacheSpec     `json:"cache,omitempty"`
}

type TracingSpec struct {
	Jaeger JaegerSpec `json:"jaeger,omitempty"`
}

type JaegerSpec struct {
	Collector CollectorSpec `json:"collector,omitempty"`
	Sampler   SamplerSpec   `json:"sampler,omitempty"`
}

type CollectorSpec struct {
	Endpoint         string `json:"endpoint,omitempty"`
	InternalEndpoint string `json:"internalEndpoint,omitempty"`
	User             string `json:"user,omitempty"`
	Password         string `json:"password,omitempty"`
}

type SamplerSpec struct {
	Server string `json:"server,omitempty"`
	Type   string `json:"type,omitempty"`
	Param  string `json:"param,omitempty"`
}

type AgentSpec struct {
	NodeAffinity    *v1.NodeAffinity        `json:"nodeAffinity,omitempty"`
	ImagePullPolicy v1.PullPolicy           `json:"imagePullPolicy,omitempty"`
	Resources       v1.ResourceRequirements `json:"resources,omitempty"`
}

type SchedulerSpec struct {
	NodeAffinity *v1.NodeAffinity        `json:"nodeAffinity,omitempty"`
	Resources    v1.ResourceRequirements `json:"resources,omitempty"`
}

type MonitorSpec struct {
	NodeAffinity *v1.NodeAffinity        `json:"nodeAffinity,omitempty"`
	Resources    v1.ResourceRequirements `json:"resources,omitempty"`
}

type CacheSpec struct {
	Enabled         bool                        `json:"enabled,omitempty"`
	NodeAffinity    *v1.NodeAffinity            `json:"nodeAffinity,omitempty"`
	Resources       v1.ResourceRequirements     `json:"resources,omitempty"`
	VolatileStorage *config.VolatileStorageSpec `json:"volatileStorage,omitempty"`
	LocalStorage    *config.LocalStorageSpec    `json:"localStorage,omitempty"`
	RemoteStorage   *config.RemoteStorageSpec   `json:"remoteStorage,omitempty"`
}

// BuildClusterStatus defines the observed state of BuildCluster.
type BuildClusterStatus struct {
	DefaultImageName string `json:"defaultImageName,omitempty"`
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
