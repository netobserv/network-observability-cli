#!/usr/bin/env bash

set -e
set +u

# e2e skips inputs
if [ -z "${isE2E+x}" ]; then isE2E=false; fi
# keep capture state
if [ -z "${captureStarted+x}" ]; then captureStarted=false; fi
# prompt copy by default
if [ -z "${copy+x}" ]; then copy="prompt"; fi
# run foreground by default
if [ -z "${runBackground+x}" ]; then runBackground="false"; fi

# force skipping cleanup
skipCleanup=false

# get either oc (favorite) or kubectl paths
# this is used only when calling commands directly
# else it will be overridden by inject.sh
K8S_CLI_BIN_PATH=$(which oc 2>/dev/null || which kubectl 2>/dev/null)
K8S_CLI_BIN=$(basename "${K8S_CLI_BIN_PATH}")

# namespace for this run
namespace="netobserv-cli"

if [ -n "$NETOBSERV_NAMESPACE" ]; then
  echo "using custom namespace $NETOBSERV_NAMESPACE"
  namespace="$NETOBSERV_NAMESPACE"
fi

# collector target host
targetHost="collector.$namespace.svc.cluster.local"

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
  namespaceYAML="${namespaceYAML/"{{NAME}}"/${namespace}}"

  saYAML='
    saYAMLContent
  '
  if [ -f ./res/service-account.yml ]; then
    saYAML="$(cat ./res/service-account.yml)"
  fi
  saYAML="${saYAML//"{{NAMESPACE}}"/${namespace}}"

  flowAgentYAML='
    flowAgentYAMLContent
  '
  if [ -f ./res/flow-capture.yml ]; then
    flowAgentYAML="$(cat ./res/flow-capture.yml)"
  fi
  flowAgentYAML="${flowAgentYAML/"{{NAMESPACE}}"/${namespace}}"
  flowAgentYAML="${flowAgentYAML/"{{TARGET_HOST}}"/${targetHost}}"
  flowAgentYAML="${flowAgentYAML/"{{AGENT_IMAGE_URL}}"/${agentImg}}"

  packetAgentYAML='
    packetAgentYAMLContent
  '
  if [ -f ./res/packet-capture.yml ]; then
    packetAgentYAML="$(cat ./res/packet-capture.yml)"
  fi
  packetAgentYAML="${packetAgentYAML/"{{NAMESPACE}}"/${namespace}}"
  packetAgentYAML="${packetAgentYAML/"{{TARGET_HOST}}"/${targetHost}}"
  packetAgentYAML="${packetAgentYAML/"{{AGENT_IMAGE_URL}}"/${agentImg}}"

  collectorServiceYAML='
    collectorServiceYAMLContent
  '
  if [ -f ./res/collector-service.yml ]; then
    collectorServiceYAML="$(cat ./res/collector-service.yml)"
  fi
  collectorServiceYAML="${collectorServiceYAML/"{{NAMESPACE}}"/${namespace}}"
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

function namespaceFound() {
  # ensure namespace doesn't exist, else we should not override content
  if ${K8S_CLI_BIN} get namespace "$namespace" --ignore-not-found=true | grep -q "$namespace"; then
    return 0
  else
    return 1
  fi
}

FLOWS_MANIFEST_FILE="flow-capture.yml"
PACKETS_MANIFEST_FILE="packet-capture.yml"
CONFIG_JSON_TEMP="config.json"
CLUSTER_CONFIG="cluster-config-v1.yaml"
NETWORK_CONFIG="cluster-network.yaml"
MANIFEST_OUTPUT_PATH="tmp"

function getSubnets() {
  declare -n sn="$1"

  # get cluster-config-v1 Configmap to retreive machine networks
  installConfig=$(${K8S_CLI_BIN} get configmap cluster-config-v1 -n kube-system -o custom-columns=":data.install-config")
  yaml="${MANIFEST_OUTPUT_PATH}/${CLUSTER_CONFIG}"
  echo "$installConfig" >${yaml}

  machines=$(yq e -oj '.networking.machineNetwork[] | select(has("cidr")).cidr' "$yaml")
  if [ "${#machines}" -gt 0 ]; then
    sn["Machines"]=$machines
  fi

  # get OCP cluster Network to retreive pod / services / external networks
  networkConfig=$(${K8S_CLI_BIN} get network cluster -o yaml)
  yaml="${MANIFEST_OUTPUT_PATH}/${NETWORK_CONFIG}"
  echo "$networkConfig" >${yaml}

  pods=$(yq e -oj '.spec.clusterNetwork[] | select(has("cidr")).cidr' "$yaml")
  if [ "${#pods}" -gt 0 ]; then
    sn["Pods"]=$pods
  fi

  services=$(yq e -oj '.spec.serviceNetwork[] | select(.)' "$yaml")
  if [ "${#services}" -gt 0 ]; then
    sn["Services"]=$services
  fi

  if [ "${#sn[@]}" -gt 0 ]; then
    echo "Found subnets:"
    for key in "${!sn[@]}"; do
      echo "    $key: ${sn[$key]}"
    done
  else
    echo "Didn't found subnets"
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

  if namespaceFound; then
    printf "%s namespace already exists. Ensure someone else is not running another capture on this cluster. Else use 'oc netobserv cleanup' to remove the namespace first.\n" "$namespace" >&2
    skipCleanup="true"
    exit 1
  fi

  # load yaml files
  loadYAMLs

  # apply yamls
  echo "creating $namespace namespace"
  echo "$namespaceYAML" | ${K8S_CLI_BIN} apply -f -

  echo "creating service account"
  echo "$saYAML" | ${K8S_CLI_BIN} apply -f -

  echo "creating collector service"
  echo "$collectorServiceYAML" | ${K8S_CLI_BIN} apply -f -

  if [ "$1" = "flows" ]; then
    shift
    echo "creating flow-capture agents:"
    if [[ ! -d ${MANIFEST_OUTPUT_PATH} ]]; then
      mkdir -p ${MANIFEST_OUTPUT_PATH} >/dev/null
    fi
    manifest="${MANIFEST_OUTPUT_PATH}/${FLOWS_MANIFEST_FILE}"
    echo "${flowAgentYAML}" >${manifest}
    options="$*"
    check_args_and_apply "$options" "$manifest" "flows"
  elif [ "$1" = "packets" ]; then
    shift
    echo "creating packet-capture agents"
    if [[ ! -d ${MANIFEST_OUTPUT_PATH} ]]; then
      mkdir -p ${MANIFEST_OUTPUT_PATH} >/dev/null
    fi
    manifest="${MANIFEST_OUTPUT_PATH}/${PACKETS_MANIFEST_FILE}"
    echo "${packetAgentYAML}" >${manifest}
    options="$*"
    check_args_and_apply "$options" "$manifest" "packets"
  fi
}

function follow {
  ${K8S_CLI_BIN} logs collector -n "$namespace" -f
}

function copyOutput {
  echo "Copying collector output files..."
  mkdir -p ./output
  ${K8S_CLI_BIN} cp -n "$namespace" collector:output ./output
}

function deleteDaemonset {
  printf "\nDeleting daemonset... "
  ${K8S_CLI_BIN} delete daemonset netobserv-cli -n "$namespace" --ignore-not-found=true
}

function deletePod {
  printf "\nDeleting pod... "
  ${K8S_CLI_BIN} delete pod collector -n "$namespace" --ignore-not-found=true
}

function deleteNamespace {
  printf "\nDeleting namespace... "
  ${K8S_CLI_BIN} delete namespace "$namespace" --ignore-not-found=true
}

function cleanup {
  if [[ "$runBackground" == "true" || "$skipCleanup" == "true" ]]; then
    return
  fi

  # shellcheck disable=SC2034
  if clusterIsReady; then
    if [ "$captureStarted" = false ]; then
      echo "Copy skipped"
    elif [[ "$isE2E" = true || "$copy" = true ]]; then
      copyOutput
    elif [ "$copy" = "prompt" ]; then
      while true; do
        read -rp "Copy the capture output locally ?" yn
        case $yn in
        [Yy]*)
          copyOutput
          break
          ;;
        [Nn]*)
          echo "copy skipped"
          break
          ;;
        *) echo "Please answer yes or no." ;;
        esac
      done
    fi

    printf "\nCleaning up..."
    deleteDaemonset
    deletePod
    deleteNamespace
    printf "\n"
  else
    echo "Cleanup namespace skipped"
    return
  fi
}

function common_usage {
  # general options
  echo "          --log-level:              components logs                            (default: info)"
  echo "          --max-time:               maximum capture time                       (default: 5m)"
  echo "          --max-bytes:              maximum capture bytes                      (default: 50000000 = 50MB)"
  echo "          --background:             run in background                          (default: false)"
  echo "          --copy:                   copy the output files locally              (default: prompt)"
  # enrichment
  echo "          --get-subnets:            get subnets informations                   (default: false)"
  # filters
  echo "          --node-selector:          capture on specific nodes                  (default: n/a)"
  echo "          --direction:              filter direction                           (default: n/a)"
  echo "          --cidr:                   filter CIDR                                (default: 0.0.0.0/0)"
  echo "          --protocol:               filter protocol                            (default: n/a)"
  echo "          --sport:                  filter source port                         (default: n/a)"
  echo "          --dport:                  filter destination port                    (default: n/a)"
  echo "          --port:                   filter port                                (default: n/a)"
  echo "          --sport_range:            filter source port range                   (default: n/a)"
  echo "          --dport_range:            filter destination port range              (default: n/a)"
  echo "          --port_range:             filter port range                          (default: n/a)"
  echo "          --sports:                 filter on either of two source ports       (default: n/a)"
  echo "          --dports:                 filter on either of two destination ports  (default: n/a)"
  echo "          --ports:                  filter on either of two ports              (default: n/a)"
  echo "          --tcp_flags:              filter TCP flags                           (default: n/a)"
  echo "          --action:                 filter action                              (default: Accept)"
  echo "          --icmp_type:              filter ICMP type                           (default: n/a)"
  echo "          --icmp_code:              filter ICMP code                           (default: n/a)"
  echo "          --peer_ip:                filter peer IP                             (default: n/a)"
  echo "          --drops:                  filter flows with only dropped packets     (default: false)"
  echo "          --regexes:                filter flows using regex                   (default: n/a)"
}

function flows_usage {
  # features
  echo "          --enable_pktdrop:         enable packet drop                         (default: false)"
  echo "          --enable_dns:             enable DNS tracking                        (default: false)"
  echo "          --enable_rtt:             enable RTT tracking                        (default: false)"
  echo "          --enable_network_events:  enable Network events monitoring           (default: false)"
  echo "          --enable_filter:          enable flow filter                         (default: false)"
  # common
  common_usage
  # specific filters
  echo "          --interfaces:             interfaces to monitor                      (default: n/a)"

}

function packets_usage {
  # common
  common_usage
}

# get current config and save it to temp file
function copyFLPConfig {
  jsonContent=$(yq e '.spec.template.spec.containers[0].env[] | select(.name=="FLP_CONFIG").value' "$1")
  # json temp file location is set as soon as this function is called
  json="${MANIFEST_OUTPUT_PATH}/${CONFIG_JSON_TEMP}"
  echo "$jsonContent" >${json}
}

# update FLP Config
function updateFLPConfig {
  # get json as string with escaped quotes
  jsonContent=$(cat "$1")
  jsonContent=${jsonContent//\"/\\\"}

  # update FLP_CONFIG env
  yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"FLP_CONFIG\").value|=\"$jsonContent\"" "$2"
}

function edit_manifest() {
  ## replace the configuration in the manifest file
  echo "opt: $1, evalue: $2"
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
  "network_events_enable")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"ENABLE_NETWORK_EVENTS_MONITORING\").value|=\"$2\"" "$3"
    ;;
  "get_subnets")
    if [[ "$2" == "true" ]]; then
      declare -A subnets
      getSubnets subnets

      if [ "${#subnets[@]}" -gt 0 ]; then
        copyFLPConfig "$3"

        # get network enrich stage
        enrichIndex=$(yq e -oj ".parameters[] | select(.name==\"enrich\") | document_index" "$json")
        enrichContent=$(yq e -oj ".parameters[$enrichIndex]" "$json")
        enrichJson="${MANIFEST_OUTPUT_PATH}/enrich.json"
        echo "$enrichContent" >${enrichJson}

        # add rules to network
        yq e -oj --inplace ".transform.network.rules +={\"type\":\"add_subnet_label\",\"add_subnet_label\":{\"input\":\"SrcAddr\",\"output\":\"SrcSubnetLabel\"}}" "$enrichJson"
        yq e -oj --inplace ".transform.network.rules +={\"type\":\"add_subnet_label\",\"add_subnet_label\":{\"input\":\"DstAddr\",\"output\":\"DstSubnetLabel\"}}" "$enrichJson"

        # add subnetLabels to network
        yq e -oj --inplace ".transform.network.subnetLabels = []" "$enrichJson"
        for key in "${!subnets[@]}"; do
          yq e -oj --inplace ".transform.network.subnetLabels += {\"name\":\"$key\",\"cidrs\":[${subnets[$key]}]}" "$enrichJson"
        done

        # override network
        enrichJsonStr=$(cat $enrichJson)
        yq e -oj --inplace ".parameters[$enrichIndex] = $enrichJsonStr" "$json"

        updateFLPConfig "$json" "$3"
      fi
    fi
    ;;
  "filter_enable")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"ENABLE_FLOW_FILTER\").value|=\"$2\"" "$3"
    ;;
  "filter_direction")
    yq e --inplace " .spec.template.spec.containers[0].env[] |= select(.name == \"FLOW_FILTER_RULES\").value |=(fromjson | map(.direction = \"$2\")| tostring)" "$3"
    ;;
  "filter_cidr")
    yq e --inplace " .spec.template.spec.containers[0].env[] |= select(.name == \"FLOW_FILTER_RULES\").value |=(fromjson | map(.ip_cidr = \"$2\")| tostring)" "$3"
    ;;
  "filter_protocol")
    yq e --inplace " .spec.template.spec.containers[0].env[] |= select(.name == \"FLOW_FILTER_RULES\").value |=(fromjson | map(.protocol = \"$2\")| tostring)" "$3"
    ;;
  "filter_sport")
    yq e --inplace " .spec.template.spec.containers[0].env[] |= select(.name == \"FLOW_FILTER_RULES\").value |=(fromjson | map(.source_port = $2)| tostring)" "$3"
    ;;
  "filter_dport")
    yq e --inplace " .spec.template.spec.containers[0].env[] |= select(.name == \"FLOW_FILTER_RULES\").value |=(fromjson | map(.destination_port = $2)| tostring)" "$3"
    ;;
  "filter_port")
    yq e --inplace " .spec.template.spec.containers[0].env[] |= select(.name == \"FLOW_FILTER_RULES\").value |=(fromjson | map(.port = $2)| tostring)" "$3"
    ;;
  "filter_sport_range")
    yq e --inplace " .spec.template.spec.containers[0].env[] |= select(.name == \"FLOW_FILTER_RULES\").value |=(fromjson | map(.source_port_range = \"$2\")| tostring)" "$3"
    ;;
  "filter_dport_range")
    yq e --inplace " .spec.template.spec.containers[0].env[] |= select(.name == \"FLOW_FILTER_RULES\").value |=(fromjson | map(.destination_port_range = \"$2\")| tostring)" "$3"
    ;;
  "filter_port_range")
    yq e --inplace " .spec.template.spec.containers[0].env[] |= select(.name == \"FLOW_FILTER_RULES\").value |=(fromjson | map(.port_range = \"$2\")| tostring)" "$3"
    ;;
  "filter_sports")
    yq e --inplace " .spec.template.spec.containers[0].env[] |= select(.name == \"FLOW_FILTER_RULES\").value |=(fromjson | map(.source_ports = \"$2\")| tostring)" "$3"
    ;;
  "filter_dports")
    yq e --inplace " .spec.template.spec.containers[0].env[] |= select(.name == \"FLOW_FILTER_RULES\").value |=(fromjson | map(.destination_ports = \"$2\")| tostring)" "$3"
    ;;
  "filter_ports")
    yq e --inplace " .spec.template.spec.containers[0].env[] |= select(.name == \"FLOW_FILTER_RULES\").value |=(fromjson | map(.ports = \"$2\")| tostring)" "$3"
    ;;
  "filter_icmp_type")
    yq e --inplace " .spec.template.spec.containers[0].env[] |= select(.name == \"FLOW_FILTER_RULES\").value |=(fromjson | map(.icmp_type = $2)| tostring)" "$3"
    ;;
  "filter_icmp_code")
    yq e --inplace " .spec.template.spec.containers[0].env[] |= select(.name == \"FLOW_FILTER_RULES\").value |=(fromjson | map(.icmp_code = $2)| tostring)" "$3"
    ;;
  "filter_peer_ip")
    yq e --inplace " .spec.template.spec.containers[0].env[] |= select(.name == \"FLOW_FILTER_RULES\").value |=(fromjson | map(.peer_ip = \"$2\")| tostring)" "$3"
    ;;
  "filter_action")
    yq e --inplace " .spec.template.spec.containers[0].env[] |= select(.name == \"FLOW_FILTER_RULES\").value |=(fromjson | map(.action = \"$2\")| tostring)" "$3"
    ;;
  "filter_tcp_flags")
    yq e --inplace " .spec.template.spec.containers[0].env[] |= select(.name == \"FLOW_FILTER_RULES\").value |=(fromjson | map(.tcp_flags = \"$2\")| tostring)" "$3"
    ;;
  "filter_pkt_drops")
    yq e --inplace " .spec.template.spec.containers[0].env[] |= select(.name == \"FLOW_FILTER_RULES\").value |=(fromjson | map(.drops = $2)| tostring)" "$3"
    ;;
  "filter_regexes")
    copyFLPConfig "$3"

    # remove send step
    yq e -oj --inplace "del(.pipeline[] | select(.name==\"send\"))" "$json"

    # define rules from arg
    IFS=',' read -ra regexes <<<"$2"
    rules=()
    for regex in "${regexes[@]}"; do
      IFS='~' read -ra keyValue <<<"$regex"
      key=${keyValue[0]}
      value=${keyValue[1]}
      echo "key: $key value: $value"
      rules+=("{\"type\":\"keep_entry_if_regex_match\",\"keepEntry\":{\"input\":\"$key\",\"value\":\"$value\"}}")
    done
    rulesStr=$(
      IFS=,
      echo "${rules[*]}"
    )

    # add filter param & pipeline
    yq e -oj --inplace ".parameters += {\"name\":\"filter\",\"transform\":{\"type\":\"filter\",\"filter\":{\"rules\":[{\"type\":\"keep_entry_all_satisfied\",\"keepEntryAllSatisfied\":[$rulesStr]}]}}}" "$json"
    yq e -oj --inplace ".pipeline += {\"name\":\"filter\",\"follows\":\"enrich\"}" "$json"

    # add send step back
    yq e -oj --inplace ".pipeline += {\"name\":\"send\",\"follows\":\"filter\"}" "$json"

    updateFLPConfig "$json" "$3"
    ;;
  "log_level")
    yq e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"LOG_LEVEL\").value|=\"$2\"" "$3"
    ;;
  "node_selector")
    key=${2%:*}
    val=${2#*:}
    yq e --inplace ".spec.template.spec.nodeSelector.\"$key\" |= \"$val\"" "$3"
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
    --background) # Run command in background
      if [[ "$value" == "true" || "$value" == "false" ]]; then
        echo "param: $key, param_value: $value"
        runBackground="$value"
      else
        echo "invalid value for --background"
      fi
      ;;
    --copy) # Copy or skip without prompt
      if [[ "$value" == "true" || "$value" == "false" || "$value" == "prompt" ]]; then
        copy="$value"
      else
        echo "invalid value for --copy"
      fi
      ;;
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
    --enable_network_events) # Enable Network events monitoring
      if [[ "$3" == "flows" ]]; then
        if [[ "$value" == "true" || "$value" == "false" ]]; then
          edit_manifest "network_events_enable" "$value" "$2"
        else
          echo "invalid value for --enable_network_events"
        fi
      else
        echo "--enable_network_events is invalid option for packets"
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
    --sports) # Configure filter source two ports using ","
      edit_manifest "filter_sports" "$value" "$2"
      ;;
    --dports) # Configure filter destination two ports using ","
      edit_manifest "filter_dports" "$value" "$2"
      ;;
    --ports) # Configure filter on two ports usig "," can either be srcport or dstport
      edit_manifest "filter_ports" "$value" "$2"
      ;;
    --tcp_flags) # Configure filter TCP flags
      if [[ "$value" == "SYN" || "$value" == "SYN-ACK" || "$value" == "ACK" || "$value" == "FIN" || "$value" == "RST" || "$value" == "FIN-ACK" || "$value" == "RST-ACK" || "$value" == "PSH" || "$value" == "URG" || "$value" == "ECE" || "$value" == "CWR" ]]; then
        edit_manifest "filter_tcp_flags" "$value" "$2"
      else
        echo "invalid value for --tcp_flags"
      fi
      ;;
    --drops) # Filter packet drops
      if [[ "$value" == "true" || "$value" == "false" ]]; then
        edit_manifest "filter_pkt_drops" "$value" "$2"
      else
        echo "invalid value for --drops"
      fi
      ;;
    --regexes) # Filter using regexes
      valueCount=$(grep -o "~" <<<"$value" | wc -l)
      splitterCount=$(grep -o "," <<<"$value" | wc -l)
      if [[ ${valueCount} -gt 0 && $((valueCount)) == $((splitterCount + 1)) ]]; then
        edit_manifest "filter_regexes" "$value" "$2"
      else
        echo "invalid value for --regexes"
      fi
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
    --log-level) # Log level
      if [[ "$value" == "trace" || "$value" == "debug" || "$value" == "info" || "$value" == "warn" || "$value" == "error" || "$value" == "fatal" || "$value" == "panic" ]]; then
        edit_manifest "log_level" "$value" "$2"
        logLevel=$value
        filter=${filter/$key=$logLevel/}
      else
        echo "invalid value for --action"
      fi
      ;;
    --max-time) # Max time
      maxTime=$value
      filter=${filter/$key=$maxTime/}
      ;;
    --max-bytes) # Max bytes
      maxBytes=$value
      filter=${filter/$key=$maxBytes/}
      ;;
    --node-selector) # Node selector
      if [[ $value == *":"* ]]; then
        edit_manifest "node_selector" "$value" "$2"
      else
        echo "invalid value for --node-selector. Use --node-selector=key:val instead."
        exit 1
      fi
      ;;
    --get-subnets) # Get subnets
      if [[ "$value" == "true" || "$value" == "false" ]]; then
        edit_manifest "get_subnets" "$value" "$2"
      else
        echo "invalid value for --get-subnets"
      fi
      ;;
    *) # Invalid option
      echo "Invalid option: $key" >&2
      exit 1
      ;;
    esac
  done

  ${K8S_CLI_BIN} apply -f "$2"
  ${K8S_CLI_BIN} rollout status daemonset netobserv-cli -n "$namespace" --timeout 60s
  rm -rf ${MANIFEST_OUTPUT_PATH}
}
