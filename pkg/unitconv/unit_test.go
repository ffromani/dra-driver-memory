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
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSizeToStringRoundTrip(t *testing.T) {
	type testcase struct {
		sval string
		uval uint64
		fail bool
	}

	testcases := []testcase{
		// good cases, add them at the bottom of the section
		{
			sval: "4k",
			uval: 4 * 1024,
		},
		{
			sval: "64k",
			uval: 64 * 1024,
		},
		{
			sval: "2m",
			uval: 2 * 1024 * 1024,
		},
		{
			sval: "1g",
			uval: 1024 * 1024 * 1024,
		},
		// bad cases, add them at the bottom of the section
		{
			sval: "",
			fail: true,
		},
		{
			sval: "-1",
			fail: true,
		},
		{
			sval: "g",
			fail: true,
		},
		{
			sval: "Kk",
			fail: true,
		},
		{
			sval: "8b",
			fail: true,
		},
		{
			sval: "42xb",
			fail: true,
		},
		{
			sval: "1pb",
			fail: true,
		},
	}

	for _, tcase := range testcases {
		t.Run(fmt.Sprintf("%s=%d", tcase.sval, tcase.uval), func(t *testing.T) {
			ugot, err := MinimizedStringToSizeInBytes(tcase.sval)
			if tcase.fail {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, ugot, tcase.uval)
			sgot := SizeInBytesToMinimizedString(ugot)
			require.Equal(t, sgot, tcase.sval)
		})
	}
}

func TestCGroupStringToSizeRoundTrip(t *testing.T) {
	type testcase struct {
		sval string
		uval uint64
		fail bool
	}

	testcases := []testcase{
		// good cases, add them at the bottom of the section
		{
			sval: "4KB",
			uval: 4 * 1024,
		},
		{
			sval: "64KB",
			uval: 64 * 1024,
		},
		{
			sval: "2MB",
			uval: 2 * 1024 * 1024,
		},
		{
			sval: "1GB",
			uval: 1024 * 1024 * 1024,
		},
		// bad cases, add them at the bottom of the section
		{
			sval: "",
			fail: true,
		},
		{
			sval: "-1",
			fail: true,
		},
		{
			sval: "GB",
			fail: true,
		},
		{
			sval: "KKB",
			fail: true,
		},
		{
			sval: "8B",
			fail: true,
		},
		{
			sval: "42XB",
			fail: true,
		},
		{
			sval: "1PB",
			fail: true,
		},
	}

	for _, tcase := range testcases {
		t.Run(fmt.Sprintf("%s=%d", tcase.sval, tcase.uval), func(t *testing.T) {
			ugot, err := CGroupStringToSizeInBytes(tcase.sval)
			if tcase.fail {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, ugot, tcase.uval)
			sgot := SizeInBytesToCGroupString(ugot)
			require.Equal(t, sgot, tcase.sval)
		})
	}
}
