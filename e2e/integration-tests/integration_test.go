package integration_tests

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

	g.BeforeEach(func() {
		// cleanup before each tests
		cliArgs := []string{"cleanup"}
		cmd := exec.Command(ocNetObservBinPath, cliArgs...)
		err := cmd.Run()
		if err != nil {
			log.Errorf("%v resulted in error %v, %v", cmd, cmd.Stderr, err)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("Verify all CLI pods are deployed", g.Label("Sanity"), func() {
		cliArgs := []string{"flows", "--copy=false"}
		cmd := exec.Command(ocNetObservBinPath, cliArgs...)
		err := cmd.Start()
		o.Expect(err).NotTo(o.HaveOccurred())
		clientset, err := getNewClient()
		o.Expect(err).NotTo(o.HaveOccurred())
		var allPods []string
		nodes, err := getClusterNodes(clientset, metav1.ListOptions{})
		// agent pods + collector pods
		totalExpectedPods := len(nodes) + 1
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = isCLIReady(clientset, cliNS)
		o.Expect(err).NotTo(o.HaveOccurred(), "CLI didn't come ready")
		allPods, err = getNamespacePods(clientset, cliNS, metav1.ListOptions{})
		o.Expect(len(allPods)).To(o.BeNumerically("==", totalExpectedPods), fmt.Sprintf("Number of CLI pods are not as expected %d", totalExpectedPods))
	})

	g.It("Verify regexes filters are applied", func() {
		// capture upto 500KB
		nsfilter := "openshift-monitoring"
		cliArgs := []string{"flows", "--regexes=SrcK8S_Namespace~" + nsfilter, "--copy=true", "--max-bytes=500000"}
		cmd := exec.Command(ocNetObservBinPath, cliArgs...)
		err := cmd.Run()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cmd.ProcessState.ExitCode()).To(o.BeNumerically("==", 0), "oc-netobserv returned non-zero exit code")
		flowsJsonfile, err := getFlowsJSONFile()
		o.Expect(err).NotTo(o.HaveOccurred())
		flowsFile, err := os.Open(flowsJsonfile)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer flowsFile.Close()
		decoder := json.NewDecoder(flowsFile)
		decoder.Token()
		var flow Flowlog
		for decoder.More() {
			err := decoder.Decode(&flow)
			o.Expect(err).NotTo(o.HaveOccurred())
			if flow.SrcK8S_Namespace != nsfilter {
				o.Expect(true).To(o.Equal(false), fmt.Sprintf("Flow %v does not meet regexes condition SrcK8S_Namespace~%s", flow, nsfilter))
			}
		}
	})
})
