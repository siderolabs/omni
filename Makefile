# THIS FILE WAS AUTOMATICALLY GENERATED, PLEASE DO NOT EDIT.
#
# Generated on 2026-01-30T12:27:48Z by kres dc032d7.

# common variables

SHA := $(shell git describe --match=none --always --abbrev=8 --dirty)
TAG := $(shell git describe --tag --always --dirty --match v[0-9]\*)
TAG_SUFFIX ?=
ABBREV_TAG := $(shell git describe --tags >/dev/null 2>/dev/null && git describe --tag --always --match v[0-9]\* --abbrev=0 || echo 'undefined')
BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
ARTIFACTS := _out
IMAGE_TAG ?= $(TAG)$(TAG_SUFFIX)
OPERATING_SYSTEM := $(shell uname -s | tr '[:upper:]' '[:lower:]')
GOARCH := $(shell uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/')
WITH_DEBUG ?= false
WITH_RACE ?= false
REGISTRY ?= ghcr.io
USERNAME ?= siderolabs
REGISTRY_AND_USERNAME ?= $(REGISTRY)/$(USERNAME)
PROTOBUF_GRPC_GATEWAY_TS_VERSION ?= 1.3.1
TESTPKGS ?= ./...
JS_BUILD_ARGS ?=
PROTOBUF_GO_VERSION ?= 1.36.11
GRPC_GO_VERSION ?= 1.6.0
GRPC_GATEWAY_VERSION ?= 2.27.4
VTPROTOBUF_VERSION ?= 0.6.0
GOIMPORTS_VERSION ?= 0.41.0
GOMOCK_VERSION ?= 0.6.0
DEEPCOPY_VERSION ?= v0.5.8
GOLANGCILINT_VERSION ?= v2.8.0
GOFUMPT_VERSION ?= v0.9.2
GO_VERSION ?= 1.25.6
GO_BUILDFLAGS ?=
GO_BUILDTAGS ?= ,
GO_LDFLAGS ?=
CGO_ENABLED ?= 0
GOTOOLCHAIN ?= local
GOEXPERIMENT ?=
GO_BUILDFLAGS += -tags $(GO_BUILDTAGS)
HELMREPO ?= $(REGISTRY)/$(USERNAME)/charts
COSIGN_ARGS ?=
HELMDOCS_VERSION ?= v1.14.2
KRES_IMAGE ?= ghcr.io/siderolabs/kres:latest
CONFORMANCE_IMAGE ?= ghcr.io/siderolabs/conform:latest

# docker build settings

BUILD := docker buildx build
PLATFORM ?= linux/amd64
PROGRESS ?= auto
PUSH ?= false
CI_ARGS ?=
WITH_BUILD_DEBUG ?=
BUILDKIT_MULTI_PLATFORM ?=
COMMON_ARGS = --file=Dockerfile
COMMON_ARGS += --provenance=false
COMMON_ARGS += --progress=$(PROGRESS)
COMMON_ARGS += --platform=$(PLATFORM)
COMMON_ARGS += --build-arg=BUILDKIT_MULTI_PLATFORM=$(BUILDKIT_MULTI_PLATFORM)
COMMON_ARGS += --push=$(PUSH)
COMMON_ARGS += --build-arg=ARTIFACTS="$(ARTIFACTS)"
COMMON_ARGS += --build-arg=SHA="$(SHA)"
COMMON_ARGS += --build-arg=TAG="$(TAG)"
COMMON_ARGS += --build-arg=ABBREV_TAG="$(ABBREV_TAG)"
COMMON_ARGS += --build-arg=USERNAME="$(USERNAME)"
COMMON_ARGS += --build-arg=REGISTRY="$(REGISTRY)"
COMMON_ARGS += --build-arg=JS_TOOLCHAIN="$(JS_TOOLCHAIN)"
COMMON_ARGS += --build-arg=PROTOBUF_GRPC_GATEWAY_TS_VERSION="$(PROTOBUF_GRPC_GATEWAY_TS_VERSION)"
COMMON_ARGS += --build-arg=JS_BUILD_ARGS="$(JS_BUILD_ARGS)"
COMMON_ARGS += --build-arg=TOOLCHAIN="$(TOOLCHAIN)"
COMMON_ARGS += --build-arg=CGO_ENABLED="$(CGO_ENABLED)"
COMMON_ARGS += --build-arg=GO_BUILDFLAGS="$(GO_BUILDFLAGS)"
COMMON_ARGS += --build-arg=GO_LDFLAGS="$(GO_LDFLAGS)"
COMMON_ARGS += --build-arg=GOTOOLCHAIN="$(GOTOOLCHAIN)"
COMMON_ARGS += --build-arg=GOEXPERIMENT="$(GOEXPERIMENT)"
COMMON_ARGS += --build-arg=PROTOBUF_GO_VERSION="$(PROTOBUF_GO_VERSION)"
COMMON_ARGS += --build-arg=GRPC_GO_VERSION="$(GRPC_GO_VERSION)"
COMMON_ARGS += --build-arg=GRPC_GATEWAY_VERSION="$(GRPC_GATEWAY_VERSION)"
COMMON_ARGS += --build-arg=VTPROTOBUF_VERSION="$(VTPROTOBUF_VERSION)"
COMMON_ARGS += --build-arg=GOIMPORTS_VERSION="$(GOIMPORTS_VERSION)"
COMMON_ARGS += --build-arg=GOMOCK_VERSION="$(GOMOCK_VERSION)"
COMMON_ARGS += --build-arg=DEEPCOPY_VERSION="$(DEEPCOPY_VERSION)"
COMMON_ARGS += --build-arg=GOLANGCILINT_VERSION="$(GOLANGCILINT_VERSION)"
COMMON_ARGS += --build-arg=GOFUMPT_VERSION="$(GOFUMPT_VERSION)"
COMMON_ARGS += --build-arg=TESTPKGS="$(TESTPKGS)"
COMMON_ARGS += --build-arg=HELMDOCS_VERSION="$(HELMDOCS_VERSION)"
JS_TOOLCHAIN ?= docker.io/node:24.13.0-alpine
TOOLCHAIN ?= docker.io/golang:1.25-alpine

# extra variables

REMOVE_VOLUMES ?= false

# help menu

export define HELP_MENU_HEADER
# Getting Started

To build this project, you must have the following installed:

- git
- make
- docker (19.03 or higher)

## Creating a Builder Instance

The build process makes use of experimental Docker features (buildx).
To enable experimental features, add 'experimental: "true"' to '/etc/docker/daemon.json' on
Linux or enable experimental features in Docker GUI for Windows or Mac.

To create a builder instance, run:

	docker buildx create --name local --use

If running builds that needs to be cached aggresively create a builder instance with the following:

	docker buildx create --name local --use --config=config.toml

config.toml contents:

[worker.oci]
  gc = true
  gckeepstorage = 50000

  [[worker.oci.gcpolicy]]
    keepBytes = 10737418240
    keepDuration = 604800
    filters = [ "type==source.local", "type==exec.cachemount", "type==source.git.checkout"]
  [[worker.oci.gcpolicy]]
    all = true
    keepBytes = 53687091200

If you already have a compatible builder instance, you may use that instead.

## Artifacts

All artifacts will be output to ./$(ARTIFACTS). Images will be tagged with the
registry "$(REGISTRY)", username "$(USERNAME)", and a dynamic tag (e.g. $(IMAGE):$(IMAGE_TAG)).
The registry and username can be overridden by exporting REGISTRY, and USERNAME
respectively.

endef

ifneq (, $(filter $(WITH_BUILD_DEBUG), t true TRUE y yes 1))
BUILD := BUILDX_EXPERIMENTAL=1 docker buildx debug --invoke /bin/sh --on error build
endif

ifneq (, $(filter $(WITH_RACE), t true TRUE y yes 1))
GO_BUILDFLAGS += -race
CGO_ENABLED := 1
GO_LDFLAGS += -linkmode=external -extldflags '-static'
endif

ifneq (, $(filter $(WITH_DEBUG), t true TRUE y yes 1))
GO_BUILDTAGS := $(GO_BUILDTAGS)sidero.debug,
else
GO_LDFLAGS += -s
endif

all: unit-tests-frontend lint-eslint frontend unit-tests-client unit-tests acompat make-cookies omni image-omni omnictl helm integration-test image-omni-integration-test lint

$(ARTIFACTS):  ## Creates artifacts directory.
	@mkdir -p $(ARTIFACTS)

.PHONY: clean
clean:  ## Cleans up all artifacts.
	@rm -rf $(ARTIFACTS)

target-%:  ## Builds the specified target defined in the Dockerfile. The build result will only remain in the build cache.
	@$(BUILD) --target=$* $(COMMON_ARGS) $(TARGET_ARGS) $(CI_ARGS) .

registry-%:  ## Builds the specified target defined in the Dockerfile and the output is an image. The image is pushed to the registry if PUSH=true.
	@$(MAKE) target-$* TARGET_ARGS="--tag=$(REGISTRY)/$(USERNAME)/$(IMAGE_NAME):$(IMAGE_TAG)" BUILDKIT_MULTI_PLATFORM=1

local-%:  ## Builds the specified target defined in the Dockerfile using the local output type. The build result will be output to the specified local destination.
	@$(MAKE) target-$* TARGET_ARGS="--output=type=local,dest=$(DEST) $(TARGET_ARGS)"
	@PLATFORM=$(PLATFORM) DEST=$(DEST) bash -c '\
	  for platform in $$(tr "," "\n" <<< "$$PLATFORM"); do \
	    directory="$${platform//\//_}"; \
	    if [[ -d "$$DEST/$$directory" ]]; then \
		  echo $$platform; \
	      mv -f "$$DEST/$$directory/"* $$DEST; \
	      rmdir "$$DEST/$$directory/"; \
	    fi; \
	  done'

.PHONY: check-dirty
check-dirty:
	@if test -n "`git status --porcelain`"; then echo "Source tree is dirty"; git status; git diff; exit 1 ; fi

generate-frontend:  ## Generate .proto definitions.
	@$(MAKE) local-$@ DEST=./

.PHONY: js
js:  ## Prepare js base toolchain.
	@$(MAKE) target-$@

.PHONY: unit-tests-frontend
unit-tests-frontend:  ## Performs unit tests
	@$(MAKE) target-$@

lint-eslint:  ## Runs eslint linter & prettier style check.
	@$(MAKE) target-$@

lint-eslint-fmt:  ## Runs eslint & prettier and tries to fix issues automatically, updating the source tree.
	@$(MAKE) local-$@ DEST=.

.PHONY: $(ARTIFACTS)/frontend-js
$(ARTIFACTS)/frontend-js:
	@$(MAKE) target-frontend

.PHONY: frontend
frontend: $(ARTIFACTS)/frontend-js  ## Builds js release for frontend.

generate:  ## Generate .proto definitions.
	@$(MAKE) local-$@ DEST=./
	@sed -i "s/appVersion: .*/appVersion: \"$$(cat internal/version/data/tag)\"/" deploy/helm/v2/omni/Chart.yaml

lint-golangci-lint-client:  ## Runs golangci-lint linter.
	@$(MAKE) target-$@

lint-golangci-lint-client-fmt:  ## Runs golangci-lint formatter and tries to fix issues automatically.
	@$(MAKE) local-$@ DEST=.

lint-gofumpt-client:  ## Runs gofumpt linter.
	@$(MAKE) target-$@

.PHONY: fmt
fmt:  ## Formats the source code
	@docker run --rm -it -v $(PWD):/src -w /src golang:$(GO_VERSION) \
		bash -c "export GOTOOLCHAIN=local; \
		export GO111MODULE=on; export GOPROXY=https://proxy.golang.org; \
		go install mvdan.cc/gofumpt@$(GOFUMPT_VERSION) && \
		gofumpt -w ."

lint-govulncheck-client:  ## Runs govulncheck linter.
	@$(MAKE) target-$@

lint-golangci-lint:  ## Runs golangci-lint linter.
	@$(MAKE) target-$@

lint-golangci-lint-fmt:  ## Runs golangci-lint formatter and tries to fix issues automatically.
	@$(MAKE) local-$@ DEST=.

lint-gofumpt:  ## Runs gofumpt linter.
	@$(MAKE) target-$@

lint-govulncheck:  ## Runs govulncheck linter.
	@$(MAKE) target-$@

.PHONY: base
base: frontend  ## Prepare base toolchain
	@$(MAKE) target-$@

.PHONY: unit-tests-client
unit-tests-client:  ## Performs unit tests
	@$(MAKE) local-$@ DEST=$(ARTIFACTS)

.PHONY: unit-tests-client-race
unit-tests-client-race:  ## Performs unit tests with race detection enabled.
	@$(MAKE) target-$@

.PHONY: unit-tests
unit-tests:  ## Performs unit tests
	@$(MAKE) local-$@ DEST=$(ARTIFACTS)

.PHONY: unit-tests-race
unit-tests-race:  ## Performs unit tests with race detection enabled.
	@$(MAKE) target-$@

.PHONY: $(ARTIFACTS)/acompat-linux-amd64
$(ARTIFACTS)/acompat-linux-amd64:
	@$(MAKE) local-acompat-linux-amd64 DEST=$(ARTIFACTS)

.PHONY: acompat-linux-amd64
acompat-linux-amd64: $(ARTIFACTS)/acompat-linux-amd64  ## Builds executable for acompat-linux-amd64.

.PHONY: acompat
acompat: acompat-linux-amd64  ## Builds executables for acompat.

.PHONY: $(ARTIFACTS)/make-cookies-linux-amd64
$(ARTIFACTS)/make-cookies-linux-amd64:
	@$(MAKE) local-make-cookies-linux-amd64 DEST=$(ARTIFACTS)

.PHONY: make-cookies-linux-amd64
make-cookies-linux-amd64: $(ARTIFACTS)/make-cookies-linux-amd64  ## Builds executable for make-cookies-linux-amd64.

.PHONY: make-cookies
make-cookies: make-cookies-linux-amd64  ## Builds executables for make-cookies.

.PHONY: $(ARTIFACTS)/omni-darwin-amd64
$(ARTIFACTS)/omni-darwin-amd64:
	@$(MAKE) local-omni-darwin-amd64 DEST=$(ARTIFACTS)

.PHONY: omni-darwin-amd64
omni-darwin-amd64: $(ARTIFACTS)/omni-darwin-amd64  ## Builds executable for omni-darwin-amd64.

.PHONY: $(ARTIFACTS)/omni-darwin-arm64
$(ARTIFACTS)/omni-darwin-arm64:
	@$(MAKE) local-omni-darwin-arm64 DEST=$(ARTIFACTS)

.PHONY: omni-darwin-arm64
omni-darwin-arm64: $(ARTIFACTS)/omni-darwin-arm64  ## Builds executable for omni-darwin-arm64.

.PHONY: $(ARTIFACTS)/omni-linux-amd64
$(ARTIFACTS)/omni-linux-amd64:
	@$(MAKE) local-omni-linux-amd64 DEST=$(ARTIFACTS)

.PHONY: omni-linux-amd64
omni-linux-amd64: $(ARTIFACTS)/omni-linux-amd64  ## Builds executable for omni-linux-amd64.

.PHONY: $(ARTIFACTS)/omni-linux-arm64
$(ARTIFACTS)/omni-linux-arm64:
	@$(MAKE) local-omni-linux-arm64 DEST=$(ARTIFACTS)

.PHONY: omni-linux-arm64
omni-linux-arm64: $(ARTIFACTS)/omni-linux-arm64  ## Builds executable for omni-linux-arm64.

.PHONY: omni
omni: omni-darwin-amd64 omni-darwin-arm64 omni-linux-amd64 omni-linux-arm64  ## Builds executables for omni.

.PHONY: lint-markdown
lint-markdown:  ## Runs markdownlint.
	@$(MAKE) target-$@

.PHONY: lint
lint: lint-eslint lint-golangci-lint-client lint-gofumpt-client lint-govulncheck-client lint-golangci-lint lint-gofumpt lint-govulncheck lint-markdown  ## Run all linters for the project.

.PHONY: lint-fmt
lint-fmt: lint-eslint-fmt lint-golangci-lint-client-fmt lint-golangci-lint-fmt  ## Run all linter formatters and fix up the source tree.

.PHONY: image-omni
image-omni:  ## Builds image for omni.
	@$(MAKE) registry-$@ IMAGE_NAME="omni"

.PHONY: $(ARTIFACTS)/omnictl-darwin-amd64
$(ARTIFACTS)/omnictl-darwin-amd64:
	@$(MAKE) local-omnictl-darwin-amd64 DEST=$(ARTIFACTS)

.PHONY: omnictl-darwin-amd64
omnictl-darwin-amd64: $(ARTIFACTS)/omnictl-darwin-amd64  ## Builds executable for omnictl-darwin-amd64.

.PHONY: $(ARTIFACTS)/omnictl-darwin-arm64
$(ARTIFACTS)/omnictl-darwin-arm64:
	@$(MAKE) local-omnictl-darwin-arm64 DEST=$(ARTIFACTS)

.PHONY: omnictl-darwin-arm64
omnictl-darwin-arm64: $(ARTIFACTS)/omnictl-darwin-arm64  ## Builds executable for omnictl-darwin-arm64.

.PHONY: $(ARTIFACTS)/omnictl-linux-amd64
$(ARTIFACTS)/omnictl-linux-amd64:
	@$(MAKE) local-omnictl-linux-amd64 DEST=$(ARTIFACTS)

.PHONY: omnictl-linux-amd64
omnictl-linux-amd64: $(ARTIFACTS)/omnictl-linux-amd64  ## Builds executable for omnictl-linux-amd64.

.PHONY: $(ARTIFACTS)/omnictl-linux-arm64
$(ARTIFACTS)/omnictl-linux-arm64:
	@$(MAKE) local-omnictl-linux-arm64 DEST=$(ARTIFACTS)

.PHONY: omnictl-linux-arm64
omnictl-linux-arm64: $(ARTIFACTS)/omnictl-linux-arm64  ## Builds executable for omnictl-linux-arm64.

.PHONY: $(ARTIFACTS)/omnictl-windows-amd64.exe
$(ARTIFACTS)/omnictl-windows-amd64.exe:
	@$(MAKE) local-omnictl-windows-amd64.exe DEST=$(ARTIFACTS)

.PHONY: omnictl-windows-amd64.exe
omnictl-windows-amd64.exe: $(ARTIFACTS)/omnictl-windows-amd64.exe  ## Builds executable for omnictl-windows-amd64.exe.

.PHONY: omnictl
omnictl: omnictl-darwin-amd64 omnictl-darwin-arm64 omnictl-linux-amd64 omnictl-linux-arm64 omnictl-windows-amd64.exe  ## Builds executables for omnictl.

.PHONY: helm
helm: $(ARTIFACTS)  ## Package helm chart
	@helm package deploy/helm/v2/omni -d $(ARTIFACTS)

.PHONY: helm-release
helm-release: helm  ## Release helm chart
	@helm push $(ARTIFACTS)/omni-*.tgz oci://$(HELMREPO) 2>&1 | tee $(ARTIFACTS)/.digest
	@cosign sign --yes $(COSIGN_ARGS) $(HELMREPO)/omni@$$(cat $(ARTIFACTS)/.digest | awk -F "[, ]+" '/Digest/{print $$NF}')

.PHONY: chart-lint
chart-lint:  ## Lint helm chart
	@helm lint deploy/helm/v2/omni

.PHONY: helm-plugin-install
helm-plugin-install:  ## Install helm plugins
	-helm plugin install https://github.com/helm-unittest/helm-unittest.git --verify=false --version=v1.0.3
	-helm plugin install https://github.com/losisin/helm-values-schema-json.git --verify=false --version=v2.3.1

.PHONY: kuttl-plugin-install
kuttl-plugin-install:  ## Install kubectl kuttl plugin
	kubectl krew install kuttl

.PHONY: chart-e2e
chart-e2e:  ## Run helm chart e2e tests
	export KUBECONFIG=$(shell pwd)/$(ARTIFACTS)/kubeconfig && cd deploy/helm/v2/e2e && kubectl kuttl test

.PHONY: chart-unittest
chart-unittest: $(ARTIFACTS)  ## Run helm chart unit tests
	@helm unittest deploy/helm/v2/omni --output-type junit --output-file $(ARTIFACTS)/helm-unittest-report.xml

.PHONY: chart-gen-schema
chart-gen-schema:  ## Generate helm chart schema
	@helm schema --use-helm-docs --draft=7 --indent=2 --values=deploy/helm/v2/omni/values.yaml --output=deploy/helm/v2/omni/values.schema.json

.PHONY: helm-docs
helm-docs:  ## Runs helm-docs and generates chart documentation
	@$(MAKE) local-$@ DEST=.

.PHONY: $(ARTIFACTS)/integration-test-darwin-amd64
$(ARTIFACTS)/integration-test-darwin-amd64:
	@$(MAKE) local-integration-test-darwin-amd64 DEST=$(ARTIFACTS)

.PHONY: integration-test-darwin-amd64
integration-test-darwin-amd64: $(ARTIFACTS)/integration-test-darwin-amd64  ## Builds executable for integration-test-darwin-amd64.

.PHONY: $(ARTIFACTS)/integration-test-darwin-arm64
$(ARTIFACTS)/integration-test-darwin-arm64:
	@$(MAKE) local-integration-test-darwin-arm64 DEST=$(ARTIFACTS)

.PHONY: integration-test-darwin-arm64
integration-test-darwin-arm64: $(ARTIFACTS)/integration-test-darwin-arm64  ## Builds executable for integration-test-darwin-arm64.

.PHONY: $(ARTIFACTS)/integration-test-linux-amd64
$(ARTIFACTS)/integration-test-linux-amd64:
	@$(MAKE) local-integration-test-linux-amd64 DEST=$(ARTIFACTS)

.PHONY: integration-test-linux-amd64
integration-test-linux-amd64: $(ARTIFACTS)/integration-test-linux-amd64  ## Builds executable for integration-test-linux-amd64.

.PHONY: $(ARTIFACTS)/integration-test-linux-arm64
$(ARTIFACTS)/integration-test-linux-arm64:
	@$(MAKE) local-integration-test-linux-arm64 DEST=$(ARTIFACTS)

.PHONY: integration-test-linux-arm64
integration-test-linux-arm64: $(ARTIFACTS)/integration-test-linux-arm64  ## Builds executable for integration-test-linux-arm64.

.PHONY: integration-test
integration-test: integration-test-darwin-amd64 integration-test-darwin-arm64 integration-test-linux-amd64 integration-test-linux-arm64  ## Builds executables for integration-test.

.PHONY: image-omni-integration-test
image-omni-integration-test:  ## Builds image for omni-integration-test.
	@$(MAKE) registry-$@ IMAGE_NAME="omni-integration-test"

.PHONY: docker-compose-up
docker-compose-up:
	ARTIFACTS="$(ARTIFACTS)" SHA="$(SHA)" TAG="$(TAG)" USERNAME="$(USERNAME)" REGISTRY="$(REGISTRY)" JS_TOOLCHAIN="$(JS_TOOLCHAIN)" PROTOBUF_TS_VERSION="$(PROTOBUF_TS_VERSION)" PROTOBUF_GRPC_GATEWAY_TS_VERSION="$(PROTOBUF_GRPC_GATEWAY_TS_VERSION)" NODE_BUILD_ARGS="$(NODE_BUILD_ARGS)" TOOLCHAIN="$(TOOLCHAIN)" CGO_ENABLED="$(CGO_ENABLED)" GO_BUILDFLAGS="$(GO_BUILDFLAGS)" GOLANGCILINT_VERSION="$(GOLANGCILINT_VERSION)" GOFUMPT_VERSION="$(GOFUMPT_VERSION)" GOIMPORTS_VERSION="$(GOIMPORTS_VERSION)" GOMOCK_VERSION="$(GOMOCK_VERSION)" PROTOBUF_GO_VERSION="$(PROTOBUF_GO_VERSION)" GRPC_GO_VERSION="$(GRPC_GO_VERSION)" GRPC_GATEWAY_VERSION="$(GRPC_GATEWAY_VERSION)" VTPROTOBUF_VERSION="$(VTPROTOBUF_VERSION)" DEEPCOPY_VERSION="$(DEEPCOPY_VERSION)" TESTPKGS="$(TESTPKGS)" COMPOSE_DOCKER_CLI_BUILD=1 DOCKER_BUILDKIT=1 GO_LDFLAGS="$(GO_LDFLAGS)" GOTOOLCHAIN="$(GOTOOLCHAIN)" GOEXPERIMENT="$(GOEXPERIMENT)" WITH_DEBUG="$(WITH_DEBUG)" docker compose --file ./hack/compose/docker-compose.yml --file ./hack/compose/docker-compose.override.yml up --build

.PHONY: docker-compose-down
docker-compose-down:
	ARTIFACTS="$(ARTIFACTS)" SHA="$(SHA)" TAG="$(TAG)" USERNAME="$(USERNAME)" REGISTRY="$(REGISTRY)" JS_TOOLCHAIN="$(JS_TOOLCHAIN)" PROTOBUF_TS_VERSION="$(PROTOBUF_TS_VERSION)" PROTOBUF_GRPC_GATEWAY_TS_VERSION="$(PROTOBUF_GRPC_GATEWAY_TS_VERSION)" NODE_BUILD_ARGS="$(NODE_BUILD_ARGS)" TOOLCHAIN="$(TOOLCHAIN)" CGO_ENABLED="$(CGO_ENABLED)" GO_BUILDFLAGS="$(GO_BUILDFLAGS)" GOLANGCILINT_VERSION="$(GOLANGCILINT_VERSION)" GOFUMPT_VERSION="$(GOFUMPT_VERSION)" GOIMPORTS_VERSION="$(GOIMPORTS_VERSION)" GOMOCK_VERSION="$(GOMOCK_VERSION)" PROTOBUF_GO_VERSION="$(PROTOBUF_GO_VERSION)" GRPC_GO_VERSION="$(GRPC_GO_VERSION)" GRPC_GATEWAY_VERSION="$(GRPC_GATEWAY_VERSION)" VTPROTOBUF_VERSION="$(VTPROTOBUF_VERSION)" DEEPCOPY_VERSION="$(DEEPCOPY_VERSION)" TESTPKGS="$(TESTPKGS)" COMPOSE_DOCKER_CLI_BUILD=1 DOCKER_BUILDKIT=1 GO_LDFLAGS="$(GO_LDFLAGS)" GOTOOLCHAIN="$(GOTOOLCHAIN)" GOEXPERIMENT="$(GOEXPERIMENT)" WITH_DEBUG="$(WITH_DEBUG)" docker compose --file ./hack/compose/docker-compose.yml --file ./hack/compose/docker-compose.override.yml down --rmi local --remove-orphans --volumes=$(REMOVE_VOLUMES)

.PHONY: mkcert-install
mkcert-install:
	go run ./hack/generate-certs install

.PHONY: mkcert-generate
mkcert-generate:
	go run ./hack/generate-certs -config ./hack/generate-certs.yml generate

.PHONY: mkcert-uninstall
mkcert-uninstall:
	go run ./hack/generate-certs uninstall

run-integration-test: integration-test-linux-amd64 omnictl-linux-amd64 omni-linux-amd64
	@hack/test/integration.sh

.PHONY: rekres
rekres:
	@docker pull $(KRES_IMAGE)
	@docker run --rm --net=host --user $(shell id -u):$(shell id -g) -v $(PWD):/src -w /src -e GITHUB_TOKEN $(KRES_IMAGE)

.PHONY: help
help:  ## This help menu.
	@echo "$$HELP_MENU_HEADER"
	@grep -E '^[a-zA-Z%_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: release-notes
release-notes: $(ARTIFACTS)
	@ARTIFACTS=$(ARTIFACTS) ./hack/release.sh $@ $(ARTIFACTS)/RELEASE_NOTES.md $(TAG)

.PHONY: conformance
conformance:
	@docker pull $(CONFORMANCE_IMAGE)
	@docker run --rm -it -v $(PWD):/src -w /src $(CONFORMANCE_IMAGE) enforce

