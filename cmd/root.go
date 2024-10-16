package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
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
	maxTime  time.Duration
	maxBytes int64

	currentTime = func() time.Time {
		return time.Now()
	}
	startupTime  = currentTime()
	lastRefresh  = startupTime
	totalBytes   = int64(0)
	totalPackets = uint32(0)

	mutex = sync.Mutex{}

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

	captureType      = "Flow"
	outputBuffer     *bytes.Buffer
	collectorStarted = false
	captureStarted   = false
	stopReceived     = false
	useMocks         = false
	keyboardError    = ""
)

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

// func main() {
func init() {
	cobra.OnInitialize(onInit)
	rootCmd.PersistentFlags().StringVarP(&logLevel, "loglevel", "l", "info", "Log level")
	rootCmd.PersistentFlags().IntSliceVarP(&ports, "ports", "", []int{9999}, "TCP ports to listen")
	rootCmd.PersistentFlags().StringSliceVarP(&nodes, "nodes", "", []string{""}, "Node names per port (optionnal)")
	rootCmd.PersistentFlags().StringVarP(&filter, "filter", "", "", "Filter(s)")
	rootCmd.PersistentFlags().DurationVarP(&maxTime, "maxtime", "", 5*time.Minute, "Maximum capture time")
	rootCmd.PersistentFlags().Int64VarP(&maxBytes, "maxbytes", "", 50000000, "Maximum capture bytes")
	rootCmd.PersistentFlags().BoolVarP(&useMocks, "mock", "", false, "Use mock")

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM)
	go func() {
		<-c
		log.Info("Received SIGTERM; cleaning up...")
		stopReceived = true

		os.Exit(0)
	}()

	// IPFIX flow
	rootCmd.AddCommand(flowCmd)

	// packet
	rootCmd.AddCommand(pktCmd)
}

func onInit() {
	lvl, _ := logrus.ParseLevel(logLevel)
	log.SetLevel(lvl)

	if len(nodes) != len(ports) {
		log.Fatalf("specified nodes names doesn't match ports length")
	}

	log.Infof("Running network-observability-cli\nLog level: %s\nFilter(s): %s", logLevel, filter)
	showKernelVersion()

	if useMocks {
		log.Info("Using mocks...")
		go MockForever()
	}
}

func showKernelVersion() {
	output, err := exec.Command("uname", "-r").Output()
	if err != nil {
		log.Errorf("Can't get kernel version: %v", err)
	}
	if len(output) == 0 {
		log.Infof("Kernel version not found")
	} else {
		log.Infof("Kernel version: %s", strings.TrimSpace(string(output)))
	}
}
