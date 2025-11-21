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
	"fmt"
	"strings"

	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/ffromani/dra-driver-memory/pkg/unitconv"
)

type ResourceKind string

const (
	Memory    ResourceKind = "memory"
	Hugepages ResourceKind = "hugepages"
)

type ResourceIdent struct {
	Kind     ResourceKind
	Pagesize uint64 //bytes
}

// name is in the form `memory-4k` or `hugepages-1g`
func ResourceIdentFromName(name string) (ResourceIdent, error) {
	parts := strings.SplitN(name, "-", 2)
	if len(parts) != 2 {
		return ResourceIdent{}, fmt.Errorf("malformed name: %q", name)
	}
	if parts[0] != string(Memory) && parts[0] != string(Hugepages) {
		return ResourceIdent{}, fmt.Errorf("unknown resource: %q", parts[0])
	}
	sizeInBytes, err := unitconv.MinimizedStringToSizeInBytes(parts[1])
	if err != nil {
		return ResourceIdent{}, err
	}
	return ResourceIdent{
		Kind:     ResourceKind(parts[0]),
		Pagesize: sizeInBytes,
	}, nil
}

// FullName returns a non-canonical, roundtrip-able name
func (ri ResourceIdent) FullName() string {
	return string(ri.Kind) + "-" + ri.PagesizeString()
}

// Name returns the canonical name which is not roundtrip-able
func (ri ResourceIdent) Name() string {
	if ri.Kind == Memory {
		return string(Memory)
	}
	return string(Hugepages) + "-" + ri.PagesizeString()
}

func (ri ResourceIdent) PagesizeString() string {
	return unitconv.SizeInBytesToMinimizedString(ri.Pagesize)
}

func (ri ResourceIdent) NeedsHugeTLB() bool {
	return ri.Kind != Memory
}

func (ri ResourceIdent) CapacityName() resourceapi.QualifiedName {
	// hugepages are represented as memory intentionally,
	// to be closer to what kubelet did.
	// We may revisit this in the future, but we don't want
	// to diverge until and unless we have very strong reason to
	return resourceapi.QualifiedName("size")
}

func (ri ResourceIdent) MinimumAllocatable() uint64 {
	if ri.Kind == Hugepages {
		return ri.Pagesize
	}
	return 1 << 20 // hardly makes sense to allocate less than 1 MiB on kubernetes on 2025 and onwards. And we're being very conservative.
}

// A Span is a memory area
type Span struct {
	ResourceIdent
	Amount   int64 // bytes
	NUMAZone int64
}

func (sp Span) String() string {
	return fmt.Sprintf("%s size=%s numaZone=%d", sp.Name(), unitconv.SizeInBytesToMinimizedString(uint64(sp.Amount)), sp.NUMAZone)
}

func (sp Span) Pages() int64 {
	return int64(uint64(sp.Amount) / sp.Pagesize)
}

func (sp Span) MakeAllocation(amount int64) Allocation {
	return Allocation{
		ResourceIdent: sp.ResourceIdent,
		Amount:        amount,
		NUMAZone:      sp.NUMAZone,
	}
}

// Currently, an Allocation currently can only be a proper subset of a Span.
type Allocation struct {
	ResourceIdent
	Amount   int64 // bytes
	NUMAZone int64
}

func (ac Allocation) String() string {
	return fmt.Sprintf("%s size=%s numaZone=%d", ac.Name(), unitconv.SizeInBytesToMinimizedString(uint64(ac.Amount)), ac.NUMAZone)
}

func (ac Allocation) ToQuantityString() string {
	return resource.NewQuantity(ac.Amount, resource.BinarySI).String()
}

func (ac Allocation) Pages() int64 {
	return int64(uint64(ac.Amount) / ac.Pagesize)
}
