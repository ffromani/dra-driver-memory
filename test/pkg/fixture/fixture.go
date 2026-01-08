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

package fixture

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/onsi/ginkgo/v2"

	corev1 "k8s.io/api/core/v1"
	resourcev1 "k8s.io/api/resource/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"github.com/ffromani/dra-driver-memory/test/pkg/client"
)

func By(format string, args ...any) {
	ginkgo.By(fmt.Sprintf(format, args...))
}

type Fixture struct {
	Prefix       string
	K8SClientset kubernetes.Interface
	Namespace    *corev1.Namespace
	Log          logr.Logger
}

func ForGinkgo() (*Fixture, error) {
	cs, err := client.NewK8SClientset()
	if err != nil {
		return nil, err
	}
	return &Fixture{
		K8SClientset: cs,
		Log:          ginkgo.GinkgoLogr,
	}, nil
}

func (fxt *Fixture) WithPrefix(prefix string) *Fixture {
	return &Fixture{
		Prefix:       prefix,
		K8SClientset: fxt.K8SClientset,
		Log:          fxt.Log,
	}
}

func (fxt *Fixture) Setup(ctx context.Context) error {
	if fxt.Namespace != nil {
		Fail("Setup called, but namespace object already exists: %q", fxt.Namespace.Name)
	}
	generateName := "dramem-e2e-"
	if fxt.Prefix != "" {
		generateName += fxt.Prefix + "-"
	}
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: generateName,
		},
	}
	nsCreated, err := fxt.K8SClientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create namespace %s: %w", ns.Name, err)
	}
	fxt.Namespace = nsCreated
	fxt.Log.Info("fixture setup", "namespace", fxt.Namespace.Name)
	return nil
}

func (fxt *Fixture) Teardown(ctx context.Context) error {
	if fxt.Namespace == nil {
		Fail("Teardown called, but namespace object is nil")
	}
	err := fxt.K8SClientset.CoreV1().Namespaces().Delete(ctx, fxt.Namespace.Name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete namespace %s: %w", fxt.Namespace.Name, err)
	}
	err = waitForNamespaceToBeDeleted(ctx, fxt.K8SClientset, fxt.Namespace.Name)
	if err != nil {
		return err
	}
	fxt.Log.Info("fixture teardown", "namespace", fxt.Namespace.Name)
	fxt.Namespace = nil
	return nil
}

func (fxt *Fixture) NodeHasMemoryResource(ctx context.Context, nodeName, size string, amount int64) (string, string, bool) {
	lh := fxt.Log.WithValues("nodeName", nodeName)
	resourceSliceList, err := fxt.K8SClientset.ResourceV1().ResourceSlices().List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
	})
	if err != nil {
		fxt.Log.Error(err, "cannot list resourceslices", "nodeName", nodeName)
		return "", "", false
	}
	lh.Info("checking resource slices", "count", len(resourceSliceList.Items))
	desiredQty := *resource.NewQuantity(amount, resource.BinarySI)
	for idx := range resourceSliceList.Items {
		resourceSlice := &resourceSliceList.Items[idx]
		lh.Info("checking resource slices", "name", resourceSlice.Name)
		rdev := findMemoryDeviceInResourceSlice(lh, resourceSlice, size)
		if rdev == nil {
			lh.Info("missing device in resource slice", "size", size, "name", resourceSlice.Name)
			continue // go to the next slice
		}
		lh.Info("found device in resource slice", "size", size, "name", resourceSlice.Name, "deviceName", rdev.Name)
		size, ok := rdev.Capacity[resourcev1.QualifiedName("size")]
		if !ok {
			lh.Info("device in resource slice lacks capacity", "size", size, "name", resourceSlice.Name, "deviceName", rdev.Name)
			continue // how come?
		}
		if size.Value.Cmp(desiredQty) < 0 {
			lh.Info("device in resource slice has not enough capacity", "size", size, "name", resourceSlice.Name, "deviceName", rdev.Name, "capacityCurrent", size.Value.String(), "capacityDesired", desiredQty.String())
			continue
		}
		return resourceSlice.Name, rdev.Name, true
	}
	return "", "", false
}

func findMemoryDeviceInResourceSlice(lh logr.Logger, resourceSlice *resourcev1.ResourceSlice, size string) *resourcev1.Device {
	for idx := range resourceSlice.Spec.Devices {
		rdev := &resourceSlice.Spec.Devices[idx]
		lh.Info("checking device", "resourceSlice", resourceSlice.Name, "deviceName", rdev.Name)
		if matchesByAttributes(lh.WithValues("deviceName", rdev.Name), rdev.Attributes, size) {
			return rdev
		}
	}
	return nil
}

func matchesByAttributes(lh logr.Logger, attrs map[resourcev1.QualifiedName]resourcev1.DeviceAttribute, size string) bool {
	lh.Info("inspecting", "attributes", attrs)
	val, ok := attrs[resourcev1.QualifiedName("resource.kubernetes.io/hugeTLB")]
	if !ok || val.BoolValue == nil {
		return false
	}
	lh.Info("hugeTLB bool present")
	val, ok = attrs[resourcev1.QualifiedName("resource.kubernetes.io/pageSize")]
	if !ok || val.StringValue == nil || *val.StringValue != size {
		return false
	}
	lh.Info("size attribute match")
	return true
}

func Skipf(fmts_ string, args ...any) {
	ginkgo.Skip(fmt.Sprintf(fmts_, args...))
}

func Fail(fmts_ string, args ...any) {
	ginkgo.Fail(fmt.Sprintf(fmts_, args...))
}

const (
	nsPollInterval = time.Second * 10
	nsPollTimeout  = time.Minute * 2
)

func waitForNamespaceToBeDeleted(ctx context.Context, cs kubernetes.Interface, nsName string) error {
	immediate := true
	err := wait.PollUntilContextTimeout(ctx, nsPollInterval, nsPollTimeout, immediate, func(ctx2 context.Context) (done bool, err error) {
		_, err = cs.CoreV1().Namespaces().Get(ctx2, nsName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("namespace=%s was not deleted: %w", nsName, err)
	}
	return nil
}
