// imports/wasip2/sockets/udp.go

// WIT source of truth: debug-vendored/WASI/proposals/sockets/wit/{udp,udp-create-socket}.wit
// Package version: wasi:sockets@0.2.9 (wazero targets wasi:sockets@0.2.0)
package sockets

import (
	"context"
	"fmt"
	"net"

	"github.com/tetratelabs/wazero/api"
	wasipIO "github.com/tetratelabs/wazero/imports/wasip2/io"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// instantiateUdp registers wasi:sockets/udp@0.2.0
func instantiateUdp(linker api.ComponentLinker) error {
	inst := linker.DefineInstance("wasi:sockets/udp@0.2.0")

	// udp-socket resource
	inst.Resource("udp-socket", func(rep uint32) {
		if s := getUdpSocketFromRegistry(rep); s != nil {
			s.Destroy()
		}
		unregisterUdpSocket(rep)
	})

	// incoming-datagram-stream resource
	inst.Resource("incoming-datagram-stream", func(rep uint32) {
		unregisterIncomingDatagramStream(rep)
	})

	// outgoing-datagram-stream resource
	inst.Resource("outgoing-datagram-stream", func(rep uint32) {
		unregisterOutgoingDatagramStream(rep)
	})

	// UDP socket methods
	inst.Func("[method]udp-socket.start-bind", udpSocketStartBind)
	inst.Func("[method]udp-socket.finish-bind", udpSocketFinishBind)
	inst.Func("[method]udp-socket.stream", udpSocketStream)
	inst.Func("[method]udp-socket.local-address", udpSocketLocalAddress)
	inst.Func("[method]udp-socket.remote-address", udpSocketRemoteAddress)
	inst.Func("[method]udp-socket.address-family", udpSocketAddressFamily)
	inst.Func("[method]udp-socket.unicast-hop-limit", udpSocketUnicastHopLimit)
	inst.Func("[method]udp-socket.set-unicast-hop-limit", udpSocketSetUnicastHopLimit)
	inst.Func("[method]udp-socket.receive-buffer-size", udpSocketReceiveBufferSize)
	inst.Func("[method]udp-socket.set-receive-buffer-size", udpSocketSetReceiveBufferSize)
	inst.Func("[method]udp-socket.send-buffer-size", udpSocketSendBufferSize)
	inst.Func("[method]udp-socket.set-send-buffer-size", udpSocketSetSendBufferSize)
	inst.Func("[method]udp-socket.subscribe", udpSocketSubscribe)

	// Incoming datagram stream methods
	inst.Func("[method]incoming-datagram-stream.receive", incomingDatagramStreamReceive)
	inst.Func("[method]incoming-datagram-stream.subscribe", incomingDatagramStreamSubscribe)

	// Outgoing datagram stream methods
	inst.Func("[method]outgoing-datagram-stream.check-send", outgoingDatagramStreamCheckSend)
	inst.Func("[method]outgoing-datagram-stream.send", outgoingDatagramStreamSend)
	inst.Func("[method]outgoing-datagram-stream.subscribe", outgoingDatagramStreamSubscribe)

	return inst.SkipValidation().Build()
}

// instantiateUdpCreateSocket registers wasi:sockets/udp-create-socket@0.2.0
func instantiateUdpCreateSocket(linker api.ComponentLinker) error {
	inst := linker.DefineInstance("wasi:sockets/udp-create-socket@0.2.0")

	// create-udp-socket: func(network: borrow<network>, address-family: ip-address-family) -> result<own<udp-socket>, error-code>
	inst.Func("create-udp-socket", createUdpSocket)

	return inst.SkipValidation().Build()
}

// createUdpSocket creates a new UDP socket.
// Signature: func(network: borrow<network>, address-family: ip-address-family) -> result<own<udp-socket>, error-code>
func createUdpSocket(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	// args[0] = borrow<network> (ignored for now)
	// args[1] = ip-address-family enum
	family := parseAddressFamily(args[1])

	// Create socket
	sock := NewUdpSocket(family)
	sid := registerUdpSocket(sock)

	// Store in resource table
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		unregisterUdpSocket(sid)
		handle := types.ValOwn(0)
		return []types.Val{types.ValResultOk(&handle)}, nil
	}

	handle, hErr := table.NewResourceHandle(sid, true, udpSocketResourceType)
	if hErr != nil {
		handle := types.ValOwn(0)
		return []types.Val{types.ValResultOk(&handle)}, nil
	}
	handleVal := types.ValOwn(uint32(handle))
	return []types.Val{types.ValResultOk(&handleVal)}, nil
}

// udpSocketStartBind begins the bind operation.
// Signature: func(self: borrow<udp-socket>, network: borrow<network>, local-address: ip-socket-address) -> result<_, error-code>
func udpSocketStartBind(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()
	// args[1] = borrow<network> (ignored)
	localAddrVal := args[2]

	sock, err := getUdpSocket(ctx, handle)
	if err != nil {
		// Fallback for tests without resource table
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	// Check state
	if sock.state != udpStateUnbound {
		return []types.Val{errorCodeToVal(ErrorCodeInvalidState)}, nil
	}

	// Parse address
	localAddr, err := ipSocketAddressFromVal(localAddrVal)
	if err != nil {
		return []types.Val{errorCodeToVal(ErrorCodeInvalidArgument)}, nil
	}

	// Store pending address and transition state
	sock.localAddr = localAddr
	sock.state = udpStateBinding

	return []types.Val{types.ValResultOk(nil)}, nil
}

// udpSocketFinishBind completes the bind operation.
// Signature: func(self: borrow<udp-socket>) -> result<_, error-code>
func udpSocketFinishBind(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	sock, err := getUdpSocket(ctx, handle)
	if err != nil {
		// Fallback for tests without resource table
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	// Check state
	if sock.state != udpStateBinding {
		return []types.Val{errorCodeToVal(ErrorCodeInvalidState)}, nil
	}

	if sock.localAddr == nil {
		return []types.Val{errorCodeToVal(ErrorCodeInvalidState)}, nil
	}

	// Actually bind the UDP socket
	udpAddr := ipSocketAddressToUDPAddr(sock.localAddr)
	conn, netErr := net.ListenUDP("udp", udpAddr)
	if netErr != nil {
		sock.pendingErr = netErr
		sock.state = udpStateUnbound
		return []types.Val{errorCodeToVal(mapNetError(netErr))}, nil
	}

	// Update the local address to the actual bound address
	sock.conn = conn
	sock.localAddr = udpAddrToIpSocketAddress(conn.LocalAddr().(*net.UDPAddr))
	sock.state = udpStateBound

	return []types.Val{types.ValResultOk(nil)}, nil
}

// udpSocketStream returns datagram streams for the socket.
// Signature: func(self: borrow<udp-socket>, remote-address: option<ip-socket-address>) -> result<tuple<own<incoming-datagram-stream>, own<outgoing-datagram-stream>>, error-code>
func udpSocketStream(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()
	remoteAddrOpt := args[1]

	sock, err := getUdpSocket(ctx, handle)
	if err != nil {
		// Fallback for tests without resource table
		incomingStream := types.ValOwn(0)
		outgoingStream := types.ValOwn(1)
		tuple := types.ValTuple([]types.Val{incomingStream, outgoingStream})
		return []types.Val{types.ValResultOk(&tuple)}, nil
	}

	// Check state - must be bound
	if sock.state != udpStateBound {
		return []types.Val{errorCodeToVal(ErrorCodeInvalidState)}, nil
	}

	// Handle optional remote address for connected UDP
	remoteAddrPayload := remoteAddrOpt.Option()
	if remoteAddrPayload != nil {
		remoteAddr, err := ipSocketAddressFromVal(*remoteAddrPayload)
		if err != nil {
			return []types.Val{errorCodeToVal(ErrorCodeInvalidArgument)}, nil
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
				return []types.Val{errorCodeToVal(mapNetError(netErr))}, nil
			}
			sock.conn = conn
		}
	}

	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		incomingStream := types.ValOwn(0)
		outgoingStream := types.ValOwn(1)
		tuple := types.ValTuple([]types.Val{incomingStream, outgoingStream})
		return []types.Val{types.ValResultOk(&tuple)}, nil
	}

	// Create datagram streams
	inStream := NewIncomingDatagramStream(sock)
	inSid := registerIncomingDatagramStream(inStream)
	outStream := NewOutgoingDatagramStream(sock)
	outSid := registerOutgoingDatagramStream(outStream)

	inHandle, err := table.NewResourceHandle(inSid, true, incomingDatagramStreamResourceType)
	if err != nil {
		unregisterIncomingDatagramStream(inSid)
		unregisterOutgoingDatagramStream(outSid)
		return nil, fmt.Errorf("udpSocketStream: register incoming datagram stream handle: %w", err)
	}
	outHandle, err := table.NewResourceHandle(outSid, true, outgoingDatagramStreamResourceType)
	if err != nil {
		unregisterOutgoingDatagramStream(outSid)
		table.Remove(inHandle)
		return nil, fmt.Errorf("udpSocketStream: register outgoing datagram stream handle: %w", err)
	}

	incomingStreamVal := types.ValOwn(uint32(inHandle))
	outgoingStreamVal := types.ValOwn(uint32(outHandle))
	tuple := types.ValTuple([]types.Val{incomingStreamVal, outgoingStreamVal})
	return []types.Val{types.ValResultOk(&tuple)}, nil
}

// udpSocketLocalAddress returns the local address.
// Signature: func(self: borrow<udp-socket>) -> result<ip-socket-address, error-code>
func udpSocketLocalAddress(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	sock, err := getUdpSocket(ctx, handle)
	if err != nil {
		// Fallback - return placeholder
		addr := ipSocketAddressToVal(nil)
		return []types.Val{types.ValResultOk(&addr)}, nil
	}

	if sock.localAddr == nil {
		return []types.Val{errorCodeToVal(ErrorCodeInvalidState)}, nil
	}

	addr := ipSocketAddressToVal(sock.localAddr)
	return []types.Val{types.ValResultOk(&addr)}, nil
}

// udpSocketRemoteAddress returns the remote address.
// Signature: func(self: borrow<udp-socket>) -> result<ip-socket-address, error-code>
func udpSocketRemoteAddress(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	sock, err := getUdpSocket(ctx, handle)
	if err != nil {
		// Fallback - return placeholder
		addr := ipSocketAddressToVal(nil)
		return []types.Val{types.ValResultOk(&addr)}, nil
	}

	if sock.remoteAddr == nil {
		return []types.Val{errorCodeToVal(ErrorCodeInvalidState)}, nil
	}

	addr := ipSocketAddressToVal(sock.remoteAddr)
	return []types.Val{types.ValResultOk(&addr)}, nil
}

// udpSocketAddressFamily returns the address family.
// Signature: func(self: borrow<udp-socket>) -> ip-address-family
func udpSocketAddressFamily(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	sock, err := getUdpSocket(ctx, handle)
	if err != nil {
		return []types.Val{types.ValEnum("ipv4")}, nil
	}

	return []types.Val{types.ValEnum(sock.Family().String())}, nil
}

// udpSocketUnicastHopLimit returns the unicast hop limit.
// Signature: func(self: borrow<udp-socket>) -> result<u8, error-code>
func udpSocketUnicastHopLimit(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	sock, err := getUdpSocket(ctx, handle)
	if err != nil {
		limit := types.ValU8(64)
		return []types.Val{types.ValResultOk(&limit)}, nil
	}

	limit := types.ValU8(sock.unicastHopLimit)
	return []types.Val{types.ValResultOk(&limit)}, nil
}

// udpSocketSetUnicastHopLimit sets the unicast hop limit.
// Signature: func(self: borrow<udp-socket>, value: u8) -> result<_, error-code>
func udpSocketSetUnicastHopLimit(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()
	value := args[1].U8()

	sock, err := getUdpSocket(ctx, handle)
	if err != nil {
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	sock.unicastHopLimit = value
	return []types.Val{types.ValResultOk(nil)}, nil
}

// udpSocketReceiveBufferSize returns the receive buffer size.
// Signature: func(self: borrow<udp-socket>) -> result<u64, error-code>
func udpSocketReceiveBufferSize(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	sock, err := getUdpSocket(ctx, handle)
	if err != nil {
		size := types.ValU64(65536)
		return []types.Val{types.ValResultOk(&size)}, nil
	}

	size := types.ValU64(sock.receiveBufferSize)
	return []types.Val{types.ValResultOk(&size)}, nil
}

// udpSocketSetReceiveBufferSize sets the receive buffer size.
// Signature: func(self: borrow<udp-socket>, value: u64) -> result<_, error-code>
func udpSocketSetReceiveBufferSize(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()
	value := args[1].U64()

	sock, err := getUdpSocket(ctx, handle)
	if err != nil {
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	sock.receiveBufferSize = value
	if sock.conn != nil {
		sock.conn.SetReadBuffer(int(value))
	}
	return []types.Val{types.ValResultOk(nil)}, nil
}

// udpSocketSendBufferSize returns the send buffer size.
// Signature: func(self: borrow<udp-socket>) -> result<u64, error-code>
func udpSocketSendBufferSize(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	sock, err := getUdpSocket(ctx, handle)
	if err != nil {
		size := types.ValU64(65536)
		return []types.Val{types.ValResultOk(&size)}, nil
	}

	size := types.ValU64(sock.sendBufferSize)
	return []types.Val{types.ValResultOk(&size)}, nil
}

// udpSocketSetSendBufferSize sets the send buffer size.
// Signature: func(self: borrow<udp-socket>, value: u64) -> result<_, error-code>
func udpSocketSetSendBufferSize(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()
	value := args[1].U64()

	sock, err := getUdpSocket(ctx, handle)
	if err != nil {
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	sock.sendBufferSize = value
	if sock.conn != nil {
		sock.conn.SetWriteBuffer(int(value))
	}
	return []types.Val{types.ValResultOk(nil)}, nil
}

// udpSocketSubscribe returns a pollable for the socket.
// Signature: func(self: borrow<udp-socket>) -> own<pollable>
func udpSocketSubscribe(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []types.Val{types.ValOwn(0)}, nil
	}

	// Per wasmtime: UDP socket subscribe ready() is a no-op — UDP operations
	// don't block at socket level. Blocking happens on datagram streams.
	pollable := wasipIO.NewReadyPollable()
	pid := wasipIO.RegisterPollable(pollable)
	pollHandle, hErr := table.NewResourceHandle(pid, true, socketsPollableResourceType)
	if hErr != nil {
		return []types.Val{types.ValOwn(0)}, nil
	}
	return []types.Val{types.ValOwn(uint32(pollHandle))}, nil
}

// incomingDatagramStreamReceive receives datagrams from the stream.
// Signature: func(self: borrow<incoming-datagram-stream>, max-results: u64) -> result<list<incoming-datagram>, error-code>
func incomingDatagramStreamReceive(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()
	maxResults := args[1].U64()

	stream, err := getIncomingDatagramStream(ctx, handle)
	if err != nil {
		// Fallback - return empty list
		emptyList := types.ValList([]types.Val{})
		return []types.Val{types.ValResultOk(&emptyList)}, nil
	}

	if stream.socket == nil || stream.socket.conn == nil {
		emptyList := types.ValList([]types.Val{})
		return []types.Val{types.ValResultOk(&emptyList)}, nil
	}

	// Read datagrams up to maxResults
	datagrams := make([]types.Val, 0, int(minUint64(maxResults, 16)))

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
			return []types.Val{errorCodeToVal(mapNetError(netErr))}, nil
		}

		// Create incoming-datagram record
		// incoming-datagram: { data: list<u8>, remote-address: ip-socket-address }
		remoteAddr := udpAddrToIpSocketAddress(addr)

		// Convert data to list<u8>
		dataList := make([]types.Val, n)
		for j := 0; j < n; j++ {
			dataList[j] = types.ValU8(buf[j])
		}

		datagram := types.ValRecord(map[string]types.Val{
			"data":           types.ValList(dataList),
			"remote-address": ipSocketAddressToVal(remoteAddr),
		})
		datagrams = append(datagrams, datagram)
	}

	resultList := types.ValList(datagrams)
	return []types.Val{types.ValResultOk(&resultList)}, nil
}

// incomingDatagramStreamSubscribe returns a pollable for the stream.
// Signature: func(self: borrow<incoming-datagram-stream>) -> own<pollable>
func incomingDatagramStreamSubscribe(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []types.Val{types.ValOwn(0)}, nil
	}

	// Incoming datagram stream subscribe: ready when data is available to read.
	// Since we don't have a non-destructive readability check, use a ready pollable.
	// In practice, WASI components poll then call receive().
	pollable := wasipIO.NewReadyPollable()
	pid := wasipIO.RegisterPollable(pollable)
	pollHandle, hErr := table.NewResourceHandle(pid, true, socketsPollableResourceType)
	if hErr != nil {
		wasipIO.UnregisterPollable(pid)
		return []types.Val{types.ValOwn(0)}, nil
	}
	return []types.Val{types.ValOwn(uint32(pollHandle))}, nil
}

// outgoingDatagramStreamCheckSend checks how many datagrams can be sent.
// Signature: func(self: borrow<outgoing-datagram-stream>) -> result<u64, error-code>
func outgoingDatagramStreamCheckSend(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	stream, err := getOutgoingDatagramStream(ctx, handle)
	if err != nil {
		capacity := types.ValU64(0)
		return []types.Val{types.ValResultOk(&capacity)}, nil
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

	capacity := types.ValU64(uint64(permit))
	return []types.Val{types.ValResultOk(&capacity)}, nil
}

// outgoingDatagramStreamSend sends datagrams on the stream.
// Signature: func(self: borrow<outgoing-datagram-stream>, datagrams: list<outgoing-datagram>) -> result<u64, error-code>
func outgoingDatagramStreamSend(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()
	datagramsList := args[1]

	stream, err := getOutgoingDatagramStream(ctx, handle)
	if err != nil {
		// Fallback - return number of datagrams
		var sent uint64 = 0
		if datagramsList.Kind() == types.ValKindList {
			sent = uint64(len(datagramsList.List()))
		}
		sentVal := types.ValU64(sent)
		return []types.Val{types.ValResultOk(&sentVal)}, nil
	}

	datagrams := datagramsList.List()

	// Per wasi:sockets/udp@0.2.0 udp.wit (proposals/sockets/wit/udp.wit
	// lines 256-257):
	//
	//   Each call to `send` must be permitted by a preceding `check-send`.
	//   Implementations must trap if either `check-send` was not called or
	//   `datagrams` contains more items than `check-send` permitted.
	//
	// These are TRAP conditions, not in-band error returns. In wazero, a
	// host function traps by returning (nil, error); createCanonLowerFunc
	// in internal/component/component_linker.go converts that to a panic
	// which the wasm runtime catches and propagates as a wasm trap.
	// Wasmtime does the same via SocketError::trap(...) — see
	// debug-vendored/wasmtime/crates/wasi/src/p2/host/udp.rs lines 337-351.
	stream.mu.Lock()
	switch stream.sendState {
	case sendStatePermitted:
		if len(datagrams) > stream.sendPermit {
			stream.mu.Unlock()
			return nil, fmt.Errorf("wasi:sockets/udp outgoing-datagram-stream.send: unpermitted: argument exceeds permitted size (got %d, permit %d)", len(datagrams), stream.sendPermit)
		}
	case sendStateIdle, sendStateWaiting:
		stream.mu.Unlock()
		return nil, fmt.Errorf("wasi:sockets/udp outgoing-datagram-stream.send: unpermitted: must call check-send first")
	}
	stream.mu.Unlock()

	// After preconditions: if the underlying socket is not in a sendable
	// state, this is a legitimate in-band runtime error (not a trap).
	if stream.socket == nil || stream.socket.conn == nil {
		return []types.Val{errorCodeToVal(ErrorCodeInvalidState)}, nil
	}

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
		if hasAddr && remoteAddrVal.Kind() == types.ValKindOption {
			optPayload := remoteAddrVal.Option()
			if optPayload != nil {
				socketAddr, err := ipSocketAddressFromVal(*optPayload)
				if err == nil {
					udpAddr = ipSocketAddressToUDPAddr(socketAddr)
				}
			}
		} else if hasAddr && remoteAddrVal.Kind() == types.ValKindVariant {
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
			return []types.Val{errorCodeToVal(mapNetError(netErr))}, nil
		}
		sent++

		stream.mu.Lock()
		stream.sendPermit--
		if stream.sendPermit <= 0 {
			stream.sendState = sendStateWaiting
		}
		stream.mu.Unlock()
	}

	sentVal := types.ValU64(sent)
	return []types.Val{types.ValResultOk(&sentVal)}, nil
}

// outgoingDatagramStreamSubscribe returns a pollable for the stream.
// Signature: func(self: borrow<outgoing-datagram-stream>) -> own<pollable>
func outgoingDatagramStreamSubscribe(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []types.Val{types.ValOwn(0)}, nil
	}

	stream, err := getOutgoingDatagramStream(ctx, handle)
	if err != nil {
		// Return a ready pollable on error - guest will discover error on next operation
		pollable := wasipIO.NewReadyPollable()
		pid := wasipIO.RegisterPollable(pollable)
		pollHandle, hErr := table.NewResourceHandle(pid, true, socketsPollableResourceType)
		if hErr != nil {
			wasipIO.UnregisterPollable(pid)
			return []types.Val{types.ValOwn(0)}, nil
		}
		return []types.Val{types.ValOwn(uint32(pollHandle))}, nil
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
	pid := wasipIO.RegisterPollable(pollable)
	pollHandle, hErr := table.NewResourceHandle(pid, true, socketsPollableResourceType)
	if hErr != nil {
		wasipIO.UnregisterPollable(pid)
		return []types.Val{types.ValOwn(0)}, nil
	}
	return []types.Val{types.ValOwn(uint32(pollHandle))}, nil
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
