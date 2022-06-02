# nsnet

[![Gitlab pipeline status](https://gitlab.com/schoentoon/nsnet/badges/master/pipeline.svg)](https://gitlab.com/schoentoon/nsnet)

Yet another way to do networking within namespaces.
This is however aimed at rootless namespaces and not needing any extra software installed.
The only requirement is that you must have a tun device (library assumes this is at /dev/net/tun).
For an example of how to use this library, either have a look at the test program in [cmd/test](./cmd/test).
All the relevant pieces for the host side are in [pkg/host](./pkg/host).
And all the relevant pieces for the container/namespaced side are in [pkg/container](./pkg/container).

## Usage

For the host side the following snippet is enough to get up and running.

```go
opts := host.DefaultOptions()
tun, err := host.New(opts)
if err != nil {
    panic(err)
}

tun.AttachToCmd(cmd)
```

This assumes that cmd is a [exec.Cmd](https://pkg.go.dev/os/exec#Cmd), it's important to call AttachToCmd() before starting this exec.Cmd.
An implementation detail, this will add an internal file descriptor under the ExtraFiles field of exec.Cmd.
Keep this in mind if you're adding file descriptors of your own there.

For the container side, the following snippet is enough to get networking within the network namespace.

```go
ifce, err := container.New(0)
if err != nil {
    panic(err)
}
defer ifce.Close()

if err := pivotRoot(wd); err != nil {
    panic(err)
}

err = ifce.SetupNetwork()
if err != nil {
    panic(err)
}

go ifce.ReadLoop()
go ifce.WriteLoop()
```

In case you're also creating a new mount namespace and intend to pivotroot or chroot, call container.New() before doing this.
As this function will open the /dev/net/tun, by calling it before pivotroot/chroot you won't have to bind mount it into your eventual root.
In case you added extra files to the exec.Cmd for this container process yourself, use the amount of extra files you added yourself as an argument to container.New().
The SetupNetwork() call will assign an ip address and routes to the appropriate network interface.
And finally the ReadLoop() and WriteLoop() calls will start the according loops to actually get network traffic going.

All connections made from within this namespace will now go through the internal socket pair it has with the host process.
Which will decode any TCP/UDP using [the gvisor network stack](https://github.com/google/gvisor/tree/master/pkg/tcpip), to then make the outgoing connections itself using the specified Dialer.
Meaning the host process will get to see all the traffic and could even act as a firewall for the namespaced process.

## Benchmarks

All these benchmarks are performed using [a statically compiled iperf3](https://github.com/userdocs/iperf3-static).
The test programs for this can be found in [./cmd](./cmd), where `test` is using nsnet. And `slirp4netns-test` uses slirp4netns (you will need to have this installed on your system).
They will both use the MTU specified in [./pkg/common/mtu.go](./pkg/common/mtu.go), which at this time of writing is set at `32 * 1024`.
Note: In these tests 192.168.100.123 is my local ip address, which I run iperf3 on in server mode. This is just a reliable way to connect to the outside.

### **netns**

```bash
/ # ./iperf3-amd64 -c 192.168.100.123
Connecting to host 192.168.100.123, port 5201
[  8] local 10.0.0.1 port 44194 connected to 192.168.100.123 port 5201
[ ID] Interval           Transfer     Bitrate         Retr  Cwnd
[  8]   0.00-1.00   sec  1.06 GBytes  9.13 Gbits/sec    0    639 KBytes
[  8]   1.00-2.00   sec  1.04 GBytes  8.96 Gbits/sec    0    639 KBytes
[  8]   2.00-3.00   sec  1.00 GBytes  8.59 Gbits/sec    0    639 KBytes
[  8]   3.00-4.00   sec  1.04 GBytes  8.94 Gbits/sec    0    639 KBytes
[  8]   4.00-5.00   sec  1023 MBytes  8.58 Gbits/sec    0    639 KBytes
[  8]   5.00-6.00   sec  1.03 GBytes  8.81 Gbits/sec    0    639 KBytes
[  8]   6.00-7.00   sec  1.01 GBytes  8.69 Gbits/sec    0    639 KBytes
[  8]   7.00-8.00   sec  1.04 GBytes  8.97 Gbits/sec    0    639 KBytes
[  8]   8.00-9.00   sec  1.02 GBytes  8.75 Gbits/sec    0    639 KBytes
[  8]   9.00-10.00  sec  1.01 GBytes  8.68 Gbits/sec    0    639 KBytes
- - - - - - - - - - - - - - - - - - - - - - - - -
[ ID] Interval           Transfer     Bitrate         Retr
[  8]   0.00-10.00  sec  10.3 GBytes  8.81 Gbits/sec    0             sender
[  8]   0.00-10.00  sec  10.3 GBytes  8.81 Gbits/sec                  receiver

iperf Done.
```

### **slirp4netns**

```bash
/ # ./iperf3-amd64 -c 192.168.100.123
Connecting to host 192.168.100.123, port 5201
[  5] local 10.0.2.100 port 32954 connected to 192.168.100.123 port 5201
[ ID] Interval           Transfer     Bitrate         Retr  Cwnd
[  5]   0.00-1.00   sec  1.46 GBytes  12.5 Gbits/sec    0    320 KBytes
[  5]   1.00-2.00   sec  1.37 GBytes  11.7 Gbits/sec    0    320 KBytes
[  5]   2.00-3.00   sec  1.50 GBytes  12.9 Gbits/sec    0    320 KBytes
[  5]   3.00-4.00   sec  1.18 GBytes  10.2 Gbits/sec    0    320 KBytes
[  5]   4.00-5.00   sec  1.58 GBytes  13.6 Gbits/sec    0    320 KBytes
[  5]   5.00-6.00   sec  1.54 GBytes  13.2 Gbits/sec    0    320 KBytes
[  5]   6.00-7.00   sec  1.15 GBytes  9.88 Gbits/sec    0    320 KBytes
[  5]   7.00-8.00   sec  1.22 GBytes  10.5 Gbits/sec    0    320 KBytes
[  5]   8.00-9.00   sec   892 MBytes  7.48 Gbits/sec    0    320 KBytes
[  5]   9.00-10.00  sec  1.64 GBytes  14.1 Gbits/sec    0    320 KBytes
- - - - - - - - - - - - - - - - - - - - - - - - -
[ ID] Interval           Transfer     Bitrate         Retr
[  5]   0.00-10.00  sec  13.5 GBytes  11.6 Gbits/sec    0             sender
[  5]   0.00-10.00  sec  13.5 GBytes  11.6 Gbits/sec                  receiver

iperf Done.
```
