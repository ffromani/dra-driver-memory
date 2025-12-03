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
	"os/signal"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"golang.org/x/sys/unix"

	"k8s.io/utils/cpuset"

	"github.com/ffromani/dra-driver-memory/pkg/cgroups"
	"github.com/ffromani/dra-driver-memory/pkg/hugepages"
	"github.com/ffromani/dra-driver-memory/pkg/sysinfo"
	"github.com/ffromani/dra-driver-memory/pkg/unitconv"
	"github.com/ffromani/dra-driver-memory/test/pkg/memalign"
	"github.com/ffromani/dra-driver-memory/test/pkg/result"
)

func main() {
	var useHugeTLB bool = true
	var runForever bool
	var shouldFail bool
	var singleNUMA bool
	var anyNUMA bool
	var procRoot string = "/"
	var sysRoot string = "/"
	var numaNodes cpuset.CPUSet
	var allocSize uint64 = uint64(8 * (1 << 20)) // bytes

	flag.BoolVar(&runForever, "run-forever", runForever, "Run forever after the operation is completed.")
	flag.BoolVar(&useHugeTLB, "use-hugetlb", useHugeTLB, "Use HugeTLB for allocation.")
	flag.BoolVar(&shouldFail, "should-fail", shouldFail, "Expect failure, not success.")
	flag.StringVar(&procRoot, "proc-root", procRoot, "procfs root path.")
	flag.StringVar(&sysRoot, "sys-root", sysRoot, "sysfs root path.")
	flag.Var(&UnitValue{SizeInBytes: &allocSize}, "alloc-size", "Amount of memory to allocate.")
	flag.Var(&NUMAValue{Nodes: &numaNodes, Single: &singleNUMA, Any: &anyNUMA}, "numa-align", "NUMA alignment required.")
	flag.Parse()

	var lh logr.Logger = stdr.New(log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile))

	res := result.New(allocSize, useHugeTLB, numaNodes.String())

	var mgr *Manager
	if runForever {
		mgr = NewManagerWaiting(res)
	} else {
		mgr = NewManager(res)
	}

	disc := sysinfo.NewDiscoverer(sysRoot)

	prot := unix.PROT_READ | unix.PROT_WRITE
	flags := unix.MAP_ANONYMOUS | unix.MAP_PRIVATE
	if useHugeTLB {
		flags |= unix.MAP_HUGETLB
	}

	lh.Info("mmap", "size", unitconv.SizeInBytesToMinimizedString(allocSize), "prot", prot, "flags", flags)

	logCurrentLimits(lh.WithValues("trace", "pre"), disc, procRoot)
	data, err := unix.Mmap(-1, 0, int(allocSize), prot, flags)
	logCurrentLimits(lh.WithValues("trace", "pos"), disc, procRoot)

	if err != nil {
		if shouldFail && err == unix.ENOMEM { // TODO: is equality check the best option here?
			mgr.Complete(0, result.FailedAsExpected, "Allocation failed as expected with 'ENOMEM' (Out of memory)")
		}
		// Any other error is a different problem
		mgr.Complete(1, result.UnexpectedMMapError, "mmap error: %v", err)
	}

	checkAllocatedMemory(lh, data)

	memNodes, err := memalign.NUMANodesByPID(lh, memalign.PIDSelf, procRoot)
	if err != nil {
		mgr.Complete(2, result.CannotCheckAllocation, "cannot check allocation: %v", err)
	}

	if singleNUMA {
		if memNodes.Size() != 1 {
			mgr.Complete(4, result.NUMAOverflown, "NUMA nodes allocation don't come from a single NUMA node actual=%q", memNodes.String())
		}
		mgr.Complete(0, result.Succeeded, "completed")
	}
	if anyNUMA {
		mgr.Complete(0, result.Succeeded, "completed")
	}
	if !numaNodes.Equals(memNodes) {
		mgr.Complete(4, result.NUMAMismatch, "NUMA nodes allocation mismatch expected=%q actual=%q", numaNodes.String(), memNodes.String())
	}

	mgr.Complete(0, result.Succeeded, "completed")
}

type Manager struct {
	res      *result.Result
	signalCh chan os.Signal
}

func NewManager(res *result.Result) *Manager {
	return &Manager{
		res: res,
	}
}

func NewManagerWaiting(res *result.Result) *Manager {
	mgr := &Manager{
		res:      res,
		signalCh: make(chan os.Signal, 2),
	}
	signal.Notify(mgr.signalCh, os.Interrupt, unix.SIGINT)
	return mgr
}

func (pl *Manager) Complete(code int, reason result.Reason, fmt_ string, args ...any) {
	if pl.signalCh != nil {
		fmt_ = "waiting for a signal to quit; " + fmt_
	}
	pl.res.Finalize(code, reason, fmt_, args...)
	if pl.signalCh != nil {
		<-pl.signalCh
	}
	os.Exit(code)
}

func checkAllocatedMemory(lh logr.Logger, data []byte) {
	// write memory to ensure the kernel actually allocates to us
	// this can trigger an abort (SIGBUS/SIGSEGV) in the worst case,
	// and we can't recover
	ts := time.Now()
	for i := range data {
		data[i] = 42
	}
	elapsed := time.Since(ts)
	lh.Info("allocation memory check done", "elapsed", elapsed)
}

type NUMAValue struct {
	Nodes  *cpuset.CPUSet
	Single *bool
	Any    *bool
}

func (v NUMAValue) String() string {
	if v.Single != nil && *v.Single {
		return "single"
	}
	if v.Any != nil && *v.Any {
		return "any"
	}
	if v.Nodes != nil {
		return v.Nodes.String()
	}
	return ""
}

func (v NUMAValue) Set(s string) error {
	s = strings.ToLower(s)
	if s == "single" {
		*v.Single = true
		return nil
	}
	if s == "any" {
		*v.Any = true
		return nil
	}
	nodes, err := cpuset.Parse(s)
	if err != nil {
		return err
	}
	*v.Nodes = nodes
	return nil
}

type UnitValue struct {
	SizeInBytes *uint64
}

func (v UnitValue) String() string {
	if v.SizeInBytes == nil {
		return ""
	}
	return unitconv.SizeInBytesToMinimizedString(*v.SizeInBytes)
}

func (v UnitValue) Set(s string) error {
	val, err := unitconv.MinimizedStringToSizeInBytes(s)
	if err != nil {
		return err
	}
	*v.SizeInBytes = val
	return nil
}

// intentionally swallows error
func logCurrentLimits(lh logr.Logger, disc *sysinfo.Discoverer, procRoot string) {
	machineData, err := disc.GetFreshMachineData(lh)
	if err != nil {
		return
	}
	limits, err := hugepages.LimitsFromSystemPID(lh, machineData, procRoot, cgroups.PIDSelf)
	if err != nil {
		return
	}
	for _, limit := range limits {
		lh.Info("hugepage", "limit", limit.String())
	}
}
