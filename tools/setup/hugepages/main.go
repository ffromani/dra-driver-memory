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

package main

import (
	"flag"
	"log"
	"os"

	"github.com/go-logr/stdr"
	ghwopt "github.com/jaypipes/ghw/pkg/option"
	ghwtopology "github.com/jaypipes/ghw/pkg/topology"

	"github.com/ffromani/dra-driver-memory/pkg/hugepages/provision"
)

func main() {
	var sysRoot string = "/"
	setupLogger := stdr.New(log.New(os.Stderr, "", log.Lshortfile))
	flag.StringVar(&sysRoot, "sysfs-root", sysRoot, "root point where sysfs is mounted.")
	flag.Parse()

	sysinfo, err := ghwtopology.New(ghwopt.WithChroot(sysRoot))
	if err != nil {
		setupLogger.Error(err, "cannot discover machine topology")
		os.Exit(1)
	}
	for _, arg := range flag.Args() {
		config, err := provision.ReadConfiguration(arg)
		if err != nil {
			setupLogger.Error(err, "cannot read hugepages configuration", "path", arg)
			os.Exit(2)
		}
		err = provision.RuntimeHugepages(setupLogger, config, sysRoot, len(sysinfo.Nodes))
		if err != nil {
			setupLogger.Error(err, "cannot provision hugepages")
			os.Exit(4)
		}
	}
}
