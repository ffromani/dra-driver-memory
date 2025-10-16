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

package ghwinfo

import (
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/google/go-cmp/cmp"
	ghwmemory "github.com/jaypipes/ghw/pkg/memory"
	ghwtopology "github.com/jaypipes/ghw/pkg/topology"

	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/dynamic-resource-allocation/resourceslice"
	"k8s.io/utils/ptr"
)

func TestDiscover(t *testing.T) {
	type testcase struct {
		name              string
		topo              *ghwtopology.Info
		makeDeviceName    func(string, int64) string
		expectedSlices    []resourceslice.Slice
		expectedDevToNode map[string]int64
	}

	testcases := []testcase{
		{
			name: "single NUMA-node",
			topo: &ghwtopology.Info{
				Architecture: ghwtopology.ArchitectureSMP, // ghw quirk
				Nodes: []*ghwtopology.Node{
					{
						ID: 0,
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
			makeDeviceName: func(devName string, _ int64) string {
				return devName + "-XXXXXX"
			},
			expectedSlices: []resourceslice.Slice{
				{
					Devices: []resourceapi.Device{
						{
							Name:       "memory-XXXXXX",
							Attributes: attributesForNUMANode(int64(0)),
							Capacity: map[resourceapi.QualifiedName]resourceapi.DeviceCapacity{
								"memory": {
									Value: *resource.NewQuantity(33332322304, resource.DecimalSI),
								},
							},
							AllowMultipleAllocations: ptr.To(true),
						},
					},
				},
				{
					Devices: []resourceapi.Device{
						{
							Name:       "hugepages-2m-XXXXXX",
							Attributes: attributesForNUMANode(int64(0)),
							Capacity: map[resourceapi.QualifiedName]resourceapi.DeviceCapacity{
								"pages": {
									Value: *resource.NewQuantity(0, resource.DecimalSI),
								},
							},
							AllowMultipleAllocations: ptr.To(true),
						},
						{
							Name:       "hugepages-1g-XXXXXX",
							Attributes: attributesForNUMANode(int64(0)),
							Capacity: map[resourceapi.QualifiedName]resourceapi.DeviceCapacity{
								"pages": {
									Value: *resource.NewQuantity(0, resource.DecimalSI),
								},
							},
							AllowMultipleAllocations: ptr.To(true),
						},
					},
				},
			},
			expectedDevToNode: map[string]int64{
				"hugepages-1g-XXXXXX": 0,
				"hugepages-2m-XXXXXX": 0,
				"memory-XXXXXX":       0,
			},
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			saveMakeDeviceName := MakeDeviceName
			defer func() {
				MakeDeviceName = saveMakeDeviceName
			}()
			MakeDeviceName = tcase.makeDeviceName

			logger := testr.New(t)
			gotSlices, gotMap := Discover(logger, tcase.topo)

			if diff := cmp.Diff(gotSlices, tcase.expectedSlices); diff != "" {
				t.Errorf("unexpected resourceslice: %s", diff)
			}
			if diff := cmp.Diff(gotMap, tcase.expectedDevToNode); diff != "" {
				t.Errorf("unexpected deviceToNode mapping: %s", diff)
			}
		})
	}
}

func attributesForNUMANode(numaNode int64) map[resourceapi.QualifiedName]resourceapi.DeviceAttribute {
	return map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
		"dra.cpu/numaNode":    {IntValue: ptr.To(numaNode)},
		"dra.memory/numaNode": {IntValue: ptr.To(numaNode)},
		"dra.net/numaNode":    {IntValue: ptr.To(numaNode)},
	}
}
