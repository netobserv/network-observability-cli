package cmd

import (
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	maxRefreshRate = 100 * time.Millisecond
)

var (
	log       = logrus.New()
	logLevel  string
	host      string
	nodes     []string
	ports     []int
	addresses []string
	filter    string

	startupTime = time.Now()
	lastRefresh = startupTime
	mutex       = sync.Mutex{}

	rootCmd = &cobra.Command{
		Use:   "network-observability-cli",
		Short: "network-observability-cli is an interactive Flow and Packet visualizer",
		Long:  `An interactive Flow / PCAP collector and visualization tool`,
		Run: func(cmd *cobra.Command, args []string) {
		},
	}
)

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

// func main() {
func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVarP(&logLevel, "loglevel", "l", "info", "Log level")
	rootCmd.PersistentFlags().StringVarP(&host, "host", "", "localhost", "Agent IP")
	rootCmd.PersistentFlags().StringSliceVarP(&nodes, "nodes", "", []string{"node"}, "node names to monitor")
	rootCmd.PersistentFlags().IntSliceVarP(&ports, "ports", "", []int{9999}, "TCP ports to listen")
	rootCmd.PersistentFlags().StringVarP(&filter, "filter", "", "", "Filter")

	// IPFIX flow
	rootCmd.AddCommand(flowCmd)

	// packet
	rootCmd.AddCommand(pktCmd)
}

func initConfig() {
	lvl, _ := logrus.ParseLevel(logLevel)
	log.SetLevel(lvl)

	if len(nodes) != len(ports) {
		log.Fatalf("specified nodes names doesn't match ports length")
	}

	for _, port := range ports {
		addresses = append(addresses, host+":"+fmt.Sprintf("%v", port))
	}
	log.Infof("Running network-observability-cli\nLog level: %s\nAddresses:\n%s\nFilter: %s", logLevel, strings.Join(addresses, "\n"), filter)
}

func CleanupCapture(c *net.TCPConn, f *os.File) {
	log.Printf("Stopping Capture")

	if c != nil {
		c.Close()
	}
	if f != nil {
		f.Close()
	}
}
