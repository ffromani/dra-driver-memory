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

package alloc

import (
	"maps"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	k8stypes "k8s.io/apimachinery/pkg/types"

	"github.com/ffromani/dra-driver-memory/pkg/types"
)

func TestCreate(t *testing.T) {
	mgr := NewManager()
	require.Equal(t, mgr.CountClaims(), 0, "empty allocationManager has claims")
	require.Equal(t, mgr.CountPods(), 0, "empty allocationManager has pods")
}

func TestRegisterUnregister(t *testing.T) {
	claimAllocs := map[string]types.Allocation{
		"memory": types.Allocation{
			ResourceIdent: types.ResourceIdent{
				Kind:     types.Memory,
				Pagesize: 4 * 1024,
			},
			Amount:   16 * 4 * 1024,
			NUMAZone: 1,
		},
	}
	mgr := NewManager()
	mgr.RegisterClaim(k8stypes.UID("foobar"), claimAllocs)
	mgr.UnregisterClaim(k8stypes.UID("foobar"))
	require.Equal(t, mgr.CountClaims(), 0, "empty allocationManager has claims")
	require.Equal(t, mgr.CountPods(), 0, "empty allocationManager has pods")

	_, ok := mgr.GetClaim("foobar")
	require.False(t, ok, "got unregistered claim")
}

func TestRegisterGetClones(t *testing.T) {
	claimAllocs := map[string]types.Allocation{
		"memory": types.Allocation{
			ResourceIdent: types.ResourceIdent{
				Kind:     types.Memory,
				Pagesize: 4 * 1024,
			},
			Amount:   16 * 4 * 1024,
			NUMAZone: 1,
		},
	}
	expected := maps.Clone(claimAllocs)

	mgr := NewManager()
	mgr.RegisterClaim(k8stypes.UID("foobar"), claimAllocs)

	claimAllocs["hugepages-2m"] = types.Allocation{}

	_, ok := mgr.GetClaim("xxx")
	require.False(t, ok, "found nonexistent claim")

	got, ok := mgr.GetClaim("foobar")
	require.True(t, ok, "missing expected claim")
	if diff := cmp.Diff(got, expected); diff != "" {
		t.Fatalf("unexpected diff: %s", diff)
	}
}

func TestRegisterUpdatesExistingData(t *testing.T) {
	claimAllocs := map[string]types.Allocation{
		"memory": types.Allocation{
			ResourceIdent: types.ResourceIdent{
				Kind:     types.Memory,
				Pagesize: 4 * 1024,
			},
			Amount:   16 * 4 * 1024,
			NUMAZone: 1,
		},
	}

	mgr := NewManager()
	mgr.RegisterClaim(k8stypes.UID("foobar"), claimAllocs)

	claimAllocs["hugepages-2m"] = types.Allocation{
		ResourceIdent: types.ResourceIdent{
			Kind:     types.Hugepages,
			Pagesize: 2 * 1024 * 1024,
		},
		Amount:   16 * 2 * 1024 * 1024,
		NUMAZone: 1,
	}
	mgr.RegisterClaim(k8stypes.UID("foobar"), claimAllocs)

	expected := maps.Clone(claimAllocs)

	got, ok := mgr.GetClaim("foobar")
	require.True(t, ok, "can't find expected claim")
	if diff := cmp.Diff(got, expected); diff != "" {
		t.Fatalf("unexpected diff: %s", diff)
	}
}

func TestCannotDeleteIfUnbounded(t *testing.T) {
	claimAllocs := map[string]types.Allocation{
		"memory": types.Allocation{
			ResourceIdent: types.ResourceIdent{
				Kind:     types.Memory,
				Pagesize: 4 * 1024,
			},
			Amount:   16 * 4 * 1024,
			NUMAZone: 1,
		},
	}
	expected := maps.Clone(claimAllocs)

	mgr := NewManager()
	mgr.RegisterClaim(k8stypes.UID("foobar"), claimAllocs)
	require.Equal(t, mgr.CountClaims(), 1)
	require.Equal(t, mgr.CountPods(), 0)

	mgr.UnregisterClaimsForPod("pod-AAA")
	require.Equal(t, mgr.CountClaims(), 1)
	require.Equal(t, mgr.CountPods(), 0)

	got, ok := mgr.GetClaim("foobar")
	require.True(t, ok, "can't find expected claim")
	if diff := cmp.Diff(got, expected); diff != "" {
		t.Fatalf("unexpected diff: %s", diff)
	}
}

func TestUnregisterByPod(t *testing.T) {
	mgr := NewManager()

	mgr.RegisterClaim(k8stypes.UID("foo"), map[string]types.Allocation{
		"memory": types.Allocation{
			ResourceIdent: types.ResourceIdent{
				Kind:     types.Memory,
				Pagesize: 4 * 1024,
			},
			Amount:   16 * 4 * 1024,
			NUMAZone: 1,
		},
	})
	mgr.BindClaimToPod("pod-BBB", k8stypes.UID("foo"))

	mgr.RegisterClaim(k8stypes.UID("bar"), map[string]types.Allocation{
		"hugepages-2m": types.Allocation{
			ResourceIdent: types.ResourceIdent{
				Kind:     types.Hugepages,
				Pagesize: 2 * 1024 * 1024,
			},
			Amount:   16 * 2 * 1024 * 1024,
			NUMAZone: 1,
		},
	})
	mgr.BindClaimToPod("pod-BBB", k8stypes.UID("bar"))

	mgr.UnregisterClaimsForPod("pod-BBB")
	require.Equal(t, mgr.CountClaims(), 0)
	require.Equal(t, mgr.CountPods(), 0)

	var ok bool
	_, ok = mgr.GetClaim("foo")
	require.False(t, ok, "claim should be removed by podId")
	_, ok = mgr.GetClaim("bar")
	require.False(t, ok, "claim should be removed by podId")
}
