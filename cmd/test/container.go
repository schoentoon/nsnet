package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/docker/docker/pkg/reexec"
	"github.com/schoentoon/nsnet/pkg/container"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netns"
)

func init() {
	reexec.Register("namespace", namespace)
	if reexec.Init() {
		os.Exit(0)
	}
}

func namespace() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetReportCaller(true)

	wd := "/tmp/newroot"

	if err := mountProc(wd); err != nil {
		logrus.Fatal(err)
	}

	if err := mountTunDev(wd); err != nil {
		logrus.Fatal(err)
	}

	if err := pivotRoot(wd); err != nil {
		logrus.Fatal(err)
	}

	ns, err := netns.Get()
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.Info(ns)

	ifce, err := container.New()
	if err != nil {
		logrus.Fatal(err)
	}
	defer ifce.Close()

	err = ifce.SetupNetwork()
	if err != nil {
		logrus.Fatal(err)
	}

	go ifce.ReadLoop()

	cmd := exec.Command("/busybox", "sh")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	err = cmd.Run()
	if err != nil {
		logrus.Fatal(err)
	}
}

func pivotRoot(newroot string) error {
	putold := filepath.Join(newroot, "/.pivot_root")

	// bind mount newroot to itself - this is a slight hack needed to satisfy the
	// pivot_root requirement that newroot and putold must not be on the same
	// filesystem as the current root
	if err := syscall.Mount(newroot, newroot, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return err
	}

	// create putold directory
	if err := os.MkdirAll(putold, 0700); err != nil {
		return err
	}

	// call pivot_root
	if err := syscall.PivotRoot(newroot, putold); err != nil {
		return err
	}

	// ensure current working directory is set to new root
	if err := os.Chdir("/"); err != nil {
		return err
	}

	// umount putold, which now lives at /.pivot_root
	putold = "/.pivot_root"
	if err := syscall.Unmount(putold, syscall.MNT_DETACH); err != nil {
		return err
	}

	// remove putold
	if err := os.RemoveAll(putold); err != nil {
		return err
	}

	return nil
}

func mountProc(newroot string) error {
	source := "proc"
	target := filepath.Join(newroot, "/proc")
	fstype := "proc"
	flags := 0
	data := ""

	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}
	if err := syscall.Mount(source, target, fstype, uintptr(flags), data); err != nil {
		return err
	}

	return nil
}

func mountTunDev(newroot string) error {
	if err := os.MkdirAll("/dev/net", 0700); err != nil {
		return err
	}

	if err := syscall.Mount("/dev/net/tun", filepath.Join(newroot, "/dev/net/tun"), "bind", syscall.MS_BIND, ""); err != nil {
		return err
	}

	/*if err := unix.Mknod("/dev/net/tun", unix.S_IFCHR|uint32(os.FileMode(0666)), int(unix.Mkdev(10, 200))); err != nil {
		if os.IsExist(err) {
			return nil
		}
		return err
	}*/

	return nil
}
