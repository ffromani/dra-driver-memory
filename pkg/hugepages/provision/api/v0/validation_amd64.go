//go:build amd64

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
package v0

import "errors"

// ValidateHugePageSize returns the internal (sysfs) hugepage size to use
// and nil error if is a supported size; otherwise returns empty string
// and an error detailing the reason
func ValidateHugePageSize(hps HugePageSize) (string, error) {
	hpSize := string(hps) // shortcut
	if hpSize == "1G" || hpSize == "1Gi" || hpSize == "1g" {
		return "1048576kB", nil
	}
	if hpSize == "2M" || hpSize == "2Mi" || hpSize == "2m" {
		return "2048kB", nil
	}
	return "", errors.New("unsupported size")
}
