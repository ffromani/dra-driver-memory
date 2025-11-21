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

import "testing"

func TestValidateHugePageSize(t *testing.T) {
	type testcase struct {
		hps           HugePageSize
		expectedValue string
		expectedError bool
	}
	prefix := func(tc testcase) string {
		if !tc.expectedError {
			return "valid"
		}
		return "invalid"
	}
	testcases := []testcase{
		// positive cases
		{
			hps:           "1G",
			expectedValue: "1048576kB",
			expectedError: false,
		},
		{
			hps:           "1Gi",
			expectedValue: "1048576kB",
			expectedError: false,
		},
		{
			hps:           "1g",
			expectedValue: "1048576kB",
			expectedError: false,
		},
		{
			hps:           "2M",
			expectedValue: "2048kB",
			expectedError: false,
		},
		{
			hps:           "2Mi",
			expectedValue: "2048kB",
			expectedError: false,
		},
		{
			hps:           "2m",
			expectedValue: "2048kB",
			expectedError: false,
		},
		// negative cases
		{
			hps:           "4k",
			expectedError: true,
		},
		{
			hps:           "64k",
			expectedError: true,
		},
	}

	for _, tcase := range testcases {
		t.Run(prefix(tcase)+"-"+string(tcase.hps), func(t *testing.T) {
			val, err := ValidateHugePageSize(tcase.hps)
			gotErr := (err != nil)
			if gotErr != tcase.expectedError {
				t.Fatalf("got error %v expected %v", err, tcase.expectedError)
			}
			if val != tcase.expectedValue {
				t.Fatalf("got value %v expected %v", val, tcase.expectedValue)
			}
		})
	}
}
