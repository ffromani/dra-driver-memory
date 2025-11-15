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

FROM --platform=$BUILDPLATFORM golang:1.24 AS builder
ARG TARGETARCH
ARG GOARCH=${TARGETARCH} CGO_ENABLED=0

# cache go modules
WORKDIR /go/src/drv
COPY go.mod go.sum .
RUN go mod download

# build
COPY . .
RUN go build -o /go/bin/dramemory ./cmd/dramemory
RUN go build -o /go/bin/setup-containerd ./config/containerd/setup.go

# copy binary onto base image
FROM gcr.io/distroless/base-debian12
COPY --from=builder --chown=root:root /go/bin/dramemory /dramemory
COPY --from=builder --chown=root:root /go/bin/setup-containerd /setup-containerd
CMD ["/dramemory"]
