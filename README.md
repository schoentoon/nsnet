# nsnet

Yet another way to do networking within namespaces.
This is however aimed at rootless namespaces and not needing any extra software installed.
The only requirement is that you must have a tun device (library assumes this is at /dev/net/tun).
For an example of how to use this library, either have a look at the test program in [./cmd/test/](cmd/test/).
All the relevant pieces for the host side are in [./pkg/host](pkg/host).
And all the relevant pieces for the container/namespaced side are in [./pkg/container](pkg/container).

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

This assumes that cmd is a [https://pkg.go.dev/os/exec#Cmd](exec.Cmd), it's important to call AttachToCmd() before starting this exec.Cmd.
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
Which will decode any TCP/UDP using [https://github.com/google/gvisor/tree/master/pkg/tcpip](the gvisor network stack), to then make the outgoing connections itself using the specified Dialer.
Meaning the host process will get to see all the traffic and could even act as a firewall for the namespaced process.
