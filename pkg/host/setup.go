package host

import (
	"errors"
	"io"
	"os"
	"os/exec"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
)

const MTU = 1500
const nicID = 1

type TunDevice struct {
	readPipe   io.ReadCloser  // host
	wReadPipe  *os.File       // child
	writePipe  io.WriteCloser // host
	rWritePipe *os.File       // child

	stack      *stack.Stack
	dispatcher stack.NetworkDispatcher

	udpHandler *udpHandler
	tcpHandler *tcpHandler
}

func New() (out *TunDevice, err error) {
	out = &TunDevice{
		stack: stack.New(stack.Options{
			NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol, ipv6.NewProtocol},
			TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, udp.NewProtocol},
		}),
	}

	out.readPipe, out.wReadPipe, err = Pipe()
	if err != nil {
		return nil, err
	}
	out.rWritePipe, out.writePipe, err = Pipe()
	if err != nil {
		return nil, err
	}

	out.stack.AddRoute(tcpip.Route{Destination: header.IPv4EmptySubnet, NIC: 1})

	udpHandler, err := newUdpForwarder(out, 4)
	if err != nil {
		return nil, err
	}
	out.udpHandler = udpHandler

	tcpHandler, err := newTcpForwarder(out)
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
		AddressWithPrefix: tcpip.Address("10.0.0.1").WithPrefix(),
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
	t.readPipe.Close()
	t.wReadPipe.Close()
	t.writePipe.Close()
	t.rWritePipe.Close()
	t.udpHandler.Close()
	return nil
}

func (t *TunDevice) AttachToCmd(cmd *exec.Cmd) {
	cmd.ExtraFiles = []*os.File{t.wReadPipe, t.rWritePipe}
}
