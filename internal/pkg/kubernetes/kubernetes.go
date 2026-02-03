package kubernetes

import (
	"context"
	"fmt"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func DeleteDaemonSet(ctx context.Context) error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("cannot get Kubernetes InClusterConfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("cannot create Kubernetes client from InClusterConfig: %w", err)
	}

	return clientset.AppsV1().DaemonSets("netobserv-cli").Delete(ctx, "netobserv-cli", v1.DeleteOptions{})
}
