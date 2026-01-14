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

	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/cpuset"

	"github.com/ffromani/dra-driver-memory/pkg/env"
	"github.com/ffromani/dra-driver-memory/pkg/hugepages"
	"github.com/ffromani/dra-driver-memory/pkg/sysinfo"
	"github.com/ffromani/dra-driver-memory/pkg/types"
)

// NRI is the actuation layer. Once we reach this point, all the allocation decisions
// are already done and this layer "just" needs to enforce them.

func (mdrv *MemoryDriver) Synchronize(ctx context.Context, pods []*api.PodSandbox, containers []*api.Container) ([]*api.ContainerUpdate, error) {
	lh := mdrv.logrFromContext(ctx)
	lh = lh.WithName("Synchronize")
	lh.V(4).Info("start")
	defer lh.V(4).Info("done")

	// we start from empty state, so we can just be additive
	// we recover in reverse (container, then sandbox) because we have a easy way
	// to detect the containers which we processed, so from these we can find the
	// relevant sandboxes
	podsBySandboxID := make(map[string]*api.PodSandbox, len(pods))
	for _, pod := range pods {
		lh.V(4).Info("reverse map", "podSandboxID", pod.Id, "podUID", pod.Uid)
		podsBySandboxID[pod.Id] = pod
	}
	knownPods := sets.New[string]()

	for _, ctr := range containers {
		lh_ := lh.WithValues("podSandboxID", ctr.PodSandboxId, "container", ctr.Name, "containerID", ctr.Id)
		pod, ok := podsBySandboxID[ctr.PodSandboxId]
		if !ok {
			return nil, fmt.Errorf("unknown sandbox: %q for container %q (%q)", ctr.PodSandboxId, ctr.Name, ctr.Id)
		}
		_, _, ok, err := mdrv.handleContainer(lh_, pod, ctr)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		lh_.V(4).Info("backreferencing")
		knownPods.Insert(ctr.PodSandboxId)
	}

	for _, pod := range pods {
		lh_ := lh.WithValues("podSandboxID", pod.Id, "podUID", pod.Uid, "pod", pod.Namespace+"/"+pod.Name)
		if !knownPods.Has(pod.Id) {
			continue
		}
		lh_.V(4).Info("backreferenced pod")
		err := mdrv.handlePodSandbox(lh_, pod)
		if err != nil {
			return nil, err
		}
	}

	return []*api.ContainerUpdate{}, nil
}

func (mdrv *MemoryDriver) CreateContainer(ctx context.Context, pod *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	lh := mdrv.logrFromContext(ctx)
	lh = lh.WithName("CreateContainer").WithValues("pod", pod.Namespace+"/"+pod.Name, "podUID", pod.Uid, "container", ctr.Name, "containerID", ctr.Id)
	lh.V(4).Info("start")
	defer lh.V(4).Info("done")

	lh.V(4).Info("container backref", "sandboxID", ctr.PodSandboxId)
	numaNodes, allocs, ok, err := mdrv.handleContainer(lh, pod, ctr)
	if err != nil {
		lh.Error(err, "cannot create container")
		return nil, nil, err
	}
	var updates []*api.ContainerUpdate
	if !ok {
		lh.V(4).Info("No memory pinning for container")
		return &api.ContainerAdjustment{}, updates, nil
	}

	machineData := mdrv.discoverer.GetCachedMachineData()
	hpLimits := hugepages.LimitsFromAllocations(lh, machineData, allocs)
	cgroupParent := mdrv.cgPathByPodUID[pod.Uid]
	if cgroupParent != "" {
		lh.V(2).Info("setting deferred pod cgroup limit", "cgroupParent", cgroupParent)
		_ = mdrv.updatePodLimits(lh, machineData, cgroupParent, hpLimits)
	}

	adjust := &api.ContainerAdjustment{}
	adjust.SetLinuxCPUSetMems(numaNodes.String())
	for _, hpLimit := range hpLimits {
		adjust.AddLinuxHugepageLimit(hpLimit.PageSize, hpLimit.Limit.Value) // MUST be set
	}

	logAdjust(lh, adjust)

	return adjust, updates, nil
}

func (mdrv *MemoryDriver) UpdatePodSandbox(ctx context.Context, pod *api.PodSandbox, over *api.LinuxResources, res *api.LinuxResources) error {
	lh := mdrv.logrFromContext(ctx)
	lh = lh.WithName("UpdatePodSandbox").WithValues("pod", pod.Namespace+"/"+pod.Name, "podUID", pod.Uid, "podSandboxID", pod.Id)
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
	lh = lh.WithName("RunPodSandbox").WithValues("pod", pod.Namespace+"/"+pod.Name, "podUID", pod.Uid, "podSandboxID", pod.Id)
	lh.V(4).Info("start")
	defer lh.V(4).Info("done")

	return mdrv.handlePodSandbox(lh, pod)
}

func (mdrv *MemoryDriver) StopPodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	lh := mdrv.logrFromContext(ctx)
	lh = lh.WithName("StopPodSandbox").WithValues("pod", pod.Namespace+"/"+pod.Name, "podUID", pod.Uid, "podSandboxID", pod.Id)
	lh.V(4).Info("start")
	defer lh.V(4).Info("done")

	delete(mdrv.cgPathByPodUID, pod.Uid)
	return nil
}

func (mdrv *MemoryDriver) RemovePodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	lh := mdrv.logrFromContext(ctx)
	lh = lh.WithName("RemovePodSandbox").WithValues("pod", pod.Namespace+"/"+pod.Name, "podUID", pod.Uid, "podSandboxID", pod.Id)
	lh.V(4).Info("start")
	defer lh.V(4).Info("done")

	claimUIDs := mdrv.allocMgr.CleanupPod(lh, pod.Id)
	mdrv.bindMgr.Cleanup(lh, claimUIDs...)
	return nil
}

func (mdrv *MemoryDriver) handleContainer(lh logr.Logger, pod *api.PodSandbox, ctr *api.Container) (cpuset.CPUSet, []types.Allocation, bool, error) {
	nodesByClaim, allocsByClaim, err := env.ExtractAll(lh, ctr.Env, mdrv.discoverer.AllResourceNames())
	if err != nil {
		return cpuset.CPUSet{}, nil, false, err
	}

	if len(nodesByClaim) == 0 {
		return cpuset.CPUSet{}, nil, false, nil
	}

	lh.V(4).Info("extracted", "nodesByClaim", len(nodesByClaim), "allocsByClaim", len(allocsByClaim))

	claimUIDs := sets.New[k8stypes.UID]()
	var numaNodes cpuset.CPUSet
	var allocs []types.Allocation

	for claimUID, claimNUMANodes := range nodesByClaim {
		numaNodes = numaNodes.Union(claimNUMANodes)
		claimUIDs.Insert(claimUID)
	}
	for claimUID, alloc := range allocsByClaim {
		allocs = append(allocs, alloc)
		claimUIDs.Insert(claimUID)
	}

	for _, claimUID := range claimUIDs.UnsortedList() {
		mdrv.allocMgr.BindClaim(lh, claimUID, ctr.PodSandboxId)
		err := mdrv.bindMgr.SetOwner(lh, claimUID, pod.Uid, ctr.Name)
		if err != nil {
			return cpuset.CPUSet{}, nil, false, err
		}
	}

	return numaNodes, allocs, true, nil
}

func (mdrv *MemoryDriver) handlePodSandbox(lh logr.Logger, pod *api.PodSandbox) error {
	mdrv.cgPathByPodUID[pod.Uid] = pod.Linux.CgroupParent
	lh.V(2).Info("registered pod cgroup path", "cgroupParent", pod.Linux.CgroupParent)
	return nil
}

func (mdrv *MemoryDriver) updatePodLimits(lh logr.Logger, machineData sysinfo.MachineData, cgroupParent string, limits []hugepages.Limit) error {
	if mdrv.cgMount == "" {
		return nil // nothing to do
	}
	cgPath := filepath.Join(mdrv.cgMount, cgroupParent)

	curLimits, err := hugepages.LimitsFromSystemPath(lh, machineData, cgroupParent)
	if err != nil {
		lh.V(2).Error(err, "failed to get the current pod cgroup limits", "root", mdrv.cgMount, "path", cgroupParent)
		return err
	}

	newLimits := hugepages.SumLimits(curLimits, limits)
	lh.V(4).Info("pod limits",
		"previous", hugepages.LimitsToString(curLimits),
		"current", hugepages.LimitsToString(limits),
		"enforcing", hugepages.LimitsToString(newLimits),
	)

	err = hugepages.SetSystemLimits(lh, cgPath, newLimits)
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

func logAdjust(lh logr.Logger, adjust *api.ContainerAdjustment) {
	lh.V(2).Info("memory pinning", "memoryNodes", adjust.GetLinux().GetResources().GetCpu().GetMems())
	for _, hp := range adjust.GetLinux().GetResources().GetHugepageLimits() {
		lh.V(2).Info("hugepage limits", "hugepageSize", hp.PageSize, "limit", hp.Limit)
	}
}
