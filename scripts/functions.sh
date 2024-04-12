#!/usr/bin/env bash
set -eu

# get either oc (favorite) or kubectl paths
# this is used only when calling commands directly
# else it will be overridden by inject.sh
K8S_CLI_BIN_PATH=$( which oc 2>/dev/null || which kubectl 2>/dev/null )
K8S_CLI_BIN=$( basename "${K8S_CLI_BIN_PATH}" )

function loadYAMLs() {
  namespaceYAML='
    namespaceYAMLContent
  '
  if [ -f ./res/namespace.yml ]; then
    namespaceYAML="$(cat ./res/namespace.yml)"
  fi

  saYAML='
    saYAMLContent
  '
  if [ -f ./res/service-account.yml ]; then
    saYAML="$(cat ./res/service-account.yml)"
  fi

  flowAgentYAML='
    flowAgentYAMLContent
  '
  if [ -f ./res/flow-capture.yml ]; then
    flowAgentYAML="$(cat ./res/flow-capture.yml)"
  fi

  packetAgentYAML='
    packetAgentYAMLContent
  '
  if [ -f ./res/packet-capture.yml ]; then
    packetAgentYAML="$(cat ./res/packet-capture.yml)"
  fi

  collectorServiceYAML='
    collectorServiceYAMLContent
  '
  if [ -f ./res/collector-service.yml ]; then
    collectorServiceYAML="$(cat ./res/collector-service.yml)"
  fi
}

function clusterIsReady() {
    # use oc whoami as connectivity check by default and fallback to kubectl get all if needed
    K8S_CLI_CONNECTIVITY="${K8S_CLI_BIN} whoami"
    if [ "${K8S_CLI_BIN}" = "kubectl" ]; then
      K8S_CLI_CONNECTIVITY="${K8S_CLI_BIN} get all"
    fi
    if ${K8S_CLI_CONNECTIVITY} 2>&1 || ${K8S_CLI_BIN} cluster-info | grep -q "Kubernetes control plane"; then
      return 0
    else
      return 1
    fi
}

function setup {
  echo "Setting up... "

  # check for mandatory arguments
  if ! [[ $1 =~ flows|packets ]]; then
    echo "invalid setup argument"
    return
  fi

  if ! clusterIsReady; then
    printf 'You must be connected to cluster\n' >&2
    exit 1
  fi

  # load yaml files
  loadYAMLs

  # apply yamls
  echo "creating netobserv-cli namespace"
  echo "$namespaceYAML" | ${K8S_CLI_BIN} apply -f -

  echo "creating service account"
  echo "$saYAML" | ${K8S_CLI_BIN} apply -f -

  echo "creating collector service"
  echo "$collectorServiceYAML" | ${K8S_CLI_BIN} apply -f -

  if [ "$1" = "flows" ]; then
    echo "creating flow-capture agents"
    echo "${flowAgentYAML/"{{FLOW_FILTER_VALUE}}"/${2:-}}" | ${K8S_CLI_BIN} apply -f -
    ${K8S_CLI_BIN} rollout status daemonset netobserv-cli -n netobserv-cli --timeout 60s
  elif [ "$1" = "packets" ]; then
    echo "creating packet-capture agents"
    echo "${packetAgentYAML/"{{PCA_FILTER_VALUE}}"/${2:-}}" | ${K8S_CLI_BIN} apply -f -
    ${K8S_CLI_BIN} rollout status daemonset netobserv-cli -n netobserv-cli --timeout 60s
  fi
}

function cleanup {
  # shellcheck disable=SC2034
  if clusterIsReady; then
    echo "Copying collector output files..."
    mkdir -p ./output
    ${K8S_CLI_BIN} cp -n netobserv-cli collector:output ./output

    printf "\nCleaning up... "
    ${K8S_CLI_BIN} delete namespace netobserv-cli
  else
    echo "Cleanup namespace skipped"
    return
  fi
}
