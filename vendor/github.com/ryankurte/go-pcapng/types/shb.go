package types

import (
	"bytes"
	"encoding/binary"
)

const (
	// BlockTypeSectionHeader section header block type
	BlockTypeSectionHeader uint32 = 0x0A0D0D0A
)

// SectionHeader option codes
const (
	OptionCodeSectionHeaderHardware    uint16 = 2
	OptionCodeSectionHeaderOS          uint16 = 3
	OptionCodeSectionHeaderApplication uint16 = 4
)

// SectionHeaderHeader is the static header component of a section header
type SectionHeaderHeader struct {
	Magic         uint32
	VersionMajor  uint16
	VersionMinor  uint16
	SectionLength uint64
}

// SectionHeader is the internals of a section header block
type SectionHeader struct {
	SectionHeaderHeader
	Options Options
}

// SectionHeaderOptions options that can be passed in the section header
type SectionHeaderOptions struct {
	Comment     string
	Hardware    string
	OS          string
	Application string
}

// NewSectionHeader creates a section header with the provided options
func NewSectionHeader(options SectionHeaderOptions) *SectionHeader {
	opts := make([]Option, 0)

	if options.Comment != "" {
		opts = append(opts, *NewOption(OptionCodeComment, []byte(options.Comment)))
	}

	if options.Hardware != "" {
		opts = append(opts, *NewOption(OptionCodeSectionHeaderHardware, []byte(options.Hardware)))
	}

	if options.OS != "" {
		opts = append(opts, *NewOption(OptionCodeSectionHeaderOS, []byte(options.OS)))
	}

	if options.Application != "" {
		opts = append(opts, *NewOption(OptionCodeSectionHeaderApplication, []byte(options.Application)))
	}

	return &SectionHeader{
		SectionHeaderHeader: SectionHeaderHeader{
			Magic:         Magic,
			VersionMajor:  MajorVersion,
			VersionMinor:  MinorVersion,
			SectionLength: SectionLengthDefault,
		},
		Options: opts,
	}
}

// MarshalBinary encodes a SectionHeader to a byte array
func (shb *SectionHeader) MarshalBinary() ([]byte, error) {
	buff := bytes.NewBuffer(nil)

	opts, err := shb.Options.MarshalBinary()
	if err != nil {
		return nil, err
	}

	if err := binary.Write(buff, binary.LittleEndian, &shb.SectionHeaderHeader); err != nil {
		return nil, err
	}

	if _, err := buff.Write(opts); err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

// UnmarshalBinary decodes a SectionHeader from a byte array
func (shb *SectionHeader) UnmarshalBinary(d []byte) error {
	buff := bytes.NewBuffer(d)

	if err := binary.Read(buff, binary.LittleEndian, &shb.SectionHeaderHeader); err != nil {
		return err
	}

	optd := make([]byte, buff.Len())
	if _, err := buff.Read(optd); err != nil {
		return err
	}
	if err := shb.Options.UnmarshalBinary(optd); err != nil {
		return err
	}

	return nil
}
