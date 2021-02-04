PLUGIN = kubernetes
PKG = get.porter.sh/plugin/$(PLUGIN)
SHELL = bash

PORTER_HOME ?= $(HOME)/.porter

COMMIT ?= $(shell git rev-parse --short HEAD)
VERSION ?= $(shell git describe --tags 2> /dev/null || echo v0)
PERMALINK ?= $(shell git describe --tags --exact-match &> /dev/null && echo latest || echo canary)

GO = GO111MODULE=on go
RECORDTEST = RECORDER_MODE=record $(GO)
LDFLAGS = -w -X $(PKG)/pkg.Version=$(VERSION) -X $(PKG)/pkg.Commit=$(COMMIT)
XBUILD = CGO_ENABLED=0 $(GO) build -a -tags netgo -ldflags '$(LDFLAGS)'
BINDIR = bin/plugins/$(PLUGIN)
KUBERNETES_CONTEXT = docker-desktop

CLIENT_PLATFORM ?= $(shell go env GOOS)
CLIENT_ARCH ?= $(shell go env GOARCH)
SUPPORTED_PLATFORMS = linux darwin windows
SUPPORTED_ARCHES = amd64

ifeq ($(CLIENT_PLATFORM),windows)
FILE_EXT=.exe
else
FILE_EXT=
endif

debug: build-for-debug install

build-for-debug:
	mkdir -p $(BINDIR)
	$(GO) build -o $(BINDIR)/$(PLUGIN)$(FILE_EXT) ./cmd/$(PLUGIN)

.PHONY: build
build:
	mkdir -p $(BINDIR)
	$(GO) build -ldflags '$(LDFLAGS)' -o $(BINDIR)/$(PLUGIN)$(FILE_EXT) ./cmd/$(PLUGIN)

xbuild-all:
	$(foreach OS, $(SUPPORTED_PLATFORMS), \
		$(foreach ARCH, $(SUPPORTED_ARCHES), \
				$(MAKE) $(MAKE_OPTS) CLIENT_PLATFORM=$(OS) CLIENT_ARCH=$(ARCH) PLUGIN=$(PLUGIN) xbuild; \
		))

xbuild: $(BINDIR)/$(VERSION)/$(PLUGIN)-$(CLIENT_PLATFORM)-$(CLIENT_ARCH)$(FILE_EXT)
$(BINDIR)/$(VERSION)/$(PLUGIN)-$(CLIENT_PLATFORM)-$(CLIENT_ARCH)$(FILE_EXT):
	mkdir -p $(dir $@)
	GOOS=$(CLIENT_PLATFORM) GOARCH=$(CLIENT_ARCH) $(XBUILD) -o $@ ./cmd/$(PLUGIN)

test: test-unit test-integration test-in-kubernetes
	$(BINDIR)/$(PLUGIN)$(FILE_EXT) version

test-unit: build
	$(GO) test ./...;	

test-integration: build
	export CURRENT_CONTEXT=$$(kubectl config current-context)
 	kubectl config use-context $(KUBERNETES_CONTEXT)
	$(GO) test -tags=integration ./tests/integration/...
	if [[ ! -z $$CURRENT_CONTEXT ]]; then 
		kubectl config use-context $$CURRENT_CONTEXT; 
	fi

test-in-kubernetes: build
	export CURRENT_CONTEXT=$$(kubectl config current-context)
 	kubectl config use-context $(KUBERNETES_CONTEXT)
	kubectl apply -f tests/setup.yaml
	docker build -f ./tests/Dockerfile -t localhost:5000/test:latest .
	docker push localhost:5000/test:latest
	kubectl run test-$$RANDOM --attach=true --image=localhost:5000/test:latest --restart=Never --serviceaccount=porter-plugin-test-sa -n porter-plugin-test-ns
	kubectl delete -f tests/setup.yaml
	if [[ ! -z $$CURRENT_CONTEXT ]]; then
		kubectl config use-context $$CURRENT_CONTEXT
	fi

clean:
	-rm -fr bin/
