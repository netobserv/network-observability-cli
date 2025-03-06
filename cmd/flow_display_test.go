package cmd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlowDisplayRefreshDelay(t *testing.T) {
	setup(t)
	assert.Nil(t, outputBuffer)

	parseGenericMapAndAppendFlow([]byte(`{"TimeFlowEndMs": 1709741962017}`))
	assert.Nil(t, outputBuffer)

	updateTable()
	rows := strings.Split(outputBuffer.String(), "\n")

	assert.Equal(t, 5, len(rows))
	assert.Equal(t, `Running network-observability-cli as Flow Capture`, rows[0])
	assert.Equal(t, `Log level: info Duration: 0s Capture size: 0B`, rows[1])
	assert.Empty(t, rows[2])
	assert.Empty(t, rows[3])
	assert.Equal(t, `Collector is waiting for messages... Please wait.`, rows[4])
}

func TestFlowDisplayDefaultDisplay(t *testing.T) {
	setup(t)

	parseGenericMapAndAppendFlow([]byte(sampleFlow))
	tickTimeAndAddBytes()
	updateTable()

	// get table output as string
	rows := strings.Split(outputBuffer.String(), "\n")

	assert.Equal(t, 10, len(rows))
	assert.Equal(t, `Running network-observability-cli as Flow Capture`, rows[0])
	assert.Equal(t, `Log level: info Duration: 1s Capture size: 1B`, rows[1])
	assert.Equal(t, `Showing last: 20 Use Up / Down keyboard arrows to increase / decrease limit`, rows[2])
	assert.Equal(t, `Display: Standard Use Left / Right keyboard arrows to cycle views`, rows[3])
	assert.Equal(t, `Enrichment: Resource Use Page Up / Page Down keyboard keys to cycle enrichment scopes`, rows[4])
	assert.Equal(t, `End Time         Src Kind    Dst Kind    Src Name         Dst Name         Src Namespace    Dst Namespace     Interfaces       Interface Dirs  Node Dir    L3 Layer Protocol  L3 Layer DSCP  Bytes  Packets  `, rows[5])
	assert.Equal(t, `17:25:28.703000  Pod         Pod         src-pod          dst-pod          first-namespace  second-namespace  f18b970c2ce8fdd  Egress          Ingress     TCP                Standard       456B   5        `, rows[6])
	assert.Equal(t, `---------------  ----------  ----------  ---------------  ---------------  ---------------  ---------------   ----------       ----------      ----------  ----------         ----------     -----  -----    `, rows[7])
	assert.Equal(t, `Type anything to filter incoming flows in view`, rows[8])
	assert.Empty(t, rows[9])
}

func TestFlowDisplayMultipleFlows(t *testing.T) {
	setup(t)

	// set display to standard without enrichment
	display.current = 1
	enrichment.current = 0

	// set time and bytes per flow
	flowTime := 1704063600000
	bytes := 1

	// sent 40 flows to the table
	for range 40 {
		// update time and bytes for next flow
		flowTime += 1000
		bytes += 1000

		// add flow to table
		parseGenericMapAndAppendFlow([]byte(fmt.Sprintf(`{
			"AgentIP":"10.0.1.1",
			"Bytes":%d,
			"DstAddr":"10.0.0.6",
			"Packets":1,
			"SrcAddr":"10.0.0.5",
			"TimeFlowEndMs":%d
		}`, bytes, flowTime)))

		tickTimeAndAddBytes()
	}

	updateTable()

	// get table output as string
	rows := strings.Split(outputBuffer.String(), "\n")
	// table must display only 29 rows (20 flows (max displayed limit) + headers + footer + empty line)
	assert.Equal(t, 29, len(rows))
	// header
	assert.Equal(t, `Running network-observability-cli as Flow Capture`, rows[0])
	assert.Equal(t, `Log level: info Duration: 1s Capture size: 41B`, rows[1])
	assert.Equal(t, `Showing last: 20 Use Up / Down keyboard arrows to increase / decrease limit`, rows[2])
	assert.Equal(t, `Display: Standard Use Left / Right keyboard arrows to cycle views`, rows[3])
	assert.Equal(t, `Enrichment: None Use Page Up / Page Down keyboard keys to cycle enrichment scopes`, rows[4])
	// table columns
	assert.Equal(t, `End Time         Src IP      Src Port    Dst IP      Dst Port    Interfaces  Interface Dirs  Node Dir    L3 Layer Protocol  L3 Layer DSCP  Bytes  Packets  `, rows[5])
	// first flow is the 21st one that came to the display
	assert.Equal(t, `00:00:21.000000  10.0.0.5    n/a         10.0.0.6    n/a         n/a         n/a             n/a         n/a                n/a            21KB   1        `, rows[6])
	assert.Equal(t, `00:00:22.000000  10.0.0.5    n/a         10.0.0.6    n/a         n/a         n/a             n/a         n/a                n/a            22KB   1        `, rows[7])
	assert.Equal(t, `00:00:23.000000  10.0.0.5    n/a         10.0.0.6    n/a         n/a         n/a             n/a         n/a                n/a            23KB   1        `, rows[8])
	assert.Equal(t, `00:00:24.000000  10.0.0.5    n/a         10.0.0.6    n/a         n/a         n/a             n/a         n/a                n/a            24KB   1        `, rows[9])
	assert.Equal(t, `00:00:25.000000  10.0.0.5    n/a         10.0.0.6    n/a         n/a         n/a             n/a         n/a                n/a            25KB   1        `, rows[10])
	// last flow is the 40th one
	assert.Equal(t, `00:00:40.000000  10.0.0.5    n/a         10.0.0.6    n/a         n/a         n/a             n/a         n/a                n/a            40KB   1        `, rows[25])
	assert.Equal(t, `---------------  ----------  ----------  ----------  ----------  ----------  ----------      ----------  ----------         ----------     -----  -----    `, rows[26])
	// footer
	assert.Equal(t, `Type anything to filter incoming flows in view`, rows[27])
	assert.Empty(t, rows[28])

}

func TestFlowDisplayAdvancedDisplay(t *testing.T) {
	// getRows function cleanup everything and redraw table with sample flow
	getRows := func(displayName string, displayIds []string, enrichmentName string, enrichmentIds []string) []string {
		// prepare display options
		display = option{
			all: []optionItem{
				{name: displayName, ids: displayIds},
			},
			current: 0,
		}

		enrichment = option{
			all: []optionItem{
				{name: enrichmentName, ids: enrichmentIds},
			},
			current: 0,
		}

		// clear previous data and buffer
		setup(t)
		parseGenericMapAndAppendFlow([]byte(sampleFlow))
		tickTimeAndAddBytes()
		updateTable()

		// get table output per rows
		return strings.Split(outputBuffer.String(), "\n")
	}

	// set display without enrichment
	rows := getRows(allOptions, []string{pktDropFeature, dnsFeature, rttFeature, networkEventsDisplay}, noOptions, []string{})
	assert.Equal(t, 10, len(rows))
	assert.Equal(t, `End Time         Src IP       Src Port    Dst IP       Dst Port    Interfaces       Interface Dirs  Dropped Bytes  Dropped Packets  Drop State         Drop Cause                            Drop Flags  DNS Id  DNS Latency  DNS RCode  DNS Error  Flow RTT  Network Events                                                      `, rows[5])
	assert.Equal(t, `17:25:28.703000  10.128.0.29  1234        10.129.0.26  5678        f18b970c2ce8fdd  Egress          32B            1                TCP_INVALID_STATE  SKB_DROP_REASON_TCP_INVALID_SEQUENCE  16          31319   1ms          NoError    0          10µs      Allowed by default allow from local node policy, direction Ingress  `, rows[6])
	assert.Equal(t, `---------------  ----------   ----------  ----------   ----------  ----------       ----------      -----          -----            ----------         ----------                            ----------  -----   -----        -----      -----      -----     ---------------                                                     `, rows[7])
	assert.Empty(t, rows[9])

	// set display to standard
	rows = getRows(standardDisplay, []string{}, noOptions, []string{})

	assert.Equal(t, 10, len(rows))
	assert.Equal(t, `End Time         Src IP       Src Port    Dst IP       Dst Port    Interfaces       Interface Dirs  Node Dir    L3 Layer Protocol  L3 Layer DSCP  Bytes  Packets  `, rows[5])
	assert.Equal(t, `17:25:28.703000  10.128.0.29  1234        10.129.0.26  5678        f18b970c2ce8fdd  Egress          Ingress     TCP                Standard       456B   5        `, rows[6])
	assert.Equal(t, `---------------  ----------   ----------  ----------   ----------  ----------       ----------      ----------  ----------         ----------     -----  -----    `, rows[7])
	assert.Empty(t, rows[9])

	// set display to pktDrop
	rows = getRows("Packet drops", []string{pktDropFeature}, noOptions, []string{})

	assert.Equal(t, 10, len(rows))
	assert.Equal(t, `End Time         Src IP       Src Port    Dst IP       Dst Port    Interfaces       Interface Dirs  Dropped Bytes  Dropped Packets  Drop State         Drop Cause                            Drop Flags  `, rows[5])
	assert.Equal(t, `17:25:28.703000  10.128.0.29  1234        10.129.0.26  5678        f18b970c2ce8fdd  Egress          32B            1                TCP_INVALID_STATE  SKB_DROP_REASON_TCP_INVALID_SEQUENCE  16          `, rows[6])
	assert.Equal(t, `---------------  ----------   ----------  ----------   ----------  ----------       ----------      -----          -----            ----------         ----------                            ----------  `, rows[7])
	assert.Empty(t, rows[9])

	// set display to DNS
	rows = getRows("DNS", []string{dnsFeature}, noOptions, []string{})

	assert.Equal(t, 10, len(rows))
	assert.Equal(t, `End Time         Src IP       Src Port    Dst IP       Dst Port    Interfaces       Interface Dirs  DNS Id  DNS Latency  DNS RCode  DNS Error  `, rows[5])
	assert.Equal(t, `17:25:28.703000  10.128.0.29  1234        10.129.0.26  5678        f18b970c2ce8fdd  Egress          31319   1ms          NoError    0          `, rows[6])
	assert.Equal(t, `---------------  ----------   ----------  ----------   ----------  ----------       ----------      -----   -----        -----      -----      `, rows[7])
	assert.Empty(t, rows[9])

	// set display to RTT
	rows = getRows("RTT", []string{rttFeature}, noOptions, []string{})

	assert.Equal(t, 10, len(rows))
	assert.Equal(t, `End Time         Src IP       Src Port    Dst IP       Dst Port    Interfaces       Interface Dirs  Flow RTT  `, rows[5])
	assert.Equal(t, `17:25:28.703000  10.128.0.29  1234        10.129.0.26  5678        f18b970c2ce8fdd  Egress          10µs      `, rows[6])
	assert.Equal(t, `---------------  ----------   ----------  ----------   ----------  ----------       ----------      -----     `, rows[7])
	assert.Empty(t, rows[9])

	// set display to NetworkEvents
	rows = getRows("Network events", []string{networkEventsDisplay}, noOptions, []string{})
	assert.Equal(t, 10, len(rows))
	assert.Equal(t, `End Time         Src IP       Src Port    Dst IP       Dst Port    Interfaces       Interface Dirs  Network Events                                                      `, rows[5])
	assert.Equal(t, `17:25:28.703000  10.128.0.29  1234        10.129.0.26  5678        f18b970c2ce8fdd  Egress          Allowed by default allow from local node policy, direction Ingress  `, rows[6])
	assert.Equal(t, `---------------  ----------   ----------  ----------   ----------  ----------       ----------      ---------------                                                     `, rows[7])
	assert.Empty(t, rows[9])
}
