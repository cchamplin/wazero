// imports/wasip2/sockets/udp.go

package sockets

import (
	"context"

	"github.com/tetratelabs/wazero/internal/component"
)

// instantiateUdp registers wasi:sockets/udp@0.2.0
func instantiateUdp(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:sockets/udp@0.2.0")

	// udp-socket resource
	inst.Resource("udp-socket", func(rep uint32) {
		// Destructor - close socket
	})

	// incoming-datagram-stream resource
	inst.Resource("incoming-datagram-stream", func(rep uint32) {
		// Destructor - clean up stream
	})

	// outgoing-datagram-stream resource
	inst.Resource("outgoing-datagram-stream", func(rep uint32) {
		// Destructor - clean up stream
	})

	// UDP socket methods
	inst.FuncNoType("[method]udp-socket.start-bind", udpSocketStartBind)
	inst.FuncNoType("[method]udp-socket.finish-bind", udpSocketFinishBind)
	inst.FuncNoType("[method]udp-socket.stream", udpSocketStream)
	inst.FuncNoType("[method]udp-socket.local-address", udpSocketLocalAddress)
	inst.FuncNoType("[method]udp-socket.remote-address", udpSocketRemoteAddress)
	inst.FuncNoType("[method]udp-socket.address-family", udpSocketAddressFamily)
	inst.FuncNoType("[method]udp-socket.unicast-hop-limit", udpSocketUnicastHopLimit)
	inst.FuncNoType("[method]udp-socket.set-unicast-hop-limit", udpSocketSetUnicastHopLimit)
	inst.FuncNoType("[method]udp-socket.receive-buffer-size", udpSocketReceiveBufferSize)
	inst.FuncNoType("[method]udp-socket.set-receive-buffer-size", udpSocketSetReceiveBufferSize)
	inst.FuncNoType("[method]udp-socket.send-buffer-size", udpSocketSendBufferSize)
	inst.FuncNoType("[method]udp-socket.set-send-buffer-size", udpSocketSetSendBufferSize)
	inst.FuncNoType("[method]udp-socket.subscribe", udpSocketSubscribe)

	// Incoming datagram stream methods
	inst.FuncNoType("[method]incoming-datagram-stream.receive", incomingDatagramStreamReceive)
	inst.FuncNoType("[method]incoming-datagram-stream.subscribe", incomingDatagramStreamSubscribe)

	// Outgoing datagram stream methods
	inst.FuncNoType("[method]outgoing-datagram-stream.check-send", outgoingDatagramStreamCheckSend)
	inst.FuncNoType("[method]outgoing-datagram-stream.send", outgoingDatagramStreamSend)
	inst.FuncNoType("[method]outgoing-datagram-stream.subscribe", outgoingDatagramStreamSubscribe)

	return inst.Build()
}

// instantiateUdpCreateSocket registers wasi:sockets/udp-create-socket@0.2.0
func instantiateUdpCreateSocket(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:sockets/udp-create-socket@0.2.0")

	// create-udp-socket: func(network: borrow<network>, address-family: ip-address-family) -> result<own<udp-socket>, error-code>
	inst.FuncNoType("create-udp-socket", createUdpSocket)

	return inst.Build()
}

// createUdpSocket creates a new UDP socket.
// Signature: func(network: borrow<network>, address-family: ip-address-family) -> result<own<udp-socket>, error-code>
func createUdpSocket(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return placeholder socket handle
	handle := component.ValOwn(0)
	return []component.Val{component.ValResultOk(&handle)}, nil
}

// udpSocketStartBind begins the bind operation.
// Signature: func(self: borrow<udp-socket>, network: borrow<network>, local-address: ip-socket-address) -> result<_, error-code>
func udpSocketStartBind(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}

// udpSocketFinishBind completes the bind operation.
// Signature: func(self: borrow<udp-socket>) -> result<_, error-code>
func udpSocketFinishBind(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}

// udpSocketStream returns datagram streams for the socket.
// Signature: func(self: borrow<udp-socket>, remote-address: option<ip-socket-address>) -> result<tuple<own<incoming-datagram-stream>, own<outgoing-datagram-stream>>, error-code>
func udpSocketStream(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return placeholder streams
	incomingStream := component.ValOwn(0)
	outgoingStream := component.ValOwn(1)
	tuple := component.ValTuple([]component.Val{incomingStream, outgoingStream})
	return []component.Val{component.ValResultOk(&tuple)}, nil
}

// udpSocketLocalAddress returns the local address.
// Signature: func(self: borrow<udp-socket>) -> result<ip-socket-address, error-code>
func udpSocketLocalAddress(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return placeholder IPv4 address 0.0.0.0:0
	addrRecord := component.ValRecord(map[string]component.Val{
		"port":    component.ValU16(0),
		"address": component.ValTuple([]component.Val{component.ValU8(0), component.ValU8(0), component.ValU8(0), component.ValU8(0)}),
	})
	addr := component.ValVariant("ipv4", &addrRecord)
	return []component.Val{component.ValResultOk(&addr)}, nil
}

// udpSocketRemoteAddress returns the remote address.
// Signature: func(self: borrow<udp-socket>) -> result<ip-socket-address, error-code>
func udpSocketRemoteAddress(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return placeholder IPv4 address 0.0.0.0:0
	addrRecord := component.ValRecord(map[string]component.Val{
		"port":    component.ValU16(0),
		"address": component.ValTuple([]component.Val{component.ValU8(0), component.ValU8(0), component.ValU8(0), component.ValU8(0)}),
	})
	addr := component.ValVariant("ipv4", &addrRecord)
	return []component.Val{component.ValResultOk(&addr)}, nil
}

// udpSocketAddressFamily returns the address family.
// Signature: func(self: borrow<udp-socket>) -> ip-address-family
func udpSocketAddressFamily(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return IPv4 as placeholder
	return []component.Val{component.ValEnum("ipv4")}, nil
}

// udpSocketUnicastHopLimit returns the unicast hop limit.
// Signature: func(self: borrow<udp-socket>) -> result<u8, error-code>
func udpSocketUnicastHopLimit(ctx context.Context, args []component.Val) ([]component.Val, error) {
	limit := component.ValU8(64)
	return []component.Val{component.ValResultOk(&limit)}, nil
}

// udpSocketSetUnicastHopLimit sets the unicast hop limit.
// Signature: func(self: borrow<udp-socket>, value: u8) -> result<_, error-code>
func udpSocketSetUnicastHopLimit(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}

// udpSocketReceiveBufferSize returns the receive buffer size.
// Signature: func(self: borrow<udp-socket>) -> result<u64, error-code>
func udpSocketReceiveBufferSize(ctx context.Context, args []component.Val) ([]component.Val, error) {
	size := component.ValU64(65536)
	return []component.Val{component.ValResultOk(&size)}, nil
}

// udpSocketSetReceiveBufferSize sets the receive buffer size.
// Signature: func(self: borrow<udp-socket>, value: u64) -> result<_, error-code>
func udpSocketSetReceiveBufferSize(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}

// udpSocketSendBufferSize returns the send buffer size.
// Signature: func(self: borrow<udp-socket>) -> result<u64, error-code>
func udpSocketSendBufferSize(ctx context.Context, args []component.Val) ([]component.Val, error) {
	size := component.ValU64(65536)
	return []component.Val{component.ValResultOk(&size)}, nil
}

// udpSocketSetSendBufferSize sets the send buffer size.
// Signature: func(self: borrow<udp-socket>, value: u64) -> result<_, error-code>
func udpSocketSetSendBufferSize(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}

// udpSocketSubscribe returns a pollable for the socket.
// Signature: func(self: borrow<udp-socket>) -> own<pollable>
func udpSocketSubscribe(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return placeholder pollable handle
	return []component.Val{component.ValOwn(0)}, nil
}

// incomingDatagramStreamReceive receives datagrams from the stream.
// Signature: func(self: borrow<incoming-datagram-stream>, max-results: u64) -> result<list<incoming-datagram>, error-code>
func incomingDatagramStreamReceive(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return empty list as placeholder (no datagrams)
	emptyList := component.ValList([]component.Val{})
	return []component.Val{component.ValResultOk(&emptyList)}, nil
}

// incomingDatagramStreamSubscribe returns a pollable for the stream.
// Signature: func(self: borrow<incoming-datagram-stream>) -> own<pollable>
func incomingDatagramStreamSubscribe(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return placeholder pollable handle
	return []component.Val{component.ValOwn(0)}, nil
}

// outgoingDatagramStreamCheckSend checks how many datagrams can be sent.
// Signature: func(self: borrow<outgoing-datagram-stream>) -> result<u64, error-code>
func outgoingDatagramStreamCheckSend(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return maximum send capacity
	capacity := component.ValU64(1024)
	return []component.Val{component.ValResultOk(&capacity)}, nil
}

// outgoingDatagramStreamSend sends datagrams on the stream.
// Signature: func(self: borrow<outgoing-datagram-stream>, datagrams: list<outgoing-datagram>) -> result<u64, error-code>
func outgoingDatagramStreamSend(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return the number of datagrams sent (all of them)
	var sent uint64 = 0
	if len(args) > 1 && args[1].Kind() == component.ValKindList {
		sent = uint64(len(args[1].List()))
	}
	sentVal := component.ValU64(sent)
	return []component.Val{component.ValResultOk(&sentVal)}, nil
}

// outgoingDatagramStreamSubscribe returns a pollable for the stream.
// Signature: func(self: borrow<outgoing-datagram-stream>) -> own<pollable>
func outgoingDatagramStreamSubscribe(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return placeholder pollable handle
	return []component.Val{component.ValOwn(0)}, nil
}
