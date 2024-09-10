#!/usr/bin/env bash

echo "Updating container file"

: "${COMMIT:=$(git rev-list --abbrev-commit --tags --max-count=1)}"
: "${CONTAINER_FILE:=./Dockerfile}"




cat <<EOF >>"${CONTAINER_FILE}"
LABEL com.redhat.component="network-observability-cli-container"
LABEL name="network-observability-cli"
LABEL io.k8s.display-name="Network Observability CLI"
LABEL io.k8s.description="Network Observability CLI"
LABEL summary="Network Observability CLI"
LABEL maintainer="support@redhat.com"
LABEL io.openshift.tags="network-observability-cli"
LABEL upstream-vcs-ref="${COMMIT}"
LABEL upstream-vcs-type="git"
EOF

echo "Updating container file"
