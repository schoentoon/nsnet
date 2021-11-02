package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"

	"github.com/docker/docker/pkg/reexec"
	"github.com/schoentoon/nsnet/pkg/host"
	"github.com/sirupsen/logrus"

	"golang.org/x/sys/unix"
)

func NSEnter(pid int) error {
	logrus.Debugf("NSEnter(%d)", pid)
	runtime.LockOSThread()
	path := filepath.Join("/proc", strconv.Itoa(pid), "ns", "net")

	f, err := os.Open(path)
	if err != nil {
		return err
	}

	return unix.Setns(int(f.Fd()), syscall.CLONE_NEWNET)
}

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

	go tun.ReadLoop()

	cmd.Wait()
}
