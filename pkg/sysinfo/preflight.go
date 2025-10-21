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
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/moby/sys/mountinfo"
)

const (
	cgroup2FSType = "cgroup2"
)

var (
	ErrMemoryHugeTLBAccounting = errors.New("memory hugetlb accounting not supported")
)

func Validate(lh logr.Logger, procRoot string) error {
	mounts, err := mountinfo.GetMounts(mountinfo.FSTypeFilter(cgroup2FSType))
	if err != nil {
		return fmt.Errorf("discovering mount infos: %w", err)
	}
	for _, mount := range mounts {
		if mount.FSType != cgroup2FSType {
			continue
		}
		if strings.Contains(mount.Options, "memory_hugetlb_accounting") {
			return ErrMemoryHugeTLBAccounting
		}
	}
	return nil
}
