//go:build e2e

package e2e

import (
	"os"
	"path"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

var (
	slog = logrus.WithField("component", "script_test")
)

func TestHelpCommand(t *testing.T) {
	t.Run("help command", func(t *testing.T) {
		output, err := RunCommand(slog, "commands/oc-netobserv", "help")
		assert.Nil(t, err)

		err = os.WriteFile(path.Join("output", StartupDate+"-helpOutput"), []byte(output), 0666)
		assert.Nil(t, err)

		assert.NotEmpty(t, output)
		// ensure help display overall description
		assert.Contains(t, output, "Netobserv allows you to capture flows, packets and metrics from your cluster.")
		assert.Contains(t, output, "Find more information at: https://github.com/netobserv/network-observability-cli/")
		// ensure help to display main commands
		assert.Contains(t, output, "main commands:")
		assert.Contains(t, output, "Syntax: netobserv [flows|packets|metrics|follow|stop|copy|cleanup|version] [options]")
		assert.Contains(t, output, "flows      Capture flows information in JSON format using collector pod.")
		assert.Contains(t, output, "metrics    Capture metrics information in Prometheus using a ServiceMonitor (OCP cluster only).")
		assert.Contains(t, output, "packets    Capture packets information in pcap format using collector pod.")
		// ensure help to display extra commands
		assert.Contains(t, output, "extra commands:")
		assert.Contains(t, output, "cleanup    Remove netobserv components and configurations.")
		assert.Contains(t, output, "copy       Copy collector generated files locally.")
		assert.Contains(t, output, "follow     Follow collector logs when running in background.")
		assert.Contains(t, output, "stop       Stop collection by removing agent daemonset.")
		assert.Contains(t, output, "version    Print software version.")
		// ensure help to display examples
		assert.Contains(t, output, "basic examples:")
		assert.Contains(t, output, "netobserv flows --drops                                     # Capture dropped flows on all nodes")
		assert.Contains(t, output, "netobserv flows --query='SrcK8S_Namespace=~\"app-.*\"'        # Capture flows from any namespace starting by app-")
		assert.Contains(t, output, "netobserv packets --port=8080                               # Capture packets on port 8080")
		assert.Contains(t, output, "netobserv metrics --enable_all                              # Capture default cluster metrics including packet drop, dns, rtt, network events packet translation and UDN mapping features informations")
		assert.Contains(t, output, "advanced examples:")
		assert.Contains(t, output, "Capture flows in background and copy output locally")
		assert.Contains(t, output, "Capture flows from a specific pod")
		assert.Contains(t, output, "Capture packets on specific nodes and port")
		assert.Contains(t, output, "Capture node and namespace drop metrics")

	})
}

func TestVersionCommand(t *testing.T) {
	t.Run("version command", func(t *testing.T) {
		output, err := RunCommand(slog, "commands/oc-netobserv", "version")
		assert.Nil(t, err)

		err = os.WriteFile(path.Join("output", StartupDate+"-versionOutput"), []byte(output), 0666)
		assert.Nil(t, err)

		assert.NotEmpty(t, output)
		// ensure version display test
		assert.Contains(t, output, "Netobserv CLI version test")
	})
}
