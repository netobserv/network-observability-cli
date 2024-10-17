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

	tableRow := ToTableRow(flow, []string{"EndTime", "SrcAddr", "DstAddr", "Bytes", "Packets", "FlowDirection", "Proto", "Dscp"})
	assert.Equal(t, []interface{}{"17:25:28.703000", "10.128.0.29", "10.129.0.26", "456B", float64(5), "Ingress", "TCP", "Standard"}, tableRow)

	tableRow = ToTableRow(flow, []string{"SrcZone", "DstZone", "SrcK8S_HostName", "DstK8S_HostName", "SrcK8S_OwnerName", "DstK8S_OwnerName", "SrcK8S_Name", "DstK8S_Name"})
	assert.Equal(t, []interface{}{"us-east-1d", "us-west-1a", "ip-XX-X-X-XX1.ec2.internal", "ip-XX-X-X-XX2.ec2.internal", "my-deployment", "my-statefulset", "src-pod", "dst-pod"}, tableRow)

	tableRow = ToTableRow(flow, []string{"PktDropBytes", "PktDropPackets", "PktDropLatestState", "PktDropLatestDropCause", "DNSLatency", "DNSResponseCode", "TimeFlowRttMs"})
	assert.Equal(t, []interface{}{"32B", float64(1), "TCP_INVALID_STATE", "SKB_DROP_REASON_TCP_INVALID_SEQUENCE", "1ms", "NoError", "10µs"}, tableRow)
}
