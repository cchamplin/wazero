// imports/wasip2/sockets/network.go

package sockets

import (
	"context"

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
	// Return placeholder resolve-address-stream handle
	handle := component.ValOwn(0)
	return []component.Val{component.ValResultOk(&handle)}, nil
}

// resolveNextAddress returns the next resolved address.
// Signature: func(self: borrow<resolve-address-stream>) -> result<option<ip-address>, error-code>
func resolveNextAddress(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return None - placeholder (no addresses resolved)
	none := component.ValOption(nil)
	return []component.Val{component.ValResultOk(&none)}, nil
}

// resolveAddressStreamSubscribe returns a pollable for the stream.
// Signature: func(self: borrow<resolve-address-stream>) -> own<pollable>
func resolveAddressStreamSubscribe(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return placeholder pollable handle
	return []component.Val{component.ValOwn(0)}, nil
}
