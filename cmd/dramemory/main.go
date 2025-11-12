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

	logger, err := command.MakeLogger(setupLogger)
	if err != nil {
		setupLogger.Error(err, "creating the main logger")
		os.Exit(1)
	}

	if params.DoInspection {
		if err := command.Inspect(params, logger); err != nil {
			logger.Error(err, "inspection failed")
			os.Exit(1)
		}
		os.Exit(0)
	}

	if params.DoValidation {
		if err := command.Validate(params, logger); err != nil {
			logger.Error(err, "validation failed")
			os.Exit(1)
		}
		os.Exit(0)
	}

	if params.DoManifests {
		if err := command.MakeManifests(params, logger); err != nil {
			logger.Error(err, "manifests creation failed")
			os.Exit(1)
		}
		os.Exit(0)
	}

	if params.HugePages.RuntimeProvisionConfig != "" {
		if err := command.ProvisionHugepages(params, logger); err != nil {
			logger.Error(err, "hugepages provisioning failed")
			os.Exit(1)
		}
		os.Exit(0)
	}

	params.DumpFlags(logger)
	if err := command.RunDaemon(ctx, params, logger); err != nil {
		logger.Error(err, "daemon failed")
		os.Exit(1)
	}
}
