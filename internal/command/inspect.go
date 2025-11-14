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
	"os"

	"github.com/go-logr/logr"
	ghwopt "github.com/jaypipes/ghw/pkg/option"
	ghwtopology "github.com/jaypipes/ghw/pkg/topology"
	libcontainercgroups "github.com/opencontainers/cgroups"

	"sigs.k8s.io/yaml"

	"github.com/ffromani/dra-driver-memory/pkg/sysinfo"
	"github.com/ffromani/dra-driver-memory/pkg/unitconv"
)

func Inspect(params Params, logger logr.Logger) error {
	machine, err := GetMachineData(params, logger)
	if err != nil {
		return err
	}
	printYAML(machine, logger)
	return nil
}

func GetMachineData(params Params, logger logr.Logger) (sysinfo.MachineData, error) {
	topo, err := ghwtopology.New(ghwopt.WithChroot(params.SysRoot))
	if err != nil {
		return sysinfo.MachineData{}, err
	}
	var Hugepagesizes []uint64
	for _, pageSize := range libcontainercgroups.HugePageSizes() {
		sz, err := unitconv.CGroupStringToSizeInBytes(pageSize)
		if err != nil {
			logger.Error(err, "getting system huge page size")
			continue
		}
		Hugepagesizes = append(Hugepagesizes, sz)
	}
	return sysinfo.MachineData{
		Pagesize:      uint64(os.Getpagesize()),
		Hugepagesizes: Hugepagesizes,
		Zones:         sysinfo.FromNodes(topo.Nodes),
	}, nil
}

func printYAML(obj any, logger logr.Logger) {
	data, err := yaml.Marshal(obj)
	if err != nil {
		logger.Error(err, "marshaling data")
	}
	fmt.Print(string(data))
}
