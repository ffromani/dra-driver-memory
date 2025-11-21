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
	"fmt"
	"os"
	"os/exec"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ffromani/dra-driver-memory/test/pkg/fixture"
	"github.com/ffromani/dra-driver-memory/test/pkg/node"
)

var _ = ginkgo.Describe("Machine Setup", ginkgo.Serial, ginkgo.Ordered, ginkgo.ContinueOnFailure, ginkgo.Label("setup"), func() {
	var rootFxt *fixture.Fixture
	var targetNode *v1.Node
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

	ginkgo.When("running on kind", ginkgo.Label("platform:kind"), func() {
		var fxt *fixture.Fixture

		ginkgo.BeforeEach(func(ctx context.Context) {
			fxt = rootFxt.WithPrefix("kind")
			gomega.Expect(fxt.Setup(ctx)).To(gomega.Succeed())
		})

		ginkgo.AfterEach(func(ctx context.Context) {
			gomega.Expect(fxt.Teardown(ctx)).To(gomega.Succeed())
		})

		ginkgo.It("should configure HugeTLB support", func(ctx context.Context) {
			fixture.By("getting the containerd configuration for target node %q", targetNode.Name)
			// the awk part combines grep and tail. We store all the entries, we emit the last stored once `awk` ends.
			// note we sneak in another optimization: we only return the `config={...}` portion column 11. A line would look like:
			// columns:
			// 1   2  3        4                        5                6                                     7          8             9   10      11
			// Nov 17 13:03:33 dra-driver-memory-worker containerd[112]: time="2025-11-17T13:03:33.453648941Z" level=info msg="starting cri plugin" config="{\"containerd\":{ ...
			cmdline := fmt.Sprintf("docker exec %s journalctl -u containerd | awk '/starting cri plugin/ { CONF=$11 } END { print CONF }'", targetNode.Name)
			fxt.Log.Info("about to run", "commandLine", cmdline)

			cmd := exec.CommandContext(ctx, "/bin/bash", "-c", cmdline)
			out, err := cmd.Output()
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					fxt.Log.Info("command failed", "stderr", string(exitErr.Stderr))
				}
			}
			gomega.Expect(err).ToNot(gomega.HaveOccurred(), "error getting the containerd raw configuration from target node")
			rawConf := string(out)
			fxt.Log.Info("found raw configuration", "rawConf", rawConf)

			gomega.Expect(rawConf).ToNot(gomega.BeEmpty(), "the raw configuration is empty")
			gomega.Expect(rawConf).To(gomega.ContainSubstring(`\"disableHugetlbController\":false`))
			gomega.Expect(rawConf).To(gomega.ContainSubstring(`\"tolerateMissingHugetlbController\":false`))
		})
	})
})
