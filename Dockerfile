# syntax = docker/dockerfile-upstream:1.8.1-labs

# THIS FILE WAS AUTOMATICALLY GENERATED, PLEASE DO NOT EDIT.
#
# Generated on 2024-07-09T15:20:03Z by kres 8c8b007.

ARG JS_TOOLCHAIN
ARG TOOLCHAIN

FROM ghcr.io/siderolabs/ca-certificates:v1.7.0 AS image-ca-certificates

FROM ghcr.io/siderolabs/fhs:v1.7.0 AS image-fhs

# base toolchain image
FROM --platform=${BUILDPLATFORM} ${JS_TOOLCHAIN} AS js-toolchain
RUN apk --update --no-cache add bash curl protoc protobuf-dev go
COPY ./go.mod .
COPY ./go.sum .
ENV GOPATH=/go
ENV PATH=${PATH}:/usr/local/go/bin

# runs markdownlint
FROM docker.io/oven/bun:1.1.17-alpine AS lint-markdown
WORKDIR /src
RUN bun i markdownlint-cli@0.41.0 sentences-per-line@0.2.1
COPY .markdownlint.json .
COPY ./docs ./docs
COPY ./CHANGELOG.md ./CHANGELOG.md
COPY ./CONTRIBUTING.md ./CONTRIBUTING.md
COPY ./DEVELOPMENT.md ./DEVELOPMENT.md
COPY ./README.md ./README.md
RUN bunx markdownlint --ignore "CHANGELOG.md" --ignore "**/node_modules/**" --ignore '**/hack/chglog/**' --rules node_modules/sentences-per-line/index.js .

# collects proto specs
FROM scratch AS proto-specs
ADD client/api/common/omni.proto /client/api/common/
ADD client/api/omni/resources/resources.proto /client/api/omni/resources/
ADD client/api/omni/management/management.proto /client/api/omni/management/
ADD client/api/omni/oidc/oidc.proto /client/api/omni/oidc/
ADD client/api/omni/specs/auth.proto /client/api/omni/specs/
ADD client/api/omni/specs/virtual.proto /client/api/omni/specs/
ADD client/api/omni/specs/ephemeral.proto /client/api/omni/specs/
ADD client/api/omni/specs/oidc.proto /client/api/omni/specs/
ADD client/api/omni/specs/omni.proto /client/api/omni/specs/
ADD client/api/omni/specs/siderolink.proto /client/api/omni/specs/
ADD client/api/omni/specs/system.proto /client/api/omni/specs/
ADD https://raw.githubusercontent.com/googleapis/googleapis/master/google/rpc/status.proto /client/api/google/rpc/
ADD https://raw.githubusercontent.com/siderolabs/talos/3c8f51d707b897fb34ed3a9f7c32b7cd3e5ee5b0/api/common/common.proto /client/api/common/
ADD https://raw.githubusercontent.com/siderolabs/talos/3c8f51d707b897fb34ed3a9f7c32b7cd3e5ee5b0/api/machine/machine.proto /client/api/talos/machine/
ADD https://raw.githubusercontent.com/cosi-project/specification/c644a4b0fd408ec41bd29193bcdbd1a5b7feead2/proto/v1alpha1/resource.proto /client/api/v1alpha1/

# collects proto specs
FROM scratch AS proto-specs-frontend
ADD client/api/common/omni.proto /frontend/src/api/common/
ADD client/api/omni/resources/resources.proto /frontend/src/api/omni/resources/
ADD client/api/omni/management/management.proto /frontend/src/api/omni/management/
ADD client/api/omni/oidc/oidc.proto /frontend/src/api/omni/oidc/
ADD https://raw.githubusercontent.com/siderolabs/go-api-signature/a034e9ff315ba4a56115acc7ad0fb99d0dc77800/api/auth/auth.proto /frontend/src/api/omni/auth/
ADD client/api/omni/specs/omni.proto /frontend/src/api/omni/specs/
ADD client/api/omni/specs/siderolink.proto /frontend/src/api/omni/specs/
ADD client/api/omni/specs/system.proto /frontend/src/api/omni/specs/
ADD client/api/omni/specs/auth.proto /frontend/src/api/omni/specs/
ADD client/api/omni/specs/virtual.proto /frontend/src/api/omni/specs/
ADD client/api/omni/specs/ephemeral.proto /frontend/src/api/omni/specs/
ADD https://raw.githubusercontent.com/googleapis/googleapis/master/google/rpc/status.proto /frontend/src/api/google/rpc/
ADD https://raw.githubusercontent.com/siderolabs/talos/03a20da9da5916c63015d355f4b56823778e994f/api/machine/machine.proto /frontend/src/api/talos/machine/
ADD https://raw.githubusercontent.com/protocolbuffers/protobuf/master/src/google/protobuf/any.proto /frontend/src/api/google/protobuf/
ADD https://raw.githubusercontent.com/protocolbuffers/protobuf/master/src/google/protobuf/duration.proto /frontend/src/api/google/protobuf/
ADD https://raw.githubusercontent.com/protocolbuffers/protobuf/master/src/google/protobuf/empty.proto /frontend/src/api/google/protobuf/
ADD https://raw.githubusercontent.com/protocolbuffers/protobuf/master/src/google/protobuf/timestamp.proto /frontend/src/api/google/protobuf/
ADD https://raw.githubusercontent.com/googleapis/googleapis/master/google/rpc/code.proto /frontend/src/api/google/rpc/
ADD https://raw.githubusercontent.com/siderolabs/talos/03a20da9da5916c63015d355f4b56823778e994f/api/common/common.proto /frontend/src/api/common/
ADD https://raw.githubusercontent.com/cosi-project/specification/5c734257bfa6a3acb01417809797dbfbe0e73c71/proto/v1alpha1/resource.proto /frontend/src/api/v1alpha1/

# base toolchain image
FROM --platform=${BUILDPLATFORM} ${TOOLCHAIN} AS toolchain
RUN apk --update --no-cache add bash curl build-base protoc protobuf-dev

# tools and sources
FROM --platform=${BUILDPLATFORM} js-toolchain AS js
WORKDIR /src
ARG PROTOBUF_GRPC_GATEWAY_TS_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go install github.com/siderolabs/protoc-gen-grpc-gateway-ts@v${PROTOBUF_GRPC_GATEWAY_TS_VERSION}
RUN mv /go/bin/protoc-gen-grpc-gateway-ts /bin
COPY frontend/package.json ./
RUN --mount=type=cache,target=/src/node_modules bun install
COPY frontend/tsconfig*.json ./
COPY frontend/bunfig.toml ./
COPY frontend/*.html ./
COPY frontend/*.ts ./
COPY frontend/*.js ./
COPY ./frontend/src ./src
COPY ./frontend/test ./test
COPY ./frontend/eslint.config.js ./eslint.config.js
COPY ./frontend/postcss.config.js ./postcss.config.js

# build tools
FROM --platform=${BUILDPLATFORM} toolchain AS tools
ENV GO111MODULE=on
ARG CGO_ENABLED
ENV CGO_ENABLED=${CGO_ENABLED}
ARG GOTOOLCHAIN
ENV GOTOOLCHAIN=${GOTOOLCHAIN}
ARG GOEXPERIMENT
ENV GOEXPERIMENT=${GOEXPERIMENT}
ENV GOPATH=/go
ARG GOIMPORTS_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go install golang.org/x/tools/cmd/goimports@v${GOIMPORTS_VERSION}
RUN mv /go/bin/goimports /bin
ARG PROTOBUF_GO_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go install google.golang.org/protobuf/cmd/protoc-gen-go@v${PROTOBUF_GO_VERSION}
RUN mv /go/bin/protoc-gen-go /bin
ARG GRPC_GO_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v${GRPC_GO_VERSION}
RUN mv /go/bin/protoc-gen-go-grpc /bin
ARG GRPC_GATEWAY_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v${GRPC_GATEWAY_VERSION}
RUN mv /go/bin/protoc-gen-grpc-gateway /bin
ARG VTPROTOBUF_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go install github.com/planetscale/vtprotobuf/cmd/protoc-gen-go-vtproto@v${VTPROTOBUF_VERSION}
RUN mv /go/bin/protoc-gen-go-vtproto /bin
ARG DEEPCOPY_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go install github.com/siderolabs/deep-copy@${DEEPCOPY_VERSION} \
	&& mv /go/bin/deep-copy /bin/deep-copy
ARG GOLANGCILINT_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go install github.com/golangci/golangci-lint/cmd/golangci-lint@${GOLANGCILINT_VERSION} \
	&& mv /go/bin/golangci-lint /bin/golangci-lint
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go install golang.org/x/vuln/cmd/govulncheck@latest \
	&& mv /go/bin/govulncheck /bin/govulncheck
ARG GOFUMPT_VERSION
RUN go install mvdan.cc/gofumpt@${GOFUMPT_VERSION} \
	&& mv /go/bin/gofumpt /bin/gofumpt

# builds frontend
FROM --platform=${BUILDPLATFORM} js AS frontend
ARG JS_BUILD_ARGS
RUN --mount=type=cache,target=/src/node_modules bun run build ${JS_BUILD_ARGS}
RUN mkdir -p /internal/frontend/dist
RUN cp -rf ./dist/* /internal/frontend/dist

# runs eslint
FROM js AS lint-eslint
RUN --mount=type=cache,target=/src/node_modules bun run lint

# runs protobuf compiler
FROM js AS proto-compile-frontend
COPY --from=proto-specs-frontend / /
RUN protoc -I/frontend/src/api --grpc-gateway-ts_out=source_relative:/frontend/src/api --grpc-gateway-ts_opt=use_proto_names=true /frontend/src/api/common/omni.proto
RUN protoc -I/frontend/src/api --grpc-gateway-ts_out=source_relative:/frontend/src/api --grpc-gateway-ts_opt=use_proto_names=true /frontend/src/api/omni/resources/resources.proto
RUN protoc -I/frontend/src/api --grpc-gateway-ts_out=source_relative:/frontend/src/api --grpc-gateway-ts_opt=use_proto_names=true /frontend/src/api/omni/management/management.proto
RUN protoc -I/frontend/src/api --grpc-gateway-ts_out=source_relative:/frontend/src/api --grpc-gateway-ts_opt=use_proto_names=true /frontend/src/api/omni/oidc/oidc.proto
RUN protoc -I/frontend/src/api --grpc-gateway-ts_out=source_relative:/frontend/src/api --grpc-gateway-ts_opt=use_proto_names=true /frontend/src/api/omni/auth/auth.proto
RUN protoc -I/frontend/src/api --grpc-gateway-ts_out=source_relative:/frontend/src/api --grpc-gateway-ts_opt=use_proto_names=true /frontend/src/api/omni/specs/omni.proto
RUN protoc -I/frontend/src/api --grpc-gateway-ts_out=source_relative:/frontend/src/api --grpc-gateway-ts_opt=use_proto_names=true /frontend/src/api/omni/specs/siderolink.proto
RUN protoc -I/frontend/src/api --grpc-gateway-ts_out=source_relative:/frontend/src/api --grpc-gateway-ts_opt=use_proto_names=true /frontend/src/api/omni/specs/system.proto
RUN protoc -I/frontend/src/api --grpc-gateway-ts_out=source_relative:/frontend/src/api --grpc-gateway-ts_opt=use_proto_names=true /frontend/src/api/omni/specs/auth.proto
RUN protoc -I/frontend/src/api --grpc-gateway-ts_out=source_relative:/frontend/src/api --grpc-gateway-ts_opt=use_proto_names=true /frontend/src/api/omni/specs/virtual.proto
RUN protoc -I/frontend/src/api --grpc-gateway-ts_out=source_relative:/frontend/src/api --grpc-gateway-ts_opt=use_proto_names=true /frontend/src/api/omni/specs/ephemeral.proto
RUN protoc -I/frontend/src/api --grpc-gateway-ts_out=source_relative:/frontend/src/api --grpc-gateway-ts_opt=use_proto_names=true /frontend/src/api/google/rpc/status.proto
RUN protoc -I/frontend/src/api --grpc-gateway-ts_out=source_relative:/frontend/src/api --grpc-gateway-ts_opt=use_proto_names=true /frontend/src/api/talos/machine/machine.proto
RUN protoc -I/frontend/src/api --grpc-gateway-ts_out=source_relative:/frontend/src/api --grpc-gateway-ts_opt=use_proto_names=true /frontend/src/api/google/protobuf/any.proto
RUN protoc -I/frontend/src/api --grpc-gateway-ts_out=source_relative:/frontend/src/api --grpc-gateway-ts_opt=use_proto_names=true /frontend/src/api/google/protobuf/duration.proto
RUN protoc -I/frontend/src/api --grpc-gateway-ts_out=source_relative:/frontend/src/api --grpc-gateway-ts_opt=use_proto_names=true /frontend/src/api/google/protobuf/empty.proto
RUN protoc -I/frontend/src/api --grpc-gateway-ts_out=source_relative:/frontend/src/api --grpc-gateway-ts_opt=use_proto_names=true /frontend/src/api/google/protobuf/timestamp.proto
RUN protoc -I/frontend/src/api --grpc-gateway-ts_out=source_relative:/frontend/src/api --grpc-gateway-ts_opt=use_proto_names=true /frontend/src/api/google/rpc/code.proto
RUN protoc -I/frontend/src/api --grpc-gateway-ts_out=source_relative:/frontend/src/api --grpc-gateway-ts_opt=use_proto_names=true /frontend/src/api/common/common.proto
RUN protoc -I/frontend/src/api --grpc-gateway-ts_out=source_relative:/frontend/src/api --grpc-gateway-ts_opt=use_proto_names=true /frontend/src/api/v1alpha1/resource.proto
RUN rm /frontend/src/api/common/omni.proto
RUN rm /frontend/src/api/omni/resources/resources.proto
RUN rm /frontend/src/api/omni/management/management.proto
RUN rm /frontend/src/api/omni/oidc/oidc.proto
RUN rm /frontend/src/api/omni/specs/omni.proto
RUN rm /frontend/src/api/omni/specs/siderolink.proto
RUN rm /frontend/src/api/omni/specs/system.proto
RUN rm /frontend/src/api/omni/specs/auth.proto
RUN rm /frontend/src/api/omni/specs/virtual.proto
RUN rm /frontend/src/api/omni/specs/ephemeral.proto

# runs js unit-tests
FROM js AS unit-tests-frontend
RUN --mount=type=cache,target=/src/node_modules bun add -d @happy-dom/global-registrator
RUN --mount=type=cache,target=/src/node_modules CI=true bun run test

FROM tools AS embed-generate
ARG SHA
ARG TAG
WORKDIR /src
RUN mkdir -p internal/version/data && \
    echo -n ${SHA} > internal/version/data/sha && \
    echo -n ${TAG} > internal/version/data/tag

# runs protobuf compiler
FROM tools AS proto-compile
COPY --from=proto-specs / /
RUN protoc -I/client/api --go_out=paths=source_relative:/client/api --go-grpc_out=paths=source_relative:/client/api --go-vtproto_out=paths=source_relative:/client/api --go-vtproto_opt=features=marshal+unmarshal+size+equal+clone /client/api/common/omni.proto
RUN protoc -I/client/api --grpc-gateway_out=paths=source_relative:/client/api --grpc-gateway_opt=generate_unbound_methods=true --go_out=paths=source_relative:/client/api --go-grpc_out=paths=source_relative:/client/api --go-vtproto_out=paths=source_relative:/client/api --go-vtproto_opt=features=marshal+unmarshal+size+equal+clone /client/api/omni/resources/resources.proto /client/api/omni/management/management.proto /client/api/omni/oidc/oidc.proto /client/api/omni/specs/auth.proto /client/api/omni/specs/virtual.proto /client/api/omni/specs/ephemeral.proto /client/api/omni/specs/oidc.proto /client/api/omni/specs/omni.proto /client/api/omni/specs/siderolink.proto /client/api/omni/specs/system.proto
RUN protoc -I/client/api --grpc-gateway_out=paths=source_relative:/client/api --grpc-gateway_opt=generate_unbound_methods=true --grpc-gateway_opt=standalone=true /client/api/google/rpc/status.proto /client/api/common/common.proto /client/api/talos/machine/machine.proto /client/api/v1alpha1/resource.proto
RUN rm /client/api/common/omni.proto
RUN rm /client/api/omni/resources/resources.proto
RUN rm /client/api/omni/management/management.proto
RUN rm /client/api/omni/oidc/oidc.proto
RUN rm /client/api/omni/specs/auth.proto
RUN rm /client/api/omni/specs/virtual.proto
RUN rm /client/api/omni/specs/ephemeral.proto
RUN rm /client/api/omni/specs/oidc.proto
RUN rm /client/api/omni/specs/omni.proto
RUN rm /client/api/omni/specs/siderolink.proto
RUN rm /client/api/omni/specs/system.proto
RUN goimports -w -local github.com/siderolabs/omni/client,github.com/siderolabs/omni /client/api
RUN gofumpt -w /client/api

# tools and sources
FROM tools AS base
WORKDIR /src
COPY client/go.mod client/go.mod
COPY client/go.sum client/go.sum
COPY go.mod go.mod
COPY go.sum go.sum
RUN cd client
RUN --mount=type=cache,target=/go/pkg go mod download
RUN --mount=type=cache,target=/go/pkg go mod verify
RUN cd .
RUN --mount=type=cache,target=/go/pkg go mod download
RUN --mount=type=cache,target=/go/pkg go mod verify
COPY ./client/api ./client/api
COPY ./client/pkg ./client/pkg
COPY ./cmd ./cmd
COPY ./internal ./internal
COPY --from=frontend /internal/frontend/dist ./internal/frontend/dist
RUN --mount=type=cache,target=/go/pkg go list -mod=readonly all >/dev/null

# cleaned up specs and compiled versions
FROM scratch AS generate-frontend
ADD https://www.talos.dev/v1.7/schemas/config.schema.json frontend/src/schemas/config.schema.json
COPY --from=proto-compile-frontend frontend/ frontend/

FROM embed-generate AS embed-abbrev-generate
WORKDIR /src
ARG ABBREV_TAG
RUN echo -n 'undefined' > internal/version/data/sha && \
    echo -n ${ABBREV_TAG} > internal/version/data/tag

# run go generate
FROM base AS go-generate-0
WORKDIR /src
COPY .license-header.go.txt hack/.license-header.go.txt
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go generate ./internal/...
RUN goimports -w -local github.com/siderolabs/omni/client,github.com/siderolabs/omni ./internal

# runs gofumpt
FROM base AS lint-gofumpt
RUN FILES="$(gofumpt -l .)" && test -z "${FILES}" || (echo -e "Source code is not formatted with 'gofumpt -w .':\n${FILES}"; exit 1)

# runs gofumpt
FROM base AS lint-gofumpt-client
RUN FILES="$(gofumpt -l client)" && test -z "${FILES}" || (echo -e "Source code is not formatted with 'gofumpt -w client':\n${FILES}"; exit 1)

# runs golangci-lint
FROM base AS lint-golangci-lint
WORKDIR /src
COPY .golangci.yml .
ENV GOGC=50
RUN golangci-lint config verify --config .golangci.yml
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/root/.cache/golangci-lint --mount=type=cache,target=/go/pkg golangci-lint run --config .golangci.yml

# runs golangci-lint
FROM base AS lint-golangci-lint-client
WORKDIR /src/client
COPY client/.golangci.yml .
ENV GOGC=50
RUN golangci-lint config verify --config .golangci.yml
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/root/.cache/golangci-lint --mount=type=cache,target=/go/pkg golangci-lint run --config .golangci.yml

# runs govulncheck
FROM base AS lint-govulncheck
WORKDIR /src
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg govulncheck ./...

# runs govulncheck
FROM base AS lint-govulncheck-client
WORKDIR /src/client
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg govulncheck ./...

# runs unit-tests with race detector
FROM base AS unit-tests-client-race
WORKDIR /src/client
ARG TESTPKGS
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg --mount=type=cache,target=/tmp CGO_ENABLED=1 go test -v -race -count 1 ${TESTPKGS}

# runs unit-tests
FROM base AS unit-tests-client-run
WORKDIR /src/client
ARG TESTPKGS
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg --mount=type=cache,target=/tmp go test -v -covermode=atomic -coverprofile=coverage.txt -coverpkg=${TESTPKGS} -count 1 ${TESTPKGS}

# runs unit-tests with race detector
FROM base AS unit-tests-race
WORKDIR /src
ARG TESTPKGS
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg --mount=type=cache,target=/tmp CGO_ENABLED=1 go test -v -race -count 1 ${TESTPKGS}

# runs unit-tests
FROM base AS unit-tests-run
WORKDIR /src
ARG TESTPKGS
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg --mount=type=cache,target=/tmp go test -v -covermode=atomic -coverprofile=coverage.txt -coverpkg=${TESTPKGS} -count 1 ${TESTPKGS}

# cleaned up specs and compiled versions
FROM scratch AS generate
COPY --from=proto-compile /client/api/ /client/api/
COPY --from=go-generate-0 /src/frontend frontend
COPY --from=embed-abbrev-generate /src/internal/version internal/version

FROM scratch AS unit-tests-client
COPY --from=unit-tests-client-run /src/client/coverage.txt /coverage-unit-tests-client.txt

FROM scratch AS unit-tests
COPY --from=unit-tests-run /src/coverage.txt /coverage-unit-tests.txt

# builds integration-test-linux-amd64
FROM base AS integration-test-linux-amd64-build
COPY --from=generate / /
COPY --from=embed-generate / /
WORKDIR /src/cmd/integration-test
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
ARG VERSION_PKG="internal/version"
ARG SHA
ARG TAG
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg GOARCH=amd64 GOOS=linux go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS} -X ${VERSION_PKG}.Name=integration-test -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /integration-test-linux-amd64

# builds omni-darwin-amd64
FROM base AS omni-darwin-amd64-build
COPY --from=generate / /
COPY --from=embed-generate / /
WORKDIR /src/cmd/omni
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
ARG VERSION_PKG="internal/version"
ARG SHA
ARG TAG
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg GOARCH=amd64 GOOS=darwin go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS} -X ${VERSION_PKG}.Name=omni -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /omni-darwin-amd64

# builds omni-darwin-arm64
FROM base AS omni-darwin-arm64-build
COPY --from=generate / /
COPY --from=embed-generate / /
WORKDIR /src/cmd/omni
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
ARG VERSION_PKG="internal/version"
ARG SHA
ARG TAG
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg GOARCH=arm64 GOOS=darwin go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS} -X ${VERSION_PKG}.Name=omni -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /omni-darwin-arm64

# builds omni-linux-amd64
FROM base AS omni-linux-amd64-build
COPY --from=generate / /
COPY --from=embed-generate / /
WORKDIR /src/cmd/omni
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
ARG VERSION_PKG="internal/version"
ARG SHA
ARG TAG
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg GOARCH=amd64 GOOS=linux go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS} -X ${VERSION_PKG}.Name=omni -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /omni-linux-amd64

# builds omni-linux-arm64
FROM base AS omni-linux-arm64-build
COPY --from=generate / /
COPY --from=embed-generate / /
WORKDIR /src/cmd/omni
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
ARG VERSION_PKG="internal/version"
ARG SHA
ARG TAG
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg GOARCH=arm64 GOOS=linux go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS} -X ${VERSION_PKG}.Name=omni -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /omni-linux-arm64

# builds omnictl-darwin-amd64
FROM base AS omnictl-darwin-amd64-build
COPY --from=generate / /
COPY --from=embed-generate / /
WORKDIR /src/cmd/omnictl
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
ARG VERSION_PKG="internal/version"
ARG SHA
ARG TAG
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg GOARCH=amd64 GOOS=darwin go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS} -X ${VERSION_PKG}.Name=omnictl -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /omnictl-darwin-amd64

# builds omnictl-darwin-arm64
FROM base AS omnictl-darwin-arm64-build
COPY --from=generate / /
COPY --from=embed-generate / /
WORKDIR /src/cmd/omnictl
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
ARG VERSION_PKG="internal/version"
ARG SHA
ARG TAG
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg GOARCH=arm64 GOOS=darwin go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS} -X ${VERSION_PKG}.Name=omnictl -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /omnictl-darwin-arm64

# builds omnictl-linux-amd64
FROM base AS omnictl-linux-amd64-build
COPY --from=generate / /
COPY --from=embed-generate / /
WORKDIR /src/cmd/omnictl
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
ARG VERSION_PKG="internal/version"
ARG SHA
ARG TAG
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg GOARCH=amd64 GOOS=linux go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS} -X ${VERSION_PKG}.Name=omnictl -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /omnictl-linux-amd64

# builds omnictl-linux-arm64
FROM base AS omnictl-linux-arm64-build
COPY --from=generate / /
COPY --from=embed-generate / /
WORKDIR /src/cmd/omnictl
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
ARG VERSION_PKG="internal/version"
ARG SHA
ARG TAG
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg GOARCH=arm64 GOOS=linux go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS} -X ${VERSION_PKG}.Name=omnictl -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /omnictl-linux-arm64

# builds omnictl-windows-amd64.exe
FROM base AS omnictl-windows-amd64.exe-build
COPY --from=generate / /
COPY --from=embed-generate / /
WORKDIR /src/cmd/omnictl
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
ARG VERSION_PKG="internal/version"
ARG SHA
ARG TAG
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg GOARCH=amd64 GOOS=windows go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS} -X ${VERSION_PKG}.Name=omnictl -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /omnictl-windows-amd64.exe

FROM scratch AS integration-test-linux-amd64
COPY --from=integration-test-linux-amd64-build /integration-test-linux-amd64 /integration-test-linux-amd64

FROM scratch AS omni-darwin-amd64
COPY --from=omni-darwin-amd64-build /omni-darwin-amd64 /omni-darwin-amd64

FROM scratch AS omni-darwin-arm64
COPY --from=omni-darwin-arm64-build /omni-darwin-arm64 /omni-darwin-arm64

FROM scratch AS omni-linux-amd64
COPY --from=omni-linux-amd64-build /omni-linux-amd64 /omni-linux-amd64

FROM scratch AS omni-linux-arm64
COPY --from=omni-linux-arm64-build /omni-linux-arm64 /omni-linux-arm64

FROM scratch AS omnictl-darwin-amd64
COPY --from=omnictl-darwin-amd64-build /omnictl-darwin-amd64 /omnictl-darwin-amd64

FROM scratch AS omnictl-darwin-arm64
COPY --from=omnictl-darwin-arm64-build /omnictl-darwin-arm64 /omnictl-darwin-arm64

FROM scratch AS omnictl-linux-amd64
COPY --from=omnictl-linux-amd64-build /omnictl-linux-amd64 /omnictl-linux-amd64

FROM scratch AS omnictl-linux-arm64
COPY --from=omnictl-linux-arm64-build /omnictl-linux-arm64 /omnictl-linux-arm64

FROM scratch AS omnictl-windows-amd64.exe
COPY --from=omnictl-windows-amd64.exe-build /omnictl-windows-amd64.exe /omnictl-windows-amd64.exe

FROM integration-test-linux-${TARGETARCH} AS integration-test

FROM scratch AS integration-test-all
COPY --from=integration-test-linux-amd64 / /

FROM omni-linux-${TARGETARCH} AS omni

FROM scratch AS omni-all
COPY --from=omni-darwin-amd64 / /
COPY --from=omni-darwin-arm64 / /
COPY --from=omni-linux-amd64 / /
COPY --from=omni-linux-arm64 / /

FROM omnictl-linux-${TARGETARCH} AS omnictl

FROM scratch AS omnictl-all
COPY --from=omnictl-darwin-amd64 / /
COPY --from=omnictl-darwin-arm64 / /
COPY --from=omnictl-linux-amd64 / /
COPY --from=omnictl-linux-arm64 / /
COPY --from=omnictl-windows-amd64.exe / /

FROM scratch AS image-omni
ARG TARGETARCH
COPY --from=omni omni-linux-${TARGETARCH} /omni
COPY --from=omni omni-linux-${TARGETARCH} /omni
COPY --from=image-fhs / /
COPY --from=image-ca-certificates / /
COPY --from=omnictl-all / /omnictl/
LABEL org.opencontainers.image.source=https://github.com/siderolabs/omni
ENTRYPOINT ["/omni"]

