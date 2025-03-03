package integration_tests

import (
	"context"
	"os"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func getClusterNodes(clientset *kubernetes.Clientset, options metav1.ListOptions) ([]string, error) {
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), options)
	var allNodes []string
	if err != nil {
		return allNodes, err
	}
	for _, node := range nodes.Items {
		allNodes = append(allNodes, node.Name)
	}
	return allNodes, nil
}

func getDaemonSet(clientset *kubernetes.Clientset, daemonset string, ns string) (*appsv1.DaemonSet, error) {
	ds, err := clientset.AppsV1().DaemonSets(ns).Get(context.TODO(), daemonset, metav1.GetOptions{})

	if err != nil {
		return nil, err
	}

	return ds, nil
}

func getNamespacePods(clientset *kubernetes.Clientset, namespace string, options metav1.ListOptions) ([]string, error) {
	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), options)
	var allPods []string
	if err != nil {
		return allPods, err
	}
	for _, pod := range pods.Items {
		allPods = append(allPods, pod.Name)
	}
	return allPods, nil
}
