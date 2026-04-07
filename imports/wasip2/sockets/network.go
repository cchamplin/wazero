// imports/wasip2/sockets/network.go

package sockets

import (
	"context"
	"errors"
	"net"
	"strings"

	wasipIO "github.com/tetratelabs/wazero/imports/wasip2/io"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// instantiateNetwork registers wasi:sockets/network@0.2.0
func instantiateNetwork(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:sockets/network@0.2.0")

	// network resource - represents a network capability
	inst.Resource("network", func(rep uint32) {
		// Destructor - nothing to clean up for now
	})

	inst.FuncNoType("network-error-code", networkErrorCode)

	return inst.SkipValidation().Build()
}

// instantiateInstanceNetwork registers wasi:sockets/instance-network@0.2.0
func instantiateInstanceNetwork(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:sockets/instance-network@0.2.0")

	// instance-network: func() -> own<network>
	inst.FuncNoType("instance-network", instanceNetwork)

	return inst.SkipValidation().Build()
}

// instanceNetwork returns the network capability for this instance.
// Signature: func() -> own<network>
func instanceNetwork(ctx context.Context, args []types.Val) ([]types.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []types.Val{types.ValOwn(0)}, nil
	}

	network := NewNetwork()
	handle := table.New(network, true)
	return []types.Val{types.ValOwn(uint32(handle))}, nil
}

// instantiateIpNameLookup registers wasi:sockets/ip-name-lookup@0.2.0
func instantiateIpNameLookup(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:sockets/ip-name-lookup@0.2.0")

	// resolve-address-stream resource
	inst.Resource("resolve-address-stream", func(rep uint32) {
		// Destructor - clean up resolver stream
	})

	// resolve-addresses: func(network: borrow<network>, name: string) -> result<own<resolve-address-stream>, error-code>
	inst.FuncNoType("resolve-addresses", resolveAddresses)

	// [method]resolve-address-stream.resolve-next-address: func() -> result<option<ip-address>, error-code>
	inst.FuncNoType("[method]resolve-address-stream.resolve-next-address", resolveNextAddress)

	// [method]resolve-address-stream.subscribe: func() -> own<pollable>
	inst.FuncNoType("[method]resolve-address-stream.subscribe", resolveAddressStreamSubscribe)

	return inst.SkipValidation().Build()
}

// resolveAddresses starts name resolution.
// Signature: func(network: borrow<network>, name: string) -> result<own<resolve-address-stream>, error-code>
func resolveAddresses(ctx context.Context, args []types.Val) ([]types.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		errVal := types.ValEnum("invalid-argument")
		return []types.Val{types.ValResultError(&errVal)}, nil
	}

	// args[0] = borrow<network>
	name := args[1].StringVal()

	// Validate input per WASI conformance
	if name == "" || strings.TrimSpace(name) != name || strings.Contains(name, "/") ||
		strings.Contains(name, "://") || strings.ContainsAny(name, "<>&") {
		errVal := types.ValEnum("invalid-argument")
		return []types.Val{types.ValResultError(&errVal)}, nil
	}

	// Reject if name contains a port (SplitHostPort succeeds when port present)
	if _, _, err := net.SplitHostPort(name); err == nil {
		errVal := types.ValEnum("invalid-argument")
		return []types.Val{types.ValResultError(&errVal)}, nil
	}

	// Check for IP literal
	if ip := net.ParseIP(name); ip != nil {
		addr := netIPToIpAddress(ip)
		stream := NewResolveAddressStream([]IpAddress{addr})
		handle := table.New(stream, true)
		handleVal := types.ValOwn(uint32(handle))
		return []types.Val{types.ValResultOk(&handleVal)}, nil
	}

	// Async DNS resolution with cancellable context
	dnsCtx, cancel := context.WithCancel(context.Background())
	stream := NewResolveAddressStreamAsync(cancel)
	handle := table.New(stream, true)

	go func() {
		addrs, err := net.DefaultResolver.LookupHost(dnsCtx, name)
		if err != nil {
			stream.SetResult(nil, err)
			return
		}
		var resolved []IpAddress
		for _, a := range addrs {
			if ip := net.ParseIP(a); ip != nil {
				resolved = append(resolved, netIPToIpAddress(ip))
			}
		}
		stream.SetResult(resolved, nil)
	}()

	handleVal := types.ValOwn(uint32(handle))
	return []types.Val{types.ValResultOk(&handleVal)}, nil
}

// resolveNextAddress returns the next resolved address.
// Signature: func(self: borrow<resolve-address-stream>) -> result<option<ip-address>, error-code>
func resolveNextAddress(ctx context.Context, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		none := types.ValOption(nil)
		return []types.Val{types.ValResultOk(&none)}, nil
	}

	entry, err := table.Get(runtime.Handle(handle))
	if err != nil {
		errVal := types.ValEnum("invalid-argument")
		return []types.Val{types.ValResultError(&errVal)}, nil
	}

	stream, ok := entry.Rep.(*ResolveAddressStream)
	if !ok {
		errVal := types.ValEnum("invalid-argument")
		return []types.Val{types.ValResultError(&errVal)}, nil
	}

	if !stream.IsReady() {
		errVal := types.ValEnum("would-block")
		return []types.Val{types.ValResultError(&errVal)}, nil
	}

	addr, nextErr := stream.NextAddress()
	if nextErr != nil {
		errVal := types.ValEnum("name-unresolvable")
		return []types.Val{types.ValResultError(&errVal)}, nil
	}

	if addr == nil {
		none := types.ValOption(nil)
		return []types.Val{types.ValResultOk(&none)}, nil
	}

	addrVal := ipAddressToVal(*addr)
	opt := types.ValOption(&addrVal)
	return []types.Val{types.ValResultOk(&opt)}, nil
}

// resolveAddressStreamSubscribe returns a pollable for the stream.
// Signature: func(self: borrow<resolve-address-stream>) -> own<pollable>
func resolveAddressStreamSubscribe(ctx context.Context, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []types.Val{types.ValOwn(0)}, nil
	}

	entry, err := table.Get(runtime.Handle(handle))
	if err != nil {
		return []types.Val{types.ValOwn(0)}, nil
	}

	stream, ok := entry.Rep.(*ResolveAddressStream)
	if !ok {
		return []types.Val{types.ValOwn(0)}, nil
	}

	pollable := wasipIO.NewPollable(
		func() bool { return stream.IsReady() },
		func() { <-stream.done },
	)
	pollHandle := table.New(pollable, true)
	return []types.Val{types.ValOwn(uint32(pollHandle))}, nil
}

// networkErrorCode extracts a socket error code from an io.Error resource.
// Signature: func(err: borrow<error>) -> option<error-code>
func networkErrorCode(ctx context.Context, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []types.Val{types.ValOption(nil)}, nil
	}

	entry, err := table.Get(runtime.Handle(handle))
	if err != nil {
		return []types.Val{types.ValOption(nil)}, nil
	}

	ioErr, ok := entry.Rep.(*wasipIO.Error)
	if !ok {
		return []types.Val{types.ValOption(nil)}, nil
	}

	// Unwrap the Go error and check if it's a SocketError
	var sockErr *SocketError
	if errors.As(ioErr.Unwrap(), &sockErr) {
		codeVal := types.ValEnum(string(sockErr.Code))
		return []types.Val{types.ValOption(&codeVal)}, nil
	}

	return []types.Val{types.ValOption(nil)}, nil
}
