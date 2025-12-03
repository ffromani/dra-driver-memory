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

package memalign

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-logr/logr"

	"k8s.io/utils/cpuset"
)

const (
	PIDSelf int = 0
)

// NUMANodesByPID returns the set of NUMA Nodes from which the process
// identified by <pid> actually allocated memory at time of check.
// The NUMA Nodes set is returned as CPUSet because this is the most
// popular albeit not the most correct data structure used.
// On error, the returned CPUSet is empty and the error value is not-nil.
func NUMANodesByPID(lh logr.Logger, pid int, procRoot string) (cpuset.CPUSet, error) {
	fullPath := filepath.Join(procRoot, makeProcPath(pid))
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return cpuset.CPUSet{}, err
	}
	var numaNodes []int
	scanner := bufio.NewScanner(bytes.NewBuffer(data))
	for scanner.Scan() {
		line := scanner.Text()
		items := strings.Fields(line)
		// colums:
		// <address> <policy> [properties...] [node_usage...]
		if len(items) <= 2 {
			continue
		}
		for _, attr := range items[2:] {
			if strings.HasPrefix(attr, "file=") {
				// we must skip file-backed memory regions.
				// these are read-only, don't obey the `cpuset.mem` restriction
				// and are almost always irrelevant for performance.
				// abort scanning this line
				break
			}
			if !strings.HasPrefix(attr, "N") {
				continue
			}
			attrItems := strings.SplitN(attr, "=", 2)
			if len(attrItems) != 2 {
				lh.Info("unexpected attr item count", "attr", attr, "count", len(attrItems))
				continue
			}
			key := attrItems[0]
			numaNode, err := strconv.Atoi(key[1:])
			if err != nil {
				lh.Error(err, "parsing attr %q", attr)
				continue
			}
			numaNodes = append(numaNodes, numaNode)
		}
	}
	return cpuset.New(numaNodes...), nil
}

func makeProcPath(pid int) string {
	// we intentionally use self over thread-self
	pidStr := "self"
	if pid != PIDSelf {
		pidStr = strconv.Itoa(pid)
	}
	return filepath.Join("proc", pidStr, "numa_maps")
}
