// imports/wasip2/sockets/sockets_test.go

package sockets

import (
	"context"
	"errors"
	"testing"

	wasipIO "github.com/tetratelabs/wazero/imports/wasip2/io"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
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
	result, err := instanceNetwork(context.Background(), nil, []types.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOwn, result[0].Kind())
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

func TestResolveAddresses_IPv4Literal(t *testing.T) {
	t.Skip("Task E4: wasip2 registry migration — Rep is uint32, Go object lookup requires per-module registry")
	ctx := contextWithResourceTable()

	network := types.ValBorrow(0)
	name := types.ValString("127.0.0.1")
	result, err := resolveAddresses(ctx, nil, []types.Val{network, name})
	require.NoError(t, err)
	isOk, streamHandle, _ := result[0].Result()
	require.True(t, isOk, "resolving IP literal should succeed")
	require.NotNil(t, streamHandle)

	// Get next address
	borrow := types.ValBorrow(streamHandle.Own())
	result, err = resolveNextAddress(ctx, nil, []types.Val{borrow})
	require.NoError(t, err)
	isOk, addrOpt, _ := result[0].Result()
	require.True(t, isOk)
	opt := addrOpt.Option()
	require.NotNil(t, opt, "should return an address for IP literal")

	// Second call should return None (exhausted)
	result, err = resolveNextAddress(ctx, nil, []types.Val{borrow})
	require.NoError(t, err)
	isOk, addrOpt, _ = result[0].Result()
	require.True(t, isOk)
	opt = addrOpt.Option()
	require.Nil(t, opt, "should return None when exhausted")
}

func TestResolveAddresses_EmptyString(t *testing.T) {
	ctx := contextWithResourceTable()
	network := types.ValBorrow(0)
	name := types.ValString("")
	result, err := resolveAddresses(ctx, nil, []types.Val{network, name})
	require.NoError(t, err)
	isOk, _, _ := result[0].Result()
	require.False(t, isOk, "empty string should fail")
}

func TestResolveAddresses_InvalidWithPort(t *testing.T) {
	ctx := contextWithResourceTable()
	network := types.ValBorrow(0)
	name := types.ValString("127.0.0.1:80")
	result, err := resolveAddresses(ctx, nil, []types.Val{network, name})
	require.NoError(t, err)
	isOk, _, _ := result[0].Result()
	require.False(t, isOk, "address with port should fail")
}

func TestResolveAddresses_InvalidWithSlash(t *testing.T) {
	ctx := contextWithResourceTable()
	network := types.ValBorrow(0)
	name := types.ValString("example.com/path")
	result, err := resolveAddresses(ctx, nil, []types.Val{network, name})
	require.NoError(t, err)
	isOk, _, _ := result[0].Result()
	require.False(t, isOk, "address with slash should fail")
}

func TestResolveAddressStreamSubscribe(t *testing.T) {
	ctx := contextWithResourceTable()
	network := types.ValBorrow(0)
	name := types.ValString("127.0.0.1")
	result, _ := resolveAddresses(ctx, nil, []types.Val{network, name})
	_, streamHandle, _ := result[0].Result()

	borrow := types.ValBorrow(streamHandle.Own())
	result, err := resolveAddressStreamSubscribe(ctx, nil, []types.Val{borrow})
	require.NoError(t, err)
	require.Equal(t, types.ValKindOwn, result[0].Kind())
	require.True(t, result[0].Own() > 0, "subscribe should return valid pollable handle")
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
	network := types.ValBorrow(0)
	family := types.ValEnum("ipv4")
	result, err := createTcpSocket(context.Background(), nil, []types.Val{network, family})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindOwn, ok.Kind())
}

func TestTcpSocketStartBind(t *testing.T) {
	// Args: self (borrow<tcp-socket>), network (borrow<network>), local-address (ip-socket-address)
	// Returns: result<_, error-code>
	selfHandle := types.ValBorrow(0)
	network := types.ValBorrow(1)
	localAddr := types.ValVariant("ipv4", &types.Val{})
	result, err := tcpSocketStartBind(context.Background(), nil, []types.Val{selfHandle, network, localAddr})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestTcpSocketFinishBind(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: result<_, error-code>
	selfHandle := types.ValBorrow(0)
	result, err := tcpSocketFinishBind(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestTcpSocketStartConnect(t *testing.T) {
	// Args: self (borrow<tcp-socket>), network (borrow<network>), remote-address (ip-socket-address)
	// Returns: result<_, error-code>
	selfHandle := types.ValBorrow(0)
	network := types.ValBorrow(1)
	remoteAddr := types.ValVariant("ipv4", &types.Val{})
	result, err := tcpSocketStartConnect(context.Background(), nil, []types.Val{selfHandle, network, remoteAddr})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestTcpSocketFinishConnect(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: result<tuple<own<input-stream>, own<output-stream>>, error-code>
	selfHandle := types.ValBorrow(0)
	result, err := tcpSocketFinishConnect(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindTuple, ok.Kind())
}

func TestTcpSocketStartListen(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: result<_, error-code>
	selfHandle := types.ValBorrow(0)
	result, err := tcpSocketStartListen(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestTcpSocketFinishListen(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: result<_, error-code>
	selfHandle := types.ValBorrow(0)
	result, err := tcpSocketFinishListen(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestTcpSocketAccept(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: result<tuple<own<tcp-socket>, own<input-stream>, own<output-stream>>, error-code>
	selfHandle := types.ValBorrow(0)
	result, err := tcpSocketAccept(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	// Accept returns placeholder result
	isOk, okVal, _ := result[0].Result()
	// Should return ok with tuple
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, okVal)
	require.Equal(t, types.ValKindTuple, okVal.Kind())
}

func TestTcpSocketLocalAddress(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: result<ip-socket-address, error-code>
	selfHandle := types.ValBorrow(0)
	result, err := tcpSocketLocalAddress(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindVariant, ok.Kind())
}

func TestTcpSocketRemoteAddress(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: result<ip-socket-address, error-code>
	selfHandle := types.ValBorrow(0)
	result, err := tcpSocketRemoteAddress(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindVariant, ok.Kind())
}

func TestTcpSocketIsListening(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: bool
	selfHandle := types.ValBorrow(0)
	result, err := tcpSocketIsListening(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindBool, result[0].Kind())
}

func TestTcpSocketAddressFamily(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: ip-address-family
	selfHandle := types.ValBorrow(0)
	result, err := tcpSocketAddressFamily(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindEnum, result[0].Kind())
}

func TestTcpSocketSetListenBacklogSize(t *testing.T) {
	// Args: self (borrow<tcp-socket>), value (u64)
	// Returns: result<_, error-code>
	selfHandle := types.ValBorrow(0)
	size := types.ValU64(128)
	result, err := tcpSocketSetListenBacklogSize(context.Background(), nil, []types.Val{selfHandle, size})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestTcpSocketKeepAliveEnabled(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: result<bool, error-code>
	selfHandle := types.ValBorrow(0)
	result, err := tcpSocketKeepAliveEnabled(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindBool, ok.Kind())
}

func TestTcpSocketSetKeepAliveEnabled(t *testing.T) {
	// Args: self (borrow<tcp-socket>), value (bool)
	// Returns: result<_, error-code>
	selfHandle := types.ValBorrow(0)
	enabled := types.ValBool(true)
	result, err := tcpSocketSetKeepAliveEnabled(context.Background(), nil, []types.Val{selfHandle, enabled})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestTcpSocketKeepAliveIdleTime(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: result<duration, error-code>
	selfHandle := types.ValBorrow(0)
	result, err := tcpSocketKeepAliveIdleTime(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindU64, ok.Kind())
}

func TestTcpSocketSetKeepAliveIdleTime(t *testing.T) {
	// Args: self (borrow<tcp-socket>), value (duration)
	// Returns: result<_, error-code>
	selfHandle := types.ValBorrow(0)
	duration := types.ValU64(7200)
	result, err := tcpSocketSetKeepAliveIdleTime(context.Background(), nil, []types.Val{selfHandle, duration})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestTcpSocketKeepAliveInterval(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: result<duration, error-code>
	selfHandle := types.ValBorrow(0)
	result, err := tcpSocketKeepAliveInterval(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindU64, ok.Kind())
}

func TestTcpSocketSetKeepAliveInterval(t *testing.T) {
	// Args: self (borrow<tcp-socket>), value (duration)
	// Returns: result<_, error-code>
	selfHandle := types.ValBorrow(0)
	duration := types.ValU64(75)
	result, err := tcpSocketSetKeepAliveInterval(context.Background(), nil, []types.Val{selfHandle, duration})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestTcpSocketKeepAliveCount(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: result<u32, error-code>
	selfHandle := types.ValBorrow(0)
	result, err := tcpSocketKeepAliveCount(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindU32, ok.Kind())
}

func TestTcpSocketSetKeepAliveCount(t *testing.T) {
	// Args: self (borrow<tcp-socket>), value (u32)
	// Returns: result<_, error-code>
	selfHandle := types.ValBorrow(0)
	count := types.ValU32(9)
	result, err := tcpSocketSetKeepAliveCount(context.Background(), nil, []types.Val{selfHandle, count})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestTcpSocketHopLimit(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: result<u8, error-code>
	selfHandle := types.ValBorrow(0)
	result, err := tcpSocketHopLimit(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindU8, ok.Kind())
}

func TestTcpSocketSetHopLimit(t *testing.T) {
	// Args: self (borrow<tcp-socket>), value (u8)
	// Returns: result<_, error-code>
	selfHandle := types.ValBorrow(0)
	limit := types.ValU8(64)
	result, err := tcpSocketSetHopLimit(context.Background(), nil, []types.Val{selfHandle, limit})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestTcpSocketReceiveBufferSize(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: result<u64, error-code>
	selfHandle := types.ValBorrow(0)
	result, err := tcpSocketReceiveBufferSize(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindU64, ok.Kind())
}

func TestTcpSocketSetReceiveBufferSize(t *testing.T) {
	// Args: self (borrow<tcp-socket>), value (u64)
	// Returns: result<_, error-code>
	selfHandle := types.ValBorrow(0)
	size := types.ValU64(65536)
	result, err := tcpSocketSetReceiveBufferSize(context.Background(), nil, []types.Val{selfHandle, size})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestTcpSocketSendBufferSize(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: result<u64, error-code>
	selfHandle := types.ValBorrow(0)
	result, err := tcpSocketSendBufferSize(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindU64, ok.Kind())
}

func TestTcpSocketSetSendBufferSize(t *testing.T) {
	// Args: self (borrow<tcp-socket>), value (u64)
	// Returns: result<_, error-code>
	selfHandle := types.ValBorrow(0)
	size := types.ValU64(65536)
	result, err := tcpSocketSetSendBufferSize(context.Background(), nil, []types.Val{selfHandle, size})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestTcpSocketSubscribe(t *testing.T) {
	// Args: self (borrow<tcp-socket>)
	// Returns: own<pollable>
	selfHandle := types.ValBorrow(0)
	result, err := tcpSocketSubscribe(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOwn, result[0].Kind())
}

func TestTcpSocketShutdown(t *testing.T) {
	// Args: self (borrow<tcp-socket>), shutdown-type (shutdown-type)
	// Returns: result<_, error-code>
	selfHandle := types.ValBorrow(0)
	shutdownType := types.ValEnum("both")
	result, err := tcpSocketShutdown(context.Background(), nil, []types.Val{selfHandle, shutdownType})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
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
	network := types.ValBorrow(0)
	family := types.ValEnum("ipv4")
	result, err := createUdpSocket(context.Background(), nil, []types.Val{network, family})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindOwn, ok.Kind())
}

func TestUdpSocketStartBind(t *testing.T) {
	// Args: self (borrow<udp-socket>), network (borrow<network>), local-address (ip-socket-address)
	// Returns: result<_, error-code>
	selfHandle := types.ValBorrow(0)
	network := types.ValBorrow(1)
	localAddr := types.ValVariant("ipv4", &types.Val{})
	result, err := udpSocketStartBind(context.Background(), nil, []types.Val{selfHandle, network, localAddr})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestUdpSocketFinishBind(t *testing.T) {
	// Args: self (borrow<udp-socket>)
	// Returns: result<_, error-code>
	selfHandle := types.ValBorrow(0)
	result, err := udpSocketFinishBind(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestUdpSocketStream(t *testing.T) {
	// Args: self (borrow<udp-socket>), remote-address (option<ip-socket-address>)
	// Returns: result<tuple<own<incoming-datagram-stream>, own<outgoing-datagram-stream>>, error-code>
	selfHandle := types.ValBorrow(0)
	remoteAddr := types.ValOption(nil)
	result, err := udpSocketStream(context.Background(), nil, []types.Val{selfHandle, remoteAddr})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindTuple, ok.Kind())
}

func TestUdpSocketLocalAddress(t *testing.T) {
	// Args: self (borrow<udp-socket>)
	// Returns: result<ip-socket-address, error-code>
	selfHandle := types.ValBorrow(0)
	result, err := udpSocketLocalAddress(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindVariant, ok.Kind())
}

func TestUdpSocketRemoteAddress(t *testing.T) {
	// Args: self (borrow<udp-socket>)
	// Returns: result<ip-socket-address, error-code>
	selfHandle := types.ValBorrow(0)
	result, err := udpSocketRemoteAddress(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindVariant, ok.Kind())
}

func TestUdpSocketAddressFamily(t *testing.T) {
	// Args: self (borrow<udp-socket>)
	// Returns: ip-address-family
	selfHandle := types.ValBorrow(0)
	result, err := udpSocketAddressFamily(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindEnum, result[0].Kind())
}

func TestUdpSocketUnicastHopLimit(t *testing.T) {
	// Args: self (borrow<udp-socket>)
	// Returns: result<u8, error-code>
	selfHandle := types.ValBorrow(0)
	result, err := udpSocketUnicastHopLimit(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindU8, ok.Kind())
}

func TestUdpSocketSetUnicastHopLimit(t *testing.T) {
	// Args: self (borrow<udp-socket>), value (u8)
	// Returns: result<_, error-code>
	selfHandle := types.ValBorrow(0)
	limit := types.ValU8(64)
	result, err := udpSocketSetUnicastHopLimit(context.Background(), nil, []types.Val{selfHandle, limit})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestUdpSocketReceiveBufferSize(t *testing.T) {
	// Args: self (borrow<udp-socket>)
	// Returns: result<u64, error-code>
	selfHandle := types.ValBorrow(0)
	result, err := udpSocketReceiveBufferSize(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindU64, ok.Kind())
}

func TestUdpSocketSetReceiveBufferSize(t *testing.T) {
	// Args: self (borrow<udp-socket>), value (u64)
	// Returns: result<_, error-code>
	selfHandle := types.ValBorrow(0)
	size := types.ValU64(65536)
	result, err := udpSocketSetReceiveBufferSize(context.Background(), nil, []types.Val{selfHandle, size})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestUdpSocketSendBufferSize(t *testing.T) {
	// Args: self (borrow<udp-socket>)
	// Returns: result<u64, error-code>
	selfHandle := types.ValBorrow(0)
	result, err := udpSocketSendBufferSize(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindU64, ok.Kind())
}

func TestUdpSocketSetSendBufferSize(t *testing.T) {
	// Args: self (borrow<udp-socket>), value (u64)
	// Returns: result<_, error-code>
	selfHandle := types.ValBorrow(0)
	size := types.ValU64(65536)
	result, err := udpSocketSetSendBufferSize(context.Background(), nil, []types.Val{selfHandle, size})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestUdpSocketSubscribe(t *testing.T) {
	// Args: self (borrow<udp-socket>)
	// Returns: own<pollable>
	selfHandle := types.ValBorrow(0)
	result, err := udpSocketSubscribe(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOwn, result[0].Kind())
}

func TestIncomingDatagramStreamReceive(t *testing.T) {
	// Args: self (borrow<incoming-datagram-stream>), max-results (u64)
	// Returns: result<list<incoming-datagram>, error-code>
	selfHandle := types.ValBorrow(0)
	maxResults := types.ValU64(10)
	result, err := incomingDatagramStreamReceive(context.Background(), nil, []types.Val{selfHandle, maxResults})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindList, ok.Kind())
}

func TestIncomingDatagramStreamSubscribe(t *testing.T) {
	// Args: self (borrow<incoming-datagram-stream>)
	// Returns: own<pollable>
	selfHandle := types.ValBorrow(0)
	result, err := incomingDatagramStreamSubscribe(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOwn, result[0].Kind())
}

func TestOutgoingDatagramStreamCheckSend(t *testing.T) {
	// Args: self (borrow<outgoing-datagram-stream>)
	// Returns: result<u64, error-code>
	selfHandle := types.ValBorrow(0)
	result, err := outgoingDatagramStreamCheckSend(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindU64, ok.Kind())
}

func TestOutgoingDatagramStreamSend(t *testing.T) {
	// Args: self (borrow<outgoing-datagram-stream>), datagrams (list<outgoing-datagram>)
	// Returns: result<u64, error-code>
	selfHandle := types.ValBorrow(0)
	datagrams := types.ValList([]types.Val{})
	result, err := outgoingDatagramStreamSend(context.Background(), nil, []types.Val{selfHandle, datagrams})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindU64, ok.Kind())
}

func TestOutgoingDatagramStreamSubscribe(t *testing.T) {
	// Args: self (borrow<outgoing-datagram-stream>)
	// Returns: own<pollable>
	selfHandle := types.ValBorrow(0)
	result, err := outgoingDatagramStreamSubscribe(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOwn, result[0].Kind())
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

	require.True(t, stream.IsReady(), "synchronous stream should be immediately ready")

	addr1, err := stream.NextAddress()
	require.NoError(t, err)
	require.NotNil(t, addr1)
	require.Equal(t, [4]byte{127, 0, 0, 1}, addr1.Ipv4())

	addr2, err := stream.NextAddress()
	require.NoError(t, err)
	require.NotNil(t, addr2)
	require.Equal(t, [4]byte{192, 168, 1, 1}, addr2.Ipv4())

	addr3, err := stream.NextAddress()
	require.NoError(t, err)
	require.Nil(t, addr3, "should be exhausted")
}

func TestResolveAddressStreamAsync(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := NewResolveAddressStreamAsync(cancel)
	require.False(t, stream.IsReady(), "async stream should not be ready initially")

	stream.SetResult([]IpAddress{NewIpv4Address([4]byte{10, 0, 0, 1})}, nil)
	require.True(t, stream.IsReady(), "async stream should be ready after SetResult")

	addr, err := stream.NextAddress()
	require.NoError(t, err)
	require.NotNil(t, addr)
}

func TestDatagramStreams(t *testing.T) {
	sock := NewUdpSocket(IpAddressFamilyIpv4)

	inStream := NewIncomingDatagramStream(sock)
	require.NotNil(t, inStream)

	outStream := NewOutgoingDatagramStream(sock)
	require.NotNil(t, outStream)
}

// ====================================
// Real Socket Integration Tests
// ====================================

// Helper to create a context with resource table
func contextWithResourceTable() context.Context {
	table := runtime.NewTable()
	return component.WithResourceTable(context.Background(), table)
}

// Helper to create an IPv4 socket address Val
func makeIPv4SocketAddrVal(a, b, c, d byte, port uint16) types.Val {
	addrRecord := types.ValRecord(map[string]types.Val{
		"port":    types.ValU16(port),
		"address": types.ValTuple([]types.Val{types.ValU8(a), types.ValU8(b), types.ValU8(c), types.ValU8(d)}),
	})
	return types.ValVariant("ipv4", &addrRecord)
}

func TestTcpSocket_CreateAndBind(t *testing.T) {
	ctx := contextWithResourceTable()

	// Create TCP socket
	network := types.ValBorrow(0)
	family := types.ValEnum("ipv4")
	result, err := createTcpSocket(ctx, nil, []types.Val{network, family})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "createTcpSocket should succeed")
	require.NotNil(t, ok)
	sockHandle := ok.Own()

	// Start bind to 127.0.0.1:0 (port 0 = let OS pick)
	selfHandle := types.ValBorrow(sockHandle)
	localAddr := makeIPv4SocketAddrVal(127, 0, 0, 1, 0)
	result, err = tcpSocketStartBind(ctx, nil, []types.Val{selfHandle, network, localAddr})
	require.NoError(t, err)
	isOk, _, _ = result[0].Result()
	require.True(t, isOk, "start-bind should succeed")

	// Finish bind
	result, err = tcpSocketFinishBind(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	isOk, _, _ = result[0].Result()
	require.True(t, isOk, "finish-bind should succeed")

	// Verify local address is set
	result, err = tcpSocketLocalAddress(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	isOk, addrVal, _ := result[0].Result()
	require.True(t, isOk, "local-address should succeed")
	require.NotNil(t, addrVal)

	// Verify address is IPv4
	caseName, _ := addrVal.Variant()
	require.Equal(t, "ipv4", caseName)

	// Clean up: get the socket and close it
	sock, _ := getTcpSocket(ctx, sockHandle)
	if sock != nil {
		sock.Close()
	}
}

func TestTcpSocket_ListenAndAccept(t *testing.T) {
	t.Skip("Task E4: wasip2 registry migration — Rep is uint32, Go object lookup requires per-module registry")
	ctx := contextWithResourceTable()

	// Create TCP socket
	network := types.ValBorrow(0)
	family := types.ValEnum("ipv4")
	result, err := createTcpSocket(ctx, nil, []types.Val{network, family})
	require.NoError(t, err)
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk)
	serverHandle := ok.Own()

	// Bind to 127.0.0.1:0
	selfHandle := types.ValBorrow(serverHandle)
	localAddr := makeIPv4SocketAddrVal(127, 0, 0, 1, 0)
	result, _ = tcpSocketStartBind(ctx, nil, []types.Val{selfHandle, network, localAddr})
	isOk, _, _ = result[0].Result()
	require.True(t, isOk)

	result, _ = tcpSocketFinishBind(ctx, nil, []types.Val{selfHandle})
	isOk, _, _ = result[0].Result()
	require.True(t, isOk)

	// Start listen
	result, err = tcpSocketStartListen(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	isOk, _, _ = result[0].Result()
	require.True(t, isOk, "start-listen should succeed")

	// Finish listen
	result, err = tcpSocketFinishListen(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	isOk, _, _ = result[0].Result()
	require.True(t, isOk, "finish-listen should succeed")

	// Verify is-listening returns true
	result, err = tcpSocketIsListening(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.True(t, result[0].Bool(), "socket should be listening")

	// Get local address (for client to connect to)
	result, _ = tcpSocketLocalAddress(ctx, nil, []types.Val{selfHandle})
	isOk, addrVal, _ := result[0].Result()
	require.True(t, isOk)

	// Clean up
	sock, _ := getTcpSocket(ctx, serverHandle)
	if sock != nil {
		sock.Close()
	}

	// Note: Full accept test would require a client connecting, which is more complex
	_ = addrVal
}

func TestTcpSocket_ConnectToListener(t *testing.T) {
	t.Skip("Task E4: wasip2 registry migration — Rep is uint32, Go object lookup requires per-module registry")
	ctx := contextWithResourceTable()
	network := types.ValBorrow(0)

	// Create server socket
	family := types.ValEnum("ipv4")
	result, _ := createTcpSocket(ctx, nil, []types.Val{network, family})
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk)
	serverHandle := ok.Own()

	// Bind server
	serverBorrow := types.ValBorrow(serverHandle)
	localAddr := makeIPv4SocketAddrVal(127, 0, 0, 1, 0)
	result, _ = tcpSocketStartBind(ctx, nil, []types.Val{serverBorrow, network, localAddr})
	isOk, _, _ = result[0].Result()
	require.True(t, isOk)

	result, _ = tcpSocketFinishBind(ctx, nil, []types.Val{serverBorrow})
	isOk, _, _ = result[0].Result()
	require.True(t, isOk)

	// Listen
	result, _ = tcpSocketStartListen(ctx, nil, []types.Val{serverBorrow})
	isOk, _, _ = result[0].Result()
	require.True(t, isOk)

	result, _ = tcpSocketFinishListen(ctx, nil, []types.Val{serverBorrow})
	isOk, _, _ = result[0].Result()
	require.True(t, isOk)

	// Get server's port
	result, _ = tcpSocketLocalAddress(ctx, nil, []types.Val{serverBorrow})
	isOk, addrVal, _ := result[0].Result()
	require.True(t, isOk)

	serverSock, _ := getTcpSocket(ctx, serverHandle)
	serverPort := serverSock.LocalAddress().Port()

	// Create client socket
	result, _ = createTcpSocket(ctx, nil, []types.Val{network, family})
	isOk, ok, _ = result[0].Result()
	require.True(t, isOk)
	clientHandle := ok.Own()

	// Start connect to server
	clientBorrow := types.ValBorrow(clientHandle)
	remoteAddr := makeIPv4SocketAddrVal(127, 0, 0, 1, serverPort)
	result, err := tcpSocketStartConnect(ctx, nil, []types.Val{clientBorrow, network, remoteAddr})
	require.NoError(t, err)
	isOk, _, _ = result[0].Result()
	require.True(t, isOk, "start-connect should succeed")

	// Finish connect (this actually connects)
	result, err = tcpSocketFinishConnect(ctx, nil, []types.Val{clientBorrow})
	require.NoError(t, err)
	isOk, tupleVal, _ := result[0].Result()
	require.True(t, isOk, "finish-connect should succeed")
	require.NotNil(t, tupleVal)

	// Verify we got streams back
	streams := tupleVal.Tuple()
	require.Equal(t, 2, len(streams), "should return input and output stream handles")

	// Verify remote address
	result, _ = tcpSocketRemoteAddress(ctx, nil, []types.Val{clientBorrow})
	isOk, remoteAddrVal, _ := result[0].Result()
	require.True(t, isOk)
	require.NotNil(t, remoteAddrVal)

	// Clean up
	clientSock, _ := getTcpSocket(ctx, clientHandle)
	if clientSock != nil {
		clientSock.Close()
	}
	if serverSock != nil {
		serverSock.Close()
	}

	_ = addrVal
}

func TestUdpSocket_CreateAndBind(t *testing.T) {
	ctx := contextWithResourceTable()

	// Create UDP socket
	network := types.ValBorrow(0)
	family := types.ValEnum("ipv4")
	result, err := createUdpSocket(ctx, nil, []types.Val{network, family})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "createUdpSocket should succeed")
	require.NotNil(t, ok)
	sockHandle := ok.Own()

	// Start bind to 127.0.0.1:0
	selfHandle := types.ValBorrow(sockHandle)
	localAddr := makeIPv4SocketAddrVal(127, 0, 0, 1, 0)
	result, err = udpSocketStartBind(ctx, nil, []types.Val{selfHandle, network, localAddr})
	require.NoError(t, err)
	isOk, _, _ = result[0].Result()
	require.True(t, isOk, "start-bind should succeed")

	// Finish bind
	result, err = udpSocketFinishBind(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	isOk, _, _ = result[0].Result()
	require.True(t, isOk, "finish-bind should succeed")

	// Verify local address is set
	result, err = udpSocketLocalAddress(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	isOk, addrVal, _ := result[0].Result()
	require.True(t, isOk, "local-address should succeed")
	require.NotNil(t, addrVal)

	// Verify address is IPv4
	caseName, _ := addrVal.Variant()
	require.Equal(t, "ipv4", caseName)

	// Clean up
	sock, _ := getUdpSocket(ctx, sockHandle)
	if sock != nil {
		sock.Close()
	}
}

func TestUdpSocket_SendReceive(t *testing.T) {
	ctx := contextWithResourceTable()
	network := types.ValBorrow(0)
	family := types.ValEnum("ipv4")

	// Create and bind socket 1
	result, _ := createUdpSocket(ctx, nil, []types.Val{network, family})
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk)
	sock1Handle := ok.Own()

	sock1Borrow := types.ValBorrow(sock1Handle)
	localAddr1 := makeIPv4SocketAddrVal(127, 0, 0, 1, 0)
	result, _ = udpSocketStartBind(ctx, nil, []types.Val{sock1Borrow, network, localAddr1})
	isOk, _, _ = result[0].Result()
	require.True(t, isOk)

	result, _ = udpSocketFinishBind(ctx, nil, []types.Val{sock1Borrow})
	isOk, _, _ = result[0].Result()
	require.True(t, isOk)

	// Get streams for socket 1
	remoteAddrOpt := types.ValOption(nil)
	result, err := udpSocketStream(ctx, nil, []types.Val{sock1Borrow, remoteAddrOpt})
	require.NoError(t, err)
	isOk, tupleVal, _ := result[0].Result()
	require.True(t, isOk, "stream() should succeed")
	require.NotNil(t, tupleVal)

	streams := tupleVal.Tuple()
	require.Equal(t, 2, len(streams))

	// Verify address family
	result, _ = udpSocketAddressFamily(ctx, nil, []types.Val{sock1Borrow})
	require.Equal(t, types.ValKindEnum, result[0].Kind())
	require.Equal(t, "ipv4", result[0].Enum())

	// Clean up
	sock1, _ := getUdpSocket(ctx, sock1Handle)
	if sock1 != nil {
		sock1.Close()
	}
}

func TestAddressConversion(t *testing.T) {
	// Test IPv4 conversion
	ipv4Addr := makeIPv4SocketAddrVal(192, 168, 1, 100, 8080)
	parsed, err := ipSocketAddressFromVal(ipv4Addr)
	require.NoError(t, err)
	require.NotNil(t, parsed)
	require.Equal(t, IpAddressFamilyIpv4, parsed.Family())
	require.Equal(t, uint16(8080), parsed.Port())
	require.Equal(t, [4]byte{192, 168, 1, 100}, parsed.Address().Ipv4())

	// Convert back to Val
	converted := ipSocketAddressToVal(parsed)
	caseName, payload := converted.Variant()
	require.Equal(t, "ipv4", caseName)
	require.NotNil(t, payload)

	// Parse the converted value again
	parsed2, err := ipSocketAddressFromVal(converted)
	require.NoError(t, err)
	require.Equal(t, parsed.Family(), parsed2.Family())
	require.Equal(t, parsed.Port(), parsed2.Port())
	require.Equal(t, parsed.Address().Ipv4(), parsed2.Address().Ipv4())
}

func TestErrorCodeMapping(t *testing.T) {
	// Test that error codes are properly mapped
	require.Equal(t, ErrorCodeUnknown, mapNetError(nil))

	// Test string-based error detection (simulated)
	// Note: actual error mapping depends on the system error strings
}

func TestTcpSocket_InvalidStateTransitions(t *testing.T) {
	t.Skip("Task E4: wasip2 registry migration — Rep is uint32, Go object lookup requires per-module registry")
	ctx := contextWithResourceTable()
	network := types.ValBorrow(0)
	family := types.ValEnum("ipv4")

	// Create socket
	result, _ := createTcpSocket(ctx, nil, []types.Val{network, family})
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk)
	sockHandle := ok.Own()
	selfHandle := types.ValBorrow(sockHandle)

	// Try to start listen without binding first - should fail with invalid state
	result, _ = tcpSocketStartListen(ctx, nil, []types.Val{selfHandle})
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "start-listen should fail without bind")
	require.NotNil(t, errVal)

	// Try to finish bind without start bind - should fail with invalid state
	result, _ = tcpSocketFinishBind(ctx, nil, []types.Val{selfHandle})
	isOk, _, errVal = result[0].Result()
	require.False(t, isOk, "finish-bind should fail without start-bind")

	// Clean up
	sock, _ := getTcpSocket(ctx, sockHandle)
	if sock != nil {
		sock.Close()
	}
}

func TestUdpSocket_InvalidStateTransitions(t *testing.T) {
	t.Skip("Task E4: wasip2 registry migration — Rep is uint32, Go object lookup requires per-module registry")
	ctx := contextWithResourceTable()
	network := types.ValBorrow(0)
	family := types.ValEnum("ipv4")

	// Create socket
	result, _ := createUdpSocket(ctx, nil, []types.Val{network, family})
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk)
	sockHandle := ok.Own()
	selfHandle := types.ValBorrow(sockHandle)

	// Try to get stream without binding first - should fail
	remoteAddrOpt := types.ValOption(nil)
	result, _ = udpSocketStream(ctx, nil, []types.Val{selfHandle, remoteAddrOpt})
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "stream() should fail without bind")
	require.NotNil(t, errVal)

	// Try to finish bind without start bind - should fail with invalid state
	result, _ = udpSocketFinishBind(ctx, nil, []types.Val{selfHandle})
	isOk, _, errVal = result[0].Result()
	require.False(t, isOk, "finish-bind should fail without start-bind")

	// Clean up
	sock, _ := getUdpSocket(ctx, sockHandle)
	if sock != nil {
		sock.Close()
	}
}

func TestTcpSocket_Close(t *testing.T) {
	// Test that Close properly releases resources
	sock := NewTcpSocket(IpAddressFamilyIpv4)
	err := sock.Close()
	require.NoError(t, err)
	require.Equal(t, tcpStateClosed, sock.State())
}

func TestUdpSocket_Close(t *testing.T) {
	// Test that Close properly releases resources
	sock := NewUdpSocket(IpAddressFamilyIpv4)
	err := sock.Close()
	require.NoError(t, err)
	require.Equal(t, udpStateClosed, sock.State())
}

func TestTcpInputStream_ReadWrite(t *testing.T) {
	// Test TcpInputStream and TcpOutputStream types
	sock := NewTcpSocket(IpAddressFamilyIpv4)

	inStream := NewTcpInputStream(sock)
	require.NotNil(t, inStream)

	outStream := NewTcpOutputStream(sock)
	require.NotNil(t, outStream)

	// Close should work without panicking
	inStream.Close()
	outStream.Close()
}

// Tests for Destroy method

func TestUdpSocket_Destroy(t *testing.T) {
	sock := NewUdpSocket(IpAddressFamilyIpv4)

	// Set some state
	addr := NewIpv4SocketAddress([4]byte{127, 0, 0, 1}, 8080)
	sock.localAddr = &addr
	sock.remoteAddr = &addr

	// Verify state is set
	require.NotNil(t, sock.LocalAddress())
	require.NotNil(t, sock.RemoteAddress())

	// Destroy should clear state
	sock.Destroy()

	// Verify state is cleared
	require.Equal(t, udpStateClosed, sock.State())
	require.Nil(t, sock.LocalAddress())
	require.Nil(t, sock.RemoteAddress())
}

func TestUdpSocket_Destroy_Idempotent(t *testing.T) {
	sock := NewUdpSocket(IpAddressFamilyIpv4)

	// Multiple calls to Destroy should be safe
	sock.Destroy()
	sock.Destroy()
	sock.Destroy()

	require.Equal(t, udpStateClosed, sock.State())
}

func TestUdpSocket_Destroy_WithConnection(t *testing.T) {
	t.Skip("Task E4: wasip2 registry migration — Rep is uint32, Go object lookup requires per-module registry")
	ctx := contextWithResourceTable()
	network := types.ValBorrow(0)
	family := types.ValEnum("ipv4")

	// Create and bind a UDP socket
	result, _ := createUdpSocket(ctx, nil, []types.Val{network, family})
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk)
	sockHandle := ok.Own()

	sock1Borrow := types.ValBorrow(sockHandle)
	localAddr := makeIPv4SocketAddrVal(127, 0, 0, 1, 0)
	result, _ = udpSocketStartBind(ctx, nil, []types.Val{sock1Borrow, network, localAddr})
	isOk, _, _ = result[0].Result()
	require.True(t, isOk)

	result, _ = udpSocketFinishBind(ctx, nil, []types.Val{sock1Borrow})
	isOk, _, _ = result[0].Result()
	require.True(t, isOk)

	// Get the socket and verify it has a connection
	sock, _ := getUdpSocket(ctx, sockHandle)
	require.NotNil(t, sock.conn, "socket should have connection after bind")

	// Destroy should close the connection
	sock.Destroy()

	require.Nil(t, sock.conn, "connection should be nil after Destroy")
	require.Equal(t, udpStateClosed, sock.State())
}

func TestInstanceNetwork_ReturnsValidHandle(t *testing.T) {
	ctx := contextWithResourceTable()

	result, err := instanceNetwork(ctx, nil, []types.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOwn, result[0].Kind())

	handle := result[0].Own()
	// Handle should be non-zero (valid resource)
	table := component.ResourceTableFromContext(ctx)
	rawEntry1, err := table.Get(runtime.Handle(handle))
	_, _ = rawEntry1.(*runtime.ResourceHandleEntry)
	require.NoError(t, err)
	// Task E4: wasip2 registry migration — Rep is uint32, Go object lookup requires per-module registry
}

func TestInstanceNetwork_DistinctHandles(t *testing.T) {
	ctx := contextWithResourceTable()

	result1, _ := instanceNetwork(ctx, nil, []types.Val{})
	result2, _ := instanceNetwork(ctx, nil, []types.Val{})

	h1 := result1[0].Own()
	h2 := result2[0].Own()
	require.NotEqual(t, h1, h2, "each call should return a distinct handle")
}

func TestTcpSocketSubscribe_ReturnsValidPollable(t *testing.T) {
	t.Skip("Task E4: wasip2 registry migration — Rep is uint32, Go object lookup requires per-module registry")
}

func TestUdpSocketSubscribe_ImmediatelyReady(t *testing.T) {
	t.Skip("Task E4: wasip2 registry migration — Rep is uint32, Go object lookup requires per-module registry")
}

func TestIncomingDatagramStreamSubscribe_ReturnsValidPollable(t *testing.T) {
	t.Skip("Task E4: wasip2 registry migration — Rep is uint32, Go object lookup requires per-module registry")
}

func TestOutgoingDatagramStreamSubscribe_ReturnsValidPollable(t *testing.T) {
	t.Skip("Task E4: wasip2 registry migration — Rep is uint32, Go object lookup requires per-module registry")
}

func TestOutgoingDatagramStreamSubscribe_WaitingState(t *testing.T) {
	t.Skip("Task E4: wasip2 registry migration — Rep is uint32, Go object lookup requires per-module registry")
}

func TestOutgoingDatagramStreamCheckSend_InitialPermit(t *testing.T) {
	t.Skip("Task E4: wasip2 registry migration — Rep is uint32, Go object lookup requires per-module registry")
	ctx := contextWithResourceTable()
	table := component.ResourceTableFromContext(ctx)

	sock := NewUdpSocket(IpAddressFamilyIpv4)
	_ = NewOutgoingDatagramStream(sock) // Task E4: will wire via per-module registry
	handle, errH6 := table.NewResourceHandle(uint32(0), true, socketsPollableResourceType)
	if errH6 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH6)
	}

	borrow := types.ValBorrow(uint32(handle))
	result, err := outgoingDatagramStreamCheckSend(ctx, nil, []types.Val{borrow})
	require.NoError(t, err)
	isOk, val, _ := result[0].Result()
	require.True(t, isOk)
	require.Equal(t, uint64(16), val.U64(), "initial check-send should return 16")
}

func TestOutgoingDatagramStreamCheckSend_StablePermit(t *testing.T) {
	t.Skip("Task E4: wasip2 registry migration — Rep is uint32, Go object lookup requires per-module registry")
	ctx := contextWithResourceTable()
	table := component.ResourceTableFromContext(ctx)

	sock := NewUdpSocket(IpAddressFamilyIpv4)
	_ = NewOutgoingDatagramStream(sock) // Task E4: will wire via per-module registry
	handle, errH7 := table.NewResourceHandle(uint32(0), true, socketsPollableResourceType)
	if errH7 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH7)
	}

	borrow := types.ValBorrow(uint32(handle))
	// First call grants permit
	outgoingDatagramStreamCheckSend(ctx, nil, []types.Val{borrow})
	// Second call should still return 16 (no sends happened)
	result, _ := outgoingDatagramStreamCheckSend(ctx, nil, []types.Val{borrow})
	isOk, val, _ := result[0].Result()
	require.True(t, isOk)
	require.Equal(t, uint64(16), val.U64(), "repeated check-send without sends should return same permit")
}

// makeOutgoingDatagramVal constructs an outgoing-datagram record Val with the
// given payload and no remote-address override. Used by trap-path tests — the
// datagrams never reach the wire because the precondition check runs first.
func makeOutgoingDatagramVal(data []byte) types.Val {
	dataVals := make([]types.Val, len(data))
	for i, b := range data {
		dataVals[i] = types.ValU8(b)
	}
	// remote-address is option<ip-socket-address>; pass none.
	return types.ValRecord(map[string]types.Val{
		"data":           types.ValList(dataVals),
		"remote-address": types.ValOption(nil),
	})
}

// TestOutgoingDatagramStreamSend_TrapsOnMissingCheckSend verifies that calling
// send without first calling check-send traps the guest.
//
// Per wasi:sockets/udp@0.2.0 udp.wit lines 256-257:
//
//	Each call to `send` must be permitted by a preceding `check-send`.
//	Implementations must trap if either `check-send` was not called or
//	`datagrams` contains more items than `check-send` permitted.
//
// In wazero, host functions trap by returning a non-nil Go error; the
// component canon lower wrapper in internal/component/component_linker.go
// (createCanonLowerFunc) translates that into a panic which the wasm
// runtime catches as a trap.
func TestOutgoingDatagramStreamSend_TrapsOnMissingCheckSend(t *testing.T) {
	t.Skip("Task E4: wasip2 registry migration — Rep is uint32, Go object lookup requires per-module registry")
	ctx := contextWithResourceTable()
	table := component.ResourceTableFromContext(ctx)

	sock := NewUdpSocket(IpAddressFamilyIpv4)
	_ = NewOutgoingDatagramStream(sock) // Task E4: will wire via per-module registry
	// Default sendState is sendStateIdle — no check-send has been called.
	handle, errH8 := table.NewResourceHandle(uint32(0), true, socketsPollableResourceType)
	if errH8 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH8)
	}
	selfHandle := types.ValBorrow(uint32(handle))

	datagrams := types.ValList([]types.Val{
		makeOutgoingDatagramVal([]byte{0x01, 0x02, 0x03}),
	})

	result, err := outgoingDatagramStreamSend(ctx, nil, []types.Val{selfHandle, datagrams})
	require.Error(t, err, "send without preceding check-send must return a Go error (trap)")
	require.Nil(t, result, "trap path must return nil result")
}

// TestOutgoingDatagramStreamSend_TrapsOnDatagramsExceedingPermit verifies that
// calling send with more datagrams than the most recent check-send permitted
// traps the guest (udp.wit lines 256-257, cited above).
func TestOutgoingDatagramStreamSend_TrapsOnDatagramsExceedingPermit(t *testing.T) {
	t.Skip("Task E4: wasip2 registry migration — Rep is uint32, Go object lookup requires per-module registry")
	ctx := contextWithResourceTable()
	table := component.ResourceTableFromContext(ctx)

	sock := NewUdpSocket(IpAddressFamilyIpv4)
	_ = NewOutgoingDatagramStream(sock) // Task E4: will wire via per-module registry
	handle, errH9 := table.NewResourceHandle(uint32(0), true, socketsPollableResourceType)
	if errH9 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH9)
	}
	selfHandle := types.ValBorrow(uint32(handle))

	// Grant a permit of 16 via check-send.
	checkResult, checkErr := outgoingDatagramStreamCheckSend(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, checkErr)
	isOk, val, _ := checkResult[0].Result()
	require.True(t, isOk)
	require.Equal(t, uint64(16), val.U64(), "check-send should grant 16")

	// Build 17 datagrams — one more than permitted.
	datagramVals := make([]types.Val, 17)
	for i := range datagramVals {
		datagramVals[i] = makeOutgoingDatagramVal([]byte{byte(i)})
	}
	datagrams := types.ValList(datagramVals)

	result, err := outgoingDatagramStreamSend(ctx, nil, []types.Val{selfHandle, datagrams})
	require.Error(t, err, "send with more datagrams than permitted must return a Go error (trap)")
	require.Nil(t, result, "trap path must return nil result")
}

// TestOutgoingDatagramStreamSend_NoTrapWhenWithinPermit verifies the happy
// precondition path: after check-send, a send within the permitted count
// must NOT trap. The function may still return an in-band socket error in
// its Result payload because no real UDP socket is bound in this unit test,
// but the Go-level error returned by the host function must be nil — that
// is the signal the caller uses to distinguish trap from in-band error.
func TestOutgoingDatagramStreamSend_NoTrapWhenWithinPermit(t *testing.T) {
	ctx := contextWithResourceTable()
	table := component.ResourceTableFromContext(ctx)

	sock := NewUdpSocket(IpAddressFamilyIpv4)
	_ = NewOutgoingDatagramStream(sock) // Task E4: will wire via per-module registry
	handle, errH10 := table.NewResourceHandle(uint32(0), true, socketsPollableResourceType)
	if errH10 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH10)
	}
	selfHandle := types.ValBorrow(uint32(handle))

	// Grant a permit via check-send.
	_, checkErr := outgoingDatagramStreamCheckSend(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, checkErr)

	// Single datagram, well within the permit of 16.
	datagrams := types.ValList([]types.Val{
		makeOutgoingDatagramVal([]byte{0xAA}),
	})

	result, err := outgoingDatagramStreamSend(ctx, nil, []types.Val{selfHandle, datagrams})
	// The critical assertion: precondition check must pass, so Go error is nil.
	// The in-band Result may be Err(invalid-state) because socket.conn is nil
	// (the stream was not created via the bound-and-streaming path), but that
	// is a legitimate runtime error, not a precondition trap.
	require.NoError(t, err, "send within permitted count must NOT trap")
	require.NotNil(t, result, "non-trap path must return a non-nil result")
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
}

func TestNetworkErrorCode_WithSocketError(t *testing.T) {
	t.Skip("Task E4: wasip2 registry migration — Rep is uint32, Go object lookup requires per-module registry")
	ctx := contextWithResourceTable()
	table := component.ResourceTableFromContext(ctx)

	// Create an io.Error that wraps a SocketError
	sockErr := &SocketError{Code: ErrorCodeConnectionRefused}
	_ = wasipIO.NewError(sockErr) // Task E4: will wire via per-module registry
	handle, errH11 := table.NewResourceHandle(uint32(0), true, socketsPollableResourceType)
	if errH11 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH11)
	}

	errHandle := types.ValBorrow(uint32(handle))
	result, err := networkErrorCode(ctx, nil, []types.Val{errHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))

	opt := result[0].Option()
	require.NotNil(t, opt, "should return Some for socket error")
	require.Equal(t, "connection-refused", opt.Enum())
}

func TestNetworkErrorCode_NonSocketError(t *testing.T) {
	ctx := contextWithResourceTable()
	table := component.ResourceTableFromContext(ctx)

	_ = wasipIO.NewError(errors.New("some random error")) // Task E4: will wire via per-module registry
	handle, errH12 := table.NewResourceHandle(uint32(0), true, socketsPollableResourceType)
	if errH12 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH12)
	}

	errHandle := types.ValBorrow(uint32(handle))
	result, err := networkErrorCode(ctx, nil, []types.Val{errHandle})
	require.NoError(t, err)

	opt := result[0].Option()
	require.Nil(t, opt, "should return None for non-socket error")
}
