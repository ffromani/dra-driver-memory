/*
Copyright 2025 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"golang.org/x/sys/unix"

	"github.com/go-logr/stdr"

	"github.com/ffromani/dra-driver-memory/internal/command"
)

func main() {
	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt, unix.SIGTERM)
	// ignore stop() as we gonna os.Exit() anyway. Intentional minor leak.

	setupLogger := stdr.New(log.New(os.Stderr, "", log.Lshortfile))

	params := command.DefaultParams()
	params.InitFlags()
	params.ParseFlags()
	params.DumpFlags(setupLogger)

	if params.DoInspection {
		if err := command.Inspect(params, setupLogger); err != nil {
			setupLogger.Error(err, "inspection failed")
			os.Exit(1)
		}
		os.Exit(0)
	}

	if params.DoValidation {
		if err := command.Validate(params, setupLogger); err != nil {
			setupLogger.Error(err, "validation failed")
			os.Exit(1)
		}
		os.Exit(0)
	}

	if params.HugePages.RuntimeProvisionConfig != "" {
		logger, err := command.MakeLogger(setupLogger)
		if err != nil {
			setupLogger.Error(err, "creating logger")
			os.Exit(1)
		}
		if err := command.ProvisionHugepages(params, logger); err != nil {
			setupLogger.Error(err, "hugepages provisioning failed")
			os.Exit(1)
		}
		os.Exit(0)
	}

	if err := command.RunDaemon(ctx, params, setupLogger); err != nil {
		setupLogger.Error(err, "daemon failed")
		os.Exit(1)
	}
}
