# Netobserv CLI

Users of Netobserv CLI can pass command line arguments to enable feature or pass configuration options to the eBPF agent.
This document lists all the supported command line options and their possible values.

## Command line arguments

- To build the Netobserv CLI commands locally

```shell
USER=netobserv make commands
```

- Example of running Netobserv CLI with some options

```shell
./build/oc-netobserv packets --action=Accept --cidr=0.0.0.0/0 --protocol=TCP --port=49051
```

- The following table show general options.

| Option          | Description                                 | Possible values                                  | Default   |
|-----------------|---------------------------------------------|--------------------------------------------------|-----------|
| --log-level     | Components logs                             | for example debug or trace                       | info      |
| --max-time      | Maximum capture time                        | for example 10m or 30s                           | 5m        |
| --max-bytes     | Maximum capture bytes                       | for example 10000000 (1MB)                       | 50000000  |
| --copy          | Copy the output files locally               | for example prompt, yes or no                    | prompt    |

- The following table shows filter configuration options.

| Option          | Description                                 | Possible values                                  | Mandatory | Default   |
|-----------------|---------------------------------------------|--------------------------------------------------|-----------|-----------|
| --action        | Action to apply on the flow                 | Accept, Reject                                   | yes       | Accept    |
| --cidr          | CIDR to match on the flow                   | for example 1.1.1.0/24 or 1::100/64 or 0.0.0.0/0 | yes       | 0.0.0.0/0 |
| --protocol      | Protocol to match on the flow               | TCP, UDP, SCTP, ICMP, ICMPv6                     | no        |           |
| --direction     | Direction to match on the flow              | Ingress, Egress                                  | no        |           |
| --dport         | Destination port to match on the flow       | for example 80 or 443 or 49051                   | no        |           |
| --sport         | Source port to match on the flow            | for example 80 or 443 or 49051                   | no        |           |
| --port          | Port to match on the flow                   | for example 80 or 443 or 49051                   | no        |           |
| --sport_range   | Source port range to match on the flow      | for example 80-100 or 443-445                    | no        |           |
| --dport_range   | Destination port range to match on the flow | for example 80-100                               | no        |           |
| --port_range    | Port range to match on the flow             | for example 80-100 or 443-445                    | no        |           |
| --icmp_type     | ICMP type to match on the flow              | for example 8 or 13                              | no        |           |
| --icmp_code     | ICMP code to match on the flow              | for example 0 or 1                               | no        |           |
| --peer_ip       | Peer IP to match on the flow                | for example 1.1.1.1 or 1::1                      | no        |           |