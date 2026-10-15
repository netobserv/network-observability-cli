package capture

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/netobserv/network-observability-cli/internal/k8s/client"
)

// DaemonSetName, dashboardConfigMapNamespace etc. are the fixed resource names/namespaces
// commands/netobserv and scripts/functions.sh have always used.
const DaemonSetName = "netobserv-cli"

const dashboardConfigMapNamespace = "openshift-config-managed"

var serviceMonitorGVR = schema.GroupVersionResource{Group: "monitoring.coreos.com", Version: "v1", Resource: "servicemonitors"}

// DeleteServiceMonitor replaces deleteServiceMonitor.
func DeleteServiceMonitor(ctx context.Context, client *k8sclient.Client, namespace string) error {
	err := client.Dynamic.Resource(serviceMonitorGVR).Namespace(namespace).Delete(ctx, DaemonSetName, metav1.DeleteOptions{})
	if err := k8sclient.IgnoreNotFound(err); err != nil {
		return fmt.Errorf("cannot delete ServiceMonitor: %w", err)
	}
	return nil
}

// DeleteDashboardConfigMap replaces deleteDashboardCM. Note the ConfigMap lives in
// openshift-config-managed, not the capture namespace.
func DeleteDashboardConfigMap(ctx context.Context, client *k8sclient.Client) error {
	err := client.Clientset.CoreV1().ConfigMaps(dashboardConfigMapNamespace).Delete(ctx, DaemonSetName, metav1.DeleteOptions{})
	if err := k8sclient.IgnoreNotFound(err); err != nil {
		return fmt.Errorf("cannot delete dashboard ConfigMap: %w", err)
	}
	return nil
}

// DeleteDaemonSet replaces deleteDaemonset, also what the `stop` subcommand calls on its own.
func DeleteDaemonSet(ctx context.Context, client *k8sclient.Client, namespace string) error {
	err := client.Clientset.AppsV1().DaemonSets(namespace).Delete(ctx, DaemonSetName, metav1.DeleteOptions{})
	if err := k8sclient.IgnoreNotFound(err); err != nil {
		return fmt.Errorf("cannot delete DaemonSet: %w", err)
	}
	return nil
}

// DeleteCollectorPod replaces deletePod.
func DeleteCollectorPod(ctx context.Context, client *k8sclient.Client, namespace string) error {
	err := client.Clientset.CoreV1().Pods(namespace).Delete(ctx, CollectorPodName, metav1.DeleteOptions{})
	if err := k8sclient.IgnoreNotFound(err); err != nil {
		return fmt.Errorf("cannot delete collector pod: %w", err)
	}
	return nil
}

// DeleteNamespace replaces deleteNamespace.
// DeleteNamespace waits for the namespace to be fully gone before returning, rather than just
// issuing the delete and moving on as `oc/kubectl delete --ignore-not-found` does: namespace
// deletion is asynchronous (finalizers can keep it in "Terminating" for a while), and
// namespaceExists (checked at the start of every capture) would otherwise see it as still
// existing, surfacing a confusing "netobserv-cli namespace already exists" if the user reruns a
// capture right after `netobserv cleanup`.
func DeleteNamespace(ctx context.Context, client *k8sclient.Client, namespace string) error {
	err := client.Clientset.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
	if err := k8sclient.IgnoreNotFound(err); err != nil {
		return fmt.Errorf("cannot delete namespace %q: %w", namespace, err)
	}

	err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, err := client.Clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, nil //nolint:nilerr // keep polling on transient errors too; the timeout above still bounds it
	})
	if err != nil {
		return fmt.Errorf("namespace %q did not finish terminating: %w", namespace, err)
	}
	return nil
}

// Cleanup replaces cleanup's resource-teardown half (the copy-output-first prompt lives in
// internal/plugin, which calls CopyFromPod before Cleanup when appropriate), deleting
// resources in the same order bash did.
func Cleanup(ctx context.Context, client *k8sclient.Client, namespace string) error {
	if err := DeleteServiceMonitor(ctx, client, namespace); err != nil {
		return err
	}
	if err := DeleteDashboardConfigMap(ctx, client); err != nil {
		return err
	}
	if err := DeleteDaemonSet(ctx, client, namespace); err != nil {
		return err
	}
	if err := DeleteCollectorPod(ctx, client, namespace); err != nil {
		return err
	}
	if err := DeleteNamespace(ctx, client, namespace); err != nil {
		return err
	}
	return nil
}
