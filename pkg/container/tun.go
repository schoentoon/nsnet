package container

import (
	"net"

	"github.com/sirupsen/logrus"
	"github.com/songgao/water"
	"github.com/vishvananda/netlink"
	"golang.org/x/net/ipv4"
)

type TunDevice struct {
	iface *water.Interface
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
		iface: ifce,
	}, nil
}

func (t *TunDevice) Close() error {
	return t.iface.Close()
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

func (t *TunDevice) ReadLoop() error {
	buf := make([]byte, 1500)
	for {
		err := t.readPacket(buf)
		if err != nil {
			logrus.Error(err)
		}
	}
}

func (t *TunDevice) readPacket(buf []byte) error {
	n, err := t.iface.Read(buf)
	if err != nil {
		return err
	}

	header, err := ipv4.ParseHeader(buf[:n])
	if err != nil {
		return err
	}

	logrus.Info(header)
	return nil
}
