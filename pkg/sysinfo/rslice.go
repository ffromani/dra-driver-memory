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

package sysinfo

import (
	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	k8srand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"

	"github.com/ffromani/dra-driver-memory/pkg/types"
)

func MakeAttributes(sp types.Span) map[resourceapi.QualifiedName]resourceapi.DeviceAttribute {
	pNode := ptr.To(sp.NUMAZone)
	return map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
		// alignment compatibility: dra-driver-sriov
		"resource.kubernetes.io/numaNode": {IntValue: pNode},
		// alignment compatibility: dra-driver-cpu
		"dra.cpu/numaNode": {IntValue: pNode},
		// alignment compatibility: dranet
		"dra.net/numaNode": {IntValue: pNode},
		// our own attributes, at last
		"dra.memory/numaNode": {IntValue: pNode},
		"dra.memory/pageSize": {StringValue: ptr.To(sp.PagesizeString())},
		"dra.memory/hugeTLB":  {BoolValue: ptr.To(sp.NeedsHugeTLB())},
	}
}

func MakeCapacity(sp types.Span) map[resourceapi.QualifiedName]resourceapi.DeviceCapacity {
	name := sp.CapacityName()
	qty := resource.NewQuantity(sp.Amount, resource.DecimalSI)
	return map[resourceapi.QualifiedName]resourceapi.DeviceCapacity{
		name: {
			Value: *qty,
		},
	}
}

func ToDevice(sp types.Span) resourceapi.Device {
	return resourceapi.Device{
		Name:                     MakeDeviceName(sp.Name()),
		Attributes:               MakeAttributes(sp),
		Capacity:                 MakeCapacity(sp),
		AllowMultipleAllocations: ptr.To(true),
	}
}

// MakeDeviceName creates a unique short device name from the base device name ("memory", "hugepages-2m")
// We use a random part because the device name is not really that relevant, as long as it's unique.
// We can very much construct it concatenating nodeName and NUMAZoneID, that would be unique and equally
// valid as we expose plenty of low-level details like the NUMAZoneID anyway, but the concern is that
// we would need more validation, e.g, translating the nodeName (dots->dashes) and so on.
// Since users are expected to select memory devices by attribute and not by name, we just use a
// random suffix for the time being and move on.
var MakeDeviceName = func(devName string) string {
	return devName + "-" + k8srand.String(6)
}
