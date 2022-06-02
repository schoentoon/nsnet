package main

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/docker/docker/pkg/reexec"
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

	// iperf3 needs /dev/urandom, so we bind mount it
	if err := bindMount(wd, "/dev/urandom"); err != nil {
		logrus.Fatal(err)
	}

	if err := pivotRoot(wd); err != nil {
		logrus.Fatal(err)
	}

	// we need /tmp to be able to run iperf3
	if err := os.Mkdir("/tmp", 0700); err != nil && !os.IsExist(err) {
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

func bindMount(newroot, mount string) error {
	target := filepath.Join(newroot, mount)

	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}

	stat, err := os.Stat(mount)
	if err != nil {
		return err
	}

	// basically a touch call to create the file, otherwise we can't mount to it.
	f, err := os.OpenFile(target, os.O_CREATE, stat.Mode())
	if err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}

	return unix.Mount(mount, target, "bind", unix.MS_BIND, "")
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
