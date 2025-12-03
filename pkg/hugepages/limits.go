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
	"errors"
	"io/fs"

	"github.com/go-logr/logr"

	"github.com/ffromani/dra-driver-memory/pkg/cgroups"
	"github.com/ffromani/dra-driver-memory/pkg/sysinfo"
	"github.com/ffromani/dra-driver-memory/pkg/types"
	"github.com/ffromani/dra-driver-memory/pkg/unitconv"
)

type LimitValue struct {
	Value uint64 `json:"value"`
	Unset bool   `json:"unset"`
}

// Limit is a Plain-Old-Data struct we carry around to do our computations;
// this way we can set `runtimeapi.HugepageLimit` once and avoid copies.
type Limit struct {
	// The value of PageSize has the format <size><unit-prefix>B (2MB, 1GB),
	// and must match the <hugepagesize> of the corresponding control file found in `hugetlb.<hugepagesize>.limit_in_bytes`.
	// The values of <unit-prefix> are intended to be parsed using base 1024("1KB" = 1024, "1MB" = 1048576, etc).
	PageSize string `json:"pageSize"`
	// limit in bytes of hugepagesize HugeTLB usage.
	Limit LimitValue `json:"limit"`
}

func (lim Limit) String() string {
	if lim.Limit.Unset {
		return lim.PageSize + "=max"
	}
	return lim.PageSize + "=" + unitconv.SizeInBytesToCGroupString(lim.Limit.Value)
}

func LimitsFromAllocations(lh logr.Logger, machineData sysinfo.MachineData, allocs []types.Allocation) []Limit {
	var hpLimits []Limit

	for _, hpSize := range machineData.Hugepagesizes {
		hpLimits = append(hpLimits, Limit{
			PageSize: unitconv.SizeInBytesToCGroupString(hpSize),
			Limit: LimitValue{
				Value: uint64(0),
			},
		})
	}
	lh.V(2).Info("default hugepage limits", "limits", hpLimits)

	allocationLimits := map[string]uint64{}
	for _, alloc := range allocs {
		pageSize := unitconv.SizeInBytesToCGroupString(alloc.Pagesize)
		allocationLimits[pageSize] = uint64(alloc.Amount)
	}
	lh.V(2).Info("allocation hugepage limits", "limits", allocationLimits)

	for idx := range hpLimits {
		limit, exists := allocationLimits[hpLimits[idx].PageSize]
		if !exists {
			continue
		}
		hpLimits[idx].Limit.Value = limit
	}
	lh.V(2).Info("computed hugepage limits", "limits", hpLimits)

	return hpLimits
}

func LimitsFromSystemPID(lh logr.Logger, machineData sysinfo.MachineData, procRoot string, pid int) ([]Limit, error) {
	cgPath, err := cgroups.FullPathByPID(procRoot, pid)
	if err != nil {
		return nil, err
	}
	lh.V(4).Info("reading from cgroup", "path", cgPath)
	return LimitsFromSystemPath(lh, machineData, cgPath)
}

func LimitsFromSystemPath(lh logr.Logger, machineData sysinfo.MachineData, cgPath string) ([]Limit, error) {
	lh.V(2).Info("getting system limits", "hugepageSizes", machineData.Hugepagesizes)
	var limits []Limit
	for _, hpSize := range machineData.Hugepagesizes {
		pageSize := unitconv.SizeInBytesToCGroupString(hpSize)
		// all the kernel interfaces use a different naming :\
		fileName := "hugetlb." + pageSize + ".max"
		val, err := cgroups.ParseValue(lh, cgPath, fileName)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				val = 0
			} else {
				lh.V(2).Error(err, "parsing limit", "path", cgPath, "file", fileName)
				continue
			}
		}
		lh.V(2).Info("reading limit", "file", fileName, "value", val)
		lim := Limit{
			PageSize: pageSize,
		}
		if val == -1 { // max
			lim.Limit.Unset = true
		} else {
			lim.Limit.Value = uint64(val)
		}
		limits = append(limits, lim)
	}
	return limits, nil
}

func SetSystemLimits(lh logr.Logger, cgPath string, limits []Limit) error {
	for _, limit := range limits {
		// all the kernel interfaces use a different naming :\
		fileName := "hugetlb." + limit.PageSize + ".max"
		value := convertLimitValue(limit.Limit)
		lh.V(2).Info("setting limit", "cgPath", cgPath, "file", fileName, "value", value)
		err := cgroups.WriteValue(lh, cgPath, fileName, value)
		if err != nil {
			return err
		}
	}
	return nil
}

func convertLimitValue(lv LimitValue) int64 {
	if lv.Unset {
		return -1
	}
	return int64(lv.Value)
}
