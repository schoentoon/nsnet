package main

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/docker/docker/pkg/reexec"
	"github.com/schoentoon/nsnet/pkg/container"
	"github.com/sirupsen/logrus"
	"github.com/syndtr/gocapability/capability"
	"golang.org/x/sys/unix"
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

	ifce, err := container.New(0)
	if err != nil {
		logrus.Fatal(err)
	}
	defer ifce.Close()

	if err := pivotRoot(wd); err != nil {
		logrus.Fatal(err)
	}

	err = ifce.SetupNetwork()
	if err != nil {
		logrus.Fatal(err)
	}

	go ifce.ReadLoop()
	go ifce.WriteLoop()

	err = unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0)
	if err != nil {
		logrus.Fatal(err)
	}

	caps, err := capability.NewPid2(0)
	if err != nil {
		logrus.Fatal(err)
	}

	caps.Clear(capability.CAPS | capability.BOUNDING | capability.AMBIENT)

	err = caps.Apply(capability.CAPS | capability.BOUNDING | capability.AMBIENT)
	if err != nil {
		logrus.Fatal(err)
	}

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
	if err := unix.Mount(newroot, newroot, "", unix.MS_BIND|unix.MS_REC, ""); err != nil {
		return err
	}

	// create putold directory
	if err := os.MkdirAll(putold, 0700); err != nil {
		return err
	}

	// call pivot_root
	if err := unix.PivotRoot(newroot, putold); err != nil {
		return err
	}

	// ensure current working directory is set to new root
	if err := os.Chdir("/"); err != nil {
		return err
	}

	// umount putold, which now lives at /.pivot_root
	putold = "/.pivot_root"
	if err := unix.Unmount(putold, unix.MNT_DETACH); err != nil {
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

	if err := os.MkdirAll(target, 0550); err != nil {
		return err
	}
	if err := unix.Mount(source, target, fstype, uintptr(flags), data); err != nil {
		return err
	}

	return nil
}
