/*
Copyright 2026 The Kubernetes Authors.

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
	"fmt"
	"os"
	"sync"

	"github.com/go-logr/logr"
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

const (
	maxPodsPerNode = 200 // plenty for our needs
)

var _ = ginkgo.Describe("Claim scalability", ginkgo.Serial, ginkgo.Ordered, ginkgo.ContinueOnFailure, ginkgo.Label("tier1", "memory", "allocation", "scaling", "platform:kind"), func() {
	var rootFxt *fixture.Fixture
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
	})

	ginkgo.When("using many claims as possible", func() {
		var fxt *fixture.Fixture

		ginkgo.BeforeEach(func(ctx context.Context) {
			fxt = rootFxt.WithPrefix("allocmem")
			gomega.Expect(fxt.Setup(ctx)).To(gomega.Succeed())
		})

		ginkgo.AfterEach(func(ctx context.Context) {
			gomega.Expect(fxt.Teardown(ctx)).To(gomega.Succeed())
		})

		ginkgo.It("should run successfully all the pods", func(ctx context.Context) {
			var podCount int
			var targetNode *corev1.Node

			targetMemQty := *resource.NewQuantity(256*(1<<20), resource.BinarySI)
			targetResources := corev1.ResourceList{
				corev1.ResourceCPU:    *resource.NewQuantity(1, resource.DecimalSI),
				corev1.ResourceMemory: targetMemQty,
			}

			if targetNodeName := os.Getenv("DRAMEM_E2E_TARGET_NODE"); len(targetNodeName) > 0 {
				var err error
				targetNode, err = rootFxt.K8SClientset.CoreV1().Nodes().Get(ctx, targetNodeName, metav1.GetOptions{})
				gomega.Expect(err).ToNot(gomega.HaveOccurred(), "cannot get worker node %q: %v", targetNodeName, err)
				podCount = computeMaxPodCount(fxt.Log, targetNode, targetResources)
			} else {
				workerNodes, err := node.FindWorkers(ctx, fxt.K8SClientset)
				gomega.Expect(err).ToNot(gomega.HaveOccurred(), "cannot find worker nodes: %v", err)
				gomega.Expect(workerNodes).ToNot(gomega.BeEmpty(), "no worker nodes detected")

				workerNodes, maxPodCountPerNode := filterNodesWithEnoughResources(fxt.Log, workerNodes, targetResources)
				if len(workerNodes) == 0 {
					fixture.Skip("no suitable worker nodes for resources=%v", targetResources)
				}

				targetNode = workerNodes[0] // pick random one, this is the simplest random pick
				podCount = maxPodCountPerNode[targetNode.Name]
			}
			rootFxt.Log.Info("using worker node", "node", targetNode.Name, "podCount", podCount)

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
												resourcev1.QualifiedName("size"): targetMemQty,
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
			podTmpl := corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: fxt.Namespace.Name,
					Name:      "pod-with-memory",
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "container-with-memory",
							Image:   dramemoryTesterImage,
							Command: []string{"/bin/dramemtester"},
							Args:    []string{"-use-hugetlb=false", "-alloc-size=128Mi", "-numa-align=single", "-run-forever"}, // keep a safe margin
							Resources: corev1.ResourceRequirements{
								Limits: targetResources,
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
							ResourceClaimTemplateName: ptr.To(createdTmpl.Name),
						},
					},
				},
			}

			var wg sync.WaitGroup
			for idx := 0; idx < podCount; idx++ {
				wg.Add(1)
				go func(testPod *corev1.Pod, idx int, fxt *fixture.Fixture) {
					defer ginkgo.GinkgoRecover()
					defer wg.Done()
					fixture.By("creating a test pod: %02d/%02d", idx+1, podCount)
					testPod.Name = fmt.Sprintf("%s-%02d", testPod.Name, idx)

					createdPod, err := pod.CreateSync(ctx, fxt.K8SClientset, testPod)
					gomega.Expect(err).ToNot(gomega.HaveOccurred())
					gomega.Expect(createdPod).ToNot(gomega.BeNil())
					gomega.Expect(createdPod).To(ReportReason(fxt, result.Succeeded))
				}(podTmpl.DeepCopy(), idx, fxt)
			}
			wg.Wait()
		})
	})
})

func filterNodesWithEnoughResources(lh logr.Logger, nodes []*corev1.Node, targetResources corev1.ResourceList) ([]*corev1.Node, map[string]int) {
	maxPodsPerNode := make(map[string]int)
	allowedNodes := []*corev1.Node{}

	for _, node := range nodes {
		podCount := computeMaxPodCount(lh, node, targetResources)
		if podCount == 0 {
			lh.Info("node can't run pods", "node", node.Name)
			continue
		}
		allowedNodes = append(allowedNodes, node)
		maxPodsPerNode[node.Name] = podCount
		lh.Info("node allowed", "node", node.Name, "podCount", podCount)
	}

	return allowedNodes, maxPodsPerNode
}

func computeMaxPodCount(lh logr.Logger, node *corev1.Node, targetResources corev1.ResourceList) int {
	if node == nil {
		return 0
	}

	lh = lh.WithValues("node", node.Name)

	podCount := maxPodsPerNode
	qty, ok := node.Status.Allocatable[corev1.ResourceName("pods")]
	if ok {
		val, ok := qty.AsInt64()
		if ok {
			podCount = int(val)
			lh.Info("pod count from per-node pod limits", "podCount", podCount)
		}
	}

	for resName := range targetResources {
		if resName != corev1.ResourceCPU && resName != corev1.ResourceMemory {
			lh.Info("node has not memory nor CPU resource")
			continue
		}
		val, ok := computeMaxPodCountPerResource(lh, node.Status.Allocatable, targetResources, resName)
		if !ok {
			lh.Info("can't fetch resources", "resource", resName)
			continue
		}
		if val < podCount {
			podCount = val
			lh.Info("pod count from resource value", "resource", resName, "podCount", podCount)
		}
	}

	lh.Info("computed pod count", "podCount", podCount)
	return podCount
}

func computeMaxPodCountPerResource(lh logr.Logger, nodeResources, targetResources corev1.ResourceList, resName corev1.ResourceName) (int, bool) {
	nodeVal, nodeOK := fetchResource(nodeResources, resName)
	podVal, podOK := fetchResource(targetResources, resName)
	if !nodeOK || !podOK {
		lh.Info("resource mismatch", "nodeOK", nodeOK, "podOK", podOK)
		return 0, false
	}
	podCount := int(float64(nodeVal) / float64(podVal))
	lh.Info("result", "resource", resName, "node", nodeVal, "pod", podVal, "podCount", podCount)
	// leave some buffer for the shared pool
	if podCount <= 1 {
		return 0, false
	}
	return podCount - 1, true
}

func fetchResource(ress corev1.ResourceList, resName corev1.ResourceName) (int64, bool) {
	resQty, ok := ress[resName]
	if !ok {
		return 0, false
	}
	resVal, ok := resQty.AsInt64()
	if !ok {
		return 0, false
	}
	return resVal, true
}
