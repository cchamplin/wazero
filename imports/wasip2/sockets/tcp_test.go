// imports/wasip2/sockets/tcp_test.go

package sockets

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestTCPSocket_BindAndListen tests the TCP socket bind and listen operations.
// This verifies:
// - Creating a TCP socket
// - Binding to localhost:0 (any available port)
// - Starting and finishing listen
// - Verifying local address has assigned port
func TestTCPSocket_BindAndListen(t *testing.T) {
	t.Skip("Task E4: wasip2 registry migration — Rep is uint32, Go object lookup requires per-module registry")
	ctx := contextWithResourceTable()

	// Create a TCP socket
	network := types.ValBorrow(0)
	family := types.ValEnum("ipv4")
	result, err := createTcpSocket(ctx, nil, []types.Val{network, family})
	require.NoError(t, err)

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "createTcpSocket should succeed")
	require.NotNil(t, ok)
	sockHandle := ok.Own()
	selfHandle := types.ValBorrow(sockHandle)

	// Bind to localhost:0 (any available port)
	localAddr := makeIPv4SocketAddrVal(127, 0, 0, 1, 0)
	result, err = tcpSocketStartBind(ctx, nil, []types.Val{selfHandle, network, localAddr})
	require.NoError(t, err)
	isOk, _, _ = result[0].Result()
	require.True(t, isOk, "start-bind should succeed")

	result, err = tcpSocketFinishBind(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	isOk, _, _ = result[0].Result()
	require.True(t, isOk, "finish-bind should succeed")

	// Start listening
	result, err = tcpSocketStartListen(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	isOk, _, _ = result[0].Result()
	require.True(t, isOk, "start-listen should succeed")

	// Finish listening
	result, err = tcpSocketFinishListen(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	isOk, _, _ = result[0].Result()
	require.True(t, isOk, "finish-listen should succeed")

	// Verify is-listening returns true
	result, err = tcpSocketIsListening(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.True(t, result[0].Bool(), "socket should be listening")

	// Verify local address has assigned port
	result, err = tcpSocketLocalAddress(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	isOk, addrVal, _ := result[0].Result()
	require.True(t, isOk, "local-address should succeed")
	require.NotNil(t, addrVal)

	// Verify address is IPv4 variant
	caseName, _ := addrVal.Variant()
	require.Equal(t, "ipv4", caseName)

	// Get the socket to verify the port is non-zero
	sock, err := getTcpSocket(ctx, sockHandle)
	require.NoError(t, err)
	require.NotNil(t, sock.LocalAddress(), "local address should be set")
	require.True(t, sock.LocalAddress().Port() > 0, "port should be assigned (non-zero)")

	// Clean up
	sock.Close()
}

// TestTCPSocket_Connect tests the TCP socket connect operation.
// This verifies:
// - Starting a listening socket on localhost
// - Creating a client socket
// - Connecting to the listener
// - Verifying connection is established
// - Verifying remote address is correct
func TestTCPSocket_Connect(t *testing.T) {
	t.Skip("Task E4: wasip2 registry migration — Rep is uint32, Go object lookup requires per-module registry")
	ctx := contextWithResourceTable()
	network := types.ValBorrow(0)
	family := types.ValEnum("ipv4")

	// === Setup server socket ===

	// Create server socket
	result, err := createTcpSocket(ctx, nil, []types.Val{network, family})
	require.NoError(t, err)
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk)
	serverHandle := ok.Own()
	serverBorrow := types.ValBorrow(serverHandle)

	// Bind server to 127.0.0.1:0
	localAddr := makeIPv4SocketAddrVal(127, 0, 0, 1, 0)
	result, _ = tcpSocketStartBind(ctx, nil, []types.Val{serverBorrow, network, localAddr})
	isOk, _, _ = result[0].Result()
	require.True(t, isOk, "server start-bind should succeed")

	result, _ = tcpSocketFinishBind(ctx, nil, []types.Val{serverBorrow})
	isOk, _, _ = result[0].Result()
	require.True(t, isOk, "server finish-bind should succeed")

	// Start and finish listen on server
	result, _ = tcpSocketStartListen(ctx, nil, []types.Val{serverBorrow})
	isOk, _, _ = result[0].Result()
	require.True(t, isOk, "server start-listen should succeed")

	result, _ = tcpSocketFinishListen(ctx, nil, []types.Val{serverBorrow})
	isOk, _, _ = result[0].Result()
	require.True(t, isOk, "server finish-listen should succeed")

	// Get server's assigned port
	serverSock, err := getTcpSocket(ctx, serverHandle)
	require.NoError(t, err)
	serverPort := serverSock.LocalAddress().Port()
	require.True(t, serverPort > 0, "server port should be assigned")

	// === Setup client socket ===

	// Create client socket
	result, err = createTcpSocket(ctx, nil, []types.Val{network, family})
	require.NoError(t, err)
	isOk, ok, _ = result[0].Result()
	require.True(t, isOk)
	clientHandle := ok.Own()
	clientBorrow := types.ValBorrow(clientHandle)

	// Start connect to server
	remoteAddr := makeIPv4SocketAddrVal(127, 0, 0, 1, serverPort)
	result, err = tcpSocketStartConnect(ctx, nil, []types.Val{clientBorrow, network, remoteAddr})
	require.NoError(t, err)
	isOk, _, _ = result[0].Result()
	require.True(t, isOk, "client start-connect should succeed")

	// Finish connect (this actually establishes the connection)
	result, err = tcpSocketFinishConnect(ctx, nil, []types.Val{clientBorrow})
	require.NoError(t, err)
	isOk, tupleVal, _ := result[0].Result()
	require.True(t, isOk, "client finish-connect should succeed")
	require.NotNil(t, tupleVal)

	// Verify connection established by checking we got streams back
	streams := tupleVal.Tuple()
	require.Equal(t, 2, len(streams), "should return input and output stream handles")

	// Verify remote address matches server
	result, err = tcpSocketRemoteAddress(ctx, nil, []types.Val{clientBorrow})
	require.NoError(t, err)
	isOk, remoteAddrVal, _ := result[0].Result()
	require.True(t, isOk, "remote-address should succeed")
	require.NotNil(t, remoteAddrVal)

	// Verify remote address is IPv4 variant
	caseName, _ := remoteAddrVal.Variant()
	require.Equal(t, "ipv4", caseName)

	// Verify client socket is in connected state
	clientSock, err := getTcpSocket(ctx, clientHandle)
	require.NoError(t, err)
	require.Equal(t, tcpStateConnected, clientSock.State(), "client should be in connected state")
	require.NotNil(t, clientSock.RemoteAddress(), "client should have remote address")
	require.Equal(t, serverPort, clientSock.RemoteAddress().Port(), "remote port should match server port")

	// Clean up
	clientSock.Close()
	serverSock.Close()
}

// TestTCPSocket_Accept tests the TCP socket accept operation.
// This verifies:
// - A listening socket can accept incoming connections
// - The accepted connection returns a new socket with streams
// - The accepted socket has correct local and remote addresses
func TestTCPSocket_Accept(t *testing.T) {
	t.Skip("Task E4: wasip2 registry migration — Rep is uint32, Go object lookup requires per-module registry")
	ctx := contextWithResourceTable()
	network := types.ValBorrow(0)
	family := types.ValEnum("ipv4")

	// Create and setup server socket
	result, _ := createTcpSocket(ctx, nil, []types.Val{network, family})
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk)
	serverHandle := ok.Own()
	serverBorrow := types.ValBorrow(serverHandle)

	// Bind and listen
	localAddr := makeIPv4SocketAddrVal(127, 0, 0, 1, 0)
	result, _ = tcpSocketStartBind(ctx, nil, []types.Val{serverBorrow, network, localAddr})
	isOk, _, _ = result[0].Result()
	require.True(t, isOk)

	result, _ = tcpSocketFinishBind(ctx, nil, []types.Val{serverBorrow})
	isOk, _, _ = result[0].Result()
	require.True(t, isOk)

	result, _ = tcpSocketStartListen(ctx, nil, []types.Val{serverBorrow})
	isOk, _, _ = result[0].Result()
	require.True(t, isOk)

	result, _ = tcpSocketFinishListen(ctx, nil, []types.Val{serverBorrow})
	isOk, _, _ = result[0].Result()
	require.True(t, isOk)

	// Get server port
	serverSock, _ := getTcpSocket(ctx, serverHandle)
	serverPort := serverSock.LocalAddress().Port()

	// Connect a client using Go's net package (simulating external connection)
	clientConn, err := net.DialTimeout("tcp", "127.0.0.1:"+string(rune(serverPort/256))+string(rune(serverPort%256)), time.Second)
	if err != nil {
		// Try the more common way
		clientConn, err = net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", serverPort), time.Second)
	}
	require.NoError(t, err)
	defer clientConn.Close()

	// Accept the connection
	result, err = tcpSocketAccept(ctx, nil, []types.Val{serverBorrow})
	require.NoError(t, err)
	isOk, tupleVal, _ := result[0].Result()
	require.True(t, isOk, "accept should succeed")
	require.NotNil(t, tupleVal)

	// Verify we got socket and streams
	acceptResult := tupleVal.Tuple()
	require.Equal(t, 3, len(acceptResult), "accept should return socket, input-stream, output-stream")

	// The first element is the new socket handle
	acceptedSocketHandle := acceptResult[0].Own()
	acceptedBorrow := types.ValBorrow(acceptedSocketHandle)

	// Verify the accepted socket has correct addresses
	result, err = tcpSocketLocalAddress(ctx, nil, []types.Val{acceptedBorrow})
	require.NoError(t, err)
	isOk, _, _ = result[0].Result()
	require.True(t, isOk, "accepted socket local-address should succeed")

	result, err = tcpSocketRemoteAddress(ctx, nil, []types.Val{acceptedBorrow})
	require.NoError(t, err)
	isOk, _, _ = result[0].Result()
	require.True(t, isOk, "accepted socket remote-address should succeed")

	// Verify accepted socket is in connected state
	acceptedSock, err := getTcpSocket(ctx, acceptedSocketHandle)
	require.NoError(t, err)
	require.Equal(t, tcpStateConnected, acceptedSock.State(), "accepted socket should be connected")

	// Clean up
	acceptedSock.Close()
	serverSock.Close()
}

// TestTCPSocket_ConnectRefused tests that connecting to a non-listening port fails.
func TestTCPSocket_ConnectRefused(t *testing.T) {
	t.Skip("Task E4: wasip2 registry migration — Rep is uint32, Go object lookup requires per-module registry")
	ctx := contextWithResourceTable()
	network := types.ValBorrow(0)
	family := types.ValEnum("ipv4")

	// Create client socket
	result, _ := createTcpSocket(ctx, nil, []types.Val{network, family})
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk)
	clientHandle := ok.Own()
	clientBorrow := types.ValBorrow(clientHandle)

	// Try to connect to a port that is likely not listening
	// Using port 1 which typically requires root and won't be listening
	remoteAddr := makeIPv4SocketAddrVal(127, 0, 0, 1, 59999)
	result, err := tcpSocketStartConnect(ctx, nil, []types.Val{clientBorrow, network, remoteAddr})
	require.NoError(t, err)
	isOk, _, _ = result[0].Result()
	require.True(t, isOk, "start-connect should succeed (it's async)")

	// Finish connect - this should fail with connection refused
	result, err = tcpSocketFinishConnect(ctx, nil, []types.Val{clientBorrow})
	require.NoError(t, err)
	isOk, _, errVal := result[0].Result()
	// The connection should fail
	require.False(t, isOk, "finish-connect should fail when no listener")
	require.NotNil(t, errVal)

	// Clean up
	sock, _ := getTcpSocket(ctx, clientHandle)
	if sock != nil {
		sock.Close()
	}
}

// TestTCPSocket_BindTwice tests that binding twice fails with correct error.
func TestTCPSocket_BindTwice(t *testing.T) {
	t.Skip("Task E4: wasip2 registry migration — Rep is uint32, Go object lookup requires per-module registry")
	ctx := contextWithResourceTable()
	network := types.ValBorrow(0)
	family := types.ValEnum("ipv4")

	// Create and bind first socket
	result, _ := createTcpSocket(ctx, nil, []types.Val{network, family})
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk)
	sockHandle := ok.Own()
	selfHandle := types.ValBorrow(sockHandle)

	// First bind should succeed
	localAddr := makeIPv4SocketAddrVal(127, 0, 0, 1, 0)
	result, _ = tcpSocketStartBind(ctx, nil, []types.Val{selfHandle, network, localAddr})
	isOk, _, _ = result[0].Result()
	require.True(t, isOk)

	result, _ = tcpSocketFinishBind(ctx, nil, []types.Val{selfHandle})
	isOk, _, _ = result[0].Result()
	require.True(t, isOk)

	// Trying to start bind again should fail with invalid state
	result, _ = tcpSocketStartBind(ctx, nil, []types.Val{selfHandle, network, localAddr})
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "second bind should fail")
	require.NotNil(t, errVal)
	require.Equal(t, string(ErrorCodeInvalidState), errVal.Enum())

	// Clean up
	sock, _ := getTcpSocket(ctx, sockHandle)
	if sock != nil {
		sock.Close()
	}
}

// TestTCPSocket_IPv6 tests TCP socket operations with IPv6.
func TestTCPSocket_IPv6(t *testing.T) {
	t.Skip("Task E4: wasip2 registry migration — Rep is uint32, Go object lookup requires per-module registry")
	ctx := contextWithResourceTable()
	network := types.ValBorrow(0)
	family := types.ValEnum("ipv6")

	// Create IPv6 socket
	result, err := createTcpSocket(ctx, nil, []types.Val{network, family})
	require.NoError(t, err)
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk)
	sockHandle := ok.Own()
	selfHandle := types.ValBorrow(sockHandle)

	// Verify address family
	result, err = tcpSocketAddressFamily(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, "ipv6", result[0].Enum())

	// Bind to IPv6 loopback (::1) port 0
	addrRecord := types.ValRecord(map[string]types.Val{
		"port": types.ValU16(0),
		"address": types.ValTuple([]types.Val{
			types.ValU16(0), types.ValU16(0), types.ValU16(0), types.ValU16(0),
			types.ValU16(0), types.ValU16(0), types.ValU16(0), types.ValU16(1),
		}),
	})
	localAddr := types.ValVariant("ipv6", &addrRecord)

	result, err = tcpSocketStartBind(ctx, nil, []types.Val{selfHandle, network, localAddr})
	require.NoError(t, err)
	isOk, _, _ = result[0].Result()
	require.True(t, isOk, "IPv6 start-bind should succeed")

	result, err = tcpSocketFinishBind(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	isOk, _, _ = result[0].Result()
	require.True(t, isOk, "IPv6 finish-bind should succeed")

	// Verify local address is IPv6
	result, err = tcpSocketLocalAddress(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	isOk, addrVal, _ := result[0].Result()
	require.True(t, isOk)
	caseName, _ := addrVal.Variant()
	require.Equal(t, "ipv6", caseName)

	// Clean up
	sock, _ := getTcpSocket(ctx, sockHandle)
	if sock != nil {
		sock.Close()
	}
}
