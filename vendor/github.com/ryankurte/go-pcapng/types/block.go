package types

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// BlockHeader is the header of a PCAP-NG block
type BlockHeader struct {
	Type   uint32
	Length uint32
}

// BlockTraler is the trailer of a PCAP-NG block
type BlockTrailer struct {
	Length uint32
}

// Block is the general structure of all PCAP-NG blocks
type Block struct {
	BlockHeader
	Data []byte
	BlockTrailer
}

// NewBlock creates a block of the provided size with the given data
func NewBlock(blockType uint32, data []byte) *Block {
	length := uint32(len(data) + 12)
	return &Block{
		BlockHeader:  BlockHeader{Type: blockType, Length: length},
		Data:         data,
		BlockTrailer: BlockTrailer{Length: length},
	}
}

// MarshalBinary coerces a block into a byte array
func (b *Block) MarshalBinary() ([]byte, error) {
	buff := bytes.NewBuffer(nil)
	if err := binary.Write(buff, binary.LittleEndian, b.BlockHeader); err != nil {
		return nil, err
	}

	if err := writePacked(buff, b.Data); err != nil {
		return nil, err
	}

	if err := binary.Write(buff, binary.LittleEndian, b.BlockTrailer); err != nil {
		return nil, err
	}
	return buff.Bytes(), nil
}

// UnmarshalBinary converts a data array into a block
func (b *Block) UnmarshalBinary(d []byte) error {
	buff := bytes.NewBuffer(d)

	if err := binary.Read(buff, binary.LittleEndian, &b.BlockHeader); err != nil {
		return err
	}

	dataLength := uint(b.BlockHeader.Length - 12)
	data, err := readPacked(buff, dataLength)
	if err != nil {
		return err
	}
	b.Data = data

	if err := binary.Read(buff, binary.LittleEndian, &b.BlockTrailer); err != nil {
		return err
	}

	if b.BlockHeader.Length != b.BlockTrailer.Length {
		return fmt.Errorf("PCAP-NG Block error, mismatch header and trailer lengths (%d, %d)",
			b.BlockHeader.Length, b.BlockTrailer.Length)
	}

	return nil
}
