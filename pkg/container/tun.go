package container

import (
	"io"
	"net"
	"os"

	"github.com/songgao/water"
	"github.com/vishvananda/netlink"
	"go.uber.org/multierr"
)

type TunDevice struct {
	iface *water.Interface

	bridge io.ReadWriteCloser
}

func New() (*TunDevice, error) {
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
		bridge: os.NewFile(3, "bridge"),
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

	route := &netlink.Route{
		Scope:     netlink.SCOPE_UNIVERSE,
		LinkIndex: link.Attrs().Index,
		Gw:        net.IPv4(10, 0, 0, 1),
	}
	return netlink.RouteAdd(route)
}

func (t *TunDevice) ReadLoop() {
	_, _ = io.Copy(t.bridge, t.iface)
}

func (t *TunDevice) WriteLoop() {
	_, _ = io.Copy(t.iface, t.bridge)
}
