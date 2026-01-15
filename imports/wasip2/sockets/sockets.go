// imports/wasip2/sockets/sockets.go

package sockets

import (
	"github.com/tetratelabs/wazero/internal/component"
)

// Instantiate registers all wasi:sockets interfaces with the linker.
func Instantiate(linker *component.Linker) error {
	if err := instantiateNetwork(linker); err != nil {
		return err
	}
	if err := instantiateInstanceNetwork(linker); err != nil {
		return err
	}
	if err := instantiateIpNameLookup(linker); err != nil {
		return err
	}
	if err := instantiateTcp(linker); err != nil {
		return err
	}
	if err := instantiateTcpCreateSocket(linker); err != nil {
		return err
	}
	if err := instantiateUdp(linker); err != nil {
		return err
	}
	if err := instantiateUdpCreateSocket(linker); err != nil {
		return err
	}
	return nil
}
