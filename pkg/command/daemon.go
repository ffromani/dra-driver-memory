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
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	nodeutil "k8s.io/component-helpers/node/util"
	"k8s.io/klog/v2/textlogger"

	"github.com/ffromani/dra-driver-memory/pkg/driver"
	"github.com/ffromani/dra-driver-memory/pkg/kloglevel"
	"github.com/ffromani/dra-driver-memory/pkg/sysinfo"
)

type SysinfoVerifierFunc func() error

func (f SysinfoVerifierFunc) Validate() error {
	return f()
}

type SysinfoDiscovererFunc func() (sysinfo.MachineData, error)

func (f SysinfoDiscovererFunc) Discover() (sysinfo.MachineData, error) {
	return f()
}

func RunDaemon(ctx context.Context, params Params, drvLogger logr.Logger) error {
	var ready atomic.Bool

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if !ready.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})
	mux.Handle("/metrics", promhttp.Handler())
	server := &http.Server{
		Addr:              params.BindAddress,
		Handler:           mux,
		IdleTimeout:       120 * time.Second,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
	}

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		drvLogger.Info("starting metrics and healthz server", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server failed: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		<-egCtx.Done() // Wait for cancellation from errgroup context
		drvLogger.Info("shutting down metrics and healthz server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	})

	var err error
	var config *rest.Config
	if params.Kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", params.Kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		return fmt.Errorf("cannot create client-go configuration: %w", err)
	}

	// use protobuf for better performance at scale
	// https://kubernetes.io/docs/reference/using-api/api-concepts/#alternate-representations-of-resources
	config.AcceptContentTypes = "application/vnd.kubernetes.protobuf,application/json"
	config.ContentType = "application/vnd.kubernetes.protobuf"

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("cannot create client-go client: %w", err)
	}

	nodeName, err := nodeutil.GetHostname(params.HostnameOverride)
	if err != nil {
		return fmt.Errorf("cannot obtain the node name, use the hostname-override flag if you want to set it to a specific value: %w", err)
	}

	driverEnv := driver.Environment{
		DriverName:  driver.Name,
		NodeName:    nodeName,
		Clientset:   clientset,
		Logger:      drvLogger,
		SysRoot:     params.SysRoot,
		CgroupMount: params.CgroupMount,
		SysVerifier: SysinfoVerifierFunc(func() error {
			return sysinfo.Validate(drvLogger, params.ProcRoot)
		}),
	}
	dramem, err := driver.Start(egCtx, driverEnv)
	if err != nil {
		return fmt.Errorf("driver failed to start: %w", err)
	}
	defer drvLogger.Info("driver stopped") // ensure correct ordering of logs
	defer dramem.Stop()

	ready.Store(true)
	drvLogger.Info("driver started")

	return eg.Wait()
}

func MakeLogger(setupLogger logr.Logger) (logr.Logger, error) {
	lev, err := kloglevel.Get()
	if err != nil {
		return logr.Discard(), fmt.Errorf("cannot get verbosity, going dark: %w", err)
	}
	config := textlogger.NewConfig(textlogger.Verbosity(int(lev)))
	return textlogger.NewLogger(config), nil
}
