package integrationtests

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("NetObserv CLI e2e integration test suite", g.Serial, func() {
	cliNS := "netobserv-cli"

	g.It("OCP-73458: Verify all CLI pods are deployed", g.Label("Sanity"), func() {
		cliArgs := []string{"flows", "--copy=false"}
		cmd := exec.Command(ocNetObservBinPath, cliArgs...)
		err := cmd.Start()
		o.Expect(err).NotTo(o.HaveOccurred())
		// cleanup()
		defer func() {
			cliArgs := []string{"cleanup"}
			cmd := exec.Command(ocNetObservBinPath, cliArgs...)
			err := cmd.Run()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		var allPods []string
		clientset, err := getNewClient()
		o.Expect(err).NotTo(o.HaveOccurred())
		nodes, err := getClusterNodes(clientset, &metav1.ListOptions{})
		// agent pods + collector pods
		totalExpectedPods := len(nodes) + 1
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = isCLIReady(clientset, cliNS)
		o.Expect(err).NotTo(o.HaveOccurred(), "CLI didn't come ready")
		allPods, err = getNamespacePods(clientset, cliNS, &metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(allPods).To(o.HaveLen(totalExpectedPods), fmt.Sprintf("Number of CLI pods are not as expected %d", totalExpectedPods))
	})

	g.It("OCP-73458: Verify regexes filters are applied", g.Label("Regexes"), func() {
		// capture upto 500KB
		nsfilter := "openshift-monitoring"
		cliArgs := []string{"flows", fmt.Sprintf("--query=SrcK8S_Namespace=~\"%s\"", nsfilter), "--copy=true", "--max-bytes=500000", "--max-time=1m"}
		cmd := exec.Command(ocNetObservBinPath, cliArgs...)
		err := cmd.Run()
		o.Expect(err).NotTo(o.HaveOccurred())
		// cleanup()
		defer func() {
			cliArgs := []string{"cleanup"}
			cmd := exec.Command(ocNetObservBinPath, cliArgs...)
			err := cmd.Run()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		o.Expect(cmd.ProcessState.ExitCode()).To(o.BeNumerically("==", 0), "oc-netobserv returned non-zero exit code")
		flowsJsonfile, err := getFlowsJSONFile()
		o.Expect(err).NotTo(o.HaveOccurred())
		flowsFile, err := os.Open(flowsJsonfile)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer flowsFile.Close()
		decoder := json.NewDecoder(flowsFile)
		_, err = decoder.Token()
		o.Expect(err).NotTo(o.HaveOccurred())
		var flow struct {
			SrcK8sNamespace string `json:"SrcK8S_Namespace"`
		}
		for decoder.More() {
			err := decoder.Decode(&flow)
			o.Expect(err).NotTo(o.HaveOccurred())
			if flow.SrcK8sNamespace != nsfilter {
				o.Expect(true).To(o.BeFalse(), fmt.Sprintf("Flow %v does not meet regexes condition SrcK8S_Namespace=~%s", flow, nsfilter))
			}
		}
	})

	g.It("OCP-82597: Verify sampling value of 1 is applied in captured flows", g.Label("Sampling"), func() {
		// capture upto 500KB with sampling=1
		cliArgs := []string{"flows", "--sampling=1", "--copy=true", "--max-bytes=500000", "--max-time=1m"}
		cmd := exec.Command(ocNetObservBinPath, cliArgs...)
		err := cmd.Run()
		o.Expect(err).NotTo(o.HaveOccurred())
		// cleanup()
		defer func() {
			cliArgs := []string{"cleanup"}
			cmd := exec.Command(ocNetObservBinPath, cliArgs...)
			err := cmd.Run()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		o.Expect(cmd.ProcessState.ExitCode()).To(o.BeNumerically("==", 0), "oc-netobserv returned non-zero exit code")
		flowsJsonfile, err := getFlowsJSONFile()
		o.Expect(err).NotTo(o.HaveOccurred())
		flowsFile, err := os.Open(flowsJsonfile)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer flowsFile.Close()
		decoder := json.NewDecoder(flowsFile)
		_, err = decoder.Token()
		o.Expect(err).NotTo(o.HaveOccurred())
		var flow struct {
			Sampling float64 `json:"Sampling"`
		}
		atLeastOneFlowFound := false
		for decoder.More() {
			err := decoder.Decode(&flow)
			o.Expect(err).NotTo(o.HaveOccurred())
			atLeastOneFlowFound = true
			// Verify sampling value is 1
			o.Expect(flow.Sampling).To(o.BeNumerically("==", 1), fmt.Sprintf("Flow sampling value should be 1, but got %v", flow.Sampling))
		}
		o.Expect(atLeastOneFlowFound).To(o.BeTrue(), "At least one flow should be captured to verify sampling value")
	})

	g.It("OCP-82597: Verify excluded interface genev_sys_6081 does not appear in captured flows", g.Label("ExcludeInterface"), func() {
		// capture upto 500KB with exclude_interfaces=genev_sys_6081
		cliArgs := []string{"flows", "--exclude_interfaces=genev_sys_6081", "--copy=true", "--max-bytes=500000", "--max-time=1m"}
		cmd := exec.Command(ocNetObservBinPath, cliArgs...)
		err := cmd.Run()
		o.Expect(err).NotTo(o.HaveOccurred())
		// cleanup()
		defer func() {
			cliArgs := []string{"cleanup"}
			cmd := exec.Command(ocNetObservBinPath, cliArgs...)
			err := cmd.Run()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		o.Expect(cmd.ProcessState.ExitCode()).To(o.BeNumerically("==", 0), "oc-netobserv returned non-zero exit code")
		flowsJsonfile, err := getFlowsJSONFile()
		o.Expect(err).NotTo(o.HaveOccurred())
		flowsFile, err := os.Open(flowsJsonfile)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer flowsFile.Close()
		decoder := json.NewDecoder(flowsFile)
		_, err = decoder.Token()
		o.Expect(err).NotTo(o.HaveOccurred())
		var flow struct {
			Interfaces []string `json:"Interfaces"`
		}
		for decoder.More() {
			err := decoder.Decode(&flow)
			o.Expect(err).NotTo(o.HaveOccurred())
			// Verify none of the flows contain genev_sys_6081
			for _, iface := range flow.Interfaces {
				o.Expect(iface).NotTo(o.Equal("genev_sys_6081"), fmt.Sprintf("Flow should not contain excluded interface genev_sys_6081, but found it in interfaces: %v", flow.Interfaces))
			}
		}
	})

	g.It("OCP-82598: Verify metrics command creates dashboard configmap and metrics are scraped", g.Label("Metrics"), func() {
		// Run metrics command
		cliArgs := []string{"metrics"}
		cmd := exec.Command(ocNetObservBinPath, cliArgs...)
		err := cmd.Run()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cmd.ProcessState.ExitCode()).To(o.BeNumerically("==", 0), "oc-netobserv metrics command returned non-zero exit code")

		// cleanup()
		defer func() {
			cliArgs := []string{"cleanup"}
			cmd := exec.Command(ocNetObservBinPath, cliArgs...)
			err := cmd.Run()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		clientset, err := getNewClient()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Wait for CLI to be ready
		daemonsetReady, err := isDaemonsetReady(clientset, "netobserv-cli", cliNS)
		o.Expect(err).NotTo(o.HaveOccurred(), "agent daemonset didn't come ready")
		o.Expect(daemonsetReady).To(o.BeTrue(), "agent daemonset didn't come ready")

		// Check that dashboard configmap is created
		dashboardCM, err := getConfigMap(clientset, "netobserv-cli", "openshift-config-managed")
		o.Expect(err).NotTo(o.HaveOccurred(), "Dashboard configmap should be created in openshift-config-managed namespace")
		o.Expect(dashboardCM.Name).To(o.Equal("netobserv-cli"), "Dashboard configmap should be named netobserv-cli")

		// Check that metrics are being scraped by Prometheus
		g.By("Verifying metrics are scraped by Prometheus")
		prometheusQuery := `sum(rate(on_demand_netobserv_node_egress_bytes_total[2m]))`
		metricValue, err := queryPrometheusMetric(clientset, prometheusQuery)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Failed to query Prometheus for metrics: %v", err))
		o.Expect(metricValue).To(o.BeNumerically(">=", 0), fmt.Sprintf("Prometheus should return a valid metric value, but got %v", metricValue))
	})
})
