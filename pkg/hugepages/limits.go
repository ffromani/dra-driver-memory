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
	"strings"

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

func (lv LimitValue) Clone() LimitValue {
	return LimitValue{
		Value: lv.Value,
		Unset: lv.Unset,
	}
}

func (lv LimitValue) Add(x LimitValue) LimitValue {
	if lv.Unset && x.Unset {
		return LimitValue{
			Value: 0,
			Unset: true,
		}
	}
	if lv.Unset && !x.Unset {
		return LimitValue{
			Value: x.Value,
			Unset: false,
		}
	}
	if !lv.Unset && x.Unset {
		return LimitValue{
			Value: lv.Value,
			Unset: false,
		}
	}
	return LimitValue{
		Value: lv.Value + x.Value,
		Unset: false,
	}
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

func (lim Limit) Clone() Limit {
	return Limit{
		PageSize: lim.PageSize,
		Limit:    lim.Limit.Clone(),
	}
}

func (lim Limit) String() string {
	if lim.Limit.Unset {
		return lim.PageSize + "=max"
	}
	return lim.PageSize + "=" + unitconv.SizeInBytesToCGroupString(lim.Limit.Value)
}

// SumLimits add limits "llb" to the existing "lla".
// Note we expect to have <= 4 limits, so the simplest nested for should be perfectly fine.
func SumLimits(lla, llb []Limit) []Limit {
	var ret []Limit
	for idxa := range lla {
		found := false
		for idxb := range llb {
			if lla[idxa].PageSize == llb[idxb].PageSize {
				found = true
				ret = append(ret, Limit{
					PageSize: lla[idxa].PageSize,
					Limit:    lla[idxa].Limit.Add(llb[idxb].Limit),
				})
				break
			}
		}
		if !found {
			ret = append(ret, lla[idxa].Clone())
		}
	}
	for idxb := range llb {
		found := false
		for idxa := range lla {
			if llb[idxb].PageSize == lla[idxa].PageSize {
				found = true
				break
			}
		}
		if !found {
			ret = append(ret, llb[idxb].Clone())
		}
	}
	return ret
}

func LimitsToString(lls []Limit) string {
	if len(lls) == 0 {
		return ""
	}
	sep := ", "
	var sb strings.Builder
	for _, lim := range lls {
		sb.WriteString(sep + lim.String())
	}
	return strings.TrimPrefix(sb.String(), sep)
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
				val = -1
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
	/* doortrap: HugeTLB Cgroup v2 Limits
	 * When setting hugepage limits in Cgroup v2, we MUST set two distinct values.
	 * Failing to set the reservation limit is will cause amibguous ENOMEM failures.
	 *
	 * 1. Usage Limit (hugetlb.<size>.max):
	 * - Controls: The actual physical RAM currently consumed (faulted in).
	 * - Enforced: When the app writes to memory.
	 *
	 * 2. Reservation Limit (hugetlb.<size>.rsvd.max):
	 * - Controls: The "promise" of pages that the kernel guarantees will be available.
	 * - Enforced: When the app calls mmap(MAP_HUGETLB).
	 *
	 * When an application calls mmap(), it hasn't used memory yet (Usage=0), but it demands
	 * a guarantee (Reservation). If 'rsvd.max' is 0 (default) but 'max' is > 0, the kernel
	 * allows 0 bytes of reservation. The mmap() call fails immediately with ENOMEM, despite
	 * the visible usage limit looking correct.
	 * So: always sync 'rsvd.max' to at least the value of 'max'.
	 */
	attrs := []string{".rsvd.max", ".max"}
	for _, limit := range limits {
		value := convertLimitValue(limit.Limit)
		for _, attr := range attrs {
			fileName := "hugetlb." + limit.PageSize + attr
			lh.V(2).Info("setting limit", "cgPath", cgPath, "file", fileName, "value", value)
			err := cgroups.WriteValue(lh, cgPath, fileName, value)
			if err != nil {
				return err
			}
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
