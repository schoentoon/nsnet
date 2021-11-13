package host

import (
	"bytes"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func containerCommandOutput(t *testing.T, command string) (error, string, *TCPStats, *UDPStats) {
	dir := setupRootfs(t)

	cmd := initialContainerCmd(t, dir)

	opts := DefaultOptions()
	opts.UDPOptions.Stats = true
	opts.TCPOptions.Stats = true
	tun, err := New(opts)
	assert.NoError(t, err)
	defer tun.Close()

	tun.AttachToCmd(cmd)

	stdout := bytes.Buffer{}
	stdin := bytes.NewBufferString(fmt.Sprintf("%s\n", command))

	cmd.Stdout = &stdout
	cmd.Stderr = &stdout
	cmd.Stdin = stdin

	return cmd.Run(), stdout.String(), tun.TCPStats(), tun.UDPStats()
}

func TestSetupIPAddress(t *testing.T) {
	err, out, _, _ := containerCommandOutput(t, "ip a")
	assert.NoError(t, err)

	assert.Regexp(t, `(?s)tun0.+inet 10\.0\.0\.1/24 brd 10\.0\.0\.255`, out)
}

func TestNSLookup(t *testing.T) {
	err, _, _, udp := containerCommandOutput(t, "nslookup google.com 1.1.1.1")
	assert.NoError(t, err)

	assert.Greater(t, atomic.LoadUint64(&udp.RecvBytes), uint64(0))
	assert.Greater(t, atomic.LoadUint32(&udp.RecvPacket), uint32(0))
	assert.Greater(t, atomic.LoadUint64(&udp.SentBytes), uint64(0))
	assert.Greater(t, atomic.LoadUint32(&udp.SentPacket), uint32(0))
}

func TestNetcatConnect(t *testing.T) {
	err, _, tcp, _ := containerCommandOutput(t, "echo 'GET /\n' | nc 1.1.1.1 80")
	assert.NoError(t, err)

	assert.Greater(t, atomic.LoadUint64(&tcp.RecvBytes), uint64(0))
	assert.Greater(t, atomic.LoadUint64(&tcp.SentBytes), uint64(0))
	assert.Equal(t, uint32(1), atomic.LoadUint32(&tcp.Conns))
}
