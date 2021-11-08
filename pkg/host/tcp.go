package host

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/waiter"
)

const defaultWndSize = 0

type TCPOptions struct {
	MaxConns          int
	KeepaliveIdle     time.Duration
	KeepaliveInterval time.Duration
}

type tcpHandler struct {
	tun *TunDevice
}

// mostly based on https://github.com/xjasonlyu/tun2socks/blob/main/tunnel/tcp.go
// and https://github.com/xjasonlyu/tun2socks/blob/main/core/stack/tcp.go
func newTcpForwarder(t *TunDevice, opts TCPOptions) (*tcpHandler, error) {
	out := &tcpHandler{
		tun: t,
	}

	tcpForwarder := tcp.NewForwarder(t.stack, defaultWndSize, opts.MaxConns, func(r *tcp.ForwarderRequest) {
		var wq waiter.Queue
		id := r.ID()
		ep, err := r.CreateEndpoint(&wq)
		if err != nil {
			r.Complete(true)
			return
		}
		r.Complete(false)

		out.setKeepalive(ep, opts)

		go out.handleTcp(gonet.NewTCPConn(&wq, ep), &id)
	})

	t.stack.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpForwarder.HandlePacket)

	return out, nil
}

func (h *tcpHandler) Close() error {
	// TODO: We should kill all the connections
	return nil
}

func (h *tcpHandler) setKeepalive(ep tcpip.Endpoint, opts TCPOptions) {
	ep.SocketOptions().SetKeepAlive(true)

	idle := tcpip.KeepaliveIdleOption(opts.KeepaliveIdle)
	if err := ep.SetSockOpt(&idle); err != nil {
		return
	}

	interval := tcpip.KeepaliveIntervalOption(opts.KeepaliveInterval)
	if err := ep.SetSockOpt(&interval); err != nil {
		return
	}
}

func (h *tcpHandler) handleTcp(conn *gonet.TCPConn, id *stack.TransportEndpointID) error {
	defer conn.Close()

	target, err := net.Dial("tcp", fmt.Sprintf("%s:%d", id.LocalAddress, id.LocalPort))
	if err != nil {
		return err
	}
	defer target.Close()

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _ = io.Copy(target, conn)
	}()

	go func() {
		defer wg.Done()
		_, _ = io.Copy(conn, target)
	}()

	wg.Wait()
	return nil
}
