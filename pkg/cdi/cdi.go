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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/go-logr/logr"
	cdiSpec "tags.cncf.io/container-device-interface/specs-go"

	"k8s.io/apimachinery/pkg/types"
)

const (
	SpecVersion  = "0.8.0"
	Vendor       = "dra.k8s.io"
	Class        = "memory"
	EnvVarPrefix = "DRAMEMORY"
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

func MakeKind(vendor, class string) string {
	return vendor + "/" + class
}

// NewManager creates a manager for the driver's CDI spec file.
func NewManager(driverName string, lh logr.Logger) (*Manager, error) {
	path := filepath.Join(SpecDir, fmt.Sprintf("%s.json", driverName))
	lh = lh.WithValues("path", path)

	if err := os.MkdirAll(SpecDir, 0755); err != nil {
		return nil, fmt.Errorf("error creating CDI spec directory %q: %w", SpecDir, err)
	}

	mgr := &Manager{
		path:       path,
		cdiKind:    MakeKind(Vendor, Class),
		driverName: driverName,
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := mgr.writeSpecToFile(lh, mgr.EmptySpec()); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, fmt.Errorf("error accessing CDI spec file %q: %w", path, err)
	}

	lh.Info("Initialized CDI file manager")
	return mgr, nil
}

// AddDevice adds a device to the CDI spec file.
func (mgr *Manager) AddDevice(lh logr.Logger, deviceName string, envVars ...string) error {
	mgr.mutex.Lock()
	defer mgr.mutex.Unlock()

	lh = lh.WithName("cdi").WithValues("path", mgr.path, "device", deviceName)

	spec, err := mgr.readSpecFromFile(lh)
	if err != nil {
		return err
	}

	// Remove any existing device with the same name to make this call idempotent.
	removeDeviceFromSpec(spec, deviceName)
	newDevice := cdiSpec.Device{
		Name: deviceName,
		ContainerEdits: cdiSpec.ContainerEdits{
			Env: envVars,
		},
	}

	spec.Devices = append(spec.Devices, newDevice)
	return mgr.writeSpecToFile(lh, spec)
}

// RemoveDevice removes a device from the CDI spec file.
func (mgr *Manager) RemoveDevice(lh logr.Logger, deviceName string) error {
	mgr.mutex.Lock()
	defer mgr.mutex.Unlock()

	lh = lh.WithName("cdi").WithValues("path", mgr.path, "device", deviceName)

	spec, err := mgr.readSpecFromFile(lh)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // File already gone, nothing to do.
		}
		return err
	}

	if removeDeviceFromSpec(spec, deviceName) {
		return mgr.writeSpecToFile(lh, spec)
	}

	return nil
}

func (mgr *Manager) EmptySpec() *cdiSpec.Spec {
	return &cdiSpec.Spec{
		Version: SpecVersion,
		Kind:    mgr.cdiKind,
		Devices: []cdiSpec.Device{},
	}
}

func (mgr *Manager) GetSpec(lh logr.Logger) (*cdiSpec.Spec, error) {
	lh = lh.WithName("cdi").WithValues("path", mgr.path)
	return mgr.readSpecFromFile(lh)
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

func (c *Manager) writeSpecToFile(lh logr.Logger, spec *cdiSpec.Spec) (err error) {
	lh.V(2).Info("updating CDI spec file", "path", c.path)

	tmpFile, err := os.CreateTemp(SpecDir, c.driverName)
	if err != nil {
		return fmt.Errorf("failed to create temporary CDI spec: %w", err)
	}
	defer func() {
		// avoid file descriptor leakage or undeterministic closure
		// note we ignore the error; this is intentional because in the happy
		// path we will have a double close(), which is however harmless.
		_ = tmpFile.Close()
		if err != nil {
			_ = os.Remove(tmpFile.Name())
		}
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

func MakeDeviceName(uid types.UID) string {
	return fmt.Sprintf("claim-%s", uid)
}
