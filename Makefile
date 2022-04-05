PLUGIN = kubernetes
PKG = get.porter.sh/plugin/$(PLUGIN)
SHELL = /bin/bash

PORTER_VERSION=v1.0.0-alpha.13
PORTER_HOME = ${PWD}/bin
PORTER_RT = $(PORTER_HOME)/runtimes/porter-runtime

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

test-integration: export CURRENT_CONTEXT=$(shell kubectl config current-context)
test-integration: bin/porter$(FILE_EXT) setup-tests clean-last-testrun
	./tests/integration/local/scripts/test-local-integration.sh
	$(GO) test -v ./tests/integration/local/...;
	kubectl delete namespace $(TEST_NAMESPACE)
	if [[ $$CURRENT_CONTEXT ]]; then \
		kubectl config use-context $$CURRENT_CONTEXT; \
	fi

bin/porter$(FILE_EXT):
	mkdir -p $(PORTER_HOME)
	@curl --silent --http1.1 -lfsSLo bin/porter$(FILE_EXT) https://cdn.porter.sh/$(PORTER_VERSION)/porter-$(CLIENT_PLATFORM)-$(CLIENT_ARCH)$(FILE_EXT)
	chmod +x bin/porter$(FILE_EXT)
	mkdir -p $(PORTER_HOME)/credentials
	echo $(PORTER_HOME)
	cp tests/integration/local/scripts/config-*.toml $(PORTER_HOME)
	cp tests/testdata/kubernetes-plugin-test-*.json $(PORTER_HOME)/credentials
	mkdir -p $(PORTER_HOME)/runtimes
	

bin/runtimes/porter-runtime:
	@echo $(PORTER_HOME)
	mkdir -p bin/runtimes
	@curl --silent --http1.1 -lfsSLo bin/runtimes/porter-runtime https://cdn.porter.sh/$(PORTER_VERSION)/porter-linux-amd64$(FILE_EXT)
	chmod +x bin/runtimes/porter-runtime
	./bin/porter mixin install exec
	mkdir -p $(PORTER_HOME)/outputs/porter-state

setup-tests: | bin/porter$(FILE_EXT) bin/runtimes/porter-runtime
	@echo "Local Porter Home: $(PORTER_HOME)"
	@echo "Porter Runtime: $(PORTER_RT)"

install-linux-porter:
	mkdir -p bin/runtimes
	@curl --silent --http1.1 -lfsSLo bin/runtimes/porter-runtime https://cdn.porter.sh/$(PORTER_VERSION)/porter-linux-amd64$(FILE_EXT)
	chmod +x bin/runtimes/porter-runtime

install:
	mkdir -p $(PORTER_HOME)/plugins/$(PLUGIN)
	install $(BINDIR)/$(PLUGIN)$(FILE_EXT) $(PORTER_HOME)/plugins/$(PLUGIN)/$(PLUGIN)$(FILE_EXT)

clean-last-testrun: 
	-rm -fr testdata/.cnab

clean:
	-rm -fr bin/