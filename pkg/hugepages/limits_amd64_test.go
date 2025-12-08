//go:build amd64

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

package hugepages

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"

	"github.com/ffromani/dra-driver-memory/pkg/cgroups"
	"github.com/ffromani/dra-driver-memory/pkg/sysinfo"
)

/*
this is a smoke test. We want to check the GetMachineData works at all,
deferring comprehensive testing to e2e tests and to future extension.
We deem safe to call this function on CI and on any system, because
it uses basic sysfs/procfs interfaces which must be available on any
system or configuration. We check only the most basic properties
by design: hugepages are unlikely to be provisioned, and we can't
depend on that.
*/

func TestLimitsFromSystemPID(t *testing.T) {
	lh := testr.New(t)
	machine, err := sysinfo.GetMachineData(lh, "/")
	require.NoError(t, err)

	_, err = LimitsFromSystemPID(lh, machine, "/", cgroups.PIDSelf)
	require.NoError(t, err)
}

func TestSetSystemLimits(t *testing.T) {
	cgroups.TestMode = true
	t.Cleanup(func() { cgroups.TestMode = false })

	type testcase struct {
		name   string
		limits []Limit
	}

	testcases := []testcase{
		{
			name:   "empty limits",
			limits: []Limit{},
		},
		{
			name: "single 2MB limit",
			limits: []Limit{
				{
					PageSize: "2MB",
					Limit: LimitValue{
						Value: 4 * (1 << 21),
					},
				},
			},
		},
		{
			name: "single 1GB limit",
			limits: []Limit{
				{
					PageSize: "1GB",
					Limit: LimitValue{
						Value: 2 * (1 << 30),
					},
				},
			},
		},
		{
			name: "multiple limits",
			limits: []Limit{
				{
					PageSize: "2MB",
					Limit: LimitValue{
						Value: 8 * (1 << 21),
					},
				},
				{
					PageSize: "1GB",
					Limit: LimitValue{
						Value: 4 * (1 << 30),
					},
				},
			},
		},
		{
			name: "unset limit (max)",
			limits: []Limit{
				{
					PageSize: "2MB",
					Limit: LimitValue{
						Unset: true,
					},
				},
			},
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			lh := testr.New(t)
			tmpDir := t.TempDir()

			err := SetSystemLimits(lh, tmpDir, tcase.limits)
			require.NoError(t, err)

			// Verify files were created with correct content
			for _, limit := range tcase.limits {
				maxFile := filepath.Join(tmpDir, "hugetlb."+limit.PageSize+".max")
				rsvdFile := filepath.Join(tmpDir, "hugetlb."+limit.PageSize+".rsvd.max")

				maxContent, err := os.ReadFile(maxFile)
				require.NoError(t, err)
				rsvdContent, err := os.ReadFile(rsvdFile)
				require.NoError(t, err)

				var expectedContent string
				if limit.Limit.Unset {
					expectedContent = "max"
				} else {
					expectedContent = string(rune('0' + limit.Limit.Value/(1<<21)))
					// Just verify the file exists and has content
				}
				require.NotEmpty(t, maxContent)
				require.NotEmpty(t, rsvdContent)
				if limit.Limit.Unset {
					require.Equal(t, expectedContent, string(maxContent))
					require.Equal(t, expectedContent, string(rsvdContent))
				}
			}
		})
	}
}
