package container

import (
	"os"
	"path/filepath"
	"syscall"
)

// MountTunDev this will bind mount /dev/net/tun into the container rootfs, this is intended to be ran inside a container BEFORE pivot root/chroot
func MountTunDev(newroot string) error {
	if err := os.MkdirAll("/dev/net", 0700); err != nil {
		return err
	}

	return syscall.Mount("/dev/net/tun", filepath.Join(newroot, "/dev/net/tun"), "bind", syscall.MS_BIND, "")
}
