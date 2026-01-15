// imports/wasip2/sockets/sockets_test.go

package sockets

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestInstantiate(t *testing.T) {
	linker := component.NewLinker()
	err := Instantiate(linker)
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
		def, err := linker.MatchImport(iface)
		require.NoError(t, err, "interface %s should be registered", iface)
		_, ok := def.(*component.InstanceDef)
		require.True(t, ok, "expected InstanceDef for %s", iface)
	}
}

func TestInstantiate_Duplicate(t *testing.T) {
	linker := component.NewLinker()

	// First registration should succeed
	err := Instantiate(linker)
	require.NoError(t, err)

	// Second registration should fail
	err = Instantiate(linker)
	require.Error(t, err)
}

// ====================
// Network Interface Tests
// ====================

func TestInstantiateNetwork(t *testing.T) {
	linker := component.NewLinker()
	err := instantiateNetwork(linker)
	require.NoError(t, err)

	def, err := linker.MatchImport("wasi:sockets/network@0.2.0")
	require.NoError(t, err)

	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	// Verify network resource is defined
	_, hasNetwork := instDef.Exports["network"]
	require.True(t, hasNetwork, "network resource should be defined")
}

func TestInstantiateInstanceNetwork(t *testing.T) {
	linker := component.NewLinker()
	err := instantiateInstanceNetwork(linker)
	require.NoError(t, err)

	def, err := linker.MatchImport("wasi:sockets/instance-network@0.2.0")
	require.NoError(t, err)

	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	// Verify instance-network function is defined
	_, hasInstanceNetwork := instDef.Exports["instance-network"]
	require.True(t, hasInstanceNetwork, "instance-network function should be defined")
}

func TestInstanceNetwork(t *testing.T) {
	// Returns: own<network>
	result, err := instanceNetwork(context.Background(), []component.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindOwn, result[0].Kind())
}

// ====================
// IP Name Lookup Interface Tests
// ====================

func TestInstantiateIpNameLookup(t *testing.T) {
	linker := component.NewLinker()
	err := instantiateIpNameLookup(linker)
	require.NoError(t, err)

	def, err := linker.MatchImport("wasi:sockets/ip-name-lookup@0.2.0")
	require.NoError(t, err)

	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	// Verify resources and functions
	_, hasStream := instDef.Exports["resolve-address-stream"]
	require.True(t, hasStream, "resolve-address-stream resource should be defined")

	_, hasResolve := instDef.Exports["resolve-addresses"]
	require.True(t, hasResolve, "resolve-addresses function should be defined")

	_, hasNext := instDef.Exports["[method]resolve-address-stream.resolve-next-address"]
	require.True(t, hasNext, "resolve-next-address method should be defined")

	_, hasSubscribe := instDef.Exports["[method]resolve-address-stream.subscribe"]
	require.True(t, hasSubscribe, "subscribe method should be defined")
}

func TestResolveAddresses(t *testing.T) {
	// Args: network (borrow<network>), name (string)
	// Returns: result<own<resolve-address-stream>, error-code>
	network := component.ValBorrow(0)
	name := component.ValString("localhost")
	result, err := resolveAddresses(context.Background(), []component.Val{network, name})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindOwn, ok.Kind())
}

func TestResolveNextAddress(t *testing.T) {
	// Args: self (borrow<resolve-address-stream>)
	// Returns: result<option<ip-address>, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := resolveNextAddress(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindOption, ok.Kind())
}

func TestResolveAddressStreamSubscribe(t *testing.T) {
	// Args: self (borrow<resolve-address-stream>)
	// Returns: own<pollable>
	selfHandle := component.ValBorrow(0)
	result, err := resolveAddressStreamSubscribe(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindOwn, result[0].Kind())
}

// ====================
// TCP Interface Tests
// ====================

func TestInstantiateTcp(t *testing.T) {
	linker := component.NewLinker()
	err := instantiateTcp(linker)
	require.NoError(t, err)

	def, err := linker.MatchImport("wasi:sockets/tcp@0.2.0")
	require.NoError(t, err)

	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	// Verify tcp-socket resource is defined
	_, hasTcpSocket := instDef.Exports["tcp-socket"]
	require.True(t, hasTcpSocket, "tcp-socket resource should be defined")

	// Verify TCP socket methods
	tcpMethods := []string{
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

	for _, method := range tcpMethods {
		_, hasMethod := instDef.Exports[method]
		require.True(t, hasMethod, "%s should be defined", method)
	}
}

func TestInstantiateTcpCreateSocket(t *testing.T) {
	linker := component.NewLinker()
	err := instantiateTcpCreateSocket(linker)
	require.NoError(t, err)

	def, err := linker.MatchImport("wasi:sockets/tcp-create-socket@0.2.0")
	require.NoError(t, err)

	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	// Verify create-tcp-socket function is defined
	_, hasCreate := instDef.Exports["create-tcp-socket"]
	require.True(t, hasCreate, "create-tcp-socket function should be defined")
}

func TestCreateTcpSocket(t *testing.T) {
	// Args: network (borrow<network>), address-family (ip-address-family)
	// Returns: result<own<tcp-socket>, error-code>
	network := component.ValBorrow(0)
	family := component.ValEnum("ipv4")
	result, err := createTcpSocket(context.Background(), []component.Val{network, family})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindOwn, ok.Kind())
}

func TestTcpSocketStartBind(t *testing.T) {
	// Args: self (borrow<tcp-socket>), network (borrow<network>), local-address (ip-socket-address)
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	network := component.ValBorrow(1)
	localAddr := component.ValVariant("ipv4", &component.Val{})
	result, err := tcpSocketStartBind(context.Background(), []component.Val{selfHandle, network, localAddr})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestTcpSocketFinishBind(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := tcpSocketFinishBind(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestTcpSocketStartConnect(t *testing.T) {
	// Args: self (borrow<tcp-socket>), network (borrow<network>), remote-address (ip-socket-address)
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	network := component.ValBorrow(1)
	remoteAddr := component.ValVariant("ipv4", &component.Val{})
	result, err := tcpSocketStartConnect(context.Background(), []component.Val{selfHandle, network, remoteAddr})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestTcpSocketFinishConnect(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: result<tuple<own<input-stream>, own<output-stream>>, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := tcpSocketFinishConnect(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindTuple, ok.Kind())
}

func TestTcpSocketStartListen(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := tcpSocketStartListen(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestTcpSocketFinishListen(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := tcpSocketFinishListen(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestTcpSocketAccept(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: result<tuple<own<tcp-socket>, own<input-stream>, own<output-stream>>, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := tcpSocketAccept(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	// Accept returns placeholder result
	isOk, okVal, _ := result[0].Result()
	// Should return ok with tuple
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, okVal)
	require.Equal(t, component.ValKindTuple, okVal.Kind())
}

func TestTcpSocketLocalAddress(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: result<ip-socket-address, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := tcpSocketLocalAddress(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindVariant, ok.Kind())
}

func TestTcpSocketRemoteAddress(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: result<ip-socket-address, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := tcpSocketRemoteAddress(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindVariant, ok.Kind())
}

func TestTcpSocketIsListening(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: bool
	selfHandle := component.ValBorrow(0)
	result, err := tcpSocketIsListening(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindBool, result[0].Kind())
}

func TestTcpSocketAddressFamily(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: ip-address-family
	selfHandle := component.ValBorrow(0)
	result, err := tcpSocketAddressFamily(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindEnum, result[0].Kind())
}

func TestTcpSocketSetListenBacklogSize(t *testing.T) {
	// Args: self (borrow<tcp-socket>), value (u64)
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	size := component.ValU64(128)
	result, err := tcpSocketSetListenBacklogSize(context.Background(), []component.Val{selfHandle, size})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestTcpSocketKeepAliveEnabled(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: result<bool, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := tcpSocketKeepAliveEnabled(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindBool, ok.Kind())
}

func TestTcpSocketSetKeepAliveEnabled(t *testing.T) {
	// Args: self (borrow<tcp-socket>), value (bool)
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	enabled := component.ValBool(true)
	result, err := tcpSocketSetKeepAliveEnabled(context.Background(), []component.Val{selfHandle, enabled})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestTcpSocketKeepAliveIdleTime(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: result<duration, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := tcpSocketKeepAliveIdleTime(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindU64, ok.Kind())
}

func TestTcpSocketSetKeepAliveIdleTime(t *testing.T) {
	// Args: self (borrow<tcp-socket>), value (duration)
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	duration := component.ValU64(7200)
	result, err := tcpSocketSetKeepAliveIdleTime(context.Background(), []component.Val{selfHandle, duration})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestTcpSocketKeepAliveInterval(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: result<duration, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := tcpSocketKeepAliveInterval(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindU64, ok.Kind())
}

func TestTcpSocketSetKeepAliveInterval(t *testing.T) {
	// Args: self (borrow<tcp-socket>), value (duration)
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	duration := component.ValU64(75)
	result, err := tcpSocketSetKeepAliveInterval(context.Background(), []component.Val{selfHandle, duration})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestTcpSocketKeepAliveCount(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: result<u32, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := tcpSocketKeepAliveCount(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindU32, ok.Kind())
}

func TestTcpSocketSetKeepAliveCount(t *testing.T) {
	// Args: self (borrow<tcp-socket>), value (u32)
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	count := component.ValU32(9)
	result, err := tcpSocketSetKeepAliveCount(context.Background(), []component.Val{selfHandle, count})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestTcpSocketHopLimit(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: result<u8, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := tcpSocketHopLimit(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindU8, ok.Kind())
}

func TestTcpSocketSetHopLimit(t *testing.T) {
	// Args: self (borrow<tcp-socket>), value (u8)
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	limit := component.ValU8(64)
	result, err := tcpSocketSetHopLimit(context.Background(), []component.Val{selfHandle, limit})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestTcpSocketReceiveBufferSize(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: result<u64, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := tcpSocketReceiveBufferSize(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindU64, ok.Kind())
}

func TestTcpSocketSetReceiveBufferSize(t *testing.T) {
	// Args: self (borrow<tcp-socket>), value (u64)
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	size := component.ValU64(65536)
	result, err := tcpSocketSetReceiveBufferSize(context.Background(), []component.Val{selfHandle, size})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestTcpSocketSendBufferSize(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: result<u64, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := tcpSocketSendBufferSize(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindU64, ok.Kind())
}

func TestTcpSocketSetSendBufferSize(t *testing.T) {
	// Args: self (borrow<tcp-socket>), value (u64)
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	size := component.ValU64(65536)
	result, err := tcpSocketSetSendBufferSize(context.Background(), []component.Val{selfHandle, size})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestTcpSocketSubscribe(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: own<pollable>
	selfHandle := component.ValBorrow(0)
	result, err := tcpSocketSubscribe(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindOwn, result[0].Kind())
}

func TestTcpSocketShutdown(t *testing.T) {
	// Args: self (borrow<tcp-socket>), shutdown-type (shutdown-type)
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	shutdownType := component.ValEnum("both")
	result, err := tcpSocketShutdown(context.Background(), []component.Val{selfHandle, shutdownType})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

// ====================
// UDP Interface Tests
// ====================

func TestInstantiateUdp(t *testing.T) {
	linker := component.NewLinker()
	err := instantiateUdp(linker)
	require.NoError(t, err)

	def, err := linker.MatchImport("wasi:sockets/udp@0.2.0")
	require.NoError(t, err)

	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	// Verify udp-socket resource is defined
	_, hasUdpSocket := instDef.Exports["udp-socket"]
	require.True(t, hasUdpSocket, "udp-socket resource should be defined")

	// Verify incoming-datagram-stream resource
	_, hasIncoming := instDef.Exports["incoming-datagram-stream"]
	require.True(t, hasIncoming, "incoming-datagram-stream resource should be defined")

	// Verify outgoing-datagram-stream resource
	_, hasOutgoing := instDef.Exports["outgoing-datagram-stream"]
	require.True(t, hasOutgoing, "outgoing-datagram-stream resource should be defined")

	// Verify UDP socket methods
	udpMethods := []string{
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

	for _, method := range udpMethods {
		_, hasMethod := instDef.Exports[method]
		require.True(t, hasMethod, "%s should be defined", method)
	}

	// Verify datagram stream methods
	streamMethods := []string{
		"[method]incoming-datagram-stream.receive",
		"[method]incoming-datagram-stream.subscribe",
		"[method]outgoing-datagram-stream.check-send",
		"[method]outgoing-datagram-stream.send",
		"[method]outgoing-datagram-stream.subscribe",
	}

	for _, method := range streamMethods {
		_, hasMethod := instDef.Exports[method]
		require.True(t, hasMethod, "%s should be defined", method)
	}
}

func TestInstantiateUdpCreateSocket(t *testing.T) {
	linker := component.NewLinker()
	err := instantiateUdpCreateSocket(linker)
	require.NoError(t, err)

	def, err := linker.MatchImport("wasi:sockets/udp-create-socket@0.2.0")
	require.NoError(t, err)

	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	// Verify create-udp-socket function is defined
	_, hasCreate := instDef.Exports["create-udp-socket"]
	require.True(t, hasCreate, "create-udp-socket function should be defined")
}

func TestCreateUdpSocket(t *testing.T) {
	// Args: network (borrow<network>), address-family (ip-address-family)
	// Returns: result<own<udp-socket>, error-code>
	network := component.ValBorrow(0)
	family := component.ValEnum("ipv4")
	result, err := createUdpSocket(context.Background(), []component.Val{network, family})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindOwn, ok.Kind())
}

func TestUdpSocketStartBind(t *testing.T) {
	// Args: self (borrow<udp-socket>), network (borrow<network>), local-address (ip-socket-address)
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	network := component.ValBorrow(1)
	localAddr := component.ValVariant("ipv4", &component.Val{})
	result, err := udpSocketStartBind(context.Background(), []component.Val{selfHandle, network, localAddr})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestUdpSocketFinishBind(t *testing.T) {
	// Args: self (borrow<udp-socket>)
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := udpSocketFinishBind(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestUdpSocketStream(t *testing.T) {
	// Args: self (borrow<udp-socket>), remote-address (option<ip-socket-address>)
	// Returns: result<tuple<own<incoming-datagram-stream>, own<outgoing-datagram-stream>>, error-code>
	selfHandle := component.ValBorrow(0)
	remoteAddr := component.ValOption(nil)
	result, err := udpSocketStream(context.Background(), []component.Val{selfHandle, remoteAddr})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindTuple, ok.Kind())
}

func TestUdpSocketLocalAddress(t *testing.T) {
	// Args: self (borrow<udp-socket>)
	// Returns: result<ip-socket-address, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := udpSocketLocalAddress(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindVariant, ok.Kind())
}

func TestUdpSocketRemoteAddress(t *testing.T) {
	// Args: self (borrow<udp-socket>)
	// Returns: result<ip-socket-address, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := udpSocketRemoteAddress(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindVariant, ok.Kind())
}

func TestUdpSocketAddressFamily(t *testing.T) {
	// Args: self (borrow<udp-socket>)
	// Returns: ip-address-family
	selfHandle := component.ValBorrow(0)
	result, err := udpSocketAddressFamily(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindEnum, result[0].Kind())
}

func TestUdpSocketUnicastHopLimit(t *testing.T) {
	// Args: self (borrow<udp-socket>)
	// Returns: result<u8, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := udpSocketUnicastHopLimit(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindU8, ok.Kind())
}

func TestUdpSocketSetUnicastHopLimit(t *testing.T) {
	// Args: self (borrow<udp-socket>), value (u8)
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	limit := component.ValU8(64)
	result, err := udpSocketSetUnicastHopLimit(context.Background(), []component.Val{selfHandle, limit})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestUdpSocketReceiveBufferSize(t *testing.T) {
	// Args: self (borrow<udp-socket>)
	// Returns: result<u64, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := udpSocketReceiveBufferSize(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindU64, ok.Kind())
}

func TestUdpSocketSetReceiveBufferSize(t *testing.T) {
	// Args: self (borrow<udp-socket>), value (u64)
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	size := component.ValU64(65536)
	result, err := udpSocketSetReceiveBufferSize(context.Background(), []component.Val{selfHandle, size})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestUdpSocketSendBufferSize(t *testing.T) {
	// Args: self (borrow<udp-socket>)
	// Returns: result<u64, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := udpSocketSendBufferSize(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindU64, ok.Kind())
}

func TestUdpSocketSetSendBufferSize(t *testing.T) {
	// Args: self (borrow<udp-socket>), value (u64)
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	size := component.ValU64(65536)
	result, err := udpSocketSetSendBufferSize(context.Background(), []component.Val{selfHandle, size})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestUdpSocketSubscribe(t *testing.T) {
	// Args: self (borrow<udp-socket>)
	// Returns: own<pollable>
	selfHandle := component.ValBorrow(0)
	result, err := udpSocketSubscribe(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindOwn, result[0].Kind())
}

func TestIncomingDatagramStreamReceive(t *testing.T) {
	// Args: self (borrow<incoming-datagram-stream>), max-results (u64)
	// Returns: result<list<incoming-datagram>, error-code>
	selfHandle := component.ValBorrow(0)
	maxResults := component.ValU64(10)
	result, err := incomingDatagramStreamReceive(context.Background(), []component.Val{selfHandle, maxResults})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindList, ok.Kind())
}

func TestIncomingDatagramStreamSubscribe(t *testing.T) {
	// Args: self (borrow<incoming-datagram-stream>)
	// Returns: own<pollable>
	selfHandle := component.ValBorrow(0)
	result, err := incomingDatagramStreamSubscribe(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindOwn, result[0].Kind())
}

func TestOutgoingDatagramStreamCheckSend(t *testing.T) {
	// Args: self (borrow<outgoing-datagram-stream>)
	// Returns: result<u64, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := outgoingDatagramStreamCheckSend(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindU64, ok.Kind())
}

func TestOutgoingDatagramStreamSend(t *testing.T) {
	// Args: self (borrow<outgoing-datagram-stream>), datagrams (list<outgoing-datagram>)
	// Returns: result<u64, error-code>
	selfHandle := component.ValBorrow(0)
	datagrams := component.ValList([]component.Val{})
	result, err := outgoingDatagramStreamSend(context.Background(), []component.Val{selfHandle, datagrams})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindU64, ok.Kind())
}

func TestOutgoingDatagramStreamSubscribe(t *testing.T) {
	// Args: self (borrow<outgoing-datagram-stream>)
	// Returns: own<pollable>
	selfHandle := component.ValBorrow(0)
	result, err := outgoingDatagramStreamSubscribe(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindOwn, result[0].Kind())
}

// ====================
// Type Tests
// ====================

func TestIpAddressFamily(t *testing.T) {
	require.Equal(t, IpAddressFamily(0), IpAddressFamilyIpv4)
	require.Equal(t, IpAddressFamily(1), IpAddressFamilyIpv6)
}

func TestIpAddressFamilyString(t *testing.T) {
	require.Equal(t, "ipv4", IpAddressFamilyIpv4.String())
	require.Equal(t, "ipv6", IpAddressFamilyIpv6.String())
}

func TestIpAddress(t *testing.T) {
	ipv4 := NewIpv4Address([4]byte{127, 0, 0, 1})
	require.Equal(t, IpAddressFamilyIpv4, ipv4.Family())
	require.Equal(t, [4]byte{127, 0, 0, 1}, ipv4.Ipv4())

	ipv6 := NewIpv6Address([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1})
	require.Equal(t, IpAddressFamilyIpv6, ipv6.Family())
	require.Equal(t, [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, ipv6.Ipv6())
}

func TestIpSocketAddress(t *testing.T) {
	ipv4Addr := NewIpv4SocketAddress([4]byte{192, 168, 1, 1}, 8080)
	require.Equal(t, IpAddressFamilyIpv4, ipv4Addr.Family())
	require.Equal(t, uint16(8080), ipv4Addr.Port())
	require.Equal(t, [4]byte{192, 168, 1, 1}, ipv4Addr.Address().Ipv4())

	ipv6Addr := NewIpv6SocketAddress([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, 443)
	require.Equal(t, IpAddressFamilyIpv6, ipv6Addr.Family())
	require.Equal(t, uint16(443), ipv6Addr.Port())
}

func TestErrorCodeValues(t *testing.T) {
	require.Equal(t, ErrorCode("unknown"), ErrorCodeUnknown)
	require.Equal(t, ErrorCode("access-denied"), ErrorCodeAccessDenied)
	require.Equal(t, ErrorCode("address-in-use"), ErrorCodeAddressInUse)
	require.Equal(t, ErrorCode("connection-refused"), ErrorCodeConnectionRefused)
	require.Equal(t, ErrorCode("would-block"), ErrorCodeWouldBlock)
}

func TestShutdownType(t *testing.T) {
	require.Equal(t, ShutdownType(0), ShutdownTypeReceive)
	require.Equal(t, ShutdownType(1), ShutdownTypeSend)
	require.Equal(t, ShutdownType(2), ShutdownTypeBoth)
}

func TestShutdownTypeString(t *testing.T) {
	require.Equal(t, "receive", ShutdownTypeReceive.String())
	require.Equal(t, "send", ShutdownTypeSend.String())
	require.Equal(t, "both", ShutdownTypeBoth.String())
}

func TestTcpSocket(t *testing.T) {
	sock := NewTcpSocket(IpAddressFamilyIpv4)
	require.Equal(t, IpAddressFamilyIpv4, sock.Family())
	require.Equal(t, tcpStateUnbound, sock.State())
	require.False(t, sock.IsListening())
	require.Nil(t, sock.LocalAddress())
	require.Nil(t, sock.RemoteAddress())
}

func TestUdpSocket(t *testing.T) {
	sock := NewUdpSocket(IpAddressFamilyIpv6)
	require.Equal(t, IpAddressFamilyIpv6, sock.Family())
	require.Equal(t, udpStateUnbound, sock.State())
	require.Nil(t, sock.LocalAddress())
	require.Nil(t, sock.RemoteAddress())
}

func TestNetwork(t *testing.T) {
	net := NewNetwork()
	require.NotNil(t, net)
}

func TestResolveAddressStream(t *testing.T) {
	addresses := []IpAddress{
		NewIpv4Address([4]byte{127, 0, 0, 1}),
		NewIpv4Address([4]byte{192, 168, 1, 1}),
	}
	stream := NewResolveAddressStream(addresses)

	addr1, ok := stream.ResolveNextAddress()
	require.True(t, ok)
	require.Equal(t, [4]byte{127, 0, 0, 1}, addr1.Ipv4())

	addr2, ok := stream.ResolveNextAddress()
	require.True(t, ok)
	require.Equal(t, [4]byte{192, 168, 1, 1}, addr2.Ipv4())

	_, ok = stream.ResolveNextAddress()
	require.False(t, ok)
}

func TestDatagramStreams(t *testing.T) {
	sock := NewUdpSocket(IpAddressFamilyIpv4)

	inStream := NewIncomingDatagramStream(sock)
	require.NotNil(t, inStream)

	outStream := NewOutgoingDatagramStream(sock)
	require.NotNil(t, outStream)
}
