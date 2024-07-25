package pcapng

import (
	"io"
	"time"

	"github.com/ryankurte/go-pcapng/types"
)

func writeSectionHeaderBlock(w io.Writer, options types.SectionHeaderOptions) error {
	sh := types.NewSectionHeader(options)
	shd, err := sh.MarshalBinary()
	if err != nil {
		return err
	}
	b := types.NewBlock(types.BlockTypeSectionHeader, shd)
	bd, err := b.MarshalBinary()
	if err != nil {
		return err
	}
	w.Write(bd)
	return nil
}

func writeInterfaceDescriptionBlock(w io.Writer, linkType uint16, options types.InterfaceOptions) error {
	sh, err := types.NewInterfaceDescription(linkType, options)
	if err != nil {
		return err
	}

	shd, err := sh.MarshalBinary()
	if err != nil {
		return err
	}
	b := types.NewBlock(types.BlockTypeInterfaceDescription, shd)
	bd, err := b.MarshalBinary()
	if err != nil {
		return err
	}
	w.Write(bd)
	return nil
}

func writeEnhancedPacketBlock(w io.Writer, interfaceID uint32, timestamp time.Time, data []byte, options types.EnhancedPacketOptions) error {
	ep, err := types.NewEnhancedPacket(interfaceID, timestamp, data, options)
	if err != nil {
		return err
	}

	epd, err := ep.MarshalBinary()
	if err != nil {
		return err
	}
	b := types.NewBlock(types.BlockTypeEnhancedPacket, epd)
	bd, err := b.MarshalBinary()
	if err != nil {
		return err
	}
	w.Write(bd)
	return nil
}
