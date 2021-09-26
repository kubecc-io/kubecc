/*
Copyright 2021 Rancher Labs, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*******************************************************************
 * NOTICE: This file is redistributed from github.com/rancher/opni *
 *******************************************************************/

package util

import (
	"bytes"
	"log"

	"github.com/kubecc-io/kubecc/staging"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	memory "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

// ForEachStagingResource will call the given callback function for each
// Kubernetes resource embedded in the binary. See the staging package for
// more details.
// This function will not abort if the callback returns an error, rather it
// will collect all errors that have been returned and return them all at
// once.
func ForEachStagingResource(
	clientConfig *rest.Config,
	callback func(dynamic.ResourceInterface, *unstructured.Unstructured) error,
) (errors []string) {
	errors = []string{}

	decodingSerializer := yaml.NewDecodingSerializer(
		unstructured.UnstructuredJSONScheme)
	decoder := yamlutil.NewYAMLOrJSONDecoder(
		bytes.NewReader([]byte(staging.StagingAutogenYaml)), 32)
	dynamicClient := dynamic.NewForConfigOrDie(clientConfig)

	dc, err := discovery.NewDiscoveryClientForConfig(clientConfig)
	if err != nil {
		log.Fatal(err)
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(
		memory.NewMemCacheClient(dc))

	for {
		var rawObj runtime.RawExtension
		if err = decoder.Decode(&rawObj); err != nil {
			break
		}

		obj := &unstructured.Unstructured{}
		_, gvk, err := decodingSerializer.Decode(rawObj.Raw, nil, obj)
		if err != nil {
			log.Fatal(err)
		}

		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			errors = append(errors, err.Error())
			continue
		}

		var dr dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			// namespaced resources should specify the namespace
			dr = dynamicClient.Resource(mapping.Resource).
				Namespace(obj.GetNamespace())
		} else {
			// for cluster-wide resources
			dr = dynamicClient.Resource(mapping.Resource)
		}

		if err := callback(dr, obj); err != nil {
			errors = append(errors, err.Error())
		}
	}
	return
}
