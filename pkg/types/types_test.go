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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResourceIdentNameRoundTrip(t *testing.T) {
	type testcase struct {
		fullName string
		name     string
		hugeTLB  bool
		ident    ResourceIdent
	}

	testcases := []testcase{
		{
			fullName: "memory-4k",
			name:     "memory",
			ident: ResourceIdent{
				Kind:     Memory,
				Pagesize: 4 * 1024,
			},
		},
		{
			fullName: "hugepages-2m",
			name:     "hugepages-2m",
			hugeTLB:  true,
			ident: ResourceIdent{
				Kind:     Hugepages,
				Pagesize: 2 * 1024 * 1024,
			},
		},
		{
			fullName: "hugepages-1g",
			name:     "hugepages-1g",
			hugeTLB:  true,
			ident: ResourceIdent{
				Kind:     Hugepages,
				Pagesize: 1024 * 1024 * 1024,
			},
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.fullName, func(t *testing.T) {
			gotIdent, err := ResourceIdentFromName(tcase.fullName)
			require.NoError(t, err)
			require.Equal(t, gotIdent.FullName(), tcase.fullName)
			require.Equal(t, gotIdent.Name(), tcase.name)
			require.Equal(t, gotIdent, tcase.ident)
			require.Equal(t, gotIdent.NeedsHugeTLB(), tcase.hugeTLB)
		})
	}
}
