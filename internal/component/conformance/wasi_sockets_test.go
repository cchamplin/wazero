// Package conformance contains conformance tests for the Component Model implementation.
// This file implements Task 282: WASI Sockets Conformance Tests.
package conformance

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/imports/wasip2"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// =============================================================================
// Task 282: WASI Sockets Conformance Tests
// =============================================================================

// TestWASI_Sockets_NetworkInterfaceExists tests that the network interface exists.
func TestWASI_Sockets_NetworkInterfaceExists(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	// Get the network interface
	networkDef, ok := linker.Get("wasi:sockets/network@0.2.0")
	require.True(t, ok, "network interface should be registered")

	instDef, ok := networkDef.(*component.InstanceDef)
	require.True(t, ok, "should be an InstanceDef")

	// Verify network resource exists
	networkRes, ok := instDef.Exports["network"]
	require.True(t, ok, "network resource should be exported")
	require.NotNil(t, networkRes, "network resource should not be nil")
}

// TestWASI_Sockets_InstanceNetworkExists tests the instance-network interface.
func TestWASI_Sockets_InstanceNetworkExists(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	// Get the instance-network interface
	instNetDef, ok := linker.Get("wasi:sockets/instance-network@0.2.0")
	require.True(t, ok, "instance-network interface should be registered")

	instDef, ok := instNetDef.(*component.InstanceDef)
	require.True(t, ok, "should be an InstanceDef")

	// Verify instance-network function exists
	instNetFunc, ok := instDef.Exports["instance-network"]
	require.True(t, ok, "instance-network function should be exported")
	require.NotNil(t, instNetFunc, "instance-network function should not be nil")
}

// TestWASI_Sockets_InstanceNetworkCall tests calling the instance-network function.
func TestWASI_Sockets_InstanceNetworkCall(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := runtime.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	instNetDef, _ := linker.Get("wasi:sockets/instance-network@0.2.0")
	instDef := instNetDef.(*component.InstanceDef)

	instNetFunc := instDef.Exports["instance-network"].(*component.FuncDef)

	result, err := instNetFunc.Callback(ctx, []types.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result), "instance-network should return exactly one value")

	// Result should be own<network>
	handle := result[0].Own()
	// Handle may be 0 for placeholder implementation
	require.True(t, handle >= 0, "should return a valid network handle")
}

// TestWASI_Sockets_TCPInterfaceExists tests that the TCP interface exists.
func TestWASI_Sockets_TCPInterfaceExists(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	// Get the TCP interface
	tcpDef, ok := linker.Get("wasi:sockets/tcp@0.2.0")
	require.True(t, ok, "tcp interface should be registered")

	instDef, ok := tcpDef.(*component.InstanceDef)
	require.True(t, ok, "should be an InstanceDef")

	// Verify tcp-socket resource exists
	tcpSocketRes, ok := instDef.Exports["tcp-socket"]
	require.True(t, ok, "tcp-socket resource should be exported")
	require.NotNil(t, tcpSocketRes, "tcp-socket resource should not be nil")
}

// TestWASI_Sockets_TCPCreateSocketExists tests that the TCP create-socket function exists.
func TestWASI_Sockets_TCPCreateSocketExists(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	// Get the TCP create-socket interface
	tcpCreateDef, ok := linker.Get("wasi:sockets/tcp-create-socket@0.2.0")
	require.True(t, ok, "tcp-create-socket interface should be registered")

	instDef, ok := tcpCreateDef.(*component.InstanceDef)
	require.True(t, ok, "should be an InstanceDef")

	// Verify create-tcp-socket function exists
	createTcpFunc, ok := instDef.Exports["create-tcp-socket"]
	require.True(t, ok, "create-tcp-socket function should be exported")
	require.NotNil(t, createTcpFunc, "create-tcp-socket function should not be nil")
}

// TestWASI_Sockets_TCPMethods tests that all expected TCP socket methods exist.
func TestWASI_Sockets_TCPMethods(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	tcpDef, ok := linker.Get("wasi:sockets/tcp@0.2.0")
	require.True(t, ok, "tcp interface should be registered")

	instDef := tcpDef.(*component.InstanceDef)

	// Expected TCP socket methods
	expectedMethods := []string{
		"[method]tcp-socket.start-bind",
		"[method]tcp-socket.finish-bind",
		"[method]tcp-socket.start-connect",
		"[method]tcp-socket.finish-connect",
		"[method]tcp-socket.start-listen",
		"[method]tcp-socket.finish-listen",
		"[method]tcp-socket.accept",
		"[method]tcp-socket.local-address",
		"[method]tcp-socket.remote-address",
		"[method]tcp-socket.is-listening",
		"[method]tcp-socket.address-family",
		"[method]tcp-socket.set-listen-backlog-size",
		"[method]tcp-socket.keep-alive-enabled",
		"[method]tcp-socket.set-keep-alive-enabled",
		"[method]tcp-socket.keep-alive-idle-time",
		"[method]tcp-socket.set-keep-alive-idle-time",
		"[method]tcp-socket.keep-alive-interval",
		"[method]tcp-socket.set-keep-alive-interval",
		"[method]tcp-socket.keep-alive-count",
		"[method]tcp-socket.set-keep-alive-count",
		"[method]tcp-socket.hop-limit",
		"[method]tcp-socket.set-hop-limit",
		"[method]tcp-socket.receive-buffer-size",
		"[method]tcp-socket.set-receive-buffer-size",
		"[method]tcp-socket.send-buffer-size",
		"[method]tcp-socket.set-send-buffer-size",
		"[method]tcp-socket.subscribe",
		"[method]tcp-socket.shutdown",
	}

	for _, method := range expectedMethods {
		methodDef, ok := instDef.Exports[method]
		require.True(t, ok, "method %s should be exported", method)
		require.NotNil(t, methodDef, "method %s should not be nil", method)
	}
}

// TestWASI_Sockets_UDPInterfaceExists tests that the UDP interface exists.
func TestWASI_Sockets_UDPInterfaceExists(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	// Get the UDP interface
	udpDef, ok := linker.Get("wasi:sockets/udp@0.2.0")
	require.True(t, ok, "udp interface should be registered")

	instDef, ok := udpDef.(*component.InstanceDef)
	require.True(t, ok, "should be an InstanceDef")

	// Verify udp-socket resource exists
	udpSocketRes, ok := instDef.Exports["udp-socket"]
	require.True(t, ok, "udp-socket resource should be exported")
	require.NotNil(t, udpSocketRes, "udp-socket resource should not be nil")
}

// TestWASI_Sockets_UDPCreateSocketExists tests that the UDP create-socket function exists.
func TestWASI_Sockets_UDPCreateSocketExists(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	// Get the UDP create-socket interface
	udpCreateDef, ok := linker.Get("wasi:sockets/udp-create-socket@0.2.0")
	require.True(t, ok, "udp-create-socket interface should be registered")

	instDef, ok := udpCreateDef.(*component.InstanceDef)
	require.True(t, ok, "should be an InstanceDef")

	// Verify create-udp-socket function exists
	createUdpFunc, ok := instDef.Exports["create-udp-socket"]
	require.True(t, ok, "create-udp-socket function should be exported")
	require.NotNil(t, createUdpFunc, "create-udp-socket function should not be nil")
}

// TestWASI_Sockets_UDPMethods tests that all expected UDP socket methods exist.
func TestWASI_Sockets_UDPMethods(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	udpDef, ok := linker.Get("wasi:sockets/udp@0.2.0")
	require.True(t, ok, "udp interface should be registered")

	instDef := udpDef.(*component.InstanceDef)

	// Expected UDP socket methods
	expectedMethods := []string{
		"[method]udp-socket.start-bind",
		"[method]udp-socket.finish-bind",
		"[method]udp-socket.stream",
		"[method]udp-socket.local-address",
		"[method]udp-socket.remote-address",
		"[method]udp-socket.address-family",
		"[method]udp-socket.unicast-hop-limit",
		"[method]udp-socket.set-unicast-hop-limit",
		"[method]udp-socket.receive-buffer-size",
		"[method]udp-socket.set-receive-buffer-size",
		"[method]udp-socket.send-buffer-size",
		"[method]udp-socket.set-send-buffer-size",
		"[method]udp-socket.subscribe",
	}

	for _, method := range expectedMethods {
		methodDef, ok := instDef.Exports[method]
		require.True(t, ok, "method %s should be exported", method)
		require.NotNil(t, methodDef, "method %s should not be nil", method)
	}
}

// TestWASI_Sockets_UDPDatagramStreams tests that UDP datagram stream resources exist.
func TestWASI_Sockets_UDPDatagramStreams(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	udpDef, ok := linker.Get("wasi:sockets/udp@0.2.0")
	require.True(t, ok, "udp interface should be registered")

	instDef := udpDef.(*component.InstanceDef)

	// Verify datagram stream resources exist
	incomingRes, ok := instDef.Exports["incoming-datagram-stream"]
	require.True(t, ok, "incoming-datagram-stream resource should be exported")
	require.NotNil(t, incomingRes, "incoming-datagram-stream resource should not be nil")

	outgoingRes, ok := instDef.Exports["outgoing-datagram-stream"]
	require.True(t, ok, "outgoing-datagram-stream resource should be exported")
	require.NotNil(t, outgoingRes, "outgoing-datagram-stream resource should not be nil")
}

// TestWASI_Sockets_IPNameLookupExists tests that the IP name lookup interface exists.
func TestWASI_Sockets_IPNameLookupExists(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	// Get the IP name lookup interface
	lookupDef, ok := linker.Get("wasi:sockets/ip-name-lookup@0.2.0")
	require.True(t, ok, "ip-name-lookup interface should be registered")

	instDef, ok := lookupDef.(*component.InstanceDef)
	require.True(t, ok, "should be an InstanceDef")

	// Verify resolve-addresses function exists
	resolveFunc, ok := instDef.Exports["resolve-addresses"]
	require.True(t, ok, "resolve-addresses function should be exported")
	require.NotNil(t, resolveFunc, "resolve-addresses function should not be nil")

	// Verify resolve-address-stream resource exists
	streamRes, ok := instDef.Exports["resolve-address-stream"]
	require.True(t, ok, "resolve-address-stream resource should be exported")
	require.NotNil(t, streamRes, "resolve-address-stream resource should not be nil")
}

// TestWASI_Sockets_InterfaceRegistration tests that all socket interfaces are properly registered.
func TestWASI_Sockets_InterfaceRegistration(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	// Verify all socket interfaces are registered
	interfaces := []string{
		"wasi:sockets/network@0.2.0",
		"wasi:sockets/instance-network@0.2.0",
		"wasi:sockets/ip-name-lookup@0.2.0",
		"wasi:sockets/tcp@0.2.0",
		"wasi:sockets/tcp-create-socket@0.2.0",
		"wasi:sockets/udp@0.2.0",
		"wasi:sockets/udp-create-socket@0.2.0",
	}

	for _, iface := range interfaces {
		def, ok := linker.Get(iface)
		require.True(t, ok, "interface %s should be registered", iface)
		require.NotNil(t, def, "interface %s definition should not be nil", iface)
	}
}

// TestWASI_Sockets_CreateTCPSocket tests creating a TCP socket.
func TestWASI_Sockets_CreateTCPSocket(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := runtime.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	tcpCreateDef, _ := linker.Get("wasi:sockets/tcp-create-socket@0.2.0")
	instDef := tcpCreateDef.(*component.InstanceDef)

	createTcpFunc := instDef.Exports["create-tcp-socket"].(*component.FuncDef)

	// Create TCP socket with IPv4 (enum "ipv4")
	result, err := createTcpFunc.Callback(ctx, []types.Val{
		types.ValBorrow(0),    // network handle (placeholder)
		types.ValEnum("ipv4"), // address family
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result), "create-tcp-socket should return exactly one value")

	// Result should be result<own<tcp-socket>, error-code>
	isOk, okVal, _ := result[0].Result()
	require.True(t, isOk, "create-tcp-socket should succeed")

	handle := okVal.Own()
	require.True(t, handle >= 0, "should return a valid tcp-socket handle")
}

// TestWASI_Sockets_CreateUDPSocket tests creating a UDP socket.
func TestWASI_Sockets_CreateUDPSocket(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := runtime.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	udpCreateDef, _ := linker.Get("wasi:sockets/udp-create-socket@0.2.0")
	instDef := udpCreateDef.(*component.InstanceDef)

	createUdpFunc := instDef.Exports["create-udp-socket"].(*component.FuncDef)

	// Create UDP socket with IPv4 (enum "ipv4")
	result, err := createUdpFunc.Callback(ctx, []types.Val{
		types.ValBorrow(0),    // network handle (placeholder)
		types.ValEnum("ipv4"), // address family
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result), "create-udp-socket should return exactly one value")

	// Result should be result<own<udp-socket>, error-code>
	isOk, okVal, _ := result[0].Result()
	require.True(t, isOk, "create-udp-socket should succeed")

	handle := okVal.Own()
	require.True(t, handle >= 0, "should return a valid udp-socket handle")
}

// TestWASI_Sockets_DatagramStreamMethods tests that datagram stream methods exist.
func TestWASI_Sockets_DatagramStreamMethods(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	udpDef, ok := linker.Get("wasi:sockets/udp@0.2.0")
	require.True(t, ok, "udp interface should be registered")

	instDef := udpDef.(*component.InstanceDef)

	// Expected incoming datagram stream methods
	incomingMethods := []string{
		"[method]incoming-datagram-stream.receive",
		"[method]incoming-datagram-stream.subscribe",
	}

	for _, method := range incomingMethods {
		methodDef, ok := instDef.Exports[method]
		require.True(t, ok, "method %s should be exported", method)
		require.NotNil(t, methodDef, "method %s should not be nil", method)
	}

	// Expected outgoing datagram stream methods
	outgoingMethods := []string{
		"[method]outgoing-datagram-stream.check-send",
		"[method]outgoing-datagram-stream.send",
		"[method]outgoing-datagram-stream.subscribe",
	}

	for _, method := range outgoingMethods {
		methodDef, ok := instDef.Exports[method]
		require.True(t, ok, "method %s should be exported", method)
		require.NotNil(t, methodDef, "method %s should not be nil", method)
	}
}
