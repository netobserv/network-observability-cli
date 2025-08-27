package cmd

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/spf13/cobra"
)

var metricCmd = &cobra.Command{
	Use:   "get-metrics",
	Short: "",
	Long:  "",
	Run:   runMetricCapture,
}

var (
	client           *api.Client
	selectedDuration = 0
	durations        = []string{"5m", "10m", "30m", "1h", "6h"}
	metricCounts     = map[string]int{
		"5m":  60,
		"10m": 70,
		"30m": 80,
		"1h":  90,
		"6h":  100,
	}
)

func runMetricCapture(c *cobra.Command, _ []string) {
	capture = Metric
	go startMetricCollector(c.Context())
	createMetricDisplay()
}

func startMetricCollector(ctx context.Context) {
	cl, err := newClient(
		time.Duration(30*time.Second),
		false,
		"/var/run/secrets/kubernetes.io/serviceaccount/service-ca.crt",
		"/var/run/secrets/kubernetes.io/serviceaccount/token",
		"https://thanos-querier.openshift-monitoring.svc:9091/",
	)
	if err != nil {
		log.Errorf("Error creating client: %v", err.Error())
		log.Fatal(err)
	}

	// save client to be able to call queries from display
	client = &cl
	log.Debug("Created client")

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	log.Trace("Ready ! Querying metrics...")
	for ; true; <-ticker.C {
		if stopReceived {
			log.Debug("Stop received")
			return
		}

		// run query on tick
		queryGraphs(ctx, cl)

		// terminate capture if max time reached
		now := currentTime()
		duration := now.Sub(startupTime)
		if int(duration) > int(maxTime) {
			if exit := onLimitReached(); exit {
				log.Infof("Capture reached %s, exiting now...", maxTime)
				return
			}
		}

		captureStarted = true
	}
}

func queryGraphs(ctx context.Context, client api.Client) {
	for index := range graphs {
		go queryGraph(ctx, client, index)
	}
}

func queryGraph(ctx context.Context, client api.Client, index int) {
	query, result := queryProm(ctx, client, graphs[index].Query.PromQL)
	if errAdvancedDisplay != nil {
		// simply print metrics into logs
		log.Printf("%v\n", result)
	} else {
		appendMetrics(query, result, index)
	}
}

func queryProm(ctx context.Context, client api.Client, promQL string) (*Query, *Matrix) {
	now := currentTime()

	ran, err := time.ParseDuration(durations[selectedDuration])
	if err != nil {
		log.Fatal(err)
	}
	end := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), 0, 0, now.Location())
	start := end.Add(-ran)
	step := time.Duration(ran.Nanoseconds() / int64(showCount))

	// update query with start / end / step
	query := Query{
		Range: v1.Range{
			Start: start,
			End:   end,
			Step:  step,
		},
		PromQL: promQL,
	}
	response, err := queryMatrix(ctx, client, &query)
	if err != nil {
		log.Error(err)
		return &query, nil
	}

	matrix := response.Data.Result.(Matrix)
	return &query, &matrix
}
