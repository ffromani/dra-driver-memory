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
	"testing"

	"github.com/google/go-cmp/cmp"

	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/utils/ptr"

	"github.com/ffromani/dra-driver-memory/pkg/types"
)

func TestMakeAttributes(t *testing.T) {
	type testcase struct {
		span     types.Span
		expected map[resourceapi.QualifiedName]resourceapi.DeviceAttribute
	}

	// TODO: at the moment, we explicitly inline all the test permutations we care
	// about because these are few and manageable; if we add more simple variations,
	// we should factor out these and create helper functions.
	testcases := []testcase{
		{
			span: types.Span{
				ResourceIdent: types.ResourceIdent{
					Kind:     types.Hugepages,
					Pagesize: uint64(2 * 1 << 20),
				},
				Amount:   1, // not really relevant
				NUMAZone: 0, // we want to be explicit here and not depend on zero-value init of golang
			},
			expected: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
				StandardDeviceAttributePrefix + "numaNode": {IntValue: ptr.To(int64(0))},
				StandardDeviceAttributePrefix + "pageSize": {StringValue: ptr.To("2Mi")},
				StandardDeviceAttributePrefix + "hugeTLB":  {BoolValue: ptr.To(true)},
				"dra.cpu/numaNodeID":                       {IntValue: ptr.To(int64(0))},
				"dra.net/numaNode":                         {IntValue: ptr.To(int64(0))},
			},
		},
		{
			span: types.Span{
				ResourceIdent: types.ResourceIdent{
					Kind:     types.Hugepages,
					Pagesize: uint64(2 * 1 << 20),
				},
				Amount:   1, // not really relevant
				NUMAZone: 3, // random non-zero value; 1 would have been fine as well
			},
			expected: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
				StandardDeviceAttributePrefix + "numaNode": {IntValue: ptr.To(int64(3))},
				StandardDeviceAttributePrefix + "pageSize": {StringValue: ptr.To("2Mi")},
				StandardDeviceAttributePrefix + "hugeTLB":  {BoolValue: ptr.To(true)},
				"dra.cpu/numaNodeID":                       {IntValue: ptr.To(int64(3))},
				"dra.net/numaNode":                         {IntValue: ptr.To(int64(3))},
			},
		},
		{
			span: types.Span{
				ResourceIdent: types.ResourceIdent{
					Kind:     types.Hugepages,
					Pagesize: uint64(1 << 30),
				},
				Amount:   3, // not really relevant
				NUMAZone: 0, // we want to be explicit here and not depend on zero-value init of golang
			},
			expected: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
				StandardDeviceAttributePrefix + "numaNode": {IntValue: ptr.To(int64(0))},
				StandardDeviceAttributePrefix + "pageSize": {StringValue: ptr.To("1Gi")},
				StandardDeviceAttributePrefix + "hugeTLB":  {BoolValue: ptr.To(true)},
				"dra.cpu/numaNodeID":                       {IntValue: ptr.To(int64(0))},
				"dra.net/numaNode":                         {IntValue: ptr.To(int64(0))},
			},
		},
		{
			span: types.Span{
				ResourceIdent: types.ResourceIdent{
					Kind:     types.Hugepages,
					Pagesize: uint64(1 << 30),
				},
				Amount:   5, // not really relevant
				NUMAZone: 3, // random non-zero value; 1 would have been fine as well
			},
			expected: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
				StandardDeviceAttributePrefix + "numaNode": {IntValue: ptr.To(int64(3))},
				StandardDeviceAttributePrefix + "pageSize": {StringValue: ptr.To("1Gi")},
				StandardDeviceAttributePrefix + "hugeTLB":  {BoolValue: ptr.To(true)},
				"dra.cpu/numaNodeID":                       {IntValue: ptr.To(int64(3))},
				"dra.net/numaNode":                         {IntValue: ptr.To(int64(3))},
			},
		},
		{
			span: types.Span{
				ResourceIdent: types.ResourceIdent{
					Kind:     types.Memory,
					Pagesize: uint64(4 * 1 << 10),
				},
				Amount:   2048, // not really relevant
				NUMAZone: 0,    // we want to be explicit here and not depend on zero-value init of golang
			},
			expected: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
				StandardDeviceAttributePrefix + "numaNode": {IntValue: ptr.To(int64(0))},
				StandardDeviceAttributePrefix + "pageSize": {StringValue: ptr.To("4Ki")},
				StandardDeviceAttributePrefix + "hugeTLB":  {BoolValue: ptr.To(false)},
				"dra.cpu/numaNodeID":                       {IntValue: ptr.To(int64(0))},
				"dra.net/numaNode":                         {IntValue: ptr.To(int64(0))},
			},
		},
		{
			span: types.Span{
				ResourceIdent: types.ResourceIdent{
					Kind:     types.Memory,
					Pagesize: uint64(4 * 1 << 10),
				},
				Amount:   8192, // not really relevant
				NUMAZone: 2,    // random non-zero value; 1 would have been fine as well
			},
			expected: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
				StandardDeviceAttributePrefix + "numaNode": {IntValue: ptr.To(int64(2))},
				StandardDeviceAttributePrefix + "pageSize": {StringValue: ptr.To("4Ki")},
				StandardDeviceAttributePrefix + "hugeTLB":  {BoolValue: ptr.To(false)},
				"dra.cpu/numaNodeID":                       {IntValue: ptr.To(int64(2))},
				"dra.net/numaNode":                         {IntValue: ptr.To(int64(2))},
			},
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.span.String(), func(t *testing.T) {
			got := MakeAttributes(tcase.span)
			if diff := cmp.Diff(tcase.expected, got); diff != "" {
				t.Fatalf("unexpected diff: %v", diff)
			}
		})
	}
}
