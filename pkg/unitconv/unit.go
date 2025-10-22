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

import "strings"

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

func AmountFor(size uint64) (uint64, string) {
	if size < MiB {
		return KiB, "KiB"
	}
	if size < GiB {
		return MiB, "MiB"
	}
	if size < TiB {
		return GiB, "GiB"
	}
	if size < PiB {
		return TiB, "TiB"
	}
	if size < EiB {
		return PiB, "PiB"
	}
	return EiB, "EiB"
}

func Minimize(unitName string) string {
	if unitName == "" {
		return ""
	}
	return strings.ToLower(string(unitName[0]))
}
