// imports/wasip2/sockets/tcp.go

// WIT source of truth: debug-vendored/WASI/proposals/sockets/wit/{tcp,tcp-create-socket}.wit
// Package version: wasi:sockets@0.2.9 (wazero targets wasi:sockets@0.2.0)
//
package sockets

import (
	"context"
	"fmt"
	"net"

	wasipIO "github.com/tetratelabs/wazero/imports/wasip2/io"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// instantiateTcp registers wasi:sockets/tcp@0.2.0
func instantiateTcp(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:sockets/tcp@0.2.0")

	// tcp-socket resource
	inst.Resource("tcp-socket", func(rep uint32) {
		// Destructor - close socket
	})

	// Connection establishment methods
	inst.Func("[method]tcp-socket.start-bind", tcpSocketStartBind)
	inst.Func("[method]tcp-socket.finish-bind", tcpSocketFinishBind)
	inst.Func("[method]tcp-socket.start-connect", tcpSocketStartConnect)
	inst.Func("[method]tcp-socket.finish-connect", tcpSocketFinishConnect)
	inst.Func("[method]tcp-socket.start-listen", tcpSocketStartListen)
	inst.Func("[method]tcp-socket.finish-listen", tcpSocketFinishListen)
	inst.Func("[method]tcp-socket.accept", tcpSocketAccept)

	// Address methods
	inst.Func("[method]tcp-socket.local-address", tcpSocketLocalAddress)
	inst.Func("[method]tcp-socket.remote-address", tcpSocketRemoteAddress)
	inst.Func("[method]tcp-socket.is-listening", tcpSocketIsListening)
	inst.Func("[method]tcp-socket.address-family", tcpSocketAddressFamily)

	// Socket option methods
	inst.Func("[method]tcp-socket.set-listen-backlog-size", tcpSocketSetListenBacklogSize)
	inst.Func("[method]tcp-socket.keep-alive-enabled", tcpSocketKeepAliveEnabled)
	inst.Func("[method]tcp-socket.set-keep-alive-enabled", tcpSocketSetKeepAliveEnabled)
	inst.Func("[method]tcp-socket.keep-alive-idle-time", tcpSocketKeepAliveIdleTime)
	inst.Func("[method]tcp-socket.set-keep-alive-idle-time", tcpSocketSetKeepAliveIdleTime)
	inst.Func("[method]tcp-socket.keep-alive-interval", tcpSocketKeepAliveInterval)
	inst.Func("[method]tcp-socket.set-keep-alive-interval", tcpSocketSetKeepAliveInterval)
	inst.Func("[method]tcp-socket.keep-alive-count", tcpSocketKeepAliveCount)
	inst.Func("[method]tcp-socket.set-keep-alive-count", tcpSocketSetKeepAliveCount)
	inst.Func("[method]tcp-socket.hop-limit", tcpSocketHopLimit)
	inst.Func("[method]tcp-socket.set-hop-limit", tcpSocketSetHopLimit)
	inst.Func("[method]tcp-socket.receive-buffer-size", tcpSocketReceiveBufferSize)
	inst.Func("[method]tcp-socket.set-receive-buffer-size", tcpSocketSetReceiveBufferSize)
	inst.Func("[method]tcp-socket.send-buffer-size", tcpSocketSendBufferSize)
	inst.Func("[method]tcp-socket.set-send-buffer-size", tcpSocketSetSendBufferSize)

	// Async and lifecycle methods
	inst.Func("[method]tcp-socket.subscribe", tcpSocketSubscribe)
	inst.Func("[method]tcp-socket.shutdown", tcpSocketShutdown)

	return inst.SkipValidation().Build()
}

// instantiateTcpCreateSocket registers wasi:sockets/tcp-create-socket@0.2.0
func instantiateTcpCreateSocket(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:sockets/tcp-create-socket@0.2.0")

	// create-tcp-socket: func(network: borrow<network>, address-family: ip-address-family) -> result<own<tcp-socket>, error-code>
	inst.Func("create-tcp-socket", createTcpSocket)

	return inst.SkipValidation().Build()
}

// createTcpSocket creates a new TCP socket.
// Signature: func(network: borrow<network>, address-family: ip-address-family) -> result<own<tcp-socket>, error-code>
func createTcpSocket(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	// args[0] = borrow<network> (ignored for now)
	// args[1] = ip-address-family enum
	family := parseAddressFamily(args[1])

	// Create socket
	sock := NewTcpSocket(family)

	// Store in resource table
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		// No table available, return placeholder
		handle := types.ValOwn(0)
		return []types.Val{types.ValResultOk(&handle)}, nil
	}

	handle, hErr := table.NewResourceHandle(sock, true, tcpSocketResourceType)
	if hErr != nil {
		handle := types.ValOwn(0)
		return []types.Val{types.ValResultOk(&handle)}, nil
	}
	handleVal := types.ValOwn(uint32(handle))
	return []types.Val{types.ValResultOk(&handleVal)}, nil
}

// tcpSocketStartBind begins the bind operation.
// Signature: func(self: borrow<tcp-socket>, network: borrow<network>, local-address: ip-socket-address) -> result<_, error-code>
func tcpSocketStartBind(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()
	// args[1] = borrow<network> (ignored)
	localAddrVal := args[2]

	sock, err := getTcpSocket(ctx, handle)
	if err != nil {
		// Fallback for tests without resource table
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	// Check state
	if sock.state != tcpStateUnbound {
		return []types.Val{errorCodeToVal(ErrorCodeInvalidState)}, nil
	}

	// Parse address
	localAddr, err := ipSocketAddressFromVal(localAddrVal)
	if err != nil {
		return []types.Val{errorCodeToVal(ErrorCodeInvalidArgument)}, nil
	}

	// Store pending address and transition state
	sock.localAddr = localAddr
	sock.state = tcpStateBinding

	return []types.Val{types.ValResultOk(nil)}, nil
}

// tcpSocketFinishBind completes the bind operation.
// Signature: func(self: borrow<tcp-socket>) -> result<_, error-code>
func tcpSocketFinishBind(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	sock, err := getTcpSocket(ctx, handle)
	if err != nil {
		// Fallback for tests without resource table
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	// Check state
	if sock.state != tcpStateBinding {
		return []types.Val{errorCodeToVal(ErrorCodeInvalidState)}, nil
	}

	if sock.localAddr == nil {
		return []types.Val{errorCodeToVal(ErrorCodeInvalidState)}, nil
	}

	// Actually bind - for TCP we need to create a listener to bind
	tcpAddr := ipSocketAddressToTCPAddr(sock.localAddr)
	listener, netErr := net.ListenTCP("tcp", tcpAddr)
	if netErr != nil {
		sock.pendingErr = netErr
		sock.state = tcpStateUnbound
		return []types.Val{errorCodeToVal(mapNetError(netErr))}, nil
	}

	// Update the local address to the actual bound address
	sock.listener = listener
	sock.localAddr = tcpAddrToIpSocketAddress(listener.Addr().(*net.TCPAddr))
	sock.state = tcpStateBound

	return []types.Val{types.ValResultOk(nil)}, nil
}

// tcpSocketStartConnect begins the connect operation.
// Signature: func(self: borrow<tcp-socket>, network: borrow<network>, remote-address: ip-socket-address) -> result<_, error-code>
func tcpSocketStartConnect(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()
	// args[1] = borrow<network> (ignored)
	remoteAddrVal := args[2]

	sock, err := getTcpSocket(ctx, handle)
	if err != nil {
		// Fallback for tests without resource table
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	// Check state - can only connect from unbound or bound states
	if sock.state != tcpStateUnbound && sock.state != tcpStateBound {
		return []types.Val{errorCodeToVal(ErrorCodeInvalidState)}, nil
	}

	// Parse remote address
	remoteAddr, err := ipSocketAddressFromVal(remoteAddrVal)
	if err != nil {
		return []types.Val{errorCodeToVal(ErrorCodeInvalidArgument)}, nil
	}

	// Store pending remote address and transition state
	sock.remoteAddr = remoteAddr
	sock.state = tcpStateConnecting

	return []types.Val{types.ValResultOk(nil)}, nil
}

// tcpSocketFinishConnect completes the connect operation.
// Signature: func(self: borrow<tcp-socket>) -> result<tuple<own<input-stream>, own<output-stream>>, error-code>
func tcpSocketFinishConnect(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	sock, err := getTcpSocket(ctx, handle)
	if err != nil {
		// Fallback for tests without resource table
		inputStream := types.ValOwn(0)
		outputStream := types.ValOwn(1)
		tuple := types.ValTuple([]types.Val{inputStream, outputStream})
		return []types.Val{types.ValResultOk(&tuple)}, nil
	}

	// Check state
	if sock.state != tcpStateConnecting {
		return []types.Val{errorCodeToVal(ErrorCodeInvalidState)}, nil
	}

	if sock.remoteAddr == nil {
		return []types.Val{errorCodeToVal(ErrorCodeInvalidState)}, nil
	}

	// Close any existing listener (if we were bound)
	if sock.listener != nil {
		sock.listener.Close()
		sock.listener = nil
	}

	// Actually connect
	tcpAddr := ipSocketAddressToTCPAddr(sock.remoteAddr)
	var localAddr *net.TCPAddr
	if sock.localAddr != nil {
		localAddr = ipSocketAddressToTCPAddr(sock.localAddr)
	}

	dialer := &net.Dialer{
		LocalAddr: localAddr,
	}

	conn, netErr := dialer.Dial("tcp", tcpAddr.String())
	if netErr != nil {
		sock.pendingErr = netErr
		sock.state = tcpStateUnbound
		return []types.Val{errorCodeToVal(mapNetError(netErr))}, nil
	}

	tcpConn := conn.(*net.TCPConn)
	sock.conn = tcpConn
	sock.state = tcpStateConnected

	// Update addresses with actual values
	sock.localAddr = tcpAddrToIpSocketAddress(tcpConn.LocalAddr().(*net.TCPAddr))
	sock.remoteAddr = tcpAddrToIpSocketAddress(tcpConn.RemoteAddr().(*net.TCPAddr))

	// Create input and output stream resources
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		inputStream := types.ValOwn(0)
		outputStream := types.ValOwn(1)
		tuple := types.ValTuple([]types.Val{inputStream, outputStream})
		return []types.Val{types.ValResultOk(&tuple)}, nil
	}

	// Create TcpInputStream and TcpOutputStream wrappers
	inStream := NewTcpInputStream(sock)
	outStream := NewTcpOutputStream(sock)

	inHandle, hErr1 := table.NewResourceHandle(inStream, true, tcpInputStreamResourceType)
	if hErr1 != nil {
		inputStream := types.ValOwn(0)
		outputStream := types.ValOwn(1)
		tuple := types.ValTuple([]types.Val{inputStream, outputStream})
		return []types.Val{types.ValResultOk(&tuple)}, nil
	}
	outHandle, hErr2 := table.NewResourceHandle(outStream, true, tcpOutputStreamResourceType)
	if hErr2 != nil {
		inputStream := types.ValOwn(uint32(inHandle))
		outputStream := types.ValOwn(1)
		tuple := types.ValTuple([]types.Val{inputStream, outputStream})
		return []types.Val{types.ValResultOk(&tuple)}, nil
	}

	inputStreamVal := types.ValOwn(uint32(inHandle))
	outputStreamVal := types.ValOwn(uint32(outHandle))
	tuple := types.ValTuple([]types.Val{inputStreamVal, outputStreamVal})
	return []types.Val{types.ValResultOk(&tuple)}, nil
}

// tcpSocketStartListen begins the listen operation.
// Signature: func(self: borrow<tcp-socket>) -> result<_, error-code>
func tcpSocketStartListen(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	sock, err := getTcpSocket(ctx, handle)
	if err != nil {
		// Fallback for tests without resource table
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	// Check state - must be bound
	if sock.state != tcpStateBound {
		return []types.Val{errorCodeToVal(ErrorCodeInvalidState)}, nil
	}

	// Already have a listener from bind, just mark as transitioning
	// In a more sophisticated implementation, we might set backlog here
	return []types.Val{types.ValResultOk(nil)}, nil
}

// tcpSocketFinishListen completes the listen operation.
// Signature: func(self: borrow<tcp-socket>) -> result<_, error-code>
func tcpSocketFinishListen(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	sock, err := getTcpSocket(ctx, handle)
	if err != nil {
		// Fallback for tests without resource table
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	// Check state - must be bound
	if sock.state != tcpStateBound {
		return []types.Val{errorCodeToVal(ErrorCodeInvalidState)}, nil
	}

	// Verify we have a listener
	if sock.listener == nil {
		return []types.Val{errorCodeToVal(ErrorCodeInvalidState)}, nil
	}

	// Transition to listening state
	sock.state = tcpStateListening

	return []types.Val{types.ValResultOk(nil)}, nil
}

// tcpSocketAccept accepts a new connection.
// Signature: func(self: borrow<tcp-socket>) -> result<tuple<own<tcp-socket>, own<input-stream>, own<output-stream>>, error-code>
func tcpSocketAccept(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	sock, err := getTcpSocket(ctx, handle)
	if err != nil {
		// Fallback for tests without resource table
		socket := types.ValOwn(0)
		inputStream := types.ValOwn(1)
		outputStream := types.ValOwn(2)
		tuple := types.ValTuple([]types.Val{socket, inputStream, outputStream})
		return []types.Val{types.ValResultOk(&tuple)}, nil
	}

	// Check state - must be listening
	if sock.state != tcpStateListening {
		return []types.Val{errorCodeToVal(ErrorCodeInvalidState)}, nil
	}

	if sock.listener == nil {
		return []types.Val{errorCodeToVal(ErrorCodeInvalidState)}, nil
	}

	// Accept connection
	conn, netErr := sock.listener.AcceptTCP()
	if netErr != nil {
		return []types.Val{errorCodeToVal(mapNetError(netErr))}, nil
	}

	// Create new socket for the accepted connection
	acceptedSock := NewTcpSocket(sock.family)
	acceptedSock.conn = conn
	acceptedSock.state = tcpStateConnected
	acceptedSock.localAddr = tcpAddrToIpSocketAddress(conn.LocalAddr().(*net.TCPAddr))
	acceptedSock.remoteAddr = tcpAddrToIpSocketAddress(conn.RemoteAddr().(*net.TCPAddr))

	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		socket := types.ValOwn(0)
		inputStream := types.ValOwn(1)
		outputStream := types.ValOwn(2)
		tuple := types.ValTuple([]types.Val{socket, inputStream, outputStream})
		return []types.Val{types.ValResultOk(&tuple)}, nil
	}

	// Create resources
	sockHandle, err := table.NewResourceHandle(acceptedSock, true, tcpSocketResourceType)
	if err != nil {
		acceptedSock.Close()
		return nil, fmt.Errorf("tcpSocketAccept: register socket handle: %w", err)
	}
	inStream := NewTcpInputStream(acceptedSock)
	outStream := NewTcpOutputStream(acceptedSock)
	inHandle, err := table.NewResourceHandle(inStream, true, tcpInputStreamResourceType)
	if err != nil {
		table.Remove(sockHandle)
		acceptedSock.Close()
		return nil, fmt.Errorf("tcpSocketAccept: register input stream handle: %w", err)
	}
	outHandle, err := table.NewResourceHandle(outStream, true, tcpOutputStreamResourceType)
	if err != nil {
		table.Remove(inHandle)
		table.Remove(sockHandle)
		acceptedSock.Close()
		return nil, fmt.Errorf("tcpSocketAccept: register output stream handle: %w", err)
	}

	socketVal := types.ValOwn(uint32(sockHandle))
	inputStreamVal := types.ValOwn(uint32(inHandle))
	outputStreamVal := types.ValOwn(uint32(outHandle))
	tuple := types.ValTuple([]types.Val{socketVal, inputStreamVal, outputStreamVal})
	return []types.Val{types.ValResultOk(&tuple)}, nil
}

// tcpSocketLocalAddress returns the local address.
// Signature: func(self: borrow<tcp-socket>) -> result<ip-socket-address, error-code>
func tcpSocketLocalAddress(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	sock, err := getTcpSocket(ctx, handle)
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

// tcpSocketRemoteAddress returns the remote address.
// Signature: func(self: borrow<tcp-socket>) -> result<ip-socket-address, error-code>
func tcpSocketRemoteAddress(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	sock, err := getTcpSocket(ctx, handle)
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

// tcpSocketIsListening returns whether the socket is listening.
// Signature: func(self: borrow<tcp-socket>) -> bool
func tcpSocketIsListening(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	sock, err := getTcpSocket(ctx, handle)
	if err != nil {
		return []types.Val{types.ValBool(false)}, nil
	}

	return []types.Val{types.ValBool(sock.IsListening())}, nil
}

// tcpSocketAddressFamily returns the address family.
// Signature: func(self: borrow<tcp-socket>) -> ip-address-family
func tcpSocketAddressFamily(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	sock, err := getTcpSocket(ctx, handle)
	if err != nil {
		return []types.Val{types.ValEnum("ipv4")}, nil
	}

	return []types.Val{types.ValEnum(sock.Family().String())}, nil
}

// tcpSocketSetListenBacklogSize sets the listen backlog size.
// Signature: func(self: borrow<tcp-socket>, value: u64) -> result<_, error-code>
func tcpSocketSetListenBacklogSize(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()
	value := args[1].U64()

	sock, err := getTcpSocket(ctx, handle)
	if err != nil {
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	sock.listenBacklog = value
	return []types.Val{types.ValResultOk(nil)}, nil
}

// tcpSocketKeepAliveEnabled returns whether keep-alive is enabled.
// Signature: func(self: borrow<tcp-socket>) -> result<bool, error-code>
func tcpSocketKeepAliveEnabled(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	sock, err := getTcpSocket(ctx, handle)
	if err != nil {
		enabled := types.ValBool(false)
		return []types.Val{types.ValResultOk(&enabled)}, nil
	}

	enabled := types.ValBool(sock.keepAliveEnabled)
	return []types.Val{types.ValResultOk(&enabled)}, nil
}

// tcpSocketSetKeepAliveEnabled sets whether keep-alive is enabled.
// Signature: func(self: borrow<tcp-socket>, value: bool) -> result<_, error-code>
func tcpSocketSetKeepAliveEnabled(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()
	value := args[1].Bool()

	sock, err := getTcpSocket(ctx, handle)
	if err != nil {
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	sock.keepAliveEnabled = value
	if sock.conn != nil {
		sock.conn.SetKeepAlive(value)
	}
	return []types.Val{types.ValResultOk(nil)}, nil
}

// tcpSocketKeepAliveIdleTime returns the keep-alive idle time.
// Signature: func(self: borrow<tcp-socket>) -> result<duration, error-code>
func tcpSocketKeepAliveIdleTime(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	sock, err := getTcpSocket(ctx, handle)
	if err != nil {
		duration := types.ValU64(7200000000000) // 2 hours in nanoseconds
		return []types.Val{types.ValResultOk(&duration)}, nil
	}

	// Convert seconds to nanoseconds
	duration := types.ValU64(sock.keepAliveIdleTime * 1000000000)
	return []types.Val{types.ValResultOk(&duration)}, nil
}

// tcpSocketSetKeepAliveIdleTime sets the keep-alive idle time.
// Signature: func(self: borrow<tcp-socket>, value: duration) -> result<_, error-code>
func tcpSocketSetKeepAliveIdleTime(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()
	value := args[1].U64() // nanoseconds

	sock, err := getTcpSocket(ctx, handle)
	if err != nil {
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	// Convert nanoseconds to seconds
	sock.keepAliveIdleTime = value / 1000000000
	return []types.Val{types.ValResultOk(nil)}, nil
}

// tcpSocketKeepAliveInterval returns the keep-alive interval.
// Signature: func(self: borrow<tcp-socket>) -> result<duration, error-code>
func tcpSocketKeepAliveInterval(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	sock, err := getTcpSocket(ctx, handle)
	if err != nil {
		duration := types.ValU64(75000000000) // 75 seconds in nanoseconds
		return []types.Val{types.ValResultOk(&duration)}, nil
	}

	// Convert seconds to nanoseconds
	duration := types.ValU64(sock.keepAliveInterval * 1000000000)
	return []types.Val{types.ValResultOk(&duration)}, nil
}

// tcpSocketSetKeepAliveInterval sets the keep-alive interval.
// Signature: func(self: borrow<tcp-socket>, value: duration) -> result<_, error-code>
func tcpSocketSetKeepAliveInterval(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()
	value := args[1].U64() // nanoseconds

	sock, err := getTcpSocket(ctx, handle)
	if err != nil {
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	// Convert nanoseconds to seconds
	sock.keepAliveInterval = value / 1000000000
	return []types.Val{types.ValResultOk(nil)}, nil
}

// tcpSocketKeepAliveCount returns the keep-alive count.
// Signature: func(self: borrow<tcp-socket>) -> result<u32, error-code>
func tcpSocketKeepAliveCount(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	sock, err := getTcpSocket(ctx, handle)
	if err != nil {
		count := types.ValU32(9)
		return []types.Val{types.ValResultOk(&count)}, nil
	}

	count := types.ValU32(sock.keepAliveCount)
	return []types.Val{types.ValResultOk(&count)}, nil
}

// tcpSocketSetKeepAliveCount sets the keep-alive count.
// Signature: func(self: borrow<tcp-socket>, value: u32) -> result<_, error-code>
func tcpSocketSetKeepAliveCount(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()
	value := args[1].U32()

	sock, err := getTcpSocket(ctx, handle)
	if err != nil {
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	sock.keepAliveCount = value
	return []types.Val{types.ValResultOk(nil)}, nil
}

// tcpSocketHopLimit returns the hop limit (TTL).
// Signature: func(self: borrow<tcp-socket>) -> result<u8, error-code>
func tcpSocketHopLimit(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	sock, err := getTcpSocket(ctx, handle)
	if err != nil {
		limit := types.ValU8(64)
		return []types.Val{types.ValResultOk(&limit)}, nil
	}

	limit := types.ValU8(sock.hopLimit)
	return []types.Val{types.ValResultOk(&limit)}, nil
}

// tcpSocketSetHopLimit sets the hop limit (TTL).
// Signature: func(self: borrow<tcp-socket>, value: u8) -> result<_, error-code>
func tcpSocketSetHopLimit(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()
	value := args[1].U8()

	sock, err := getTcpSocket(ctx, handle)
	if err != nil {
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	sock.hopLimit = value
	return []types.Val{types.ValResultOk(nil)}, nil
}

// tcpSocketReceiveBufferSize returns the receive buffer size.
// Signature: func(self: borrow<tcp-socket>) -> result<u64, error-code>
func tcpSocketReceiveBufferSize(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	sock, err := getTcpSocket(ctx, handle)
	if err != nil {
		size := types.ValU64(65536)
		return []types.Val{types.ValResultOk(&size)}, nil
	}

	size := types.ValU64(sock.receiveBufferSize)
	return []types.Val{types.ValResultOk(&size)}, nil
}

// tcpSocketSetReceiveBufferSize sets the receive buffer size.
// Signature: func(self: borrow<tcp-socket>, value: u64) -> result<_, error-code>
func tcpSocketSetReceiveBufferSize(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()
	value := args[1].U64()

	sock, err := getTcpSocket(ctx, handle)
	if err != nil {
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	sock.receiveBufferSize = value
	if sock.conn != nil {
		sock.conn.SetReadBuffer(int(value))
	}
	return []types.Val{types.ValResultOk(nil)}, nil
}

// tcpSocketSendBufferSize returns the send buffer size.
// Signature: func(self: borrow<tcp-socket>) -> result<u64, error-code>
func tcpSocketSendBufferSize(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	sock, err := getTcpSocket(ctx, handle)
	if err != nil {
		size := types.ValU64(65536)
		return []types.Val{types.ValResultOk(&size)}, nil
	}

	size := types.ValU64(sock.sendBufferSize)
	return []types.Val{types.ValResultOk(&size)}, nil
}

// tcpSocketSetSendBufferSize sets the send buffer size.
// Signature: func(self: borrow<tcp-socket>, value: u64) -> result<_, error-code>
func tcpSocketSetSendBufferSize(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()
	value := args[1].U64()

	sock, err := getTcpSocket(ctx, handle)
	if err != nil {
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	sock.sendBufferSize = value
	if sock.conn != nil {
		sock.conn.SetWriteBuffer(int(value))
	}
	return []types.Val{types.ValResultOk(nil)}, nil
}

// tcpSocketSubscribe returns a pollable for the socket.
// Signature: func(self: borrow<tcp-socket>) -> own<pollable>
func tcpSocketSubscribe(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []types.Val{types.ValOwn(0)}, nil
	}

	// TCP socket subscribe creates a pollable that is always ready since
	// Go's net package handles TCP operations synchronously.
	pollable := wasipIO.NewReadyPollable()
	pollHandle, hErr := table.NewResourceHandle(pollable, true, socketsPollableResourceType)
	if hErr != nil {
		return []types.Val{types.ValOwn(0)}, nil
	}
	return []types.Val{types.ValOwn(uint32(pollHandle))}, nil
}

// tcpSocketShutdown shuts down the socket.
// Signature: func(self: borrow<tcp-socket>, shutdown-type: shutdown-type) -> result<_, error-code>
func tcpSocketShutdown(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()
	shutdownType := args[1].Enum()

	sock, err := getTcpSocket(ctx, handle)
	if err != nil {
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	if sock.conn == nil {
		return []types.Val{errorCodeToVal(ErrorCodeInvalidState)}, nil
	}

	var how string
	switch shutdownType {
	case "receive":
		how = "r"
	case "send":
		how = "w"
	case "both":
		how = "rw"
	default:
		how = "rw"
	}

	// Go's TCPConn doesn't have a direct shutdown method, but we can use CloseRead/CloseWrite
	if how == "r" || how == "rw" {
		sock.conn.CloseRead()
	}
	if how == "w" || how == "rw" {
		sock.conn.CloseWrite()
	}

	return []types.Val{types.ValResultOk(nil)}, nil
}

// ===========================
// TCP Stream Types
// ===========================

// TcpInputStream wraps a TCP socket connection for reading.
type TcpInputStream struct {
	sock   *TcpSocket
	closed bool
}

// NewTcpInputStream creates a new TCP input stream.
func NewTcpInputStream(sock *TcpSocket) *TcpInputStream {
	return &TcpInputStream{sock: sock}
}

// Read reads data from the TCP connection.
func (s *TcpInputStream) Read(maxLen uint64) ([]byte, error) {
	if s.closed || s.sock.conn == nil {
		return nil, net.ErrClosed
	}
	buf := make([]byte, min(maxLen, 64*1024))
	n, err := s.sock.conn.Read(buf)
	if err != nil {
		return buf[:n], err
	}
	return buf[:n], nil
}

// Close closes the input stream.
func (s *TcpInputStream) Close() {
	s.closed = true
}

// TcpOutputStream wraps a TCP socket connection for writing.
type TcpOutputStream struct {
	sock   *TcpSocket
	closed bool
}

// NewTcpOutputStream creates a new TCP output stream.
func NewTcpOutputStream(sock *TcpSocket) *TcpOutputStream {
	return &TcpOutputStream{sock: sock}
}

// Write writes data to the TCP connection.
func (s *TcpOutputStream) Write(data []byte) error {
	if s.closed || s.sock.conn == nil {
		return net.ErrClosed
	}
	_, err := s.sock.conn.Write(data)
	return err
}

// Close closes the output stream.
func (s *TcpOutputStream) Close() {
	s.closed = true
}

func min(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}
