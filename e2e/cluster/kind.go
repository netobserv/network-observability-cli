package cluster

import (
	"context"
	"fmt"
	"os"
	"path"
	"runtime"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/vladimirvivien/gexe"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/support/kind"
)

const (
	cliContainerName = "localhost/netobserv-cli:test"
	kindImage        = "kindest/node:v1.31.0"
	logsSubDir       = "e2e-logs"
	localArchiveName = "cli-e2e-img.tar"
)

var klog = logrus.WithField("component", "cluster.Kind")

type Kind struct {
	clusterName string
	baseDir     string
	testEnv     env.Environment
	timeout     time.Duration
}

// Option that can be passed to the NewKind function in order to change the configuration
// of the test cluster
type Option func(k *Kind)

// NewKind creates a kind cluster given a name and set of Option instances. The base dir
// must point to the folder where the logs are going to be stored and, in case your docker
// backend doesn't provide access to the local images, where the cli-e2e-img.tar container image
// is located. Usually it will be the project root.
func NewKind(kindClusterName, baseDir string, options ...Option) *Kind {
	k := &Kind{
		testEnv:     env.New(),
		clusterName: kindClusterName,
		baseDir:     baseDir,
		timeout:     2 * time.Minute,
	}
	for _, option := range options {
		option(k)
	}
	return k
}

// Run the Kind cluster for the later execution of tests.
func (k *Kind) Run(m *testing.M) {
	envFuncs := []env.Func{
		envfuncs.CreateClusterWithConfig(
			kind.NewProvider(),
			k.clusterName,
			path.Join(packageDir(), "res", "kind.yaml"),
			kind.WithImage(kindImage)),
		k.loadLocalImage(),
	}
	klog.Info("starting kind setup")
	code := k.testEnv.Setup(envFuncs...).
		Finish(
			k.exportLogs(),
			k.deleteNamespace(),
			envfuncs.DestroyCluster(k.clusterName),
		).Run(m)
	klog.WithField("returnCode", code).Info("tests finished run")
}

func (k *Kind) GetLogsDir() string {
	logsDir := path.Join(k.baseDir, logsSubDir)
	err := os.MkdirAll(logsDir, 0700)
	if err != nil {
		klog.Error(err)
	}
	return logsDir
}

// export logs into the e2e-logs folder of the base directory.
func (k *Kind) exportLogs() env.Func {
	return func(ctx context.Context, config *envconf.Config) (context.Context, error) {
		logsDir := k.GetLogsDir()
		klog.WithField("directory", logsDir).Info("exporting cluster logs")
		exe := gexe.New()
		out := exe.Run("kind export logs " + logsDir + " --name " + k.clusterName)
		klog.WithField("out", out).Info("exported cluster logs")

		//move output files to cluster logs folder
		err := os.Rename(path.Join(k.baseDir, "e2e", "tmp"), path.Join(logsDir, "output"))
		if err != nil {
			klog.Error(err)
		}

		return ctx, nil
	}
}

func (k *Kind) GetAgentLogs() string {
	exe := gexe.New()
	contextOut := exe.Run("kubectl cluster-info --context " + k.clusterName)
	logsOut := exe.Run("kubectl logs -l app=netobserv-cli -n netobserv-cli --tail -1")

	return fmt.Sprintf("Set context: %s\n\nLogs: %s", contextOut, logsOut)
}

// delete netobserv-cli namespace
func (k *Kind) deleteNamespace() env.Func {
	return func(ctx context.Context, config *envconf.Config) (context.Context, error) {
		exe := gexe.New()
		out := exe.Run("kubectl delete namespace netobserv-cli")
		klog.WithField("out", out).Info("deleted namespace")
		return ctx, nil
	}
}

func (k *Kind) TestEnv() env.Environment {
	return k.testEnv
}

// loadLocalImage loads the cli docker image into the test cluster. It tries both available
// methods, which will selectively work depending on the container backend type
func (k *Kind) loadLocalImage() env.Func {
	return func(ctx context.Context, config *envconf.Config) (context.Context, error) {
		klog.Info("trying to load docker image from local registry")
		ctx, err := envfuncs.LoadDockerImageToCluster(
			k.clusterName, cliContainerName)(ctx, config)
		if err == nil {
			klog.Info("loaded docker image from local registry")
			return ctx, nil
		}
		klog.WithError(err).WithField("archive", localArchiveName).
			Info("couldn't load image from local registry. Trying from local archive")
		ctx, err = envfuncs.LoadImageArchiveToCluster(
			k.clusterName, path.Join(k.baseDir, localArchiveName))(ctx, config)
		if err == nil {
			klog.Info("loaded docker image from local archive")
			return ctx, nil
		}
		return ctx, err
	}
}

// helper to get the base directory of this package, allowing to load the test deployment
// files whatever the working directory is
func packageDir() string {
	_, file, _, ok := runtime.Caller(1)
	if !ok {
		panic("can't find package directory for (project_dir)/test/cluster")
	}
	return path.Dir(file)
}
