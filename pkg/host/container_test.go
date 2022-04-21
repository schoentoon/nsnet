package host

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func containerCommandNoOutput(tb testing.TB, command string) error {
	dir := setupRootfs(tb)

	cmd := initialContainerCmd(tb, dir)

	opts := DefaultOptions()
	opts.TCPOptions.AllowHostConnections = true
	tun, err := New(opts)
	assert.NoError(tb, err)
	defer tun.Close()

	tun.AttachToCmd(cmd)

	stdin := bytes.NewBufferString(fmt.Sprintf("%s\n", command))

	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Stdin = stdin

	return cmd.Run()
}

func containerCommandOutput(tb testing.TB, command string) (error, string, *TCPStats, *UDPStats) {
	dir := setupRootfs(tb)

	cmd := initialContainerCmd(tb, dir)

	opts := DefaultOptions()
	opts.UDPOptions.Stats = true
	opts.TCPOptions.Stats = true
	tun, err := New(opts)
	assert.NoError(tb, err)
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
	validateHost(t)

	err, out, _, _ := containerCommandOutput(t, "ip a")
	assert.NoError(t, err)

	assert.Regexp(t, `(?s)tun0.+inet 10\.0\.0\.1/24 brd 10\.0\.0\.255`, out)
}

func TestNSLookup(t *testing.T) {
	validateHost(t)

	err, _, _, udp := containerCommandOutput(t, "nslookup google.com 1.1.1.1")
	assert.NoError(t, err)

	assert.Greater(t, atomic.LoadUint64(&udp.RecvBytes), uint64(0))
	assert.Greater(t, atomic.LoadUint32(&udp.RecvPacket), uint32(0))
	assert.Greater(t, atomic.LoadUint64(&udp.SentBytes), uint64(0))
	assert.Greater(t, atomic.LoadUint32(&udp.SentPacket), uint32(0))
}

func TestNetcatConnect(t *testing.T) {
	validateHost(t)

	err, _, tcp, _ := containerCommandOutput(t, "echo 'GET /\n' | nc 1.1.1.1 80")
	assert.NoError(t, err)

	assert.Greater(t, atomic.LoadUint64(&tcp.RecvBytes), uint64(0))
	assert.Greater(t, atomic.LoadUint64(&tcp.SentBytes), uint64(0))
	assert.Equal(t, uint32(1), atomic.LoadUint32(&tcp.Conns))
}

func TestConnectToHost(t *testing.T) {
	validateHost(t)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip(err)
	}
	defer listener.Close()

	port := strings.Split(listener.Addr().String(), ":")[1]

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		err := containerCommandNoOutput(t, fmt.Sprintf("echo test | nc 10.0.0.100 %s", port))
		assert.NoError(t, err)
		wg.Done()
	}(&wg)

	conn, err := listener.Accept()
	if !assert.NoError(t, err) {
		t.Skipf("Didn't get a connection?? %s", err)
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	assert.NoError(t, err)

	assert.Equal(t, []byte("test\n"), buf[:n])

	conn.Close()

	wg.Wait()
}
