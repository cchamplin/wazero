// imports/wasip2/http/http.go

package http

import (
	"context"

	"github.com/tetratelabs/wazero/internal/component"
)

// Instantiate registers all wasi:http interfaces with the linker.
func Instantiate(linker *component.Linker) error {
	if err := instantiateTypes(linker); err != nil {
		return err
	}
	if err := instantiateOutgoingHandler(linker); err != nil {
		return err
	}
	if err := instantiateIncomingHandler(linker); err != nil {
		return err
	}
	return nil
}

// instantiateTypes registers wasi:http/types@0.2.0
func instantiateTypes(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:http/types@0.2.0")

	// ==================
	// Fields resource
	// ==================
	inst.Resource("fields", func(rep uint32) {
		// Destructor - clean up fields
	})

	inst.FuncNoType("[constructor]fields", fieldsConstructor)
	inst.FuncNoType("[static]fields.from-list", fieldsFromList)
	inst.FuncNoType("[method]fields.get", fieldsGet)
	inst.FuncNoType("[method]fields.set", fieldsSet)
	inst.FuncNoType("[method]fields.delete", fieldsDelete)
	inst.FuncNoType("[method]fields.append", fieldsAppend)
	inst.FuncNoType("[method]fields.entries", fieldsEntries)
	inst.FuncNoType("[method]fields.clone", fieldsClone)

	// ==================
	// Incoming request resource
	// ==================
	inst.Resource("incoming-request", func(rep uint32) {
		// Destructor
	})

	inst.FuncNoType("[method]incoming-request.method", incomingRequestMethod)
	inst.FuncNoType("[method]incoming-request.path-with-query", incomingRequestPathWithQuery)
	inst.FuncNoType("[method]incoming-request.scheme", incomingRequestScheme)
	inst.FuncNoType("[method]incoming-request.authority", incomingRequestAuthority)
	inst.FuncNoType("[method]incoming-request.headers", incomingRequestHeaders)
	inst.FuncNoType("[method]incoming-request.consume", incomingRequestConsume)

	// ==================
	// Outgoing request resource
	// ==================
	inst.Resource("outgoing-request", func(rep uint32) {
		// Destructor
	})

	inst.FuncNoType("[constructor]outgoing-request", outgoingRequestConstructor)
	inst.FuncNoType("[method]outgoing-request.method", outgoingRequestMethod)
	inst.FuncNoType("[method]outgoing-request.set-method", outgoingRequestSetMethod)
	inst.FuncNoType("[method]outgoing-request.path-with-query", outgoingRequestPathWithQuery)
	inst.FuncNoType("[method]outgoing-request.set-path-with-query", outgoingRequestSetPathWithQuery)
	inst.FuncNoType("[method]outgoing-request.scheme", outgoingRequestScheme)
	inst.FuncNoType("[method]outgoing-request.set-scheme", outgoingRequestSetScheme)
	inst.FuncNoType("[method]outgoing-request.authority", outgoingRequestAuthority)
	inst.FuncNoType("[method]outgoing-request.set-authority", outgoingRequestSetAuthority)
	inst.FuncNoType("[method]outgoing-request.headers", outgoingRequestHeaders)
	inst.FuncNoType("[method]outgoing-request.body", outgoingRequestBody)

	// ==================
	// Incoming response resource
	// ==================
	inst.Resource("incoming-response", func(rep uint32) {
		// Destructor
	})

	inst.FuncNoType("[method]incoming-response.status", incomingResponseStatus)
	inst.FuncNoType("[method]incoming-response.headers", incomingResponseHeaders)
	inst.FuncNoType("[method]incoming-response.consume", incomingResponseConsume)

	// ==================
	// Outgoing response resource
	// ==================
	inst.Resource("outgoing-response", func(rep uint32) {
		// Destructor
	})

	inst.FuncNoType("[constructor]outgoing-response", outgoingResponseConstructor)
	inst.FuncNoType("[method]outgoing-response.status-code", outgoingResponseStatusCode)
	inst.FuncNoType("[method]outgoing-response.set-status-code", outgoingResponseSetStatusCode)
	inst.FuncNoType("[method]outgoing-response.headers", outgoingResponseHeaders)
	inst.FuncNoType("[method]outgoing-response.body", outgoingResponseBody)

	// ==================
	// Incoming body resource
	// ==================
	inst.Resource("incoming-body", func(rep uint32) {
		// Destructor
	})

	inst.FuncNoType("[method]incoming-body.stream", incomingBodyStream)
	inst.FuncNoType("[static]incoming-body.finish", incomingBodyFinish)

	// ==================
	// Outgoing body resource
	// ==================
	inst.Resource("outgoing-body", func(rep uint32) {
		// Destructor
	})

	inst.FuncNoType("[method]outgoing-body.write", outgoingBodyWrite)
	inst.FuncNoType("[static]outgoing-body.finish", outgoingBodyFinish)

	// ==================
	// Future incoming response resource
	// ==================
	inst.Resource("future-incoming-response", func(rep uint32) {
		// Destructor
	})

	inst.FuncNoType("[method]future-incoming-response.get", futureIncomingResponseGet)
	inst.FuncNoType("[method]future-incoming-response.subscribe", futureIncomingResponseSubscribe)

	// ==================
	// Future trailers resource
	// ==================
	inst.Resource("future-trailers", func(rep uint32) {
		// Destructor
	})

	inst.FuncNoType("[method]future-trailers.get", futureTrailersGet)
	inst.FuncNoType("[method]future-trailers.subscribe", futureTrailersSubscribe)

	// ==================
	// Response outparam resource
	// ==================
	inst.Resource("response-outparam", func(rep uint32) {
		// Destructor
	})

	inst.FuncNoType("[static]response-outparam.set", responseOutparamSet)

	// ==================
	// Request options resource
	// ==================
	inst.Resource("request-options", func(rep uint32) {
		// Destructor
	})

	inst.FuncNoType("[constructor]request-options", requestOptionsConstructor)
	inst.FuncNoType("[method]request-options.connect-timeout", requestOptionsConnectTimeout)
	inst.FuncNoType("[method]request-options.set-connect-timeout", requestOptionsSetConnectTimeout)
	inst.FuncNoType("[method]request-options.first-byte-timeout", requestOptionsFirstByteTimeout)
	inst.FuncNoType("[method]request-options.set-first-byte-timeout", requestOptionsSetFirstByteTimeout)
	inst.FuncNoType("[method]request-options.between-bytes-timeout", requestOptionsBetweenBytesTimeout)
	inst.FuncNoType("[method]request-options.set-between-bytes-timeout", requestOptionsSetBetweenBytesTimeout)

	// ==================
	// Functions
	// ==================
	inst.FuncNoType("http-error-code", httpErrorCode)

	return inst.Build()
}

// ====================
// Fields functions
// ====================

// fieldsConstructor creates a new empty fields.
// Signature: func() -> own<fields>
func fieldsConstructor(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValOwn(0)}, nil
}

// fieldsFromList creates fields from a list of entries.
// Signature: func(entries: list<tuple<field-key, field-value>>) -> result<own<fields>, header-error>
func fieldsFromList(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := component.ValOwn(0)
	return []component.Val{component.ValResultOk(&handle)}, nil
}

// fieldsGet returns the values for a field name.
// Signature: func(self: borrow<fields>, name: field-key) -> list<field-value>
func fieldsGet(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return empty list as placeholder
	return []component.Val{component.ValList([]component.Val{})}, nil
}

// fieldsSet sets the values for a field name.
// Signature: func(self: borrow<fields>, name: field-key, values: list<field-value>) -> result<_, header-error>
func fieldsSet(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValResultOk(nil)}, nil
}

// fieldsDelete removes a field by name.
// Signature: func(self: borrow<fields>, name: field-key) -> result<_, header-error>
func fieldsDelete(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValResultOk(nil)}, nil
}

// fieldsAppend appends a value to a field.
// Signature: func(self: borrow<fields>, name: field-key, value: field-value) -> result<_, header-error>
func fieldsAppend(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValResultOk(nil)}, nil
}

// fieldsEntries returns all field entries.
// Signature: func(self: borrow<fields>) -> list<tuple<field-key, field-value>>
func fieldsEntries(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return empty list as placeholder
	return []component.Val{component.ValList([]component.Val{})}, nil
}

// fieldsClone creates a copy of the fields.
// Signature: func(self: borrow<fields>) -> own<fields>
func fieldsClone(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValOwn(0)}, nil
}

// ====================
// Outgoing request functions
// ====================

// outgoingRequestConstructor creates a new outgoing request.
// Signature: func(headers: own<fields>) -> own<outgoing-request>
func outgoingRequestConstructor(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValOwn(0)}, nil
}

// outgoingRequestMethod returns the HTTP method.
// Signature: func(self: borrow<outgoing-request>) -> method
func outgoingRequestMethod(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return GET as placeholder
	return []component.Val{component.ValVariant("get", nil)}, nil
}

// outgoingRequestSetMethod sets the HTTP method.
// Signature: func(self: borrow<outgoing-request>, method: method) -> result<_, _>
func outgoingRequestSetMethod(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValResultOk(nil)}, nil
}

// outgoingRequestPathWithQuery returns the path with query.
// Signature: func(self: borrow<outgoing-request>) -> option<string>
func outgoingRequestPathWithQuery(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return None as placeholder
	return []component.Val{component.ValOption(nil)}, nil
}

// outgoingRequestSetPathWithQuery sets the path with query.
// Signature: func(self: borrow<outgoing-request>, path: option<string>) -> result<_, _>
func outgoingRequestSetPathWithQuery(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValResultOk(nil)}, nil
}

// outgoingRequestScheme returns the scheme.
// Signature: func(self: borrow<outgoing-request>) -> option<scheme>
func outgoingRequestScheme(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return None as placeholder
	return []component.Val{component.ValOption(nil)}, nil
}

// outgoingRequestSetScheme sets the scheme.
// Signature: func(self: borrow<outgoing-request>, scheme: option<scheme>) -> result<_, _>
func outgoingRequestSetScheme(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValResultOk(nil)}, nil
}

// outgoingRequestAuthority returns the authority.
// Signature: func(self: borrow<outgoing-request>) -> option<string>
func outgoingRequestAuthority(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return None as placeholder
	return []component.Val{component.ValOption(nil)}, nil
}

// outgoingRequestSetAuthority sets the authority.
// Signature: func(self: borrow<outgoing-request>, authority: option<string>) -> result<_, _>
func outgoingRequestSetAuthority(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValResultOk(nil)}, nil
}

// outgoingRequestHeaders returns the headers.
// Signature: func(self: borrow<outgoing-request>) -> own<fields>
func outgoingRequestHeaders(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValOwn(0)}, nil
}

// outgoingRequestBody returns the body.
// Signature: func(self: borrow<outgoing-request>) -> result<own<outgoing-body>, _>
func outgoingRequestBody(ctx context.Context, args []component.Val) ([]component.Val, error) {
	body := component.ValOwn(0)
	return []component.Val{component.ValResultOk(&body)}, nil
}

// ====================
// Incoming request functions
// ====================

// incomingRequestMethod returns the HTTP method.
// Signature: func(self: borrow<incoming-request>) -> method
func incomingRequestMethod(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return GET as placeholder
	return []component.Val{component.ValVariant("get", nil)}, nil
}

// incomingRequestPathWithQuery returns the path with query.
// Signature: func(self: borrow<incoming-request>) -> option<string>
func incomingRequestPathWithQuery(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return None as placeholder
	return []component.Val{component.ValOption(nil)}, nil
}

// incomingRequestScheme returns the scheme.
// Signature: func(self: borrow<incoming-request>) -> option<scheme>
func incomingRequestScheme(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return None as placeholder
	return []component.Val{component.ValOption(nil)}, nil
}

// incomingRequestAuthority returns the authority.
// Signature: func(self: borrow<incoming-request>) -> option<string>
func incomingRequestAuthority(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return None as placeholder
	return []component.Val{component.ValOption(nil)}, nil
}

// incomingRequestHeaders returns the headers.
// Signature: func(self: borrow<incoming-request>) -> own<fields>
func incomingRequestHeaders(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValOwn(0)}, nil
}

// incomingRequestConsume consumes the request body.
// Signature: func(self: borrow<incoming-request>) -> result<own<incoming-body>, _>
func incomingRequestConsume(ctx context.Context, args []component.Val) ([]component.Val, error) {
	body := component.ValOwn(0)
	return []component.Val{component.ValResultOk(&body)}, nil
}

// ====================
// Outgoing response functions
// ====================

// outgoingResponseConstructor creates a new outgoing response.
// Signature: func(headers: own<fields>) -> own<outgoing-response>
func outgoingResponseConstructor(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValOwn(0)}, nil
}

// outgoingResponseStatusCode returns the status code.
// Signature: func(self: borrow<outgoing-response>) -> status-code
func outgoingResponseStatusCode(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValU16(200)}, nil
}

// outgoingResponseSetStatusCode sets the status code.
// Signature: func(self: borrow<outgoing-response>, status-code: status-code) -> result<_, _>
func outgoingResponseSetStatusCode(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValResultOk(nil)}, nil
}

// outgoingResponseHeaders returns the headers.
// Signature: func(self: borrow<outgoing-response>) -> own<fields>
func outgoingResponseHeaders(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValOwn(0)}, nil
}

// outgoingResponseBody returns the body.
// Signature: func(self: borrow<outgoing-response>) -> result<own<outgoing-body>, _>
func outgoingResponseBody(ctx context.Context, args []component.Val) ([]component.Val, error) {
	body := component.ValOwn(0)
	return []component.Val{component.ValResultOk(&body)}, nil
}

// ====================
// Incoming response functions
// ====================

// incomingResponseStatus returns the status code.
// Signature: func(self: borrow<incoming-response>) -> status-code
func incomingResponseStatus(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValU16(200)}, nil
}

// incomingResponseHeaders returns the headers.
// Signature: func(self: borrow<incoming-response>) -> own<fields>
func incomingResponseHeaders(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValOwn(0)}, nil
}

// incomingResponseConsume consumes the response body.
// Signature: func(self: borrow<incoming-response>) -> result<own<incoming-body>, _>
func incomingResponseConsume(ctx context.Context, args []component.Val) ([]component.Val, error) {
	body := component.ValOwn(0)
	return []component.Val{component.ValResultOk(&body)}, nil
}

// ====================
// Incoming body functions
// ====================

// incomingBodyStream returns the input stream for reading the body.
// Signature: func(self: borrow<incoming-body>) -> result<own<input-stream>, _>
func incomingBodyStream(ctx context.Context, args []component.Val) ([]component.Val, error) {
	stream := component.ValOwn(0)
	return []component.Val{component.ValResultOk(&stream)}, nil
}

// incomingBodyFinish finishes consuming the body.
// Signature: func(body: own<incoming-body>) -> own<future-trailers>
func incomingBodyFinish(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValOwn(0)}, nil
}

// ====================
// Outgoing body functions
// ====================

// outgoingBodyWrite returns the output stream for writing the body.
// Signature: func(self: borrow<outgoing-body>) -> result<own<output-stream>, _>
func outgoingBodyWrite(ctx context.Context, args []component.Val) ([]component.Val, error) {
	stream := component.ValOwn(0)
	return []component.Val{component.ValResultOk(&stream)}, nil
}

// outgoingBodyFinish finishes writing the body.
// Signature: func(body: own<outgoing-body>, trailers: option<own<fields>>) -> result<_, error-code>
func outgoingBodyFinish(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValResultOk(nil)}, nil
}

// ====================
// Future incoming response functions
// ====================

// futureIncomingResponseGet polls for the response.
// Signature: func(self: borrow<future-incoming-response>) -> option<result<result<own<incoming-response>, error-code>, _>>
func futureIncomingResponseGet(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return None - not ready yet
	return []component.Val{component.ValOption(nil)}, nil
}

// futureIncomingResponseSubscribe returns a pollable for the future.
// Signature: func(self: borrow<future-incoming-response>) -> own<pollable>
func futureIncomingResponseSubscribe(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValOwn(0)}, nil
}

// ====================
// Future trailers functions
// ====================

// futureTrailersGet polls for the trailers.
// Signature: func(self: borrow<future-trailers>) -> option<result<result<option<own<fields>>, error-code>, _>>
func futureTrailersGet(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return None - not ready yet
	return []component.Val{component.ValOption(nil)}, nil
}

// futureTrailersSubscribe returns a pollable for the future.
// Signature: func(self: borrow<future-trailers>) -> own<pollable>
func futureTrailersSubscribe(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValOwn(0)}, nil
}

// ====================
// Response outparam functions
// ====================

// responseOutparamSet sets the response.
// Signature: func(param: own<response-outparam>, response: result<own<outgoing-response>, error-code>)
func responseOutparamSet(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Returns nothing
	return []component.Val{}, nil
}

// ====================
// Request options functions
// ====================

// requestOptionsConstructor creates new request options.
// Signature: func() -> own<request-options>
func requestOptionsConstructor(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValOwn(0)}, nil
}

// requestOptionsConnectTimeout returns the connect timeout.
// Signature: func(self: borrow<request-options>) -> option<duration>
func requestOptionsConnectTimeout(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return None as placeholder
	return []component.Val{component.ValOption(nil)}, nil
}

// requestOptionsSetConnectTimeout sets the connect timeout.
// Signature: func(self: borrow<request-options>, timeout: option<duration>) -> result<_, _>
func requestOptionsSetConnectTimeout(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValResultOk(nil)}, nil
}

// requestOptionsFirstByteTimeout returns the first byte timeout.
// Signature: func(self: borrow<request-options>) -> option<duration>
func requestOptionsFirstByteTimeout(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return None as placeholder
	return []component.Val{component.ValOption(nil)}, nil
}

// requestOptionsSetFirstByteTimeout sets the first byte timeout.
// Signature: func(self: borrow<request-options>, timeout: option<duration>) -> result<_, _>
func requestOptionsSetFirstByteTimeout(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValResultOk(nil)}, nil
}

// requestOptionsBetweenBytesTimeout returns the between bytes timeout.
// Signature: func(self: borrow<request-options>) -> option<duration>
func requestOptionsBetweenBytesTimeout(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return None as placeholder
	return []component.Val{component.ValOption(nil)}, nil
}

// requestOptionsSetBetweenBytesTimeout sets the between bytes timeout.
// Signature: func(self: borrow<request-options>, timeout: option<duration>) -> result<_, _>
func requestOptionsSetBetweenBytesTimeout(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValResultOk(nil)}, nil
}

// ====================
// HTTP error code function
// ====================

// httpErrorCode extracts an error code from an error.
// Signature: func(err: borrow<io-error>) -> option<error-code>
func httpErrorCode(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return None as placeholder
	return []component.Val{component.ValOption(nil)}, nil
}
