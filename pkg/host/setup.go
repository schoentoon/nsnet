package host

import (
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"time"

	"go.uber.org/multierr"
	"golang.org/x/sys/unix"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
)

const nicID = 1

type Options struct {
	UDPOptions UDPOptions
	TCPOptions TCPOptions
}

func DefaultOptions() Options {
	return Options{
		UDPOptions: UDPOptions{
			QueueSize: 4096,
			Threads:   16,
			Stats:     false,
		},
		TCPOptions: TCPOptions{
			MaxConns:          2048,
			KeepaliveIdle:     time.Second * 60,
			KeepaliveInterval: time.Second * 30,
			Stats:             false,
		},
	}
}

type TunDevice struct {
	bridge io.ReadWriteCloser

	containerFd *os.File

	stack      *stack.Stack
	dispatcher stack.NetworkDispatcher

	udpHandler *udpHandler
	tcpHandler *tcpHandler
}

func New(opts Options) (out *TunDevice, err error) {
	out = &TunDevice{
		stack: stack.New(stack.Options{
			NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol, ipv6.NewProtocol},
			TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, udp.NewProtocol},
		}),
	}

	fds, err := unix.Socketpair(unix.AF_LOCAL, unix.SOCK_STREAM|unix.SOCK_SEQPACKET, 0)
	if err != nil {
		return nil, err
	}

	out.bridge = os.NewFile(uintptr(fds[0]), "bridge")
	out.containerFd = os.NewFile(uintptr(fds[1]), "bridge-container")

	out.stack.AddRoute(tcpip.Route{
		Destination: header.IPv4EmptySubnet,
		NIC:         nicID,
	})

	udpHandler, err := newUdpForwarder(out, opts.UDPOptions)
	if err != nil {
		return nil, err
	}
	out.udpHandler = udpHandler

	tcpHandler, err := newTcpForwarder(out, opts.TCPOptions)
	if err != nil {
		return nil, err
	}
	out.tcpHandler = tcpHandler

	tcpipErr := out.stack.CreateNIC(nicID, out)
	if tcpipErr != nil {
		return nil, errors.New(tcpipErr.String())
	}

	tcpipErr = out.stack.AddProtocolAddress(nicID, tcpip.ProtocolAddress{
		Protocol:          ipv4.ProtocolNumber,
		AddressWithPrefix: tcpip.Address(net.IPv4(10, 0, 0, 1).To4()).WithPrefix(),
	}, stack.AddressProperties{})
	if tcpipErr != nil {
		return nil, errors.New(tcpipErr.String())
	}

	tcpipErr = out.stack.SetPromiscuousMode(1, true)
	if tcpipErr != nil {
		return nil, errors.New(tcpipErr.String())
	}

	tcpipErr = out.stack.SetSpoofing(1, true)
	if tcpipErr != nil {
		return nil, errors.New(tcpipErr.String())
	}

	return out, nil
}

func (t *TunDevice) Close() error {
	return multierr.Combine(t.bridge.Close(),
		t.udpHandler.Close(),
		t.tcpHandler.Close(),
	)
}

func (t *TunDevice) AttachToCmd(cmd *exec.Cmd) {
	cmd.ExtraFiles = []*os.File{t.containerFd}
}
