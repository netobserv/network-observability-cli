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
	options  string
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
	captureEnded     = false
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
	rootCmd.PersistentFlags().StringVarP(&options, "options", "", "", "Options(s)")
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

	printBanner()
	log.Infof("Log level: %s\nOption(s): %s", logLevel, options)
	showKernelVersion()

	if useMocks {
		log.Info("Using mocks...")
		go MockForever()
	}
}

func printBanner() {
	fmt.Print(`
------------------------------------------------------------------------
         _  _     _       _                       ___ _    ___
        | \| |___| |_ ___| |__ ___ ___ _ ___ __  / __| |  |_ _|
        | .' / -_)  _/ _ \ '_ (_-</ -_) '_\ V / | (__| |__ | | 
        |_|\_\___|\__\___/_.__/__/\___|_|  \_/   \___|____|___|

------------------------------------------------------------------------`)
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

func onLimitReached() bool {
	shouldExit := false
	if !captureEnded {
		if strings.Contains(options, "background=true") {
			captureEnded = true
			resetTerminal()
			out, err := exec.Command("/oc-netobserv", "stop").Output()
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("%s", out)
			fmt.Print(`Thank you for using...`)
			printBanner()
			fmt.Print(`

	- Download the generated output using 'oc netobserv copy' command

	- Once finished, clean the collector pod using 'oc netobserv cleanup'

                                                      See you soon !
																											
																											
		`)
		} else {
			shouldExit = true
		}
	}

	return shouldExit
}
