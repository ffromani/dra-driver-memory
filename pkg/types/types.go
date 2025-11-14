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
	if ri.Kind == Memory {
		return resourceapi.QualifiedName("memory")
	}
	return resourceapi.QualifiedName("pages")
}

// A Span is a memory area
type Span struct {
	ResourceIdent
	Amount   int64 // bytes
	NUMAZone int64
}

// Currently, an Allocation currently can only be a proper subset of a Span.
type Allocation struct {
	ResourceIdent
	Amount   int64 // bytes
	NUMAZone int64
}
