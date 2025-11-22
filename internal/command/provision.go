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
	"github.com/go-logr/logr"
	ghwopt "github.com/jaypipes/ghw/pkg/option"
	ghwtopology "github.com/jaypipes/ghw/pkg/topology"

	"github.com/ffromani/dra-driver-memory/pkg/hugepages/provision"
)

func ProvisionHugepages(params Params, setupLogger logr.Logger) error {
	sysinfo, err := ghwtopology.New(ghwopt.WithChroot(params.SysRoot))
	if err != nil {
		return err
	}
	config, err := provision.ReadConfiguration(params.HugePages.RuntimeProvisionConfig)
	if err != nil {
		return err
	}
	err = provision.RuntimeHugepages(setupLogger, config, params.SysRoot, len(sysinfo.Nodes))
	if err != nil {
		return err
	}
	return nil
}
