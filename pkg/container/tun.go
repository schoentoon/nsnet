package container

import (
	"fmt"
	"io"
	"net"
	"os"

	"github.com/schoentoon/nsnet/pkg/common"
	"github.com/songgao/water"
	"github.com/vishvananda/netlink"
	"go.uber.org/multierr"
)

type TunDevice struct {
	iface *water.Interface

	bridge io.ReadWriteCloser
}

func New(fdOffset int) (*TunDevice, error) {
	bridge := os.NewFile(uintptr(3+fdOffset), "bridge")
	if bridge == nil {
		return nil, fmt.Errorf("%d is not a valid file descriptor, wrong offset?", 3+fdOffset)
	}

	config := water.Config{
		DeviceType: water.TUN,
	}
	config.Name = "tun0"

	ifce, err := water.New(config)
	if err != nil {
		return nil, err
	}

	return &TunDevice{
		iface:  ifce,
		bridge: bridge,
	}, nil
}

func (t *TunDevice) Close() error {
	return multierr.Combine(t.iface.Close(),
		t.bridge.Close(),
	)
}

func (t *TunDevice) SetupNetwork() error {
	link, err := netlink.LinkByName(t.iface.Name())
	if err != nil {
		return err
	}

	addr := &netlink.Addr{
		IPNet: &net.IPNet{
			IP:   net.IPv4(10, 0, 0, 1),
			Mask: net.IPv4Mask(255, 255, 255, 0),
		},
	}
	err = netlink.AddrAdd(link, addr)
	if err != nil {
		return err
	}

	err = netlink.LinkSetUp(link)
	if err != nil {
		return err
	}

	err = netlink.LinkSetMTU(link, common.MTU)
	if err != nil {
		return err
	}

	route := &netlink.Route{
		Scope:     netlink.SCOPE_UNIVERSE,
		LinkIndex: link.Attrs().Index,
		Gw:        net.IPv4(10, 0, 0, 1),
	}
	return netlink.RouteAdd(route)
}

func (t *TunDevice) ReadLoop() {
	buf := make([]byte, common.MTU)
	_, _ = io.CopyBuffer(t.bridge, t.iface, buf)
}

func (t *TunDevice) WriteLoop() {
	buf := make([]byte, common.MTU)
	_, _ = io.CopyBuffer(t.iface, t.bridge, buf)
}
