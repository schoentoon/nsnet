package host

import (
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/sirupsen/logrus"
)

func (t *TunDevice) ReadLoop() error {
	buf := make([]byte, MTU)
	for {
		err := t.readPacket(buf)
		if err != nil {
			logrus.Error(err)
			return err
		}
	}
}

func (t *TunDevice) readPacket(buf []byte) error {
	n, err := t.readPipe.Read(buf)
	if err != nil {
		return err
	}

	packet := gopacket.NewPacket(buf[:n], layers.LayerTypeIPv4, gopacket.DecodeOptions{
		Lazy:   true,
		NoCopy: true,
	})
	transport := packet.TransportLayer()
	if transport == nil {
		return nil
	}

	switch transport.LayerType() {
	case layers.LayerTypeTCP:
		logrus.Debug(transport.(*layers.TCP))
		return nil
	case layers.LayerTypeUDP:
		return t.handleUDP(packet.Layer(layers.LayerTypeIPv4).(*layers.IPv4), transport.(*layers.UDP))
	}

	logrus.Debug(packet)
	return nil
}
