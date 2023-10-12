# Build the manager binary
ARG BUILDER_GOLANG_VERSION
# First stage: build the executable.
FROM --platform=$TARGETPLATFORM gcr.io/spectro-images-public/golang:${BUILDER_GOLANG_VERSION}-alpine as toolchain

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
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -ldflags "${LDFLAGS} -extldflags '-static'" -a -o manager main.go

RUN  --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.local/share/golang \
    if [ ${CRYPTO_LIB} ];\
     then \
        GOARCH=${ARCH} go-build-fips.sh -a -o manager . ;\
     else \
        GOARCH=${ARCH} go-build-static.sh -a -o manager . ;\
     fi

RUN if [ "${CRYPTO_LIB}" ]; then assert-static.sh manager; fi
RUN if [ "${CRYPTO_LIB}" ]; then assert-fips.sh manager; fi
RUN scan-govulncheck.sh manager


# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
#FROM gcr.io/distroless/static:latest
FROM alpine:3.18
RUN rm /usr/lib/engines-3/padlock.so
RUN rm /lib/libcrypto.so.3
RUN rm /usr/lib/ossl-modules/legacy.so
RUN addgroup -S spectro
RUN adduser -S -D -h / spectro spectro
USER spectro
WORKDIR /
COPY --from=builder /workspace/manager .
ENTRYPOINT ["/manager"]
