package cmd

import (
	"encoding/json"
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestFormatTableRows(t *testing.T) {
	var flow config.GenericMap
	err := json.Unmarshal([]byte(sampleFlow), &flow)
	assert.Nil(t, err)

	tableRow := ToTableRow(flow, []string{"Time", "SrcAddr", "DstAddr", "Bytes", "Packets", "Dir", "Proto", "Dscp"})
	assert.Equal(t, []interface{}{"17:25:28.703000", "10.128.0.29", "10.129.0.26", "456B", float64(5), "Ingress", "TCP", "Standard"}, tableRow)

	tableRow = ToTableRow(flow, []string{"SrcZone", "DstZone", "SrcHostName", "DstHostName", "SrcOwnerName", "DstOwnerName", "SrcName", "DstName"})
	assert.Equal(t, []interface{}{"us-east-1d", "us-west-1a", "ip-XX-X-X-XX1.ec2.internal", "ip-XX-X-X-XX2.ec2.internal", "my-deployment", "my-statefulset", "src-pod", "dst-pod"}, tableRow)

	tableRow = ToTableRow(flow, []string{"DropBytes", "DropPackets", "DropState", "DropCause", "DnsLatency", "DnsRCode", "RTT"})
	assert.Equal(t, []interface{}{"32B", float64(1), "TCP_INVALID_STATE", "SKB_DROP_REASON_TCP_INVALID_SEQUENCE", "1ms", "NoError", "10µs"}, tableRow)
}
