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

func TestAddLimitValue(t *testing.T) {
	type testcase struct {
		name     string
		ref      LimitValue
		op       LimitValue
		expected LimitValue
	}

	testcases := []testcase{
		{
			name: "zero value",
		},
		{
			name: "unset adding unset",
			ref: LimitValue{
				Unset: true,
			},
			op: LimitValue{
				Unset: true,
			},
			expected: LimitValue{
				Unset: true,
			},
		},
		{
			name: "unset adding set",
			ref: LimitValue{
				Unset: true,
				Value: 8, // impossible, but to prove the point
			},
			op: LimitValue{
				Value: 32,
			},
			expected: LimitValue{
				Value: 32,
			},
		},
		{
			name: "set adding unset",
			ref: LimitValue{
				Value: 48,
			},
			op: LimitValue{
				Unset: true,
				Value: 32, // impossible, but to prove the point
			},
			expected: LimitValue{
				Value: 48,
			},
		},
		{
			name: "set adding set",
			ref: LimitValue{
				Value: 48,
			},
			op: LimitValue{
				Value: 32,
			},
			expected: LimitValue{
				Value: 80,
			},
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			got := tcase.ref.Add(tcase.op)
			if diff := cmp.Diff(got, tcase.expected); diff != "" {
				t.Errorf("unexpected diff=%v", diff)
			}
		})
	}
}

func TestSumLimits(t *testing.T) {
	type testcase struct {
		name     string
		lla      []Limit
		llb      []Limit
		expected []Limit
	}

	testcases := []testcase{
		{
			name:     "all empty",
			expected: nil,
		},
		{
			name: "total overlap",
			lla: []Limit{
				{
					PageSize: "2MB",
					Limit: LimitValue{
						Value: 4 * (1 << 21),
					},
				},
				{
					PageSize: "1GB",
					Limit: LimitValue{
						Value: 2 * (1 << 30),
					},
				},
			},
			llb: []Limit{
				{
					PageSize: "2MB",
					Limit: LimitValue{
						Value: 1 * (1 << 21),
					},
				},
				{
					PageSize: "1GB",
					Limit: LimitValue{
						Value: 1 * (1 << 30),
					},
				},
			},
			expected: []Limit{
				{
					PageSize: "2MB",
					Limit: LimitValue{
						Value: 5 * (1 << 21),
					},
				},
				{
					PageSize: "1GB",
					Limit: LimitValue{
						Value: 3 * (1 << 30),
					},
				},
			},
		},
		{
			name: "partial overlap 2MB",
			lla: []Limit{
				{
					PageSize: "2MB",
					Limit: LimitValue{
						Value: 4 * (1 << 21),
					},
				},
				{
					PageSize: "1GB",
					Limit: LimitValue{
						Value: 2 * (1 << 30),
					},
				},
			},
			llb: []Limit{
				{
					PageSize: "2MB",
					Limit: LimitValue{
						Value: 4 * (1 << 21),
					},
				},
			},
			expected: []Limit{
				{
					PageSize: "2MB",
					Limit: LimitValue{
						Value: 8 * (1 << 21),
					},
				},
				{
					PageSize: "1GB",
					Limit: LimitValue{
						Value: 2 * (1 << 30),
					},
				},
			},
		},
		{
			name: "partial overlap 1GB",
			lla: []Limit{
				{
					PageSize: "2MB",
					Limit: LimitValue{
						Value: 4 * (1 << 21),
					},
				},
				{
					PageSize: "1GB",
					Limit: LimitValue{
						Value: 2 * (1 << 30),
					},
				},
			},
			llb: []Limit{
				{
					PageSize: "1GB",
					Limit: LimitValue{
						Value: 2 * (1 << 30),
					},
				},
			},
			expected: []Limit{
				{
					PageSize: "2MB",
					Limit: LimitValue{
						Value: 4 * (1 << 21),
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
		{
			name: "no overlap",
			lla: []Limit{
				{
					PageSize: "2MB",
					Limit: LimitValue{
						Value: 4 * (1 << 21),
					},
				},
			},
			llb: []Limit{
				{
					PageSize: "1GB",
					Limit: LimitValue{
						Value: 2 * (1 << 30),
					},
				},
			},
			expected: []Limit{
				{
					PageSize: "2MB",
					Limit: LimitValue{
						Value: 4 * (1 << 21),
					},
				},
				{
					PageSize: "1GB",
					Limit: LimitValue{
						Value: 2 * (1 << 30),
					},
				},
			},
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			got := SumLimits(tcase.lla, tcase.llb)
			if diff := cmp.Diff(got, tcase.expected); diff != "" {
				t.Errorf("sum is different: %s", diff)
			}
		})
	}
}

func TestLimitString(t *testing.T) {
	type testcase struct {
		name     string
		limit    Limit
		expected string
	}

	testcases := []testcase{
		{
			name:     "zero value",
			limit:    Limit{},
			expected: "",
		},
		{
			name: "value set",
			limit: Limit{
				PageSize: "4k",
				Limit: LimitValue{
					Value: 8 * (1 << 20),
				},
			},
			expected: "4k=8MB",
		},
		{
			name: "zero value set",
			limit: Limit{
				PageSize: "2m",
				Limit: LimitValue{
					Value: 0,
				},
			},
			expected: "2m=0KB",
		},
		{
			name: "unset",
			limit: Limit{
				PageSize: "2m",
				Limit: LimitValue{
					Unset: true,
				},
			},
			expected: "2m=max",
		},
		{
			name: "unset conflict",
			limit: Limit{
				PageSize: "2m",
				Limit: LimitValue{
					Unset: true,
					Value: 16 * (2 << 20),
				},
			},
			expected: "2m=max",
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			got := tcase.limit.String()
			if got != tcase.expected {
				t.Errorf("got=%q expected=%q", got, tcase.expected)
			}
		})
	}
}

func TestLimitsToString(t *testing.T) {
	type testcase struct {
		name     string
		limits   []Limit
		expected string
	}

	testcases := []testcase{
		{
			name:     "zero value",
			limits:   []Limit{},
			expected: "",
		},
		{
			name: "single limit - unset",
			limits: []Limit{
				{
					PageSize: "4k",
					Limit: LimitValue{
						Unset: true,
					},
				},
			},
			expected: "4k=max",
		},
		{
			name: "single limit - set",
			limits: []Limit{
				{
					PageSize: "4k",
					Limit: LimitValue{
						Value: 8 * (2 << 20),
					},
				},
			},
			expected: "4k=16MB",
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			got := LimitsToString(tcase.limits)
			if diff := cmp.Diff(got, tcase.expected); diff != "" {
				t.Errorf("unexpected diff: %v", diff)
			}
		})
	}
}

func TestLimitsFromAllocation(t *testing.T) {
	machineDataX86 := sysinfo.MachineData{
		Hugepagesizes: []uint64{
			(1 << 21),
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
