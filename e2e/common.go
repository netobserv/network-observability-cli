package e2e

import (
	"bufio"
	"io"
	"os/exec"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/sirupsen/logrus"
)

const (
	CommandTimeout = 60 * time.Second
)

var (
	StartupDate = time.Now().Format("20060102-150405")
)

// run command with tty support
func RunCommand(log *logrus.Entry, commandName string, arg ...string) (string, error) {
	cmdStr := path.Join("commands", commandName)
	log.WithFields(logrus.Fields{"cmd": cmdStr, "arg": arg}).Info("running command")

	log.Print("Executing command...")
	cmd := exec.Command(cmdStr, arg...)
	cmd.Env = append(cmd.Environ(), "isE2E=true")

	outPipe, _ := cmd.StdoutPipe()
	errPipe, _ := cmd.StderrPipe()

	timer := time.AfterFunc(CommandTimeout, func() {
		log.Print("Terminating command...")
		err := cmd.Process.Signal(syscall.SIGTERM)
		if err != nil {
			log.Error(err)
		}
	})
	defer timer.Stop()

	var sb strings.Builder
	go func(_ io.ReadCloser) {
		reader := bufio.NewReader(errPipe)
		line, err := reader.ReadString('\n')
		for err == nil {
			sb.WriteString(line)
			line, err = reader.ReadString('\n')
		}
	}(errPipe)

	go func(_ io.ReadCloser) {
		reader := bufio.NewReader(outPipe)
		line, err := reader.ReadString('\n')
		for err == nil {
			sb.WriteString(line)
			line, err = reader.ReadString('\n')
		}
	}(outPipe)

	in, err := pty.Start(cmd)
	if err != nil {
		panic(err)
	}

	timer = time.AfterFunc(30*time.Second, func() {
		log.Print("Simulating keyboard typing...")
		_, err := in.Write([]byte("netobserv"))
		if err != nil {
			log.Error(err)
		}
	})
	defer timer.Stop()

	if err := cmd.Wait(); err != nil {
		return sb.String(), err
	}

	return sb.String(), nil
}
