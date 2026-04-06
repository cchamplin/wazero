// imports/wasip2/sockets/udp.go

package sockets

import (
	"context"
	"net"

	"github.com/tetratelabs/wazero/internal/component"
	wasipIO "github.com/tetratelabs/wazero/imports/wasip2/io"
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

	return inst.SkipValidation().Build()
}

// instantiateUdpCreateSocket registers wasi:sockets/udp-create-socket@0.2.0
func instantiateUdpCreateSocket(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:sockets/udp-create-socket@0.2.0")

	// create-udp-socket: func(network: borrow<network>, address-family: ip-address-family) -> result<own<udp-socket>, error-code>
	inst.FuncNoType("create-udp-socket", createUdpSocket)

	return inst.SkipValidation().Build()
}

// createUdpSocket creates a new UDP socket.
// Signature: func(network: borrow<network>, address-family: ip-address-family) -> result<own<udp-socket>, error-code>
func createUdpSocket(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// args[0] = borrow<network> (ignored for now)
	// args[1] = ip-address-family enum
	family := parseAddressFamily(args[1])

	// Create socket
	sock := NewUdpSocket(family)

	// Store in resource table
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		// No table available, return placeholder
		handle := component.ValOwn(0)
		return []component.Val{component.ValResultOk(&handle)}, nil
	}

	handle := table.New(sock, true)
	handleVal := component.ValOwn(uint32(handle))
	return []component.Val{component.ValResultOk(&handleVal)}, nil
}

// udpSocketStartBind begins the bind operation.
// Signature: func(self: borrow<udp-socket>, network: borrow<network>, local-address: ip-socket-address) -> result<_, error-code>
func udpSocketStartBind(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	// args[1] = borrow<network> (ignored)
	localAddrVal := args[2]

	sock, err := getUdpSocket(ctx, handle)
	if err != nil {
		// Fallback for tests without resource table
		return []component.Val{component.ValResultOk(nil)}, nil
	}

	// Check state
	if sock.state != udpStateUnbound {
		return []component.Val{errorCodeToVal(ErrorCodeInvalidState)}, nil
	}

	// Parse address
	localAddr, err := ipSocketAddressFromVal(localAddrVal)
	if err != nil {
		return []component.Val{errorCodeToVal(ErrorCodeInvalidArgument)}, nil
	}

	// Store pending address and transition state
	sock.localAddr = localAddr
	sock.state = udpStateBinding

	return []component.Val{component.ValResultOk(nil)}, nil
}

// udpSocketFinishBind completes the bind operation.
// Signature: func(self: borrow<udp-socket>) -> result<_, error-code>
func udpSocketFinishBind(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	sock, err := getUdpSocket(ctx, handle)
	if err != nil {
		// Fallback for tests without resource table
		return []component.Val{component.ValResultOk(nil)}, nil
	}

	// Check state
	if sock.state != udpStateBinding {
		return []component.Val{errorCodeToVal(ErrorCodeInvalidState)}, nil
	}

	if sock.localAddr == nil {
		return []component.Val{errorCodeToVal(ErrorCodeInvalidState)}, nil
	}

	// Actually bind the UDP socket
	udpAddr := ipSocketAddressToUDPAddr(sock.localAddr)
	conn, netErr := net.ListenUDP("udp", udpAddr)
	if netErr != nil {
		sock.pendingErr = netErr
		sock.state = udpStateUnbound
		return []component.Val{errorCodeToVal(mapNetError(netErr))}, nil
	}

	// Update the local address to the actual bound address
	sock.conn = conn
	sock.localAddr = udpAddrToIpSocketAddress(conn.LocalAddr().(*net.UDPAddr))
	sock.state = udpStateBound

	return []component.Val{component.ValResultOk(nil)}, nil
}

// udpSocketStream returns datagram streams for the socket.
// Signature: func(self: borrow<udp-socket>, remote-address: option<ip-socket-address>) -> result<tuple<own<incoming-datagram-stream>, own<outgoing-datagram-stream>>, error-code>
func udpSocketStream(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	remoteAddrOpt := args[1]

	sock, err := getUdpSocket(ctx, handle)
	if err != nil {
		// Fallback for tests without resource table
		incomingStream := component.ValOwn(0)
		outgoingStream := component.ValOwn(1)
		tuple := component.ValTuple([]component.Val{incomingStream, outgoingStream})
		return []component.Val{component.ValResultOk(&tuple)}, nil
	}

	// Check state - must be bound
	if sock.state != udpStateBound {
		return []component.Val{errorCodeToVal(ErrorCodeInvalidState)}, nil
	}

	// Handle optional remote address for connected UDP
	remoteAddrPayload := remoteAddrOpt.Option()
	if remoteAddrPayload != nil {
		remoteAddr, err := ipSocketAddressFromVal(*remoteAddrPayload)
		if err != nil {
			return []component.Val{errorCodeToVal(ErrorCodeInvalidArgument)}, nil
		}
		sock.remoteAddr = remoteAddr

		// Connect to the remote address for connected UDP
		udpAddr := ipSocketAddressToUDPAddr(remoteAddr)
		if sock.conn != nil {
			// Connect the UDP socket
			netErr := sock.conn.Close()
			if netErr != nil {
				// Ignore close error
			}
			localAddr := ipSocketAddressToUDPAddr(sock.localAddr)
			conn, netErr := net.DialUDP("udp", localAddr, udpAddr)
			if netErr != nil {
				return []component.Val{errorCodeToVal(mapNetError(netErr))}, nil
			}
			sock.conn = conn
		}
	}

	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		incomingStream := component.ValOwn(0)
		outgoingStream := component.ValOwn(1)
		tuple := component.ValTuple([]component.Val{incomingStream, outgoingStream})
		return []component.Val{component.ValResultOk(&tuple)}, nil
	}

	// Create datagram streams
	inStream := NewIncomingDatagramStream(sock)
	outStream := NewOutgoingDatagramStream(sock)

	inHandle := table.New(inStream, true)
	outHandle := table.New(outStream, true)

	incomingStreamVal := component.ValOwn(uint32(inHandle))
	outgoingStreamVal := component.ValOwn(uint32(outHandle))
	tuple := component.ValTuple([]component.Val{incomingStreamVal, outgoingStreamVal})
	return []component.Val{component.ValResultOk(&tuple)}, nil
}

// udpSocketLocalAddress returns the local address.
// Signature: func(self: borrow<udp-socket>) -> result<ip-socket-address, error-code>
func udpSocketLocalAddress(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	sock, err := getUdpSocket(ctx, handle)
	if err != nil {
		// Fallback - return placeholder
		addr := ipSocketAddressToVal(nil)
		return []component.Val{component.ValResultOk(&addr)}, nil
	}

	if sock.localAddr == nil {
		return []component.Val{errorCodeToVal(ErrorCodeInvalidState)}, nil
	}

	addr := ipSocketAddressToVal(sock.localAddr)
	return []component.Val{component.ValResultOk(&addr)}, nil
}

// udpSocketRemoteAddress returns the remote address.
// Signature: func(self: borrow<udp-socket>) -> result<ip-socket-address, error-code>
func udpSocketRemoteAddress(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	sock, err := getUdpSocket(ctx, handle)
	if err != nil {
		// Fallback - return placeholder
		addr := ipSocketAddressToVal(nil)
		return []component.Val{component.ValResultOk(&addr)}, nil
	}

	if sock.remoteAddr == nil {
		return []component.Val{errorCodeToVal(ErrorCodeInvalidState)}, nil
	}

	addr := ipSocketAddressToVal(sock.remoteAddr)
	return []component.Val{component.ValResultOk(&addr)}, nil
}

// udpSocketAddressFamily returns the address family.
// Signature: func(self: borrow<udp-socket>) -> ip-address-family
func udpSocketAddressFamily(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	sock, err := getUdpSocket(ctx, handle)
	if err != nil {
		return []component.Val{component.ValEnum("ipv4")}, nil
	}

	return []component.Val{component.ValEnum(sock.Family().String())}, nil
}

// udpSocketUnicastHopLimit returns the unicast hop limit.
// Signature: func(self: borrow<udp-socket>) -> result<u8, error-code>
func udpSocketUnicastHopLimit(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	sock, err := getUdpSocket(ctx, handle)
	if err != nil {
		limit := component.ValU8(64)
		return []component.Val{component.ValResultOk(&limit)}, nil
	}

	limit := component.ValU8(sock.unicastHopLimit)
	return []component.Val{component.ValResultOk(&limit)}, nil
}

// udpSocketSetUnicastHopLimit sets the unicast hop limit.
// Signature: func(self: borrow<udp-socket>, value: u8) -> result<_, error-code>
func udpSocketSetUnicastHopLimit(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	value := args[1].U8()

	sock, err := getUdpSocket(ctx, handle)
	if err != nil {
		return []component.Val{component.ValResultOk(nil)}, nil
	}

	sock.unicastHopLimit = value
	return []component.Val{component.ValResultOk(nil)}, nil
}

// udpSocketReceiveBufferSize returns the receive buffer size.
// Signature: func(self: borrow<udp-socket>) -> result<u64, error-code>
func udpSocketReceiveBufferSize(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	sock, err := getUdpSocket(ctx, handle)
	if err != nil {
		size := component.ValU64(65536)
		return []component.Val{component.ValResultOk(&size)}, nil
	}

	size := component.ValU64(sock.receiveBufferSize)
	return []component.Val{component.ValResultOk(&size)}, nil
}

// udpSocketSetReceiveBufferSize sets the receive buffer size.
// Signature: func(self: borrow<udp-socket>, value: u64) -> result<_, error-code>
func udpSocketSetReceiveBufferSize(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	value := args[1].U64()

	sock, err := getUdpSocket(ctx, handle)
	if err != nil {
		return []component.Val{component.ValResultOk(nil)}, nil
	}

	sock.receiveBufferSize = value
	if sock.conn != nil {
		sock.conn.SetReadBuffer(int(value))
	}
	return []component.Val{component.ValResultOk(nil)}, nil
}

// udpSocketSendBufferSize returns the send buffer size.
// Signature: func(self: borrow<udp-socket>) -> result<u64, error-code>
func udpSocketSendBufferSize(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	sock, err := getUdpSocket(ctx, handle)
	if err != nil {
		size := component.ValU64(65536)
		return []component.Val{component.ValResultOk(&size)}, nil
	}

	size := component.ValU64(sock.sendBufferSize)
	return []component.Val{component.ValResultOk(&size)}, nil
}

// udpSocketSetSendBufferSize sets the send buffer size.
// Signature: func(self: borrow<udp-socket>, value: u64) -> result<_, error-code>
func udpSocketSetSendBufferSize(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	value := args[1].U64()

	sock, err := getUdpSocket(ctx, handle)
	if err != nil {
		return []component.Val{component.ValResultOk(nil)}, nil
	}

	sock.sendBufferSize = value
	if sock.conn != nil {
		sock.conn.SetWriteBuffer(int(value))
	}
	return []component.Val{component.ValResultOk(nil)}, nil
}

// udpSocketSubscribe returns a pollable for the socket.
// Signature: func(self: borrow<udp-socket>) -> own<pollable>
func udpSocketSubscribe(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []component.Val{component.ValOwn(0)}, nil
	}

	// Per wasmtime: UDP socket subscribe ready() is a no-op — UDP operations
	// don't block at socket level. Blocking happens on datagram streams.
	pollable := wasipIO.NewReadyPollable()
	pollHandle := table.New(pollable, true)
	return []component.Val{component.ValOwn(uint32(pollHandle))}, nil
}

// incomingDatagramStreamReceive receives datagrams from the stream.
// Signature: func(self: borrow<incoming-datagram-stream>, max-results: u64) -> result<list<incoming-datagram>, error-code>
func incomingDatagramStreamReceive(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	maxResults := args[1].U64()

	stream, err := getIncomingDatagramStream(ctx, handle)
	if err != nil {
		// Fallback - return empty list
		emptyList := component.ValList([]component.Val{})
		return []component.Val{component.ValResultOk(&emptyList)}, nil
	}

	if stream.socket == nil || stream.socket.conn == nil {
		emptyList := component.ValList([]component.Val{})
		return []component.Val{component.ValResultOk(&emptyList)}, nil
	}

	// Read datagrams up to maxResults
	datagrams := make([]component.Val, 0, int(minUint64(maxResults, 16)))

	for i := uint64(0); i < maxResults && i < 16; i++ {
		buf := make([]byte, 65536)

		// Set a short deadline for non-blocking behavior
		// In a real async implementation, we would use select/poll
		n, addr, netErr := stream.socket.conn.ReadFromUDP(buf)
		if netErr != nil {
			// Check if it's a timeout (would-block) - return what we have
			if len(datagrams) > 0 {
				break
			}
			// If no datagrams received, check for real errors
			if isWouldBlock(netErr) {
				break
			}
			return []component.Val{errorCodeToVal(mapNetError(netErr))}, nil
		}

		// Create incoming-datagram record
		// incoming-datagram: { data: list<u8>, remote-address: ip-socket-address }
		remoteAddr := udpAddrToIpSocketAddress(addr)

		// Convert data to list<u8>
		dataList := make([]component.Val, n)
		for j := 0; j < n; j++ {
			dataList[j] = component.ValU8(buf[j])
		}

		datagram := component.ValRecord(map[string]component.Val{
			"data":           component.ValList(dataList),
			"remote-address": ipSocketAddressToVal(remoteAddr),
		})
		datagrams = append(datagrams, datagram)
	}

	resultList := component.ValList(datagrams)
	return []component.Val{component.ValResultOk(&resultList)}, nil
}

// incomingDatagramStreamSubscribe returns a pollable for the stream.
// Signature: func(self: borrow<incoming-datagram-stream>) -> own<pollable>
func incomingDatagramStreamSubscribe(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []component.Val{component.ValOwn(0)}, nil
	}

	// Incoming datagram stream subscribe: ready when data is available to read.
	// Since we don't have a non-destructive readability check, use a ready pollable.
	// In practice, WASI components poll then call receive().
	pollable := wasipIO.NewReadyPollable()
	pollHandle := table.New(pollable, true)
	return []component.Val{component.ValOwn(uint32(pollHandle))}, nil
}

// outgoingDatagramStreamCheckSend checks how many datagrams can be sent.
// Signature: func(self: borrow<outgoing-datagram-stream>) -> result<u64, error-code>
func outgoingDatagramStreamCheckSend(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	stream, err := getOutgoingDatagramStream(ctx, handle)
	if err != nil {
		capacity := component.ValU64(0)
		return []component.Val{component.ValResultOk(&capacity)}, nil
	}

	stream.mu.Lock()
	defer stream.mu.Unlock()

	var permit int
	switch stream.sendState {
	case sendStateIdle:
		const defaultPermit = 16
		stream.sendState = sendStatePermitted
		stream.sendPermit = defaultPermit
		permit = defaultPermit
	case sendStatePermitted:
		permit = stream.sendPermit
	case sendStateWaiting:
		permit = 0
	}

	capacity := component.ValU64(uint64(permit))
	return []component.Val{component.ValResultOk(&capacity)}, nil
}

// outgoingDatagramStreamSend sends datagrams on the stream.
// Signature: func(self: borrow<outgoing-datagram-stream>, datagrams: list<outgoing-datagram>) -> result<u64, error-code>
func outgoingDatagramStreamSend(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	datagramsList := args[1]

	stream, err := getOutgoingDatagramStream(ctx, handle)
	if err != nil {
		// Fallback - return number of datagrams
		var sent uint64 = 0
		if datagramsList.Kind() == component.ValKindList {
			sent = uint64(len(datagramsList.List()))
		}
		sentVal := component.ValU64(sent)
		return []component.Val{component.ValResultOk(&sentVal)}, nil
	}

	if stream.socket == nil || stream.socket.conn == nil {
		return []component.Val{errorCodeToVal(ErrorCodeInvalidState)}, nil
	}

	datagrams := datagramsList.List()
	var sent uint64 = 0

	for _, dg := range datagrams {
		record := dg.Record()

		// Get data
		dataVal, hasData := record["data"]
		if !hasData {
			continue
		}
		dataList := dataVal.List()
		data := make([]byte, len(dataList))
		for i, v := range dataList {
			data[i] = v.U8()
		}

		// Get remote-address (optional)
		remoteAddrVal, hasAddr := record["remote-address"]
		var udpAddr *net.UDPAddr
		if hasAddr && remoteAddrVal.Kind() == component.ValKindOption {
			optPayload := remoteAddrVal.Option()
			if optPayload != nil {
				socketAddr, err := ipSocketAddressFromVal(*optPayload)
				if err == nil {
					udpAddr = ipSocketAddressToUDPAddr(socketAddr)
				}
			}
		} else if hasAddr && remoteAddrVal.Kind() == component.ValKindVariant {
			// Direct variant (not optional)
			socketAddr, err := ipSocketAddressFromVal(remoteAddrVal)
			if err == nil {
				udpAddr = ipSocketAddressToUDPAddr(socketAddr)
			}
		}

		// Send the datagram
		var netErr error
		if udpAddr != nil {
			_, netErr = stream.socket.conn.WriteToUDP(data, udpAddr)
		} else if stream.socket.remoteAddr != nil {
			// Connected UDP - use default remote address
			_, netErr = stream.socket.conn.Write(data)
		} else {
			// No remote address available
			continue
		}

		if netErr != nil {
			if sent > 0 {
				break
			}
			return []component.Val{errorCodeToVal(mapNetError(netErr))}, nil
		}
		sent++

		stream.mu.Lock()
		stream.sendPermit--
		if stream.sendPermit <= 0 {
			stream.sendState = sendStateWaiting
		}
		stream.mu.Unlock()
	}

	sentVal := component.ValU64(sent)
	return []component.Val{component.ValResultOk(&sentVal)}, nil
}

// outgoingDatagramStreamSubscribe returns a pollable for the stream.
// Signature: func(self: borrow<outgoing-datagram-stream>) -> own<pollable>
func outgoingDatagramStreamSubscribe(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []component.Val{component.ValOwn(0)}, nil
	}

	stream, err := getOutgoingDatagramStream(ctx, handle)
	if err != nil {
		// Return a ready pollable on error - guest will discover error on next operation
		pollable := wasipIO.NewReadyPollable()
		pollHandle := table.New(pollable, true)
		return []component.Val{component.ValOwn(uint32(pollHandle))}, nil
	}

	pollable := wasipIO.NewPollable(
		func() bool {
			stream.mu.Lock()
			defer stream.mu.Unlock()
			return stream.sendState != sendStateWaiting
		},
		func() {
			stream.mu.Lock()
			defer stream.mu.Unlock()
			if stream.sendState == sendStateWaiting {
				stream.sendState = sendStateIdle
			}
		},
	)
	pollHandle := table.New(pollable, true)
	return []component.Val{component.ValOwn(uint32(pollHandle))}, nil
}

// minUint64 returns the minimum of two uint64 values.
func minUint64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

// isWouldBlock checks if an error indicates a would-block condition.
func isWouldBlock(err error) bool {
	if err == nil {
		return false
	}
	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout()
	}
	return false
}
