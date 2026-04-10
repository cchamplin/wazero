// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: WASI sockets conformance tests verify that
// wazero's sockets host module types and factory functions behave
// correctly.
package conformance

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasip2/sockets"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestWASISockets exercises the wasi:sockets host module API surface.
//
// No counterpart (justified): WASI P2 host module conformance invariant.
func TestWASISockets(t *testing.T) {

	// ------------------------------------------------------------------
	// Case 1: Instantiate registers all wasi:sockets interfaces.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. sockets.Instantiate must register all socket-related
	// interfaces with the linker.
	t.Run("InstantiateRegistersInterfaces", func(t *testing.T) {
		rt := wazero.NewRuntime(context.TODO())
		defer rt.Close(context.TODO())
		linker := component.NewComponentLinker(rt)
		err := sockets.Instantiate(linker)
		require.NoError(t, err)

		for _, iface := range []string{
			"wasi:sockets/network@0.2.0",
			"wasi:sockets/instance-network@0.2.0",
			"wasi:sockets/ip-name-lookup@0.2.0",
			"wasi:sockets/tcp@0.2.0",
			"wasi:sockets/tcp-create-socket@0.2.0",
			"wasi:sockets/udp@0.2.0",
			"wasi:sockets/udp-create-socket@0.2.0",
		} {
			def, lookupErr := linker.MatchImport(iface)
			require.NoError(t, lookupErr, "interface %s should be registered", iface)
			_, ok := def.(*component.InstanceDef)
			require.True(t, ok, "expected InstanceDef for %s", iface)
		}
	})

	// ------------------------------------------------------------------
	// Case 2: NewTcpSocket creates an unbound IPv4 socket.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. A freshly created TCP socket must be in the unbound
	// state with the requested address family.
	t.Run("NewTcpSocketIPv4", func(t *testing.T) {
		sock := sockets.NewTcpSocket(sockets.IpAddressFamilyIpv4)
		require.Equal(t, sockets.IpAddressFamilyIpv4, sock.Family())
		require.False(t, sock.IsListening(), "new socket should not be listening")
		require.True(t, sock.LocalAddress() == nil, "new socket should have no local address")
		require.True(t, sock.RemoteAddress() == nil, "new socket should have no remote address")
	})

	// ------------------------------------------------------------------
	// Case 3: NewUdpSocket creates an unbound IPv4 socket.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. A freshly created UDP socket must be in the unbound
	// state.
	t.Run("NewUdpSocketIPv4", func(t *testing.T) {
		sock := sockets.NewUdpSocket(sockets.IpAddressFamilyIpv4)
		require.Equal(t, sockets.IpAddressFamilyIpv4, sock.Family())
		require.True(t, sock.LocalAddress() == nil, "new socket should have no local address")
	})

	// ------------------------------------------------------------------
	// Case 4: IpAddress types roundtrip correctly.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. IPv4 and IPv6 address constructors must preserve
	// the address bytes and report the correct family.
	t.Run("IpAddressTypes", func(t *testing.T) {
		ipv4 := sockets.NewIpv4Address([4]byte{127, 0, 0, 1})
		require.Equal(t, sockets.IpAddressFamilyIpv4, ipv4.Family())
		require.Equal(t, [4]byte{127, 0, 0, 1}, ipv4.Ipv4())

		var ipv6Bytes [16]byte
		ipv6Bytes[15] = 1 // ::1
		ipv6 := sockets.NewIpv6Address(ipv6Bytes)
		require.Equal(t, sockets.IpAddressFamilyIpv6, ipv6.Family())
		require.Equal(t, ipv6Bytes, ipv6.Ipv6())
	})

	// ------------------------------------------------------------------
	// Case 5: IpSocketAddress preserves port and address.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. ip-socket-address must preserve the port and address.
	t.Run("IpSocketAddress", func(t *testing.T) {
		sa := sockets.NewIpv4SocketAddress([4]byte{10, 0, 0, 1}, 8080)
		require.Equal(t, uint16(8080), sa.Port())
		require.Equal(t, sockets.IpAddressFamilyIpv4, sa.Family())
		require.Equal(t, [4]byte{10, 0, 0, 1}, sa.Address().Ipv4())
	})

	// ------------------------------------------------------------------
	// Case 6: ResolveAddressStream with pre-resolved addresses iterates
	// correctly.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. resolve-address-stream with pre-resolved addresses
	// must yield each address once and then nil.
	t.Run("ResolveAddressStreamIteration", func(t *testing.T) {
		addrs := []sockets.IpAddress{
			sockets.NewIpv4Address([4]byte{127, 0, 0, 1}),
			sockets.NewIpv4Address([4]byte{192, 168, 1, 1}),
		}
		stream := sockets.NewResolveAddressStream(addrs)
		require.True(t, stream.IsReady(), "pre-resolved stream should be immediately ready")

		a1, err := stream.NextAddress()
		require.NoError(t, err)
		require.True(t, a1 != nil, "first address should not be nil")
		require.Equal(t, [4]byte{127, 0, 0, 1}, a1.Ipv4())

		a2, err := stream.NextAddress()
		require.NoError(t, err)
		require.True(t, a2 != nil, "second address should not be nil")

		a3, err := stream.NextAddress()
		require.NoError(t, err)
		require.True(t, a3 == nil, "third call should return nil (exhausted)")
	})
}
