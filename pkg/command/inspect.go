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

package command

import (
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	ghwmemory "github.com/jaypipes/ghw/pkg/memory"

	"sigs.k8s.io/yaml"

	"github.com/ffromani/dra-driver-memory/pkg/sysinfo"
	"github.com/ffromani/dra-driver-memory/pkg/unitconv"
)

type InspectMode int

const (
	InspectNone InspectMode = iota
	InspectRaw
	InspectSummary
)

type InspectValue struct {
	Mode *InspectMode
}

func (v InspectValue) String() string {
	if v.Mode == nil {
		return ""
	}
	switch *v.Mode {
	case InspectRaw:
		return "raw"
	case InspectSummary:
		return "summary"
	default:
		return "none"
	}
}

func (v InspectValue) Set(s string) error {
	s = strings.ToLower(s)
	switch s {
	case "raw":
		*v.Mode = InspectRaw
	case "summary":
		*v.Mode = InspectSummary
	case "none":
		*v.Mode = InspectNone
	default:
		return fmt.Errorf("unsupported mode: %q", s)
	}
	return nil
}

func Inspect(params Params, logger logr.Logger) error {
	machine, err := sysinfo.GetMachineData(logger, params.SysRoot)
	if err != nil {
		return err
	}
	if params.InspectMode == InspectSummary {
		logYAML(logger, convertMachineData(machine))
		return nil
	}
	logYAML(logger, machine)
	return nil
}

func logYAML(logger logr.Logger, obj any) {
	data, err := yaml.Marshal(obj)
	if err != nil {
		logger.Error(err, "marshaling data")
	}
	fmt.Print(string(data))
}

type machineData struct {
	Pagesize      string        `json:"page_size"`
	Hugepagesizes []string      `json:"huge_page_sizes"`
	Zones         []machineZone `json:"zones"`
}

type machineZone struct {
	ID        int           `json:"id"`
	Distances []int         `json:"distances"`
	Memory    machineMemory `json:"memory"`
}

type machineMemory struct {
	TotalPhysicalSize     string                     `json:"total_physical_size"`
	TotalUsableSize       string                     `json:"total_usable_size"`
	SupportedPageSizes    []string                   `json:"supported_page_sizes"`
	DefaultHugePageSize   string                     `json:"default_huge_page_size"`
	TotalHugePageSize     string                     `json:"total_huge_page_size"`
	HugePageAmountsBySize map[string]HugePageAmounts `json:"huge_page_amounts_by_size"`
}

type HugePageAmounts = ghwmemory.HugePageAmounts

func convertMachineData(md sysinfo.MachineData) machineData {
	ret := machineData{
		Pagesize:      unitconv.SizeInBytesToMinimizedString(md.Pagesize),
		Hugepagesizes: make([]string, 0, len(md.Hugepagesizes)),
		Zones:         make([]machineZone, 0, len(md.Zones)),
	}
	for _, hpSize := range md.Hugepagesizes {
		ret.Hugepagesizes = append(ret.Hugepagesizes, unitconv.SizeInBytesToMinimizedString(hpSize))
	}
	for _, zone := range md.Zones {
		zn := machineZone{
			ID:        zone.ID,
			Distances: append([]int{}, zone.Distances...),
		}
		if zone.Memory != nil {
			mem := machineMemory{
				TotalPhysicalSize:     unitconv.SizeInBytesToMinimizedString(uint64(zone.Memory.TotalPhysicalBytes)),
				TotalUsableSize:       unitconv.SizeInBytesToMinimizedString(uint64(zone.Memory.TotalUsableBytes)),
				SupportedPageSizes:    make([]string, 0, len(zone.Memory.SupportedPageSizes)),
				DefaultHugePageSize:   unitconv.SizeInBytesToMinimizedString(zone.Memory.DefaultHugePageSize),
				TotalHugePageSize:     unitconv.SizeInBytesToMinimizedString(uint64(zone.Memory.TotalHugePageBytes)),
				HugePageAmountsBySize: make(map[string]HugePageAmounts, len(zone.Memory.HugePageAmountsBySize)),
			}
			for _, supSize := range zone.Memory.SupportedPageSizes {
				mem.SupportedPageSizes = append(mem.SupportedPageSizes, unitconv.SizeInBytesToMinimizedString(supSize))
			}
			for hpSize, hpAmounts := range zone.Memory.HugePageAmountsBySize {
				mem.HugePageAmountsBySize[unitconv.SizeInBytesToMinimizedString(hpSize)] = *hpAmounts
			}
			zn.Memory = mem
		}
		ret.Zones = append(ret.Zones, zn)
	}
	return ret
}
