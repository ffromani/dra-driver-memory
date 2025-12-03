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

package memalign

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"

	"k8s.io/utils/cpuset"
)

func TestNUMANodesByPID(t *testing.T) {
	type testcase struct {
		name     string
		pid      int
		testFile string
		expected cpuset.CPUSet
	}

	testcases := []testcase{
		{
			name:     "simplest single-NUMA, with self",
			pid:      PIDSelf,
			testFile: "numa_maps_simple.01.txt",
			expected: cpuset.New(0),
		},
		{
			name:     "simple multi-NUMA, with self",
			pid:      PIDSelf,
			testFile: "numa_maps_multinuma.01.txt",
			expected: cpuset.New(0, 1),
		},
		{
			name:     "simple multi-NUMA, with self, pinning ok but file-backed cross-NUMA",
			pid:      PIDSelf,
			testFile: "numa_maps_multinuma.02.txt",
			expected: cpuset.New(0),
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			logger := testr.New(t)
			tmpDir := t.TempDir()
			err := setupNUMAMaps(tmpDir, tcase.pid, tcase.testFile)
			require.NoError(t, err)
			got, err := NUMANodesByPID(logger, tcase.pid, tmpDir)
			require.NoError(t, err)
			ok := tcase.expected.Equals(got)
			require.True(t, ok, "expected NUMA nodes: %q got: %q", tcase.expected.String(), got.String())
		})
	}
}

// setupNUMAMaps creates the proc expected layout in the root
// directory `tmpDir`. So, if you use tmpDir="temp-foo" and
// call it with pid 123 it will create `temp-foo/proc/123/numa_maps`.
// The content to populate `numa_maps` is assumed to be in a file
// named `fileName` placed in `./testdata`
func setupNUMAMaps(tmpDir string, pid int, fileName string) error {
	fullPath := filepath.Join(tmpDir, makeProcPath(pid))
	err := os.MkdirAll(filepath.Dir(fullPath), 0755)
	if err != nil {
		return err
	}
	// we can use symlinks but on the real procfs the content
	// looks like a regular file (or at least on linux 6.17
	// is not a symlink) so we create a regular file as well
	// even if it is more complex
	data, err := os.ReadFile(filepath.Join("testdata", fileName))
	if err != nil {
		return err
	}
	return os.WriteFile(fullPath, data, 0444)
}
