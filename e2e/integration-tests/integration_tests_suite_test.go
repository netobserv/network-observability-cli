//go:build int

package integrationtests

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

var (
	ocNetObservBinPath string
	itlog              = logrus.WithField("component", "integration_test_suite_test")
)

func TestIntegrationTests(t *testing.T) {
	o.RegisterFailHandler(g.Fail)
	g.RunSpecs(t, "IntegrationTests Suite")
}

var _ = g.BeforeSuite(func() {
	//  kubeconfig env var and see if the cluster is reachable.
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig == "" {
		itlog.Errorf("error, KUBECONFIG is: %v", kubeconfig)
		g.Skip("Set KUBECONFIG env variable")
	}

	// Set ARTIFACT_DIR env var to output directory if not set
	if artifactDir := os.Getenv("ARTIFACT_DIR"); artifactDir == "" {
		os.Setenv("ARTIFACT_DIR", "output")
	}
	err := os.MkdirAll(os.Getenv("ARTIFACT_DIR"), 0700)
	o.Expect(err).NotTo(o.HaveOccurred())

	cmd := exec.Command("which", "oc-netobserv")
	out, err := cmd.Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	ocNetObservBinPath = strings.TrimSuffix(string(out), "\n")
})
