// imports/wasip2/http/outgoing.go

package http

import (
	"context"

	"github.com/tetratelabs/wazero/internal/component"
)

// instantiateOutgoingHandler registers wasi:http/outgoing-handler@0.2.0
func instantiateOutgoingHandler(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:http/outgoing-handler@0.2.0")

	// handle: func(request: own<outgoing-request>, options: option<own<request-options>>) -> result<own<future-incoming-response>, error-code>
	inst.FuncNoType("handle", outgoingHandlerHandle)

	return inst.Build()
}

// outgoingHandlerHandle sends an HTTP request and returns a future response.
// Signature: func(request: own<outgoing-request>, options: option<own<request-options>>) -> result<own<future-incoming-response>, error-code>
func outgoingHandlerHandle(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return a placeholder future-incoming-response handle
	futureHandle := component.ValOwn(0)
	return []component.Val{component.ValResultOk(&futureHandle)}, nil
}
