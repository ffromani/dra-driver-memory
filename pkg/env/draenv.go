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
	"fmt"
	"strings"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/api/resource"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/cpuset"

	"github.com/ffromani/dra-driver-memory/pkg/cdi"
	"github.com/ffromani/dra-driver-memory/pkg/types"
)

const (
	partNUMANodes = "NUMANodes"
)

// This is the internal "communication" layer helpers. DRA and NRI layers communicate
// through CDI specs and other channels whose code sits here.

func CreateNUMANodes(_ logr.Logger, claimUID k8stypes.UID, claimNodes sets.Set[int64]) string {
	return fmt.Sprintf("%s_%s_%s=%s", cdi.EnvVarPrefix, claimUID, partNUMANodes, numaNodesToString(claimNodes))
}

func CreateAlloc(_ logr.Logger, claimUID k8stypes.UID, alloc types.Allocation) string {
	return fmt.Sprintf("%s_%s_%s=numanode:%d,size:%s", cdi.EnvVarPrefix, claimUID, resourceNameToEnv(alloc.Name()), alloc.NUMAZone, alloc.ToQuantityString())
}

func ExtractNUMANodesInto(lh logr.Logger, env string, numaNodesByClaim map[k8stypes.UID]cpuset.CPUSet) (bool, error) {
	parts := strings.SplitN(env, "=", 2)
	if len(parts) != 2 {
		return false, fmt.Errorf("malformed DRA env entry %q", env)
	}
	key, value := parts[0], parts[1]

	keyParts := strings.SplitN(key, "_", 3)
	if len(keyParts) != 3 {
		return false, fmt.Errorf("malformed DRA env key %q", key)
	}
	if keyParts[2] != partNUMANodes {
		return false, nil // it's another env. Move on.
	}
	claimUID := k8stypes.UID(keyParts[1])
	numaNodes, err := cpuset.Parse(value)
	if err != nil {
		return true, fmt.Errorf("failed to parse cpuset (for memory nodes) value %q from env %q: %w", value, env, err)
	}
	numaNodesByClaim[claimUID] = numaNodes
	lh.V(4).Info("parsed NUMA Nodes", "claimUID", claimUID, "numaNodes", numaNodes.String())
	return true, nil
}

func ExtractAllocsInto(lh logr.Logger, env string, resourceNames sets.Set[string], allocsByClaim map[k8stypes.UID]types.Allocation) (bool, error) {
	parts := strings.SplitN(env, "=", 2)
	if len(parts) != 2 {
		return false, fmt.Errorf("malformed DRA env entry %q", env)
	}
	key, value := parts[0], parts[1]

	keyParts := strings.SplitN(key, "_", 3)
	if len(keyParts) != 3 {
		return false, fmt.Errorf("malformed DRA env key %q", key)
	}
	resourceName := envToResourceName(keyParts[2])
	if !resourceNames.Has(resourceName) {
		return false, nil // it's another env. Move on.
	}
	claimUID := k8stypes.UID(keyParts[1])

	ident, err := types.ResourceIdentFromName(resourceName)
	if err != nil {
		return false, err
	}
	alloc := types.Allocation{
		ResourceIdent: ident,
	}
	err = extractAllocValueInto(value, &alloc)
	if err != nil {
		return false, err
	}
	allocsByClaim[claimUID] = alloc
	lh.V(4).Info("parsed allocation", "claimUID", claimUID, "resourceName", alloc.Name(), "amount", alloc.Amount, "NUMANode", alloc.NUMAZone)
	return true, nil
}

func ExtractAll(lh logr.Logger, envs []string, resourceNames sets.Set[string]) (map[k8stypes.UID]cpuset.CPUSet, map[k8stypes.UID]types.Allocation, error) {
	numaNodesByClaim := make(map[k8stypes.UID]cpuset.CPUSet)
	allocsByClaim := make(map[k8stypes.UID]types.Allocation)

	for _, env := range envs {
		if !strings.HasPrefix(env, cdi.EnvVarPrefix) {
			continue
		}
		lh.V(4).Info("Parsing DRA env", "entry", env)
		// we will ignore errors related to envs we didn't set: these are not significant
		found, err := ExtractNUMANodesInto(lh, env, numaNodesByClaim)
		if found && err != nil {
			return nil, nil, err
		}
		found, err = ExtractAllocsInto(lh, env, resourceNames, allocsByClaim)
		if found && err != nil {
			return nil, nil, err
		}
	}

	return numaNodesByClaim, allocsByClaim, nil
}

// numaNodesToString assumes to be passed a nonempty set (nodes.Len() >= 1)
func numaNodesToString(nodes sets.Set[int64]) string {
	var sb strings.Builder
	for _, numaNode := range sets.List(nodes) {
		fmt.Fprintf(&sb, ",%d", numaNode)
	}
	return sb.String()[1:]
}

func resourceNameToEnv(resourceName string) string {
	return strings.ReplaceAll(resourceName, "-", "_")
}

func envToResourceName(ev string) string {
	return strings.ReplaceAll(ev, "_", "-")
}

func extractAllocValueInto(value string, alloc *types.Allocation) error {
	var allocStr string
	var numaNode int64
	n, err := fmt.Sscanf(value, "numanode:%d,size:%s", &numaNode, &allocStr)
	if n != 2 || err != nil {
		return fmt.Errorf("malformed DRA env value %q: %w", value, err)
	}
	qty, err := resource.ParseQuantity(allocStr)
	if err != nil {
		return fmt.Errorf("malformed DRA env size %q: %w", value, err)
	}
	amount, ok := qty.AsInt64()
	if !ok {
		return fmt.Errorf("cannot convert DRA env amount %v: %w", qty.String(), err)
	}
	alloc.Amount = amount
	alloc.NUMAZone = numaNode
	return nil
}
