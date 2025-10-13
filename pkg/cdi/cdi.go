/*
Copyright 2025 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cdi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/go-logr/logr"
	cdiSpec "tags.cncf.io/container-device-interface/specs-go"
)

const (
	SpecVersion  = "0.8.0"
	Vendor       = "dra.k8s.io"
	Class        = "memory"
	EnvVarPrefix = "DRA_MEMORY_NODES"
)

var (
	SpecDir = "/var/run/cdi"
)

// Manager manages a single CDI JSON spec file using a mutex for thread safety.
type Manager struct {
	path       string
	mutex      sync.Mutex
	cdiKind    string
	driverName string
}

// NewManager creates a manager for the driver's CDI spec file.
func NewManager(driverName string, lh logr.Logger) (*Manager, error) {
	path := filepath.Join(SpecDir, fmt.Sprintf("%s.json", driverName))
	lh = lh.WithValues("path", path)

	if err := os.MkdirAll(SpecDir, 0755); err != nil {
		return nil, fmt.Errorf("error creating CDI spec directory %q: %w", SpecDir, err)
	}

	cdiKind := Vendor + "/" + Class
	c := &Manager{
		path:       path,
		cdiKind:    cdiKind,
		driverName: driverName,
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		initialSpec := &cdiSpec.Spec{
			Version: SpecVersion,
			Kind:    cdiKind,
			Devices: []cdiSpec.Device{},
		}
		if err := c.writeSpecToFile(lh, initialSpec); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, fmt.Errorf("error accessing CDI spec file %q: %w", path, err)
	}

	lh.Info("Initialized CDI file manager")
	return c, nil
}

// AddDevice adds a device to the CDI spec file.
func (c *Manager) AddDevice(lh logr.Logger, deviceName string, envVar string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	lh = lh.WithName("cdi").WithValues("path", c.path, "device", deviceName)

	spec, err := c.readSpecFromFile(lh)
	if err != nil {
		return err
	}

	// Remove any existing device with the same name to make this call idempotent.
	removeDeviceFromSpec(spec, deviceName)
	newDevice := cdiSpec.Device{
		Name: deviceName,
		ContainerEdits: cdiSpec.ContainerEdits{
			Env: []string{
				envVar,
			},
		},
	}

	spec.Devices = append(spec.Devices, newDevice)
	return c.writeSpecToFile(lh, spec)
}

// RemoveDevice removes a device from the CDI spec file.
func (c *Manager) RemoveDevice(lh logr.Logger, deviceName string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	lh = lh.WithName("cdi").WithValues("path", c.path, "device", deviceName)

	spec, err := c.readSpecFromFile(lh)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File already gone, nothing to do.
		}
		return err
	}

	if removeDeviceFromSpec(spec, deviceName) {
		return c.writeSpecToFile(lh, spec)
	}

	return nil
}

func removeDeviceFromSpec(spec *cdiSpec.Spec, deviceName string) bool {
	deviceFound := false
	newDevices := []cdiSpec.Device{}
	for _, d := range spec.Devices {
		if d.Name != deviceName {
			newDevices = append(newDevices, d)
		} else {
			deviceFound = true
		}
	}
	spec.Devices = newDevices
	return deviceFound
}

func (c *Manager) readSpecFromFile(lh logr.Logger) (*cdiSpec.Spec, error) {
	data, err := os.ReadFile(c.path)
	if err != nil {
		return nil, fmt.Errorf("error reading CDI spec file %q: %w", c.path, err)
	}

	if len(data) == 0 {
		return &cdiSpec.Spec{
			Version: SpecVersion,
			Kind:    c.cdiKind,
			Devices: []cdiSpec.Device{},
		}, nil
	}

	spec := &cdiSpec.Spec{}
	if err := json.Unmarshal(data, spec); err != nil {
		return nil, fmt.Errorf("error unmarshaling CDI spec from %q: %w", c.path, err)
	}
	lh.V(2).Info("Read CDI spec", "spec", spec)
	return spec, nil
}

func (c *Manager) writeSpecToFile(lh logr.Logger, spec *cdiSpec.Spec) error {
	lh.V(2).Info("updating CDI spec file", "path", c.path)

	tmpFile, err := os.CreateTemp(SpecDir, c.driverName)
	if err != nil {
		return fmt.Errorf("failed to create temporary CDI spec: %w", err)
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling CDI spec: %w", err)
	}

	lh.V(6).Info("updating temporary CDI spec")
	if _, err := tmpFile.Write(data); err != nil {
		return fmt.Errorf("failed to write to temporary CDI spec: %w", err)
	}

	lh.V(6).Info("syncing temporary CDI spec")
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temporary CDI spec: %w", err)
	}

	lh.V(6).Info("finalizing file content")
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary CDI spec: %w", err)
	}

	if err := os.Rename(tmpFile.Name(), c.path); err != nil {
		return fmt.Errorf("failed to rename temporary CDI spec: %w", err)
	}

	lh.V(2).Info("Successfully updated and synced CDI spec file")
	return nil
}
