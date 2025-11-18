/*
 * code borrowed from github.com/opencontainers/cgroups v0.0.6
 * with minimal changes (logrus -> logr)
 */

package sysinfo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"golang.org/x/sys/unix"
)

func HugepageSizes(lh logr.Logger, sysRoot string) []string {
	hpPath := filepath.Join(sysRoot, "sys", "kernel", "mm", "hugepages")
	lh.V(4).Info("system hugepages", "path", hpPath)

	dir, err := os.OpenFile(hpPath, unix.O_DIRECTORY|unix.O_RDONLY, 0)
	if err != nil {
		lh.V(2).Error(err, "opening sysfs hugepages")
		return nil
	}

	files, err := dir.Readdirnames(0)
	_ = dir.Close() // nonfatal, and can hardly fail
	if err != nil {
		lh.V(2).Error(err, "reading sysfs hugepages")
		return nil
	}

	hugepageSizes, err := getHugepageSizeFromFilenames(files)
	if err != nil {
		lh.V(2).Error(err, "detecting system hugepages")
	}

	lh.V(4).Info("detected system hugepages", "supportedSizes", hugepageSizes)
	return hugepageSizes
}

func getHugepageSizeFromFilenames(fileNames []string) ([]string, error) {
	pageSizes := make([]string, 0, len(fileNames))
	var warn error

	for _, file := range fileNames {
		// example: hugepages-1048576kB
		val, ok := strings.CutPrefix(file, "hugepages-")
		if !ok {
			// Unexpected file name: no prefix found, ignore it.
			continue
		}
		// The suffix is always "kB" (as of Linux 5.13). If we find
		// something else, produce an error but keep going.
		eLen := len(val) - 2
		val = strings.TrimSuffix(val, "kB")
		if len(val) != eLen {
			// Highly unlikely.
			if warn == nil {
				warn = errors.New(file + `: invalid suffix (expected "kB")`)
			}
			continue
		}
		size, err := strconv.Atoi(val)
		if err != nil {
			// Highly unlikely.
			if warn == nil {
				warn = fmt.Errorf("%s: %w", file, err)
			}
			continue
		}
		// Model after https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/tree/mm/hugetlb_cgroup.c?id=eff48ddeab782e35e58ccc8853f7386bbae9dec4#n574
		// but in our case the size is in KB already.
		switch {
		case size >= (1 << 20):
			val = strconv.Itoa(size>>20) + "GB"
		case size >= (1 << 10):
			val = strconv.Itoa(size>>10) + "MB"
		default:
			val += "KB"
		}
		pageSizes = append(pageSizes, val)
	}

	return pageSizes, warn
}
