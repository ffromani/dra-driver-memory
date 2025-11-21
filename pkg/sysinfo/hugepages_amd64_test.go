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
	"os"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"
)

func TestHugepageSizes(t *testing.T) {
	type testcase struct {
		name     string
		mkMMTree func(*testing.T, string)
		expected []string
	}

	testcases := []testcase{
		{
			name: "happy path",
			mkMMTree: func(t *testing.T, root string) {
				require.NoError(t, os.MkdirAll(filepath.Join(root, "sys", "kernel", "mm", "hugepages", "hugepages-2048kB"), 0755))
				require.NoError(t, os.MkdirAll(filepath.Join(root, "sys", "kernel", "mm", "hugepages", "hugepages-1048576kB"), 0755))
			},
			expected: []string{"1GB", "2MB"},
		},
		{
			name: "empty content - likely impossible",
			mkMMTree: func(t *testing.T, root string) {
				require.NoError(t, os.MkdirAll(filepath.Join(root, "sys", "kernel", "mm", "hugepages"), 0755))
			},
			expected: []string{},
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			lh := testr.New(t)
			tcase.mkMMTree(t, tmpDir)
			hpSizes := HugepageSizes(lh, tmpDir)
			require.Equal(t, hpSizes, tcase.expected)
		})
	}
}
