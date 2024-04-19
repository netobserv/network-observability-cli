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

MANIFEST_FILE="flow-capture.yml"
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
    manifest="${MANIFEST_OUTPUT_PATH}/${MANIFEST_FILE}"
    echo "${flowAgentYAML}" > ${manifest}
    options="$*"
    # Iterate through the command-line arguments
    for option in $options; do
        key="${option%%=*}"
        value="${option#*=}"
        case "$key" in
            --help) # show help
                usage
                exit
                ;;
            --interfaces) # Interfaces
                edit_manifest "interfaces" "$value" "$manifest"
                ;;
            --enable_pktdrop) # Enable packet drop
                if [[ "$value" == "true" || "$value" == "false" ]]; then
                  edit_manifest "pktdrop_enable" "$value" "$manifest"
                else
                  echo "invalid value for --enable_pktdrop"
                fi
                ;;
            --enable_dns) # Enable DNS
                if [[ "$value" == "true" || "$value" == "false" ]]; then
                  edit_manifest "dns_enable" "$value" "$manifest"
                else
                  echo "invalid value for --enable_dns"
                fi
                ;;
            --enable_rtt) # Enable RTT
                if [[ "$value" == "true" || "$value" == "false" ]]; then
                  edit_manifest "rtt_enable" "$value" "$manifest"
                else
                  echo "invalid value for --enable_rtt"
                fi
                ;;
            --enable_filter) # Enable flow filter
                if [[ "$value" == "true" || "$value" == "false" ]]; then
                  edit_manifest "filter_enable" "$value" "$manifest"
                else
                  echo "invalid value for --enable_filter"
                fi
                ;;
            --direction) # Configure flow filter direction
                if [[ "$value" == "Ingress" || "$value" == "Egress" ]]; then
                  edit_manifest "filter_direction" "$value" "$manifest"
                else
                  echo "invalid value for --direction"
                fi
                ;;
            --cidr) # Configure flow filter CIDR
                edit_manifest "filter_cidr" "$value" "$manifest"
                ;;
            --protocol) # Configure flow filter protocol
                if [[ "$value" == "TCP" || "$value" == "UDP" || "$value" == "SCTP" || "$value" == "ICMP" || "$value" == "ICMPv6" ]]; then
                  edit_manifest "filter_protocol" "$value" "$manifest"
                else
                  echo "invalid value for --protocol"
                fi
                ;;
            --sport) # Configure flow filter source port
                edit_manifest "filter_sport" "$value" "$manifest"
                ;;
            --dport) # Configure flow filter destination port
                edit_manifest "filter_dport" "$value" "$manifest"
                ;;
            --port) # Configure flow filter port
                edit_manifest "filter_port" "$value" "$manifest"
                ;;
            --sport_range) # Configure flow filter source port range
                edit_manifest "filter_sport_range" "$value" "$manifest"
                ;;
            --dport_range) # Configure flow filter destination port range
                edit_manifest "filter_dport_range" "$value" "$manifest"
                ;;
            --port_range) # Configure flow filter port range
                edit_manifest "filter_port_range" "$value" "$manifest"
                ;;
            --icmp_type) # ICMP type
                edit_manifest "filter_icmp_type" "$value" "$manifest"
                ;;
            --icmp_code) # ICMP code
                edit_manifest "filter_icmp_code" "$value" "$manifest"
                ;;
            --peer_ip) # Peer IP
                edit_manifest "filter_peer_ip" "$value" "$manifest"
                ;;
            --action) # Filter action
                if [[ "$value" == "Accept" || "$value" == "Reject" ]]; then
                  edit_manifest "filter_action" "$value" "$manifest"
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

    ${K8S_CLI_BIN} apply -f "${manifest}"
    ${K8S_CLI_BIN} rollout status daemonset netobserv-cli -n netobserv-cli --timeout 60s
    rm -rf ${MANIFEST_OUTPUT_PATH}
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

function usage {
  echo "Usage: [OPTIONS]"
  echo "Options:"
  echo "  --help: show this help message"
  echo "  --interfaces: interfaces to monitor"
  echo "  --enable_pktdrop: enable packet drop (default: true)"
  echo "  --enable_dns: enable DNS tracking (default: true)"
  echo "  --enable_rtt: enable RTT tracking (default: true)"
  echo "  --enable_filter: enable flow filter (default: false)"
  echo "  --direction: flow filter direction"
  echo "  --cidr: flow filter CIDR (default: 0.0.0.0/0)"
  echo "  --protocol: flow filter protocol (default: TCP)"
  echo "  --sport: flow filter source port"
  echo "  --dport: flow filter destination port"
  echo "  --port: flow filter port"
  echo "  --sport_range: flow filter source port range"
  echo "  --dport_range: flow filter destination port range"
  echo "  --port_range: flow filter port range"
  echo "  --icmp_type: ICMP type"
  echo "  --icmp_code: ICMP code"
  echo "  --peer_ip: peer IP"
  echo "  --action: flow filter action (default: Accept)"
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
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"FLOW_FILTER_DIRECTION\").value|=\"$2\"" "$3"
    ;;
  "filter_cidr")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"FLOW_FILTER_IP_CIDR\").value|=\"$2\"" "$3"
    ;;
  "filter_protocol")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"FLOW_FILTER_PROTOCOL\").value|=\"$2\"" "$3"
    ;;
  "filter_sport")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"FLOW_FILTER_SOURCE_PORT\").value|=\"$2\"" "$3"
    ;;
  "filter_dport")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"FLOW_FILTER_DESTINATION_PORT\").value|=\"$2\"" "$3"
    ;;
  "filter_port")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"FLOW_FILTER_PORT\").value|=\"$2\"" "$3"
    ;;
  "filter_sport_range")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"FLOW_FILTER_SOURCE_PORT_RANGE\").value|=\"$2\"" "$3"
    ;;
  "filter_dport_range")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"FLOW_FILTER_DESTINATION_PORT_RANGE\").value|=\"$2\"" "$3"
    ;;
  "filter_port_range")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"FLOW_FILTER_PORT_RANGE\").value|=\"$2\"" "$3"
    ;;
  "filter_icmp_type")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"FLOW_FILTER_ICMP_TYPE\").value|=\"$2\"" "$3"
    ;;
  "filter_icmp_code")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"FLOW_FILTER_ICMP_CODE\").value|=\"$2\"" "$3"
    ;;
  "filter_peer_ip")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"FLOW_FILTER_PEER_IP\").value|=\"$2\"" "$3"
    ;;
  "filter_action")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"FLOW_FILTER_ACTION\").value|=\"$2\"" "$3"
    ;;
  esac
}
