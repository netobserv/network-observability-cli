# We do not use --platform feature to auto fill this ARG because of incompatibility between podman and docker
ARG TARGETARCH=amd64

# Build the manager binary
FROM docker.io/library/golang:1.22 as builder

ARG TARGETARCH
ARG LDFLAGS

WORKDIR /opt/app-root

COPY cmd cmd
COPY main.go main.go
COPY go.mod go.mod
COPY go.sum go.sum
COPY vendor/ vendor/

# Build collector
RUN GOARCH=$TARGETARCH go build -ldflags "$LDFLAGS" -mod vendor -a -o build/network-observability-cli

# We still need Makefile & resources for oc-commands; copy them after go build for caching
COPY commands/ commands/
COPY res/ res/
COPY scripts/ scripts/
COPY Makefile Makefile
COPY .mk/ .mk/

# Install oc to allow collector to run commands
RUN set -x; \
    OC_TAR_URL="https://mirror.openshift.com/pub/openshift-v4/$(uname -m)/clients/ocp/latest/openshift-client-linux.tar.gz" && \
    curl -L -q -o /tmp/oc.tar.gz "$OC_TAR_URL" && \
    tar -C /tmp -xvf /tmp/oc.tar.gz oc kubectl

# Embedd commands in case users want to pull it from collector image
RUN USER=netobserv VERSION=main make oc-commands

# Prepare output dir
RUN mkdir -p output

# Create final image from ubi + built binary and command
FROM --platform=linux/$TARGETARCH registry.access.redhat.com/ubi9/ubi:9.4
WORKDIR /

COPY --from=builder /opt/app-root/build .
COPY --from=builder /tmp/oc /usr/bin/oc
COPY --from=builder /tmp/kubectl /usr/bin/kubectl
COPY --from=builder --chown=65532:65532 /opt/app-root/output /output
USER 65532:65532

ENTRYPOINT ["/network-observability-cli"]
