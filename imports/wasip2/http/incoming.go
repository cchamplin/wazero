// imports/wasip2/http/incoming.go

package http

import (
	"context"

	"github.com/tetratelabs/wazero/internal/component"
)

// instantiateIncomingHandler registers wasi:http/incoming-handler@0.2.0
func instantiateIncomingHandler(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:http/incoming-handler@0.2.0")

	// handle: func(request: own<incoming-request>, response-out: own<response-outparam>)
	inst.FuncNoType("handle", incomingHandlerHandle)

	return inst.SkipValidation().Build()
}

// incomingHandlerHandle handles an incoming HTTP request.
// Signature: func(request: own<incoming-request>, response-out: own<response-outparam>)
func incomingHandlerHandle(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// This is a no-op placeholder - returns nothing (unit)
	return []component.Val{}, nil
}
