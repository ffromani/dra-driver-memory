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
	"maps"
	"slices"

	"github.com/go-logr/logr"
	cdiparser "tags.cncf.io/container-device-interface/pkg/parser"

	resourceapi "k8s.io/api/resource/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/dynamic-resource-allocation/kubeletplugin"
	"k8s.io/dynamic-resource-allocation/resourceslice"
	"k8s.io/klog/v2"

	"github.com/ffromani/dra-driver-memory/pkg/cdi"
	"github.com/ffromani/dra-driver-memory/pkg/env"
	"github.com/ffromani/dra-driver-memory/pkg/sysinfo"
	"github.com/ffromani/dra-driver-memory/pkg/types"
)

// This is the DRA frontend. Allocation, if and when required, will happen at this layer.
// The core responsibility of this layer is to translate Device Requests into CDI specs,
// and to manage the latter on the node.

func (mdrv *MemoryDriver) PublishResources(ctx context.Context) {
	lh := mdrv.logrFromContext(ctx)
	lh = lh.WithName("PublishResources")
	lh.V(2).Info("start")
	defer lh.V(2).Info("done")

	machinedata, err := mdrv.sysinformer.Discover()
	if err != nil {
		lh.Error(err, "enumerating memory resources")
		return
	}

	resourceInfo := sysinfo.Process(lh, machinedata)
	// TODO: what about races?
	mdrv.spanByDeviceName = resourceInfo.GetSpanByDeviceName()
	mdrv.resourceNames = resourceInfo.GetResourceNames()
	mdrv.machineData = machinedata

	resources := resourceslice.DriverResources{
		Pools: map[string]resourceslice.Pool{
			mdrv.nodeName: {
				Slices: resourceInfo.GetResourceSlices(),
			},
		},
	}

	err = mdrv.draPlugin.PublishResources(ctx, resources)
	if err != nil {
		lh.Error(err, "publishing resources through DRA")
	}
}

func (mdrv *MemoryDriver) PrepareResourceClaims(ctx context.Context, claims []*resourceapi.ResourceClaim) (map[k8stypes.UID]kubeletplugin.PrepareResult, error) {
	lh := mdrv.logrFromContext(ctx)
	lh = lh.WithName("PrepareResourceClaims")
	lh.V(4).Info("start", "claimCount", len(claims))
	defer lh.V(4).Info("done", "claimCount", len(claims))

	result := make(map[k8stypes.UID]kubeletplugin.PrepareResult)
	if len(claims) == 0 {
		return result, nil
	}

	for _, claim := range claims {
		result[claim.UID] = mdrv.prepareResourceClaim(ctx, claim)
	}
	return result, nil
}

// UnprepareResourceClaims is called by the kubelet to unprepare the resources for a claim.
func (mdrv *MemoryDriver) UnprepareResourceClaims(ctx context.Context, claims []kubeletplugin.NamespacedObject) (map[k8stypes.UID]error, error) {
	lh := mdrv.logrFromContext(ctx)
	lh = lh.WithName("UnprepareResourceClaims")
	lh.V(4).Info("start", "claimCount", len(claims))
	defer lh.V(4).Info("done", "claimCount", len(claims))

	result := make(map[k8stypes.UID]error)
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
	lh := mdrv.logrFromContext(ctx)
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

	deviceName := cdi.MakeDeviceName(claim.UID)
	qualifiedName := cdiparser.QualifiedName(cdi.Vendor, cdi.Class, deviceName)
	lh.V(4).Info("CDI data", "DeviceName", deviceName, "qualifiedName", qualifiedName)

	var envs []string
	preparedDevices := []kubeletplugin.Device{}
	claimAllocs := make(map[string]types.Allocation)
	claimNodes := sets.New[int64]()
	for _, devRes := range claim.Status.Allocation.Devices.Results {
		if devRes.Driver != mdrv.driverName {
			continue
		}

		span, ok := mdrv.spanByDeviceName[devRes.Device]
		if !ok {
			return kubeletplugin.PrepareResult{
				Err: fmt.Errorf("device %q not matches any registered memory span", devRes.Device),
			}
		}

		capName := span.CapacityName()
		capList := slices.Collect(maps.Keys(devRes.ConsumedCapacity))
		lh.V(4).Info("consumed capacity", "expected", capName, "effective", capList)
		res, ok := devRes.ConsumedCapacity[capName]
		if !ok {
			return kubeletplugin.PrepareResult{
				Err: fmt.Errorf("device %q not matches consumed capacity. Expected: %q Consumed: %q", devRes.Device, capName, capList),
			}
		}
		amount, ok := res.AsInt64()
		if !ok {
			return kubeletplugin.PrepareResult{
				Err: fmt.Errorf("device %q not matches consumed capacity. Expected: %q Consumed: %q", devRes.Device, capName, capList),
			}
		}

		alloc := types.Allocation{
			ResourceIdent: types.ResourceIdent{
				Kind:     span.Kind,
				Pagesize: span.Pagesize,
			},
			Amount:   amount,
			NUMAZone: span.NUMAZone,
		}

		envs = append(envs, env.CreateSpan(lh, claim.UID, alloc.Name(), alloc.Amount, alloc.NUMAZone))

		lh.V(2).Info("prepareResourceClaim", "pool", devRes.Pool, "device", devRes.Device, "resource", span.Name(), "amount", alloc.Amount, "numaNode", alloc.NUMAZone)
		claimAllocs[alloc.Name()] = alloc
		claimNodes.Insert(alloc.NUMAZone)
		preparedDevices = append(preparedDevices, kubeletplugin.Device{
			PoolName:     devRes.Pool,
			DeviceName:   devRes.Device,
			CDIDeviceIDs: []string{qualifiedName},
		})
	}

	if len(claimAllocs) == 0 {
		lh.V(2).Info("no valid allocation for this driver")
		return kubeletplugin.PrepareResult{}
	}

	envs = append(envs, env.CreateNUMANodes(lh, claim.UID, claimNodes))

	err := mdrv.cdiMgr.AddDevice(lh, deviceName, envs...)
	if err != nil {
		return kubeletplugin.PrepareResult{
			Err: err,
		}
	}

	mdrv.allocMgr.RegisterClaim(claim.UID, claimAllocs)

	return kubeletplugin.PrepareResult{
		Devices: preparedDevices,
	}
}

func (mdrv *MemoryDriver) unprepareResourceClaim(ctx context.Context, claim kubeletplugin.NamespacedObject) error {
	lh, _ := logr.FromContext(ctx)
	lh = lh.WithName("UnprepareResourceClaims").WithValues("claim", claim.String())
	mdrv.allocMgr.UnregisterClaim(claim.UID)
	return mdrv.cdiMgr.RemoveDevice(lh, cdi.MakeDeviceName(claim.UID))
}
