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
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/containerd/nri/pkg/api"
	"github.com/go-logr/logr"

	"k8s.io/utils/cpuset"

	"github.com/ffromani/dra-driver-memory/pkg/env"
	"github.com/ffromani/dra-driver-memory/pkg/hugepages"
	"github.com/ffromani/dra-driver-memory/pkg/types"
)

// NRI is the actuation layer. Once we reach this point, all the allocation decisions
// are already done and this layer "just" needs to enforce them.

func (mdrv *MemoryDriver) Synchronize(ctx context.Context, pods []*api.PodSandbox, containers []*api.Container) ([]*api.ContainerUpdate, error) {
	lh := mdrv.logrFromContext(ctx)
	lh = lh.WithName("Synchronize").WithValues("podCount", len(pods), "containerCount", len(containers))
	lh.V(4).Info("start")
	defer lh.V(4).Info("done")

	// TODO: restore the internal state
	return nil, nil
}

func (mdrv *MemoryDriver) CreateContainer(ctx context.Context, pod *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	lh := mdrv.logrFromContext(ctx)
	lh = lh.WithName("CreateContainer").WithValues("pod", pod.Namespace+"/"+pod.Name, "podUID", pod.Uid, "container", ctr.Name, "containerID", ctr.Id)
	lh.V(4).Info("start")
	defer lh.V(4).Info("done")

	cgroupParent, ok := mdrv.cgPathByPOD[pod.Uid]
	if ok {
		// TODO: this was initially introduced out of caution to handle pod sandbox creation race, which
		// are however unlikely (or impossible?). Deferring the pod-level setting at container level would
		// however allowing us to set more precise pod-level limits. This is something we can explore in the future.
		lh.V(2).Info("setting deferred pod cgroup limit", "podUID", pod.Uid, "cgroupParent", cgroupParent)
		_ = mdrv.setPodLimits(lh, cgroupParent)
	}

	adjust := &api.ContainerAdjustment{}
	var updates []*api.ContainerUpdate

	nodesByClaim, allocsByClaim, err := env.ExtractAll(lh, ctr.Env, mdrv.discoverer.AllResourceNames())
	if err != nil {
		lh.Error(err, "parsing DRA env for container")
	}

	if len(nodesByClaim) == 0 {
		lh.V(4).Info("No memory pinning for container")
		return adjust, updates, nil
	}

	lh.V(4).Info("extracted", "nodesByClaim", len(nodesByClaim), "allocsByClaim", len(allocsByClaim))

	var numaNodes cpuset.CPUSet
	for claimUID, claimNUMANodes := range nodesByClaim {
		numaNodes = numaNodes.Union(claimNUMANodes)
		mdrv.allocMgr.BindClaimToPod(lh, pod.Id, claimUID)
	}
	var allocs []types.Allocation
	for claimUID, alloc := range allocsByClaim {
		allocs = append(allocs, alloc)
		mdrv.allocMgr.BindClaimToPod(lh, pod.Id, claimUID)
	}

	adjust.SetLinuxCPUSetMems(numaNodes.String())
	for _, hpLimit := range hugepages.LimitsFromAllocations(lh, mdrv.discoverer.GetCachedMachineData(), allocs) {
		adjust.AddLinuxHugepageLimit(hpLimit.PageSize, hpLimit.Limit.Value) // MUST be set
	}

	lh.V(2).Info("memory pinning", "memoryNodes", numaNodes.String())
	for _, hp := range adjust.GetLinux().GetResources().GetHugepageLimits() {
		lh.V(2).Info("hugepage limits", "hugepageSize", hp.PageSize, "limit", hp.Limit)
	}

	return adjust, updates, nil
}

func (mdrv *MemoryDriver) UpdatePodSandbox(ctx context.Context, pod *api.PodSandbox, over *api.LinuxResources, res *api.LinuxResources) error {
	lh := mdrv.logrFromContext(ctx)
	lh = lh.WithName("UpdatePodSandbox").WithValues("pod", pod.Namespace+"/"+pod.Name, "podUID", pod.Uid)
	lh.V(4).Info("start")
	defer lh.V(4).Info("done")

	lh.V(2).Info("updates", "overhead", toJSON(over), "resources", toJSON(res))

	return nil
}

func (mdrv *MemoryDriver) UpdateContainer(ctx context.Context, pod *api.PodSandbox, ctr *api.Container, res *api.LinuxResources) ([]*api.ContainerUpdate, error) {
	lh := mdrv.logrFromContext(ctx)
	lh = lh.WithName("UpdateContainer").WithValues("pod", pod.Namespace+"/"+pod.Name, "podUID", pod.Uid, "container", ctr.Name, "containerID", ctr.Id)
	lh.V(4).Info("start")
	defer lh.V(4).Info("done")

	lh.V(2).Info("updates", "resources", toJSON(res))
	return nil, nil
}

func (mdrv *MemoryDriver) StopContainer(ctx context.Context, pod *api.PodSandbox, ctr *api.Container) ([]*api.ContainerUpdate, error) {
	lh := mdrv.logrFromContext(ctx)
	lh = lh.WithName("StopContainer").WithValues("pod", pod.Namespace+"/"+pod.Name, "podUID", pod.Uid, "container", ctr.Name, "containerID", ctr.Id)
	lh.V(4).Info("start")
	defer lh.V(4).Info("done")

	// TODO: downsize the pod limits?
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

	err := mdrv.setPodLimits(lh, pod.Linux.CgroupParent)
	if err != nil {
		mdrv.cgPathByPOD[pod.Uid] = pod.Linux.CgroupParent
		lh.V(2).Info("deferring pod limits settings", "podUID", pod.Uid, "cgroupParent", pod.Linux.CgroupParent)
	}
	return nil
}

func (mdrv *MemoryDriver) StopPodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	lh := mdrv.logrFromContext(ctx)
	lh = lh.WithName("StopPodSandbox").WithValues("pod", pod.Namespace+"/"+pod.Name, "podUID", pod.Uid)
	lh.V(4).Info("start")
	defer lh.V(4).Info("done")

	delete(mdrv.cgPathByPOD, pod.Uid)
	return nil
}

func (mdrv *MemoryDriver) RemovePodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	lh := mdrv.logrFromContext(ctx)
	lh = lh.WithName("RemovePodSandbox").WithValues("pod", pod.Namespace+"/"+pod.Name, "podUID", pod.Uid)
	lh.V(4).Info("start")
	defer lh.V(4).Info("done")

	mdrv.allocMgr.UnregisterClaimsForPod(lh, pod.Id)
	return nil
}

func (mdrv *MemoryDriver) setPodLimits(lh logr.Logger, cgroupParent string) error {
	if mdrv.cgMount == "" {
		return nil // nothing to do
	}
	cgPath := filepath.Join(mdrv.cgMount, cgroupParent)
	err := hugepages.SetSystemLimits(lh, cgPath, mdrv.hpRootLimits)
	if err != nil {
		lh.V(2).Error(err, "failed to set pod cgroup limits", "root", mdrv.cgMount, "path", cgroupParent)
		return err
	}
	return nil
}

func toJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("<JSON marshal error: %v>", err)
	}
	return string(data)
}
