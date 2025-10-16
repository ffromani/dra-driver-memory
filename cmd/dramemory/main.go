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
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/unix"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	ghwopt "github.com/jaypipes/ghw/pkg/option"
	ghwtopology "github.com/jaypipes/ghw/pkg/topology"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	nodeutil "k8s.io/component-helpers/node/util"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/textlogger"

	"sigs.k8s.io/yaml"

	"github.com/ffromani/dra-driver-memory/internal/kloglevel"
	"github.com/ffromani/dra-driver-memory/pkg/driver"
	"github.com/ffromani/dra-driver-memory/pkg/hugepages/provision"
)

const (
	driverName = "dra.memory"
)

type SysinformerFunc func() (*ghwtopology.Info, error)

func (f SysinformerFunc) Topology() (*ghwtopology.Info, error) {
	return f()
}

func main() {
	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt, unix.SIGTERM)
	// ignore stop() as we gonna os.Exit() anyway. Intentional minor leak.

	setupLogger := stdr.New(log.New(os.Stderr, "", log.Lshortfile))

	params := DefaultParams()
	params.InitFlags()
	params.ParseFlags()
	params.DumpFlags(setupLogger)

	if params.InspectOnly {
		if err := runInspect(params, setupLogger); err != nil {
			setupLogger.Error(err, "inspection failed")
			os.Exit(1)
		}
		os.Exit(0)
	}

	if params.HugePages.RuntimeProvisionConfig != "" {
		logger, err := makeLogger(setupLogger)
		if err != nil {
			setupLogger.Error(err, "creating logger")
			os.Exit(1)
		}
		if err := runHugePagesProvision(params, logger); err != nil {
			setupLogger.Error(err, "hugepages provisioning failed")
			os.Exit(1)
		}
		os.Exit(0)
	}

	if err := runDaemon(ctx, params, setupLogger); err != nil {
		setupLogger.Error(err, "daemon failed")
		os.Exit(1)
	}
}

func runInspect(params Params, setupLogger logr.Logger) error {
	sysinfo, err := ghwtopology.New(ghwopt.WithChroot(params.SysRoot))
	if err != nil {
		return err
	}
	dumpMemoryInfo(sysinfo, setupLogger)
	return nil
}

func runDaemon(ctx context.Context, params Params, setupLogger logr.Logger) error {
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
		setupLogger.Info("starting metrics and healthz server", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server failed: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		<-egCtx.Done() // Wait for cancellation from errgroup context
		setupLogger.Info("shutting down metrics and healthz server")
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

	drvLogger, err := makeLogger(setupLogger)
	if err != nil {
		return err
	}

	driverEnv := driver.Environment{
		DriverName: driverName,
		NodeName:   nodeName,
		Clientset:  clientset,
		Logger:     drvLogger,
		Sysinform: SysinformerFunc(func() (*ghwtopology.Info, error) {
			return ghwtopology.New(ghwopt.WithChroot(params.SysRoot))
		}),
	}
	dramem, err := driver.Start(egCtx, driverEnv)
	if err != nil {
		return fmt.Errorf("driver failed to start: %w", err)
	}
	defer setupLogger.Info("driver stopped") // ensure correct ordering of logs
	defer dramem.Stop()

	ready.Store(true)
	setupLogger.Info("driver started")

	return eg.Wait()
}

func printVersion(lh logr.Logger) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	var vcsRevision string
	for _, f := range info.Settings {
		if f.Key == "vcs.revision" {
			vcsRevision = f.Value
		}
	}
	lh.Info("dramemory", "golang", info.GoVersion, "build", vcsRevision)
}

type HugePagesParams struct {
	RuntimeProvisionConfig string
}

type Params struct {
	HostnameOverride string
	Kubeconfig       string
	BindAddress      string
	SysRoot          string
	InspectOnly      bool
	HugePages        HugePagesParams
}

func DefaultParams() Params {
	return Params{
		SysRoot: "/",
	}
}

func (par *Params) InitFlags() {
	klog.InitFlags(nil)
	flag.StringVar(&par.Kubeconfig, "kubeconfig", par.Kubeconfig, "Absolute path to the kubeconfig file.")
	flag.StringVar(&par.HostnameOverride, "hostname-override", par.HostnameOverride, "If non-empty, will be used as the name of the Node that kube-network-policies is running on. If unset, the node name is assumed to be the same as the node's hostname.")
	flag.StringVar(&par.SysRoot, "sysfs-root", par.SysRoot, "root point where sysfs is mounted.")
	flag.BoolVar(&par.InspectOnly, "inspect", par.InspectOnly, "inspect machine properties and exit.")
	flag.StringVar(&par.HugePages.RuntimeProvisionConfig, "hugepages-provision", par.HugePages.RuntimeProvisionConfig, "provision hugepages at runtime (now) using the config at path (`-` for stdin).")
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

func makeLogger(setupLogger logr.Logger) (logr.Logger, error) {
	lev, err := kloglevel.Get()
	if err != nil {
		return logr.Discard(), fmt.Errorf("cannot get verbosity, going dark: %w", err)
	}
	config := textlogger.NewConfig(textlogger.Verbosity(int(lev)))
	return textlogger.NewLogger(config), nil
}

func dumpMemoryInfo(sysinfo *ghwtopology.Info, logger logr.Logger) {
	for _, node := range sysinfo.Nodes {
		data, err := yaml.Marshal(node.Memory)
		if err != nil {
			logger.Error(err, "marshaling data for node %d", node.ID)
		}
		// re-indent, bruteforce way
		var lines []string
		sc := bufio.NewScanner(strings.NewReader(string(data)))
		for sc.Scan() {
			lines = append(lines, sc.Text())
		}
		fmt.Printf("* node=%d:\n  %s\n", node.ID, strings.Join(lines, "\n  "))
	}
}

func runHugePagesProvision(params Params, setupLogger logr.Logger) error {
	config, err := provision.ReadConfiguration(params.HugePages.RuntimeProvisionConfig)
	if err != nil {
		return err
	}
	err = provision.RuntimeHugepages(setupLogger, config, params.SysRoot)
	if err != nil {
		return err
	}
	return nil
}
