//go:build !e2e

package integrationtests

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"github.com/sirupsen/logrus"
)

const (
	PollInterval = 5 * time.Second
	PollTimeout  = 10 * time.Minute
	outputDir    = "./output/flow"
)

var (
	clog = logrus.WithField("component", "cli")
)

func isNamespace(clientset *kubernetes.Clientset, cliNS string, exists bool) (bool, error) {
	err := wait.PollUntilContextTimeout(context.Background(), PollInterval, PollTimeout, true, func(ctx context.Context) (done bool, err error) {
		namespace, err := getNamespace(ctx, clientset, cliNS)
		if exists {
			if err != nil {
				return false, err
			}
			return namespace != nil, err
		} else if errors.IsNotFound(err) {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

func isCollector(clientset *kubernetes.Clientset, cliNS string, ready bool) (bool, error) {
	err := wait.PollUntilContextTimeout(context.Background(), PollInterval, PollTimeout, true, func(ctx context.Context) (done bool, err error) {
		collectorPod, err := getNamespacePods(ctx, clientset, cliNS, &metav1.ListOptions{FieldSelector: "status.phase=Running", LabelSelector: "run=collector"})
		if err != nil {
			return false, err
		}
		if ready {
			return len(collectorPod) > 0, nil
		}
		return len(collectorPod) == 0, nil
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

func isDaemonsetReady(clientset *kubernetes.Clientset, daemonsetName string, cliNS string) (bool, error) {
	err := wait.PollUntilContextTimeout(context.Background(), PollInterval, PollTimeout, true, func(ctx context.Context) (done bool, err error) {
		cliDaemonset, err := getDaemonSet(ctx, clientset, daemonsetName, cliNS)
		if err != nil {
			if errors.IsNotFound(err) {
				clog.Infof("daemonset not found %v", err)
				return false, nil
			}
			return false, err
		}

		desired := cliDaemonset.Status.DesiredNumberScheduled
		ready := cliDaemonset.Status.NumberReady
		current := cliDaemonset.Status.CurrentNumberScheduled

		clog.Debugf("daemonset %s status: DesiredNumberScheduled=%d, CurrentNumberScheduled=%d, NumberReady=%d",
			daemonsetName, desired, current, ready)

		// Ensure daemonset has scheduled pods before checking readiness
		// This prevents race condition where both DesiredNumberScheduled and NumberReady are 0
		if desired == 0 {
			clog.Debugf("daemonset %s has not scheduled any pods yet (DesiredNumberScheduled=0)", daemonsetName)
			return false, nil
		}

		// Check both that all desired pods are scheduled AND ready
		// This ensures pods actually exist before we return true
		return desired == current && current == ready, nil
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

func isCLIRuning(clientset *kubernetes.Clientset, cliNS string) (bool, error) {
	namespaceCreated, err := isNamespace(clientset, cliNS, true)
	if err != nil {
		return false, err
	}
	clog.Debugf("Namespace created: %v", namespaceCreated)

	daemonsetReady, err := isDaemonsetReady(clientset, "netobserv-cli", cliNS)
	if err != nil {
		return false, err
	}
	clog.Infof("Daemonset ready: %v", daemonsetReady)

	collectorReady, err := isCollector(clientset, cliNS, true)
	if err != nil {
		return false, err
	}
	clog.Infof("Collector ready: %v", collectorReady)

	return namespaceCreated && daemonsetReady && collectorReady, nil
}

func isCLIDone(clientset *kubernetes.Clientset, cliNS string) (bool, error) {
	collectorDone, err := isCollector(clientset, cliNS, false)
	if err != nil {
		return false, err
	}
	clog.Debugf("Collector done: %v", collectorDone)
	return collectorDone, nil
}

// get latest flows.json file
func getFlowsJSONFile() (string, error) {
	// var files []fs.DirEntry
	var files []string
	dirFS := os.DirFS(outputDir)

	files, err := fs.Glob(dirFS, "*.json")
	if err != nil {
		return "", err
	}
	// this could be problematic if two tests are running in parallel with --copy=true
	var mostRecentFile fs.FileInfo
	for _, file := range files {
		fileInfo, err := os.Stat(filepath.Join(outputDir, file))
		if err != nil {
			return "", nil
		}
		if mostRecentFile == nil || fileInfo.ModTime().After(mostRecentFile.ModTime()) {
			mostRecentFile = fileInfo
		}
	}
	absPath, err := filepath.Abs(filepath.Join(outputDir, mostRecentFile.Name()))
	if err != nil {
		return "", err
	}
	return absPath, nil
}

// get latest .pcapng file
func getPcapngFile() (string, error) {
	var files []string
	outputDir := "./output/pcap/"
	dirFS := os.DirFS(outputDir)
	files, err := fs.Glob(dirFS, "*.pcapng")
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
