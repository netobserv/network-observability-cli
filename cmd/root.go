package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	maxRefreshRate = 100 * time.Millisecond
)

var (
	log      = logrus.New()
	logLevel string
	ports    []int
	nodes    []string
	filter   string

	currentTime = func() time.Time {
		return time.Now()
	}
	startupTime = currentTime()
	lastRefresh = startupTime
	mutex       = sync.Mutex{}

	resetTerminal = func() {
		// clear terminal to render table properly
		fmt.Print("\x1bc")
		// no wrap
		fmt.Print("\033[?7l")
	}

	rootCmd = &cobra.Command{
		Use:   "network-observability-cli",
		Short: "network-observability-cli is an interactive Flow and Packet visualizer",
		Long:  `An interactive Flow / PCAP collector and visualization tool`,
		Run: func(cmd *cobra.Command, args []string) {
		},
	}

	outputBuffer  *bytes.Buffer
	stopReceived  = false
	keyboardError = ""
)

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

// func main() {
func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVarP(&logLevel, "loglevel", "l", "info", "Log level")
	rootCmd.PersistentFlags().IntSliceVarP(&ports, "ports", "", []int{9999}, "TCP ports to listen")
	rootCmd.PersistentFlags().StringSliceVarP(&nodes, "nodes", "", []string{""}, "Node names per port (optionnal)")
	rootCmd.PersistentFlags().StringVarP(&filter, "filter", "", "", "Filter(s)")

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM)
	go func() {
		<-c
		log.Info("Received SIGTERM; cleaning up...")
		stopReceived = true

		os.Exit(1)
	}()

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

	log.Infof("Running network-observability-cli\nLog level: %s\nFilter(s): %s", logLevel, filter)
}
