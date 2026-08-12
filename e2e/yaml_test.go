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

func TestFlowFiltersYAML(t *testing.T) {
	f1 := features.New("flow yaml").Setup(
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			output, err := RunCommand(ylog, "commands/oc-netobserv", "flows",
				"--protocol=TCP",
				"--port=8080",
				"or",
				"--protocol=UDP",
				"--yaml")
			assert.Nil(t, err)

			err = os.WriteFile(path.Join("output", StartupDate+"-flowYAMLOutput"), []byte(output), 0666)
			assert.Nil(t, err)

			assert.NotEmpty(t, output)
			// ensure script setup is fine
			assert.Contains(t, output, "creating netobserv-cli namespace")
			assert.Contains(t, output, "creating service account")
			assert.Contains(t, output, "creating collector service")
			assert.Contains(t, output, "creating flow-capture agents")
			// check CLI done successfully
			assert.Contains(t, output, "Check the generated YAML file in output folder")

			return ctx
		},
	).Assess("check generated yaml output",
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			dirPath := "output"
			assert.True(t, dirExists(dirPath), "directory %s not found", dirPath)

			var yamlStr string
			err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() || filepath.Ext(path) != ".yml" || !strings.Contains(path, "flows_capture") {
					return nil
				}
				yamlBytes, readErr := os.ReadFile(path)
				if readErr != nil {
					return readErr
				}
				content := string(yamlBytes)
				// Prefer the filter-focused capture (not S3 export)
				if strings.Contains(content, `"protocol": "TCP"`) && strings.Contains(content, `"port": 8080`) &&
					!strings.Contains(content, `"name": "s3-export"`) {
					yamlStr = content
				}
				return nil
			})
			assert.Nil(t, err)
			assert.NotEmpty(t, yamlStr, "expected a flows_capture yaml with TCP/8080 filters")

			// check yamls parts
			yamls := strings.Split(yamlStr, "---")
			assert.Equal(t, 8, len(yamls))

			// check yaml contents
			assert.Contains(t, yamls[0], "kind: Namespace")
			assert.Contains(t, yamls[0], "name: \"netobserv-cli\"")
			assert.Contains(t, Normalize(yamls[0]), Normalize("labels:app:netobserv-clipod-security.kubernetes.io/enforce:privilegedpod-security.kubernetes.io/audit:privilegedopenshift.io/cluster-monitoring:\"true\""))

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

			assert.Contains(t, yamls[4], "kind: ClusterRoleBinding")
			assert.Contains(t, yamls[4], "name: netobserv-cli-monitoring")
			assert.Contains(t, yamls[4], "namespace: \"netobserv-cli\"")
			assert.Contains(t, Normalize(yamls[4]), Normalize("subjects: - kind: ServiceAccount name: netobserv-cli namespace: \"netobserv-cli\""))
			assert.Contains(t, Normalize(yamls[4]), Normalize("roleRef: apiGroup: rbac.authorization.k8s.io kind: ClusterRole name: cluster-monitoring-view"))

			assert.Contains(t, yamls[5], "kind: SecurityContextConstraints")
			assert.Contains(t, yamls[5], "name: netobserv-cli")
			assert.Contains(t, yamls[5], "namespace: \"netobserv-cli\"")

			assert.Contains(t, yamls[6], "kind: Service")
			assert.Contains(t, yamls[6], "name: collector")
			assert.Contains(t, yamls[6], "namespace: \"netobserv-cli\"")
			assert.Contains(t, Normalize(yamls[6]), Normalize("ports: - name: collector protocol: TCP port: 9999 targetPort: 9999"))

			assert.Contains(t, yamls[7], "kind: DaemonSet")
			assert.Contains(t, yamls[7], "name: netobserv-cli")
			assert.Contains(t, yamls[7], "namespace: \"netobserv-cli\"")
			assert.Contains(t, Normalize(yamls[7]), Normalize("[{\"direction\": \"\", \"ip_cidr\": \"0.0.0.0/0\", \"protocol\": \"TCP\", \"source_port\": 0, \"destination_port\": 0, \"port\": 8080, \"source_port_range\": \"\", \"source_ports\": \"\", \"destination_port_range\": \"\", \"destination_ports\": \"\", \"port_range\": \"\", \"ports\": \"\", \"icmp_type\": 0, \"icmp_code\": 0, \"peer_ip\": \"\", \"peer_cidr\": \"\", \"action\": \"Accept\", \"tcp_flags\": \"\", \"drops\": false}, {\"direction\": \"\", \"ip_cidr\": \"0.0.0.0/0\", \"protocol\": \"UDP\", \"source_port\": 0, \"destination_port\": 0, \"port\": 0, \"source_port_range\": \"\", \"source_ports\": \"\", \"destination_port_range\": \"\", \"destination_ports\": \"\", \"port_range\": \"\", \"ports\": \"\", \"icmp_type\": 0, \"icmp_code\": 0, \"peer_ip\": \"\", \"peer_cidr\": \"\", \"action\": \"Accept\", \"tcp_flags\": \"\", \"drops\": false}]"))
			assert.Contains(t, Normalize(yamls[7]), Normalize("\"grpc\": { \"targetHost\": \"collector.netobserv-cli.svc.cluster.local\", \"targetPort\": 9999 }"))

			return ctx
		},
	).Feature()
	testCluster.TestEnv().Test(t, f1)
}

func TestPacketFiltersYAML(t *testing.T) {
	f1 := features.New("packet yaml").Setup(
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			output, err := RunCommand(ylog, "commands/oc-netobserv", "packets",
				"--node-selector=netobserv:true",
				"--port=80",
				"--yaml")
			assert.Nil(t, err)

			err = os.WriteFile(path.Join("output", StartupDate+"-packetYAMLOutput"), []byte(output), 0666)
			assert.Nil(t, err)

			assert.NotEmpty(t, output)
			// ensure script setup is fine
			assert.Contains(t, output, "creating netobserv-cli namespace")
			assert.Contains(t, output, "creating service account")
			assert.Contains(t, output, "creating collector service")
			assert.Contains(t, output, "creating packet-capture agents")
			// check CLI done successfully
			assert.Contains(t, output, "Check the generated YAML file in output folder")

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
					if strings.Contains(path, "packets_capture") && filepath.Ext(path) == ".yml" {
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
			assert.Contains(t, Normalize(yamls[0]), Normalize("labels:app:netobserv-clipod-security.kubernetes.io/enforce:privilegedpod-security.kubernetes.io/audit:privilegedopenshift.io/cluster-monitoring:\"true\""))

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

			assert.Contains(t, yamls[4], "kind: ClusterRoleBinding")
			assert.Contains(t, yamls[4], "name: netobserv-cli-monitoring")
			assert.Contains(t, yamls[4], "namespace: \"netobserv-cli\"")
			assert.Contains(t, Normalize(yamls[4]), Normalize("subjects: - kind: ServiceAccount name: netobserv-cli namespace: \"netobserv-cli\""))
			assert.Contains(t, Normalize(yamls[4]), Normalize("roleRef: apiGroup: rbac.authorization.k8s.io kind: ClusterRole name: cluster-monitoring-view"))

			assert.Contains(t, yamls[5], "kind: SecurityContextConstraints")
			assert.Contains(t, yamls[5], "name: netobserv-cli")
			assert.Contains(t, yamls[5], "namespace: \"netobserv-cli\"")

			assert.Contains(t, yamls[6], "kind: Service")
			assert.Contains(t, yamls[6], "name: collector")
			assert.Contains(t, yamls[6], "namespace: \"netobserv-cli\"")
			assert.Contains(t, Normalize(yamls[6]), Normalize("ports: - name: collector protocol: TCP port: 9999 targetPort: 9999"))

			assert.Contains(t, yamls[7], "kind: DaemonSet")
			assert.Contains(t, yamls[7], "name: netobserv-cli")
			assert.Contains(t, yamls[7], "namespace: \"netobserv-cli\"")
			assert.Contains(t, Normalize(yamls[7]), Normalize("[{\"direction\": \"\", \"ip_cidr\": \"0.0.0.0/0\", \"protocol\": \"\", \"source_port\": 0, \"destination_port\": 0, \"port\": 80, \"source_port_range\": \"\", \"source_ports\": \"\", \"destination_port_range\": \"\", \"destination_ports\": \"\", \"port_range\": \"\", \"ports\": \"\", \"icmp_type\": 0, \"icmp_code\": 0, \"peer_ip\": \"\", \"peer_cidr\": \"\", \"action\": \"Accept\", \"tcp_flags\": \"\", \"drops\": false}]"))
			assert.Contains(t, Normalize(yamls[7]), Normalize("nodeSelector: netobserv: \"true\""))

			return ctx
		},
	).Feature()
	testCluster.TestEnv().Test(t, f1)
}

// test metrics only as YAML output as kind can't manage ServiceMonitor CR
func TestMetricYAML(t *testing.T) {
	f1 := features.New("metric yaml").Setup(
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			output, err := RunCommand(ylog, "commands/oc-netobserv", "metrics", "--yaml")
			assert.Nil(t, err)

			err = os.WriteFile(path.Join("output", StartupDate+"-metricYAMLOutput"), []byte(output), 0666)
			assert.Nil(t, err)

			assert.NotEmpty(t, output)
			// ensure script setup is fine
			assert.Contains(t, output, "creating netobserv-cli namespace")
			assert.Contains(t, output, "creating service account")
			assert.Contains(t, output, "creating service monitor")
			assert.Contains(t, output, "creating metric-capture agents")
			// check CLI done successfully
			assert.Contains(t, output, "Check the generated YAML file in output folder.")

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
					if strings.Contains(path, "metrics_capture") && filepath.Ext(path) == ".yml" {
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
			assert.Equal(t, 12, len(yamls))

			// check yaml contents
			assert.Contains(t, yamls[0], "kind: Namespace")
			assert.Contains(t, yamls[0], "name: \"netobserv-cli\"")
			assert.Contains(t, Normalize(yamls[0]), Normalize("labels:app:netobserv-clipod-security.kubernetes.io/enforce:privilegedpod-security.kubernetes.io/audit:privilegedopenshift.io/cluster-monitoring:\"true\""))

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

			assert.Contains(t, yamls[4], "kind: ClusterRoleBinding")
			assert.Contains(t, yamls[4], "name: netobserv-cli-monitoring")
			assert.Contains(t, yamls[4], "namespace: \"netobserv-cli\"")
			assert.Contains(t, Normalize(yamls[4]), Normalize("subjects: - kind: ServiceAccount name: netobserv-cli namespace: \"netobserv-cli\""))
			assert.Contains(t, Normalize(yamls[4]), Normalize("roleRef: apiGroup: rbac.authorization.k8s.io kind: ClusterRole name: cluster-monitoring-view"))

			assert.Contains(t, yamls[5], "kind: SecurityContextConstraints")
			assert.Contains(t, yamls[5], "name: netobserv-cli")
			assert.Contains(t, yamls[5], "namespace: \"netobserv-cli\"")

			assert.Contains(t, yamls[6], "kind: ClusterRole")
			assert.Contains(t, yamls[6], "name: netobserv-cli-metrics")
			assert.Contains(t, yamls[6], "namespace: \"netobserv-cli\"")
			assert.Contains(t, Normalize(yamls[6]), Normalize("- apiGroups: - resources: - pods - services - endpoints verbs: - list - get - watch - nonResourceURLs: - /metrics verbs: - get"))

			assert.Contains(t, yamls[7], "kind: ClusterRoleBinding")
			assert.Contains(t, yamls[7], "name: netobserv-cli")
			assert.Contains(t, yamls[7], "namespace: \"netobserv-cli\"")
			assert.Contains(t, Normalize(yamls[7]), Normalize("subjects: - kind: ServiceAccount name: prometheus-k8s namespace: openshift-monitoring"))
			assert.Contains(t, Normalize(yamls[7]), Normalize("roleRef: apiGroup: rbac.authorization.k8s.io kind: ClusterRole name: netobserv-cli-metrics"))

			assert.Contains(t, yamls[8], "kind: ServiceMonitor")
			assert.Contains(t, yamls[8], "name: netobserv-cli")
			assert.Contains(t, yamls[8], "namespace: \"netobserv-cli\"")
			assert.Contains(t, Normalize(yamls[8]), Normalize("namespaceSelector: matchNames: - \"netobserv-cli\""))
			assert.Contains(t, Normalize(yamls[8]), Normalize("selector: matchLabels: app: netobserv-cli"))

			assert.Contains(t, yamls[9], "kind: Service")
			assert.Contains(t, yamls[9], "name: netobserv-cli")
			assert.Contains(t, yamls[9], "namespace: \"netobserv-cli\"")
			assert.Contains(t, Normalize(yamls[9]), Normalize("ports: - name: prometheus protocol: TCP port: 9401 targetPort: 9401"))

			assert.Contains(t, yamls[10], "kind: ConfigMap")
			assert.Contains(t, yamls[10], "name: netobserv-cli")
			assert.Contains(t, yamls[10], "namespace: openshift-config-managed")
			assert.Contains(t, yamls[10], "console.openshift.io/dashboard: \"true\"")

			assert.Contains(t, yamls[11], "kind: DaemonSet")
			assert.Contains(t, yamls[11], "name: netobserv-cli")
			assert.Contains(t, yamls[11], "namespace: \"netobserv-cli\"")
			assert.Contains(t, Normalize(yamls[11]), Normalize("ports: - name: prometheus containerPort: 9401 protocol: TCP"))

			return ctx
		},
	).Feature()
	testCluster.TestEnv().Test(t, f1)
}

func TestFlowS3ExportYAML(t *testing.T) {
	f1 := features.New("flow s3 yaml").Setup(
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Setenv("NETOBSERV_S3_ACCESS_KEY", "test-access-key")
			t.Setenv("NETOBSERV_S3_SECRET_KEY", "test-secret-key")

			output, err := RunCommand(ylog, "commands/oc-netobserv", "flows",
				"--s3-endpoint=http://minio.example:9000",
				"--s3-bucket=netobserv-flows",
				"--s3-account=e2e-cluster",
				"--s3-prefix=netobserv",
				"--s3-batch-size=100",
				"--s3-write-timeout=10s",
				"--max-time=1m",
				"--yaml")
			assert.Nil(t, err)

			err = os.WriteFile(path.Join("output", StartupDate+"-flowS3YAMLOutput"), []byte(output), 0666)
			assert.Nil(t, err)

			assert.NotEmpty(t, output)
			assert.Contains(t, output, "creating flow-capture agents")
			assert.Contains(t, output, "S3 Parquet export enabled")
			assert.Contains(t, output, "Check the generated YAML file in output folder")
			// S3 is agent FLP_CONFIG only — not collector --options
			assert.NotContains(t, output, `--options "s3-`)

			return ctx
		},
	).Assess("check S3 encode stage in agent FLP_CONFIG",
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			dirPath := "output"
			assert.True(t, dirExists(dirPath), "directory %s not found", dirPath)

			var foundS3YAML bool
			err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() || filepath.Ext(path) != ".yml" || !strings.Contains(path, "flows_capture") {
					return nil
				}
				yamlBytes, readErr := os.ReadFile(path)
				if readErr != nil {
					return readErr
				}
				yamlStr := string(yamlBytes)
				if !strings.Contains(yamlStr, `"name": "s3-export"`) {
					return nil
				}
				foundS3YAML = true
				assert.Contains(t, yamlStr, `"type": "s3"`)
				assert.Contains(t, yamlStr, `"format": "parquet"`)
				assert.Contains(t, yamlStr, `"endpoint": "http://minio.example:9000"`)
				assert.Contains(t, yamlStr, `"bucket": "netobserv-flows"`)
				assert.Contains(t, yamlStr, `"account": "e2e-cluster"`)
				assert.Contains(t, yamlStr, `"prefix": "netobserv"`)
				assert.Contains(t, yamlStr, `"batchSize": 100`)
				assert.Contains(t, yamlStr, `"writeTimeout": "10s"`)
				assert.Contains(t, yamlStr, `"accessKeyId": "test-access-key"`)
				assert.Contains(t, yamlStr, `"secretAccessKey": "test-secret-key"`)
				assert.Contains(t, yamlStr, `"follows": "enrich"`)
				assert.Contains(t, yamlStr, `name: FLP_CONFIG`)
				assert.Contains(t, Normalize(yamlStr), Normalize(`"grpc": { "targetHost": "collector.netobserv-cli.svc.cluster.local", "targetPort": 9999 }`))
				return nil
			})
			assert.Nil(t, err)
			assert.True(t, foundS3YAML, "expected a flows_capture yaml containing s3-export in FLP_CONFIG")

			return ctx
		},
	).Feature()
	testCluster.TestEnv().Test(t, f1)
}

func TestFlowS3MissingCredentialsYAML(t *testing.T) {
	f1 := features.New("flow s3 missing credentials").Setup(
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Setenv("NETOBSERV_S3_ACCESS_KEY", "")
			t.Setenv("NETOBSERV_S3_SECRET_KEY", "")
			t.Setenv("AWS_ACCESS_KEY_ID", "")
			t.Setenv("AWS_SECRET_ACCESS_KEY", "")

			output, err := RunCommand(ylog, "commands/oc-netobserv", "flows",
				"--s3-endpoint=http://minio.example:9000",
				"--s3-bucket=netobserv-flows",
				"--yaml")
			assert.NotNil(t, err)
			assert.Contains(t, output, "S3 export requires credentials")
			assert.NotContains(t, output, "S3 Parquet export enabled")
			return ctx
		},
	).Feature()
	testCluster.TestEnv().Test(t, f1)
}

func TestFlowParquetFormatYAML(t *testing.T) {
	f1 := features.New("flow parquet format yaml").Setup(
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			output, err := RunCommand(ylog, "commands/oc-netobserv", "flows",
				"--format=parquet",
				"--yaml")
			assert.Nil(t, err)

			err = os.WriteFile(path.Join("output", StartupDate+"-flowParquetFormatYAMLOutput"), []byte(output), 0666)
			assert.Nil(t, err)

			assert.NotEmpty(t, output)
			assert.Contains(t, output, "creating flow-capture agents")
			assert.Contains(t, output, "Local output format(s): parquet")
			assert.Contains(t, output, "Check the generated YAML file in output folder")
			// Local format is collector-side; agent FLP still sends gRPC (no parquet encode stage)
			assert.NotContains(t, output, "S3 Parquet export enabled")

			return ctx
		},
	).Assess("agent yaml unchanged except still has grpc send",
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			dirPath := "output"
			assert.True(t, dirExists(dirPath), "directory %s not found", dirPath)

			var foundYAML bool
			err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() || filepath.Ext(path) != ".yml" || !strings.Contains(path, "flows_capture") {
					return nil
				}
				yamlBytes, readErr := os.ReadFile(path)
				if readErr != nil {
					return readErr
				}
				yamlStr := string(yamlBytes)
				// Prefer the parquet-format run (no s3-export)
				if strings.Contains(yamlStr, `"name": "s3-export"`) {
					return nil
				}
				foundYAML = true
				assert.NotContains(t, yamlStr, `"type": "s3"`)
				assert.Contains(t, Normalize(yamlStr), Normalize(`"grpc": { "targetHost": "collector.netobserv-cli.svc.cluster.local", "targetPort": 9999 }`))
				return nil
			})
			assert.Nil(t, err)
			assert.True(t, foundYAML, "expected a flows_capture yaml without s3-export")

			return ctx
		},
	).Feature()
	testCluster.TestEnv().Test(t, f1)
}

func TestFlowMultiFormatYAML(t *testing.T) {
	f1 := features.New("flow multi format yaml").Setup(
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			output, err := RunCommand(ylog, "commands/oc-netobserv", "flows",
				"--format=json,parquet",
				"--yaml")
			assert.Nil(t, err)
			assert.Contains(t, output, "Local output format(s): json,parquet")
			assert.NotContains(t, output, "Auto-enabling --copy=true for parquet")
			assert.NotContains(t, output, "S3 Parquet export enabled")
			return ctx
		},
	).Feature()
	testCluster.TestEnv().Test(t, f1)
}

func TestFlowDefaultFormatNoParquetYAML(t *testing.T) {
	f1 := features.New("flow default format no parquet").Setup(
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			output, err := RunCommand(ylog, "commands/oc-netobserv", "flows", "--yaml")
			assert.Nil(t, err)
			// Default path must not enable parquet
			assert.NotContains(t, output, "Local output format(s):")
			assert.NotContains(t, output, "Auto-enabling --copy=true for parquet")
			assert.NotContains(t, output, "S3 Parquet export enabled")
			return ctx
		},
	).Feature()
	testCluster.TestEnv().Test(t, f1)
}

func TestFlowInvalidFormatYAML(t *testing.T) {
	f1 := features.New("flow invalid format yaml").Setup(
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			output, err := RunCommand(ylog, "commands/oc-netobserv", "flows",
				"--format=json,bogus",
				"--yaml")
			assert.NotNil(t, err)
			assert.Contains(t, output, "invalid --format token")
			return ctx
		},
	).Feature()
	testCluster.TestEnv().Test(t, f1)
}

func TestFlowParquetFormatWithS3YAML(t *testing.T) {
	f1 := features.New("flow parquet format with s3 yaml").Setup(
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Setenv("NETOBSERV_S3_ACCESS_KEY", "test-access-key")
			t.Setenv("NETOBSERV_S3_SECRET_KEY", "test-secret-key")

			output, err := RunCommand(ylog, "commands/oc-netobserv", "flows",
				"--format=parquet",
				"--s3-endpoint=http://minio.example:9000",
				"--s3-bucket=netobserv-flows",
				"--yaml")
			assert.Nil(t, err)

			assert.Contains(t, output, "Local output format(s): parquet")
			assert.Contains(t, output, "S3 Parquet export enabled")

			return ctx
		},
	).Assess("s3 encode still present when format=parquet",
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var foundS3YAML bool
			err := filepath.Walk("output", func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() || filepath.Ext(path) != ".yml" || !strings.Contains(path, "flows_capture") {
					return nil
				}
				yamlBytes, readErr := os.ReadFile(path)
				if readErr != nil {
					return readErr
				}
				yamlStr := string(yamlBytes)
				if !strings.Contains(yamlStr, `"name": "s3-export"`) {
					return nil
				}
				foundS3YAML = true
				assert.Contains(t, yamlStr, `"format": "parquet"`)
				assert.Contains(t, yamlStr, `"bucket": "netobserv-flows"`)
				return nil
			})
			assert.Nil(t, err)
			assert.True(t, foundS3YAML, "expected s3-export when combining --format=parquet with --s3-*")
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
