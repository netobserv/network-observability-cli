package integrationtests

import (
	"context"
	"io/fs"
	"log"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

func isCollectorReady(clientset *kubernetes.Clientset, cliNS string) (bool, error) {
	err := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 300*time.Second, false, func(context.Context) (done bool, err error) {
		collectorPod, err := getNamespacePods(clientset, cliNS, &metav1.ListOptions{FieldSelector: "status.phase=Running", LabelSelector: "run=collector"})
		if err != nil {
			return false, err
		}
		return len(collectorPod) > 0, nil
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

func isDaemonsetReady(clientset *kubernetes.Clientset, daemonsetName string, cliNS string) (bool, error) {
	err := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 300*time.Second, false, func(context.Context) (done bool, err error) {

		cliDaemonset, err := getDaemonSet(clientset, daemonsetName, cliNS)
		if err != nil {
			return false, err
		}
		return cliDaemonset.Status.DesiredNumberScheduled == cliDaemonset.Status.NumberReady, nil
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

func isCLIReady(clientset *kubernetes.Clientset, cliNS string) (bool, error) {
	collectorReady, err := isCollectorReady(clientset, cliNS)
	if err != nil {
		return false, err
	}
	log.Printf("Collector ready: %v", collectorReady)
	daemonsetReady, err := isDaemonsetReady(clientset, "netobserv-cli", cliNS)
	if err != nil {
		return false, err
	}
	log.Printf("Daemonset ready: %v", daemonsetReady)

	return collectorReady && daemonsetReady, nil
}

// get latest flows.json file
func getFlowsJSONFile() (string, error) {
	// var files []fs.DirEntry
	var files []string
	outputDir := "./output/flow/"
	dirFS := os.DirFS(outputDir)
	files, err := fs.Glob(dirFS, "*.json")
	if err != nil {
		return "", err
	}
	// this could be problematic if two tests are running in parallel with --copy=true
	var mostRecentFile fs.FileInfo
	for _, file := range files {
		fileInfo, err := os.Stat(outputDir + file)
		if err != nil {
			return "", nil
		}
		if mostRecentFile == nil || fileInfo.ModTime().After(mostRecentFile.ModTime()) {
			mostRecentFile = fileInfo
		}
	}
	return outputDir + mostRecentFile.Name(), nil
}
