package host

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func runTcpHostToContainer(b *testing.B, bufsize int) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Skip(err)
	}
	defer listener.Close()

	port := strings.Split(listener.Addr().String(), ":")[1]

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		err := containerCommandNoOutput(b, fmt.Sprintf("nc 10.0.0.100 %s", port))
		assert.NoError(b, err)
		wg.Done()
	}(&wg)

	conn, err := listener.Accept()
	if !assert.NoError(b, err) {
		b.Skipf("Didn't get a connection?? %s", err)
	}

	data := make([]byte, bufsize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		n, err := conn.Write(data)
		assert.NoError(b, err)
		b.SetBytes(int64(n))
	}

	b.StopTimer()
	conn.Close()

	wg.Wait()
}

func BenchmarkTcpHostToContainer(b *testing.B) {
	validateHost(b)

	b.Run("4KB", func(b *testing.B) { runTcpHostToContainer(b, 1024*4) })
	b.Run("8KB", func(b *testing.B) { runTcpHostToContainer(b, 1024*8) })
	b.Run("16KB", func(b *testing.B) { runTcpHostToContainer(b, 1024*16) })
	b.Run("32KB", func(b *testing.B) { runTcpHostToContainer(b, 1024*32) })
	b.Run("64KB", func(b *testing.B) { runTcpHostToContainer(b, 1024*64) })
	b.Run("1MB", func(b *testing.B) { runTcpHostToContainer(b, 1024*1024) })
	b.Run("2MB", func(b *testing.B) { runTcpHostToContainer(b, 1024*1024*2) })
	b.Run("4MB", func(b *testing.B) { runTcpHostToContainer(b, 1024*1024*4) })
	b.Run("8MB", func(b *testing.B) { runTcpHostToContainer(b, 1024*1024*8) })
	b.Run("16MB", func(b *testing.B) { runTcpHostToContainer(b, 1024*1024*16) })
	b.Run("32MB", func(b *testing.B) { runTcpHostToContainer(b, 1024*1024*32) })
	b.Run("64MB", func(b *testing.B) { runTcpHostToContainer(b, 1024*1024*64) })
}
