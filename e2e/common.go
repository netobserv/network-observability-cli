package e2e

import (
	"os/exec"
	"path"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	commandTimeout = 30 * time.Second
)

var (
	startupDate = time.Now().Format("20060102-150405")
)

// run command with tty support
func runCommand(log *logrus.Entry, commandName string, arg ...string) ([]byte, error) {
	cmdStr := path.Join("commands", commandName)
	log.WithFields(logrus.Fields{"cmd": cmdStr, "arg": arg}).Info("running command")

	log.Print("Executing command...")
	cmd := exec.Command(cmdStr, arg...)

	timer := time.AfterFunc(commandTimeout, func() {
		log.Print("Terminating command...")
		err := cmd.Process.Signal(syscall.SIGTERM)
		if err != nil {
			log.Error(err)
		}
	})
	defer timer.Stop()

	return cmd.CombinedOutput()
}
