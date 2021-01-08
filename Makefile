CONTAINER_RUNTIME ?= podman

IMAGE_REGISTRY ?= docker.io
IMAGE_NAME ?= capabilities-demo
IMAGE_PULL_POLICY ?= Always
IMAGE_TAG ?= latest

NAMESPACE ?= default

TARGETS = \
    fmt \
    fmt-check \
    vet

# tools
GITHUB_RELEASE ?= $(GOBIN)/github-release

# Make does not offer a recursive wildcard function, so here's one:
rwildcard=$(wildcard $1$2) $(foreach d,$(wildcard $1*),$(call rwildcard,$d/,$2))

# Gather needed source files and directories to create target dependencies
directories=$(filter-out ./ ./vendor/ ,$(sort $(dir $(wildcard ./*/))))
all_sources=$(call rwildcard,$(directories),*) $(filter-out $(TARGETS), $(wildcard *))
go_sources=$(call rwildcard,cmd/,*.go) $(call rwildcard,pkg/,*.go) $(call rwildcard,tests/,*.go)

# Configure Go
export GOOS=linux
export GOARCH=amd64
export CGO_ENABLED=0
export GO111MODULE=on
export GOFLAGS=-mod=vendor

BIN_DIR = $(CURDIR)/build/_output/bin/
export GOROOT=$(BIN_DIR)/go/
export GOBIN=$(GOROOT)/bin/
export PATH := $(GOBIN):$(PATH)
GOFMT := $(GOBIN)/gofmt
export GO := $(GOBIN)/go

.ONESHELL:

all: clean vendor format container-build

$(GO):
	hack/install-go.sh $(BIN_DIR) > /dev/null

$(GOFMT): $(GO)

fmt: $(GOFMT)
	$(GOFMT) -d cmd/

fmt-check: $(GOFMT)
	$(GOFMT) -w cmd/

vet: $(GO)
	$(GO) vet ./cmd/...

vendor: $(GO)
	$(GO) mod tidy
	$(GO) mod vendor

format: fmt fmt-check vet

container-build: fmt-check vendor
	$(CONTAINER_RUNTIME) build -t ${IMAGE_REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG} -f ./build/Dockerfile .

container-push: container-build
	$(CONTAINER_RUNTIME) push ${IMAGE_REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}

clean:
	rm -rf build/_output

.PHONY: \
	all \
	clean \
	container-build \
	container-push \
	format \
	vendor \
	vet
