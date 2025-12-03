# Copyright 2025 The Kubernetes Authors.
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

# setup cross-compile env
FROM --platform=$BUILDPLATFORM golang:1.24 AS builder
ARG TARGETARCH
ARG GOARCH=${TARGETARCH} CGO_ENABLED=0

# cache go modules
WORKDIR /go/src/app
COPY go.mod go.sum .
RUN go mod download

# build
COPY . .
RUN go build -o /go/bin/dramemtester ./test/image/dramemtester

# copy binaries onto base image
FROM registry.fedoraproject.org/fedora-minimal:43
RUN rpm -ivh http://fedora.ipacct.com/fedora/linux/updates/43/Everything/x86_64/Packages/p/procps-ng-4.0.4-7.fc43.1.x86_64.rpm
COPY --from=builder --chown=root:root /go/bin/dramemtester /bin/dramemtester
CMD ["/bin/dramemtester"]

