//go:build !e2e

package integrationtests

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func getNewClient() (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)

	if err != nil {
		return nil, err
	}

	return clientset, nil
}

func getClusterNodes(clientset *kubernetes.Clientset, options *metav1.ListOptions) ([]string, error) {
	nodes, err := clientset.CoreV1().Nodes().List(context.Background(), *options)
	var allNodes []string
	if err != nil {
		return allNodes, err
	}
	for i := range nodes.Items {
		allNodes = append(allNodes, nodes.Items[i].Name)
	}
	return allNodes, nil
}

func getDaemonSet(ctx context.Context, clientset *kubernetes.Clientset, daemonset string, ns string) (*appsv1.DaemonSet, error) {
	ds, err := clientset.AppsV1().DaemonSets(ns).Get(ctx, daemonset, metav1.GetOptions{})

	if err != nil {
		return nil, err
	}

	return ds, nil
}

func getNamespace(ctx context.Context, clientset *kubernetes.Clientset, name string) (*corev1.Namespace, error) {
	namespace, err := clientset.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return namespace, nil
}

func getNamespacePods(ctx context.Context, clientset *kubernetes.Clientset, namespace string, options *metav1.ListOptions) ([]string, error) {
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, *options)
	var allPods []string
	if err != nil {
		return allPods, err
	}
	for i := range pods.Items {
		allPods = append(allPods, pods.Items[i].Name)
	}
	return allPods, nil
}

func getConfigMap(ctx context.Context, clientset *kubernetes.Clientset, name string, namespace string) (*corev1.ConfigMap, error) {
	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return cm, nil
}

func getPrometheusServiceAccountToken(clientset *kubernetes.Clientset) (string, error) {
	// Create a token for the prometheus-k8s service account with 1 hour expiration
	token, err := createServiceAccountToken(clientset, "prometheus-k8s", "openshift-monitoring", 3600)
	if err != nil {
		return "", fmt.Errorf("failed to create token for prometheus service account: %w", err)
	}
	return token, nil
}

func queryPrometheusMetric(clientset *kubernetes.Clientset, query string) (float64, error) {
	// Get OpenShift route client
	config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	if err != nil {
		return 0.0, fmt.Errorf("failed to build config: %w", err)
	}

	dynclient, err := dynamic.NewForConfig(config)
	if err != nil {
		return 0.0, fmt.Errorf("failed to create route client: %w", err)
	}

	// Get the Prometheus route from openshift-monitoring namespace
	var routeGVR = schema.GroupVersionResource{Group: "route.openshift.io", Version: "v1", Resource: "routes"}
	unstructuredRoute, err := dynclient.Resource(routeGVR).Namespace("openshift-monitoring").Get(context.Background(), "prometheus-k8s", metav1.GetOptions{})
	if err != nil {
		return 0.0, fmt.Errorf("failed to get prometheus route: %w", err)
	}

	// Extract host from unstructured
	host, found, err := unstructured.NestedString(unstructuredRoute.UnstructuredContent(), "spec", "host")
	if err != nil {
		return 0, fmt.Errorf("failed to extract host from unstructured route: %w", err)
	}
	if !found {
		return 0, fmt.Errorf("host not found in unstructured route: %w", err)
	}

	// Construct the Prometheus API URL using the route host
	prometheusURL := fmt.Sprintf("https://%s/api/v1/query", host)

	// Create HTTP client with proper TLS configuration for OpenShift
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // For testing purposes
			},
		},
		Timeout: 30 * time.Second,
	}

	// Prepare the query request
	req, err := http.NewRequest("GET", prometheusURL, nil)
	if err != nil {
		return 0.0, fmt.Errorf("failed to create request: %w", err)
	}

	// Add query parameters
	q := req.URL.Query()
	q.Add("query", query)
	req.URL.RawQuery = q.Encode()

	// Add headers for OpenShift authentication
	token, err := getPrometheusServiceAccountToken(clientset)
	if err != nil {
		return 0.0, fmt.Errorf("failed to get prometheus service account token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	var finalResult float64
	// Poll for 5 minutes at 20-second intervals
	err = wait.PollUntilContextTimeout(context.Background(), 20*time.Second, 5*time.Minute, false, func(_ context.Context) (done bool, err error) {
		// Execute the request
		resp, err := httpClient.Do(req)
		if err != nil {
			// HTTP errors are retryable
			return false, nil
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			// HTTP status errors are retryable
			return false, nil
		}

		// Parse the response
		var result struct {
			Status string `json:"status"`
			Data   struct {
				ResultType string `json:"resultType"`
				Result     []struct {
					Value []interface{} `json:"value"`
				} `json:"result"`
			} `json:"data"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			// JSON decode errors are retryable
			return false, nil
		}

		if result.Status != "success" {
			// Prometheus query failures are retryable
			return false, nil
		}

		if len(result.Data.Result) == 0 {
			// No data returned - this is retryable
			return false, nil
		}

		// Extract the metric value (first result)
		value := result.Data.Result[0].Value[1]

		// Convert to float64
		switch v := value.(type) {
		case string:
			if parsedValue, parseErr := strconv.ParseFloat(v, 64); parseErr == nil {
				finalResult = parsedValue
				return true, nil
			}
			// Parse errors are retryable
			return false, nil
		case float64:
			finalResult = v
			return true, nil
		default:
			// Type conversion errors are retryable
			return false, nil
		}
	})

	if err != nil {
		return 0.0, fmt.Errorf("failed to get prometheus metrics after 5 minutes of polling: %w", err)
	}

	return finalResult, nil
}

func createServiceAccountToken(clientset *kubernetes.Clientset, serviceAccountName, namespace string, expirationSeconds int64) (string, error) {
	tokenRequest := &authv1.TokenRequest{
		Spec: authv1.TokenRequestSpec{
			ExpirationSeconds: &expirationSeconds,
		},
	}

	token, err := clientset.CoreV1().ServiceAccounts(namespace).CreateToken(
		context.Background(),
		serviceAccountName,
		tokenRequest,
		metav1.CreateOptions{},
	)
	if err != nil {
		return "", fmt.Errorf("failed to create token for service account %s/%s: %w", namespace, serviceAccountName, err)
	}

	return token.Status.Token, nil
}
