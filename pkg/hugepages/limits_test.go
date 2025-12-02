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

package hugepages

import (
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/google/go-cmp/cmp"

	"github.com/ffromani/dra-driver-memory/pkg/sysinfo"
	"github.com/ffromani/dra-driver-memory/pkg/types"
)

func TestLimitsFromAllocation(t *testing.T) {
	machineDataX86 := sysinfo.MachineData{
		Hugepagesizes: []uint64{
			2 * (1 << 20),
			(1 << 30),
		},
	}

	type testcase struct {
		description string
		machineData sysinfo.MachineData
		allocs      []types.Allocation
		expected    []Limit
	}

	testcases := []testcase{
		{
			description: "no allocs",
			machineData: machineDataX86,
			allocs:      []types.Allocation{},
			expected: []Limit{
				{
					PageSize: "2MB",
					Limit: LimitValue{
						Value: 0,
					},
				},
				{
					PageSize: "1GB",
					Limit: LimitValue{
						Value: 0,
					},
				},
			},
		},
		{
			description: "simple hugepages-2m",
			machineData: machineDataX86,
			allocs: []types.Allocation{
				{
					ResourceIdent: types.ResourceIdent{
						Kind:     types.Hugepages,
						Pagesize: 2 * (1 << 20),
					},
					Amount:   128 * 2 * (1 << 20),
					NUMAZone: 1,
				},
			},
			expected: []Limit{
				{
					PageSize: "2MB",
					Limit: LimitValue{
						Value: 128 * 2 * (1 << 20),
					},
				},
				{
					PageSize: "1GB",
					Limit: LimitValue{
						Value: 0,
					},
				},
			},
		},
		{
			description: "simple hugepages-1g",
			machineData: machineDataX86,
			allocs: []types.Allocation{
				{
					ResourceIdent: types.ResourceIdent{
						Kind:     types.Hugepages,
						Pagesize: (1 << 30),
					},
					Amount:   4 * (1 << 30),
					NUMAZone: 1,
				},
			},
			expected: []Limit{
				{
					PageSize: "2MB",
					Limit: LimitValue{
						Value: 0,
					},
				},
				{
					PageSize: "1GB",
					Limit: LimitValue{
						Value: 4 * (1 << 30),
					},
				},
			},
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.description, func(t *testing.T) {
			logger := testr.New(t)
			got := LimitsFromAllocations(logger, tcase.machineData, tcase.allocs)
			if diff := cmp.Diff(got, tcase.expected); diff != "" {
				t.Errorf("limits are different: %s", diff)
			}
		})
	}
}
