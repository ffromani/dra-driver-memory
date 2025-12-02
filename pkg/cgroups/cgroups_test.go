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
