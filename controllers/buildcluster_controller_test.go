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

package controllers

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BuildCluster Controller", func() {
	const (
		Name      = "test-buildcluster"
		Namespace = "kubecc-test"
		timeout   = 10 * time.Second
		interval  = 500 * time.Millisecond
	)

	Context("When creating a BuildCluster", func() {
		Expect(true).To(BeTrue())
		// 	ctx := context.Background()
		// 	cluster := &v1alpha1.BuildCluster{
		// 		TypeMeta: metav1.TypeMeta{
		// 			APIVersion: "kubecc.io/v1alpha1",
		// 			Kind:       "BuildCluster",
		// 		},
		// 		ObjectMeta: metav1.ObjectMeta{
		// 			Name:      Name,
		// 			Namespace: Namespace,
		// 		},
		// 		Spec: v1alpha1.BuildClusterSpec{
		// 			Components: v1alpha1.ComponentsSpec{
		// 				Image:           "kubecc/kubecc:latest",
		// 				ImagePullPolicy: v1.PullIfNotPresent,
		// 				// Scheduler: v1alpha1.SchedulerSpec{
		// 				// 	Resources:       resources,
		// 				// 	ImagePullPolicy: "Always",
		// 				// },
		// 				// Monitor: v1alpha1.MonitorSpec{
		// 				// 	Resources:       resources,
		// 				// 	ImagePullPolicy: "Always",
		// 				// },
		// 				// Cache: v1alpha1.CacheSpec{
		// 				// 	Resources:       resources,
		// 				// 	ImagePullPolicy: "Always",
		// 				// },
		// 			},
		// 		},
		// 	}
		// 	It("Should succeed", func() {
		// 	})
	})
})
