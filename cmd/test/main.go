package main

import (
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/docker/docker/pkg/reexec"
	"github.com/schoentoon/nsnet/pkg/host"
	"github.com/sirupsen/logrus"
)

func main() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetReportCaller(true)

	err := os.MkdirAll("/tmp/newroot", 0775)
	if err != nil {
		logrus.Error(err)
	}
	// we assume that this busybox binary is a STATIC binary, otherwise none of this will work as the container doesn't have a libc
	err = cp("/bin/busybox")
	if err != nil {
		logrus.Error(err)
	}

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

	opts := host.DefaultOptions()
	opts.UDPOptions.Stats = true
	opts.TCPOptions.Stats = true
	tun, err := host.New(opts)
	if err != nil {
		logrus.Fatal(err)
	}

	tun.AttachToCmd(cmd)

	err = cmd.Start()
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Infof("Container is at pid: %d", cmd.Process.Pid)

	err = cmd.Wait()
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Debugf("%+v", tun.UDPStats())
	logrus.Debugf("%+v", tun.TCPStats())
}

func cp(src string) error {
	dst := filepath.Join("/tmp/newroot", filepath.Base(src))

	srcf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcf.Close()

	dstf, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer dstf.Close()

	_, err = io.Copy(dstf, srcf)
	return err
}
