package host

import (
	"fmt"
	"net"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/sirupsen/logrus"
)

func (t *TunDevice) getUdpConn(ip4 *layers.IPv4, udp *layers.UDP) (*net.UDPConn, error) {
	key := udp.TransportFlow().FastHash()
	t.udpPoolLock.RLock()
	conn, ok := t.udpPool[key]
	t.udpPoolLock.RUnlock()
	if !ok {
		return t.createUdpConn(ip4, udp)
	}
	return conn, nil
}

func (t *TunDevice) createUdpConn(ip4 *layers.IPv4, udp *layers.UDP) (*net.UDPConn, error) {
	conn, err := net.Dial("udp", fmt.Sprintf("%s:%d", ip4.DstIP, udp.DstPort))
	if err != nil {
		return nil, err
	}
	udpConn := conn.(*net.UDPConn)
	key := udp.TransportFlow().FastHash()
	t.udpPoolLock.Lock()
	t.udpPool[key] = udpConn
	t.udpPoolLock.Unlock()

	go func() {
		err := t.forwardUdp(udpConn, udp.SrcPort, udp.DstPort, ip4.SrcIP, ip4.DstIP)
		if err != nil {
			logrus.Error(err)
		}
	}()

	return udpConn, nil
}

func (t *TunDevice) handleUDP(ip4 *layers.IPv4, udp *layers.UDP) error {
	conn, err := t.getUdpConn(ip4, udp)
	if err != nil {
		return err
	}

	_, err = conn.Write(udp.Payload)
	return err
}

func (t *TunDevice) forwardUdp(udpConn *net.UDPConn, srcPort, dstPort layers.UDPPort, srcIp, dstIp net.IP) error {
	buf := make([]byte, MTU)

	for {
		n, err := udpConn.Read(buf)
		if err != nil {
			return err
		}

		buffer := gopacket.NewSerializeBuffer()

		ip4 := layers.IPv4{
			Version:  4,
			DstIP:    srcIp,
			SrcIP:    dstIp,
			Protocol: layers.IPProtocolUDP,
		}

		udp := layers.UDP{
			SrcPort: dstPort,
			DstPort: srcPort,
		}
		udp.SetNetworkLayerForChecksum(&ip4)

		err = gopacket.SerializeLayers(buffer,
			gopacket.SerializeOptions{
				FixLengths:       true,
				ComputeChecksums: true,
			},
			&ip4,
			&udp,
			gopacket.Payload(buf[:n]),
		)
		if err != nil {
			return err
		}
		_, err = t.writePipe.Write(buffer.Bytes())
		if err != nil {
			return err
		}
	}
}
