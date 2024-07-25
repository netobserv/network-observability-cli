package types

// PCAP File constants
const (
	Magic                uint32 = 0x1A2B3C4D
	MajorVersion         uint16 = 1
	MinorVersion         uint16 = 0
	SectionLengthDefault uint64 = 0xFFFFFFFFFFFFFFFF
)

const (
	// LinkTypeIEEE802_15_4 IEEE802.15.4 link type
	LinkTypeIEEE802_15_4 uint16 = 195
	// LinkTypePrivate Private link type
	LinkTypePrivate uint16 = 147
)
