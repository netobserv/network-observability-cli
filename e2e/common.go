package e2e

import (
	"bufio"
	"io"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/sirupsen/logrus"
)

const (
	StartCommandWait  = 30 * time.Second
	RunCommandTimeout = 60 * time.Second
)

var (
	StartupDate = time.Now().Format("20060102-150405")
)

// start command with tty support and wait for some time before returning output
// the command will keep running after this call
func StartCommand(log *logrus.Entry, commandName string, arg ...string) (string, error) {
	log.WithFields(logrus.Fields{"cmd": commandName, "arg": arg}).Info("Starting command")

	cmd := exec.Command(commandName, arg...)
	cmd.Env = append(cmd.Environ(), "isE2E=true")

	outPipe, _ := cmd.StdoutPipe()
	errPipe, _ := cmd.StderrPipe()

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

	// start async
	go func() {
		log.Debug("Starting async ...")
		_, err := pty.Start(cmd)
		if err != nil {
			log.Errorf("Start returned error: %v", err)
		}
	}()

	log.Debugf("Waiting %v ...", StartCommandWait)
	time.Sleep(StartCommandWait)

	log.Debug("Returning result while command still running")
	return sb.String(), nil
}

// run command with tty support and wait for stop
func RunCommand(log *logrus.Entry, commandName string, arg ...string) (string, error) {
	log.WithFields(logrus.Fields{"cmd": commandName, "arg": arg}).Info("Running command")

	cmd := exec.Command(commandName, arg...)
	cmd.Env = append(cmd.Environ(), "isE2E=true")

	outPipe, _ := cmd.StdoutPipe()
	errPipe, _ := cmd.StderrPipe()

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

	log.Debug("Starting ...")
	_, err := pty.Start(cmd)
	if err != nil {
		log.Errorf("Start returned error: %v", err)
		return "", err
	}

	log.Debug("Waiting ...")
	err = cmd.Wait()
	if err != nil {
		log.Errorf("Wait returned error: %v", err)
	}

	// TODO: find why this returns -1. That may be related to pty implementation
	/*if cmd.ProcessState.ExitCode() != 0 {
		return sb.String(), fmt.Errorf("Cmd returned code %d", cmd.ProcessState.ExitCode())
	}*/

	return sb.String(), nil
}

// run command with tty support and terminate it after timeout
// it will also simulate a keyboard input during the run
func RunCommandAndTerminate(log *logrus.Entry, commandName string, arg ...string) (string, error) {
	log.WithFields(logrus.Fields{"cmd": commandName, "arg": arg}).Info("Running command and terminate")

	cmd := exec.Command(commandName, arg...)
	cmd.Env = append(cmd.Environ(), "isE2E=true")

	outPipe, _ := cmd.StdoutPipe()
	errPipe, _ := cmd.StderrPipe()

	timer := time.AfterFunc(RunCommandTimeout, func() {
		log.Debug("Terminating command...")
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

	log.Debug("Starting ...")
	in, err := pty.Start(cmd)
	if err != nil {
		log.Errorf("Start returned error: %v", err)
		return "", err
	}

	timer = time.AfterFunc(30*time.Second, func() {
		log.Debug("Simulating keyboard typing...")
		_, err := in.Write([]byte("netobserv"))
		if err != nil {
			log.Error(err)
		}
	})
	defer timer.Stop()

	log.Debug("Waiting ...")
	err = cmd.Wait()
	if err != nil {
		log.Errorf("Wait returned error: %v", err)
	}

	// TODO: find why this returns -1. That may be related to pty implementation
	/*if cmd.ProcessState.ExitCode() != 0 {
		return sb.String(), fmt.Errorf("Cmd returned code %d", cmd.ProcessState.ExitCode())
	}*/

	return sb.String(), nil
}
