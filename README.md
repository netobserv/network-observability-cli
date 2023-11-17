# Network Observability CLI

network-observability-cli is a lightweight Flow and Packet visualisation tool.
It deploy [netobserv eBPF agent](https://github.com/netobserv/netobserv-ebpf-agent) on your k8s cluster to collect flows or packets from nodes network interfaces
and streams data to a local collector for analysis and visualisation.
Output files are generated under output/flow and output/pcap directories per host name

## Work In Progress

This project is still a WIP. The following list gives an overview of the current progression:

- [ ] Capture flows
- [ ] Capture packets
- [ ] Basic filter capabilities
- [ ] Advanced filter capabilities
- [ ] Testing
- [ ] Linting
- [ ] Dockerfile

Feel free to contribute !
