package cmd

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlowDisplayRefreshDelay(t *testing.T) {
	setup(t)
	assert.Empty(t, getTableRows())

	parseGenericMapAndAppendFlow([]byte(`{"TimeFlowEndMs": 1709741962017}`))
	assert.Empty(t, getTableRows())

	updateTableAndSuggestions()
	rows := getTableRows()
	assert.Equal(t, 2, len(rows))
	assert.Equal(t, "End Time            Src Kind       Dst Kind       Src Name            Dst Name            Src Namespace       Dst Namespace       Interfaces     Interface Dirs Node Dir       L3 Protocol    L3 DSCP        Bytes     Packets   ", rows[0])
	assert.Equal(t, "17:19:22.017000     n/a            n/a            n/a                 n/a                 n/a                 n/a                 n/a            n/a            n/a            n/a            n/a            n/a       n/a       ", rows[1])
}

func TestFlowDisplayDefaultDisplay(t *testing.T) {
	setup(t)

	parseGenericMapAndAppendFlow([]byte(sampleFlow))
	tickTimeAndAddBytes()
	updateTableAndSuggestions()

	// get table output as string
	rows := getTableRows()
	assert.Equal(t, 2, len(rows))
	assert.Equal(t, "End Time            Src Kind       Dst Kind       Src Name            Dst Name            Src Namespace       Dst Namespace       Interfaces     Interface Dirs Node Dir       L3 Protocol    L3 DSCP        Bytes     Packets   ", rows[0])
	assert.Equal(t, "17:25:28.703000     Pod            Pod            src-pod             dst-pod             first-namespace     second-namespace    f18b970c2ce8fddEgress         Ingress        TCP            Standard       456B      5         ", rows[1])
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

	updateTableAndSuggestions()

	// get table output as string
	rows := getTableRows()
	// table must display only 31 rows (30 flows (max displayed limit) + 1 columns row)
	assert.Equal(t, 31, len(rows))
	// table columns
	assert.Equal(t, "End Time            Src IP         Src Port       Dst IP         Dst Port       Interfaces     Interface Dirs Node Dir       L3 Protocol    L3 DSCP        Bytes     Packets   ", rows[0])
	// first flow is the 11th one that came to the display
	assert.Equal(t, "00:00:11.000000     10.0.0.5       n/a            10.0.0.6       n/a            n/a            n/a            n/a            n/a            n/a            11KB      1         ", rows[1])
	assert.Equal(t, "00:00:12.000000     10.0.0.5       n/a            10.0.0.6       n/a            n/a            n/a            n/a            n/a            n/a            12KB      1         ", rows[2])
	assert.Equal(t, "00:00:13.000000     10.0.0.5       n/a            10.0.0.6       n/a            n/a            n/a            n/a            n/a            n/a            13KB      1         ", rows[3])
	assert.Equal(t, "00:00:14.000000     10.0.0.5       n/a            10.0.0.6       n/a            n/a            n/a            n/a            n/a            n/a            14KB      1         ", rows[4])
	assert.Equal(t, "00:00:15.000000     10.0.0.5       n/a            10.0.0.6       n/a            n/a            n/a            n/a            n/a            n/a            15KB      1         ", rows[5])
	// last flow is the 40th one
	assert.Equal(t, "00:00:40.000000     10.0.0.5       n/a            10.0.0.6       n/a            n/a            n/a            n/a            n/a            n/a            40KB      1         ", rows[30])
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
		updateTableAndSuggestions()

		// get table output per rows
		return getTableRows()
	}

	// set display without enrichment
	rows := getRows(allOptions, []string{pktDropFeature, dnsFeature, rttFeature, networkEventsDisplay}, noOptions, []string{})
	assert.Equal(t, 2, len(rows))
	assert.Equal(t, "End Time            Src IP         Src Port       Dst IP         Dst Port       Interfaces     Interface Dirs Drop BytesDrop…     Drop State     Drop Cause     Drop Flags     DNS Id    DNS…      DNS RCode DNS Error Flow RTT  Network Events      ", rows[0])
	assert.Equal(t, "17:25:28.703000     10.128.0.29    1234           10.129.0.26    5678           f18b970c2ce8fddEgress         32B       1         TCP_INVALID…   SKB_DROP…      16             31319     1ms       NoError   0         10µs      Allowed by default… ", rows[1])

	// set display to standard
	rows = getRows(standardDisplay, []string{}, noOptions, []string{})

	assert.Equal(t, 2, len(rows))
	assert.Equal(t, "End Time            Src IP         Src Port       Dst IP         Dst Port       Interfaces     Interface Dirs Node Dir       L3 Protocol    L3 DSCP        Bytes     Packets   ", rows[0])
	assert.Equal(t, "17:25:28.703000     10.128.0.29    1234           10.129.0.26    5678           f18b970c2ce8fddEgress         Ingress        TCP            Standard       456B      5         ", rows[1])

	// set display to pktDrop
	rows = getRows("Packet drops", []string{pktDropFeature}, noOptions, []string{})

	assert.Equal(t, 2, len(rows))
	assert.Equal(t, "End Time            Src IP         Src Port       Dst IP         Dst Port       Interfaces     Interface Dirs Drop BytesDrop…     Drop State     Drop Cause     Drop Flags     ", rows[0])
	assert.Equal(t, "17:25:28.703000     10.128.0.29    1234           10.129.0.26    5678           f18b970c2ce8fddEgress         32B       1         TCP_INVALID…   SKB_DROP…      16             ", rows[1])

	// set display to DNS
	rows = getRows("DNS", []string{dnsFeature}, noOptions, []string{})

	assert.Equal(t, 2, len(rows))
	assert.Equal(t, "End Time            Src IP         Src Port       Dst IP         Dst Port       Interfaces     Interface Dirs DNS Id    DNS…      DNS RCode DNS Error ", rows[0])
	assert.Equal(t, "17:25:28.703000     10.128.0.29    1234           10.129.0.26    5678           f18b970c2ce8fddEgress         31319     1ms       NoError   0         ", rows[1])

	// set display to RTT
	rows = getRows("RTT", []string{rttFeature}, noOptions, []string{})

	assert.Equal(t, 2, len(rows))
	assert.Equal(t, "End Time            Src IP         Src Port       Dst IP         Dst Port       Interfaces     Interface Dirs Flow RTT  ", rows[0])
	assert.Equal(t, "17:25:28.703000     10.128.0.29    1234           10.129.0.26    5678           f18b970c2ce8fddEgress         10µs      ", rows[1])

	// set display to NetworkEvents
	rows = getRows("Network events", []string{networkEventsDisplay}, noOptions, []string{})
	assert.Equal(t, 2, len(rows))
	assert.Equal(t, "End Time            Src IP         Src Port       Dst IP         Dst Port       Interfaces     Interface Dirs Network Events      ", rows[0])
	assert.Equal(t, "17:25:28.703000     10.128.0.29    1234           10.129.0.26    5678           f18b970c2ce8fddEgress         Allowed by default… ", rows[1])
}
