package types

import (
	"bytes"
	"encoding/binary"
	"time"
)

const (
	// BlockTypeEnhancedPacket is the type code for an interface description block
	BlockTypeEnhancedPacket uint32 = 0x00000006
)

// Enhanced Packet block option codes
const (
	OptionCodeEnhancedPacketFlags     uint16 = 2
	OptionCodeEnhancedPacketHash      uint16 = 3
	OptionCodeEnhancedPacketDropCount uint16 = 4
)

// EnhancedPacketHeader statically sized header portion of an EnhancedPacket
type EnhancedPacketHeader struct {
	InterfaceID    uint32
	TimestampHigh  uint32
	TimestampLow   uint32
	CaptureLength  uint32
	OriginalLength uint32
}

// EnhancedPacket is the container for information describing an interface on which packet data is captured.
type EnhancedPacket struct {
	EnhancedPacketHeader
	PacketData []byte
	Options    Options
}

type EnhancedPacketOptions struct {
	Comment        string
	OriginalLength uint32
}

// NewEnhancedPacket creates an interface description with the provided options
func NewEnhancedPacket(interfaceID uint32, timestamp time.Time, data []byte, options EnhancedPacketOptions) (*EnhancedPacket, error) {
	opts := Options{}

	if options.Comment != "" {
		opts = append(opts, *NewOption(OptionCodeComment, []byte(options.Comment)))
	}

	originalLength := uint32(len(data))
	if options.OriginalLength != 0 {
		originalLength = options.OriginalLength
	}

	micros := uint64(timestamp.UnixNano() / 1e3)

	return &EnhancedPacket{
		EnhancedPacketHeader: EnhancedPacketHeader{
			InterfaceID:    interfaceID,
			TimestampHigh:  uint32(micros >> 32),
			TimestampLow:   uint32(micros),
			CaptureLength:  uint32(len(data)),
			OriginalLength: originalLength,
		},
		PacketData: data,
		Options:    opts,
	}, nil
}

// MarshalBinary encodes a EnhancedPacket to a byte array
func (epb *EnhancedPacket) MarshalBinary() ([]byte, error) {
	buff := bytes.NewBuffer(nil)

	opts, err := epb.Options.MarshalBinary()
	if err != nil {
		return nil, err
	}

	if err := binary.Write(buff, binary.LittleEndian, &epb.EnhancedPacketHeader); err != nil {
		return nil, err
	}

	if err := writePacked(buff, epb.PacketData); err != nil {
		return nil, err
	}

	if _, err := buff.Write(opts); err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

// UnmarshalBinary decodes a EnhancedPacket from a byte array
func (epb *EnhancedPacket) UnmarshalBinary(d []byte) error {
	buff := bytes.NewBuffer(d)

	if err := binary.Read(buff, binary.LittleEndian, &epb.EnhancedPacketHeader); err != nil {
		return err
	}

	data, err := readPacked(buff, uint(epb.CaptureLength))
	if err != nil {
		return err
	}
	epb.PacketData = data

	if buff.Len() > 0 {
		optd := make([]byte, buff.Len())
		if _, err := buff.Read(optd); err != nil {
			return err
		}
		if err := epb.Options.UnmarshalBinary(optd); err != nil {
			return err
		}
	}

	return nil
}
