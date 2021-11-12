package host

import (
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
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
	Stats             bool
}

type tcpHandler struct {
	tun *TunDevice

	stats *TCPStats
}

type TCPStats struct {
	Conns uint32

	SentBytes uint64
	RecvBytes uint64
}

// mostly based on https://github.com/xjasonlyu/tun2socks/blob/main/tunnel/tcp.go
// and https://github.com/xjasonlyu/tun2socks/blob/main/core/stack/tcp.go
func newTcpForwarder(t *TunDevice, opts TCPOptions) (*tcpHandler, error) {
	out := &tcpHandler{
		tun: t,
	}

	if opts.Stats {
		out.stats = new(TCPStats)
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

		var conn net.Conn
		conn = gonet.NewTCPConn(&wq, ep)
		if out.stats != nil {
			atomic.AddUint32(&out.stats.Conns, 1)
			conn = newTcpTracker(out.stats, conn)
		}

		out.setKeepalive(ep, opts)

		go out.handleTcp(conn, &id)
	})

	t.stack.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpForwarder.HandlePacket)

	return out, nil
}

func (h *tcpHandler) Close() error {
	// TODO: We should kill all the connections
	return nil
}

func (h *tcpHandler) Stats() *TCPStats {
	return h.stats
}

func (t *TunDevice) TCPStats() *TCPStats {
	return t.tcpHandler.Stats()
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

func (h *tcpHandler) handleTcp(conn net.Conn, id *stack.TransportEndpointID) {
	defer conn.Close()

	target, err := net.Dial("tcp", fmt.Sprintf("%s:%d", id.LocalAddress, id.LocalPort))
	if err != nil {
		return
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
}

type tcpTracker struct {
	net.Conn
	stats *TCPStats
}

func newTcpTracker(stats *TCPStats, conn net.Conn) *tcpTracker {
	return &tcpTracker{
		Conn:  conn,
		stats: stats,
	}
}

func (t *tcpTracker) Read(b []byte) (int, error) {
	n, err := t.Conn.Read(b)
	atomic.AddUint64(&t.stats.RecvBytes, uint64(n))
	return n, err
}

func (t *tcpTracker) Write(b []byte) (int, error) {
	n, err := t.Conn.Write(b)
	atomic.AddUint64(&t.stats.SentBytes, uint64(n))
	return n, err
}
