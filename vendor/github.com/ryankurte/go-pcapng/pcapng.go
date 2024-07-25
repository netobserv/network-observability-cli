package pcapng

import (
	"os"
	"time"

	"github.com/ryankurte/go-pcapng/types"
)

// FileWriter is a PCAP-NG file writer
type FileWriter struct {
	f *os.File
}

// NewFileWriter creates a new PCAP-NG file writing instanew
func NewFileWriter(fileName string) (*FileWriter, error) {
	// Open capture file
	f, err := os.OpenFile(fileName, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0666)
	if err != nil {
		return nil, err
	}

	return &FileWriter{f: f}, nil
}

// WriteSectionHeader writes a pcap-ng section header
// This is required at the start of a file, and optional to start new sections
func (fw *FileWriter) WriteSectionHeader(options types.SectionHeaderOptions) error {
	return writeSectionHeaderBlock(fw.f, options)
}

// WriteInterfaceDescription writes an interface description block
// This creates an interface which should be referenced by order created in enhanced packets
func (fw *FileWriter) WriteInterfaceDescription(linkType uint16, options types.InterfaceOptions) error {
	return writeInterfaceDescriptionBlock(fw.f, linkType, options)
}

// WriteEnhancedPacketBlock writes an enhanced packet block
// InterfaceID must be the index of a previously created interface description
func (fw *FileWriter) WriteEnhancedPacketBlock(interfaceID uint32, timestamp time.Time, data []byte, options types.EnhancedPacketOptions) error {
	return writeEnhancedPacketBlock(fw.f, interfaceID, timestamp, data, options)
}

// Close closes the file writer
func (fw *FileWriter) Close() {
	fw.f.Close()
}
