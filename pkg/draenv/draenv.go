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
	"fmt"
	"strings"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/cpuset"

	"github.com/ffromani/dra-driver-memory/pkg/cdi"
)

// This is the internal "communication" layer helpers. DRA and NRI layers communicate
// through CDI specs and other channels whose code sits here.

func FromClaimAllocations(lh logr.Logger, claimUID types.UID, claimNodes sets.Set[int64]) string {
	return fmt.Sprintf("%s_%s=%s", cdi.EnvVarPrefix, claimUID, numaNodesToString(claimNodes))
}

func ToClaimAllocations(lh logr.Logger, envs []string) (map[types.UID]cpuset.CPUSet, error) {
	allocations := make(map[types.UID]cpuset.CPUSet)

	for _, env := range envs {
		if !strings.HasPrefix(env, cdi.EnvVarPrefix) {
			continue
		}
		lh.V(4).Info("Parsing DRA env", "entry", env)
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("malformed DRA env entry %q", env)
		}
		key, value := parts[0], parts[1]
		var claimUID types.UID
		if !strings.HasPrefix(key, cdi.EnvVarPrefix+"_") {
			continue
		}
		uidStr := strings.TrimPrefix(key, cdi.EnvVarPrefix+"_")
		claimUID = types.UID(uidStr)

		numaNodes, err := cpuset.Parse(value)
		if err != nil {
			return nil, fmt.Errorf("failed to parse cpuset (for memory nodes) value %q from env %q: %w", value, env, err)
		}
		allocations[claimUID] = numaNodes

		lh.V(4).Info("parsed allocation", "claimUID", claimUID, "numaNodes", numaNodes.String())
	}

	return allocations, nil
}

// numaNodesToString assumes to be passed a nonempty set (nodes.Len() >= 1)
func numaNodesToString(nodes sets.Set[int64]) string {
	var sb strings.Builder
	for _, numaNode := range sets.List(nodes) {
		fmt.Fprintf(&sb, ",%d", numaNode)
	}
	return sb.String()[1:]
}
