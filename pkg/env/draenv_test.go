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

package env

import (
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/cpuset"

	"github.com/ffromani/dra-driver-memory/pkg/types"
)

func TestCreateNUMANodesRoundTrip(t *testing.T) {
	type testcase struct {
		name     string
		uid      k8stypes.UID
		nodes    sets.Set[int64]
		expected map[k8stypes.UID]cpuset.CPUSet
	}

	testcases := []testcase{
		{
			name:  "simplest, single node",
			uid:   k8stypes.UID("FOOBAR"),
			nodes: sets.New[int64](0),
			expected: map[k8stypes.UID]cpuset.CPUSet{
				k8stypes.UID("FOOBAR"): cpuset.New(0),
			},
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			logger := testr.New(t)
			env := CreateNUMANodes(logger, tcase.uid, tcase.nodes)
			got := make(map[k8stypes.UID]cpuset.CPUSet)
			ok, err := ExtractNUMANodesInto(logger, env, got)
			require.NoError(t, err)
			require.True(t, ok, "cannot extract from env var %q", env)
			if diff := cmp.Diff(got, tcase.expected, cmpopts.IgnoreUnexported(cpuset.CPUSet{})); diff != "" {
				t.Errorf("unexpected value: %v", diff)
			}
		})
	}
}

func TestCreateAllocRoundTrip(t *testing.T) {
	type testcase struct {
		name     string
		uid      k8stypes.UID
		alloc    types.Allocation
		expected map[k8stypes.UID]types.Allocation
	}

	testcases := []testcase{
		{
			name: "simplest, single node",
			uid:  k8stypes.UID("FOOBAR"),
			alloc: types.Allocation{
				ResourceIdent: types.ResourceIdent{
					Kind:     types.Hugepages,
					Pagesize: 2 * 1024 * 1024,
				},
				Amount:   8 * 2 * 1024 * 1024,
				NUMAZone: 2,
			},
			expected: map[k8stypes.UID]types.Allocation{
				k8stypes.UID("FOOBAR"): {
					ResourceIdent: types.ResourceIdent{
						Kind:     types.Hugepages,
						Pagesize: 2 * 1024 * 1024,
					},
					Amount:   8 * 2 * 1024 * 1024,
					NUMAZone: 2,
				},
			},
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			logger := testr.New(t)
			env := CreateAlloc(logger, tcase.uid, tcase.alloc)
			logger.Info("CreateAlloc", "env", env)
			got := make(map[k8stypes.UID]types.Allocation)
			ok, err := ExtractAllocsInto(logger, env, sets.New(tcase.alloc.Name()), got)
			require.NoError(t, err)
			require.True(t, ok, "cannot extract from env var %q", env)
			if diff := cmp.Diff(got, tcase.expected); diff != "" {
				t.Errorf("unexpected value: %v", diff)
			}
		})
	}
}

func TestExtractAll(t *testing.T) {
	type testcase struct {
		name          string
		uid           k8stypes.UID
		alloc         types.Allocation
		nodes         sets.Set[int64]
		expectedNodes map[k8stypes.UID]cpuset.CPUSet
		expectedSpans map[k8stypes.UID]types.Allocation
	}

	testcases := []testcase{
		{
			name: "simplest, single node",
			uid:  k8stypes.UID("FOOBAR"),
			alloc: types.Allocation{
				ResourceIdent: types.ResourceIdent{
					Kind:     types.Hugepages,
					Pagesize: 1024 * 1024 * 1024,
				},
				Amount:   8 * 1024 * 1024 * 1024,
				NUMAZone: 0,
			},
			nodes: sets.New[int64](0),
			expectedNodes: map[k8stypes.UID]cpuset.CPUSet{
				k8stypes.UID("FOOBAR"): cpuset.New(0),
			},
			expectedSpans: map[k8stypes.UID]types.Allocation{
				k8stypes.UID("FOOBAR"): {
					ResourceIdent: types.ResourceIdent{
						Kind:     types.Hugepages,
						Pagesize: 1024 * 1024 * 1024,
					},
					Amount:   8 * 1024 * 1024 * 1024,
					NUMAZone: 0,
				},
			},
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			logger := testr.New(t)
			envs := []string{
				CreateAlloc(logger, tcase.uid, tcase.alloc),
				CreateNUMANodes(logger, tcase.uid, tcase.nodes),
			}
			gotNodes, gotSpans, err := ExtractAll(logger, envs, sets.New(tcase.alloc.Name()))
			require.NoError(t, err)
			if diff := cmp.Diff(gotNodes, tcase.expectedNodes, cmpopts.IgnoreUnexported(cpuset.CPUSet{})); diff != "" {
				t.Errorf("unexpected value: %v", diff)
			}
			if diff := cmp.Diff(gotSpans, tcase.expectedSpans); diff != "" {
				t.Errorf("unexpected value: %v", diff)
			}
		})
	}
}
