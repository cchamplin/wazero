// imports/wasip2/http/outgoing.go

// WIT source of truth: debug-vendored/WASI/proposals/http/wit/{types,handler,proxy}.wit
// Package version: wasi:http@0.2.9 (wazero targets wasi:http@0.2.0)
//
package http

import (
	"context"
	"fmt"
	gohttp "net/http"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// DefaultHTTPClient is the HTTP client used for outgoing requests.
// It can be replaced with a custom client for testing or custom configuration.
var DefaultHTTPClient = gohttp.DefaultClient

// instantiateOutgoingHandler registers wasi:http/outgoing-handler@0.2.0
func instantiateOutgoingHandler(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:http/outgoing-handler@0.2.0")

	// handle: func(request: own<outgoing-request>, options: option<own<request-options>>) -> result<own<future-incoming-response>, error-code>
	inst.Func("handle", outgoingHandlerHandle)

	return inst.SkipValidation().Build()
}

// errorCodeToVariant converts an ErrorCode to a types.Val variant.
func errorCodeToVariant(code ErrorCode) types.Val {
	return types.ValVariant(string(code), nil)
}

// outgoingHandlerHandle sends an HTTP request and returns a future response.
// Signature: func(request: own<outgoing-request>, options: option<own<request-options>>) -> result<own<future-incoming-response>, error-code>
func outgoingHandlerHandle(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		// No resource table, fall back to placeholder behavior
		futureHandle := types.ValOwn(0)
		return []types.Val{types.ValResultOk(&futureHandle)}, nil
	}

	// Get the request handle (own<outgoing-request>)
	reqHandle := runtime.Handle(args[0].Own())

	// Remove and consume the request from the table (ownership transfer)
	reqEntry, err := table.Remove(reqHandle)
	if err != nil {
		errVal := errorCodeToVariant(ErrorCodeInternalError)
		return []types.Val{types.ValResultError(&errVal)}, nil
	}

	_ = reqEntry // Task E4: resolve *OutgoingRequest via per-module registry using reqEntry.Rep

	// Get optional request options
	optionVal := args[1].Option()
	if optionVal != nil {
		optsHandle := runtime.Handle(optionVal.Own())
		table.Remove(optsHandle)
		// Task E4: resolve *RequestOptions via per-module registry using optsEntry.Rep
	}

	// Task E4: when req is resolved via registry, convert to Go HTTP request
	var httpReq *gohttp.Request
	httpReq, err = gohttp.NewRequestWithContext(ctx, "GET", "http://localhost/", nil)
	if err != nil {
		errVal := errorCodeToVariant(ErrorCodeHTTPRequestURIInvalid)
		return []types.Val{types.ValResultError(&errVal)}, nil
	}

	// Create the future response
	future := NewFutureIncomingResponse()

	// Task E4: when opts is resolved via registry, use MakeHTTPClient(opts)
	client := DefaultHTTPClient

	// Start the async HTTP request
	go func() {
		resp, err := client.Do(httpReq)
		if err != nil {
			errCode := ErrorCodeFromError(err)
			future.SetError(errCode)
			return
		}

		// Create incoming response
		incomingResp := NewIncomingResponseFromHTTP(resp)
		future.SetResponse(incomingResp)
	}()

	// Register the future in the resource table
	futureHandle, err := table.NewResourceHandle(uint32(0), true, httpFutureIncomingResponseResourceType)
	if err != nil {
		return nil, fmt.Errorf("create resource handle: %w", err)
	}

	// Return success with the future handle
	result := types.ValOwn(uint32(futureHandle))
	return []types.Val{types.ValResultOk(&result)}, nil
}

// Note: getOutgoingRequest and getFutureIncomingResponse are defined in http.go
