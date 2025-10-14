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
	"fmt"

	"github.com/go-logr/logr"
	cdiparser "tags.cncf.io/container-device-interface/pkg/parser"

	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/dynamic-resource-allocation/kubeletplugin"
	"k8s.io/dynamic-resource-allocation/resourceslice"
	"k8s.io/klog/v2"

	"github.com/ffromani/dra-driver-memory/pkg/cdi"
	"github.com/ffromani/dra-driver-memory/pkg/draenv"
	"github.com/ffromani/dra-driver-memory/pkg/ghwinfo"
)

// This is the DRA frontend. Allocation, if and when required, will happen at this layer.
// The core responsability of this layer is to translate Device Requests into CDI specs,
// and to manage the latter on the node.

func (mdrv *MemoryDriver) PublishResources(ctx context.Context) {
	lh, _ := logr.FromContext(ctx)
	lh = lh.WithName("PublishResources")
	lh.V(2).Info("start")
	defer lh.V(2).Info("done")

	systopology, err := mdrv.sysinformer.Topology()
	if err != nil {
		lh.Error(err, "enumerating memory resources")
		return
	}

	slices, deviceNameToNUMANodeMap := ghwinfo.Discover(lh, systopology)
	mdrv.deviceNameToNUMANode = deviceNameToNUMANodeMap

	resources := resourceslice.DriverResources{
		Pools: map[string]resourceslice.Pool{
			mdrv.nodeName: {
				Slices: slices,
			},
		},
	}

	err = mdrv.draPlugin.PublishResources(ctx, resources)
	if err != nil {
		lh.Error(err, "publishing resources through DRA")
	}
}

func (mdrv *MemoryDriver) PrepareResourceClaims(ctx context.Context, claims []*resourceapi.ResourceClaim) (map[types.UID]kubeletplugin.PrepareResult, error) {
	lh, _ := logr.FromContext(ctx)
	lh = lh.WithName("PrepareResourceClaims")
	lh.V(4).Info("start", "claimCount", len(claims))
	defer lh.V(4).Info("done", "claimCount", len(claims))

	result := make(map[types.UID]kubeletplugin.PrepareResult)
	if len(claims) == 0 {
		return result, nil
	}

	for _, claim := range claims {
		result[claim.UID] = mdrv.prepareResourceClaim(ctx, claim)
	}
	return result, nil
}

// UnprepareResourceClaims is called by the kubelet to unprepare the resources for a claim.
func (mdrv *MemoryDriver) UnprepareResourceClaims(ctx context.Context, claims []kubeletplugin.NamespacedObject) (map[types.UID]error, error) {
	lh, _ := logr.FromContext(ctx)
	lh = lh.WithName("UnprepareResourceClaims")
	lh.V(4).Info("start", "claimCount", len(claims))
	defer lh.V(4).Info("done", "claimCount", len(claims))

	result := make(map[types.UID]error)
	if len(claims) == 0 {
		return result, nil
	}

	for _, claim := range claims {
		err := mdrv.unprepareResourceClaim(ctx, claim)
		result[claim.UID] = err
		if err != nil {
			lh.Error(err, "unpreparing resources", "claim", claim.String())
		}
	}
	return result, nil
}

func (mdrv *MemoryDriver) HandleError(ctx context.Context, err error, msg string) {
	lh, _ := logr.FromContext(ctx)
	lh = lh.WithName("HandleError")
	// TODO: Implement this function
	lh.Error(err, msg)
}

func (mdrv *MemoryDriver) prepareResourceClaim(ctx context.Context, claim *resourceapi.ResourceClaim) kubeletplugin.PrepareResult {
	lh, _ := logr.FromContext(ctx)
	lh = lh.WithName("PrepareResourceClaims").WithValues("claim", klog.KObj(claim))

	if claim.Status.Allocation == nil {
		return kubeletplugin.PrepareResult{
			Err: fmt.Errorf("claim %s has no allocation", klog.KObj(claim)),
		}
	}

	claimNodes := sets.New[int64]()
	for _, alloc := range claim.Status.Allocation.Devices.Results {
		if alloc.Driver != mdrv.driverName {
			continue
		}
		numaNode, ok := mdrv.deviceNameToNUMANode[alloc.Device]
		if !ok {
			return kubeletplugin.PrepareResult{
				Err: fmt.Errorf("device %q not found in device mapping", alloc.Device),
			}
		}
		claimNodes.Insert(numaNode)
	}

	if claimNodes.Len() == 0 {
		lh.V(2).Info("no valid allocation for this driver")
		return kubeletplugin.PrepareResult{}
	}

	envVar := draenv.FromClaimAllocations(lh, claim.UID, claimNodes)
	deviceName := cdi.MakeDeviceName(claim.UID)

	err := mdrv.cdiMgr.AddDevice(lh, deviceName, envVar)
	if err != nil {
		return kubeletplugin.PrepareResult{Err: err}
	}

	qualifiedName := cdiparser.QualifiedName(cdi.Vendor, cdi.Class, deviceName)
	lh.V(2).Info("CDI data", "DeviceName", deviceName, "envVar", envVar, "qualifiedName", qualifiedName)

	preparedDevices := []kubeletplugin.Device{}
	for _, allocResult := range claim.Status.Allocation.Devices.Results {
		preparedDevice := kubeletplugin.Device{
			PoolName:     allocResult.Pool,
			DeviceName:   allocResult.Device,
			CDIDeviceIDs: []string{qualifiedName},
		}
		preparedDevices = append(preparedDevices, preparedDevice)
	}

	return kubeletplugin.PrepareResult{
		Devices: preparedDevices,
	}
}

func (mdrv *MemoryDriver) unprepareResourceClaim(ctx context.Context, claim kubeletplugin.NamespacedObject) error {
	lh, _ := logr.FromContext(ctx)
	lh = lh.WithName("UnprepareResourceClaims").WithValues("claim", claim.String())
	return mdrv.cdiMgr.RemoveDevice(lh, cdi.MakeDeviceName(claim.UID))
}
