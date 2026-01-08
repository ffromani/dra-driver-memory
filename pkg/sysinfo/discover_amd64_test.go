//go:build amd64

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
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/google/go-cmp/cmp"
	ghwmemory "github.com/jaypipes/ghw/pkg/memory"
	"github.com/stretchr/testify/require"

	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/dynamic-resource-allocation/resourceslice"
	"k8s.io/utils/ptr"

	"github.com/ffromani/dra-driver-memory/pkg/types"
)

func TestRefreshWithData(t *testing.T) {
	type testcase struct {
		name             string
		machine          MachineData
		expectedResNames []string
		expectedSlices   []resourceslice.Slice
	}

	testcases := []testcase{
		{
			name: "single NUMA-node, no hugepages",
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
						},
					},
				},
			},
			expectedResNames: []string{"memory"},
			expectedSlices: []resourceslice.Slice{
				{
					Devices: []resourceapi.Device{
						{
							Name: "memory-XXXXXX",
							Attributes: makeAttributes(attrInfo{
								numaNode: 0,
								sizeName: "4Ki",
								hugeTLB:  false,
							}),
							Capacity: map[resourceapi.QualifiedName]resourceapi.DeviceCapacity{
								"size": {
									Value: *resource.NewQuantity(33332322304, resource.BinarySI),
									RequestPolicy: &resourceapi.CapacityRequestPolicy{
										Default: resource.NewQuantity(1<<20, resource.BinarySI),
										ValidRange: &resourceapi.CapacityRequestPolicyRange{
											Min:  resource.NewQuantity(4*1<<10, resource.BinarySI),
											Max:  resource.NewQuantity(33332322304, resource.BinarySI),
											Step: resource.NewQuantity(4*1<<10, resource.BinarySI),
										},
									},
								},
							},
							AllowMultipleAllocations: ptr.To(true),
						},
					},
				},
			},
		},
		{
			name: "single NUMA-node, with hugepages",
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
								1073741824: {
									Total: 8,
								},
								2097152: {
									Total: 2048,
								},
							},
						},
					},
				},
			},
			expectedResNames: []string{"hugepages-1Gi", "hugepages-2Mi", "memory"},
			expectedSlices: []resourceslice.Slice{
				{
					Devices: []resourceapi.Device{
						{
							Name: "hugepages-1gi-XXXXXX",
							Attributes: makeAttributes(attrInfo{
								numaNode: 0,
								sizeName: "1Gi",
								hugeTLB:  true,
							}),
							Capacity: map[resourceapi.QualifiedName]resourceapi.DeviceCapacity{
								"size": {
									Value: *resource.NewQuantity(8589934592, resource.BinarySI),
									RequestPolicy: &resourceapi.CapacityRequestPolicy{
										Default: resource.NewQuantity(1<<30, resource.BinarySI),
										ValidRange: &resourceapi.CapacityRequestPolicyRange{
											Min:  resource.NewQuantity(1<<30, resource.BinarySI),
											Max:  resource.NewQuantity(8589934592, resource.BinarySI),
											Step: resource.NewQuantity(1<<30, resource.BinarySI),
										},
									},
								},
							},
							AllowMultipleAllocations: ptr.To(true),
						},
					},
				},
				{
					Devices: []resourceapi.Device{
						{
							Name: "hugepages-2mi-XXXXXX",
							Attributes: makeAttributes(attrInfo{
								numaNode: 0,
								sizeName: "2Mi",
								hugeTLB:  true,
							}),
							Capacity: map[resourceapi.QualifiedName]resourceapi.DeviceCapacity{
								"size": {
									Value: *resource.NewQuantity(4294967296, resource.BinarySI),
									RequestPolicy: &resourceapi.CapacityRequestPolicy{
										Default: resource.NewQuantity(2*1<<20, resource.BinarySI),
										ValidRange: &resourceapi.CapacityRequestPolicyRange{
											Min:  resource.NewQuantity(2*1<<20, resource.BinarySI),
											Max:  resource.NewQuantity(4294967296, resource.BinarySI),
											Step: resource.NewQuantity(2*1<<20, resource.BinarySI),
										},
									},
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
								sizeName: "4Ki",
								hugeTLB:  false,
							}),
							Capacity: map[resourceapi.QualifiedName]resourceapi.DeviceCapacity{
								"size": {
									Value: *resource.NewQuantity(33332322304, resource.BinarySI),
									RequestPolicy: &resourceapi.CapacityRequestPolicy{
										Default: resource.NewQuantity(1<<20, resource.BinarySI),
										ValidRange: &resourceapi.CapacityRequestPolicyRange{
											Min:  resource.NewQuantity(4*1<<10, resource.BinarySI),
											Max:  resource.NewQuantity(33332322304, resource.BinarySI),
											Step: resource.NewQuantity(4*1<<10, resource.BinarySI),
										},
									},
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
			fakeSysRoot := t.TempDir()

			saveMakeDeviceName := MakeDeviceName
			t.Cleanup(func() {
				MakeDeviceName = saveMakeDeviceName
			})
			MakeDeviceName = makeTestDeviceName

			logger := testr.New(t)

			disc := NewDiscoverer(fakeSysRoot) // not really needed, but let's be clean
			disc.GetMachineData = func(_ logr.Logger, _ string) (MachineData, error) {
				return tcase.machine, nil
			}
			err := disc.Refresh(logger)
			require.NoError(t, err)
			gotMachineData := disc.GetCachedMachineData()
			if diff := cmp.Diff(gotMachineData, tcase.machine); diff != "" {
				t.Fatalf("unexpected fetched machinedata: %v", diff)
			}

			gotResNames := sets.List(disc.AllResourceNames())
			if diff := cmp.Diff(gotResNames, tcase.expectedResNames); diff != "" {
				t.Errorf("unexpected resource names: %s", diff)
			}

			gotSlices := disc.ResourceSlices()
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

func TestGetFreshMachineData(t *testing.T) {
	fakeSysRoot := t.TempDir()
	logger := testr.New(t)

	expectedMachine := MachineData{
		Pagesize: 4096,
		Zones: []Zone{
			{
				ID:        0,
				Distances: []int{10},
			},
		},
	}

	disc := NewDiscoverer(fakeSysRoot)
	disc.GetMachineData = func(_ logr.Logger, _ string) (MachineData, error) {
		return expectedMachine, nil
	}

	gotMachine, err := disc.GetFreshMachineData(logger)
	require.NoError(t, err)
	if diff := cmp.Diff(gotMachine, expectedMachine); diff != "" {
		t.Fatalf("unexpected machine data: %v", diff)
	}
}

func TestGetSpanForDeviceNotFound(t *testing.T) {
	fakeSysRoot := t.TempDir()
	logger := testr.New(t)

	disc := NewDiscoverer(fakeSysRoot)
	disc.GetMachineData = func(_ logr.Logger, _ string) (MachineData, error) {
		return MachineData{}, nil
	}
	err := disc.Refresh(logger)
	require.NoError(t, err)

	_, err = disc.GetSpanForDevice(logger, "nonexistent-device")
	require.Error(t, err)
}

func TestGetSpanForDevice(t *testing.T) {
	type testcase struct {
		name     string
		machine  MachineData
		devName  string
		expected types.Span
	}

	testcases := []testcase{
		{
			name: "single NUMA-node, no hugepages",
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
						},
					},
				},
			},
			devName: "memory-XXXXXX",
			expected: types.Span{
				ResourceIdent: types.ResourceIdent{
					Kind:     types.Memory,
					Pagesize: 4096,
				},
				Amount:   int64(33332322304),
				NUMAZone: 0,
			},
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			fakeSysRoot := t.TempDir()

			saveMakeDeviceName := MakeDeviceName
			t.Cleanup(func() {
				MakeDeviceName = saveMakeDeviceName
			})
			MakeDeviceName = makeTestDeviceName

			logger := testr.New(t)

			disc := NewDiscoverer(fakeSysRoot) // not really needed, but let's be clean
			disc.GetMachineData = func(_ logr.Logger, _ string) (MachineData, error) {
				return tcase.machine, nil
			}
			err := disc.Refresh(logger)
			require.NoError(t, err)

			span, err := disc.GetSpanForDevice(logger, tcase.devName)
			require.NoError(t, err)
			if diff := cmp.Diff(span, tcase.expected); diff != "" {
				t.Fatalf("unexpected span: %v", diff)
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
		"resource.kubernetes.io/numaNode": {IntValue: pNode},
		"resource.kubernetes.io/pageSize": {StringValue: ptr.To(info.sizeName)},
		"resource.kubernetes.io/hugeTLB":  {BoolValue: ptr.To(info.hugeTLB)},
		"dra.cpu/numaNode":                {IntValue: pNode},
		"dra.net/numaNode":                {IntValue: pNode},
	}
}

func makeTestDeviceName(devName string) string {
	return strings.ToLower(devName) + "-XXXXXX"
}
