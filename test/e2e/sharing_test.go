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
	"time"

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
	"github.com/ffromani/dra-driver-memory/test/pkg/result"
)

var _ = ginkgo.Describe("Claim sharing", ginkgo.Serial, ginkgo.Ordered, ginkgo.ContinueOnFailure, ginkgo.Label("tier0", "memory", "sharing", "platform:kind"), func() {
	var rootFxt *fixture.Fixture
	var targetNode *corev1.Node
	var dramemoryTesterImage string

	ginkgo.BeforeAll(func(ctx context.Context) {
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

	ginkgo.When("sharing a memory claim", func() {
		var fxt *fixture.Fixture
		var claim *resourcev1.ResourceClaim

		ginkgo.BeforeEach(func(ctx context.Context) {
			fxt = rootFxt.WithPrefix("sharingmem")
			gomega.Expect(fxt.Setup(ctx)).To(gomega.Succeed())

			fixture.By("creating a memory ResourceClaim on %q", fxt.Namespace.Name)
			memClaim := resourcev1.ResourceClaim{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: fxt.Namespace.Name,
					Name:      "claim-memory-512m",
				},
				Spec: resourcev1.ResourceClaimSpec{
					Devices: resourcev1.DeviceClaim{
						Requests: []resourcev1.DeviceRequest{
							{
								Name: "mem",
								Exactly: &resourcev1.ExactDeviceRequest{
									DeviceClassName: "dra.memory",
									Capacity: &resourcev1.CapacityRequirements{
										Requests: map[resourcev1.QualifiedName]resource.Quantity{
											resourcev1.QualifiedName("size"): *resource.NewQuantity(512*(1<<20), resource.BinarySI),
										},
									},
								},
							},
						},
					},
				},
			}

			var err error
			claim, err = fxt.K8SClientset.ResourceV1().ResourceClaims(fxt.Namespace.Name).Create(ctx, &memClaim, metav1.CreateOptions{})
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(claim).ToNot(gomega.BeNil())
		})

		ginkgo.AfterEach(func(ctx context.Context) {
			gomega.Expect(fxt.Teardown(ctx)).To(gomega.Succeed())
		})

		ginkgo.It("should fail to run pods which share a claim", ginkgo.Label("negative"), func(ctx context.Context) {
			fixture.By("creating a pod consuming the ResourceClaim on %q", fxt.Namespace.Name)
			testPod := corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:    fxt.Namespace.Name,
					GenerateName: "pod-with-memory-claim-",
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "container-with-memory-1",
							Image:   dramemoryTesterImage,
							Command: []string{"/bin/dramemtester"},
							Args:    []string{"-use-hugetlb=false", "-alloc-size=480Mi", "-numa-align=single", "-run-forever"}, // keep a safe margin
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
							Name:              "mem",
							ResourceClaimName: ptr.To(claim.Name),
						},
					},
				},
			}

			createdPod1, err := pod.CreateSync(ctx, fxt.K8SClientset, &testPod)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(createdPod1).ToNot(gomega.BeNil())
			gomega.Expect(createdPod1).To(ReportReason(fxt, result.Succeeded))

			createdPod2, err := fxt.K8SClientset.CoreV1().Pods(testPod.Namespace).Create(ctx, &testPod, metav1.CreateOptions{})
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Eventually(func() *corev1.Pod {
				pod, err := fxt.K8SClientset.CoreV1().Pods(createdPod2.Namespace).Get(ctx, createdPod2.Name, metav1.GetOptions{})
				if err != nil {
					return nil
				}
				return pod
			}).WithTimeout(time.Minute).WithPolling(2 * time.Second).Should(BeFailedToCreate(fxt))
		})

		ginkgo.It("should fail to run a pod with multiple containers which share a claim", ginkgo.Label("negative"), func(ctx context.Context) {
			fixture.By("creating a pod with multiple containers consuming the ResourceClaim on %q", fxt.Namespace.Name)
			testPod := corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: fxt.Namespace.Name,
					Name:      "pod-with-memory-claim-multicnt",
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "container-with-memory-1",
							Image:   dramemoryTesterImage,
							Command: []string{"/bin/dramemtester"},
							Args:    []string{"-use-hugetlb=false", "-alloc-size=240Mi", "-numa-align=single", "-run-forever"}, // keep a safe margin
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    *resource.NewQuantity(1, resource.DecimalSI),
									corev1.ResourceMemory: *resource.NewQuantity(256*(1<<20), resource.BinarySI),
								},
								Claims: []corev1.ResourceClaim{
									{
										Name: "mem",
									},
								},
							},
						},
						{
							Name:    "container-with-memory-2",
							Image:   dramemoryTesterImage,
							Command: []string{"/bin/dramemtester"},
							Args:    []string{"-use-hugetlb=false", "-alloc-size=240Mi", "-numa-align=single", "-run-forever"}, // keep a safe margin
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    *resource.NewQuantity(1, resource.DecimalSI),
									corev1.ResourceMemory: *resource.NewQuantity(256*(1<<20), resource.BinarySI),
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
							Name:              "mem",
							ResourceClaimName: ptr.To(claim.Name),
						},
					},
				},
			}

			createdPod, err := fxt.K8SClientset.CoreV1().Pods(testPod.Namespace).Create(ctx, &testPod, metav1.CreateOptions{})
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Eventually(func() *corev1.Pod {
				pod, err := fxt.K8SClientset.CoreV1().Pods(createdPod.Namespace).Get(ctx, createdPod.Name, metav1.GetOptions{})
				if err != nil {
					return nil
				}
				return pod
			}).WithTimeout(time.Minute).WithPolling(2 * time.Second).Should(BeFailedToCreate(fxt))
		})
	})
})
