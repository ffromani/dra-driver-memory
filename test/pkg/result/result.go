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

package result

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ffromani/dra-driver-memory/pkg/unitconv"
)

const (
	Prefix = ">>>::RESULT="
)

type Result struct {
	Request Request `json:"request"`
	Status  Status  `json:"status"`
}

type Request struct {
	Size        string `json:"size"`
	SizeInBytes uint64 `json:"sizeInBytes"`
	HugeTLB     bool   `json:"hugeTLB"`
	NUMANodes   string `json:"numaNodes"`
}

type Status struct {
	Code    int    `json:"code"`
	Reason  Reason `json:"reason"`
	Message string `json:"message"`
}

func New(allocSize uint64, hugeTLB bool, numaNodes string) *Result {
	return &Result{
		Request: Request{
			Size:        unitconv.SizeInBytesToMinimizedString(allocSize),
			SizeInBytes: allocSize,
			HugeTLB:     hugeTLB,
			NUMANodes:   numaNodes,
		},
	}
}

func (res *Result) Finalize(code int, reason Reason, fmt_ string, args ...any) int {
	message := fmt.Sprintf(fmt_, args...)
	res.Status = Status{
		Code:    code,
		Reason:  reason,
		Message: message,
	}
	data, err := json.Marshal(res)
	if err == nil {
		fmt.Println(Prefix + string(data))
	}
	return code
}

func FromString(s string) (st *Result, err error) {
	st = &Result{}
	err = json.Unmarshal([]byte(s), st)
	return
}

func FromLogs(logs string) (st *Result, err error) {
	scanner := bufio.NewScanner(strings.NewReader(logs))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, Prefix) {
			continue
		}
		line = strings.TrimPrefix(line, Prefix)
		return FromString(line)
	}
	return nil, errors.New("no result found in logs")
}

type Reason string

const (
	Succeeded             Reason = "Succeeded"
	FailureGeneric        Reason = "GenericFailure"
	FailedAsExpected      Reason = "FailedAsExpected"
	UnexpectedMMapError   Reason = "UnexpectedMMapError"
	UnexpectedMMapSuccess Reason = "MMapShouldHaveFailed"
	CannotCheckAllocation Reason = "CannotCheckAllocation"
	NUMAOverflown         Reason = "AllocatedOverMultipleNUMANodes"
	NUMAMismatch          Reason = "AllocatedOverUnexpectedNUMANodes"
)
