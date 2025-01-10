#!/usr/bin/env bash

# display main help
function help {
  echo
  echo "Netobserv allows you to capture flows, packets and metrics from your cluster."
  echo "Find more information at: https://github.com/netobserv/network-observability-cli/"
  echo
  echo "Syntax: netobserv [flows|packets|metrics|follow|stop|copy|cleanup|version] [options]"
  echo
  echo "commands:"
  echo "  flows      Capture flows information in JSON format using collector pod."
  echo "  packets    Capture packets information in pcap format using collector pod."
  echo "  metrics    Capture metrics information in Prometheus using a ServiceMonitor (OCP cluster only)."
  echo "  follow     Follow collector logs when running in background."
  echo "  stop       Stop collection by removing agent daemonset."
  echo "  copy       Copy collector generated files locally."
  echo "  cleanup    Remove netobserv components and configurations."
  echo "  version    Print software version."
  echo
}

# display version
function version {
  echo "Netobserv CLI version $1"
}

# agent / flp features
function features_usage {
  echo "  --enable_pktdrop:         enable packet drop                         (default: false)"
  echo "  --enable_dns:             enable DNS tracking                        (default: false)"
  echo "  --enable_rtt:             enable RTT tracking                        (default: false)"
  echo "  --enable_network_events:  enable Network events monitoring           (default: false)"
  echo "  --enable_all:             enable all eBPF features                   (default: false)"
  echo "  --get-subnets:            get subnets informations                   (default: false)"
}

# collector options
function collector_usage {
  echo "  --log-level:              components logs                            (default: info)"
  echo "  --max-time:               maximum capture time                       (default: 5m)"
  echo "  --max-bytes:              maximum capture bytes                      (default: 50000000 = 50MB)"
  echo "  --background:             run in background                          (default: false)"
  echo "  --copy:                   copy the output files locally              (default: prompt)"
}

# agent selector / filters
function filters_usage {
  # node selector
  echo "  --node-selector:          capture on specific nodes                  (default: n/a)"
  # filters
  echo "  --direction:              filter direction                           (default: n/a)"
  echo "  --cidr:                   filter CIDR                                (default: 0.0.0.0/0)"
  echo "  --protocol:               filter protocol                            (default: n/a)"
  echo "  --sport:                  filter source port                         (default: n/a)"
  echo "  --dport:                  filter destination port                    (default: n/a)"
  echo "  --port:                   filter port                                (default: n/a)"
  echo "  --sport_range:            filter source port range                   (default: n/a)"
  echo "  --dport_range:            filter destination port range              (default: n/a)"
  echo "  --port_range:             filter port range                          (default: n/a)"
  echo "  --sports:                 filter on either of two source ports       (default: n/a)"
  echo "  --dports:                 filter on either of two destination ports  (default: n/a)"
  echo "  --ports:                  filter on either of two ports              (default: n/a)"
  echo "  --tcp_flags:              filter TCP flags                           (default: n/a)"
  echo "  --action:                 filter action                              (default: Accept)"
  echo "  --icmp_type:              filter ICMP type                           (default: n/a)"
  echo "  --icmp_code:              filter ICMP code                           (default: n/a)"
  echo "  --peer_ip:                filter peer IP                             (default: n/a)"
  echo "  --peer_cidr:              filter peer CIDR                           (default: n/a)"
  echo "  --drops:                  filter flows with only dropped packets     (default: false)"
  echo "  --regexes:                filter flows using regular expression      (default: n/a)"
}

function specific_filters_usage {
  # specific filters
  echo "  --interfaces:             interfaces to monitor                      (default: n/a)"
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
  echo "options:"
  collector_usage
  echo
  echo "filters:"
  filters_usage
  specific_filters_usage
}

function packets_usage {
  echo
  echo "Netobserv allows you to capture packets from your cluster."
  echo "Find more information at: https://github.com/netobserv/network-observability-cli/"
  echo
  echo "Syntax: netobserv packets [options]"
  echo
  echo "options:"
  collector_usage
  echo
  echo "filters:"
  filters_usage
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
