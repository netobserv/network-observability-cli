# We do not use --platform feature to auto fill this ARG because of incompatibility between podman and docker
ARG TARGETARCH=amd64

# Create final image from ubi + built binary and command
FROM --platform=linux/$TARGETARCH registry.access.redhat.com/ubi9/ubi-minimal:9.7-1769056855

RUN microdnf install -y tar && \
    microdnf clean all

WORKDIR /

USER 65532:65532
