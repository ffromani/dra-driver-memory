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

	v1helper "k8s.io/kubernetes/pkg/apis/core/v1/helper"
	"k8s.io/utils/cpuset"

	"github.com/ffromani/dra-driver-memory/pkg/env"
	"github.com/ffromani/dra-driver-memory/pkg/sysinfo"
	"github.com/ffromani/dra-driver-memory/pkg/types"
	"github.com/ffromani/dra-driver-memory/pkg/unitconv"
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

	adjust := &api.ContainerAdjustment{}
	var updates []*api.ContainerUpdate

	nodesByClaim, allocsByClaim, err := env.ExtractAll(lh, ctr.Env, mdrv.resourceNames)
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
		mdrv.allocMgr.BindClaimToPod(pod.Id, claimUID)
	}
	var allocs []types.Allocation
	for claimUID, alloc := range allocsByClaim {
		allocs = append(allocs, alloc)
		mdrv.allocMgr.BindClaimToPod(pod.Id, claimUID)
	}

	adjust.SetLinuxCPUSetMems(numaNodes.String())
	for _, hpLimit := range hugepageLimitsFromAllocations(lh, mdrv.machineData, allocs) {
		adjust.AddLinuxHugepageLimit(hpLimit.PageSize, hpLimit.Limit)
	}

	lh.V(2).Info("memory pinning", "memoryNodes", numaNodes.String())
	for _, hp := range ctr.GetLinux().GetResources().GetHugepageLimits() {
		lh.V(2).Info("hugepage limits", "hugepageSize", hp.PageSize, "limit", hp.Limit)
	}

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

// hugepageLimit is a Plain-Old-Data struct we carry around to do our computations;
// this way we can set `runtimeapi.HugepageLimit` once and avoid copies.
type hugepageLimit struct {
	// The value of PageSize has the format <size><unit-prefix>B (2MB, 1GB),
	// and must match the <hugepagesize> of the corresponding control file found in `hugetlb.<hugepagesize>.limit_in_bytes`.
	// The values of <unit-prefix> are intended to be parsed using base 1024("1KB" = 1024, "1MB" = 1048576, etc).
	PageSize string
	// limit in bytes of hugepagesize HugeTLB usage.
	Limit uint64
}

func hugepageLimitsFromAllocations(lh logr.Logger, machineData sysinfo.MachineData, allocs []types.Allocation) []hugepageLimit {
	var hugepageLimits []hugepageLimit

	for _, hpSize := range machineData.Hugepagesizes {
		hugepageLimits = append(hugepageLimits, hugepageLimit{
			PageSize: unitconv.SizeInBytesToCGroupString(hpSize),
			Limit:    uint64(0),
		})
	}

	requiredHugepageLimits := map[string]uint64{}
	for _, alloc := range allocs {
		sizeString, err := v1helper.HugePageUnitSizeFromByteSize(int64(alloc.Pagesize))
		if err != nil {
			lh.V(2).Info("Size is invalid", "allocation", alloc.Name(), "err", err)
			continue
		}
		requiredHugepageLimits[sizeString] = uint64(alloc.Amount)
	}

	for _, hugepageLimit := range hugepageLimits {
		limit, exists := requiredHugepageLimits[hugepageLimit.PageSize]
		if !exists {
			continue
		}
		hugepageLimit.Limit = limit
	}

	return hugepageLimits
}
