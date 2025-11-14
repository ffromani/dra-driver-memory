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
	"sort"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/google/go-cmp/cmp"
	ghwmemory "github.com/jaypipes/ghw/pkg/memory"

	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/dynamic-resource-allocation/resourceslice"
	"k8s.io/utils/ptr"
)

func TestProcess(t *testing.T) {
	type testcase struct {
		name           string
		machine        MachineData
		makeDeviceName func(string) string
		expectedSlices []resourceslice.Slice
	}

	testcases := []testcase{
		{
			name: "single NUMA-node",
			machine: MachineData{
				Pagesize: 4096,
				Zones: []Zone{
					{
						ID:        0,
						Distances: []int{10},
						Memory: &ghwmemory.Area{
							TotalPhysicalBytes: 34225520640,
							TotalUsableBytes:   33332322304,
							SupportedPageSizes: []uint64{
								1073741824,
								2097152,
							},
							DefaultHugePageSize: 2097152,
							HugePageAmountsBySize: map[uint64]*ghwmemory.HugePageAmounts{
								1073741824: {},
								2097152:    {},
							},
						},
					},
				},
			},
			makeDeviceName: func(devName string) string {
				return devName + "-XXXXXX"
			},
			expectedSlices: []resourceslice.Slice{
				{
					Devices: []resourceapi.Device{
						{
							Name: "hugepages-1g-XXXXXX",
							Attributes: makeAttributes(attrInfo{
								numaNode: 0,
								sizeName: "1g",
								hugeTLB:  true,
							}),
							Capacity: map[resourceapi.QualifiedName]resourceapi.DeviceCapacity{
								"pages": {
									Value: *resource.NewQuantity(0, resource.DecimalSI),
								},
							},
							AllowMultipleAllocations: ptr.To(true),
						},
					},
				},
				{
					Devices: []resourceapi.Device{
						{
							Name: "hugepages-2m-XXXXXX",
							Attributes: makeAttributes(attrInfo{
								numaNode: 0,
								sizeName: "2m",
								hugeTLB:  true,
							}),
							Capacity: map[resourceapi.QualifiedName]resourceapi.DeviceCapacity{
								"pages": {
									Value: *resource.NewQuantity(0, resource.DecimalSI),
								},
							},
							AllowMultipleAllocations: ptr.To(true),
						},
					},
				},
				{
					Devices: []resourceapi.Device{
						{
							Name: "memory-XXXXXX",
							Attributes: makeAttributes(attrInfo{
								numaNode: 0,
								sizeName: "4k",
								hugeTLB:  false,
							}),
							Capacity: map[resourceapi.QualifiedName]resourceapi.DeviceCapacity{
								"memory": {
									Value: *resource.NewQuantity(33332322304, resource.DecimalSI),
								},
							},
							AllowMultipleAllocations: ptr.To(true),
						},
					},
				},
			},
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			saveMakeDeviceName := MakeDeviceName
			t.Cleanup(func() {
				MakeDeviceName = saveMakeDeviceName
			})
			MakeDeviceName = tcase.makeDeviceName

			logger := testr.New(t)
			rinfo := Process(logger, tcase.machine)

			gotSlices := rinfo.GetResourceSlices()
			// CRITICAL NOTE: this is deeply tied to the layout of the resource.
			// at the same time there's no need to sort every time in `GetResourceSlices`,
			// let alone add a sorted variant. So looks like this is the lesser evil.
			sort.Slice(gotSlices, func(i, j int) bool {
				return gotSlices[i].Devices[0].Name < gotSlices[j].Devices[0].Name
			})
			if diff := cmp.Diff(gotSlices, tcase.expectedSlices); diff != "" {
				t.Errorf("unexpected resourceslice: %s", diff)
			}
		})
	}
}

type attrInfo struct {
	numaNode int64
	sizeName string
	hugeTLB  bool
}

func makeAttributes(info attrInfo) map[resourceapi.QualifiedName]resourceapi.DeviceAttribute {
	pNode := ptr.To(info.numaNode)
	return map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
		// alignment compatibility: dra-driver-sriov
		"resource.kubernetes.io/numaNode": {IntValue: pNode},
		// alignment compatibility: dra-driver-cpu
		"dra.cpu/numaNode": {IntValue: pNode},
		// alignment compatibility: dranet
		"dra.net/numaNode": {IntValue: pNode},
		// our own attributes, at last
		"dra.memory/numaNode": {IntValue: pNode},
		"dra.memory/pageSize": {StringValue: ptr.To(info.sizeName)},
		"dra.memory/hugeTLB":  {BoolValue: ptr.To(info.hugeTLB)},
	}
}
