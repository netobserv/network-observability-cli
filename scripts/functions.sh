#!/usr/bin/env bash

set -eu

# e2e skips inputs
if [ -z "${isE2E+x}" ]; then isE2E=false; fi
# keep capture state
if [ -z "${captureStarted+x}" ]; then captureStarted=false; fi

# get either oc (favorite) or kubectl paths
# this is used only when calling commands directly
# else it will be overridden by inject.sh
K8S_CLI_BIN_PATH=$( which oc 2>/dev/null || which kubectl 2>/dev/null )
K8S_CLI_BIN=$( basename "${K8S_CLI_BIN_PATH}" )

# eBPF agent image to use
agentImg="quay.io/netobserv/netobserv-ebpf-agent:main"

if [ -n "$NETOBSERV_AGENT_IMAGE" ]; then
  echo "using custom agent image $NETOBSERV_AGENT_IMAGE"
  agentImg="$NETOBSERV_AGENT_IMAGE"
fi

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
  flowAgentYAML="${flowAgentYAML/"{{AGENT_IMAGE_URL}}"/${agentImg}}"

  packetAgentYAML='
    packetAgentYAMLContent
  '
  if [ -f ./res/packet-capture.yml ]; then
    packetAgentYAML="$(cat ./res/packet-capture.yml)"
  fi
  packetAgentYAML="${packetAgentYAML/"{{AGENT_IMAGE_URL}}"/${agentImg}}"

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

FLOWS_MANIFEST_FILE="flow-capture.yml"
PACKETS_MANIFEST_FILE="packet-capture.yml"
MANIFEST_OUTPUT_PATH="tmp"

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
    shift
    echo "creating flow-capture agents:"
    if [[ ! -d ${MANIFEST_OUTPUT_PATH} ]]; then
      mkdir -p ${MANIFEST_OUTPUT_PATH} > /dev/null
    fi
    manifest="${MANIFEST_OUTPUT_PATH}/${FLOWS_MANIFEST_FILE}"
    echo "${flowAgentYAML}" > ${manifest}
    options="$*"
    check_args_and_apply "$options" "$manifest" "$1"
  elif [ "$1" = "packets" ]; then
    shift
    echo "creating packet-capture agents"
    if [[ ! -d ${MANIFEST_OUTPUT_PATH} ]]; then
      mkdir -p ${MANIFEST_OUTPUT_PATH} > /dev/null
    fi
    manifest="${MANIFEST_OUTPUT_PATH}/${PACKETS_MANIFEST_FILE}"
    echo "${packetAgentYAML}" > ${manifest}
    options="$*"
    check_args_and_apply "$options" "$manifest" "$1"
  fi
}

function copyOutput {
  echo "Copying collector output files..."
  mkdir -p ./output
  ${K8S_CLI_BIN} cp -n netobserv-cli collector:output ./output
}

function cleanup {
  # shellcheck disable=SC2034
  if clusterIsReady; then
    if [ "$isE2E" = true ]; then
      copyOutput
    elif [ "$captureStarted" = true ]; then
      while true; do
          read -rp "Copy the capture output locally ?" yn
          case $yn in
              [Yy]* ) copyOutput; break;;
              [Nn]* ) echo "copy skipped"; break;;
              * ) echo "Please answer yes or no.";;
          esac
      done
    fi

    printf "\nCleaning up... "
    ${K8S_CLI_BIN} delete namespace netobserv-cli
  else
    echo "Cleanup namespace skipped"
    return
  fi
}

function flows_usage {
  echo "        Options:"
  echo "          --interfaces: interfaces to monitor"
  echo "          --enable_pktdrop: enable packet drop (default: false)"
  echo "          --enable_dns: enable DNS tracking (default: false)"
  echo "          --enable_rtt: enable RTT tracking (default: false)"
  echo "          --enable_filter: enable flow filter (default: false)"
  echo "          --direction: flow filter direction"
  echo "          --cidr: flow filter CIDR (default: 0.0.0.0/0)"
  echo "          --protocol: flow filter protocol"
  echo "          --sport: flow filter source port"
  echo "          --dport: flow filter destination port"
  echo "          --port: flow filter port"
  echo "          --sport_range: flow filter source port range"
  echo "          --dport_range: flow filter destination port range"
  echo "          --port_range: flow filter port range"
  echo "          --icmp_type: ICMP type"
  echo "          --icmp_code: ICMP code"
  echo "          --peer_ip: peer IP"
  echo "          --action: flow filter action (default: Accept)"
}

function packets_usage {
  echo "        Options:"
  echo "          --direction: flow filter direction"
  echo "          --cidr: flow filter CIDR (default: 0.0.0.0/0)"
  echo "          --protocol: flow filter protocol"
  echo "          --sport: flow filter source port"
  echo "          --dport: flow filter destination port"
  echo "          --port: flow filter port"
  echo "          --sport_range: flow filter source port range"
  echo "          --dport_range: flow filter destination port range"
  echo "          --port_range: flow filter port range"
  echo "          --icmp_type: ICMP type"
  echo "          --icmp_code: ICMP code"
  echo "          --peer_ip: peer IP"
  echo "          --action: flow filter action (default: Accept)"
}

function edit_manifest() {
  ## replace the env variable in the manifest file
  echo "env: $1, env_value: $2"
  case "$1" in
  "interfaces")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"INTERFACES\").value|=\"$2\"" "$3"
    ;;
  "pktdrop_enable")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"ENABLE_PKT_DROPS\").value|=\"$2\"" "$3"
    ;;
  "dns_enable")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"ENABLE_DNS_TRACKING\").value|=\"$2\"" "$3"
    ;;
  "rtt_enable")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"ENABLE_RTT\").value|=\"$2\"" "$3"
    ;;
  "filter_enable")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"ENABLE_FLOW_FILTER\").value|=\"$2\"" "$3"
    ;;
  "filter_direction")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"FILTER_DIRECTION\").value|=\"$2\"" "$3"
    ;;
  "filter_cidr")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"FILTER_IP_CIDR\").value|=\"$2\"" "$3"
    ;;
  "filter_protocol")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"FILTER_PROTOCOL\").value|=\"$2\"" "$3"
    ;;
  "filter_sport")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"FILTER_SOURCE_PORT\").value|=\"$2\"" "$3"
    ;;
  "filter_dport")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"FILTER_DESTINATION_PORT\").value|=\"$2\"" "$3"
    ;;
  "filter_port")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"FILTER_PORT\").value|=\"$2\"" "$3"
    ;;
  "filter_sport_range")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"FILTER_SOURCE_PORT_RANGE\").value|=\"$2\"" "$3"
    ;;
  "filter_dport_range")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"FILTER_DESTINATION_PORT_RANGE\").value|=\"$2\"" "$3"
    ;;
  "filter_port_range")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"FILTER_PORT_RANGE\").value|=\"$2\"" "$3"
    ;;
  "filter_icmp_type")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"FILTER_ICMP_TYPE\").value|=\"$2\"" "$3"
    ;;
  "filter_icmp_code")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"FILTER_ICMP_CODE\").value|=\"$2\"" "$3"
    ;;
  "filter_peer_ip")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"FILTER_PEER_IP\").value|=\"$2\"" "$3"
    ;;
  "filter_action")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"FILTER_ACTION\").value|=\"$2\"" "$3"
    ;;
  esac
}

# Check if the arguments are valid
#$1: options
#$2: manifest
#$3: either flows or packets
function check_args_and_apply() {
    # Iterate through the command-line arguments
    for option in $1; do
        key="${option%%=*}"
        value="${option#*=}"
        case "$key" in
            --interfaces) # Interfaces
                edit_manifest "interfaces" "$value" "$2"
                ;;
            --enable_pktdrop) # Enable packet drop
                if [[ "$3" == "flows" ]]; then
                  if [[ "$value" == "true" || "$value" == "false" ]]; then
                    edit_manifest "pktdrop_enable" "$value" "$2"
                  else
                    echo "invalid value for --enable_pktdrop"
                  fi
                else
                  echo "--enable_pktdrop is invalid option for packets"
                  exit 1
                fi
                ;;
            --enable_dns) # Enable DNS
                if [[ "$3" == "flows" ]]; then
                  if [[ "$value" == "true" || "$value" == "false" ]]; then
                    edit_manifest "dns_enable" "$value" "$2"
                  else
                    echo "invalid value for --enable_dns"
                  fi
                else
                  echo "--enable_dns is invalid option for packets"
                  exit 1
                fi
                ;;
            --enable_rtt) # Enable RTT
                if [[ "$3" == "flows" ]]; then
                  if [[ "$value" == "true" || "$value" == "false" ]]; then
                    edit_manifest "rtt_enable" "$value" "$2"
                  else
                    echo "invalid value for --enable_rtt"
                  fi
                else
                  echo "--enable_rtt is invalid option for packets"
                  exit 1
                fi
                ;;
            --enable_filter) # Enable flow filter
                if [[ "$3" == "flows" ]]; then
                  if [[ "$value" == "true" || "$value" == "false" ]]; then
                    edit_manifest "filter_enable" "$value" "$2"
                  else
                    echo "invalid value for --enable_filter"
                  fi
                else
                  echo "--enable_filter is invalid option for packets"
                  exit 1
                fi
                ;;
            --direction) # Configure filter direction
                if [[ "$value" == "Ingress" || "$value" == "Egress" ]]; then
                  edit_manifest "filter_direction" "$value" "$2"
                else
                  echo "invalid value for --direction"
                fi
                ;;
            --cidr) # Configure flow CIDR
                edit_manifest "filter_cidr" "$value" "$2"
                ;;
            --protocol) # Configure filter protocol
                if [[ "$value" == "TCP" || "$value" == "UDP" || "$value" == "SCTP" || "$value" == "ICMP" || "$value" == "ICMPv6" ]]; then
                  edit_manifest "filter_protocol" "$value" "$2"
                else
                  echo "invalid value for --protocol"
                fi
                ;;
            --sport) # Configure filter source port
                edit_manifest "filter_sport" "$value" "$2"
                ;;
            --dport) # Configure filter destination port
                edit_manifest "filter_dport" "$value" "$2"
                ;;
            --port) # Configure filter port
                edit_manifest "filter_port" "$value" "$2"
                ;;
            --sport_range) # Configure filter source port range
                edit_manifest "filter_sport_range" "$value" "$2"
                ;;
            --dport_range) # Configure filter destination port range
                edit_manifest "filter_dport_range" "$value" "$2"
                ;;
            --port_range) # Configure filter port range
                edit_manifest "filter_port_range" "$value" "$2"
                ;;
            --icmp_type) # ICMP type
                edit_manifest "filter_icmp_type" "$value" "$2"
                ;;
            --icmp_code) # ICMP code
                edit_manifest "filter_icmp_code" "$value" "$2"
                ;;
            --peer_ip) # Peer IP
                edit_manifest "filter_peer_ip" "$value" "$2"
                ;;
            --action) # Filter action
                if [[ "$value" == "Accept" || "$value" == "Reject" ]]; then
                  edit_manifest "filter_action" "$value" "$2"
                else
                  echo "invalid value for --action"
                fi
                ;;
            *) # Invalid option
                echo "Invalid option: $key" >&2
                exit 1
                ;;
        esac
    done

    ${K8S_CLI_BIN} apply -f "$2"
    ${K8S_CLI_BIN} rollout status daemonset netobserv-cli -n netobserv-cli --timeout 60s
    rm -rf ${MANIFEST_OUTPUT_PATH}
}
