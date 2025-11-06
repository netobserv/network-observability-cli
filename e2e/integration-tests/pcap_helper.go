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
