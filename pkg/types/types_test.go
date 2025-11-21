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

package types

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestResourceIdentNameRoundTrip(t *testing.T) {
	type testcase struct {
		fullName string
		name     string
		hugeTLB  bool
		ident    ResourceIdent
	}

	testcases := []testcase{
		{
			fullName: "memory-4k",
			name:     "memory",
			ident: ResourceIdent{
				Kind:     Memory,
				Pagesize: 4 * 1024,
			},
		},
		{
			fullName: "hugepages-2m",
			name:     "hugepages-2m",
			hugeTLB:  true,
			ident: ResourceIdent{
				Kind:     Hugepages,
				Pagesize: 2 * 1024 * 1024,
			},
		},
		{
			fullName: "hugepages-1g",
			name:     "hugepages-1g",
			hugeTLB:  true,
			ident: ResourceIdent{
				Kind:     Hugepages,
				Pagesize: 1024 * 1024 * 1024,
			},
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.fullName, func(t *testing.T) {
			gotIdent, err := ResourceIdentFromName(tcase.fullName)
			require.NoError(t, err)
			require.Equal(t, gotIdent.FullName(), tcase.fullName)
			require.Equal(t, gotIdent.Name(), tcase.name)
			require.Equal(t, gotIdent, tcase.ident)
			require.Equal(t, gotIdent.NeedsHugeTLB(), tcase.hugeTLB)
		})
	}
}

func TestResourceIdentCapacityName(t *testing.T) {
	type testcase struct {
		fullName string
		ident    ResourceIdent
	}

	testcases := []testcase{
		{
			fullName: "memory-4k",
			ident: ResourceIdent{
				Kind:     Memory,
				Pagesize: 4 * 1024,
			},
		},
		{
			fullName: "hugepages-2m",
			ident: ResourceIdent{
				Kind:     Hugepages,
				Pagesize: 2 * 1024 * 1024,
			},
		},
		{
			fullName: "hugepages-1g",
			ident: ResourceIdent{
				Kind:     Hugepages,
				Pagesize: 1024 * 1024 * 1024,
			},
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.fullName, func(t *testing.T) {
			got := string(tcase.ident.CapacityName())
			require.Equal(t, got, "size")
		})
	}
}

func TestResourceIdentMinimumAllocatable(t *testing.T) {
	type testcase struct {
		fullName string
		ident    ResourceIdent
	}

	testcases := []testcase{
		{
			fullName: "memory-4k",
			ident: ResourceIdent{
				Kind:     Memory,
				Pagesize: 4 * 1024,
			},
		},
		{
			fullName: "hugepages-2m",
			ident: ResourceIdent{
				Kind:     Hugepages,
				Pagesize: 2 * 1024 * 1024,
			},
		},
		{
			fullName: "hugepages-1g",
			ident: ResourceIdent{
				Kind:     Hugepages,
				Pagesize: 1024 * 1024 * 1024,
			},
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.fullName, func(t *testing.T) {
			minAlloc := tcase.ident.MinimumAllocatable()
			require.GreaterOrEqual(t, minAlloc, tcase.ident.Pagesize)
		})
	}
}

func TestResourceIdentNameNegative(t *testing.T) {
	type testcase struct {
		fullName string
	}

	testcases := []testcase{
		{
			fullName: "memory-XX",
		},
		{
			fullName: "hugepages2m",
		},
		{
			fullName: "hugepages-1x",
		},
		{
			fullName: "foobar-X",
		},
		{
			fullName: "foobar-1m",
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.fullName, func(t *testing.T) {
			_, err := ResourceIdentFromName(tcase.fullName)
			require.Error(t, err)
		})
	}
}

func TestResourceQuantityStringRepr(t *testing.T) {
	type testcase struct {
		name     string
		alloc    Allocation
		expected string
	}

	testcases := []testcase{
		{
			name: "memory-simple-0",
			alloc: Allocation{
				ResourceIdent: ResourceIdent{
					Kind:     Memory,
					Pagesize: 4 * 1 << 10,
				},
				Amount:   32 * 1 << 10,
				NUMAZone: 1, // not really significant
			},
			expected: "32Ki",
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			got := tcase.alloc.ToQuantityString()
			require.Equal(t, got, tcase.expected)
		})
	}
}

func TestSpanMakeAllocation(t *testing.T) {
	type testcase struct {
		name     string
		span     Span
		expected Allocation
	}

	testcases := []testcase{
		{
			name: "memory-simple-0",
			span: Span{
				ResourceIdent: ResourceIdent{
					Kind:     Memory,
					Pagesize: 4 * 1 << 10,
				},
				Amount:   1 * 1 << 30,
				NUMAZone: 1,
			},
			expected: Allocation{
				ResourceIdent: ResourceIdent{
					Kind:     Memory,
					Pagesize: 4 * 1 << 10,
				},
				Amount:   256 * 1 << 20,
				NUMAZone: 1,
			},
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			got := tcase.span.MakeAllocation(256 * 1024 * 1024)
			if diff := cmp.Diff(got, tcase.expected); diff != "" {
				t.Fatalf("unexpected diff: %q", diff)
			}
		})
	}
}

func TestSpanPages(t *testing.T) {
	type testcase struct {
		name     string
		span     Span
		expected int64
	}

	testcases := []testcase{
		{
			name: "memory-simple-0",
			span: Span{
				ResourceIdent: ResourceIdent{
					Kind:     Memory,
					Pagesize: 4 * 1 << 10,
				},
				Amount:   1 * 1 << 30,
				NUMAZone: 1, // not really significant
			},
			expected: 256 * 1024,
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			got := tcase.span.Pages()
			require.Equal(t, got, tcase.expected)
		})
	}
}

func TestAllocationPages(t *testing.T) {
	type testcase struct {
		name     string
		alloc    Allocation
		expected int64
	}

	testcases := []testcase{
		{
			name: "memory-simple-0",
			alloc: Allocation{
				ResourceIdent: ResourceIdent{
					Kind:     Memory,
					Pagesize: 4 * 1 << 10,
				},
				Amount:   32 * 1 << 10,
				NUMAZone: 1, // not really significant
			},
			expected: 8,
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			got := tcase.alloc.Pages()
			require.Equal(t, got, tcase.expected)
		})
	}
}

func TestSpanString(t *testing.T) {
	type testcase struct {
		name string
		span Span
	}

	testcases := []testcase{
		{
			name: "memory-simple-0",
			span: Span{
				ResourceIdent: ResourceIdent{
					Kind:     Memory,
					Pagesize: 4 * 1 << 10,
				},
				Amount:   1 * 1 << 30,
				NUMAZone: 1, // not really significant
			},
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			got := tcase.span.String()
			require.Contains(t, got, tcase.span.Kind)
			require.Contains(t, got, "size")
			require.Contains(t, got, "numaZone")
		})
	}
}

func TestAllocationString(t *testing.T) {
	type testcase struct {
		name  string
		alloc Allocation
	}

	testcases := []testcase{
		{
			name: "memory-simple-0",
			alloc: Allocation{
				ResourceIdent: ResourceIdent{
					Kind:     Memory,
					Pagesize: 4 * 1 << 10,
				},
				Amount:   1 * 1 << 30,
				NUMAZone: 1, // not really significant
			},
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			got := tcase.alloc.String()
			require.Contains(t, got, tcase.alloc.Kind)
			require.Contains(t, got, "size")
			require.Contains(t, got, "numaZone")
		})
	}
}
