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

	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/dynamic-resource-allocation/kubeletplugin"
	"k8s.io/dynamic-resource-allocation/resourceslice"
	"k8s.io/utils/ptr"
)

func (cp *MemoryDriver) PublishResources(ctx context.Context) {
	lh, _ := logr.FromContext(ctx)
	lh = lh.WithName("PublishResources")

	lh.V(2).Info("start")
	defer lh.V(2).Info("done")

	slices, err := cp.createMemoryDeviceSlices(lh)
	if err != nil {
		lh.Error(err, "no devices available")
		return
	}

	resources := resourceslice.DriverResources{
		Pools: map[string]resourceslice.Pool{
			// All slices are published under the same pool for this node.
			cp.nodeName: {
				Slices: slices,
			},
		},
	}

	err = cp.draPlugin.PublishResources(ctx, resources)
	if err != nil {
		lh.Error(err, "publishing resources through DRA")
	}
}

func (cp *MemoryDriver) PrepareResourceClaims(ctx context.Context, claims []*resourceapi.ResourceClaim) (map[types.UID]kubeletplugin.PrepareResult, error) {
	lh, _ := logr.FromContext(ctx)
	lh = lh.WithName("PrepareResourceClaims")
	lh.Info("called", "claimCount", len(claims))
	result := make(map[types.UID]kubeletplugin.PrepareResult)
	return result, nil
}

// UnprepareResourceClaims is called by the kubelet to unprepare the resources for a claim.
func (cp *MemoryDriver) UnprepareResourceClaims(ctx context.Context, claims []kubeletplugin.NamespacedObject) (map[types.UID]error, error) {
	lh, _ := logr.FromContext(ctx)
	lh = lh.WithName("UnprepareResourceClaims")
	lh.Info("called", "claimCount", len(claims))
	result := make(map[types.UID]error)
	return result, nil
}

func (cp *MemoryDriver) HandleError(ctx context.Context, err error, msg string) {
	lh, _ := logr.FromContext(ctx)
	lh = lh.WithName("HandleError")
	// TODO: Implement this function
	lh.Error(err, msg)
}

const (
	pageSize = 4 * 1024 // TODO: review
)

func (cp *MemoryDriver) createMemoryDeviceSlices(lh logr.Logger) ([]resourceslice.Slice, error) {
	systopology, err := cp.sysinformer.Topology()
	if err != nil {
		return nil, err
	}
	memorySlice := resourceslice.Slice{}
	hugepageSlice := resourceslice.Slice{}
	for numaNode, nodeInfo := range systopology.Nodes {
		if nodeInfo.Memory == nil {
			continue // TODO: how come? log
		}
		numaNode := int64(numaNode)

		memName := fmt.Sprintf("memory-%d", numaNode)
		pageQty := resource.NewQuantity(nodeInfo.Memory.TotalUsableBytes/pageSize, resource.DecimalSI)
		counterName := "dramemory"
		memDevice := resourceapi.Device{
			Name: memName,
			Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
				"dra.memory/numaNode": {IntValue: ptr.To(numaNode)},
				"dra.cpu/numaNode":    {IntValue: ptr.To(numaNode)},
				"dra.net/numaNode":    {IntValue: ptr.To(numaNode)},
			},
			Capacity: map[resourceapi.QualifiedName]resourceapi.DeviceCapacity{
				"pages": resourceapi.DeviceCapacity{
					Value: *pageQty,
				},
			},
			ConsumesCounters: []resourceapi.DeviceCounterConsumption{
				{
					CounterSet: counterName,
					Counters: map[string]resourceapi.Counter{
						memName: resourceapi.Counter{
							Value: *resource.NewQuantity(1, resource.DecimalSI),
						},
					},
				},
			},
			AllowMultipleAllocations: ptr.To(true),
		}
		memorySlice.Devices = append(memorySlice.Devices, memDevice)

		memCounterSet := resourceapi.CounterSet{
			Name: counterName,
			Counters: map[string]resourceapi.Counter{
				memName: resourceapi.Counter{
					Value: *pageQty,
				},
			},
		}
		memorySlice.SharedCounters = append(memorySlice.SharedCounters, memCounterSet)

		for sizeInBytes, amounts := range nodeInfo.Memory.HugePageAmountsBySize {
			hpBasename := hugepageNameBySizeInBytes(sizeInBytes)
			hpName := fmt.Sprintf("%s-%d", hpBasename, numaNode)
			hpQty := resource.NewQuantity(amounts.Total, resource.DecimalSI)
			counterName := "dra" + hpBasename
			hpDevice := resourceapi.Device{
				Name: hpName,
				Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
					"dra.memory/numaNode": {IntValue: ptr.To(numaNode)},
					"dra.cpu/numaNode":    {IntValue: ptr.To(numaNode)},
					"dra.net/numaNode":    {IntValue: ptr.To(numaNode)},
				},
				Capacity: map[resourceapi.QualifiedName]resourceapi.DeviceCapacity{
					"pages": resourceapi.DeviceCapacity{
						Value: *hpQty,
					},
				},
				ConsumesCounters: []resourceapi.DeviceCounterConsumption{
					{
						CounterSet: counterName,
						Counters: map[string]resourceapi.Counter{
							hpName: resourceapi.Counter{
								Value: *resource.NewQuantity(1, resource.DecimalSI),
							},
						},
					},
				},
				AllowMultipleAllocations: ptr.To(true),
			}
			hugepageSlice.Devices = append(hugepageSlice.Devices, hpDevice)

			hpCounterSet := resourceapi.CounterSet{
				Name: counterName,
				Counters: map[string]resourceapi.Counter{
					hpName: resourceapi.Counter{
						Value: *hpQty,
					},
				},
			}
			hugepageSlice.SharedCounters = append(hugepageSlice.SharedCounters, hpCounterSet)
		}
	}

	return []resourceslice.Slice{
		memorySlice,
		hugepageSlice,
	}, nil
}

// TODO: only amd64 supported atm
// NOTE: need to be a lowercase RFC 1123 label
func hugepageNameBySizeInBytes(sizeInBytes uint64) string {
	sizeInKB := sizeInBytes / 1024
	if sizeInKB == 2048 {
		return "hugepages-2m"
	}
	sizeInMB := sizeInKB / 1024
	if sizeInMB == 1024 {
		return "hugepages-1g"
	}
	return fmt.Sprintf("hugepages-%dk", sizeInKB) // should never happen
}
