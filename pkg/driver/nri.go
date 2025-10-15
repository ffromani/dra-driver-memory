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

package driver

import (
	"context"

	"github.com/containerd/nri/pkg/api"
	"github.com/go-logr/logr"

	"k8s.io/utils/cpuset"

	"github.com/ffromani/dra-driver-memory/pkg/draenv"
)

// NRI is the actuation layer. Once we reach this point, all the allocation decisions
// are already done and this layer "just" needs to enforce them.

func (mdrv *MemoryDriver) Synchronize(ctx context.Context, pods []*api.PodSandbox, containers []*api.Container) ([]*api.ContainerUpdate, error) {
	lh, _ := logr.FromContext(ctx)
	lh = lh.WithName("Synchronize").WithValues("podCount", len(pods), "containerCount", len(containers))
	lh.V(4).Info("start")
	defer lh.V(4).Info("done")
	return nil, nil
}

func (mdrv *MemoryDriver) CreateContainer(ctx context.Context, pod *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	lh := mdrv.logrFromContext(ctx)
	lh = lh.WithName("CreateContainer").WithValues("pod", pod.Namespace+"/"+pod.Name, "podUID", pod.Uid, "container", ctr.Name, "containerID", ctr.Id)
	lh.V(4).Info("start")
	defer lh.V(4).Info("done")

	adjust := &api.ContainerAdjustment{}
	var updates []*api.ContainerUpdate

	claimAllocations, err := draenv.ToClaimAllocations(lh, ctr.Env)
	if err != nil {
		lh.Error(err, "parsing DRA env for container")
	}

	if len(claimAllocations) == 0 {
		lh.V(4).Info("No memory pinning for container")
		return adjust, updates, nil
	}

	var numaNodes cpuset.CPUSet
	for _, allocNodes := range claimAllocations {
		numaNodes = numaNodes.Union(allocNodes)
	}

	lh.V(2).Info("memory pinning", "memoryNodes", numaNodes.String())
	adjust.SetLinuxCPUSetMems(numaNodes.String())

	return adjust, updates, nil
}

func (mdrv *MemoryDriver) StopContainer(ctx context.Context, pod *api.PodSandbox, ctr *api.Container) ([]*api.ContainerUpdate, error) {
	lh := mdrv.logrFromContext(ctx)
	lh = lh.WithName("StopContainer").WithValues("pod", pod.Namespace+"/"+pod.Name, "podUID", pod.Uid, "container", ctr.Name, "containerID", ctr.Id)
	lh.V(4).Info("start")
	defer lh.V(4).Info("done")
	return nil, nil
}

func (mdrv *MemoryDriver) RemoveContainer(ctx context.Context, pod *api.PodSandbox, ctr *api.Container) error {
	lh := mdrv.logrFromContext(ctx)
	lh = lh.WithName("RemoveContainer").WithValues("pod", pod.Namespace+"/"+pod.Name, "podUID", pod.Uid, "container", ctr.Name, "containerID", ctr.Id)
	lh.V(4).Info("start")
	defer lh.V(4).Info("done")
	return nil
}

func (mdrv *MemoryDriver) RunPodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	lh := mdrv.logrFromContext(ctx)
	lh = lh.WithName("RunPodSandbox").WithValues("pod", pod.Namespace+"/"+pod.Name, "podUID", pod.Uid)
	lh.V(4).Info("start")
	defer lh.V(4).Info("done")
	return nil
}

func (mdrv *MemoryDriver) StopPodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	lh := mdrv.logrFromContext(ctx)
	lh = lh.WithName("StopPodSandbox").WithValues("pod", pod.Namespace+"/"+pod.Name, "podUID", pod.Uid)
	lh.V(4).Info("start")
	defer lh.V(4).Info("done")
	return nil
}

func (mdrv *MemoryDriver) RemovePodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	lh := mdrv.logrFromContext(ctx)
	lh = lh.WithName("RemovePodSandbox").WithValues("pod", pod.Namespace+"/"+pod.Name, "podUID", pod.Uid)
	lh.V(4).Info("start")
	defer lh.V(4).Info("done")
	return nil
}

func (mdrv *MemoryDriver) logrFromContext(ctx context.Context) logr.Logger {
	lh, err := logr.FromContext(ctx)
	if err != nil {
		return mdrv.logger.WithName("nri")
	}
	return lh
}
