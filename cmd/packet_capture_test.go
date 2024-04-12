package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPacketTableRefreshDelay(t *testing.T) {
	setup()

	// set output buffer to draw table
	buf := bytes.Buffer{}
	setOutputBuffer(&buf)

	managePacketsDisplay(PcapResult{Name: "test", ByteCount: 0, PacketCount: 0})

	out := buf.String()
	assert.Empty(t, out)
}

func TestPacketTableDefaultDisplay(t *testing.T) {
	setup()

	// set output buffer to draw table
	buf := bytes.Buffer{}
	setOutputBuffer(&buf)

	getRows := func(name string, byteCount int64, packetCount int64) []string {
		// add 1s to current time to avoid maxRefreshRate limit
		tickTime()
		buf.Reset()

		// add bytes and packets to table
		managePacketsDisplay(PcapResult{Name: name, ByteCount: byteCount, PacketCount: packetCount})

		// get table output per rows
		return strings.Split(buf.String(), "\n")
	}

	// start with 123 bytes and 1 packet
	rows := getRows("test", 123, 1)

	assert.Equal(t, 3, len(rows))
	assert.Equal(t, `Name          Packets          Bytes          `, rows[0])
	assert.Equal(t, `test          1                123B           `, rows[1])
	assert.Empty(t, rows[2])

	// add 10k bytes and 99 packets
	rows = getRows("test", 10000, 99)

	assert.Equal(t, 3, len(rows))
	assert.Equal(t, `Name          Packets          Bytes           `, rows[0])
	assert.Equal(t, `test          100              10.1KB          `, rows[1])
	assert.Empty(t, rows[2])

	// add another source
	rows = getRows("test2", 1, 1)

	assert.Equal(t, 4, len(rows))
	assert.Equal(t, `Name           Packets          Bytes           `, rows[0])
	assert.Equal(t, `test           100              10.1KB          `, rows[1])
	assert.Equal(t, `test2          1                1B              `, rows[2])
	assert.Empty(t, rows[3])
}
