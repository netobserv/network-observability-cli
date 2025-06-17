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
	assert.Equal(t, "17:25:28.703000     ", toColValue(flow, "EndTime", toColWidth("EndTime")))
	assert.Equal(t, "us-east-1d          ", toColValue(flow, "SrcZone", toColWidth("SrcZone")))
	assert.Equal(t, "us-west-1a          ", toColValue(flow, "DstZone", toColWidth("DstZone")))
	assert.Equal(t, "my-deployment       ", toColValue(flow, "SrcK8S_OwnerName", toColWidth("SrcK8S_OwnerName")))
	assert.Equal(t, "my-statefulset      ", toColValue(flow, "DstK8S_OwnerName", toColWidth("DstK8S_OwnerName")))
	assert.Equal(t, "src-pod             ", toColValue(flow, "SrcK8S_Name", toColWidth("SrcK8S_Name")))
	assert.Equal(t, "dst-pod             ", toColValue(flow, "DstK8S_Name", toColWidth("DstK8S_Name")))

	// truncated
	assert.Equal(t, "ip-XX-X-X-XX1.ec2.in…", toColValue(flow, "SrcK8S_HostName", toColWidth("SrcK8S_HostName")))
	assert.Equal(t, "ip-XX-X-X-XX2.ec2.in…", toColValue(flow, "DstK8S_HostName", toColWidth("DstK8S_HostName")))

	// skip truncate
	assert.Equal(t, "ip-XX-X-X-XX1.ec2.internal", toColValue(flow, "SrcK8S_HostName", 0))
	assert.Equal(t, "ip-XX-X-X-XX2.ec2.internal", toColValue(flow, "DstK8S_HostName", 0))

	// medium
	assert.Equal(t, "10.128.0.29    ", toColValue(flow, "SrcAddr", toColWidth("SrcAddr")))
	assert.Equal(t, "10.129.0.26    ", toColValue(flow, "DstAddr", toColWidth("DstAddr")))
	assert.Equal(t, "Ingress        ", toColValue(flow, "FlowDirection", toColWidth("FlowDirection")))
	assert.Equal(t, "TCP            ", toColValue(flow, "Proto", toColWidth("Proto")))
	assert.Equal(t, "Standard       ", toColValue(flow, "Dscp", toColWidth("Dscp")))
	assert.Equal(t, "TCP_INVALID…   ", toColValue(flow, "PktDropLatestState", toColWidth("PktDropLatestState")))
	assert.Equal(t, "SKB_DROP…      ", toColValue(flow, "PktDropLatestDropCause", toColWidth("PktDropLatestDropCause")))

	// small
	assert.Equal(t, "456B      ", toColValue(flow, "Bytes", toColWidth("Bytes")))
	assert.Equal(t, "5         ", toColValue(flow, "Packets", toColWidth("Packets")))
	assert.Equal(t, "32B       ", toColValue(flow, "PktDropBytes", toColWidth("PktDropBytes")))
	assert.Equal(t, "1         ", toColValue(flow, "PktDropPackets", toColWidth("PktDropPackets")))
	assert.Equal(t, "1ms       ", toColValue(flow, "DNSLatency", toColWidth("DNSLatency")))
	assert.Equal(t, "NoError   ", toColValue(flow, "DNSResponseCode", toColWidth("DNSResponseCode")))
	assert.Equal(t, "10µs      ", toColValue(flow, "TimeFlowRttMs", toColWidth("TimeFlowRttMs")))
}
