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
TEST_NAMESPACE=porter-plugin-test-ns

CLIENT_PLATFORM ?= $(shell go env GOOS)
CLIENT_ARCH ?= $(shell go env GOARCH)
SUPPORTED_PLATFORMS = linux darwin windows
SUPPORTED_ARCHES = amd64
TESTS = secret storage both
TIMEOUT = 240s

ifeq ($(CLIENT_PLATFORM),windows)
FILE_EXT=.exe
else
FILE_EXT=
endif

debug: clean build-for-debug  bin/porter$(FILE_EXT)

debug-in-vscode: clean build-for-debug install

build-for-debug:
	mkdir -p $(BINDIR)
	$(GO) build -o $(BINDIR)/$(PLUGIN)$(FILE_EXT) ./cmd/$(PLUGIN)

.PHONY: build
build: clean
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
test-integration: export CURRENT_CONTEXT=$(shell kubectl config current-context)
test-integration: export PORTER_HOME=$(shell echo $${PWD}/bin)
test-integration: build bin/porter$(FILE_EXT) setup-tests clean-last-testrun
	kubectl config use-context $(KUBERNETES_CONTEXT)
	kubectl create namespace $(TEST_NAMESPACE)  --dry-run=client -o yaml | kubectl apply -f -
	kubectl create secret generic password --from-literal=credential=test --namespace $(TEST_NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	$(foreach TEST,$(TESTS), \
			cp ./tests/integration/scripts/config-$(TEST)-ns.toml $$PORTER_HOME/config.toml; \
			cp ./tests/testdata/kubernetes-plugin-test-$(TEST).json $$PORTER_HOME/credentials/kubernetes-plugin-test.json; \
			if [[ $(TEST) == "storage" ]]; then kubectl apply -f ./tests/testdata/credentials-storage.yaml -n $(TEST_NAMESPACE); fi; \
			if [[ $(TEST) == "both" ]]; then kubectl apply -f ./tests/testdata/credentials-secret.yaml -n $(TEST_NAMESPACE); fi; \
			./bin/porter storage migrate; \
			cd tests/testdata && ../../bin/porter install --cred kubernetes-plugin-test && cd ../..; \
			if [[ $$(./bin/porter installations outputs show test_out -i kubernetes-plugin-test) != "test" ]]; then (exit 1); fi; \
			./bin/porter installations show kubernetes-plugin-test; \
		)
	$(GO) test -tags=integration ./tests/integration/...;
	kubectl delete namespace $(TEST_NAMESPACE)
	if [[ $$CURRENT_CONTEXT ]]; then \
		kubectl config use-context $$CURRENT_CONTEXT; \
	fi

test-in-kubernetes: export CURRENT_CONTEXT=$(shell kubectl config current-context)
test-in-kubernetes: export PORTER_HOME=$(shell echo $${PWD}/bin)
test-in-kubernetes: build bin/porter$(FILE_EXT) setup-tests clean-last-testrun
	kubectl config use-context $(KUBERNETES_CONTEXT)
	kubectl apply -f ./tests/integration/scripts/setup.yaml
	kubectl wait --timeout=$(TIMEOUT) --for=condition=ready pod/docker-registry --namespace $(TEST_NAMESPACE) 
	cd tests/testdata && ../../bin/porter publish 
	docker build -f ./tests/integration/scripts/Dockerfile -t localhost:5000/test:latest .
	docker push localhost:5000/test:latest
	kubectl apply -f ./tests/integration/scripts/run-test-pod.yaml --namespace $(TEST_NAMESPACE)
	kubectl wait --timeout=$(TIMEOUT) --for=condition=ready pod/test --namespace $(TEST_NAMESPACE) 
	cd tests/testdata && ../../bin/porter publish 
	kubectl create secret generic password --from-literal=credential=test --namespace $(TEST_NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	kubectl exec --stdin --tty test -n $(TEST_NAMESPACE) -- go test -tags=integration ./tests/integration/...
	kubectl exec --stdin --tty test -n  $(TEST_NAMESPACE) -- tests/integration/scripts/test-with-porter.sh
	kubectl delete -f ./tests/integration/scripts/setup.yaml
	if [[ $$CURRENT_CONTEXT ]]; then \
			kubectl config use-context $$CURRENT_CONTEXT; \
	fi

publish: bin/porter$(FILE_EXT)
	# AZURE_STORAGE_CONNECTION_STRING will be used for auth in the following commands
	if [[ "$(PERMALINK)" == "latest" ]]; then \
		az storage blob upload-batch -d porter/plugins/$(PLUGIN)/$(VERSION) -s $(BINDIR)/$(VERSION); \
		az storage blob upload-batch -d porter/plugins/$(PLUGIN)/$(PERMALINK) -s $(BINDIR)/$(VERSION); \
	else \
		mv $(BINDIR)/$(VERSION) $(BINDIR)/$(PERMALINK); \
		az storage blob upload-batch -d porter/plugins/$(PLUGIN)/$(PERMALINK) -s $(BINDIR)/$(PERMALINK); \
	fi

	# Generate the plugin feed
	az storage blob download -c porter -n plugins/atom.xml -f bin/plugins/atom.xml
	bin/porter mixins feed generate -d bin/plugins -f bin/plugins/atom.xml -t build/atom-template.xml
	az storage blob upload -c porter -n plugins/atom.xml -f bin/plugins/atom.xml

bin/porter$(FILE_EXT): export PORTER_HOME=$(shell echo $${PWD}/bin)
bin/porter$(FILE_EXT): 
	curl --http1.1 -lvfsSLo bin/porter$(FILE_EXT) https://cdn.porter.sh/latest/porter-$(CLIENT_PLATFORM)-$(CLIENT_ARCH)$(FILE_EXT)
	chmod +x bin/porter$(FILE_EXT)

setup-tests:
	mkdir -p $$PORTER_HOME/credentials
	cp tests/integration/scripts/config-*.toml $$PORTER_HOME
	cp tests/testdata/kubernetes-plugin-test-*.json $$PORTER_HOME/credentials
	mkdir -p $$PORTER_HOME/runtimes
	cp bin/porter $$PORTER_HOME/runtimes/porter-runtime
	./bin/porter mixin install exec

install:
	mkdir -p $(PORTER_HOME)/plugins/$(PLUGIN)
	install $(BINDIR)/$(PLUGIN)$(FILE_EXT) $(PORTER_HOME)/plugins/$(PLUGIN)/$(PLUGIN)$(FILE_EXT)

clean-last-testrun: 
	-rm -fr testdata/.cnab

clean:
	-rm -fr bin/