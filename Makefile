# Copyright 2025 Kubernetes Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

REPO_ROOT:=${CURDIR}
OUT_DIR=$(REPO_ROOT)/bin

# platform on which we run
OS=$(shell go env GOOS)
ARCH=$(shell go env GOARCH)

# dependencies
## versions
YQ_VERSION ?= 4.47.1
# matches golang 1.24.z
GOLANGCI_LINT_VERSION ?= 2.4.0
# paths
YQ = $(OUT_DIR)/yq
GOLANGCI_LINT = $(OUT_DIR)/golangci-lint

# disable CGO by default for static binaries
CGO_ENABLED=0
export GOROOT GO111MODULE CGO_ENABLED

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

default: build ## Default builds

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-27s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

build: build-dramemory build-setuphelpers ## build all the binaries

build-dramemory: ## build dramemory
	go build -v -o "$(OUT_DIR)/dramemory" ./cmd/dramemory

build-setuphelpers: build-setup-containerd ## build the configuration setup helpers

build-setup-containerd: ## build the containerd configuration setup helper
	go build -v -o "$(OUT_DIR)/setup-containerd" ./config/containerd

clean: ## clean
	rm -rf "$(OUT_DIR)/"

test-unit: ## run tests
	go test -coverprofile=coverage.out ./pkg/... ./internal/...

update: ## runs go mod tidy
	go mod tidy

$(OUT_DIR):  ## creates the output directory (used internally)
	mkdir -p $(OUT_DIR)

.PHONY: vet
vet:  ## vet the source code tree
	go vet ./pkg/... ./internal/... ./cmd/...

# get image name from directory we're building
CLUSTER_NAME=dra-driver-memory
STAGING_REPO_NAME=dra-driver-memory
IMAGE_NAME=dra-driver-memory
# container image registry, default to upstream
REGISTRY := quay.io/fromani
# this is an intentionally non-existent registry to be used only by local CI using the local image loading
REGISTRY_CI := dev.kind.local/ci
STAGING_IMAGE_NAME := ${REGISTRY}/${STAGING_REPO_NAME}/${IMAGE_NAME}
TESTING_IMAGE_NAME := ${REGISTRY}/${IMAGE_NAME}-test
# tag based on date-sha
GIT_VERSION := $(shell date +v%Y%m%d)-$(shell git rev-parse --short HEAD)
ifneq ($(shell git status --porcelain),)
	GIT_VERSION := $(GIT_VERSION)-dirty
endif
TAG ?= $(GIT_VERSION)
# the full image tag
IMAGE := ${STAGING_IMAGE_NAME}:${TAG}
IMAGE_TESTING := "${TESTING_IMAGE_NAME}:${TAG}"
IMAGE_CI := ${REGISTRY_CI}/${IMAGE_NAME}:${TAG}
IMAGE_TEST := ${REGISTRY_CI}/${IMAGE_NAME}-test:${TAG}
# target platform(s)
PLATFORMS?=linux/amd64

CONTAINER_ENGINE?=docker

build-image: ## build image
	${CONTAINER_ENGINE} build . \
		--platform="${PLATFORMS}" \
		--tag="${IMAGE}" \
		--tag="${IMAGE_CI}" \
		--load

# no need to push the test image
# never push the CI image! it intentionally refers to a non-existing registry
push-image: build-image ## build and push image
	${CONTAINER_ENGINE} push ${IMAGE}

kind-cluster:  ## create kind cluster
	kind create cluster --name ${CLUSTER_NAME} --config hack/kind.yaml

kind-load-image: build-image  ## load the current container image into kind
	kind load docker-image ${IMAGE} --name ${CLUSTER_NAME}

ci-kind-setup: ci-manifests build-image ## setup a CI cluster from scratch
	kind create cluster --name ${CLUSTER_NAME} --config hack/ci/kind-ci.yaml
	kubectl label node ${CLUSTER_NAME}-worker node-role.kubernetes.io/worker=''
	kind load docker-image --name ${CLUSTER_NAME} ${IMAGE_CI}
	kubectl create -f hack/ci/install-ci.yaml
	hack/ci/wait-resourcelices.sh

ci-kind-teardown:  ## teardown a CI cluster
	kind delete cluster --name ${CLUSTER_NAME}

lint:  ## run the linter against the codebase
	$(GOLANGCI_LINT) run ./...
$(GOLANGCI_LINT): dep-install-golangci-lint

ci-manifests: hack/ci/install.tmpl.yaml dep-install-yq ## create the CI install manifests
	@cd hack/ci && ../../bin/yq e -s '(.kind | downcase) + "-" + .metadata.name + ".part.yaml"' ../../hack/ci/install.tmpl.yaml
	@# need to make kind load docker-image working as expected: see https://kind.sigs.k8s.io/docs/user/quick-start/#loading-an-image-into-your-cluster
	@bin/yq -i '.spec.template.spec.containers[0].imagePullPolicy = "IfNotPresent"' hack/ci/daemonset-dramemory.part.yaml
	@bin/yq -i '.spec.template.spec.containers[0].image = "${IMAGE_CI}"' hack/ci/daemonset-dramemory.part.yaml
	@bin/yq -i '.spec.template.metadata.labels["build"] = "${GIT_VERSION}"' hack/ci/daemonset-dramemory.part.yaml
	@bin/yq '.' \
		hack/ci/clusterrole-dramemory.part.yaml \
		hack/ci/serviceaccount-dramemory.part.yaml \
		hack/ci/clusterrolebinding-dramemory.part.yaml \
		hack/ci/daemonset-dramemory.part.yaml \
		hack/ci/deviceclass-dra.memory.part.yaml \
		hack/ci/deviceclass-dra.hugepages-1g.part.yaml \
		hack/ci/deviceclass-dra.hugepages-2m.part.yaml \
		> hack/ci/install-ci.yaml
	@rm hack/ci/*.part.yaml

# dependencies
.PHONY:
dep-install-yq: ## make sure the yq tool is available locally
	@# TODO: generalize platform/os?
	@if [ ! -f bin/yq ]; then\
	       mkdir -p bin;\
	       curl -L https://github.com/mikefarah/yq/releases/download/v4.47.1/yq_linux_amd64 -o bin/yq;\
               chmod 0755 bin/yq;\
	fi

.PHONY: dep-install-golangci-lint
dep-install-golangci-lint: $(OUT_DIR)  ## Download golangci-lint locally if necessary, or reuse the system binary
	@[ ! -f $(OUT_DIR)/golangci-lint ] && { \
		command -v golangci-lint >/dev/null 2>&1 && {\
			ln -sf $(shell command -v golangci-lint ) $(OUT_DIR) ;\
			echo "reusing system golangci-lint" ;\
		} || { \
			curl -sSL "https://github.com/golangci/golangci-lint/releases/download/v$(GOLANGCI_LINT_VERSION)/golangci-lint-$(GOLANGCI_LINT_VERSION)-$(OS)-$(ARCH).tar.gz" -o $(GOLANGCI_LINT)-$(GOLANGCI_LINT_VERSION)-$(OS)-$(ARCH).tar.gz ;\
			tar -x -C $(OUT_DIR) -f $(GOLANGCI_LINT)-$(GOLANGCI_LINT_VERSION)-$(OS)-$(ARCH).tar.gz ;\
			ln -sf $(GOLANGCI_LINT)-$(GOLANGCI_LINT_VERSION)-$(OS)-$(ARCH)/golangci-lint $(GOLANGCI_LINT)-$(GOLANGCI_LINT_VERSION) ;\
			ln -sf $(GOLANGCI_LINT)-$(GOLANGCI_LINT_VERSION) $(GOLANGCI_LINT) ;\
		}; \
	} || true
