package cmd

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestFlowTableRefreshDelay(t *testing.T) {
	setup(t)

	// set output buffer to draw table
	buf := bytes.Buffer{}
	setOutputBuffer(&buf)

	parseGenericMapAndDisplay([]byte(`{"TimeFlowEndMs": 1709741962017}`))

	out := buf.String()
	assert.Empty(t, out)
}

func TestFlowTableDefaultDisplay(t *testing.T) {
	setup(t)

	// set output buffer to draw table
	buf := bytes.Buffer{}
	setOutputBuffer(&buf)

	// add 1s to current time to avoid maxRefreshRate limit
	tickTime()

	parseGenericMapAndDisplay([]byte(sampleFlow))

	// get table output as string
	rows := strings.Split(buf.String(), "\n")

	assert.Equal(t, 4, len(rows))
	assert.Equal(t, `End Time         Src Name         Src Namespace    Dst Name         Dst Namespace     Node Dir    Interfaces       L3 Layer Protocol  L3 Layer DSCP  Bytes  Packets  `, rows[0])
	assert.Equal(t, `17:25:28.703000  src-pod          first-namespace  dst-pod          second-namespace  Ingress     f18b970c2ce8fdd  TCP                Standard       456B   5        `, rows[1])
	assert.Equal(t, `---------------  ---------------  ---------------  ---------------  ---------------   ----------  ----------       ----------         ----------     -----  -----    `, rows[2])
	assert.Empty(t, rows[3])
}

func TestFlowTableMultipleFlows(t *testing.T) {
	setup(t)

	// set output buffer to draw table
	buf := bytes.Buffer{}
	setOutputBuffer(&buf)

	// set display to standard without enrichment
	display = []string{standardDisplay}
	enrichment = []string{noEnrichment}

	// set time and bytes per flow
	flowTime := 1704063600000
	bytes := 1

	// sent 40 flows to the table
	for i := 0; i < 40; i++ {
		// add 1s to current time to avoid maxRefreshRate limit
		tickTime()
		// reset buffer
		buf.Reset()

		// update time and bytes for next flow
		flowTime = flowTime + 1000
		bytes = bytes + 1000

		// add flow to table
		parseGenericMapAndDisplay([]byte(fmt.Sprintf(`{
			"AgentIP":"10.0.1.1",
			"Bytes":%d,
			"DstAddr":"10.0.0.6",
			"Packets":1,
			"SrcAddr":"10.0.0.5",
			"TimeFlowEndMs":%d
		}`, bytes, flowTime)))
	}

	// get table output as string
	rows := strings.Split(buf.String(), "\n")
	// table must display only 38 rows (35 flows + header + footer + empty line)
	assert.Equal(t, 38, len(rows))
	assert.Equal(t, `End Time         Src IP      Src Port    Dst IP      Dst Port    Node Dir    Interfaces  L3 Layer Protocol  L3 Layer DSCP  Bytes  Packets  `, rows[0])
	// first flow is the 6th one that came to the display
	assert.Equal(t, `00:00:06.000000  10.0.0.5    n/a         10.0.0.6    n/a         n/a         n/a         n/a                n/a            6KB    1        `, rows[1])
	assert.Equal(t, `00:00:07.000000  10.0.0.5    n/a         10.0.0.6    n/a         n/a         n/a         n/a                n/a            7KB    1        `, rows[2])
	assert.Equal(t, `00:00:08.000000  10.0.0.5    n/a         10.0.0.6    n/a         n/a         n/a         n/a                n/a            8KB    1        `, rows[3])
	assert.Equal(t, `00:00:09.000000  10.0.0.5    n/a         10.0.0.6    n/a         n/a         n/a         n/a                n/a            9KB    1        `, rows[4])
	assert.Equal(t, `00:00:10.000000  10.0.0.5    n/a         10.0.0.6    n/a         n/a         n/a         n/a                n/a            10KB   1        `, rows[5])
	// last flow is the 40th one
	assert.Equal(t, `00:00:40.000000  10.0.0.5    n/a         10.0.0.6    n/a         n/a         n/a         n/a                n/a            40KB   1        `, rows[35])
	assert.Equal(t, `---------------  ----------  ----------  ----------  ----------  ----------  ----------  ----------         ----------     -----  -----    `, rows[36])
	assert.Empty(t, rows[37])

}

func TestFlowTableAdvancedDisplay(t *testing.T) {
	setup(t)

	// set output buffer to draw table
	buf := bytes.Buffer{}
	setOutputBuffer(&buf)

	// getRows function cleanup everything and redraw table with sample flow
	getRows := func(d []string, e []string) []string {
		// prepare display options
		display = d
		enrichment = e

		// clear filters and previous flows
		regexes = []string{}
		lastFlows = []config.GenericMap{}
		buf.Reset()

		// add one second to time and draw table
		tickTime()
		parseGenericMapAndDisplay([]byte(sampleFlow))

		// get table output per rows
		return strings.Split(buf.String(), "\n")
	}

	// set display without enrichment
	rows := getRows([]string{pktDropDisplay, dnsDisplay, rttDisplay, networkEventsDisplay}, []string{noEnrichment})
	assert.Equal(t, 4, len(rows))
	assert.Equal(t, `End Time         Src IP       Src Port    Dst IP       Dst Port    Dropped Bytes  Dropped Packets  Drop State         Drop Cause                            Drop Flags  DNS Id  DNS Latency  DNS RCode  DNS Error  Flow RTT  Network Events   `, rows[0])
	assert.Equal(t, `17:25:28.703000  10.128.0.29  1234        10.129.0.26  5678        32B            1                TCP_INVALID_STATE  SKB_DROP_REASON_TCP_INVALID_SEQUENCE  16          31319   1ms          NoError    0          10µs      hello            `, rows[1])
	assert.Equal(t, `---------------  ----------   ----------  ----------   ----------  -----          -----            ----------         ----------                            ----------  -----   -----        -----      -----      -----     ---------------  `, rows[2])
	assert.Empty(t, rows[3])

	// set display to standard
	rows = getRows([]string{standardDisplay}, []string{noEnrichment})

	assert.Equal(t, 4, len(rows))
	assert.Equal(t, `End Time         Src IP       Src Port    Dst IP       Dst Port    Node Dir    Interfaces       L3 Layer Protocol  L3 Layer DSCP  Bytes  Packets  `, rows[0])
	assert.Equal(t, `17:25:28.703000  10.128.0.29  1234        10.129.0.26  5678        Ingress     f18b970c2ce8fdd  TCP                Standard       456B   5        `, rows[1])
	assert.Equal(t, `---------------  ----------   ----------  ----------   ----------  ----------  ----------       ----------         ----------     -----  -----    `, rows[2])
	assert.Empty(t, rows[3])

	// set display to pktDrop
	rows = getRows([]string{pktDropDisplay}, []string{noEnrichment})

	assert.Equal(t, 4, len(rows))
	assert.Equal(t, `End Time         Src IP       Src Port    Dst IP       Dst Port    Dropped Bytes  Dropped Packets  Drop State         Drop Cause                            Drop Flags  `, rows[0])
	assert.Equal(t, `17:25:28.703000  10.128.0.29  1234        10.129.0.26  5678        32B            1                TCP_INVALID_STATE  SKB_DROP_REASON_TCP_INVALID_SEQUENCE  16          `, rows[1])
	assert.Equal(t, `---------------  ----------   ----------  ----------   ----------  -----          -----            ----------         ----------                            ----------  `, rows[2])
	assert.Empty(t, rows[3])

	// set display to DNS
	rows = getRows([]string{dnsDisplay}, []string{noEnrichment})

	assert.Equal(t, 4, len(rows))
	assert.Equal(t, `End Time         Src IP       Src Port    Dst IP       Dst Port    DNS Id  DNS Latency  DNS RCode  DNS Error  `, rows[0])
	assert.Equal(t, `17:25:28.703000  10.128.0.29  1234        10.129.0.26  5678        31319   1ms          NoError    0          `, rows[1])
	assert.Equal(t, `---------------  ----------   ----------  ----------   ----------  -----   -----        -----      -----      `, rows[2])
	assert.Empty(t, rows[3])

	// set display to RTT
	rows = getRows([]string{rttDisplay}, []string{noEnrichment})

	assert.Equal(t, 4, len(rows))
	assert.Equal(t, `End Time         Src IP       Src Port    Dst IP       Dst Port    Flow RTT  `, rows[0])
	assert.Equal(t, `17:25:28.703000  10.128.0.29  1234        10.129.0.26  5678        10µs      `, rows[1])
	assert.Equal(t, `---------------  ----------   ----------  ----------   ----------  -----     `, rows[2])
	assert.Empty(t, rows[3])

	// set display to NetworkEvents
	rows = getRows([]string{networkEventsDisplay}, []string{noEnrichment})
	assert.Equal(t, 4, len(rows))
	assert.Equal(t, `End Time         Src IP       Src Port    Dst IP       Dst Port    Network Events   `, rows[0])
	assert.Equal(t, `17:25:28.703000  10.128.0.29  1234        10.129.0.26  5678        hello            `, rows[1])
	assert.Equal(t, `---------------  ----------   ----------  ----------   ----------  ---------------  `, rows[2])
	assert.Empty(t, rows[3])
}
