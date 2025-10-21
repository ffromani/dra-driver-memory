/*
 * Copyright 2025 The Kubernetes Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package sysinfo

import (
	"fmt"
	"slices"

	"github.com/go-logr/logr"
	ghwmemory "github.com/jaypipes/ghw/pkg/memory"
	ghwtopology "github.com/jaypipes/ghw/pkg/topology"

	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	k8srand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/dynamic-resource-allocation/resourceslice"
	"k8s.io/utils/ptr"
)

type Zone struct {
	ID        int             `json:"id"`
	Distances []int           `json:"distances"`
	Memory    *ghwmemory.Area `json:"memory"`
}

func FromNodes(nodes []*ghwtopology.Node) []Zone {
	zones := make([]Zone, 0, len(nodes))
	for _, node := range nodes {
		zones = append(zones, Zone{
			ID:        node.ID,
			Distances: node.Distances,
			Memory:    node.Memory,
		})
	}
	return zones
}

type MachineData struct {
	Pagesize int    `json:"pagesize"`
	Zones    []Zone `json:"zones"`
}

// enables testing
var MakeDeviceName = func(devName string, _ int64) string {
	return devName + "-" + k8srand.String(6)
}

// Process processes MachineData and creates resource slices out of it, plus a device:numaNode mapping.
// This function cannot really fail and never returns invalid data but it can return empty data.
func Process(lh logr.Logger, machine MachineData) ([]resourceslice.Slice, map[string]int64) {
	deviceNameToNUMANode := make(map[string]int64)
	memorySlice := resourceslice.Slice{}
	hugepageSlice := resourceslice.Slice{}

	for numaNode, nodeInfo := range machine.Zones {
		if nodeInfo.Memory == nil {
			lh.V(2).Info("NUMA node %d reports no memory", numaNode)
			continue
		}
		numaNode := int64(numaNode)

		memQty := resource.NewQuantity(nodeInfo.Memory.TotalUsableBytes, resource.DecimalSI)
		memDevice := resourceapi.Device{
			Name: MakeDeviceName("memory", numaNode),
			Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
				"dra.memory/numaNode": {IntValue: ptr.To(numaNode)},
				"dra.cpu/numaNode":    {IntValue: ptr.To(numaNode)},
				"dra.net/numaNode":    {IntValue: ptr.To(numaNode)},
			},
			Capacity: map[resourceapi.QualifiedName]resourceapi.DeviceCapacity{
				"memory": {
					Value: *memQty,
				},
			},
			AllowMultipleAllocations: ptr.To(true),
		}
		memorySlice.Devices = append(memorySlice.Devices, memDevice)
		deviceNameToNUMANode[memDevice.Name] = numaNode

		var sizeInBytes []uint64
		for sz := range nodeInfo.Memory.HugePageAmountsBySize {
			sizeInBytes = append(sizeInBytes, sz)
		}
		slices.Sort(sizeInBytes)

		for _, hpSize := range sizeInBytes {
			amounts := nodeInfo.Memory.HugePageAmountsBySize[hpSize]
			hpBasename := hugepageNameBySizeInBytes(hpSize)
			hpQty := resource.NewQuantity(amounts.Total, resource.DecimalSI)
			hpDevice := resourceapi.Device{
				Name: MakeDeviceName(hpBasename, numaNode),
				Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
					"dra.memory/numaNode": {IntValue: ptr.To(numaNode)},
					"dra.cpu/numaNode":    {IntValue: ptr.To(numaNode)},
					"dra.net/numaNode":    {IntValue: ptr.To(numaNode)},
				},
				Capacity: map[resourceapi.QualifiedName]resourceapi.DeviceCapacity{
					"pages": {
						Value: *hpQty,
					},
				},
				AllowMultipleAllocations: ptr.To(true),
			}
			hugepageSlice.Devices = append(hugepageSlice.Devices, hpDevice)
			deviceNameToNUMANode[hpDevice.Name] = numaNode
		}
	}

	if lh.V(4).Enabled() {
		for devName, numaNode := range deviceNameToNUMANode {
			lh.V(4).Info("Devices mapping", "device", devName, "NUMANode", numaNode)
		}
	}
	return []resourceslice.Slice{
		memorySlice,
		hugepageSlice,
	}, deviceNameToNUMANode
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
