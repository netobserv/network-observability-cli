package cmd

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/spf13/cobra"
)

var metricCmd = &cobra.Command{
	Use:   "get-metrics",
	Short: "",
	Long:  "",
	Run:   runMetricCapture,
}

func runMetricCapture(_ *cobra.Command, _ []string) {
	captureType = "Metric"

	// TODO: implement a UI for metrics using tview
	// https://github.com/netobserv/network-observability-cli/pull/215

	ticker := time.NewTicker(time.Second)
	for range ticker.C {
		// terminate capture if max time reached
		now := currentTime()
		duration := now.Sub(startupTime)
		if int(duration) > int(maxTime) {
			log.Infof("Capture reached %s, exiting now...", maxTime)
			out, err := exec.Command("/oc-netobserv", "stop").Output()
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("%s", out)
			fmt.Print(`Thank you for using...`)
			printBanner()
			fmt.Print(`

  - Open NetObserv / On Demand dashboard to see generated metrics

	- Once finished, remove everything using 'oc netobserv cleanup'

                                                      See you soon !
																											
																											
		`)
			return
		}
	}
}
