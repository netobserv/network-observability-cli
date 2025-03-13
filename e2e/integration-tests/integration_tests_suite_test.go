package integrationtests

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
)

var ocNetObservBinPath string

func TestIntegrationTests(t *testing.T) {
	o.RegisterFailHandler(g.Fail)
	g.RunSpecs(t, "IntegrationTests Suite")
}

var _ = g.BeforeSuite(func() {
	//  kubeconfig env var and see if the cluster is reachable.
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig == "" {
		g.Skip("Set KUBECONFIG env variable")
	}

	cmd := exec.Command("which", "oc-netobserv")
	out, err := cmd.Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	ocNetObservBinPath = strings.TrimSuffix(string(out), "\n")
})
