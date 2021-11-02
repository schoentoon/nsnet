package host

import (
	"io"
	"net"
	"os"
	"os/exec"
	"sync"
)

const MTU = 1500

type TunDevice struct {
	readPipe   io.ReadCloser  // host
	wReadPipe  *os.File       // child
	writePipe  io.WriteCloser // host
	rWritePipe *os.File       // child

	udpPoolLock sync.RWMutex
	udpPool     map[uint64]*net.UDPConn
}

func New() (out *TunDevice, err error) {
	out = &TunDevice{
		udpPool: make(map[uint64]*net.UDPConn, 32),
	}

	out.readPipe, out.wReadPipe, err = os.Pipe()
	if err != nil {
		return nil, err
	}
	out.rWritePipe, out.writePipe, err = os.Pipe()
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (t *TunDevice) Close() error {
	t.readPipe.Close()
	t.wReadPipe.Close()
	t.writePipe.Close()
	t.rWritePipe.Close()
	return nil
}

func (t *TunDevice) Attach(cmd *exec.Cmd) {
	cmd.ExtraFiles = []*os.File{t.wReadPipe, t.rWritePipe}
}
