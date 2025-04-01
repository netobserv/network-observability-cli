package integrationtests

import (
	"context"
	"io/fs"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

func isCLIReady(clientset *kubernetes.Clientset, cliNS string) (bool, error) {
	var collectorReady, cliDaemonsetReady bool
	err := wait.PollUntilContextTimeout(context.Background(), 30*time.Second, 300*time.Second, false, func(context.Context) (done bool, err error) {
		if !collectorReady {
			collectorPod, err := getNamespacePods(clientset, cliNS, &metav1.ListOptions{FieldSelector: "status.phase=Running", LabelSelector: "run=collector"})
			if err != nil {
				return false, err
			}

			if len(collectorPod) > 0 {
				collectorReady = true
			}
		}

		daemonset := "netobserv-cli"
		cliDaemonset, err := getDaemonSet(clientset, daemonset, cliNS)

		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		if cliDaemonset.Status.DesiredNumberScheduled == cliDaemonset.Status.NumberReady {
			cliDaemonsetReady = true
		}
		return collectorReady && cliDaemonsetReady, nil
	})
	if err != nil {
		return false, err
	}
	return true, nil
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
