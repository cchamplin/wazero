// imports/wasip2/http/outgoing.go

package http

import (
	"context"
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
	inst.FuncNoType("handle", outgoingHandlerHandle)

	return inst.SkipValidation().Build()
}

// errorCodeToVariant converts an ErrorCode to a types.Val variant.
func errorCodeToVariant(code ErrorCode) types.Val {
	return types.ValVariant(string(code), nil)
}

// outgoingHandlerHandle sends an HTTP request and returns a future response.
// Signature: func(request: own<outgoing-request>, options: option<own<request-options>>) -> result<own<future-incoming-response>, error-code>
func outgoingHandlerHandle(ctx context.Context, args []types.Val) ([]types.Val, error) {
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

	req, ok := reqEntry.Rep.(*OutgoingRequest)
	if !ok {
		errVal := errorCodeToVariant(ErrorCodeInternalError)
		return []types.Val{types.ValResultError(&errVal)}, nil
	}

	// Get optional request options
	var opts *RequestOptions
	optionVal := args[1].Option()
	if optionVal != nil {
		optsHandle := runtime.Handle(optionVal.Own())
		optsEntry, err := table.Remove(optsHandle)
		if err == nil {
			if o, ok := optsEntry.Rep.(*RequestOptions); ok {
				opts = o
			}
		}
	}

	// Convert to Go HTTP request
	httpReq, err := req.ToHTTPRequest(ctx)
	if err != nil {
		errVal := errorCodeToVariant(ErrorCodeHTTPRequestURIInvalid)
		return []types.Val{types.ValResultError(&errVal)}, nil
	}

	// Create the future response
	future := NewFutureIncomingResponse()

	// Get the HTTP client
	client := DefaultHTTPClient
	if opts != nil {
		client = MakeHTTPClient(opts)
	}

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
	futureHandle := table.New(future, true)

	// Return success with the future handle
	result := types.ValOwn(uint32(futureHandle))
	return []types.Val{types.ValResultOk(&result)}, nil
}

// Note: getOutgoingRequest and getFutureIncomingResponse are defined in http.go
