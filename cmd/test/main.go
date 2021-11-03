package main

import (
	"os"
	"syscall"

	"github.com/docker/docker/pkg/reexec"
	"github.com/schoentoon/nsnet/pkg/host"
	"github.com/sirupsen/logrus"
)

func main() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetReportCaller(true)
	cmd := reexec.Command("namespace")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWNS |
			syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNET |
			syscall.CLONE_NEWUSER,
		UidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getuid(),
				Size:        1,
			},
		},
		GidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getgid(),
				Size:        1,
			},
		},
	}

	tun, err := host.New()
	if err != nil {
		logrus.Fatal(err)
	}

	tun.Attach(cmd)

	err = cmd.Start()
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Infof("Container is at pid: %d", cmd.Process.Pid)

	go tun.ReadLoop()

	cmd.Wait()
}
