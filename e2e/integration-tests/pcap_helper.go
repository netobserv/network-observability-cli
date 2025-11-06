//go:build !e2e

package integrationtests

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcapgo"
)

// pcap_helper.go provides utilities for reading and filtering packets from pcapng files
// in integration tests. It supports filtering by port, protocol, IP address, and packet length.
//
// Example usage:
//
//	// Read all packets from a file
//	packets, err := ReadPcapngFile("./output/pcap/capture.pcapng")
//	if err != nil {
//		return err
//	}
//
//	// Filter packets by port 58
//	port := uint16(58)
//	filter := &PacketFilter{Port: &port}
//	filteredPackets, err := ReadPcapngFileWithFilter("./output/pcap/capture.pcapng", filter)
//
//	// Count packets matching a specific protocol
//	tcpFilter := &PacketFilter{Protocol: "TCP"}
//	count, err := CountPackets("./output/pcap/capture.pcapng", tcpFilter)
//
//	// Check if any packets exist on port 58
//	hasPort58, err := HasPacketsOnPort("./output/pcap/capture.pcapng", 58)
//
//	// Get protocol distribution
//	distribution := GetProtocolDistribution(packets)
//	// Returns map[string]int, e.g., {"TCP": 10, "UDP": 5, "ICMPv6": 2}

// PacketInfo holds information about a captured packet
type PacketInfo struct {
	Timestamp   int64
	Length      int
	SrcIP       string
	DstIP       string
	SrcPort     uint16
	DstPort     uint16
	Protocol    string
	Comments    []string
	HasTCP      bool
	HasUDP      bool
	HasIPv4     bool
	HasIPv6     bool
	K8sMetadata map[string]string
}

// PacketFilter defines filtering criteria for packets
type PacketFilter struct {
	Port      *uint16 // Filter by port (source or destination)
	SrcPort   *uint16 // Filter by source port
	DstPort   *uint16 // Filter by destination port
	Protocol  string  // Filter by protocol (TCP, UDP, ICMP, etc.)
	SrcIP     string  // Filter by source IP
	DstIP     string  // Filter by destination IP
	MinLength int     // Minimum packet length
	MaxLength int     // Maximum packet length
}

// ReadPcapngFile reads a pcapng file and returns all packets
func ReadPcapngFile(filepath string) ([]PacketInfo, error) {
	return ReadPcapngFileWithFilter(filepath, nil)
}

// ReadPcapngFileWithFilter reads a pcapng file and returns filtered packets
func ReadPcapngFileWithFilter(filepath string, filter *PacketFilter) ([]PacketInfo, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open pcapng file: %w", err)
	}
	defer f.Close()

	ngReader, err := pcapgo.NewNgReader(f, pcapgo.DefaultNgReaderOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create pcapng reader: %w", err)
	}

	var packets []PacketInfo

	for {
		data, ci, opts, err := ngReader.ReadPacketDataWithOptions()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("error reading packet data: %w", err)
		}

		packet := gopacket.NewPacket(data, layers.LayerTypeEthernet, gopacket.Default)
		packetInfo := extractPacketInfo(packet, ci, opts)

		// Apply filter if provided
		if filter != nil && !matchesFilter(packetInfo, filter) {
			continue
		}

		packets = append(packets, packetInfo)
	}

	return packets, nil
}

// CountPackets counts packets in a pcapng file matching the filter
func CountPackets(filepath string, filter *PacketFilter) (int, error) {
	packets, err := ReadPcapngFileWithFilter(filepath, filter)
	if err != nil {
		return 0, err
	}
	return len(packets), nil
}

// extractPacketInfo extracts information from a packet
func extractPacketInfo(packet gopacket.Packet, ci gopacket.CaptureInfo, opts pcapgo.NgPacketOptions) PacketInfo {
	info := PacketInfo{
		Timestamp:   ci.Timestamp.Unix(),
		Length:      ci.Length,
		K8sMetadata: make(map[string]string),
	}

	// Extract comments from NgPacketOptions (contains k8s metadata)
	if len(opts.Comments) > 0 {
		info.Comments = opts.Comments
		extractK8sMetadata(&info)
	}

	// Check for IPv4 layer
	if ipv4Layer := packet.Layer(layers.LayerTypeIPv4); ipv4Layer != nil {
		ipv4, _ := ipv4Layer.(*layers.IPv4)
		info.SrcIP = ipv4.SrcIP.String()
		info.DstIP = ipv4.DstIP.String()
		info.HasIPv4 = true
		info.Protocol = ipv4.Protocol.String()
	}

	// Check for IPv6 layer
	if ipv6Layer := packet.Layer(layers.LayerTypeIPv6); ipv6Layer != nil {
		ipv6, _ := ipv6Layer.(*layers.IPv6)
		info.SrcIP = ipv6.SrcIP.String()
		info.DstIP = ipv6.DstIP.String()
		info.HasIPv6 = true
		info.Protocol = ipv6.NextHeader.String()
	}

	// Check for TCP layer
	if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
		tcp, _ := tcpLayer.(*layers.TCP)
		info.SrcPort = uint16(tcp.SrcPort)
		info.DstPort = uint16(tcp.DstPort)
		info.HasTCP = true
		if info.Protocol == "" {
			info.Protocol = "TCP"
		}
	}

	// Check for UDP layer
	if udpLayer := packet.Layer(layers.LayerTypeUDP); udpLayer != nil {
		udp, _ := udpLayer.(*layers.UDP)
		info.SrcPort = uint16(udp.SrcPort)
		info.DstPort = uint16(udp.DstPort)
		info.HasUDP = true
		if info.Protocol == "" {
			info.Protocol = "UDP"
		}
	}

	// Check for ICMP layers
	if packet.Layer(layers.LayerTypeICMPv4) != nil {
		info.Protocol = "ICMPv4"
	}
	if packet.Layer(layers.LayerTypeICMPv6) != nil {
		info.Protocol = "ICMPv6"
	}

	return info
}

// extractK8sMetadata parses k8s metadata from packet comments
func extractK8sMetadata(info *PacketInfo) {
	for _, comment := range info.Comments {
		lines := strings.Split(comment, "\n")
		for _, line := range lines {
			if strings.Contains(line, ":") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])
					info.K8sMetadata[key] = value
				}
			}
		}
	}
}

// matchesFilter checks if a packet matches the filter criteria
func matchesFilter(info PacketInfo, filter *PacketFilter) bool {
	// Filter by port (either source or destination)
	if filter.Port != nil {
		if info.SrcPort != *filter.Port && info.DstPort != *filter.Port {
			return false
		}
	}

	// Filter by source port
	if filter.SrcPort != nil && info.SrcPort != *filter.SrcPort {
		return false
	}

	// Filter by destination port
	if filter.DstPort != nil && info.DstPort != *filter.DstPort {
		return false
	}

	// Filter by protocol
	if filter.Protocol != "" && !strings.EqualFold(info.Protocol, filter.Protocol) {
		return false
	}

	// Filter by source IP
	if filter.SrcIP != "" && info.SrcIP != filter.SrcIP {
		return false
	}

	// Filter by destination IP
	if filter.DstIP != "" && info.DstIP != filter.DstIP {
		return false
	}

	// Filter by minimum length
	if filter.MinLength > 0 && info.Length < filter.MinLength {
		return false
	}

	// Filter by maximum length
	if filter.MaxLength > 0 && info.Length > filter.MaxLength {
		return false
	}

	return true
}

// FilterPacketsByPort filters packets by port (source or destination)
func FilterPacketsByPort(packets []PacketInfo, port uint16) []PacketInfo {
	var filtered []PacketInfo
	for _, p := range packets {
		if p.SrcPort == port || p.DstPort == port {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// FilterPacketsByProtocol filters packets by protocol
func FilterPacketsByProtocol(packets []PacketInfo, protocol string) []PacketInfo {
	var filtered []PacketInfo
	for _, p := range packets {
		if strings.EqualFold(p.Protocol, protocol) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// GetPacketCount returns the total number of packets in a file
func GetPacketCount(filepath string) (int, error) {
	packets, err := ReadPcapngFile(filepath)
	if err != nil {
		return 0, err
	}
	return len(packets), nil
}

// GetProtocolDistribution returns a map of protocol -> count
func GetProtocolDistribution(packets []PacketInfo) map[string]int {
	distribution := make(map[string]int)
	for _, p := range packets {
		distribution[p.Protocol]++
	}
	return distribution
}

// HasPacketsOnPort checks if there are any packets using the specified port
func HasPacketsOnPort(filepath string, port uint16) (bool, error) {
	filter := &PacketFilter{Port: &port}
	count, err := CountPackets(filepath, filter)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetUniqueIPs returns a list of unique IP addresses in the capture
func GetUniqueIPs(packets []PacketInfo) []string {
	ipMap := make(map[string]bool)
	for _, p := range packets {
		if p.SrcIP != "" {
			ipMap[p.SrcIP] = true
		}
		if p.DstIP != "" {
			ipMap[p.DstIP] = true
		}
	}

	ips := make([]string, 0, len(ipMap))
	for ip := range ipMap {
		ips = append(ips, ip)
	}
	return ips
}
