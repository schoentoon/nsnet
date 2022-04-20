package host

import (
	"github.com/schoentoon/nsnet/pkg/common"
	"gvisor.dev/gvisor/pkg/tcpip/buffer"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

func (t *TunDevice) dispatchLoop() {
	buf := make([]byte, common.MTU)
	for {
		n, err := t.bridge.Read(buf)
		if err != nil {
			return
		}

		pkb := stack.NewPacketBuffer(stack.PacketBufferOptions{
			Data: buffer.NewVectorisedView(n, []buffer.View{buffer.NewViewFromBytes(buf)}),
		})
		switch header.IPVersion(buf) {
		case header.IPv4Version:
			t.dispatcher.DeliverNetworkPacket("", "", ipv4.ProtocolNumber, pkb)
		case header.IPv6Version:
			t.dispatcher.DeliverNetworkPacket("", "", ipv6.ProtocolNumber, pkb)
		}
	}
}
