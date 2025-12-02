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
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
)

const (
	PIDSelf int = 0

	MaxValue string = "max"
)

var (
	MountPoint = "/sys/fs/cgroup"
)

func PIDToString(pid int) (string, error) {
	if pid < 0 {
		return "", errors.New("invalid pid")
	}
	if pid == PIDSelf {
		return "self", nil
	}
	return strconv.Itoa(pid), nil
}

func FullPathByPID(procRoot string, pid int) (string, error) {
	relPath, err := PathByPID(procRoot, pid)
	if err != nil {
		return "", err
	}
	return filepath.Join(MountPoint, relPath), nil
}

func PathByPID(procRoot string, pid int) (string, error) {
	ps, err := PIDToString(pid)
	if err != nil {
		return "", err
	}
	cgroupPath := filepath.Join(procRoot, "proc", ps, "cgroup")
	data, err := os.ReadFile(cgroupPath)
	if err != nil {
		return "", err
	}
	scanner := bufio.NewScanner(bytes.NewBuffer(data))
	for scanner.Scan() {
		line := scanner.Text()
		// format: "0::/some/path"
		parts := strings.SplitN(line, ":", 3)
		if len(parts) == 3 && parts[0] == "0" && parts[1] == "" {
			// Found the unified hierarchy
			return parts[2], nil
		}
	}
	return "", fmt.Errorf("cgroup v2 entry not found in %q", cgroupPath)
}

func WriteValue(lh logr.Logger, dir, file string, val int64) error {
	var value string
	if val == -1 {
		value = "max"
	} else {
		value = strconv.FormatInt(val, 10)
	}
	// differently from ParseValue, we need to bubble up the error;
	// is it arguably safe to report "no controller" as "no limits",
	// because ultimately that's the semantics; but it would be
	// very dangerous to swallow error setting limits, it would
	// break assumptions in a possibly catastrophic way.
	return WriteFile(lh, dir, file, value)
}

func ParseValue(lh logr.Logger, dir, file string) (int64, error) {
	contentRaw, err := ReadFile(lh, dir, file)
	if err != nil {
		if os.IsNotExist(err) {
			// assume no controller enabled or mounted -> no limits
			return -1, nil
		}
		return 0, err
	}
	content := strings.TrimSpace(contentRaw)
	if content == MaxValue {
		return -1, nil
	}
	val, err := strconv.ParseInt(content, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse cgroup limit value %q: %w", content, err)
	}
	return val, nil
}
