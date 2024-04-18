# Netobserv CLI

Users of Netobserv CLI can pass command line arguments to enable features or pass configuration options to the eBPF agent.
This document lists all the supported command line options and their possible values.

## Command line arguments

- To build the Netobserv CLI commands locally

```shell
USER=netobserv make commands
```

- Example of running Netobserv CLI with some options

```shell
./build/oc-netobserv flows --enable_pktdrop=true  --enable_rtt=true --enable_filter=true --action=Accept --cidr=0.0.0.0/0 --protocol=TCP --port=49051
```

- The following table shows all supported features options.

| Option           | Description                     | Possible values   |
|------------------|---------------------------------|-------------------|
| --help           | Show help                       | -                 |
| --enable_pktdrop | Enable packet drop              | true, false       |
| --enable_rtt     | Enable round trip time          | true, false       |
| --enable_dns     | Enable DNS tracking             | true, false       |
| --interfaces     | Interfaces to match on the flow | e.g., "eth0,eth1" |

- The following table shows flow filter configuration options.

| Option          | Description                                 | Possible values                                  | Mandatory |
|-----------------|---------------------------------------------|--------------------------------------------------|-----------|
| --enable_filter | Enable flow filter                          | true, false                                      |  yes      |
| --action        | Action to apply on the flow                 | Accept, Reject                                   |  yes      |
| --cidr          | CIDR to match on the flow                   | for example 1.1.1.0/24 or 1::100/64 or 0.0.0.0/0 |  yes      |
| --protocol      | Protocol to match on the flow               | TCP, UDP, SCTP, ICMP, ICMPv6                     |  no       |
| --port          | Port to match on the flow                   | for example 80 or 443 or 49051                   |  no       |
| --direction     | Direction to match on the flow              | Ingress, Egress                                  |  no       |
| --dport         | Destination port to match on the flow       | for example 80 or 443 or 49051                   |  no       |
| --sport         | Source port to match on the flow            | for example 80 or 443 or 49051                   |  no       |
| --sport_range   | Source port range to match on the flow      | for example 80-100 or 443-445                    |  no       |
| --dport_range   | Destination port range to match on the flow | for example 80-100                               |  no       |
| --port_range    | Port range to match on the flow             | for example 80-100 or 443-445                    |  no       |
| --icmp_type     | ICMP type to match on the flow              | for example 8 or 13                              |  no       |
| --icmp_code     | ICMP code to match on the flow              | for example 0 or 1                               |  no       |
| --peer_ip       | Peer IP to match on the flow                | for example 1.1.1.1 or 1::1                      |  no       |
