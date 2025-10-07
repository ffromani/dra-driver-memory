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

	"github.com/containerd/nri/pkg/api"

	"k8s.io/klog/v2"
)

func (cp *MemoryDriver) Synchronize(ctx context.Context, pods []*api.PodSandbox, containers []*api.Container) ([]*api.ContainerUpdate, error) {
	klog.Infof("Synchronized state with the runtime (%d pods, %d containers)...",
		len(pods), len(containers))
	return nil, nil
}

func (cp *MemoryDriver) CreateContainer(_ context.Context, pod *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	adjust := &api.ContainerAdjustment{}
	var updates []*api.ContainerUpdate
	return adjust, updates, nil
}

func (cp *MemoryDriver) StopContainer(_ context.Context, pod *api.PodSandbox, ctr *api.Container) ([]*api.ContainerUpdate, error) {
	klog.Infof("StopContainer Pod:%s/%s PodUID:%s Container:%s ContainerID:%s", pod.Namespace, pod.Name, pod.Uid, ctr.Name, ctr.Id)
	return nil, nil
}

// RemoveContainer handles container removal requests from the NRI.
func (cp *MemoryDriver) RemoveContainer(_ context.Context, pod *api.PodSandbox, ctr *api.Container) error {
	klog.Infof("RemoveContainer Pod:%s/%s PodUID:%s Container:%s ContainerID:%s", pod.Namespace, pod.Name, pod.Uid, ctr.Name, ctr.Id)
	return nil
}

// RunPodSandbox handles pod sandbox creation requests from the NRI.
func (cp *MemoryDriver) RunPodSandbox(_ context.Context, pod *api.PodSandbox) error {
	klog.Infof("RunPodSandbox Pod %s/%s UID %s", pod.Namespace, pod.Name, pod.Uid)
	return nil
}

// StopPodSandbox handles pod sandbox stop requests from the NRI.
func (cp *MemoryDriver) StopPodSandbox(_ context.Context, pod *api.PodSandbox) error {
	klog.Infof("StopPodSandbox Pod %s/%s UID %s", pod.Namespace, pod.Name, pod.Uid)
	return nil
}

// RemovePodSandbox handles pod sandbox removal requests from the NRI.
func (cp *MemoryDriver) RemovePodSandbox(_ context.Context, pod *api.PodSandbox) error {
	klog.Infof("RemovePodSandbox Pod %s/%s UID %s", pod.Namespace, pod.Name, pod.Uid)
	return nil
}
