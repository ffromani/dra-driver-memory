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
	"os"

	"github.com/go-logr/logr"
	ghwmemory "github.com/jaypipes/ghw/pkg/memory"
	ghwopt "github.com/jaypipes/ghw/pkg/option"
	ghwtopology "github.com/jaypipes/ghw/pkg/topology"

	"github.com/ffromani/dra-driver-memory/pkg/unitconv"
)

type Zone struct {
	ID        int             `json:"id"`
	Distances []int           `json:"distances"`
	Memory    *ghwmemory.Area `json:"memory"`
}

func FromNodes(nodes []*ghwtopology.Node) []Zone {
	zones := make([]Zone, 0, len(nodes))
	for _, node := range nodes {
		zones = append(zones, Zone{
			ID:        node.ID,
			Distances: node.Distances,
			Memory:    node.Memory,
		})
	}
	return zones
}

type MachineData struct {
	Pagesize      uint64   `json:"page_size"`
	Hugepagesizes []uint64 `json:"huge_page_sizes"`
	Zones         []Zone   `json:"zones"`
}

func GetMachineData(lh logr.Logger, sysRoot string) (MachineData, error) {
	topo, err := ghwtopology.New(ghwopt.WithChroot(sysRoot))
	if err != nil {
		return MachineData{}, err
	}
	var Hugepagesizes []uint64
	for _, pageSize := range HugepageSizes(lh, sysRoot) {
		sz, err := unitconv.CGroupStringToSizeInBytes(pageSize)
		if err != nil {
			lh.Error(err, "getting system huge page size")
			continue
		}
		Hugepagesizes = append(Hugepagesizes, sz)
	}
	return MachineData{
		Pagesize:      uint64(os.Getpagesize()),
		Hugepagesizes: Hugepagesizes,
		Zones:         FromNodes(topo.Nodes),
	}, nil
}
