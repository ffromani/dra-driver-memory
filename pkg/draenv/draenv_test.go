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

package draenv

import (
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/cpuset"
)

func TestEnvironRoundTrip(t *testing.T) {
	type testcase struct {
		name     string
		uid      types.UID
		nodes    sets.Set[int64]
		expected map[types.UID]cpuset.CPUSet
	}

	testcases := []testcase{
		{
			name:  "simplest, single node",
			uid:   types.UID("FOOBAR"),
			nodes: sets.New[int64](0),
			expected: map[types.UID]cpuset.CPUSet{
				types.UID("FOOBAR"): cpuset.New(0),
			},
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			logger := testr.New(t)
			env := FromClaimAllocations(logger, tcase.uid, tcase.nodes)
			envs := []string{
				env,
			}
			got, err := ToClaimAllocations(logger, envs)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(got, tcase.expected, cmpopts.IgnoreUnexported(cpuset.CPUSet{})); diff != "" {
				t.Errorf("unexpected value: %v", diff)
			}
		})
	}
}
