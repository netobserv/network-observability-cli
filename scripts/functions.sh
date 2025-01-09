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

FLOWS_MANIFEST_FILE="flow-capture.yml"
PACKETS_MANIFEST_FILE="packet-capture.yml"
METRICS_MANIFEST_FILE="metric-capture.yml"
CONFIG_JSON_TEMP="config.json"
CLUSTER_CONFIG="cluster-config-v1.yaml"
NETWORK_CONFIG="cluster-network.yaml"
MANIFEST_OUTPUT_PATH="tmp"

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
  flowAgentYAML="${flowAgentYAML/"{{AGENT_IMAGE_URL}}"/${agentImg}}"

  packetAgentYAML='
    packetAgentYAMLContent
  '
  if [ -f ./res/packet-capture.yml ]; then
    packetAgentYAML="$(cat ./res/packet-capture.yml)"
  fi
  packetAgentYAML="${packetAgentYAML/"{{NAMESPACE}}"/${namespace}}"
  packetAgentYAML="${packetAgentYAML/"{{AGENT_IMAGE_URL}}"/${agentImg}}"

  metricAgentYAML='
    metricAgentYAMLContent
  '
  if [ -f ./res/metric-capture.yml ]; then
    metricAgentYAML="$(cat ./res/metric-capture.yml)"
  fi
  metricAgentYAML="${metricAgentYAML//"{{NAMESPACE}}"/${namespace}}"
  metricAgentYAML="${metricAgentYAML/"{{AGENT_IMAGE_URL}}"/${agentImg}}"

  collectorServiceYAML='
    collectorServiceYAMLContent
  '
  if [ -f ./res/collector-service.yml ]; then
    collectorServiceYAML="$(cat ./res/collector-service.yml)"
  fi
  collectorServiceYAML="${collectorServiceYAML/"{{NAMESPACE}}"/${namespace}}"

  smYAML='
    smYAMLContent
  '
  if [ -f ./res/service-monitor.yml ]; then
    smYAML="$(cat ./res/service-monitor.yml)"
  fi
  smYAML="${smYAML//"{{NAMESPACE}}"/${namespace}}"
}

# set pipeline for flows & packets using collector
function setCollectorPipelineConfig() {
  # load pipeline json
  collectorPipelineConfigJSON='
    collectorPipelineConfigJSONContent
  '
  if [ -f ./res/collector-pipeline-config.json ]; then
    collectorPipelineConfigJSON="$(< ./res/collector-pipeline-config.json tr '\n' ' ')"
  fi

  # replace target host
  collectorPipelineConfigJSON="${collectorPipelineConfigJSON/"{{TARGET_HOST}}"/${targetHost}}"

  # append json to yaml file
  "$YQ_BIN" e --inplace " .spec.template.spec.containers[0].env[] |= select(.name == \"FLP_CONFIG\").value |= ($collectorPipelineConfigJSON | tojson)" "$1"
}

# set pipeline for metrics
function setMetricsPipelineConfig() {
  # load pipeline json
  metricsPipelineConfigJSON='
    metricsPipelineConfigJSONContent
  '
  if [ -f ./res/metrics-pipeline-config.json ]; then
    metricsPipelineConfigJSON="$(< ./res/metrics-pipeline-config.json tr '\n' ' ')"
  fi

  # append json to yaml file
  "$YQ_BIN" e --inplace " .spec.template.spec.containers[0].env[] |= select(.name == \"FLP_CONFIG\").value |= ($metricsPipelineConfigJSON | tojson)" "$1"
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

function getSubnets() {
  declare -n sn="$1"

  # get cluster-config-v1 Configmap to retreive machine networks
  installConfig=$(${K8S_CLI_BIN} get configmap cluster-config-v1 -n kube-system -o custom-columns=":data.install-config")
  yaml="${MANIFEST_OUTPUT_PATH}/${CLUSTER_CONFIG}"
  echo "$installConfig" >${yaml}

  machines=$("$YQ_BIN" e -oj '.networking.machineNetwork[] | select(has("cidr")).cidr' "$yaml")
  if [ "${#machines}" -gt 0 ]; then
    sn["Machines"]=$machines
  fi

  # get OCP cluster Network to retreive pod / services / external networks
  networkConfig=$(${K8S_CLI_BIN} get network cluster -o yaml)
  yaml="${MANIFEST_OUTPUT_PATH}/${NETWORK_CONFIG}"
  echo "$networkConfig" >${yaml}

  pods=$("$YQ_BIN" e -oj '.spec.clusterNetwork[] | select(has("cidr")).cidr' "$yaml")
  if [ "${#pods}" -gt 0 ]; then
    sn["Pods"]=$pods
  fi

  services=$("$YQ_BIN" e -oj '.spec.serviceNetwork[] | select(.)' "$yaml")
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

function getNodesByLabel () {
  printf "finding nodes matching %s...\n" "$1"
  nodeStr=$("$K8S_CLI_BIN" get nodes -l "$1" -o name | tr '\n' ' ')
  IFS=' ' read -ra nodeArray <<<"$nodeStr"
  if [ "${#nodeArray[@]}" -gt 0 ]; then
    printf "%d matching nodes found: %s\n" ${#nodeArray[@]} "$nodeStr" >&2
  else
    printf "%s doesn't match any node label. Please check your --node-selector parameter\n" "$1" >&2
    exit 1
  fi
}

function setup {
  echo "Setting up... "

  # check for mandatory arguments
  if ! [[ $1 =~ flows|packets|metrics ]]; then
    echo "invalid setup argument"
    return
  fi

  if ! clusterIsReady; then
    printf 'You must be connected to cluster\n' >&2
    exit 1
  fi

  if [ -z "${YQ_BIN+x}" ]; then
    printf 'yq tools must be installed for proper usage of netobserv cli\n' >&2
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

  if [ "$1" = "flows" ]; then
    echo "creating collector service"
    echo "$collectorServiceYAML" | ${K8S_CLI_BIN} apply -f -
    shift
    echo "creating flow-capture agents:"
    if [[ ! -d ${MANIFEST_OUTPUT_PATH} ]]; then
      mkdir -p ${MANIFEST_OUTPUT_PATH} >/dev/null
    fi
    manifest="${MANIFEST_OUTPUT_PATH}/${FLOWS_MANIFEST_FILE}"
    echo "${flowAgentYAML}" >${manifest}
    setCollectorPipelineConfig "$manifest"
    options="$*"
    check_args_and_apply "$options" "$manifest" "flows"
  elif [ "$1" = "packets" ]; then
    echo "creating collector service"
    echo "$collectorServiceYAML" | ${K8S_CLI_BIN} apply -f -
    shift
    echo "creating packet-capture agents"
    if [[ ! -d ${MANIFEST_OUTPUT_PATH} ]]; then
      mkdir -p ${MANIFEST_OUTPUT_PATH} >/dev/null
    fi
    manifest="${MANIFEST_OUTPUT_PATH}/${PACKETS_MANIFEST_FILE}"
    echo "${packetAgentYAML}" >${manifest}
    setCollectorPipelineConfig "$manifest"
    options="$*"
    check_args_and_apply "$options" "$manifest" "packets"
  elif [ "$1" = "metrics" ]; then
    echo "creating service monitor"
    echo "$smYAML" | ${K8S_CLI_BIN} apply -f -
    shift
    echo "creating metric-capture agents:"
    if [[ ! -d ${MANIFEST_OUTPUT_PATH} ]]; then
      mkdir -p ${MANIFEST_OUTPUT_PATH} >/dev/null
    fi
    manifest="${MANIFEST_OUTPUT_PATH}/${METRICS_MANIFEST_FILE}"
    echo "${metricAgentYAML}" >${manifest}
    setMetricsPipelineConfig "$manifest"
    options="$*"
    check_args_and_apply "$options" "$manifest" "metrics"
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

function deleteServiceMonitor {
  printf "\nDeleting service monitor... "
  ${K8S_CLI_BIN} delete servicemonitor netobserv-cli -n "$namespace" --ignore-not-found=true
}

function deleteDashboardCM {
  printf "\nDeleting dashboard configmap... "
  ${K8S_CLI_BIN} delete configmap netobserv-cli -n openshift-config-managed --ignore-not-found=true
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
        read -rp "Copy the capture output locally? [yes/no] " yn
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
    deleteServiceMonitor
    deleteDashboardCM
    deleteDaemonset
    deletePod
    deleteNamespace
    printf "\n"
  else
    echo "Cleanup namespace skipped"
    return
  fi
}

function features_usage {
  # agent / flp features
  echo "          --enable_pktdrop:         enable packet drop                         (default: false)"
  echo "          --enable_dns:             enable DNS tracking                        (default: false)"
  echo "          --enable_rtt:             enable RTT tracking                        (default: false)"
  echo "          --enable_network_events:  enable Network events monitoring           (default: false)"
  echo "          --get-subnets:            get subnets informations                   (default: false)"
}

function collector_usage {
  # collector options
  echo "          --log-level:              components logs                            (default: info)"
  echo "          --max-time:               maximum capture time                       (default: 5m)"
  echo "          --max-bytes:              maximum capture bytes                      (default: 50000000 = 50MB)"
  echo "          --background:             run in background                          (default: false)"
  echo "          --copy:                   copy the output files locally              (default: prompt)"
}

function filters_usage {
  # agent node selector
  echo "          --node-selector:          capture on specific nodes                  (default: n/a)"
  # agent filters
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
  echo "          --peer_cidr:              filter peer CIDR                           (default: n/a)"
  echo "          --drops:                  filter flows with only dropped packets     (default: false)"
  echo "          --regexes:                filter flows using regex                   (default: n/a)"
}

function specific_filters_usage {
  # specific filters
  echo "          --interfaces:             interfaces to monitor                      (default: n/a)"
}

function flows_usage {
  features_usage
  collector_usage
  filters_usage
  specific_filters_usage
}

function packets_usage {
  collector_usage
  filters_usage
}

function metrics_usage {
  features_usage
  filters_usage
  specific_filters_usage
}

# get current config and save it to temp file
function copyFLPConfig {
  jsonContent=$("$YQ_BIN" e '.spec.template.spec.containers[0].env[] | select(.name=="FLP_CONFIG").value' "$1")
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
  "$YQ_BIN" e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"FLP_CONFIG\").value|=\"$jsonContent\"" "$2"
}

# append a new flow filter rule to array
function addFlowFilter() {
  flowFilterJSON='
    flowFilterJSONContent
  '
  if [ -f ./res/flow-filter.json ]; then
    flowFilterJSON="$(cat ./res/flow-filter.json)"
  fi
  
  "$YQ_BIN" e --inplace " .spec.template.spec.containers[0].env[] |= select(.name == \"FLOW_FILTER_RULES\").value |=(fromjson | . += $flowFilterJSON | tojson)" "$1"
}

# update last flow filter of the array
function setLastFlowFilter() { 
  "$YQ_BIN" e --inplace " .spec.template.spec.containers[0].env[] |= select(.name == \"FLOW_FILTER_RULES\").value |=(fromjson | .[-1].$1 = $2 | tostring)" "$3"
}

# replace the configuration in the manifest file
function edit_manifest() {
  if [ -z "${2}" ]; then
    echo "opt: $1"
  else
    echo "opt: $1, value: $2"
  fi

  if [[ $1 == "filter_"* ]]; then
    "$YQ_BIN" e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"ENABLE_FLOW_FILTER\").value|=\"true\"" "$3"
    
    # add first filter in the array
    currentFilters=$( "$YQ_BIN" -r ".spec.template.spec.containers[0].env[] | select(.name == \"FLOW_FILTER_RULES\").value" "$3" )
    if [[ $currentFilters == "[]" ]]; then
      addFlowFilter "$3"
    fi
  fi

  case "$1" in
  "interfaces")
    "$YQ_BIN" e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"INTERFACES\").value|=\"$2\"" "$3"
    ;;
  "pktdrop_enable")
    "$YQ_BIN" e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"ENABLE_PKT_DROPS\").value|=\"$2\"" "$3"
    ;;
  "dns_enable")
    "$YQ_BIN" e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"ENABLE_DNS_TRACKING\").value|=\"$2\"" "$3"
    ;;
  "rtt_enable")
    "$YQ_BIN" e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"ENABLE_RTT\").value|=\"$2\"" "$3"
    ;;
  "network_events_enable")
    "$YQ_BIN" e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"ENABLE_NETWORK_EVENTS_MONITORING\").value|=\"$2\"" "$3"
    ;;
  "get_subnets")
    if [[ "$2" == "true" ]]; then
      declare -A subnets
      getSubnets subnets

      if [ "${#subnets[@]}" -gt 0 ]; then
        copyFLPConfig "$3"

        # get network enrich stage
        enrichIndex=$("$YQ_BIN" e -oj ".parameters[] | select(.name==\"enrich\") | document_index" "$json")
        enrichContent=$("$YQ_BIN" e -oj ".parameters[$enrichIndex]" "$json")
        enrichJson="${MANIFEST_OUTPUT_PATH}/enrich.json"
        echo "$enrichContent" >${enrichJson}

        # add rules to network
        "$YQ_BIN" e -oj --inplace ".transform.network.rules +={\"type\":\"add_subnet_label\",\"add_subnet_label\":{\"input\":\"SrcAddr\",\"output\":\"SrcSubnetLabel\"}}" "$enrichJson"
        "$YQ_BIN" e -oj --inplace ".transform.network.rules +={\"type\":\"add_subnet_label\",\"add_subnet_label\":{\"input\":\"DstAddr\",\"output\":\"DstSubnetLabel\"}}" "$enrichJson"

        # add subnetLabels to network
        "$YQ_BIN" e -oj --inplace ".transform.network.subnetLabels = []" "$enrichJson"
        for key in "${!subnets[@]}"; do
          "$YQ_BIN" e -oj --inplace ".transform.network.subnetLabels += {\"name\":\"$key\",\"cidrs\":[${subnets[$key]}]}" "$enrichJson"
        done

        # override network
        enrichJsonStr=$(cat $enrichJson)
        "$YQ_BIN" e -oj --inplace ".parameters[$enrichIndex] = $enrichJsonStr" "$json"

        updateFLPConfig "$json" "$3"
      fi
    fi
    ;;
  "add_filter")
    addFlowFilter "$3"
    ;;
  "filter_direction")
    setLastFlowFilter "direction" "\"$2\"" "$3"
    ;;
  "filter_cidr")
    setLastFlowFilter "ip_cidr" "\"$2\"" "$3"
    ;;
  "filter_protocol")
    setLastFlowFilter "protocol" "\"$2\"" "$3"
    ;;
  "filter_sport")
    setLastFlowFilter "source_port" = "$2" "$3"
    ;;
  "filter_dport")
    setLastFlowFilter "destination_port" "$2" "$3"
    ;;
  "filter_port")
    setLastFlowFilter "port" "$2" "$3"
    ;;
  "filter_sport_range")
    setLastFlowFilter "source_port_range" "\"$2\"" "$3"
    ;;
  "filter_dport_range")
    setLastFlowFilter "destination_port_range" "\"$2\"" "$3"
    ;;
  "filter_port_range")
    setLastFlowFilter "port_range" "\"$2\"" "$3"
    ;;
  "filter_sports")
    setLastFlowFilter "source_ports" "\"$2\"" "$3"
    ;;
  "filter_dports")
    setLastFlowFilter "destination_ports" "\"$2\"" "$3"
    ;;
  "filter_ports")
    setLastFlowFilter "ports" "\"$2\"" "$3"
    ;;
  "filter_icmp_type")
    setLastFlowFilter "icmp_type" "$2" "$3"
    ;;
  "filter_icmp_code")
    setLastFlowFilter "icmp_code" "$2" "$3"
    ;;
  "filter_peer_ip")
    setLastFlowFilter "peer_ip" "\"$2\"" "$3"
    ;;
  "filter_peer_cidr")
    setLastFlowFilter "peer_cidr" "\"$2\"" "$3"
    ;;
  "filter_action")
    setLastFlowFilter "action" "\"$2\"" "$3"
    ;;
  "filter_tcp_flags")
    setLastFlowFilter "tcp_flags" "\"$2\"" "$3"
    ;;
  "filter_pkt_drops")
    if [[ "$2" == "true" ]]; then
      # force enable drops before setting filter
      edit_manifest "pktdrop_enable" "$2" "$3"
    fi
    setLastFlowFilter "drops" "$2" "$3"
    ;;
  "filter_regexes")
    copyFLPConfig "$3"

    # remove send step
    "$YQ_BIN" e -oj --inplace "del(.pipeline[] | select(.name==\"send\"))" "$json"

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
    "$YQ_BIN" e -oj --inplace ".parameters += {\"name\":\"filter\",\"transform\":{\"type\":\"filter\",\"filter\":{\"rules\":[{\"type\":\"keep_entry_all_satisfied\",\"keepEntryAllSatisfied\":[$rulesStr]}]}}}" "$json"
    "$YQ_BIN" e -oj --inplace ".pipeline += {\"name\":\"filter\",\"follows\":\"enrich\"}" "$json"

    # add send step back
    "$YQ_BIN" e -oj --inplace ".pipeline += {\"name\":\"send\",\"follows\":\"filter\"}" "$json"

    updateFLPConfig "$json" "$3"
    ;;
  "log_level")
    "$YQ_BIN" e --inplace ".spec.template.spec.containers[0].env[] |= select(.name==\"LOG_LEVEL\").value|=\"$2\"" "$3"
    ;;
  "node_selector")
    key=${2%:*}
    val=${2#*:}
    getNodesByLabel "$key=$val"
    "$YQ_BIN" e --inplace ".spec.template.spec.nodeSelector.\"$key\" |= \"$val\"" "$3"
    ;;
  esac
}


# define key and value at script level to make them available all the time
# these will be updated by check_args_and_apply first and overriden by defaultValue when needed
key=""
value=""

function defaultValue() {
  if [ "$key" == "$value" ]; then
    value="$1"
  fi
}

# Check if the arguments are valid
#$1: options
#$2: manifest
#$3: flows, packets or metrics
function check_args_and_apply() {
  # Iterate through the command-line arguments
  for option in $1; do
    key="${option%%=*}"
    value="${option#*=}"
    case "$key" in
    or) # Increment flow filter array
      edit_manifest "add_filter" "" "$2"
      ;;
    --background) # Run command in background
      defaultValue "true"
      if [[ "$value" == "true" || "$value" == "false" ]]; then
        runBackground="$value"
      else
        echo "invalid value for --background"
      fi
      ;;
    --copy) # Copy or skip without prompt
      defaultValue "true"
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
      if [[ "$3" == "flows" || "$3" == "metrics" ]]; then
        defaultValue "true"
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
      if [[ "$3" == "flows" || "$3" == "metrics" ]]; then
        defaultValue "true"
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
      if [[ "$3" == "flows" || "$3" == "metrics" ]]; then
        defaultValue "true"
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
      if [[ "$3" == "flows" || "$3" == "metrics" ]]; then
        defaultValue "true"
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
      defaultValue "true"
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
    --peer_cidr) # Peer CIDR
      edit_manifest "filter_peer_cidr" "$value" "$2"
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
      defaultValue "true"
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
