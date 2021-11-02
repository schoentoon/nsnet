package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/docker/docker/pkg/reexec"
	"github.com/sirupsen/logrus"
	"github.com/songgao/water"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

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

	err := cmd.Start()
	if err != nil {
		panic(err)
	}
	time.Sleep(time.Second * 1)

	logrus.Infof("%+v", cmd.Process)
	logrus.Infof("%+v", cmd.SysProcAttr)
	ns, err := netns.GetFromPid(cmd.Process.Pid)
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.Info(ns)

	//go tapInNS(ns)
	//defer cmd.Wait()

	/*err = NSEnter(cmd.Process.Pid)
	if err != nil {
		panic(err)
	}*/

	cmd.Wait()
}

func tapInNS(ns netns.NsHandle) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	err := netns.Set(ns)
	if err != nil {
		logrus.Fatal(err)
	}

	config := water.Config{
		DeviceType: water.TAP,
	}
	config.Name = "tap0"

	ifce, err := water.New(config)
	if err != nil {
		panic(err)
	}
	defer ifce.Close()
}

func tmp() {
	config := water.Config{
		DeviceType: water.TAP,
	}
	config.Name = "tap0"

	ifce, err := water.New(config)
	if err != nil {
		panic(err)
	}
	defer ifce.Close()

	link, err := netlink.LinkByName(ifce.Name())
	if err != nil {
		panic(err)
	}

	err = netlink.LinkSetNsPid(link, 58394)
	if err != nil {
		panic(err)
	}

	time.Sleep(time.Minute)
}
