#!/usr/bin/env bash

# display main help
function help {
  echo
  echo "Netobserv allows you to capture flows, packets and metrics from your cluster."
  echo "Find more information at: https://github.com/netobserv/network-observability-cli/"
  echo
  echo "Syntax: netobserv [flows|packets|metrics|follow|stop|copy|cleanup|version] [options]"
  echo
  echo "main commands:"
  echo "  flows      Capture flows information in JSON format using collector pod."
  echo "  metrics    Capture metrics information in Prometheus using a ServiceMonitor (OCP cluster only)."
  echo "  packets    Capture packets information in pcap format using collector pod."
  echo
  echo "extra commands:"
  echo "  cleanup    Remove netobserv components and configurations."
  echo "  copy       Copy collector generated files locally."
  echo "  follow     Follow collector logs when running in background."
  echo "  stop       Stop collection by removing agent daemonset."
  echo "  version    Print software version."
  echo
  echo "basic examples:"
  echo "  netobserv flows --drops                              # Capture dropped flows on all nodes"
  echo "  netobserv flows --regexes=SrcK8S_Namespace~app-.*    # Capture flows from any namespace starting by app-"
  echo "  netobserv packets --port=8080                        # Capture packets on port 8080"
  echo "  netobserv metrics --enable_all                       # Capture all cluster metrics including packet drop, dns, rtt, network events packet translation and UDN mapping features informations"
  echo
  echo "advanced examples:"
  echo "  Capture drops in background and copy output locally"
  echo "    netobserv flows --background \                            # Capture flows using background mode"
  echo "    --max-time=15m \                                          # for a maximum of 15 minutes"
  echo "    --protocol=TCP --port=8080 \                              # either on TCP 8080"
  echo "    or --protocol=UDP                                         # or UDP"
  echo "    netobserv follow                                          # Display the progression of the background capture"
  echo "    netobserv stop                                            # Stop the background capture by deleting eBPF agents"
  echo "    netobserv copy                                            # Copy the background capture output data"
  echo "    netobserv cleanup                                         # Cleanup netobserv CLI by removing the remaining collector pod"
  echo
  echo "  Capture flows from a specific pod"
  echo "    netobserv flows                                           # Capture flows"
  echo "    --node-selector=kubernetes.io/hostname:my-node            # on node matching label 'kubernetes.io/hostname=my-node'"
  echo "    --regexes=SrcK8S_Name~.*my-pod.*,DstK8S_Name~.*my-pod.*   # from or to any pod name containing 'my-pod'"
  echo
  echo "  Capture packets on specific nodes and port"
  echo "    netobserv packets                                         # Capture packets"
  echo "    --node-selector=netobserv:true \                          # on nodes labelled with 'netobserv=true'"
  echo "    --port=80 \                                               # on port 80 only"
  echo "    --max-bytes=100000000                                     # for a maximum of 100MB"
  echo
}

# display version
function version {
  echo "Netobserv CLI version $1"
}

# agent / flp features
function features_usage {
  echo "  --enable_all:                 enable all eBPF features                   (default: false)"
  echo "  --enable_dns:                 enable DNS tracking                        (default: false)"
  echo "  --enable_pkt_translation:     enable packet translation                  (default: false)"
  echo "  --enable_pkt_drop:            enable packet drop                         (default: false)"
  echo "  --enable_rtt:                 enable RTT tracking                        (default: false)"
  echo "  --get-subnets:                get subnets informations                   (default: false)"
}

# collector options
function collector_usage {
  echo "  --background:                 run in background                          (default: false)"
  echo "  --copy:                       copy the output files locally              (default: prompt)"
  echo "  --log-level:                  components logs                            (default: info)"
  echo "  --max-time:                   maximum capture time                       (default: 5m)"
  echo "  --max-bytes:                  maximum capture bytes                      (default: 50000000 = 50MB)"
}

# agent selector / filters
function filters_usage {
  echo "  --action:                     filter action                              (default: Accept)"
  echo "  --cidr:                       filter CIDR                                (default: 0.0.0.0/0)"
  echo "  --direction:                  filter direction                           (default: n/a)"
  echo "  --dport:                      filter destination port                    (default: n/a)"
  echo "  --dport_range:                filter destination port range              (default: n/a)"
  echo "  --dports:                     filter on either of two destination ports  (default: n/a)"
  echo "  --drops:                      filter flows with only dropped packets     (default: false)"
  echo "  --icmp_code:                  filter ICMP code                           (default: n/a)"
  echo "  --icmp_type:                  filter ICMP type                           (default: n/a)"
  echo "  --node-selector:              capture on specific nodes                  (default: n/a)"
  echo "  --peer_ip:                    filter peer IP                             (default: n/a)"
  echo "  --peer_cidr:                  filter peer CIDR                           (default: n/a)"
  echo "  --port_range:                 filter port range                          (default: n/a)"
  echo "  --port:                       filter port                                (default: n/a)"
  echo "  --ports:                      filter on either of two ports              (default: n/a)"
  echo "  --protocol:                   filter protocol                            (default: n/a)"
  echo "  --regexes:                    filter flows using regular expression      (default: n/a)"
  echo "  --sport_range:                filter source port range                   (default: n/a)"
  echo "  --sport:                      filter source port                         (default: n/a)"
  echo "  --sports:                     filter on either of two source ports       (default: n/a)"
  echo "  --tcp_flags:                  filter TCP flags                           (default: n/a)"
}

function specific_filters_usage {
  # specific filters
  echo "  --interfaces:                 interfaces to monitor                      (default: n/a)"
}

function flows_usage {
  echo
  echo "Netobserv allows you to capture flows from your cluster."
  echo "Find more information at: https://github.com/netobserv/network-observability-cli/"
  echo
  echo "Syntax: netobserv flows [options]"
  echo
  echo "features:"
  features_usage
  echo
  echo "filters:"
  filters_usage
  specific_filters_usage
  echo
  echo "options:"
  collector_usage
}

function packets_usage {
  echo
  echo "Netobserv allows you to capture packets from your cluster."
  echo "Find more information at: https://github.com/netobserv/network-observability-cli/"
  echo
  echo "Syntax: netobserv packets [options]"
  echo
  echo "filters:"
  filters_usage
  echo
  echo "options:"
  collector_usage
}

function metrics_usage {
  echo
  echo "Netobserv allows you to capture metrics on your OCP cluster."
  echo "Find more information at: https://github.com/netobserv/network-observability-cli/"
  echo
  echo "Syntax: netobserv metrics [options]"
  echo
  echo "features:"
  features_usage
  echo
  echo "filters:"
  filters_usage
  specific_filters_usage
}

function follow_usage {
  echo
  echo "Netobserv allows you to capture flows and packets asyncronously using the --background option."
  echo "While the capture is running in background, you can connect to the collector pod to see the progression using the follow command."
  echo "Find more information at: https://github.com/netobserv/network-observability-cli/"
  echo
  echo "Syntax: netobserv follow"
  echo
}

function stop_usage {
  echo
  echo "Netobserv allows you stop the collection and keep collector or dashboard for post analysis."
  echo "While the capture is running, use the stop command to remove the eBPF agents."
  echo "Find more information at: https://github.com/netobserv/network-observability-cli/"
  echo
  echo "Syntax: netobserv stop"
  echo
}

function copy_usage {
  echo
  echo "Netobserv allows you copy locally the captured flows or packets from the collector pod."
  echo "While the collector is running, use the copy command to copy the output file(s)."
  echo "To avoid modifications during the copy, it's recommended to stop the capture"
  echo "Find more information at: https://github.com/netobserv/network-observability-cli/"
  echo
  echo "Syntax: netobserv copy"
  echo
}

function cleanup_usage {
  echo
  echo "Netobserv may require manual cleanup in some cases such as after a background run or in case of failure."
  echo "Use the cleanup command to remove all the netobserv CLI components."
  echo "Find more information at: https://github.com/netobserv/network-observability-cli/"
  echo
  echo "Syntax: netobserv cleanup"
  echo
}
