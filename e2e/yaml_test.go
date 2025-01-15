//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

var (
	ylog = logrus.WithField("component", "yaml_test")
)

func TestMetricYAML(t *testing.T) {
	f1 := features.New("metric yaml").Setup(
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// use yaml output here as kind can't manage ServiceMonitor CR
			output, err := RunCommand(ylog, "oc-netobserv", "metrics", "--yaml")
			assert.Nil(t, err)

			err = os.WriteFile(path.Join("output", StartupDate+"-metricOutput"), output, 0666)
			assert.Nil(t, err)

			str := string(output)
			assert.NotEmpty(t, str)
			// ensure script setup is fine
			assert.Contains(t, str, "creating netobserv-cli namespace")
			assert.Contains(t, str, "creating service account")
			assert.Contains(t, str, "creating service monitor")
			assert.Contains(t, str, "creating metric-capture agents")
			// check CLI done successfully
			assert.Contains(t, str, "Check generated YAML file in output folder.")

			return ctx
		},
	).Assess("check generated yaml output",
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var yamls []string

			dirPath := "output"
			assert.True(t, dirExists(dirPath), "directory %s not found", dirPath)
			err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					fmt.Println(err)
				}

				if !info.IsDir() {
					if filepath.Ext(path) == ".yml" {
						yamls = append(yamls, path)
					}
				}

				return nil
			})
			assert.Nil(t, err)

			// check yaml file
			assert.Equal(t, 1, len(yamls))
			yamlBytes, err := os.ReadFile(yamls[0])
			assert.Nil(t, err)

			// check yamls parts
			yamlStr := string(yamlBytes[:])
			yamls = strings.Split(yamlStr, "---")
			assert.Equal(t, 8, len(yamls))

			// check yaml contents
			assert.Contains(t, yamls[0], "kind: Namespace")
			assert.Contains(t, yamls[0], "name: \"netobserv-cli\"")
			assert.Contains(t, Normalize(yamls[0]), Normalize("labels: app: netobserv pod-security.kubernetes.io/enforce: privileged pod-security.kubernetes.io/audit: privileged openshift.io/cluster-monitoring: \"true\""))

			assert.Contains(t, yamls[1], "kind: ServiceAccount")
			assert.Contains(t, yamls[1], "name: netobserv-cli")
			assert.Contains(t, yamls[1], "namespace: \"netobserv-cli\"")

			assert.Contains(t, yamls[2], "kind: ClusterRole")
			assert.Contains(t, yamls[2], "name: netobserv-cli")
			assert.Contains(t, yamls[2], "namespace: \"netobserv-cli\"")
			assert.Contains(t, Normalize(yamls[2]), Normalize("- apiGroups: - security.openshift.io resourceNames: - privileged resources: - securitycontextconstraints verbs: - use"))
			assert.Contains(t, Normalize(yamls[2]), Normalize("- apiGroups: - apps resources: - daemonsets verbs: - list - get - watch - delete"))
			assert.Contains(t, Normalize(yamls[2]), Normalize("- apiGroups: - resources: - pods - services - nodes verbs: - list - get - watch"))
			assert.Contains(t, Normalize(yamls[2]), Normalize("- apiGroups: - apps resources: - replicasets verbs: - list - get - watch"))

			assert.Contains(t, yamls[3], "kind: ClusterRoleBinding")
			assert.Contains(t, yamls[3], "name: netobserv-cli")
			assert.Contains(t, yamls[3], "namespace: \"netobserv-cli\"")
			assert.Contains(t, Normalize(yamls[3]), Normalize("subjects: - kind: ServiceAccount name: netobserv-cli namespace: \"netobserv-cli\""))
			assert.Contains(t, Normalize(yamls[3]), Normalize("roleRef: apiGroup: rbac.authorization.k8s.io kind: ClusterRole name: netobserv-cli"))

			assert.Contains(t, yamls[4], "kind: ServiceMonitor")
			assert.Contains(t, yamls[4], "name: netobserv-cli")
			assert.Contains(t, yamls[4], "namespace: \"netobserv-cli\"")
			assert.Contains(t, Normalize(yamls[4]), Normalize("namespaceSelector: matchNames: - \"netobserv-cli\""))
			assert.Contains(t, Normalize(yamls[4]), Normalize("selector: matchLabels: app: netobserv-cli"))

			assert.Contains(t, yamls[5], "kind: ConfigMap")
			assert.Contains(t, yamls[5], "name: netobserv-cli")
			assert.Contains(t, yamls[5], "namespace: openshift-config-managed")
			assert.Contains(t, yamls[5], "console.openshift.io/dashboard: 'true'")

			assert.Contains(t, yamls[6], "kind: DaemonSet")
			assert.Contains(t, yamls[6], "name: netobserv-cli")
			assert.Contains(t, yamls[6], "namespace: \"netobserv-cli\"")
			assert.Contains(t, Normalize(yamls[6]), Normalize("ports: - name: prometheus containerPort: 9401 protocol: TCP"))

			assert.Contains(t, yamls[7], "kind: Service")
			assert.Contains(t, yamls[7], "name: netobserv-cli")
			assert.Contains(t, yamls[7], "namespace: \"netobserv-cli\"")
			assert.Contains(t, Normalize(yamls[7]), Normalize("ports: - name: prometheus protocol: TCP port: 9401 targetPort: 9401"))

			return ctx
		},
	).Feature()
	testCluster.TestEnv().Test(t, f1)
}

func Normalize(str string) string {
	str = strings.ReplaceAll(str, "\n", "")
	str = strings.ReplaceAll(str, " ", "")
	return str
}
