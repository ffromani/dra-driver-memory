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

package unitconv

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	KB uint64 = 1000
	MB        = KB * 1000
	GB        = MB * 1000
	TB        = GB * 1000
	PB        = TB * 1000
	EB        = PB * 1000
)

const (
	KiB uint64 = 1024
	MiB        = KiB * 1024
	GiB        = MiB * 1024
	TiB        = GiB * 1024
	PiB        = TiB * 1024
	EiB        = PiB * 1024
)

func NarrowSize(size uint64) (uint64, string) {
	if size == 0 { // corner case
		return 0, "B"
	}
	if size%EiB == 0 {
		return size / EiB, "EiB"
	}
	if size%PiB == 0 {
		return size / PiB, "PiB"
	}
	if size%TiB == 0 {
		return size / TiB, "TiB"
	}
	if size%GiB == 0 {
		return size / GiB, "GiB"
	}
	if size%MiB == 0 {
		return size / MiB, "MiB"
	}
	if size%KiB == 0 {
		return size / KiB, "KiB"
	}
	return size, "B"
}

func Minimize(unitName string) string {
	if unitName == "" {
		return ""
	}
	return strings.ToLower(string(unitName[0]))
}

func SizeInBytesToMinimizedString(sizeInBytes uint64) string {
	value, unit := NarrowSize(sizeInBytes)
	return strconv.FormatUint(value, 10) + Minimize(unit)
}

func MinimizedStringToSizeInBytes(sz string) (uint64, error) {
	if len(sz) < 2 {
		return 0, errors.New("malformed string: too small")
	}
	// NOTE: need to be a lowercase RFC 1123 label
	mults := map[byte]uint64{
		byte('b'): 1 << 0,
		byte('k'): 1 << 10,
		byte('m'): 1 << 20,
		byte('g'): 1 << 30,
		byte('t'): 1 << 40,
		byte('p'): 1 << 50,
		byte('e'): 1 << 60,
	}
	unit := sz[len(sz)-1]
	rval := sz[:len(sz)-1]
	value, err := strconv.ParseUint(rval, 10, 64)
	if err != nil {
		return 0, err
	}
	mulp, ok := mults[unit]
	if !ok {
		return 0, fmt.Errorf("unsupported unit: %q", unit)
	}
	return value * mulp, nil
}

func SizeInBytesToCGroupString(sizeInBytes uint64) string {
	// translated from https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/tree/mm/hugetlb_cgroup.c?id=eff48ddeab782e35e58ccc8853f7386bbae9dec4#n574
	if sizeInBytes >= (1 << 30) {
		return fmt.Sprintf("%dGB", sizeInBytes>>30)
	}
	if sizeInBytes >= (1 << 20) {
		return fmt.Sprintf("%dMB", sizeInBytes>>20)
	}
	return fmt.Sprintf("%dKB", sizeInBytes>>10)
}

func CGroupStringToSizeInBytes(cs string) (uint64, error) {
	if len(cs) < 3 {
		return 0, errors.New("malformed string: too small")
	}
	mults := map[string]uint64{
		"KB": 1 << 10,
		"MB": 1 << 20,
		"GB": 1 << 30,
	}
	unit := cs[len(cs)-2:]
	rval := cs[:len(cs)-2]
	value, err := strconv.ParseUint(rval, 10, 64)
	if err != nil {
		return 0, err
	}
	mulp, ok := mults[unit]
	if !ok {
		return 0, fmt.Errorf("unsupported unit: %q", unit)
	}
	return value * mulp, nil
}
