PLUGIN = kubernetes
PKG = get.porter.sh/plugin/$(PLUGIN)
SHELL = /bin/bash

PORTER_HOME = $(HOME)/.porter

COMMIT ?= $(shell git rev-parse --short HEAD)
VERSION ?= $(shell git describe --tags 2> /dev/null || echo v0)
PERMALINK ?= $(shell git describe --tags --exact-match &> /dev/null && echo latest || echo canary)

GO = GO111MODULE=on go
RECORDTEST = RECORDER_MODE=record $(GO)
LDFLAGS = -w -X $(PKG)/pkg.Version=$(VERSION) -X $(PKG)/pkg.Commit=$(COMMIT)
XBUILD = CGO_ENABLED=0 $(GO) build -a -tags netgo -ldflags '$(LDFLAGS)'
BINDIR = bin/plugins/$(PLUGIN)
KUBERNETES_CONTEXT = kind-porter
TEST_NAMESPACE=porter-local-plugin-test-ns

CLIENT_PLATFORM ?= $(shell go env GOOS)
CLIENT_ARCH ?= $(shell go env GOARCH)
SUPPORTED_PLATFORMS = linux darwin windows
SUPPORTED_ARCHES = amd64
TIMEOUT = 240s

ifeq ($(CLIENT_PLATFORM),windows)
FILE_EXT=.exe
else
FILE_EXT=
endif

test: test-unit test-integration test-in-kubernetes 
	$(BINDIR)/$(PLUGIN)$(FILE_EXT) version

test-unit: build
	$(GO) test $(shell go list ./... |grep -v tests/integration|grep -v vendor );
	
publish: bin/porter$(FILE_EXT)
	go run mage.go -v Publish $(PLUGIN) $(VERSION) $(PERMALINK)
