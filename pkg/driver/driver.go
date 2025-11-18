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

package driver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/containerd/nri/pkg/stub"
	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/dynamic-resource-allocation/kubeletplugin"
	"k8s.io/dynamic-resource-allocation/resourceslice"
	"k8s.io/klog/v2"

	"github.com/ffromani/dra-driver-memory/pkg/alloc"
	"github.com/ffromani/dra-driver-memory/pkg/cdi"
	"github.com/ffromani/dra-driver-memory/pkg/sysinfo"
)

// This is the orchestration layer. All the sub-components (DRA layer, NRI layer, CDI manager...)
// are coordinated here. Besides glue code, no logic should be present here.
// Push any nontrivial logic into a subcomponent.

const (
	Name = "dra.memory"
)

const (
	kubeletPluginPath = "/var/lib/kubelet/plugins"
	// maxAttempts indicates the number of times the driver will try to recover itself before failing
	maxAttempts = 5
)

// KubeletPlugin is an interface that describes the methods used from kubeletplugin.Helper.
type KubeletPlugin interface {
	PublishResources(context.Context, resourceslice.DriverResources) error
	Stop()
}

type MemoryDriver struct {
	driverName string
	nodeName   string
	logger     logr.Logger
	kubeClient kubernetes.Interface
	draPlugin  KubeletPlugin
	nriPlugin  stub.Stub
	cdiMgr     *cdi.Manager
	allocMgr   *alloc.Manager
	discoverer *sysinfo.Discoverer
}

type SysinfoVerifier interface {
	Validate() error
}

type SysinfoDiscoverer interface {
	Discover() (sysinfo.MachineData, error)
}

type Environment struct {
	Logger      logr.Logger
	DriverName  string
	NodeName    string
	Clientset   kubernetes.Interface
	SysVerifier SysinfoVerifier
	SysRoot     string
}

// Start creates and starts a new MemoryDriver.
func Start(ctx context.Context, env Environment) (*MemoryDriver, error) {
	if err := env.SysVerifier.Validate(); err != nil {
		return nil, err
	}
	plugin := &MemoryDriver{
		driverName: env.DriverName,
		nodeName:   env.NodeName,
		kubeClient: env.Clientset,
		logger:     env.Logger.WithName(env.DriverName),
		allocMgr:   alloc.NewManager(),
		discoverer: sysinfo.NewDiscoverer(env.SysRoot),
	}

	driverPluginPath := filepath.Join(kubeletPluginPath, env.DriverName)
	if err := os.MkdirAll(driverPluginPath, 0750); err != nil {
		return nil, fmt.Errorf("failed to create plugin path %s: %w", driverPluginPath, err)
	}

	kubeletOpts := []kubeletplugin.Option{
		kubeletplugin.DriverName(env.DriverName),
		kubeletplugin.NodeName(env.NodeName),
		kubeletplugin.KubeClient(env.Clientset),
	}
	draDrv, err := kubeletplugin.Start(ctx, plugin, kubeletOpts...)
	if err != nil {
		return nil, fmt.Errorf("start kubelet plugin: %w", err)
	}
	plugin.draPlugin = draDrv
	err = wait.PollUntilContextTimeout(ctx, 1*time.Second, 30*time.Second, true, func(context.Context) (bool, error) {
		status := draDrv.RegistrationStatus()
		if status == nil {
			return false, nil
		}
		return status.PluginRegistered, nil
	})
	if err != nil {
		return nil, err
	}

	cdiMgr, err := cdi.NewManager(env.DriverName, env.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create CDI manager: %w", err)
	}
	plugin.cdiMgr = cdiMgr

	// register the NRI plugin
	nriOpts := []stub.Option{
		stub.WithPluginName(env.DriverName),
		stub.WithPluginIdx("00"),
		// https://github.com/containerd/nri/pull/173
		// Otherwise it silently exits the program
		stub.WithOnClose(func() {
			klog.Infof("%s NRI plugin closed", env.DriverName)
		}),
	}
	stub, err := stub.New(plugin, nriOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create plugin stub: %w", err)
	}
	plugin.nriPlugin = stub

	go func() {
		for i := 0; i < maxAttempts; i++ {
			err = plugin.nriPlugin.Run(ctx)
			if err != nil {
				klog.Infof("NRI plugin failed with error %v", err)
			}
			select {
			case <-ctx.Done():
				return
			default:
				klog.Infof("Restarting NRI plugin %d out of %d", i, maxAttempts)
			}
		}
		klog.Fatalf("NRI plugin failed for %d times to be restarted", maxAttempts)
	}()

	// publish available resources
	go plugin.PublishResources(ctx)

	return plugin, nil
}

func (mdrv *MemoryDriver) Stop() {
	lh := mdrv.logger // alias
	lh.V(3).Info("Driver stopping...")
}

// Shutdown is called when the runtime is shutting down.
func (mdrv *MemoryDriver) Shutdown(ctx context.Context) {
	lh := mdrv.logrFromContext(ctx)
	lh.V(3).Info("Driver shutting down...")
}

func (mdrv *MemoryDriver) logrFromContext(ctx context.Context) logr.Logger {
	lh, err := logr.FromContext(ctx)
	if err != nil {
		return mdrv.logger.WithName("nri")
	}
	return lh
}
