package cmd

import (
	"bytes"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/stretchr/testify/assert"
)

const (
	// this fake flow represent various possible values for every features
	sampleFlow = `{
		"AgentIP":"10.0.1.2",
		"Bytes":456,
		"DnsErrno":0,
		"DnsFlags":34176,
		"DnsFlagsResponseCode":"NoError",
		"DnsId":31319,
		"DnsLatencyMs":1,
		"Dscp":0,
		"DstAddr":"10.129.0.26",
		"DstK8S_HostIP":"10.0.1.2",
		"DstK8S_HostName":"ip-XX-X-X-XX2.ec2.internal",
		"DstK8S_Name":"dst-pod",
		"DstK8S_Namespace":"second-namespace",
		"DstK8S_OwnerName":"my-statefulset",
		"DstK8S_OwnerType":"StatefulSet",
		"DstK8S_Type":"Pod",
		"DstK8S_Zone":"us-west-1a",
		"DstMac":"0A:58:0A:81:00:1A",
		"DstPort":5678,
		"Duplicate":false,
		"Etype":2048,
		"Flags":16,
		"FlowDirection":0,
		"IfDirections":[1],
		"Interfaces":["f18b970c2ce8fdd"],
		"K8S_FlowLayer":"infra",
		"Packets":5,
		"PktDropBytes":32,
		"PktDropLatestDropCause":"SKB_DROP_REASON_TCP_INVALID_SEQUENCE",
		"PktDropLatestFlags":16,
		"PktDropLatestState":"TCP_INVALID_STATE",
		"PktDropPackets":1,
        "NetworkEvents":["hello"],
		"Proto":6,
		"SrcAddr":"10.128.0.29",
		"SrcK8S_HostIP":"10.0.1.1",
		"SrcK8S_HostName":"ip-XX-X-X-XX1.ec2.internal",
		"SrcK8S_Name":"src-pod",
		"SrcK8S_Namespace":"first-namespace",
		"SrcK8S_OwnerName":"my-deployment",
		"SrcK8S_OwnerType":"Deployment",
		"SrcK8S_Type":"Pod",
		"SrcK8S_Zone":"us-east-1d",
		"SrcMac":"0A:58:0A:81:00:01",
		"SrcPort":1234,
		"TimeFlowEndMs":1709742328703,
		"TimeFlowRttNs":10000,
		"TimeFlowStartMs":1709742328660,
		"TimeReceived":1709742330
	}`
)

var (
	originalTime  = currentTime
	simulatedTime = startupTime
)

func TestDefaultArguments(t *testing.T) {
	assert.Equal(t, "info", logLevel)
	assert.Equal(t, []int{9999}, ports)
	assert.Equal(t, []string{""}, nodes)
	assert.Empty(t, options)
}

func setup(t *testing.T) {
	// reset time to startup time
	resetTime()

	// clear filters and previous flows
	regexes = []string{}
	lastFlows = []config.GenericMap{}

	// load config
	err := LoadConfig()
	assert.Equal(t, nil, err)
}

func resetTime() {
	// set timezone to Paris time for all tests
	loc, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		log.Fatal(err)
	}
	time.Local = loc

	// reset all timers
	currentTime = originalTime
	lastRefresh = startupTime
	simulatedTime = startupTime
}

func tickTime() {
	currentTime = func() time.Time {
		simulatedTime = simulatedTime.Add(1 * time.Second)
		return simulatedTime
	}
}

func setOutputBuffer(buff *bytes.Buffer) {
	// set output buffer for testing
	outputBuffer = buff

	// avoid terminal clear
	resetTerminal = func() {}
}
