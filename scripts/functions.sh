#!/usr/bin/env bash
set -eu

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

function setup {
  echo "Setting up... "

  # check for mandatory arguments
  if ! [[ $1 =~ flows|packets ]]; then
    echo "invalid setup argument"
    return
  fi

  # check if cluster is reachable
  if ! output=$(oc whoami 2>&1); then
    printf 'You must be connected using oc login command first\n' >&2
    exit 1
  fi

  # load yaml files
  loadYAMLs

  # apply yamls
  echo "creating netobserv-cli namespace"
  echo "$namespaceYAML" | oc apply -f -

  echo "creating service account"
  echo "$saYAML" | oc apply -f -

  echo "creating collector service"
  echo "$collectorServiceYAML" | oc apply -f -

  if [ "$1" = "flows" ]; then
    echo "creating flow-capture agents"
    echo "${flowAgentYAML/"{{FLOW_FILTER_VALUE}}"/${2:-}}" | oc apply -f -
    oc rollout status daemonset netobserv-cli -n netobserv-cli --timeout 60s
  elif [ "$1" = "packets" ]; then
    echo "creating packet-capture agents"
    echo "${packetAgentYAML/"{{PCA_FILTER_VALUE}}"/${2:-}}" | oc apply -f -
    oc rollout status daemonset netobserv-cli -n netobserv-cli --timeout 60s
  fi
}

function cleanup {
  # shellcheck disable=SC2034
  if output=$(oc whoami 2>&1); then
    echo "Copying collector output files..."
    mkdir -p ./output
    oc cp -n netobserv-cli collector:output ./output

    printf "\nCleaning up... "
    oc delete namespace netobserv-cli
  else
    echo "Cleanup namespace skipped"
    return
  fi
}
