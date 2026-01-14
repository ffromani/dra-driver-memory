/*
Copyright 2026 The Kubernetes Authors.

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
	"fmt"
	"sync"

	"github.com/go-logr/logr"

	k8stypes "k8s.io/apimachinery/pkg/types"
)

type AlreadyBound struct {
	ClaimUID k8stypes.UID
	Owner    OwnerIdent
}

func (ab AlreadyBound) Error() string {
	return fmt.Sprintf("claimUID %q already bound to pod %q container %q", ab.ClaimUID, ab.Owner.PodUID, ab.Owner.ContainerName)
}

type OwnerIdent struct {
	PodUID        string
	ContainerName string
}

func (oi OwnerIdent) Equal(x OwnerIdent) bool {
	return oi.PodUID == x.PodUID && oi.ContainerName == x.ContainerName
}

type Binder struct {
	mu sync.Mutex
	// clamUID => podUID(+containerName) mapping.
	// No claims can be shared by containers or pods
	// But a container can have more than a claim.
	ownerByClaimUID map[k8stypes.UID]OwnerIdent
}

func NewBinder() *Binder {
	return &Binder{
		ownerByClaimUID: make(map[k8stypes.UID]OwnerIdent),
	}
}

func (bnd *Binder) SetOwner(lh logr.Logger, claimUID k8stypes.UID, podUID, containerName string) error {
	curIdent := OwnerIdent{
		PodUID:        podUID,
		ContainerName: containerName,
	}
	bnd.mu.Lock()
	defer bnd.mu.Unlock()
	owner, ok := bnd.ownerByClaimUID[claimUID]
	if ok {
		if owner.Equal(curIdent) {
			lh.V(2).Info("claim REbound", "claimUID", claimUID, "podUID", podUID, "containerName", containerName)
			return nil // not wrong, not suspicious enough to bail out
		}
		return AlreadyBound{
			ClaimUID: claimUID,
			Owner:    owner,
		}
	}
	bnd.ownerByClaimUID[claimUID] = curIdent
	lh.V(4).Info("claim bound", "claimUID", claimUID, "podUID", podUID, "containerName", containerName)
	return nil
}

func (bnd *Binder) FindOwner(lh logr.Logger, claimUID k8stypes.UID) (OwnerIdent, bool) {
	bnd.mu.Lock()
	defer bnd.mu.Unlock()
	owner, ok := bnd.ownerByClaimUID[claimUID]
	return owner, ok
}

func (bnd *Binder) Cleanup(lh logr.Logger, claimUIDs ...k8stypes.UID) {
	bnd.mu.Lock()
	defer bnd.mu.Unlock()
	for _, claimUID := range claimUIDs {
		delete(bnd.ownerByClaimUID, claimUID)
	}
}

func (bnd *Binder) Len() int {
	bnd.mu.Lock()
	defer bnd.mu.Unlock()
	return len(bnd.ownerByClaimUID)
}
