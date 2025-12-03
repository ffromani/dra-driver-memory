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
	"flag"
	"runtime/debug"

	"github.com/go-logr/logr"

	"k8s.io/klog/v2"
)

const (
	ProgramName = "dramemory"
)

type HugePagesParams struct {
	RuntimeProvisionConfig string
}

type Params struct {
	HostnameOverride string
	Kubeconfig       string
	BindAddress      string
	ProcRoot         string
	SysRoot          string
	CgroupMount      string
	DoValidation     bool
	DoManifests      bool
	DoVersion        bool
	InspectMode      InspectMode
	HugePages        HugePagesParams
}

func DefaultParams() Params {
	return Params{
		ProcRoot: "/",
		SysRoot:  "/",
	}
}

func (par *Params) InitFlags() {
	klog.InitFlags(nil)
	flag.StringVar(&par.Kubeconfig, "kubeconfig", par.Kubeconfig, "Absolute path to the kubeconfig file.")
	flag.StringVar(&par.HostnameOverride, "hostname-override", par.HostnameOverride, "If non-empty, will be used as the name of the Node that kube-network-policies is running on. If unset, the node name is assumed to be the same as the node's hostname.")
	flag.StringVar(&par.ProcRoot, "procfs-root", par.ProcRoot, "root point where procfs is mounted.")
	flag.StringVar(&par.SysRoot, "sysfs-root", par.SysRoot, "root point where sysfs is mounted.")
	flag.StringVar(&par.CgroupMount, "cgroup-mount", par.CgroupMount, "cgroupfs mount point. Set empty to DISABLE direct cgroup settings.")
	flag.BoolVar(&par.DoValidation, "validate", par.DoValidation, "validate machine properties and exit.")
	flag.BoolVar(&par.DoManifests, "make-manifests", par.DoManifests, "emit DRA manifests based on hardware discovery.")
	flag.BoolVar(&par.DoVersion, "version", par.DoVersion, "print program version and exit.")
	flag.StringVar(&par.HugePages.RuntimeProvisionConfig, "hugepages-provision", par.HugePages.RuntimeProvisionConfig, "provision hugepages at runtime (now) using the config at path (`-` for stdin).")
	flag.Var(&InspectValue{Mode: &par.InspectMode}, "inspect", "inspect machine properties and exit.")
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

type Version struct {
	Golang string
	Build  string
}

func GetVersion() (Version, bool) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return Version{}, false
	}
	ver := Version{
		Golang: info.GoVersion,
	}
	for _, f := range info.Settings {
		if f.Key == "vcs.revision" {
			ver.Build = f.Value
			return ver, true
		}
	}
	return ver, false
}

func printVersion(lh logr.Logger) {
	ver, ok := GetVersion()
	if !ok {
		return
	}
	lh.Info(ProgramName, "golang", ver.Golang, "build", ver.Build)
}
