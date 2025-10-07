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
GOLANGCI_LINT_VERSION ?= 2.3.0
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
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-23s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

build: build-dramemory ## build all the binaries

build-dramemory: ## build dramemory
	go build -v -o "$(OUT_DIR)/dramemory" ./cmd/dramemory

clean: ## clean
	rm -rf "$(OUT_DIR)/"

test-unit: ## run tests
	go test -coverprofile=coverage.out ./pkg/... ./internal/...

update: ## runs go mod tidy
	go mod tidy

$(OUT_DIR):  ## creates the output directory (used internally)
	mkdir -p $(OUT_DIR)

# get image name from directory we're building
CLUSTER_NAME=dra-driver-memory
STAGING_REPO_NAME=dra-driver-memory
IMAGE_NAME=dra-driver-memory
# podman image registry, default to upstream
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
IMAGE_LATEST?=$(STAGING_IMAGE_NAME):latest
IMAGE := ${STAGING_IMAGE_NAME}:${TAG}
IMAGE_TESTING := "${TESTING_IMAGE_NAME}:${TAG}"
IMAGE_CI := ${REGISTRY_CI}/${IMAGE_NAME}:${TAG}
IMAGE_TEST := ${REGISTRY_CI}/${IMAGE_NAME}-test:${TAG}
# target platform(s)
PLATFORMS?=linux/amd64

# required to enable buildx
export DOCKER_CLI_EXPERIMENTAL=enabled
image: ## podman build load
	podman build . -t ${STAGING_IMAGE_NAME} --load

build-image: ## build image
	podman build . \
		--platform="${PLATFORMS}" \
		--tag="${IMAGE}" \
		--tag="${IMAGE_LATEST}" \
		--tag="${IMAGE_CI}" \
		--load

# no need to push the test image
# never push the CI image! it intentionally refers to a non-existing registry
push-image: build-image ## build and push image
	podman push ${IMAGE}
	podman push ${IMAGE_LATEST}

kind-cluster:  ## create kind cluster
	kind create cluster --name ${CLUSTER_NAME} --config hack/kind.yaml

kind-load-image: build-image  ## load the current container image into kind
	kind load podman-image ${IMAGE} ${IMAGE_LATEST} --name ${CLUSTER_NAME}

kind-uninstall-dra-memory: ## remove cpu dra from kind cluster
	kubectl delete -f install.yaml || true

kind-install-dra-memory: kind-uninstall-dra-memory build-image kind-load-image ## install on cluster
	kubectl apply -f install.yaml

delete-kind-cluster: ## delete kind cluster
	kind delete cluster --name ${CLUSTER_NAME}

ci-kind-setup: ci-manifests build-image build-test-image ## setup a CI cluster from scratch
	kind create cluster --name ${CLUSTER_NAME} --config hack/ci/kind-ci.yaml
	kubectl label node ${CLUSTER_NAME}-worker node-role.kubernetes.io/worker=''
	kind load podman-image --name ${CLUSTER_NAME} ${IMAGE_CI} ${IMAGE_TEST}
	kubectl create -f hack/ci/install-ci.yaml
	hack/ci/wait-resourcelices.sh

lint:  ## run the linter against the codebase
	$(GOLANGCI_LINT) run ./...
