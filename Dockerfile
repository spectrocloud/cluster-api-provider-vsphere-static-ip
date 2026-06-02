# Build the manager binary
ARG BUILDER_GOLANG_VERSION
# First stage: build the executable.
FROM --platform=$TARGETPLATFORM us-central1-docker.pkg.dev/palette-images-dev/hardened-images/builder/golang:${BUILDER_GOLANG_VERSION}-alpine as toolchain

# FIPS
ARG CRYPTO_LIB
ENV GOEXPERIMENT=${CRYPTO_LIB:+boringcrypto}

FROM toolchain as builder
WORKDIR /workspace
RUN apk update
RUN apk add git gcc g++ curl

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN  --mount=type=cache,target=/root/.local/share/golang \
     --mount=type=cache,target=/go/pkg/mod \
     go mod download

# Copy the go source
COPY . .

# Build
RUN  --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.local/share/golang \
    if [ ${CRYPTO_LIB} ];\
     then \
        GOARCH=${ARCH} go-build-fips.sh -a -o manager github.com/spectrocloud/cluster-api-provider-vsphere-static-ip ;\
     else \
        GOARCH=${ARCH} go-build-static.sh -a -o manager github.com/spectrocloud/cluster-api-provider-vsphere-static-ip ;\
     fi

RUN if [ "${CRYPTO_LIB}" ]; then assert-static.sh manager; fi
RUN if [ "${CRYPTO_LIB}" ]; then assert-fips.sh manager; fi
#RUN scan-govulncheck.sh manager


# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532
ENTRYPOINT ["/manager"]
