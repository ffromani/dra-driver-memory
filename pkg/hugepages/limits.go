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

package hugepages

import (
	"github.com/go-logr/logr"

	"github.com/ffromani/dra-driver-memory/pkg/sysinfo"
	"github.com/ffromani/dra-driver-memory/pkg/types"
	"github.com/ffromani/dra-driver-memory/pkg/unitconv"
)

// Limit is a Plain-Old-Data struct we carry around to do our computations;
// this way we can set `runtimeapi.HugepageLimit` once and avoid copies.
type Limit struct {
	// The value of PageSize has the format <size><unit-prefix>B (2MB, 1GB),
	// and must match the <hugepagesize> of the corresponding control file found in `hugetlb.<hugepagesize>.limit_in_bytes`.
	// The values of <unit-prefix> are intended to be parsed using base 1024("1KB" = 1024, "1MB" = 1048576, etc).
	PageSize string
	// limit in bytes of hugepagesize HugeTLB usage.
	Limit uint64
}

func LimitsFromAllocations(lh logr.Logger, machineData sysinfo.MachineData, allocs []types.Allocation) []Limit {
	var hpLimits []Limit

	for _, hpSize := range machineData.Hugepagesizes {
		hpLimits = append(hpLimits, Limit{
			PageSize: unitconv.SizeInBytesToCGroupString(hpSize),
			Limit:    uint64(0),
		})
	}
	lh.V(0).Info("default hugepage limits", "limits", hpLimits)

	allocationLimits := map[string]uint64{}
	for _, alloc := range allocs {
		pageSize := unitconv.SizeInBytesToCGroupString(alloc.Pagesize)
		allocationLimits[pageSize] = uint64(alloc.Amount) * alloc.Pagesize
	}
	lh.V(0).Info("allocation hugepage limits", "limits", allocationLimits)

	for idx := range hpLimits {
		limit, exists := allocationLimits[hpLimits[idx].PageSize]
		if !exists {
			continue
		}
		hpLimits[idx].Limit = limit
	}
	lh.V(0).Info("computed hugepage limits", "limits", hpLimits)

	return hpLimits
}
