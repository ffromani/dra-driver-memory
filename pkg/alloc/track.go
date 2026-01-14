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
	"sync"

	"github.com/go-logr/logr"

	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/ffromani/dra-driver-memory/pkg/types"
)

type podItem struct {
	ClaimUIDs sets.Set[k8stypes.UID]
}

type Tracker struct {
	rwMu sync.RWMutex
	// claim -> resourceType (can be `hugepages-1g`) -> allocation
	allocationsByClaimUID map[k8stypes.UID]map[string]types.Allocation
	claimsByPodSandboxID  map[string]podItem
}

func NewTracker() *Tracker {
	return &Tracker{
		allocationsByClaimUID: make(map[k8stypes.UID]map[string]types.Allocation),
		claimsByPodSandboxID:  make(map[string]podItem),
	}
}

func (trk *Tracker) RegisterClaim(claimUID k8stypes.UID, claimAllocs map[string]types.Allocation) {
	trk.rwMu.Lock()
	defer trk.rwMu.Unlock()
	alloc, ok := trk.allocationsByClaimUID[claimUID]
	if !ok {
		trk.allocationsByClaimUID[claimUID] = maps.Clone(claimAllocs)
		return
	}
	for key, val := range claimAllocs {
		alloc[key] = val
	}
	trk.allocationsByClaimUID[claimUID] = alloc
}

func (trk *Tracker) UnregisterClaim(claimUID k8stypes.UID) {
	trk.rwMu.Lock()
	defer trk.rwMu.Unlock()
	trk.unregisterClaimUnlocked(claimUID)
}

func (trk *Tracker) GetAllocationsForClaim(claimUID k8stypes.UID) (map[string]types.Allocation, bool) {
	trk.rwMu.RLock()
	defer trk.rwMu.RUnlock()
	allocs, ok := trk.allocationsByClaimUID[claimUID]
	if !ok {
		return nil, false
	}
	return maps.Clone(allocs), true
}

func (trk *Tracker) BindClaim(lh logr.Logger, claimUID k8stypes.UID, podSandboxID string) {
	trk.rwMu.Lock()
	defer trk.rwMu.Unlock()
	info, ok := trk.claimsByPodSandboxID[podSandboxID]
	if !ok {
		lh.V(5).Info("podItem created", "podSandboxID", podSandboxID, "claimUID", claimUID)
		info = podItem{
			ClaimUIDs: sets.New[k8stypes.UID](),
		}
	}
	info.ClaimUIDs.Insert(claimUID)
	trk.claimsByPodSandboxID[podSandboxID] = info
	lh.V(4).Info("podItem bound", "claimUID", claimUID, "podSandboxID", podSandboxID)
}

func (trk *Tracker) CleanupPod(lh logr.Logger, podSandboxID string) []k8stypes.UID {
	trk.rwMu.Lock()
	defer trk.rwMu.Unlock()
	info, ok := trk.claimsByPodSandboxID[podSandboxID]
	if !ok {
		return nil
	}
	claimUIDs := info.ClaimUIDs.UnsortedList()
	lh.V(4).Info("cleaning claims", "podSandboxID", podSandboxID, "claimsCount", len(claimUIDs))
	for _, claimUID := range claimUIDs {
		trk.unregisterClaimUnlocked(claimUID)
	}
	trk.unbindClaimUnlocked(podSandboxID)
	return claimUIDs
}

func (trk *Tracker) CountClaims() int {
	return len(trk.allocationsByClaimUID)
}

func (trk *Tracker) CountPods() int {
	return len(trk.claimsByPodSandboxID)
}

func (trk *Tracker) unregisterClaimUnlocked(claimUID k8stypes.UID) {
	delete(trk.allocationsByClaimUID, claimUID)
}

func (trk *Tracker) unbindClaimUnlocked(podSandboxID string) {
	delete(trk.claimsByPodSandboxID, podSandboxID)
}
