package host

import (
	"os"
	"syscall"
)

/** This is pretty much a direct copy pasta from the original os.Pipe() call, which can be found here
 * https://cs.opensource.google/go/go/+/refs/tags/go1.17.3:src/os/pipe_linux.go;l=32
 * The main difference however is that we call Pipe2 with the O_DIRECT flag, which puts the pipe in 'packet' mode
 * as described in the official docs. This does however require Linux 3.4 or higher, but then again.. it's 2021, lol.
 * As an illustration about why this change was made, have a before and after benchmark using iperf..
 * Before:
[  5]   0.00-1.00   sec  28.2 MBytes   236 Mbits/sec
[  5]   1.00-2.00   sec  16.9 MBytes   142 Mbits/sec
[  5]   2.00-3.00   sec  4.51 MBytes  37.8 Mbits/sec
[  5]   3.00-4.00   sec  8.05 MBytes  67.5 Mbits/sec
[  5]   4.00-5.00   sec  15.7 MBytes   132 Mbits/sec
[  5]   5.00-6.00   sec  21.1 MBytes   177 Mbits/sec
[  5]   6.00-7.00   sec  9.34 MBytes  78.3 Mbits/sec
[  5]   7.00-8.00   sec  13.9 MBytes   117 Mbits/sec
[  5]   8.00-9.00   sec  10.6 MBytes  88.6 Mbits/sec
[  5]   9.00-10.00  sec  9.11 MBytes  76.4 Mbits/sec
- - - - - - - - - - - - - - - - - - - - - - - - -
[ ID] Interval           Transfer     Bitrate
[  5]   0.00-10.00  sec   137 MBytes   115 Mbits/sec
  *
  * After:
[  5]   0.00-1.00   sec   205 MBytes  1.72 Gbits/sec
[  5]   1.00-2.00   sec   194 MBytes  1.63 Gbits/sec
[  5]   2.00-3.00   sec   202 MBytes  1.69 Gbits/sec
[  5]   3.00-4.00   sec   189 MBytes  1.59 Gbits/sec
[  5]   4.00-5.00   sec   195 MBytes  1.64 Gbits/sec
[  5]   5.00-6.00   sec   193 MBytes  1.62 Gbits/sec
[  5]   6.00-7.00   sec   195 MBytes  1.64 Gbits/sec
[  5]   7.00-8.00   sec   195 MBytes  1.64 Gbits/sec
[  5]   8.00-9.00   sec   196 MBytes  1.64 Gbits/sec
[  5]   9.00-10.00  sec   203 MBytes  1.70 Gbits/sec
[  5]  10.00-10.00  sec   445 KBytes  1.32 Gbits/sec
- - - - - - - - - - - - - - - - - - - - - - - - -
[ ID] Interval           Transfer     Bitrate
[  5]   0.00-10.00  sec  1.92 GBytes  1.65 Gbits/sec
**/

func Pipe() (r *os.File, w *os.File, err error) {
	var p [2]int

	e := syscall.Pipe2(p[0:], syscall.O_CLOEXEC|syscall.O_DIRECT)
	// pipe2 was added in 2.6.27 and our minimum requirement is 2.6.23, so it
	// might not be implemented.
	if e == syscall.ENOSYS {
		// See ../syscall/exec.go for description of lock.
		syscall.ForkLock.RLock()
		e = syscall.Pipe(p[0:])
		if e != nil {
			syscall.ForkLock.RUnlock()
			return nil, nil, os.NewSyscallError("pipe", e)
		}
		syscall.CloseOnExec(p[0])
		syscall.CloseOnExec(p[1])
		syscall.ForkLock.RUnlock()
	} else if e != nil {
		return nil, nil, os.NewSyscallError("pipe2", e)
	}

	return os.NewFile(uintptr(p[0]), "pipeR"), os.NewFile(uintptr(p[1]), "pipeW"), nil
}
