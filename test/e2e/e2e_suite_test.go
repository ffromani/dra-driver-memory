/*
Copyright 2025 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

func TestE2E(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "DRA Memory Driver E2E Suite")
}

/*
explanation of the gingko flags we use in the suites:

- Serial:
because the tests want to change the memory allocation, which is a giant blob of node shared state.
- Ordered:
to do the relatively costly initial resource discovery on the target node only once
- ContinueOnFailure
to mitigate the problem that ordered suites stop on the first failure, so an initial failure can mask
a cascade of latter failure; this makes the tests failure troubleshooting painful, as we would need
to fix failures one by one vs in batches.

Note that using "Ordered" may introduce subtle bugs caused by incorrect tests which pollute or leak
state. We should keep looking for ways to eventually remove "Ordered".
Please note "Serial" is however unavoidable because we manage the shared node state.
*/
