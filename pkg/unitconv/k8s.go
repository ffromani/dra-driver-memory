package unitconv

import "fmt"

// HugePageUnitSizeFromByteSize returns hugepage size has the format.
// `size` must be guaranteed to divisible into the largest units that can be expressed.
// <size><unit-prefix>B (1024 = "1KB", 1048576 = "1MB", etc).
//
// Borrowed from pkg/apis/core/v1/helper/helpers.go@854e67bb51e
func HugePageUnitSizeFromByteSize(size int64) (string, error) {
	// hugePageSizeUnitList is borrowed from opencontainers/runc/libcontainer/cgroups/utils.go
	var hugePageSizeUnitList = []string{"B", "KB", "MB", "GB", "TB", "PB"}
	idx := 0
	len := len(hugePageSizeUnitList) - 1
	for size%1024 == 0 && idx < len {
		size /= 1024
		idx++
	}
	if size > 1024 && idx < len {
		return "", fmt.Errorf("size: %d%s must be guaranteed to divisible into the largest units", size, hugePageSizeUnitList[idx])
	}
	return fmt.Sprintf("%d%s", size, hugePageSizeUnitList[idx]), nil
}
