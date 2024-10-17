//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/netobserv/network-observability-cli/e2e/cluster"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/sirupsen/logrus"
)

const (
	clusterNamePrefix = "netobserv-cli-e2e-test-cluster"
	namespace         = "default"
	ExportLogsTimeout = 30 * time.Second
)

var (
	testCluster *cluster.Kind
	clog        = logrus.WithField("component", "capture_test")
)

func TestMain(m *testing.M) {
	if os.Getenv("ACTIONS_RUNNER_DEBUG") == "true" {
		logrus.StandardLogger().SetLevel(logrus.DebugLevel)
	}
	testCluster = cluster.NewKind(
		clusterNamePrefix+StartupDate,
		path.Join(".."),
	)
	testCluster.Run(m)
}

func TestFlowCapture(t *testing.T) {
	f1 := features.New("flow capture").Setup(
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			timer := time.AfterFunc(ExportLogsTimeout, func() {
				agentLogs := testCluster.GetAgentLogs()
				err := os.WriteFile(path.Join(testCluster.GetLogsDir(), StartupDate+"-flowAgentLogs"), []byte(agentLogs), 0666)
				assert.Nil(t, err)
			})
			defer timer.Stop()

			output, err := RunCommand(clog, "oc-netobserv", "flows", "--log-level=trace")
			// TODO: find a way to avoid error here; this is probably related to SIGTERM instead of CTRL + C call
			//assert.Nil(t, err)

			err = os.WriteFile(path.Join("output", StartupDate+"-flowOutput"), output, 0666)
			assert.Nil(t, err)

			str := string(output)
			assert.NotEmpty(t, str)
			// ensure script setup is fine
			assert.Contains(t, str, "namespace/netobserv-cli created")
			assert.Contains(t, str, "serviceaccount/netobserv-cli created")
			assert.Contains(t, str, "service/collector created")
			assert.Contains(t, str, "daemonset.apps/netobserv-cli created")
			assert.Contains(t, str, "pod/collector created")
			assert.Contains(t, str, "pod/collector condition met")
			// check that CLI is running
			assert.Contains(t, str, "Running network-observability-cli as Flow Capture")
			assert.Contains(t, str, "Time")
			assert.Contains(t, str, "Src Name")
			assert.Contains(t, str, "Src Namespace")
			assert.Contains(t, str, "Dst Name")
			assert.Contains(t, str, "Dst Namespace")
			// check that script terminated
			assert.Contains(t, str, "command terminated")
			return ctx
		},
	).Assess("check downloaded output flow files",
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var jsons []string
			var dbs []string

			dirPath := path.Join("output", "flow")
			assert.True(t, dirExists(dirPath), "directory %s not found", dirPath)
			err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					fmt.Println(err)
				}

				if !info.IsDir() {
					if filepath.Ext(path) == ".json" {
						jsons = append(jsons, path)
					} else if filepath.Ext(path) == ".db" {
						dbs = append(dbs, path)
					}
				}

				return nil
			})
			assert.Nil(t, err)

			// check json file
			assert.Equal(t, 1, len(jsons))
			jsonBytes, err := os.ReadFile(jsons[0])
			assert.Nil(t, err)
			assert.Contains(t, string(jsonBytes), "AgentIP")

			// check db file
			assert.Equal(t, 1, len(dbs))
			dbBytes, err := os.ReadFile(dbs[0])
			assert.Nil(t, err)
			assert.Contains(t, string(dbBytes), "SQLite format")
			return ctx
		},
	).Feature()
	testCluster.TestEnv().Test(t, f1)
}

func TestPacketCapture(t *testing.T) {
	f1 := features.New("packet capture").Setup(
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			timer := time.AfterFunc(ExportLogsTimeout, func() {
				agentLogs := testCluster.GetAgentLogs()
				err := os.WriteFile(path.Join(testCluster.GetLogsDir(), StartupDate+"-packetAgentLogs"), []byte(agentLogs), 0666)
				assert.Nil(t, err)
			})
			defer timer.Stop()

			output, err := RunCommand(clog, "oc-netobserv", "packets", "--log-level=trace", "--protocol=TCP", "--port=6443")
			// TODO: find a way to avoid error here; this is probably related to SIGTERM instead of CTRL + C call
			//assert.Nil(t, err)

			err = os.WriteFile(path.Join("output", StartupDate+"-packetOutput"), output, 0666)
			assert.Nil(t, err)

			str := string(output)
			assert.NotEmpty(t, str)
			// ensure script setup is fine
			assert.Contains(t, str, "namespace/netobserv-cli created")
			assert.Contains(t, str, "serviceaccount/netobserv-cli created")
			assert.Contains(t, str, "service/collector created")
			assert.Contains(t, str, "daemonset.apps/netobserv-cli created")
			assert.Contains(t, str, "pod/collector created")
			assert.Contains(t, str, "pod/collector condition met")
			// check that CLI is running
			assert.Contains(t, str, "Running network-observability-cli as Packet Capture")
			// check that script terminated
			assert.Contains(t, str, "command terminated")
			return ctx
		},
	).Assess("check downloaded output pcap files",
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var pcaps []string

			dirPath := path.Join("output", "pcap")
			assert.True(t, dirExists(dirPath), "directory %s not found", dirPath)
			err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					fmt.Println(err)
				}

				if !info.IsDir() {
					if filepath.Ext(path) == ".pcapng" {
						pcaps = append(pcaps, path)
					}
				}

				return nil
			})
			assert.Nil(t, err)

			// check pcap file
			assert.Equal(t, 1, len(pcaps))
			pcapBytes, err := os.ReadFile(pcaps[0])
			assert.Nil(t, err)

			// check pcap magic number
			assert.Equal(t, []byte{0x4d, 0x3c, 0x2b, 0x1a}, pcapBytes[8:12])

			return ctx
		},
	).Feature()
	testCluster.TestEnv().Test(t, f1)
}

func dirExists(dir string) bool {
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}
