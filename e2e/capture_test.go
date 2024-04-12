//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"syscall"
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
	commandTimeout    = 30 * time.Second
	namespace         = "default"
)

var (
	startupDate = time.Now().Format("20060102-150405")
)

var (
	testCluster *cluster.Kind
	log         = logrus.WithField("component", "capture_test")
)

func TestMain(m *testing.M) {
	if os.Getenv("ACTIONS_RUNNER_DEBUG") == "true" {
		logrus.StandardLogger().SetLevel(logrus.DebugLevel)
	}
	testCluster = cluster.NewKind(
		clusterNamePrefix+startupDate,
		path.Join(".."),
	)
	testCluster.Run(m)
}

func TestFlowCapture(t *testing.T) {
	f1 := features.New("flow capture").Setup(
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			output, err := runCommand("oc-netobserv-flows")
			// TODO: find a way to avoid error here; this is probably related to SIGTERM instead of CTRL + C call
			//assert.Nil(t, err)

			err = os.WriteFile(path.Join("output", startupDate+"-flowOutput"), output, 0666)
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
			assert.Contains(t, str, "SrcName")
			assert.Contains(t, str, "SrcType")
			assert.Contains(t, str, "DstName")
			assert.Contains(t, str, "DstType")
			// check that script terminated
			assert.Contains(t, str, "command terminated")
			return ctx
		},
	).Assess("check downloaded output flow files",
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var jsons []string
			var dbs []string

			assert.Equal(t, true, outputDirExists())
			dirPath := path.Join("output", "flow")
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
			output, err := runCommand("oc-netobserv-packets", "tcp,443")
			// TODO: find a way to avoid error here; this is probably related to SIGTERM instead of CTRL + C call
			//assert.Nil(t, err)

			err = os.WriteFile(path.Join("output", startupDate+"-packetOutput"), output, 0666)
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

			assert.Equal(t, true, outputDirExists())
			dirPath := path.Join("output", "pcap")
			err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					fmt.Println(err)
				}

				if !info.IsDir() {
					if filepath.Ext(path) == ".pcap" {
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
			assert.Equal(t, uint8(0xd4), pcapBytes[0])
			assert.Equal(t, uint8(0xc3), pcapBytes[1])
			assert.Equal(t, uint8(0xb2), pcapBytes[2])
			assert.Equal(t, uint8(0xa1), pcapBytes[3])

			return ctx
		},
	).Feature()
	testCluster.TestEnv().Test(t, f1)
}

// run command with tty support
func runCommand(commandName string, arg ...string) ([]byte, error) {
	cmdStr := path.Join("commands", commandName)
	log.WithFields(logrus.Fields{"cmd": cmdStr, "arg": arg}).Info("running command")

	log.Print("Executing command...")
	cmd := exec.Command(cmdStr, arg...)

	timer := time.AfterFunc(commandTimeout, func() {
		log.Print("Terminating command...")
		cmd.Process.Signal(syscall.SIGTERM)
	})
	defer timer.Stop()

	return cmd.CombinedOutput()
}

func outputDirExists() bool {
	if _, err := os.Stat("output"); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}
