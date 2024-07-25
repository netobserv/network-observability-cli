package types

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const (
	// OptionCodeEnd indicates the end of an option block
	OptionCodeEnd uint16 = 0
	// OptionCodeComment indicates a comment option block
	OptionCodeComment uint16 = 1
)

// OptionHeader is the header portion of a PCAP-NG option
type OptionHeader struct {
	Code   uint16
	Length uint16
}

// Option is a PCAP-NG option block, used to add comments etc. to captures
type Option struct {
	OptionHeader
	Value []byte
}

// NewOption creates a new option instance
func NewOption(code uint16, data []byte) *Option {
	return &Option{OptionHeader: OptionHeader{Code: code, Length: uint16(len(data))}, Value: data}
}

func (o *Option) String() string {
	return fmt.Sprintf("Option code: %d length: %d data: '%s'", o.Code, o.Length, string(o.Value))
}

// NewCommentOption creates a new comment option instance
func NewCommentOption(comment string) *Option {
	return NewOption(OptionCodeComment, []byte(comment))
}

// Options is a binary marshallable/unmarshallable wrapper around an options array
type Options []Option

// MarshalBinary coerces an option array into a byte array
func (o *Options) MarshalBinary() ([]byte, error) {
	buff := bytes.NewBuffer(nil)

	// Don't add any data for no options
	if len(*o) == 0 {
		return []byte{}, nil
	}

	/// Write options
	for _, o := range *o {
		if err := binary.Write(buff, binary.LittleEndian, &o.OptionHeader); err != nil {
			return nil, err
		}
		if err := writePacked(buff, o.Value); err != nil {
			return nil, err
		}
	}

	// Write EndOfOpt
	if err := binary.Write(buff, binary.LittleEndian, OptionCodeEnd); err != nil {
		return nil, err
	}
	if err := binary.Write(buff, binary.LittleEndian, uint16(0)); err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

// UnmarshalBinary converts a data array into a block
func (o *Options) UnmarshalBinary(d []byte) error {
	buff := bytes.NewBuffer(d)
	opts := Options{}

	// Skip if there are no options
	if len(d) < 8 {
		return nil
	}

	for {
		opt := Option{}

		if buff.Len() < 4 {
			break
		}

		if err := binary.Read(buff, binary.LittleEndian, &opt.OptionHeader); err != nil {
			return err
		}

		if opt.Code == OptionCodeEnd {
			break
		}

		if buff.Len() < int(opt.Length) {
			break
		}

		data, err := readPacked(buff, uint(opt.Length))
		if err != nil {
			return err
		}
		opt.Value = data

		opts = append(opts, opt)
	}

	*o = opts

	return nil
}
