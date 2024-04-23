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
		output, err := runCommand(slog, "oc-netobserv", "help")
		assert.Nil(t, err)

		err = os.WriteFile(path.Join("output", startupDate+"-helpOutput"), output, 0666)
		assert.Nil(t, err)

		str := string(output)
		assert.NotEmpty(t, str)
		// ensure help display overall description
		assert.Contains(t, str, "Netobserv allows you to capture flow and packets from your cluster.")
		assert.Contains(t, str, "Find more information at: https://github.com/netobserv/network-observability-cli/")
		// ensure help to display proper options
		assert.Contains(t, str, "Syntax: netobserv [flows|packets|cleanup] [filters]")
		assert.Contains(t, str, "flows      Capture flows information. You can specify an optionnal interface name as filter such as 'netobserv flows br-ex'.")
		assert.Contains(t, str, "packets    Capture packets from a specific protocol/port pair such as 'netobserv packets tcp,80'.")
		assert.Contains(t, str, "cleanup    Remove netobserv components.")
		assert.Contains(t, str, "version    Print software version.")
	})
}

func TestVersionCommand(t *testing.T) {
	t.Run("version command", func(t *testing.T) {
		output, err := runCommand(slog, "oc-netobserv", "version")
		assert.Nil(t, err)

		err = os.WriteFile(path.Join("output", startupDate+"-versionOutput"), output, 0666)
		assert.Nil(t, err)

		str := string(output)
		assert.NotEmpty(t, str)
		// ensure version display test
		assert.Contains(t, str, "Netobserv CLI version test")
	})
}
