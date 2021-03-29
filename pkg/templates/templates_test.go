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

package templates_test

import (
	"embed"
	"os"
	"strings"
	"testing"

	"github.com/kubecc-io/kubecc/pkg/templates"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
)

//go:embed test
var testFS embed.FS

var one = int32(1)
var sampleDeployment = &appsv1.Deployment{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "test",
		Labels: map[string]string{
			"app": "test",
		},
	},
	Spec: appsv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "test",
			},
		},
		Replicas: &one,
		Template: v1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app": "test",
				},
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:            "test",
						Image:           "test",
						ImagePullPolicy: v1.PullIfNotPresent,
						Ports: []v1.ContainerPort{
							{
								ContainerPort: 12345,
							},
						},
						Resources: v1.ResourceRequirements{
							Limits: v1.ResourceList{
								v1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
					},
				},
			},
		},
	},
}

func sanitize(b []byte) string {
	return strings.TrimSpace(string(b))
}

func TestTemplates(t *testing.T) {
	RegisterFailHandler(Fail)
	format.TruncatedDiff = false
	RunSpecsWithDefaultAndCustomReporters(t,
		"Templates",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = Describe("Template Parser", func() {
	Context("when parsing a template", func() {
		Context("and the file does not exist", func() {
			It("should error", func() {
				_, err := templates.Load(testFS, "does_not_exist.yaml", struct{}{})
				Expect(err).To(HaveOccurred())
			})
		})
		Context("without spec substitutions", func() {
			It("should load the exact file contents", func() {
				data, err := os.ReadFile("./test/deployment_nospec.yaml")
				Expect(err).NotTo(HaveOccurred())
				Expect(templates.Load(testFS, "test/deployment_nospec.yaml", struct{}{})).To(Equal(data))
			})
			It("should unmarshal fields into Kubernetes objects", func() {
				data, err := templates.Load(testFS, "test/deployment_nospec.yaml", struct{}{})
				Expect(err).NotTo(HaveOccurred())
				d, err := templates.Unmarshal(data, clientgoscheme.Scheme)
				Expect(err).NotTo(HaveOccurred())
				deployment, ok := d.(*appsv1.Deployment)
				Expect(ok).To(BeTrue())
				Expect(deployment).To(Equal(sampleDeployment))
			})
		})
		Context("with spec substitutions", func() {
			It("should substitute simple data types", func() {
				spec := struct {
					String string
					Int    int
					Float  float32
				}{
					String: "testing",
					Int:    123,
					Float:  12.3,
				}
				data, err := os.ReadFile("./test/simple_expected.yaml")
				Expect(err).NotTo(HaveOccurred())
				Expect(templates.Load(testFS, "test/simple.yaml", spec)).
					To(WithTransform(sanitize, Equal(sanitize(data))))
			})
			It("should substitute multiline strings", func() {
				By("manual indentation")
				spec := struct {
					MultilineString0 string
					MultilineString2 string
					MultilineString4 string
					MultilineString6 string
				}{
					MultilineString0: `line 1
line 2
line 3`,
					MultilineString2: `  line 1
  line 2
  line 3`,
					MultilineString4: `    line 1
    line 2
    line 3`,
					MultilineString6: `      line 1
      line 2
      line 3`,
				}
				data, err := os.ReadFile("./test/multiline_expected.yaml")
				Expect(err).NotTo(HaveOccurred())
				Expect(templates.Load(testFS, "test/multiline_manual.yaml", spec)).
					To(WithTransform(sanitize, Equal(sanitize(data))))
				By("using the indent function")
				Expect(templates.Load(testFS, "test/multiline_indent.yaml", spec)).
					To(WithTransform(sanitize, Equal(sanitize(data))))
			})
			It("should convert spec fields to YAML", func() {
				By("using the toYaml function")
				type structField struct {
					StrTest         string       `json:"strTest,omitempty"`
					NumTest         int          `json:"numTest,omitempty"`
					StringSliceTest []string     `json:"stringSliceTest"`
					NestedStruct    *structField `json:"nestedStruct"`
				}
				spec := structField{
					StrTest:         "test",
					NumTest:         5,
					StringSliceTest: []string{"a", "b", "c"},
					NestedStruct: &structField{
						NumTest: 6,
						NestedStruct: &structField{
							StrTest: "test2",
							StringSliceTest: []string{`line 1
line 2
line 3`, "test"},
						},
					},
				}
				data, err := os.ReadFile("./test/toyaml_expected.yaml")
				Expect(err).NotTo(HaveOccurred())
				var expected, actual structField
				Expect(yaml.Unmarshal(data, &expected)).To(Succeed())
				Expect(expected).To(Equal(spec))
				tmplData, err := templates.Load(testFS, "test/toyaml.yaml", spec)
				Expect(err).NotTo(HaveOccurred())
				Expect(yaml.Unmarshal(tmplData, &actual)).To(Succeed())
				Expect(actual).To(Equal(spec))
			})
		})
		It("should unmarshal fields into Kubernetes objects", func() {
			data, err := templates.Load(testFS, "test/deployment_spec.yaml", struct {
				Name       string
				Labels     map[string]string
				PullPolicy v1.PullPolicy
				Ports      []v1.ContainerPort
				Resources  v1.ResourceRequirements
			}{
				Name:       sampleDeployment.Name,
				Labels:     sampleDeployment.Labels,
				PullPolicy: sampleDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy,
				Ports:      sampleDeployment.Spec.Template.Spec.Containers[0].Ports,
				Resources:  sampleDeployment.Spec.Template.Spec.Containers[0].Resources,
			})
			Expect(err).NotTo(HaveOccurred())
			d, err := templates.Unmarshal(data, clientgoscheme.Scheme)
			Expect(err).NotTo(HaveOccurred())
			deployment, ok := d.(*appsv1.Deployment)
			Expect(ok).To(BeTrue())
			Expect(deployment).To(Equal(sampleDeployment))
		})
	})
})
