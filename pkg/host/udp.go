package host

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"gvisor.dev/gvisor/pkg/tcpip/buffer"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
)

var udpTimeout = time.Second * 60

type UDPOptions struct {
	Threads   int
	QueueSize int
	Stats     bool
}

type udpHandler struct {
	pool  sync.Map
	queue chan udpPacket
	tun   *TunDevice

	stats *UDPStats
}

type UDPStats struct {
	SentPacket uint32
	RecvPacket uint32

	SentBytes uint64
	RecvBytes uint64
}

type udpPacket struct {
	data buffer.VectorisedView
	id   *stack.TransportEndpointID
}

func (p *udpPacket) Data() []byte {
	return p.data.ToView()
}

func (p *udpPacket) ID() *stack.TransportEndpointID {
	return p.id
}

func (p *udpPacket) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: net.IP(p.ID().LocalAddress), Port: int(p.ID().LocalPort)}
}

func (p *udpPacket) RemoteAddr() net.Addr {
	return &net.UDPAddr{IP: net.IP(p.ID().RemoteAddress), Port: int(p.ID().RemotePort)}
}

func (p *udpPacket) Key() string {
	return fmt.Sprintf("%s:%d-%s:%d",
		p.ID().LocalAddress,
		p.ID().LocalPort,
		p.ID().RemoteAddress,
		p.ID().RemotePort)
}

func newUdpForwarder(t *TunDevice, opts UDPOptions) (*udpHandler, error) {
	out := &udpHandler{
		queue: make(chan udpPacket, opts.QueueSize),
		tun:   t,
	}

	if opts.Stats {
		out.stats = new(UDPStats)
	}

	udpHandler := func(id stack.TransportEndpointID, pkt *stack.PacketBuffer) bool {
		hdr := header.UDP(pkt.TransportHeader().View())
		if int(hdr.Length()) > pkt.Data().Size()+header.UDPMinimumSize {
			return true
		}

		if out.stats != nil {
			atomic.AddUint32(&out.stats.SentPacket, 1)
			atomic.AddUint64(&out.stats.SentBytes, uint64(pkt.Size()))
		}

		// TODO: Check checksum?

		packet := udpPacket{
			data: pkt.Data().ExtractVV(),
			id:   &id,
		}

		select {
		case out.queue <- packet:
		default:
			logrus.Warn("UDP Queue full, dropping packet")
		}

		return true
	}

	t.stack.SetTransportProtocolHandler(udp.ProtocolNumber, udpHandler)

	for i := 0; i < opts.Threads; i++ {
		go out.loop()
	}

	return out, nil
}

func (h *udpHandler) Close() error {
	close(h.queue)
	return nil
}

func (h *udpHandler) Stats() *UDPStats {
	return h.stats
}

func (t *TunDevice) UDPStats() *UDPStats {
	return t.udpHandler.Stats()
}

func (h *udpHandler) loop() {
	for packet := range h.queue {
		err := h.handlePacket(packet)
		if err != nil {
			logrus.Error(err)
		}
	}
}

func (h *udpHandler) getOrCreateConn(packet udpPacket) (out *net.UDPConn, err error) {
	key := packet.Key()
	val, ok := h.pool.Load(key)
	if !ok {
		addr := packet.LocalAddr()
		conn, err := net.Dial("udp", addr.String())
		if err != nil {
			return nil, err
		}
		val, stored := h.pool.LoadOrStore(key, conn)
		if stored { // if this is true it was stored elsewhere in the meantime, so we close ours
			_ = conn.Close()
		} else {
			go h.udpForwarder(val.(*net.UDPConn), packet.ID(), packet.Key())
		}
		return val.(*net.UDPConn), nil
	}
	return val.(*net.UDPConn), nil
}

func (h *udpHandler) removeConn(key string) {
	h.pool.Delete(key)
}

func (h *udpHandler) handlePacket(packet udpPacket) error {
	conn, err := h.getOrCreateConn(packet)
	if err != nil {
		return err
	}

	_, err = conn.Write(packet.Data())

	return err
}

func (h *udpHandler) udpForwarder(conn *net.UDPConn, id *stack.TransportEndpointID, key string) {
	defer conn.Close()
	defer h.removeConn(key)

	buf := make([]byte, MTU)
	r, tcpipErr := h.tun.stack.FindRoute(nicID,
		id.LocalAddress, id.RemoteAddress,
		header.IPv4ProtocolNumber, false)
	if tcpipErr != nil {
		return
	}
	defer r.Release()

	for {
		_ = conn.SetReadDeadline(time.Now().Add(udpTimeout))

		n, err := conn.Read(buf)
		if err != nil {
			return
		}

		pkt := stack.NewPacketBuffer(stack.PacketBufferOptions{
			ReserveHeaderBytes: header.UDPMinimumSize + int(r.MaxHeaderLength()),
			Data:               buffer.NewVectorisedView(n, []buffer.View{buffer.NewViewFromBytes(buf[:n])}),
		})

		udpHdr := header.UDP(pkt.TransportHeader().Push(header.UDPMinimumSize))
		pkt.TransportProtocolNumber = udp.ProtocolNumber

		length := uint16(pkt.Size())
		udpHdr.Encode(&header.UDPFields{
			SrcPort: id.LocalPort,
			DstPort: id.RemotePort,
			Length:  length,
		})

		// Set the checksum field unless TX checksum offload is enabled.
		// On IPv4, UDP checksum is optional, and a zero value indicates the
		// transmitter skipped the checksum generation (RFC768).
		// On IPv6, UDP checksum is not optional (RFC2460 Section 8.1).
		if r.RequiresTXTransportChecksum() &&
			(r.NetProto() == header.IPv6ProtocolNumber) {
			xsum := r.PseudoHeaderChecksum(udp.ProtocolNumber, length)
			for _, v := range pkt.Data().Views() {
				xsum = header.Checksum(v, xsum)
			}
			udpHdr.SetChecksum(^udpHdr.CalculateChecksum(xsum))
		}

		ttl := r.DefaultTTL()

		if tcpipErr := r.WritePacket(stack.NetworkHeaderParams{
			Protocol: udp.ProtocolNumber,
			TTL:      ttl,
			TOS:      0, /* default */
		}, pkt); tcpipErr != nil {
			return
		}

		if h.stats != nil {
			atomic.AddUint32(&h.stats.RecvPacket, 1)
			atomic.AddUint64(&h.stats.RecvBytes, uint64(pkt.Size()))
		}
	}
}
