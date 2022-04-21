package host

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/docker/docker/pkg/reexec"
	"github.com/schoentoon/nsnet/pkg/container"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func locateBusybox() (string, error) {
	return exec.LookPath("busybox")
}

func validateHost(tb testing.TB) {
	_, err := os.Stat("/dev/net/tun")
	if os.IsNotExist(err) {
		tb.Skip("/dev/net/tun not found, skipping.")
	}

	_, err = locateBusybox()
	if err != nil {
		tb.Skipf("Failed to find busybox, skipping: %s", err)
	}
}

func setupRootfs(tb testing.TB) string {
	orgBusybox, err := locateBusybox()
	if err != nil {
		tb.Skipf("Failed to find busybox, skipping: %s", err)
	}

	dir := tb.TempDir()

	f, err := os.OpenFile(filepath.Join(dir, "busybox"), os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		tb.Skipf("Failed setupRootfs: %s", err)
	}
	defer f.Close()

	org, err := os.Open(orgBusybox)
	if err != nil {
		tb.Skipf("Failed setupRootfs: %s", err)
	}
	defer org.Close()

	_, err = io.Copy(f, org)
	if err != nil {
		tb.Skipf("Failed setupRootfs: %s", err)
	}

	return dir
}

func initialContainerCmd(tb testing.TB, dir string) *exec.Cmd {
	cmd := reexec.Command("namespace")
	cmd.Env = []string{fmt.Sprintf("DIR=%s", dir)}
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
	return cmd
}

func init() {
	reexec.Register("namespace", namespace)
	if reexec.Init() {
		os.Exit(0)
	}
}

func namespace() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetReportCaller(true)

	wd := os.Getenv("DIR")

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
