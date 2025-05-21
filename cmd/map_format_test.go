package cmd

import (
	"encoding/json"
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestFormatTableRows(t *testing.T) {
	setup(t)

	var flow config.GenericMap
	err := json.Unmarshal([]byte(sampleFlow), &flow)
	assert.Nil(t, err)

	// large fields
	assert.Equal(t, "17:25:28.703000     ", ToColValue(flow, "EndTime"))
	assert.Equal(t, "us-east-1d          ", ToColValue(flow, "SrcZone"))
	assert.Equal(t, "us-west-1a          ", ToColValue(flow, "DstZone"))
	assert.Equal(t, "my-deployment       ", ToColValue(flow, "SrcK8S_OwnerName"))
	assert.Equal(t, "my-statefulset      ", ToColValue(flow, "DstK8S_OwnerName"))
	assert.Equal(t, "src-pod             ", ToColValue(flow, "SrcK8S_Name"))
	assert.Equal(t, "dst-pod             ", ToColValue(flow, "DstK8S_Name"))

	// truncated
	assert.Equal(t, "ip-XX-X-X-XX1.ec2.in…", ToColValue(flow, "SrcK8S_HostName"))
	assert.Equal(t, "ip-XX-X-X-XX2.ec2.in…", ToColValue(flow, "DstK8S_HostName"))

	// medium
	assert.Equal(t, "10.128.0.29    ", ToColValue(flow, "SrcAddr"))
	assert.Equal(t, "10.129.0.26    ", ToColValue(flow, "DstAddr"))
	assert.Equal(t, "Ingress        ", ToColValue(flow, "FlowDirection"))
	assert.Equal(t, "TCP            ", ToColValue(flow, "Proto"))
	assert.Equal(t, "Standard       ", ToColValue(flow, "Dscp"))
	assert.Equal(t, "TCP_INVALID…   ", ToColValue(flow, "PktDropLatestState"))
	assert.Equal(t, "SKB_DROP…      ", ToColValue(flow, "PktDropLatestDropCause"))

	// small
	assert.Equal(t, "456B      ", ToColValue(flow, "Bytes"))
	assert.Equal(t, "5         ", ToColValue(flow, "Packets"))
	assert.Equal(t, "32B       ", ToColValue(flow, "PktDropBytes"))
	assert.Equal(t, "1         ", ToColValue(flow, "PktDropPackets"))
	assert.Equal(t, "1ms       ", ToColValue(flow, "DNSLatency"))
	assert.Equal(t, "NoError   ", ToColValue(flow, "DNSResponseCode"))
	assert.Equal(t, "10µs      ", ToColValue(flow, "TimeFlowRttMs"))
}
