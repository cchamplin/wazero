// imports/wasip2/sockets/network.go

package sockets

import (
	"context"
	"net"
	"strings"

	wasipIO "github.com/tetratelabs/wazero/imports/wasip2/io"
	"github.com/tetratelabs/wazero/internal/component"
)

// instantiateNetwork registers wasi:sockets/network@0.2.0
func instantiateNetwork(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:sockets/network@0.2.0")

	// network resource - represents a network capability
	inst.Resource("network", func(rep uint32) {
		// Destructor - nothing to clean up for now
	})

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
func instanceNetwork(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []component.Val{component.ValOwn(0)}, nil
	}

	network := NewNetwork()
	handle := table.New(network, true)
	return []component.Val{component.ValOwn(uint32(handle))}, nil
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
func resolveAddresses(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		errVal := component.ValEnum("invalid-argument")
		return []component.Val{component.ValResultError(&errVal)}, nil
	}

	// args[0] = borrow<network>
	name := args[1].StringVal()

	// Validate input per WASI conformance
	if name == "" || strings.TrimSpace(name) != name || strings.Contains(name, "/") ||
		strings.Contains(name, "://") || strings.ContainsAny(name, "<>&") {
		errVal := component.ValEnum("invalid-argument")
		return []component.Val{component.ValResultError(&errVal)}, nil
	}

	// Reject if name contains a port (SplitHostPort succeeds when port present)
	if _, _, err := net.SplitHostPort(name); err == nil {
		errVal := component.ValEnum("invalid-argument")
		return []component.Val{component.ValResultError(&errVal)}, nil
	}

	// Check for IP literal
	if ip := net.ParseIP(name); ip != nil {
		addr := netIPToIpAddress(ip)
		stream := NewResolveAddressStream([]IpAddress{addr})
		handle := table.New(stream, true)
		handleVal := component.ValOwn(uint32(handle))
		return []component.Val{component.ValResultOk(&handleVal)}, nil
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

	handleVal := component.ValOwn(uint32(handle))
	return []component.Val{component.ValResultOk(&handleVal)}, nil
}

// resolveNextAddress returns the next resolved address.
// Signature: func(self: borrow<resolve-address-stream>) -> result<option<ip-address>, error-code>
func resolveNextAddress(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		none := component.ValOption(nil)
		return []component.Val{component.ValResultOk(&none)}, nil
	}

	entry, err := table.Get(component.Handle(handle))
	if err != nil {
		errVal := component.ValEnum("invalid-argument")
		return []component.Val{component.ValResultError(&errVal)}, nil
	}

	stream, ok := entry.Rep.(*ResolveAddressStream)
	if !ok {
		errVal := component.ValEnum("invalid-argument")
		return []component.Val{component.ValResultError(&errVal)}, nil
	}

	if !stream.IsReady() {
		errVal := component.ValEnum("would-block")
		return []component.Val{component.ValResultError(&errVal)}, nil
	}

	addr, nextErr := stream.NextAddress()
	if nextErr != nil {
		errVal := component.ValEnum("name-unresolvable")
		return []component.Val{component.ValResultError(&errVal)}, nil
	}

	if addr == nil {
		none := component.ValOption(nil)
		return []component.Val{component.ValResultOk(&none)}, nil
	}

	addrVal := ipAddressToVal(*addr)
	opt := component.ValOption(&addrVal)
	return []component.Val{component.ValResultOk(&opt)}, nil
}

// resolveAddressStreamSubscribe returns a pollable for the stream.
// Signature: func(self: borrow<resolve-address-stream>) -> own<pollable>
func resolveAddressStreamSubscribe(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []component.Val{component.ValOwn(0)}, nil
	}

	entry, err := table.Get(component.Handle(handle))
	if err != nil {
		return []component.Val{component.ValOwn(0)}, nil
	}

	stream, ok := entry.Rep.(*ResolveAddressStream)
	if !ok {
		return []component.Val{component.ValOwn(0)}, nil
	}

	pollable := wasipIO.NewPollable(
		func() bool { return stream.IsReady() },
		func() { <-stream.done },
	)
	pollHandle := table.New(pollable, true)
	return []component.Val{component.ValOwn(uint32(pollHandle))}, nil
}
