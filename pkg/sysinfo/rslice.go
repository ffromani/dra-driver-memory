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
	"maps"
	"slices"

	"github.com/go-logr/logr"
	ghwmemory "github.com/jaypipes/ghw/pkg/memory"
	ghwtopology "github.com/jaypipes/ghw/pkg/topology"

	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	k8srand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/dynamic-resource-allocation/resourceslice"
	"k8s.io/utils/ptr"

	"github.com/ffromani/dra-driver-memory/pkg/types"
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
	Pagesize      uint64   `json:"page_size"`
	Hugepagesizes []uint64 `json:"huge_page_sizes"`
	Zones         []Zone   `json:"zones"`
}

type ResourceInfo struct {
	spanByDeviceName   map[string]types.Span
	deviceTypeToSlices map[string]resourceslice.Slice
}

func (ri ResourceInfo) GetResourceSlices() []resourceslice.Slice {
	return slices.Collect(maps.Values(ri.deviceTypeToSlices))
}

func (ri ResourceInfo) GetSpanByDeviceName() map[string]types.Span {
	return ri.spanByDeviceName
}

func (ri ResourceInfo) GetResourceNames() sets.Set[string] {
	resourceNames := sets.New[string]()
	for span := range maps.Values(ri.spanByDeviceName) {
		resourceNames.Insert(span.Name())
	}
	return resourceNames
}

// Process processes MachineData and creates resource slices out of it, plus a device:numaNode mapping.
// This function cannot really fail and never returns invalid data but it can return empty data.
func Process(lh logr.Logger, machine MachineData) ResourceInfo {
	info := ResourceInfo{
		spanByDeviceName:   make(map[string]types.Span),
		deviceTypeToSlices: make(map[string]resourceslice.Slice),
	}

	for numaNode, nodeInfo := range machine.Zones {
		if nodeInfo.Memory == nil {
			lh.V(2).Info("NUMA node %d reports no memory", numaNode)
			continue
		}
		processMemory(lh, &info, machine.Pagesize, int64(numaNode), nodeInfo)
		for _, hpSize := range sortedHugepageSizes(nodeInfo) {
			processHugepages(lh, &info, hpSize, int64(numaNode), nodeInfo)
		}
	}

	if lh.V(4).Enabled() {
		for devName, devSpan := range info.spanByDeviceName {
			lh.V(4).Info("Devices mapping", "device", devName, "deviceType", devSpan.Name(), "NUMANode", devSpan.NUMAZone)
		}
	}
	return info
}

func sortedHugepageSizes(nodeInfo Zone) []uint64 {
	var sizeInBytes []uint64
	for sz := range nodeInfo.Memory.HugePageAmountsBySize {
		sizeInBytes = append(sizeInBytes, sz)
	}
	slices.Sort(sizeInBytes)
	return sizeInBytes
}

func processMemory(lh logr.Logger, info *ResourceInfo, pageSize uint64, numaNode int64, nodeInfo Zone) {
	span := types.Span{
		ResourceIdent: types.ResourceIdent{
			Kind:     types.Memory,
			Pagesize: pageSize,
		},
		Amount:   nodeInfo.Memory.TotalUsableBytes,
		NUMAZone: numaNode,
	}
	memDevice := ToDevice(span)
	info.spanByDeviceName[memDevice.Name] = span
	memorySlice := info.deviceTypeToSlices[span.Name()]
	memorySlice.Devices = append(memorySlice.Devices, memDevice)
	info.deviceTypeToSlices[span.Name()] = memorySlice
}

func processHugepages(lh logr.Logger, info *ResourceInfo, hpSize uint64, numaNode int64, nodeInfo Zone) {
	amounts := nodeInfo.Memory.HugePageAmountsBySize[hpSize]
	span := types.Span{
		ResourceIdent: types.ResourceIdent{
			Kind:     types.Hugepages,
			Pagesize: hpSize,
		},
		Amount:   amounts.Total,
		NUMAZone: numaNode,
	}
	hpDevice := ToDevice(span)
	info.spanByDeviceName[hpDevice.Name] = span
	hugepageSlice := info.deviceTypeToSlices[span.Name()]
	hugepageSlice.Devices = append(hugepageSlice.Devices, hpDevice)
	info.deviceTypeToSlices[span.Name()] = hugepageSlice
}

func MakeAttributes(sp types.Span) map[resourceapi.QualifiedName]resourceapi.DeviceAttribute {
	pNode := ptr.To(sp.NUMAZone)
	return map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
		// alignment compatibility: dra-driver-sriov
		"resource.kubernetes.io/numaNode": {IntValue: pNode},
		// alignment compatibility: dra-driver-cpu
		"dra.cpu/numaNode": {IntValue: pNode},
		// alignment compatibility: dranet
		"dra.net/numaNode": {IntValue: pNode},
		// our own attributes, at last
		"dra.memory/numaNode": {IntValue: pNode},
		"dra.memory/pageSize": {StringValue: ptr.To(sp.PagesizeString())},
		"dra.memory/hugeTLB":  {BoolValue: ptr.To(sp.NeedsHugeTLB())},
	}
}

func MakeCapacity(sp types.Span) map[resourceapi.QualifiedName]resourceapi.DeviceCapacity {
	name := sp.CapacityName()
	qty := resource.NewQuantity(sp.Amount, resource.DecimalSI)
	return map[resourceapi.QualifiedName]resourceapi.DeviceCapacity{
		name: {
			Value: *qty,
		},
	}
}

func ToDevice(sp types.Span) resourceapi.Device {
	return resourceapi.Device{
		Name:                     MakeDeviceName(sp.Name()),
		Attributes:               MakeAttributes(sp),
		Capacity:                 MakeCapacity(sp),
		AllowMultipleAllocations: ptr.To(true),
	}
}

// MakeDeviceName creates a unique short device name from the base device name ("memory", "hugepages-2m")
// We use a random part because the device name is not really that relevant, as long as it's unique.
// We can very much construct it concatenating nodeName and NUMAZoneID, that would be unique and equally
// valid as we expose plenty of low-level details like the NUMAZoneID anyway, but the concern is that
// we would need more validation, e.g, translating the nodeName (dots->dashes) and so on.
// Since users are expected to select memory devices by attribute and not by name, we just use a
// random suffix for the time being and move on.
var MakeDeviceName = func(devName string) string {
	return devName + "-" + k8srand.String(6)
}
