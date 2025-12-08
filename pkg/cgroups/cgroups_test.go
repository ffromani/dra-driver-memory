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

package cgroups

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"
)

func TestPIDToString(t *testing.T) {
	type testcase struct {
		pid         int
		expectedVal string
		expectedErr bool
	}

	testcases := []testcase{
		{
			pid:         PIDSelf,
			expectedVal: "self",
		},
		{
			pid:         -1,
			expectedErr: true,
		},
		{
			pid:         123,
			expectedVal: "123",
		},
	}

	for _, tcase := range testcases {
		t.Run("pid="+strconv.Itoa(tcase.pid), func(t *testing.T) {
			val, err := PIDToString(tcase.pid)
			gotErr := (err != nil)
			if gotErr != tcase.expectedErr {
				t.Fatalf("error got=%v expected=%v", gotErr, tcase.expectedErr)
			}
			if val != tcase.expectedVal {
				t.Fatalf("value got=%q expected=%q", val, tcase.expectedVal)
			}
		})
	}
}

func TestPathByPID(t *testing.T) {
	type testcase struct {
		name         string
		pid          int
		content      string
		expectedPath string
		expectedErr  bool
	}

	testcases := []testcase{
		{
			name:         "happy path - self",
			pid:          PIDSelf,
			content:      `0::/some/path`,
			expectedPath: `/some/path`,
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			procRoot := t.TempDir()

			pid, err := PIDToString(tcase.pid)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			ppath := filepath.Join(procRoot, "proc", pid)
			err = os.MkdirAll(ppath, 0755)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			err = os.WriteFile(filepath.Join(ppath, "cgroup"), []byte(tcase.content), 0444)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got, err := PathByPID(procRoot, tcase.pid)
			gotErr := (err != nil)
			if gotErr != tcase.expectedErr {
				t.Fatalf("error got=%v expected=%v", gotErr, tcase.expectedErr)
			}
			if got != tcase.expectedPath {
				t.Fatalf("value got=%q expected=%q", got, tcase.expectedPath)
			}
		})
	}
}

func TestFullPathByPID(t *testing.T) {
	type testcase struct {
		name         string
		pid          int
		content      string
		expectedPath string
		expectedErr  bool
	}

	makeTree := func(t *testing.T, tcase testcase, procRoot string) {
		t.Helper()
		if tcase.pid < 0 {
			return
		}
		pid, err := PIDToString(tcase.pid)
		require.NoError(t, err)
		ppath := filepath.Join(procRoot, "proc", pid)
		require.NoError(t, os.MkdirAll(ppath, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(ppath, "cgroup"), []byte(tcase.content), 0444))
	}

	testcases := []testcase{
		{
			name:         "happy path - self",
			pid:          PIDSelf,
			content:      `0::/some/path`,
			expectedPath: MountPoint + `/some/path`,
		},
		{
			name:         "happy path - specific pid",
			pid:          12345,
			content:      `0::/kubelet.slice/container.scope`,
			expectedPath: MountPoint + `/kubelet.slice/container.scope`,
		},
		{
			name:        "invalid pid",
			pid:         -1,
			expectedErr: true,
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			procRoot := t.TempDir()

			makeTree(t, tcase, procRoot)

			got, err := FullPathByPID(procRoot, tcase.pid)
			gotErr := (err != nil)
			if gotErr != tcase.expectedErr {
				t.Fatalf("error got=%v expected=%v (err=%v)", gotErr, tcase.expectedErr, err)
			}
			if got != tcase.expectedPath {
				t.Fatalf("value got=%q expected=%q", got, tcase.expectedPath)
			}
		})
	}
}

func TestPathByPIDErrors(t *testing.T) {
	type testcase struct {
		name        string
		pid         int
		content     string
		setupProc   bool
		expectedErr bool
	}

	makeTree := func(t *testing.T, tcase testcase, procRoot string) {
		t.Helper()
		if !tcase.setupProc {
			return
		}
		pid, err := PIDToString(tcase.pid)
		require.NoError(t, err)
		ppath := filepath.Join(procRoot, "proc", pid)
		require.NoError(t, os.MkdirAll(ppath, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(ppath, "cgroup"), []byte(tcase.content), 0444))
	}

	testcases := []testcase{
		{
			name:        "cgroup file does not exist",
			pid:         PIDSelf,
			setupProc:   false,
			expectedErr: true,
		},
		{
			name:        "no cgroup v2 entry in file",
			pid:         PIDSelf,
			content:     `1:cpuset:/`,
			setupProc:   true,
			expectedErr: true,
		},
		{
			name:        "multiple entries, cgroup v2 found",
			pid:         PIDSelf,
			content:     "1:cpuset:/\n0::/unified/path\n2:memory:/",
			setupProc:   true,
			expectedErr: false,
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			procRoot := t.TempDir()

			makeTree(t, tcase, procRoot)

			_, err := PathByPID(procRoot, tcase.pid)
			gotErr := (err != nil)
			if gotErr != tcase.expectedErr {
				t.Fatalf("error got=%v expected=%v (err=%v)", gotErr, tcase.expectedErr, err)
			}
		})
	}
}

func TestParseValue(t *testing.T) {
	TestMode = true
	t.Cleanup(func() { TestMode = false })

	type testcase struct {
		name        string
		content     string
		expected    int64
		createFile  bool
		expectedErr bool
	}

	testcases := []testcase{
		{
			name:       "max value",
			content:    "max\n",
			expected:   -1,
			createFile: true,
		},
		{
			name:       "numeric value",
			content:    "1048576\n",
			expected:   1048576,
			createFile: true,
		},
		{
			name:       "zero value",
			content:    "0\n",
			expected:   0,
			createFile: true,
		},
		{
			name:       "file does not exist - no limits",
			createFile: false,
			expected:   -1,
		},
		{
			name:        "invalid content",
			content:     "invalid\n",
			createFile:  true,
			expectedErr: true,
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			lh := testr.New(t)
			tmpDir := t.TempDir()

			if tcase.createFile {
				err := os.WriteFile(filepath.Join(tmpDir, "test.max"), []byte(tcase.content), 0644)
				require.NoError(t, err)
			}

			got, err := ParseValue(lh, tmpDir, "test.max")
			if tcase.expectedErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tcase.expected, got)
		})
	}
}

func TestWriteValue(t *testing.T) {
	TestMode = true
	t.Cleanup(func() { TestMode = false })

	type testcase struct {
		name            string
		value           int64
		expectedContent string
	}

	testcases := []testcase{
		{
			name:            "max value",
			value:           -1,
			expectedContent: "max",
		},
		{
			name:            "numeric value",
			value:           2097152,
			expectedContent: "2097152",
		},
		{
			name:            "zero value",
			value:           0,
			expectedContent: "0",
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			lh := testr.New(t)
			tmpDir := t.TempDir()
			fileName := "hugetlb.2MB.max"

			err := WriteValue(lh, tmpDir, fileName, tcase.value)
			require.NoError(t, err)

			content, err := os.ReadFile(filepath.Join(tmpDir, fileName))
			require.NoError(t, err)
			require.Equal(t, tcase.expectedContent, string(content))
		})
	}
}

func TestWriteFile(t *testing.T) {
	TestMode = true
	t.Cleanup(func() { TestMode = false })

	lh := testr.New(t)
	tmpDir := t.TempDir()
	fileName := "test.file"

	err := WriteFile(lh, tmpDir, fileName, "test content")
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(tmpDir, fileName))
	require.NoError(t, err)
	require.Equal(t, "test content", string(content))
}

func TestOpenFileEmptyDir(t *testing.T) {
	lh := testr.New(t)

	_, err := OpenFile(lh, "", "somefile", os.O_RDONLY)
	require.Error(t, err)
}
