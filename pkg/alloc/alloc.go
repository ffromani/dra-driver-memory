/*
Copyright 2025 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package alloc

import (
	"maps"

	"github.com/go-logr/logr"

	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/ffromani/dra-driver-memory/pkg/types"
)

type Manager struct {
	// claim -> resourceType (can be `hugepages-1g`) -> allocation
	claimedResources map[k8stypes.UID]map[string]types.Allocation
	claimsByPodId    map[string]sets.Set[k8stypes.UID]
}

func NewManager() *Manager {
	return &Manager{
		claimedResources: make(map[k8stypes.UID]map[string]types.Allocation),
		claimsByPodId:    make(map[string]sets.Set[k8stypes.UID]),
	}
}

func (mgr *Manager) RegisterClaim(claimUID k8stypes.UID, claimAllocs map[string]types.Allocation) {
	alloc, ok := mgr.claimedResources[claimUID]
	if !ok {
		mgr.claimedResources[claimUID] = maps.Clone(claimAllocs)
		return
	}
	for key, val := range claimAllocs {
		alloc[key] = val
	}
	mgr.claimedResources[claimUID] = alloc
}

func (mgr *Manager) UnregisterClaim(claimUID k8stypes.UID) {
	delete(mgr.claimedResources, claimUID)
}

func (mgr *Manager) GetClaim(claimUID k8stypes.UID) (map[string]types.Allocation, bool) {
	allocs, ok := mgr.claimedResources[claimUID]
	if !ok {
		return nil, false
	}
	return maps.Clone(allocs), true
}

func (mgr *Manager) BindClaimToPod(lh logr.Logger, podId string, claimUID k8stypes.UID) {
	claimUIDs, ok := mgr.claimsByPodId[podId]
	if !ok {
		lh.V(4).Info("claim bound", "podSandboxID", podId, "claimUID", claimUID)
		mgr.claimsByPodId[podId] = sets.New(claimUID)
		return
	}
	if claimUIDs.Has(claimUID) {
		return // minimize work and logging
	}
	claimUIDs.Insert(claimUID)
	mgr.claimsByPodId[podId] = claimUIDs
	lh.V(4).Info("claim bound", "podSandboxID", podId, "claimUID", claimUID)
}

func (mgr *Manager) UnregisterClaimsForPod(lh logr.Logger, podId string) {
	claimUIDs, ok := mgr.claimsByPodId[podId]
	if !ok {
		return
	}
	lh.V(4).Info("unbinding claims", "podSandboxID", podId, "claimsCount", claimUIDs.Len())
	for _, claimUID := range claimUIDs.UnsortedList() {
		mgr.UnregisterClaim(claimUID)
	}
	delete(mgr.claimsByPodId, podId)
}

func (mgr *Manager) CountClaims() int {
	return len(mgr.claimedResources)
}

func (mgr *Manager) CountPods() int {
	return len(mgr.claimsByPodId)
}
