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

package provision

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"

	apiv0 "github.com/ffromani/dra-driver-memory/pkg/hugepages/provision/api/v0"
)

func TestReadConfiguration(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "test-provision-2m.yaml")
	require.NoError(t, os.WriteFile(confPath, []byte(provision2M), 0600))

	hpConf, err := ReadConfiguration(confPath)
	require.NoError(t, err)
	require.Equal(t, hpConf.Name, "balanced-runtime")
	require.Len(t, hpConf.Spec.Pages, 1)
	require.Equal(t, hpConf.Spec.Pages[0].Size, apiv0.HugePageSize("2M"))
	require.Equal(t, hpConf.Spec.Pages[0].Count, int32(4096))
}

func TestProvisionBaseSingleNode(t *testing.T) {
	lh := testr.New(t)

	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "test-provision-2m.yaml")
	require.NoError(t, os.WriteFile(confPath, []byte(provision2M), 0600))

	hpPath := filepath.Join(tmpDir, "sys", "devices", "system", "node", "node0", "hugepages", "hugepages-1048576kB")
	lh.Info("creating test path", "hpPath", hpPath)
	require.NoError(t, os.MkdirAll(hpPath, 0755))

	hpPath = filepath.Join(tmpDir, "sys", "devices", "system", "node", "node0", "hugepages", "hugepages-2048kB")
	lh.Info("creating test path", "hpPath", hpPath)
	require.NoError(t, os.MkdirAll(hpPath, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(hpPath, "nr_hugepages"), []byte(""), 0600))

	hpConf, err := ReadConfiguration(confPath)
	require.NoError(t, err)

	require.NoError(t, RuntimeHugepages(lh, hpConf, tmpDir, 1))

	hpPath = filepath.Join(tmpDir, "sys", "devices", "system", "node", "node0", "hugepages", "hugepages-1048576kB")
	dents, err := os.ReadDir(hpPath)
	require.NoError(t, err)
	require.Empty(t, dents)

	data, err := os.ReadFile(filepath.Join(tmpDir, "sys", "devices", "system", "node", "node0", "hugepages", "hugepages-2048kB", "nr_hugepages"))
	require.NoError(t, err)
	numPages, err := strconv.Atoi(string(data))
	require.NoError(t, err)

	require.Equal(t, numPages, 4096)
}

func TestProvisionBaseMultiNode(t *testing.T) {
	lh := testr.New(t)

	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "test-provision-2m.yaml")
	require.NoError(t, os.WriteFile(confPath, []byte(provision2M), 0600))

	numaZones := 4
	for nn := 0; nn < numaZones; nn++ {
		hpPath := filepath.Join(tmpDir, "sys", "devices", "system", "node", fmt.Sprintf("node%d", nn), "hugepages", "hugepages-1048576kB")
		lh.Info("creating test path", "hpPath", hpPath)
		require.NoError(t, os.MkdirAll(hpPath, 0755))

		hpPath = filepath.Join(tmpDir, "sys", "devices", "system", "node", fmt.Sprintf("node%d", nn), "hugepages", "hugepages-2048kB")
		lh.Info("creating test path", "hpPath", hpPath)
		require.NoError(t, os.MkdirAll(hpPath, 0755))

		require.NoError(t, os.WriteFile(filepath.Join(hpPath, "nr_hugepages"), []byte(""), 0600))
	}

	hpConf, err := ReadConfiguration(confPath)
	require.NoError(t, err)

	require.NoError(t, RuntimeHugepages(lh, hpConf, tmpDir, numaZones))

	for nn := 0; nn < numaZones; nn++ {
		hpPath := filepath.Join(tmpDir, "sys", "devices", "system", "node", fmt.Sprintf("node%d", nn), "hugepages", "hugepages-1048576kB")
		dents, err := os.ReadDir(hpPath)
		require.NoError(t, err)
		require.Empty(t, dents)

		data, err := os.ReadFile(filepath.Join(tmpDir, "sys", "devices", "system", "node", fmt.Sprintf("node%d", nn), "hugepages", "hugepages-2048kB", "nr_hugepages"))
		require.NoError(t, err)
		numPages, err := strconv.Atoi(string(data))
		require.NoError(t, err)

		require.Equal(t, numPages, 1024) // 4096 HPs evenly split on 4 NUMA Zones
	}
}

const provision2M = `kind: HugePageProvision
metadata:
  name: balanced-runtime
spec:
  pages:
  - size: "2M"
    count: 4096`
