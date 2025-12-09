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

package cdi

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	cdiSpec "tags.cncf.io/container-device-interface/specs-go"
)

const (
	testDriverName = "dramem.test"
)

type testdevice struct {
	name string
	envs []string
}

func TestAddDevice(t *testing.T) {
	type testcase struct {
		name         string
		devices      []testdevice
		expectedSpec *cdiSpec.Spec
	}

	testcases := []testcase{
		{
			name: "empty",
			expectedSpec: &cdiSpec.Spec{
				Version: SpecVersion,
				Kind:    Vendor + "/" + Class,
				Devices: []cdiSpec.Device{},
			},
		},
		{
			name: "simple device",
			devices: []testdevice{
				{
					name: "foodev",
					envs: []string{
						"FOO=42",
					},
				},
			},
			expectedSpec: &cdiSpec.Spec{
				Version: SpecVersion,
				Kind:    Vendor + "/" + Class,
				Devices: []cdiSpec.Device{
					{
						Name: "foodev",
						ContainerEdits: cdiSpec.ContainerEdits{
							Env: []string{
								"FOO=42",
							},
						},
					},
				},
			},
		},
		{
			name: "device multienv",
			devices: []testdevice{
				{
					name: "foodev",
					envs: []string{
						"FOO=42",
						"BAR=Y",
						"FIZZ_42=buzz",
					},
				},
			},
			expectedSpec: &cdiSpec.Spec{
				Version: SpecVersion,
				Kind:    Vendor + "/" + Class,
				Devices: []cdiSpec.Device{
					{
						Name: "foodev",
						ContainerEdits: cdiSpec.ContainerEdits{
							Env: []string{
								"FOO=42",
								"BAR=Y",
								"FIZZ_42=buzz",
							},
						},
					},
				},
			},
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			saveCDIDir := SpecDir
			t.Cleanup(func() {
				SpecDir = saveCDIDir
			})
			SpecDir = t.TempDir()
			logger := testr.New(t)

			mgr, err := NewManager(testDriverName, logger)
			require.NoError(t, err)

			_, err = os.Stat(filepath.Join(SpecDir, testDriverName+".json"))
			require.NoError(t, err)

			for _, dev := range tcase.devices {
				err = mgr.AddDevice(logger, dev.name, dev.envs...)
				require.NoError(t, err)
			}

			got, err := mgr.GetSpec(logger)
			require.NoError(t, err)
			if diff := cmp.Diff(got, tcase.expectedSpec); diff != "" {
				t.Errorf("unexpected spec from empty: %v", diff)
			}
		})
	}
}

func TestRemoveDevice(t *testing.T) {
	type testcase struct {
		name         string
		initial      []testdevice
		toRemove     []testdevice
		expectedSpec *cdiSpec.Spec
	}

	testcases := []testcase{
		{
			name: "multi device",
			initial: []testdevice{
				{
					name: "foodev",
					envs: []string{
						"FOO=42",
					},
				},
				{
					name: "bardev",
					envs: []string{
						"GO=1",
					},
				},
				{
					name: "fizzbuzzdev",
					envs: []string{
						"SEQ=3,5,15",
					},
				},
			},
			toRemove: []testdevice{
				{
					name: "fizzbuzzdev",
				},
			},
			expectedSpec: &cdiSpec.Spec{
				Version: SpecVersion,
				Kind:    Vendor + "/" + Class,
				Devices: []cdiSpec.Device{
					{
						Name: "foodev",
						ContainerEdits: cdiSpec.ContainerEdits{
							Env: []string{
								"FOO=42",
							},
						},
					},
					{
						Name: "bardev",
						ContainerEdits: cdiSpec.ContainerEdits{
							Env: []string{
								"GO=1",
							},
						},
					},
				},
			},
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			saveCDIDir := SpecDir
			t.Cleanup(func() {
				SpecDir = saveCDIDir
			})
			SpecDir = t.TempDir()
			logger := testr.New(t)

			mgr, err := NewManager(testDriverName, logger)
			require.NoError(t, err)
			for _, dev := range tcase.initial {
				err = mgr.AddDevice(logger, dev.name, dev.envs...)
				require.NoError(t, err)
			}
			for _, dev := range tcase.toRemove {
				err = mgr.RemoveDevice(logger, dev.name)
				require.NoError(t, err)
			}

			got, err := mgr.GetSpec(logger)
			require.NoError(t, err)
			if diff := cmp.Diff(got, tcase.expectedSpec); diff != "" {
				t.Errorf("unexpected spec from empty: %v", diff)
			}
		})
	}
}

func TestRemoveDeviceNotFound(t *testing.T) {
	saveCDIDir := SpecDir
	t.Cleanup(func() {
		SpecDir = saveCDIDir
	})
	SpecDir = t.TempDir()
	logger := testr.New(t)

	mgr, err := NewManager(testDriverName, logger)
	require.NoError(t, err)

	// Remove a device that doesn't exist - should not error
	err = mgr.RemoveDevice(logger, "nonexistent")
	require.NoError(t, err)
}

func TestRemoveDeviceFileGone(t *testing.T) {
	saveCDIDir := SpecDir
	t.Cleanup(func() {
		SpecDir = saveCDIDir
	})
	SpecDir = t.TempDir()
	logger := testr.New(t)

	mgr, err := NewManager(testDriverName, logger)
	require.NoError(t, err)

	// Delete the spec file to simulate it being removed externally
	err = os.Remove(filepath.Join(SpecDir, testDriverName+".json"))
	require.NoError(t, err)

	// RemoveDevice should handle missing file gracefully and return nil
	err = mgr.RemoveDevice(logger, "anydevice")
	require.NoError(t, err, "RemoveDevice should return nil when spec file is gone")
}

func TestNewManagerExistingSpec(t *testing.T) {
	saveCDIDir := SpecDir
	t.Cleanup(func() {
		SpecDir = saveCDIDir
	})
	SpecDir = t.TempDir()
	logger := testr.New(t)

	// Create an initial manager and add a device
	mgr1, err := NewManager(testDriverName, logger)
	require.NoError(t, err)
	err = mgr1.AddDevice(logger, "existingdev", "VAR=value")
	require.NoError(t, err)

	// Create a new manager - should load existing spec
	mgr2, err := NewManager(testDriverName, logger)
	require.NoError(t, err)

	spec, err := mgr2.GetSpec(logger)
	require.NoError(t, err)
	require.Len(t, spec.Devices, 1)
	require.Equal(t, "existingdev", spec.Devices[0].Name)
}

func TestEmptySpec(t *testing.T) {
	saveCDIDir := SpecDir
	t.Cleanup(func() {
		SpecDir = saveCDIDir
	})
	SpecDir = t.TempDir()
	logger := testr.New(t)

	mgr, err := NewManager(testDriverName, logger)
	require.NoError(t, err)

	spec := mgr.EmptySpec()
	require.Equal(t, SpecVersion, spec.Version)
	require.Equal(t, Vendor+"/"+Class, spec.Kind)
	require.Empty(t, spec.Devices)
}
