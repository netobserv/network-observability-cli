package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDisplayLoop(t *testing.T) {
	enrichment.current = 0
	assert.Equal(t, []string{"None"}, enrichment.getNames())
	assert.Equal(t, []string{"SrcAddr", "SrcPort", "DstAddr", "DstPort"}, enrichment.getCols())

	enrichment.prev()
	assert.Equal(t, []string{"Zone", "Host", "Owner", "Resource"}, enrichment.getNames())
	assert.Equal(t, []string{
		"SrcZone",
		"DstZone",
		"SrcHostName",
		"SrcHostName",
		"SrcOwnerName",
		"SrcOwnerType",
		"DstOwnerName",
		"DstOwnerType",
		"SrcName",
		"SrcType",
		"DstName",
		"DstType",
	}, enrichment.getCols())

	enrichment.next()
	assert.Equal(t, []string{"None"}, enrichment.getNames())
	assert.Equal(t, []string{"SrcAddr", "SrcPort", "DstAddr", "DstPort"}, enrichment.getCols())

	enrichment.next()
	assert.Equal(t, []string{"Zone"}, enrichment.getNames())
	assert.Equal(t, []string{"SrcZone", "DstZone"}, enrichment.getCols())
}
