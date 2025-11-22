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

package sysinfo

import (
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"
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

func TestGetMachineDataSmoke(t *testing.T) {
	lh := testr.New(t)
	machine, err := GetMachineData(lh, "/")
	require.NoError(t, err)
	require.Equal(t, machine.Pagesize, uint64(4*(1<<10)))
	require.Equal(t, machine.Hugepagesizes, []uint64{2 * 1 << 20, 1 << 30})
}
