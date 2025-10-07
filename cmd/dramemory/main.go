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
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"golang.org/x/sys/unix"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	nodeutil "k8s.io/component-helpers/node/util"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/textlogger"

	"github.com/ffromani/dra-driver-memory/internal/kloglevel"
	"github.com/ffromani/dra-driver-memory/pkg/driver"
)

const (
	driverName = "dra.memory"
)

func main() {
	var params Params
	var ready atomic.Bool
	setupLogger := stdr.New(log.New(os.Stderr, "", log.Lshortfile))

	params.InitFlags()
	params.ParseFlags()
	params.DumpFlags(setupLogger)

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

	go func() {
		_ = server.ListenAndServe()
	}()

	var err error
	var config *rest.Config
	if params.Kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", params.Kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		setupLogger.Error(err, "can not create client-go configuration")
		os.Exit(1)
	}

	// use protobuf for better performance at scale
	// https://kubernetes.io/docs/reference/using-api/api-concepts/#alternate-representations-of-resources
	config.AcceptContentTypes = "application/vnd.kubernetes.protobuf,application/json"
	config.ContentType = "application/vnd.kubernetes.protobuf"

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		setupLogger.Error(err, "can not create client-go client")
		os.Exit(2)
	}

	nodeName, err := nodeutil.GetHostname(params.HostnameOverride)
	if err != nil {
		setupLogger.Error(err, "cannot obtain the node name, use the hostname-override flag if you want to set it to a specific value")
		os.Exit(2)
	}

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	signalCh := make(chan os.Signal, 2)
	defer func() {
		close(signalCh)
		cancel()
	}()
	signal.Notify(signalCh, os.Interrupt, unix.SIGINT)

	driverConfig := &driver.Config{
		DriverName: driverName,
		NodeName:   nodeName,
	}
	dramem, err := driver.Start(ctx, clientset, makeLogger(setupLogger), driverConfig)
	if err != nil {
		setupLogger.Error(err, "driver failed to start")
	}
	defer dramem.Stop()
	ready.Store(true)
	setupLogger.Info("driver started")

	select {
	case <-signalCh:
		setupLogger.Info("Exiting: received signal")
		cancel()
	case <-ctx.Done():
		setupLogger.Info("Exiting: context cancelled")
	}
}

func printVersion(lh logr.Logger) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	var vcsRevision string
	for _, f := range info.Settings {
		switch f.Key {
		case "vcs.revision":
			vcsRevision = f.Value
		}
	}
	lh.Info("dramemory", "golang", info.GoVersion, "build", vcsRevision)
}

type Params struct {
	HostnameOverride string
	Kubeconfig       string
	BindAddress      string
}

func (par *Params) InitFlags() {
	klog.InitFlags(nil)
	flag.StringVar(&par.Kubeconfig, "kubeconfig", par.Kubeconfig, "absolute path to the kubeconfig file")
	flag.StringVar(&par.HostnameOverride, "hostname-override", par.HostnameOverride, "If non-empty, will be used as the name of the Node that kube-network-policies is running on. If unset, the node name is assumed to be the same as the node's hostname.")
}

func (par *Params) ParseFlags() {
	flag.Parse()
}

func (par *Params) DumpFlags(lh logr.Logger) {
	printVersion(lh)
	flag.VisitAll(func(f *flag.Flag) {
		lh.Info("FLAG", f.Name, f.Value)
	})

}

func makeLogger(setupLogger logr.Logger) logr.Logger {
	lev, err := kloglevel.Get()
	if err != nil {
		setupLogger.Error(err, "cannot get verbosity, going dark")
		return logr.Discard() // TODO: fail
	}
	config := textlogger.NewConfig(textlogger.Verbosity(int(lev)))
	return textlogger.NewLogger(config)
}
