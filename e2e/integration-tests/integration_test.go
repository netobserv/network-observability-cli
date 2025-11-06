//go:build !e2e

package integrationtests

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/netobserv/network-observability-cli/e2e"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/sirupsen/logrus"
)

var (
	cliNS = "netobserv-cli"

	clientset    *kubernetes.Clientset
	StartupDate  = time.Now().Format("20060102-150405")
	lastFileName string
	ilog         = logrus.WithField("component", "integration_test")
)

func writeOutput(filename string, out string) {
	ilog.Debugf("Writing %s...", filename)

	// keep last filename written to be able to name the associated cleanup accordingly
	lastFileName = filename
	err := os.WriteFile(path.Join(os.Getenv("ARTIFACT_DIR"), filename), []byte(out), 0666)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error writing command output %v", err))
}

func cleanup() {
	ilog.Info("Cleaning up...")

	// run cli to cleanup namespace
	cliArgs := []string{"cleanup"}
	out, err := e2e.RunCommand(ilog, ocNetObservBinPath, cliArgs...)
	writeOutput(strings.Replace(lastFileName, "Output", "cleanupOutput", 1), out)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error during cleanup %v", err))

	// ensure namespace is fully removed before next lunch to avoid error
	deleted, err := isNamespace(clientset, cliNS, false)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Can't check if namespace was deleted %v", err))
	o.Expect(deleted).To(o.BeTrue())

	ilog.Debug("Cleaned up !")
}

// keep this spec ordered to name cleanup files according to command run ones
var _ = g.Describe("NetObserv CLI e2e integration test suite", g.Ordered, func() {

	g.BeforeAll(func() {
		c, err := getNewClient()
		o.Expect(err).NotTo(o.HaveOccurred())
		clientset = c
	})

	g.It("OCP-73458: Verify all CLI pods are deployed", g.Label("Sanity"), func() {
		g.DeferCleanup(func() {
			cleanup()
		})

		cliArgs := []string{"flows", "--copy=false"}
		out, err := e2e.StartCommand(ilog, ocNetObservBinPath, cliArgs...)
		writeOutput(StartupDate+"-flowOutput", out)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error starting command %v", err))

		nodes, err := getClusterNodes(clientset, &metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = isCLIRuning(clientset, cliNS)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("CLI didn't come ready %v", err))
		allPods, err := getNamespacePods(clientset, cliNS, &metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		// agent pods + collector pods
		totalExpectedPods := len(nodes) + 1
		o.Expect(allPods).To(o.HaveLen(totalExpectedPods), fmt.Sprintf("Number of CLI pods are not as expected %d", totalExpectedPods))
	})

	g.It("OCP-73458: Verify packet capture creates pcapng file and filters by port", g.Label("PacketCapture"), func() {
		g.DeferCleanup(func() {
			cleanup()
		})

		// Run packet capture with port 58 filter
		targetPort := uint16(8080)
		cliArgs := []string{"packets", "--port=8080", "--copy=true", "--max-bytes=100000000", "--max-time=1m"}
		out, err := e2e.RunCommand(ilog, ocNetObservBinPath, cliArgs...)
		writeOutput(StartupDate+"-packetOutput", out)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error running command %v", err))

		_, err = isCLIDone(clientset, cliNS)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("CLI didn't finish %v", err))

		// Verify pcapng file is created
		pcapngFile, err := getPcapngFile()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get pcapng file")
		o.Expect(pcapngFile).NotTo(o.BeEmpty(), "Pcapng file path should not be empty")

		ilog.Infof("==> Pcapng file created at: %s", pcapngFile)

		// Verify file exists and has content
		fileInfo, err := os.Stat(pcapngFile)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Pcapng file should exist at %s", pcapngFile))
		o.Expect(fileInfo.Size()).To(o.BeNumerically(">", 0), "Pcapng file should have content")

		ilog.Infof("==> Pcapng file size: %d bytes", fileInfo.Size())

		// Read and analyze packets from pcapng file
		packets, err := ReadPcapngFile(pcapngFile)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Failed to read pcapng file: %v", err))
		o.Expect(packets).NotTo(o.BeEmpty(), "Pcapng file should contain packets")

		ilog.Infof("Found %d total packets in pcapng file", len(packets))
		// Verify packets are filtered by port 58
		packetsOnPort58 := FilterPacketsByPort(packets, targetPort)
		o.Expect(packetsOnPort58).NotTo(o.BeEmpty(), fmt.Sprintf("Should have captured packets on port %d", targetPort))

		ilog.Infof("Found %d packets on port %d", len(packetsOnPort58), targetPort)
		// Get protocol distribution for analysis
		protocolDist := GetProtocolDistribution(packets)
		ilog.Infof("Protocol distribution: %v", protocolDist)
	})

	g.It("OCP-73458: Verify packet capture fails without filter", g.Label("PacketCapture", "NoFilter"), func() {
		g.DeferCleanup(func() {
			cleanup()
		})

		// Run packet capture without any filter - should show error message
		cliArgs := []string{"packets", "--copy=false", "--max-bytes=100000000", "--max-time=1m"}
		out, _ := e2e.RunCommand(ilog, ocNetObservBinPath, cliArgs...)
		writeOutput(StartupDate+"-packetNoFilterOutput", out)

		ilog.Infof("==> Command output: %s", out)

		// Verify error message contains expected text
		o.Expect(out).To(o.ContainSubstring("Error: At least one eBPF filter must be set"), "Output should contain eBPF filter requirement error")
		o.Expect(out).To(o.ContainSubstring("packet capture"), "Error should mention packet capture")
		o.Expect(out).To(o.ContainSubstring("high resource consumption"), "Error should mention resource consumption reason")
		o.Expect(out).To(o.Or(
			o.ContainSubstring("Use netobserv packets help"),
			o.ContainSubstring("help to list filters"),
		), "Error should suggest using help command")

		// Verify cleanup happened
		o.Expect(out).To(o.ContainSubstring("Cleaning up"), "Should perform cleanup after error")
		o.Expect(out).To(o.ContainSubstring("namespace \"netobserv-cli\" deleted"), "Should delete namespace during cleanup")
	})

	g.It("OCP-73458: Verify packet capture help lists all filters", g.Label("PacketCapture", "Help"), func() {
		// Run help command to list available filters
		cliArgs := []string{"packets", "help"}
		out, err := e2e.RunCommand(ilog, ocNetObservBinPath, cliArgs...)
		writeOutput(StartupDate+"-packetHelpOutput", out)
		o.Expect(err).NotTo(o.HaveOccurred(), "Help command should succeed")

		ilog.Infof("==> Help command output length: %d bytes", len(out))
		// Verify help output contains filter section
		o.Expect(out).To(o.ContainSubstring("filters:"), "Help should contain filters section")

		// Verify key filter options are listed
		expectedFilters := []string{
			"--port:",
			"--protocol:",
			"--action:",
			"--cidr:",
			"--direction:",
			"--dport:",
			"--sport:",
			"--peer_ip:",
			"--tcp_flags:",
			"--icmp_type:",
			"--icmp_code:",
			"--drops:",
			"--query:",
			"--node-selector:",
		}
		for _, filter := range expectedFilters {
			o.Expect(out).To(o.ContainSubstring(filter), fmt.Sprintf("Help should list filter: %s", filter))
		}

		// Verify options section exists
		o.Expect(out).To(o.ContainSubstring("options:"), "Help should contain options section")

		// Verify key options are listed
		expectedOptions := []string{
			"--background:",
			"--copy:",
			"--log-level:",
			"--max-time:",
			"--max-bytes:",
			"--yaml:",
		}
		for _, option := range expectedOptions {
			o.Expect(out).To(o.ContainSubstring(option), fmt.Sprintf("Help should list option: %s", option))
		}

		// Verify syntax information
		o.Expect(out).To(o.ContainSubstring("Syntax:"), "Help should show syntax")
		o.Expect(out).To(o.ContainSubstring("netobserv packets"), "Help should show command syntax")
	})

	g.It("OCP-73458: Verify packet capture with TCP protocol filter", g.Label("PacketCapture", "TCP"), func() {
		g.DeferCleanup(func() {
			cleanup()
		})

		// Run packet capture with TCP protocol filter
		targetProtocol := "TCP"
		cliArgs := []string{"packets", "--protocol=TCP", "--copy=true", "--max-bytes=100000000", "--max-time=1m"}
		out, err := e2e.RunCommand(ilog, ocNetObservBinPath, cliArgs...)
		writeOutput(StartupDate+"-packetTcpOutput", out)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error running command %v", err))

		_, err = isCLIDone(clientset, cliNS)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("CLI didn't finish %v", err))

		// Verify pcapng file is created
		pcapngFile, err := getPcapngFile()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get pcapng file")
		o.Expect(pcapngFile).NotTo(o.BeEmpty(), "Pcapng file path should not be empty")

		ilog.Infof("==> Pcapng file created at: %s", pcapngFile)

		// Verify file exists and has content
		fileInfo, err := os.Stat(pcapngFile)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Pcapng file should exist at %s", pcapngFile))
		o.Expect(fileInfo.Size()).To(o.BeNumerically(">", 0), "Pcapng file should have content")

		// Read and analyze packets from pcapng file
		packets, err := ReadPcapngFile(pcapngFile)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Failed to read pcapng file: %v", err))
		o.Expect(packets).NotTo(o.BeEmpty(), "Pcapng file should contain packets")

		ilog.Infof("Found %d total packets in pcapng file", len(packets))

		// Verify packets are filtered by TCP protocol
		tcpPackets := FilterPacketsByProtocol(packets, targetProtocol)
		o.Expect(tcpPackets).NotTo(o.BeEmpty(), fmt.Sprintf("Should have captured %s packets", targetProtocol))

		ilog.Infof("Found %d %s packets", len(tcpPackets), targetProtocol)

		// Get protocol distribution
		protocolDist := GetProtocolDistribution(packets)
		ilog.Infof("Protocol distribution: %v", protocolDist)

		// Verify all packets are TCP
		for _, p := range packets {
			o.Expect(p.Protocol).To(o.Equal(targetProtocol), fmt.Sprintf("All packets should be %s, found %s", targetProtocol, p.Protocol))
		}
	})

	g.It("OCP-73458: Verify regexes filters are applied", g.Label("Regexes"), func() {
		g.DeferCleanup(func() {
			cleanup()
		})

		// capture upto 500KB
		nsfilter := "openshift-monitoring"
		cliArgs := []string{"flows", fmt.Sprintf("--query=SrcK8S_Namespace=~\"%s\"", nsfilter), "--copy=true", "--max-bytes=500000", "--max-time=1m"}
		out, err := e2e.RunCommand(ilog, ocNetObservBinPath, cliArgs...)
		writeOutput(StartupDate+"-flowQueryOutput", out)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error running command %v", err))

		_, err = isCLIDone(clientset, cliNS)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("CLI didn't finished %v", err))

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

		found := false
		for decoder.More() {
			found = true
			err := decoder.Decode(&flow)
			o.Expect(err).NotTo(o.HaveOccurred())
			if flow.SrcK8sNamespace != nsfilter {
				o.Expect(true).To(o.BeFalse(), fmt.Sprintf("Flow %v does not meet regexes condition SrcK8S_Namespace=~%s", flow, nsfilter))
			}
		}
		o.Expect(found).To(o.BeTrue(), fmt.Sprintf("Didn't found any flow matching SrcK8S_Namespace=~%s", nsfilter))
	})
	g.Describe("OCP-84801: Verify CLI runs under correct privileges", g.Label("Privileges"), func() {

		tests := []struct {
			when    string
			it      string
			cliArgs []string
			matcher types.GomegaMatcher
		}{
			{
				when:    "Executing `oc netobserv flows`",
				it:      "does not run as privileged",
				cliArgs: []string{"flows"},
				matcher: o.BeFalse(),
			},
			{
				when:    "Executing `oc netobserv flows --privileged=true`",
				it:      "runs as privileged",
				cliArgs: []string{"flows", "--privileged=true"},
				matcher: o.BeTrue(),
			},

			{
				when:    "Executing `oc netobserv flows --drops`",
				it:      "runs as privileged",
				cliArgs: []string{"flows", "--drops"},
				matcher: o.BeTrue(),
			},
		}

		for _, t := range tests {
			g.When(t.when, func() {
				g.It(t.it, func() {
					g.DeferCleanup(func() {
						cleanup()
					})
					// run command async until done
					out, err := e2e.StartCommand(ilog, ocNetObservBinPath, t.cliArgs...)
					writeOutput(StartupDate+"-flowOutput", out)
					o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error starting command %v", err))

					// Wait for CLI to be ready
					daemonsetReady, err := isDaemonsetReady(clientset, "netobserv-cli", cliNS)
					o.Expect(err).NotTo(o.HaveOccurred(), "agent daemonset didn't come ready")
					o.Expect(daemonsetReady).To(o.BeTrue(), "agent daemonset didn't come ready")

					// Verify correct privilege setting
					ds, err := getDaemonSet(clientset, "netobserv-cli", cliNS)
					o.Expect(err).NotTo(o.HaveOccurred(), "DeamonSet should be created in CLI namespace")
					containers := ds.Spec.Template.Spec.Containers
					o.Expect(len(containers)).To(o.Equal(1), "The number of containers specified in the template is != 1")
					o.Expect(containers[0].SecurityContext.Privileged).To(o.HaveValue(t.matcher), "Priviledged is not set to true")
				})
			})

		}
	})
	g.It("OCP-82597: Verify sampling value of 1 is applied in captured flows", g.Label("Sampling"), func() {
		g.DeferCleanup(func() {
			cleanup()
		})

		// capture upto 500KB with sampling=1
		cliArgs := []string{"flows", "--sampling=1", "--copy=true", "--max-bytes=500000", "--max-time=1m"}
		out, err := e2e.RunCommand(ilog, ocNetObservBinPath, cliArgs...)
		writeOutput(StartupDate+"-flowSamplingOutput", out)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error running command %v", err))

		_, err = isCLIDone(clientset, cliNS)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("CLI didn't finished %v", err))

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
		g.DeferCleanup(func() {
			cleanup()
		})

		// capture upto 500KB with exclude_interfaces=genev_sys_6081
		cliArgs := []string{"flows", "--exclude_interfaces=genev_sys_6081", "--copy=true", "--max-bytes=500000", "--max-time=1m"}
		out, err := e2e.RunCommand(ilog, ocNetObservBinPath, cliArgs...)
		writeOutput(StartupDate+"-flowInterfacesOutput", out)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error running command %v", err))

		_, err = isCLIDone(clientset, cliNS)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("CLI didn't finished %v", err))

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
		g.DeferCleanup(func() {
			cleanup()
		})

		// Run metrics command
		cliArgs := []string{"metrics", "--background"}
		out, err := e2e.StartCommand(ilog, ocNetObservBinPath, cliArgs...)
		writeOutput(StartupDate+"-metricsOutput", out)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error starting command %v", err))

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
