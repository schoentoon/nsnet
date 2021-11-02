package container

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// MountTunDev this will bind mount /dev/net/tun into the container rootfs, this is intended to be ran inside a container BEFORE pivot root/chroot
func MountTunDev(newroot string) error {
	if err := os.MkdirAll(filepath.Join(newroot, "/dev/net"), 0700); err != nil {
		return err
	}

	f, err := os.Create(filepath.Join(newroot, "/dev/net/tun"))
	if err != nil {
		return err
	}
	f.Close()

	return unix.Mount("/dev/net/tun", filepath.Join(newroot, "/dev/net/tun"), "bind", unix.MS_BIND, "")
}
