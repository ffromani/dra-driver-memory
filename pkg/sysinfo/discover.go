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
	"maps"
	"os"
	"slices"

	"github.com/go-logr/logr"
	ghwmemory "github.com/jaypipes/ghw/pkg/memory"
	ghwopt "github.com/jaypipes/ghw/pkg/option"
	ghwtopology "github.com/jaypipes/ghw/pkg/topology"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/dynamic-resource-allocation/resourceslice"

	"github.com/ffromani/dra-driver-memory/pkg/types"
	"github.com/ffromani/dra-driver-memory/pkg/unitconv"
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

func GetMachineData(lh logr.Logger, sysRoot string) (MachineData, error) {
	topo, err := ghwtopology.New(ghwopt.WithChroot(sysRoot))
	if err != nil {
		return MachineData{}, err
	}
	var Hugepagesizes []uint64
	for _, pageSize := range HugepageSizes(lh, sysRoot) {
		sz, err := unitconv.CGroupStringToSizeInBytes(pageSize)
		if err != nil {
			lh.Error(err, "getting system huge page size")
			continue
		}
		Hugepagesizes = append(Hugepagesizes, sz)
	}
	return MachineData{
		Pagesize:      uint64(os.Getpagesize()),
		Hugepagesizes: Hugepagesizes,
		Zones:         FromNodes(topo.Nodes),
	}, nil
}

type Discoverer struct {
	sysRoot            string
	machineData        MachineData
	resourceNames      sets.Set[string]
	spanByDeviceName   map[string]types.Span
	deviceTypeToSlices map[string]resourceslice.Slice
}

func NewDiscoverer(sysRoot string) *Discoverer {
	ds := &Discoverer{
		sysRoot: sysRoot,
	}
	ds.reset()
	return ds
}

func (ds *Discoverer) AllResourceNames() sets.Set[string] {
	return ds.resourceNames.Clone()
}

func (ds *Discoverer) GetMachineData() MachineData {
	return ds.machineData
}

func (ds *Discoverer) GetSpanForDevice(lh logr.Logger, devName string) (types.Span, error) {
	span, ok := ds.spanByDeviceName[devName]
	if !ok {
		return types.Span{}, fmt.Errorf("device %q not matches any registered memory span", devName)
	}
	lh.V(4).Info("device span", "devName", devName, "span", span.String())
	return span, nil
}

func (ds *Discoverer) Refresh(lh logr.Logger) error {
	machineData, err := GetMachineData(lh, ds.sysRoot)
	if err != nil {
		return err
	}
	ds.reset()
	ds.processMachine(lh, machineData)
	ds.machineData = machineData
	ds.logMachine(lh)
	return nil
}

func (ds *Discoverer) ResourceSlices() []resourceslice.Slice {
	return slices.Collect(maps.Values(ds.deviceTypeToSlices))
}

func (ds *Discoverer) reset() {
	ds.resourceNames = sets.New[string]()
	ds.spanByDeviceName = make(map[string]types.Span)
	ds.deviceTypeToSlices = make(map[string]resourceslice.Slice)
}

// processMachine receives MachineData and creates resource slices out of it, plus a device:numaNode mapping.
// This function cannot really fail and never returns invalid data but it can return empty data.
func (ds *Discoverer) processMachine(lh logr.Logger, machine MachineData) {
	for numaNode, nodeInfo := range machine.Zones {
		if nodeInfo.Memory == nil {
			lh.V(2).Info("NUMA node %d reports no memory", numaNode)
			continue
		}
		ds.processMemory(lh, machine.Pagesize, int64(numaNode), nodeInfo)
		for _, hpSize := range sortedHugepageSizes(nodeInfo) {
			ds.processHugepages(lh, hpSize, int64(numaNode), nodeInfo)
		}
	}
}

func sortedHugepageSizes(nodeInfo Zone) []uint64 {
	var sizeInBytes []uint64
	for sz := range nodeInfo.Memory.HugePageAmountsBySize {
		sizeInBytes = append(sizeInBytes, sz)
	}
	slices.Sort(sizeInBytes)
	return sizeInBytes
}

func (ds *Discoverer) processMemory(lh logr.Logger, pageSize uint64, numaNode int64, nodeInfo Zone) {
	span := types.Span{
		ResourceIdent: types.ResourceIdent{
			Kind:     types.Memory,
			Pagesize: pageSize,
		},
		Amount:   nodeInfo.Memory.TotalUsableBytes,
		NUMAZone: numaNode,
	}
	memDevice := ToDevice(span)
	ds.spanByDeviceName[memDevice.Name] = span
	memorySlice := ds.deviceTypeToSlices[span.Name()]
	memorySlice.Devices = append(memorySlice.Devices, memDevice)
	ds.deviceTypeToSlices[span.Name()] = memorySlice
}

func (ds *Discoverer) processHugepages(lh logr.Logger, hpSize uint64, numaNode int64, nodeInfo Zone) {
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
	ds.spanByDeviceName[hpDevice.Name] = span
	hugepageSlice := ds.deviceTypeToSlices[span.Name()]
	hugepageSlice.Devices = append(hugepageSlice.Devices, hpDevice)
	ds.deviceTypeToSlices[span.Name()] = hugepageSlice
}

func (ds *Discoverer) logMachine(lh logr.Logger) {
	if !lh.V(4).Enabled() {
		return
	}
	for devName, devSpan := range ds.spanByDeviceName {
		lh.V(4).Info("Devices mapping", "device", devName, "deviceType", devSpan.Name(), "NUMANode", devSpan.NUMAZone)
	}
}
