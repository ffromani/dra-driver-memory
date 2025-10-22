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

	"sigs.k8s.io/yaml"

	"github.com/ffromani/dra-driver-memory/pkg/sysinfo"
)

func Inspect(params Params, setupLogger logr.Logger) error {
	machine, err := GetMachineData(params)
	if err != nil {
		return err
	}
	printYAML(machine, setupLogger)
	return nil
}

func GetMachineData(params Params) (sysinfo.MachineData, error) {
	topo, err := ghwtopology.New(ghwopt.WithChroot(params.SysRoot))
	if err != nil {
		return sysinfo.MachineData{}, err
	}
	return sysinfo.MachineData{
		Pagesize: os.Getpagesize(),
		Zones:    sysinfo.FromNodes(topo.Nodes),
	}, nil
}

func printYAML(obj any, logger logr.Logger) {
	data, err := yaml.Marshal(obj)
	if err != nil {
		logger.Error(err, "marshaling data")
	}
	fmt.Print(string(data))
}
