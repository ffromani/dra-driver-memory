/*
Copyright 2025 The Kubernetes Authors.

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

package e2e

import (
	"context"
	"os"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	resourcev1 "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/ffromani/dra-driver-memory/test/pkg/fixture"
	"github.com/ffromani/dra-driver-memory/test/pkg/node"
	"github.com/ffromani/dra-driver-memory/test/pkg/pod"
)

var _ = ginkgo.Describe("Memory Allocation", ginkgo.Serial, ginkgo.Ordered, ginkgo.ContinueOnFailure, ginkgo.Label("tier0", "memory", "allocation", "platform:kind"), func() {
	var rootFxt *fixture.Fixture
	var targetNode *corev1.Node
	var dramemoryTesterImage string

	ginkgo.BeforeAll(func(ctx context.Context) {
		// early cheap check before to create the Fixture, so we use GinkgoLogr directly
		dramemoryTesterImage = os.Getenv("DRAMEM_E2E_TEST_IMAGE")
		gomega.Expect(dramemoryTesterImage).ToNot(gomega.BeEmpty(), "missing environment variable DRAMEM_E2E_TEST_IMAGE")
		ginkgo.GinkgoLogr.Info("discovery image", "pullSpec", dramemoryTesterImage)

		var err error

		rootFxt, err = fixture.ForGinkgo()
		gomega.Expect(err).ToNot(gomega.HaveOccurred(), "cannot create root fixture: %v", err)
		infraFxt := rootFxt.WithPrefix("infra")
		gomega.Expect(infraFxt.Setup(ctx)).To(gomega.Succeed())
		ginkgo.DeferCleanup(infraFxt.Teardown)

		if targetNodeName := os.Getenv("DRAMEM_E2E_TARGET_NODE"); len(targetNodeName) > 0 {
			targetNode, err = rootFxt.K8SClientset.CoreV1().Nodes().Get(ctx, targetNodeName, metav1.GetOptions{})
			gomega.Expect(err).ToNot(gomega.HaveOccurred(), "cannot get worker node %q: %v", targetNodeName, err)
		} else {
			workerNodes, err := node.FindWorkers(ctx, infraFxt.K8SClientset)
			gomega.Expect(err).ToNot(gomega.HaveOccurred(), "cannot find worker nodes: %v", err)
			gomega.Expect(workerNodes).ToNot(gomega.BeEmpty(), "no worker nodes detected")
			targetNode = workerNodes[0] // pick random one, this is the simplest random pick
		}
		rootFxt.Log.Info("using worker node", "nodeName", targetNode.Name)
	})

	ginkgo.When("requesting memory", func() {
		var fxt *fixture.Fixture

		ginkgo.BeforeEach(func(ctx context.Context) {
			fxt = rootFxt.WithPrefix("allocmem")
			gomega.Expect(fxt.Setup(ctx)).To(gomega.Succeed())
		})

		ginkgo.AfterEach(func(ctx context.Context) {
			gomega.Expect(fxt.Teardown(ctx)).To(gomega.Succeed())
		})

		ginkgo.It("should run a pod with a single resourceclaimtemplate", func(ctx context.Context) {
			fixture.By("creating a ResourceClaimTemplate on %q", fxt.Namespace.Name)
			claimTmpl := resourcev1.ResourceClaimTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: fxt.Namespace.Name,
					Name:      "memory-256m",
				},
				Spec: resourcev1.ResourceClaimTemplateSpec{
					Spec: resourcev1.ResourceClaimSpec{
						Devices: resourcev1.DeviceClaim{
							Requests: []resourcev1.DeviceRequest{
								{
									Name: "mem",
									Exactly: &resourcev1.ExactDeviceRequest{
										DeviceClassName: "dra.memory",
										Capacity: &resourcev1.CapacityRequirements{
											Requests: map[resourcev1.QualifiedName]resource.Quantity{
												resourcev1.QualifiedName("size"): *resource.NewQuantity(256*1<<20, resource.BinarySI),
											},
										},
									},
								},
							},
						},
					},
				},
			}

			createdTmpl, err := fxt.K8SClientset.ResourceV1().ResourceClaimTemplates(fxt.Namespace.Name).Create(ctx, &claimTmpl, metav1.CreateOptions{})
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(createdTmpl).ToNot(gomega.BeNil())

			fixture.By("creating a pod consuming the ResourceClaimTemplate on %q", fxt.Namespace.Name)
			testPod := corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: fxt.Namespace.Name,
					Name:      "pod-with-memory",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "container-with-memory",
							Image:   "registry.fedoraproject.org/fedora-minimal:42",
							Command: []string{"/bin/sleep", "inf"},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    *resource.NewQuantity(1, resource.DecimalSI),
									corev1.ResourceMemory: *resource.NewQuantity(512*(1<<20), resource.BinarySI),
								},
								Claims: []corev1.ResourceClaim{
									{
										Name: "mem",
									},
								},
							},
						},
					},
					ResourceClaims: []corev1.PodResourceClaim{
						{
							Name:                      "mem",
							ResourceClaimTemplateName: ptr.To("memory-256m"),
						},
					},
				},
			}

			createdPod, err := pod.CreateSync(ctx, fxt.K8SClientset, &testPod)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(createdPod).ToNot(gomega.BeNil())
		})
	})
})
