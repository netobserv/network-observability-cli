# We do not use --platform feature to auto fill this ARG because of incompatibility between podman and docker
ARG TARGETPLATFORM=linux/amd64
ARG BUILDPLATFORM=linux/amd64
ARG TARGETARCH=amd64

# Build the manager binary
FROM --platform=$BUILDPLATFORM docker.io/library/golang:1.21 as builder

ARG TARGETARCH
ARG TARGETPLATFORM
ARG VERSION="unknown"

WORKDIR /opt/app-root

COPY cmd cmd
COPY main.go main.go
COPY go.mod go.mod
COPY go.sum go.sum
COPY vendor/ vendor/
COPY Makefile Makefile
COPY .mk/ .mk/

# Build
RUN GOARCH=$TARGETARCH make compile

# Create final image from ubi + built binary
FROM --platform=$TARGETPLATFORM registry.access.redhat.com/ubi9/ubi:9.3
WORKDIR /
COPY --from=builder /opt/app-root/build .
RUN mkdir output
RUN chown 65532 output
USER 65532:65532

ENTRYPOINT ["/network-observability-cli"]