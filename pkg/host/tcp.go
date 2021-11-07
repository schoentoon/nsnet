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
const maxConnAttemps = 1024
const tcpKeepAliveIdle = time.Second * 60
const tcpKeepaliveInterval = time.Second * 30

type tcpHandler struct {
	tun *TunDevice
}

// mostly based on https://github.com/xjasonlyu/tun2socks/blob/main/tunnel/tcp.go
// and https://github.com/xjasonlyu/tun2socks/blob/main/core/stack/tcp.go
func newTcpForwarder(t *TunDevice) (*tcpHandler, error) {
	out := &tcpHandler{
		tun: t,
	}

	tcpForwarder := tcp.NewForwarder(t.stack, defaultWndSize, maxConnAttemps, func(r *tcp.ForwarderRequest) {
		var wq waiter.Queue
		id := r.ID()
		ep, err := r.CreateEndpoint(&wq)
		if err != nil {
			r.Complete(true)
			return
		}
		r.Complete(false)

		out.setKeepalive(ep)

		go out.handleTcp(gonet.NewTCPConn(&wq, ep), &id)
	})

	t.stack.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpForwarder.HandlePacket)

	return out, nil
}

func (h *tcpHandler) Close() error {
	return nil
}

func (h *tcpHandler) setKeepalive(ep tcpip.Endpoint) {
	ep.SocketOptions().SetKeepAlive(true)

	idle := tcpip.KeepaliveIdleOption(tcpKeepAliveIdle)
	if err := ep.SetSockOpt(&idle); err != nil {
		return
	}

	interval := tcpip.KeepaliveIntervalOption(tcpKeepaliveInterval)
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
