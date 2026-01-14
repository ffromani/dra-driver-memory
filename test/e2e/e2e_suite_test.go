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
	"errors"
	"os"
	"strconv"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gcustom"
	"github.com/onsi/gomega/types"

	corev1 "k8s.io/api/core/v1"

	"github.com/ffromani/dra-driver-memory/test/pkg/fixture"
	"github.com/ffromani/dra-driver-memory/test/pkg/pod"
	"github.com/ffromani/dra-driver-memory/test/pkg/result"
)

func TestE2E(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "DRA Memory Driver E2E Suite")
}

/*
explanation of the gingko flags we use in the suites:

- Serial:
because the tests want to change the memory allocation, which is a giant blob of node shared state.
- Ordered:
to do the relatively costly initial resource discovery on the target node only once
- ContinueOnFailure
to mitigate the problem that ordered suites stop on the first failure, so an initial failure can mask
a cascade of latter failure; this makes the tests failure troubleshooting painful, as we would need
to fix failures one by one vs in batches.

Note that using "Ordered" may introduce subtle bugs caused by incorrect tests which pollute or leak
state. We should keep looking for ways to eventually remove "Ordered".
Please note "Serial" is however unavoidable because we manage the shared node state.
*/

// add custom matchers and generic test helpers which we didn't promote yet into `test/pkg` here

func SkipIfGithubActions() {
	val, ok := os.LookupEnv("GITHUB_ACTIONS")
	if !ok {
		return
	}
	isGHA, err := strconv.ParseBool(val)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	if !isGHA {
		return
	}
	fixture.Skip("Github Actions detected: skip flaky/fragile tests")
}

func ReportReason(fxt *fixture.Fixture, reason result.Reason) types.GomegaMatcher {
	return gcustom.MakeMatcher(func(actual *corev1.Pod) (bool, error) {
		if actual == nil {
			return false, errors.New("nil Pod")
		}
		lh := fxt.Log.WithValues("podUID", actual.UID, "namespace", actual.Namespace, "name", actual.Name)
		ctx := context.TODO()
		logs, err := pod.GetLogs(fxt.K8SClientset, ctx, actual.Namespace, actual.Name, actual.Spec.Containers[0].Name)
		if err != nil {
			return false, err
		}
		res, err := result.FromLogs(logs)
		if err != nil {
			return false, err
		}
		lh.Info("result", "reason", res.Status.Reason, "message", res.Status.Message)
		return res.Status.Reason == reason, nil
	}).WithTemplate("Pod {{.Actual.Namespace}}/{{.Actual.Name}} UID {{.Actual.UID}} did not fail with expected reason {{.Data}}").WithTemplateData(reason)
}

const (
	reasonOOMKilled            = "OOMKilled"
	reasonCreateContainerError = "CreateContainerError"
)

func BeFailedToCreate(fxt *fixture.Fixture) types.GomegaMatcher {
	return gcustom.MakeMatcher(func(actual *corev1.Pod) (bool, error) {
		if actual == nil {
			return false, errors.New("nil Pod")
		}
		lh := fxt.Log.WithValues("podUID", actual.UID, "namespace", actual.Namespace, "name", actual.Name)
		if actual.Status.Phase != corev1.PodPending {
			lh.Info("unexpected phase", "phase", actual.Status.Phase)
			return false, nil
		}
		cntSt := findWaitingContainerStatus(actual.Status.ContainerStatuses)
		if cntSt == nil {
			lh.Info("no container in waiting state")
			return false, nil
		}
		if cntSt.State.Waiting.Reason != reasonCreateContainerError {
			lh.Info("container terminated for different reason", "containerName", cntSt.Name, "reason", cntSt.State.Terminated.Reason)
			return false, nil
		}
		lh.Info("container OOMKilled", "containerName", cntSt.Name)
		return true, nil
	}).WithTemplate("Pod {{.Actual.Namespace}}/{{.Actual.Name}} UID {{.Actual.UID}} was not in failed phase")
}

func BeOOMKilled(fxt *fixture.Fixture) types.GomegaMatcher {
	return gcustom.MakeMatcher(func(actual *corev1.Pod) (bool, error) {
		if actual == nil {
			return false, errors.New("nil Pod")
		}
		lh := fxt.Log.WithValues("podUID", actual.UID, "namespace", actual.Namespace, "name", actual.Name)
		if actual.Status.Phase != corev1.PodFailed {
			lh.Info("unexpected phase", "phase", actual.Status.Phase)
			return false, nil
		}
		cntSt := findTerminatedContainerStatus(actual.Status.ContainerStatuses)
		if cntSt == nil {
			lh.Info("no container in terminated state")
			return false, nil
		}
		if cntSt.State.Terminated.Reason != reasonOOMKilled {
			lh.Info("container terminated for different reason", "containerName", cntSt.Name, "reason", cntSt.State.Terminated.Reason)
			return false, nil
		}
		lh.Info("container OOMKilled", "containerName", cntSt.Name)
		return true, nil
	}).WithTemplate("Pod {{.Actual.Namespace}}/{{.Actual.Name}} UID {{.Actual.UID}} was not OOMKilled")
}

func findWaitingContainerStatus(statuses []corev1.ContainerStatus) *corev1.ContainerStatus {
	for idx := range statuses {
		cntSt := &statuses[idx]
		if cntSt.State.Waiting != nil {
			return cntSt
		}
	}
	return nil
}
func findTerminatedContainerStatus(statuses []corev1.ContainerStatus) *corev1.ContainerStatus {
	for idx := range statuses {
		cntSt := &statuses[idx]
		if cntSt.State.Terminated != nil {
			return cntSt
		}
	}
	return nil
}
