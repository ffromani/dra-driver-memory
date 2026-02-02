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
	"fmt"
	"log"
	"os"

	"github.com/go-logr/stdr"

	"github.com/ffromani/dra-driver-memory/pkg/setup/containerd"
)

func main() {
	var emitScript bool
	setupLogger := stdr.New(log.New(os.Stderr, "", log.Lshortfile))
	flag.BoolVar(&emitScript, "script", emitScript, "emit setup script entrypoint and exit.")
	flag.Parse()

	if emitScript {
		fmt.Printf("%s", containerd.SetupScript())
		os.Exit(0)
	}
	if flag.NArg() != 1 {
		setupLogger.Error(nil, "error: you need to supply /path/to/conf.toml. Use `-` to read from stdin and write to stdout")
		flag.Usage()
		os.Exit(1)
	}

	err := containerd.Config(flag.Arg(0))
	if err != nil {
		setupLogger.Error(err, "error processing %q: %v\n", flag.Arg(0))
		os.Exit(127)
	}
}
