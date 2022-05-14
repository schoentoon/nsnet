package host

import (
	"github.com/schoentoon/nsnet/pkg/common"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/buffer"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

// this struct mostly exists to hide the methods implemented for gvisor interfaces from the public API
type tunEndPoint struct {
	tun *TunDevice
}

// MTU is the maximum transmission unit for this endpoint. This is
// usually dictated by the backing physical network; when such a
// physical network doesn't exist, the limit is generally 64k, which
// includes the maximum size of an IP packet.
func (t *tunEndPoint) MTU() uint32 {
	return common.MTU
}

// MaxHeaderLength returns the maximum size the data link (and
// lower level layers combined) headers can have. Higher levels use this
// information to reserve space in the front of the packets they're
// building.
func (t *tunEndPoint) MaxHeaderLength() uint16 {
	return 0
}

// LinkAddress returns the link address (typically a MAC) of the
// endpoint.
func (t *tunEndPoint) LinkAddress() tcpip.LinkAddress {
	return ""
}

// Capabilities returns the set of capabilities supported by the
// endpoint.
func (t *tunEndPoint) Capabilities() stack.LinkEndpointCapabilities {
	return stack.CapabilityNone
}

// Attach attaches the data link layer endpoint to the network-layer
// dispatcher of the stack.
//
// Attach is called with a nil dispatcher when the endpoint's NIC is being
// removed.
func (t *tunEndPoint) Attach(dispatcher stack.NetworkDispatcher) {
	t.tun.dispatcher = dispatcher
	go t.tun.dispatchLoop()
}

// IsAttached returns whether a NetworkDispatcher is attached to the
// endpoint.
func (t *tunEndPoint) IsAttached() bool {
	return t.tun.dispatcher != nil
}

// Wait waits for any worker goroutines owned by the endpoint to stop.
//
// For now, requesting that an endpoint's worker goroutine(s) stop is
// implementation specific.
//
// Wait will not block if the endpoint hasn't started any goroutines
// yet, even if it might later.
func (t *tunEndPoint) Wait() {
}

// ARPHardwareType returns the ARPHRD_TYPE of the link endpoint.
//
// See:
// https://github.com/torvalds/linux/blob/aa0c9086b40c17a7ad94425b3b70dd1fdd7497bf/include/uapi/linux/if_arp.h#L30
func (t *tunEndPoint) ARPHardwareType() header.ARPHardwareType {
	return header.ARPHardwareNone
}

// AddHeader adds a link layer header to pkt if required.
func (t *tunEndPoint) AddHeader(local, remote tcpip.LinkAddress, protocol tcpip.NetworkProtocolNumber, pkt *stack.PacketBuffer) {
}

// WritePacket writes a packet with the given protocol and route.
//
// WritePacket takes ownership of the packet buffer. The packet buffer's
// network and transport header must be set.
//
// To participate in transparent bridging, a LinkEndpoint implementation
// should call eth.Encode with header.EthernetFields.SrcAddr set to
// r.LocalLinkAddress if it is provided.
func (t *tunEndPoint) WritePacket(_ stack.RouteInfo, _ tcpip.NetworkProtocolNumber, pkt *stack.PacketBuffer) tcpip.Error {
	view := buffer.NewVectorisedView(pkt.Size(), pkt.Views())
	if _, err := t.tun.bridge.Write(view.ToView()); err != nil {
		return &tcpip.ErrInvalidEndpointState{}
	}
	return nil
}

// WritePackets writes packets with the given protocol and route. Must not be
// called with an empty list of packet buffers.
//
// WritePackets takes ownership of the packet buffers.
//
// Right now, WritePackets is used only when the software segmentation
// offload is enabled. If it will be used for something else, syscall filters
// may need to be updated.
func (t *tunEndPoint) WritePackets(route stack.RouteInfo, pkts stack.PacketBufferList, proto tcpip.NetworkProtocolNumber) (int, tcpip.Error) {
	n := 0
	for pkt := pkts.Front(); pkt != nil; pkt = pkt.Next() {
		if err := t.WritePacket(route, proto, pkt); err != nil {
			break
		}
		n++
	}
	return n, nil
}

// WriteRawPacket writes a packet directly to the link.
//
// If the link-layer has its own header, the payload must already include the
// header.
//
// WriteRawPacket takes ownership of the packet.
func (t *tunEndPoint) WriteRawPacket(*stack.PacketBuffer) tcpip.Error {
	return &tcpip.ErrNotSupported{}
}
