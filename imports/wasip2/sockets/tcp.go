// imports/wasip2/sockets/tcp.go

package sockets

import (
	"context"

	"github.com/tetratelabs/wazero/internal/component"
)

// instantiateTcp registers wasi:sockets/tcp@0.2.0
func instantiateTcp(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:sockets/tcp@0.2.0")

	// tcp-socket resource
	inst.Resource("tcp-socket", func(rep uint32) {
		// Destructor - close socket
	})

	// Connection establishment methods
	inst.FuncNoType("[method]tcp-socket.start-bind", tcpSocketStartBind)
	inst.FuncNoType("[method]tcp-socket.finish-bind", tcpSocketFinishBind)
	inst.FuncNoType("[method]tcp-socket.start-connect", tcpSocketStartConnect)
	inst.FuncNoType("[method]tcp-socket.finish-connect", tcpSocketFinishConnect)
	inst.FuncNoType("[method]tcp-socket.start-listen", tcpSocketStartListen)
	inst.FuncNoType("[method]tcp-socket.finish-listen", tcpSocketFinishListen)
	inst.FuncNoType("[method]tcp-socket.accept", tcpSocketAccept)

	// Address methods
	inst.FuncNoType("[method]tcp-socket.local-address", tcpSocketLocalAddress)
	inst.FuncNoType("[method]tcp-socket.remote-address", tcpSocketRemoteAddress)
	inst.FuncNoType("[method]tcp-socket.is-listening", tcpSocketIsListening)
	inst.FuncNoType("[method]tcp-socket.address-family", tcpSocketAddressFamily)

	// Socket option methods
	inst.FuncNoType("[method]tcp-socket.set-listen-backlog-size", tcpSocketSetListenBacklogSize)
	inst.FuncNoType("[method]tcp-socket.keep-alive-enabled", tcpSocketKeepAliveEnabled)
	inst.FuncNoType("[method]tcp-socket.set-keep-alive-enabled", tcpSocketSetKeepAliveEnabled)
	inst.FuncNoType("[method]tcp-socket.keep-alive-idle-time", tcpSocketKeepAliveIdleTime)
	inst.FuncNoType("[method]tcp-socket.set-keep-alive-idle-time", tcpSocketSetKeepAliveIdleTime)
	inst.FuncNoType("[method]tcp-socket.keep-alive-interval", tcpSocketKeepAliveInterval)
	inst.FuncNoType("[method]tcp-socket.set-keep-alive-interval", tcpSocketSetKeepAliveInterval)
	inst.FuncNoType("[method]tcp-socket.keep-alive-count", tcpSocketKeepAliveCount)
	inst.FuncNoType("[method]tcp-socket.set-keep-alive-count", tcpSocketSetKeepAliveCount)
	inst.FuncNoType("[method]tcp-socket.hop-limit", tcpSocketHopLimit)
	inst.FuncNoType("[method]tcp-socket.set-hop-limit", tcpSocketSetHopLimit)
	inst.FuncNoType("[method]tcp-socket.receive-buffer-size", tcpSocketReceiveBufferSize)
	inst.FuncNoType("[method]tcp-socket.set-receive-buffer-size", tcpSocketSetReceiveBufferSize)
	inst.FuncNoType("[method]tcp-socket.send-buffer-size", tcpSocketSendBufferSize)
	inst.FuncNoType("[method]tcp-socket.set-send-buffer-size", tcpSocketSetSendBufferSize)

	// Async and lifecycle methods
	inst.FuncNoType("[method]tcp-socket.subscribe", tcpSocketSubscribe)
	inst.FuncNoType("[method]tcp-socket.shutdown", tcpSocketShutdown)

	return inst.Build()
}

// instantiateTcpCreateSocket registers wasi:sockets/tcp-create-socket@0.2.0
func instantiateTcpCreateSocket(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:sockets/tcp-create-socket@0.2.0")

	// create-tcp-socket: func(network: borrow<network>, address-family: ip-address-family) -> result<own<tcp-socket>, error-code>
	inst.FuncNoType("create-tcp-socket", createTcpSocket)

	return inst.Build()
}

// createTcpSocket creates a new TCP socket.
// Signature: func(network: borrow<network>, address-family: ip-address-family) -> result<own<tcp-socket>, error-code>
func createTcpSocket(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return placeholder socket handle
	handle := component.ValOwn(0)
	return []component.Val{component.ValResultOk(&handle)}, nil
}

// tcpSocketStartBind begins the bind operation.
// Signature: func(self: borrow<tcp-socket>, network: borrow<network>, local-address: ip-socket-address) -> result<_, error-code>
func tcpSocketStartBind(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}

// tcpSocketFinishBind completes the bind operation.
// Signature: func(self: borrow<tcp-socket>) -> result<_, error-code>
func tcpSocketFinishBind(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}

// tcpSocketStartConnect begins the connect operation.
// Signature: func(self: borrow<tcp-socket>, network: borrow<network>, remote-address: ip-socket-address) -> result<_, error-code>
func tcpSocketStartConnect(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}

// tcpSocketFinishConnect completes the connect operation.
// Signature: func(self: borrow<tcp-socket>) -> result<tuple<own<input-stream>, own<output-stream>>, error-code>
func tcpSocketFinishConnect(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return placeholder streams
	inputStream := component.ValOwn(0)
	outputStream := component.ValOwn(1)
	tuple := component.ValTuple([]component.Val{inputStream, outputStream})
	return []component.Val{component.ValResultOk(&tuple)}, nil
}

// tcpSocketStartListen begins the listen operation.
// Signature: func(self: borrow<tcp-socket>) -> result<_, error-code>
func tcpSocketStartListen(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}

// tcpSocketFinishListen completes the listen operation.
// Signature: func(self: borrow<tcp-socket>) -> result<_, error-code>
func tcpSocketFinishListen(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}

// tcpSocketAccept accepts a new connection.
// Signature: func(self: borrow<tcp-socket>) -> result<tuple<own<tcp-socket>, own<input-stream>, own<output-stream>>, error-code>
func tcpSocketAccept(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return placeholder - accepted socket and streams
	socket := component.ValOwn(0)
	inputStream := component.ValOwn(1)
	outputStream := component.ValOwn(2)
	tuple := component.ValTuple([]component.Val{socket, inputStream, outputStream})
	return []component.Val{component.ValResultOk(&tuple)}, nil
}

// tcpSocketLocalAddress returns the local address.
// Signature: func(self: borrow<tcp-socket>) -> result<ip-socket-address, error-code>
func tcpSocketLocalAddress(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return placeholder IPv4 address 0.0.0.0:0
	addrRecord := component.ValRecord(map[string]component.Val{
		"port":    component.ValU16(0),
		"address": component.ValTuple([]component.Val{component.ValU8(0), component.ValU8(0), component.ValU8(0), component.ValU8(0)}),
	})
	addr := component.ValVariant("ipv4", &addrRecord)
	return []component.Val{component.ValResultOk(&addr)}, nil
}

// tcpSocketRemoteAddress returns the remote address.
// Signature: func(self: borrow<tcp-socket>) -> result<ip-socket-address, error-code>
func tcpSocketRemoteAddress(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return placeholder IPv4 address 0.0.0.0:0
	addrRecord := component.ValRecord(map[string]component.Val{
		"port":    component.ValU16(0),
		"address": component.ValTuple([]component.Val{component.ValU8(0), component.ValU8(0), component.ValU8(0), component.ValU8(0)}),
	})
	addr := component.ValVariant("ipv4", &addrRecord)
	return []component.Val{component.ValResultOk(&addr)}, nil
}

// tcpSocketIsListening returns whether the socket is listening.
// Signature: func(self: borrow<tcp-socket>) -> bool
func tcpSocketIsListening(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return false as placeholder
	return []component.Val{component.ValBool(false)}, nil
}

// tcpSocketAddressFamily returns the address family.
// Signature: func(self: borrow<tcp-socket>) -> ip-address-family
func tcpSocketAddressFamily(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return IPv4 as placeholder
	return []component.Val{component.ValEnum("ipv4")}, nil
}

// tcpSocketSetListenBacklogSize sets the listen backlog size.
// Signature: func(self: borrow<tcp-socket>, value: u64) -> result<_, error-code>
func tcpSocketSetListenBacklogSize(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}

// tcpSocketKeepAliveEnabled returns whether keep-alive is enabled.
// Signature: func(self: borrow<tcp-socket>) -> result<bool, error-code>
func tcpSocketKeepAliveEnabled(ctx context.Context, args []component.Val) ([]component.Val, error) {
	enabled := component.ValBool(false)
	return []component.Val{component.ValResultOk(&enabled)}, nil
}

// tcpSocketSetKeepAliveEnabled sets whether keep-alive is enabled.
// Signature: func(self: borrow<tcp-socket>, value: bool) -> result<_, error-code>
func tcpSocketSetKeepAliveEnabled(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}

// tcpSocketKeepAliveIdleTime returns the keep-alive idle time.
// Signature: func(self: borrow<tcp-socket>) -> result<duration, error-code>
func tcpSocketKeepAliveIdleTime(ctx context.Context, args []component.Val) ([]component.Val, error) {
	duration := component.ValU64(7200000000000) // 2 hours in nanoseconds
	return []component.Val{component.ValResultOk(&duration)}, nil
}

// tcpSocketSetKeepAliveIdleTime sets the keep-alive idle time.
// Signature: func(self: borrow<tcp-socket>, value: duration) -> result<_, error-code>
func tcpSocketSetKeepAliveIdleTime(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}

// tcpSocketKeepAliveInterval returns the keep-alive interval.
// Signature: func(self: borrow<tcp-socket>) -> result<duration, error-code>
func tcpSocketKeepAliveInterval(ctx context.Context, args []component.Val) ([]component.Val, error) {
	duration := component.ValU64(75000000000) // 75 seconds in nanoseconds
	return []component.Val{component.ValResultOk(&duration)}, nil
}

// tcpSocketSetKeepAliveInterval sets the keep-alive interval.
// Signature: func(self: borrow<tcp-socket>, value: duration) -> result<_, error-code>
func tcpSocketSetKeepAliveInterval(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}

// tcpSocketKeepAliveCount returns the keep-alive count.
// Signature: func(self: borrow<tcp-socket>) -> result<u32, error-code>
func tcpSocketKeepAliveCount(ctx context.Context, args []component.Val) ([]component.Val, error) {
	count := component.ValU32(9)
	return []component.Val{component.ValResultOk(&count)}, nil
}

// tcpSocketSetKeepAliveCount sets the keep-alive count.
// Signature: func(self: borrow<tcp-socket>, value: u32) -> result<_, error-code>
func tcpSocketSetKeepAliveCount(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}

// tcpSocketHopLimit returns the hop limit (TTL).
// Signature: func(self: borrow<tcp-socket>) -> result<u8, error-code>
func tcpSocketHopLimit(ctx context.Context, args []component.Val) ([]component.Val, error) {
	limit := component.ValU8(64)
	return []component.Val{component.ValResultOk(&limit)}, nil
}

// tcpSocketSetHopLimit sets the hop limit (TTL).
// Signature: func(self: borrow<tcp-socket>, value: u8) -> result<_, error-code>
func tcpSocketSetHopLimit(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}

// tcpSocketReceiveBufferSize returns the receive buffer size.
// Signature: func(self: borrow<tcp-socket>) -> result<u64, error-code>
func tcpSocketReceiveBufferSize(ctx context.Context, args []component.Val) ([]component.Val, error) {
	size := component.ValU64(65536)
	return []component.Val{component.ValResultOk(&size)}, nil
}

// tcpSocketSetReceiveBufferSize sets the receive buffer size.
// Signature: func(self: borrow<tcp-socket>, value: u64) -> result<_, error-code>
func tcpSocketSetReceiveBufferSize(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}

// tcpSocketSendBufferSize returns the send buffer size.
// Signature: func(self: borrow<tcp-socket>) -> result<u64, error-code>
func tcpSocketSendBufferSize(ctx context.Context, args []component.Val) ([]component.Val, error) {
	size := component.ValU64(65536)
	return []component.Val{component.ValResultOk(&size)}, nil
}

// tcpSocketSetSendBufferSize sets the send buffer size.
// Signature: func(self: borrow<tcp-socket>, value: u64) -> result<_, error-code>
func tcpSocketSetSendBufferSize(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}

// tcpSocketSubscribe returns a pollable for the socket.
// Signature: func(self: borrow<tcp-socket>) -> own<pollable>
func tcpSocketSubscribe(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return placeholder pollable handle
	return []component.Val{component.ValOwn(0)}, nil
}

// tcpSocketShutdown shuts down the socket.
// Signature: func(self: borrow<tcp-socket>, shutdown-type: shutdown-type) -> result<_, error-code>
func tcpSocketShutdown(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}
