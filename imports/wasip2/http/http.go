// imports/wasip2/http/http.go

package http

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero/imports/wasip2/io"
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
	inst.FuncNoType("[method]fields.has", fieldsHas)
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

	return inst.SkipValidation().Build()
}

// ====================
// Helper functions
// ====================

// getOrCreateTable returns the resource table from context or creates operations that don't require one.
func getOrCreateTable(ctx context.Context) *component.ResourceTable {
	return component.ResourceTableFromContext(ctx)
}

// getFields retrieves Fields from the resource table.
func getFields(ctx context.Context, handle uint32) (*Fields, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return nil, fmt.Errorf("no resource table in context")
	}
	entry, err := table.Get(component.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	fields, ok := entry.Rep.(*Fields)
	if !ok {
		return nil, fmt.Errorf("handle %d is not a Fields", handle)
	}
	return fields, nil
}

// getIncomingResponse retrieves IncomingResponse from the resource table.
func getIncomingResponse(ctx context.Context, handle uint32) (*IncomingResponse, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return nil, fmt.Errorf("no resource table in context")
	}
	entry, err := table.Get(component.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	resp, ok := entry.Rep.(*IncomingResponse)
	if !ok {
		return nil, fmt.Errorf("handle %d is not an IncomingResponse", handle)
	}
	return resp, nil
}

// getIncomingBody retrieves IncomingBody from the resource table.
func getIncomingBody(ctx context.Context, handle uint32) (*IncomingBody, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return nil, fmt.Errorf("no resource table in context")
	}
	entry, err := table.Get(component.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	body, ok := entry.Rep.(*IncomingBody)
	if !ok {
		return nil, fmt.Errorf("handle %d is not an IncomingBody", handle)
	}
	return body, nil
}

// getOutgoingBody retrieves OutgoingBody from the resource table.
func getOutgoingBody(ctx context.Context, handle uint32) (*OutgoingBody, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return nil, fmt.Errorf("no resource table in context")
	}
	entry, err := table.Get(component.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	body, ok := entry.Rep.(*OutgoingBody)
	if !ok {
		return nil, fmt.Errorf("handle %d is not an OutgoingBody", handle)
	}
	return body, nil
}

// getRequestOptions retrieves RequestOptions from the resource table.
func getRequestOptions(ctx context.Context, handle uint32) (*RequestOptions, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return nil, fmt.Errorf("no resource table in context")
	}
	entry, err := table.Get(component.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	opts, ok := entry.Rep.(*RequestOptions)
	if !ok {
		return nil, fmt.Errorf("handle %d is not a RequestOptions", handle)
	}
	return opts, nil
}

// getOutgoingRequest retrieves OutgoingRequest from the resource table.
func getOutgoingRequest(ctx context.Context, handle uint32) (*OutgoingRequest, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return nil, fmt.Errorf("no resource table in context")
	}
	entry, err := table.Get(component.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	req, ok := entry.Rep.(*OutgoingRequest)
	if !ok {
		return nil, fmt.Errorf("handle %d is not an OutgoingRequest", handle)
	}
	return req, nil
}

// getIncomingRequest retrieves IncomingRequest from the resource table.
func getIncomingRequest(ctx context.Context, handle uint32) (*IncomingRequest, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return nil, fmt.Errorf("no resource table in context")
	}
	entry, err := table.Get(component.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	req, ok := entry.Rep.(*IncomingRequest)
	if !ok {
		return nil, fmt.Errorf("handle %d is not an IncomingRequest", handle)
	}
	return req, nil
}

// createPollableHandle creates a pollable handle in the resource table.
func createPollableHandle(ctx context.Context, pollable *io.Pollable) component.Val {
	table := getOrCreateTable(ctx)
	if table == nil {
		return component.ValOwn(0)
	}
	handle := table.New(pollable, true)
	return component.ValOwn(uint32(handle))
}

// ====================
// Fields functions
// ====================

// fieldsConstructor creates a new empty fields.
// Signature: func() -> own<fields>
func fieldsConstructor(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return []component.Val{component.ValOwn(0)}, nil
	}
	fields := NewFields()
	handle := table.New(fields, true)
	return []component.Val{component.ValOwn(uint32(handle))}, nil
}

// fieldsFromList creates fields from a list of entries.
// Signature: func(entries: list<tuple<field-key, field-value>>) -> result<own<fields>, header-error>
func fieldsFromList(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		handle := component.ValOwn(0)
		return []component.Val{component.ValResultOk(&handle)}, nil
	}

	fields := NewFields()
	entries := args[0].List()
	for _, entry := range entries {
		tuple := entry.Tuple()
		if len(tuple) >= 2 {
			name := tuple[0].StringVal()
			valueList := tuple[1].List()
			value := make([]byte, len(valueList))
			for i, v := range valueList {
				value[i] = v.U8()
			}
			fields.Append(name, value)
		}
	}

	handle := table.New(fields, true)
	result := component.ValOwn(uint32(handle))
	return []component.Val{component.ValResultOk(&result)}, nil
}

// fieldsGet returns the values for a field name.
// Signature: func(self: borrow<fields>, name: field-key) -> list<field-value>
func fieldsGet(ctx context.Context, args []component.Val) ([]component.Val, error) {
	fields, err := getFields(ctx, args[0].Borrow())
	if err != nil {
		// Return empty list on error
		return []component.Val{component.ValList([]component.Val{})}, nil
	}

	name := args[1].StringVal()
	values := fields.Get(name)

	// Convert [][]byte to list<list<u8>>
	result := make([]component.Val, len(values))
	for i, v := range values {
		bytes := make([]component.Val, len(v))
		for j, b := range v {
			bytes[j] = component.ValU8(b)
		}
		result[i] = component.ValList(bytes)
	}
	return []component.Val{component.ValList(result)}, nil
}

// fieldsHas returns whether a field name exists.
// Signature: func(self: borrow<fields>, name: field-key) -> bool
func fieldsHas(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	name := args[1].StringVal()

	fields, err := getFields(ctx, handle)
	if err != nil {
		return []component.Val{component.ValBool(false)}, nil
	}

	return []component.Val{component.ValBool(fields.Has(name))}, nil
}

// fieldsSet sets the values for a field name.
// Signature: func(self: borrow<fields>, name: field-key, values: list<field-value>) -> result<_, header-error>
func fieldsSet(ctx context.Context, args []component.Val) ([]component.Val, error) {
	fields, err := getFields(ctx, args[0].Borrow())
	if err != nil {
		return []component.Val{component.ValResultOk(nil)}, nil
	}

	name := args[1].StringVal()
	valuesList := args[2].List()

	// Convert list<list<u8>> to [][]byte
	values := make([][]byte, len(valuesList))
	for i, v := range valuesList {
		bytes := v.List()
		values[i] = make([]byte, len(bytes))
		for j, b := range bytes {
			values[i][j] = b.U8()
		}
	}

	fields.Set(name, values)
	return []component.Val{component.ValResultOk(nil)}, nil
}

// fieldsDelete removes a field by name.
// Signature: func(self: borrow<fields>, name: field-key) -> result<_, header-error>
func fieldsDelete(ctx context.Context, args []component.Val) ([]component.Val, error) {
	fields, err := getFields(ctx, args[0].Borrow())
	if err != nil {
		return []component.Val{component.ValResultOk(nil)}, nil
	}

	name := args[1].StringVal()
	fields.Delete(name)
	return []component.Val{component.ValResultOk(nil)}, nil
}

// fieldsAppend appends a value to a field.
// Signature: func(self: borrow<fields>, name: field-key, value: field-value) -> result<_, header-error>
func fieldsAppend(ctx context.Context, args []component.Val) ([]component.Val, error) {
	fields, err := getFields(ctx, args[0].Borrow())
	if err != nil {
		return []component.Val{component.ValResultOk(nil)}, nil
	}

	name := args[1].StringVal()
	valueList := args[2].List()
	value := make([]byte, len(valueList))
	for i, v := range valueList {
		value[i] = v.U8()
	}

	fields.Append(name, value)
	return []component.Val{component.ValResultOk(nil)}, nil
}

// fieldsEntries returns all field entries.
// Signature: func(self: borrow<fields>) -> list<tuple<field-key, field-value>>
func fieldsEntries(ctx context.Context, args []component.Val) ([]component.Val, error) {
	fields, err := getFields(ctx, args[0].Borrow())
	if err != nil {
		return []component.Val{component.ValList([]component.Val{})}, nil
	}

	entries := fields.Entries()
	result := make([]component.Val, 0)
	for _, entry := range entries {
		for _, value := range entry.Values {
			// Convert bytes to list<u8>
			bytes := make([]component.Val, len(value))
			for i, b := range value {
				bytes[i] = component.ValU8(b)
			}
			tuple := component.ValTuple([]component.Val{
				component.ValString(entry.Name),
				component.ValList(bytes),
			})
			result = append(result, tuple)
		}
	}
	return []component.Val{component.ValList(result)}, nil
}

// fieldsClone creates a copy of the fields.
// Signature: func(self: borrow<fields>) -> own<fields>
func fieldsClone(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := getOrCreateTable(ctx)
	fields, err := getFields(ctx, args[0].Borrow())
	if err != nil || table == nil {
		return []component.Val{component.ValOwn(0)}, nil
	}

	clone := fields.Clone()
	handle := table.New(clone, true)
	return []component.Val{component.ValOwn(uint32(handle))}, nil
}

// ====================
// Outgoing request functions
// ====================

// outgoingRequestConstructor creates a new outgoing request.
// Signature: func(headers: own<fields>) -> own<outgoing-request>
func outgoingRequestConstructor(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return []component.Val{component.ValOwn(0)}, nil
	}

	// Get the headers handle and retrieve the Fields
	headersHandle := component.Handle(args[0].Own())
	headersEntry, err := table.Remove(headersHandle)
	var headers *Fields
	if err == nil {
		if h, ok := headersEntry.Rep.(*Fields); ok {
			headers = h
		}
	}
	if headers == nil {
		headers = NewFields()
	}

	req := NewOutgoingRequest(headers)
	handle := table.New(req, true)
	return []component.Val{component.ValOwn(uint32(handle))}, nil
}

// outgoingRequestMethod returns the HTTP method.
// Signature: func(self: borrow<outgoing-request>) -> method
func outgoingRequestMethod(ctx context.Context, args []component.Val) ([]component.Val, error) {
	req, err := getOutgoingRequest(ctx, args[0].Borrow())
	if err != nil {
		return []component.Val{component.ValVariant("get", nil)}, nil
	}
	return []component.Val{component.ValVariant(req.Method().String(), nil)}, nil
}

// outgoingRequestSetMethod sets the HTTP method.
// Signature: func(self: borrow<outgoing-request>, method: method) -> result<_, _>
func outgoingRequestSetMethod(ctx context.Context, args []component.Val) ([]component.Val, error) {
	req, err := getOutgoingRequest(ctx, args[0].Borrow())
	if err != nil {
		return []component.Val{component.ValResultOk(nil)}, nil
	}

	methodName, _ := args[1].Variant()
	method := methodFromString(methodName)
	req.SetMethod(method)
	return []component.Val{component.ValResultOk(nil)}, nil
}

// outgoingRequestPathWithQuery returns the path with query.
// Signature: func(self: borrow<outgoing-request>) -> option<string>
func outgoingRequestPathWithQuery(ctx context.Context, args []component.Val) ([]component.Val, error) {
	req, err := getOutgoingRequest(ctx, args[0].Borrow())
	if err != nil {
		return []component.Val{component.ValOption(nil)}, nil
	}

	path := req.PathWithQuery()
	if path == nil {
		return []component.Val{component.ValOption(nil)}, nil
	}
	pathVal := component.ValString(*path)
	return []component.Val{component.ValOption(&pathVal)}, nil
}

// outgoingRequestSetPathWithQuery sets the path with query.
// Signature: func(self: borrow<outgoing-request>, path: option<string>) -> result<_, _>
func outgoingRequestSetPathWithQuery(ctx context.Context, args []component.Val) ([]component.Val, error) {
	req, err := getOutgoingRequest(ctx, args[0].Borrow())
	if err != nil {
		return []component.Val{component.ValResultOk(nil)}, nil
	}

	optVal := args[1].Option()
	if optVal == nil {
		req.SetPathWithQuery(nil)
	} else {
		path := optVal.StringVal()
		req.SetPathWithQuery(&path)
	}
	return []component.Val{component.ValResultOk(nil)}, nil
}

// outgoingRequestScheme returns the scheme.
// Signature: func(self: borrow<outgoing-request>) -> option<scheme>
func outgoingRequestScheme(ctx context.Context, args []component.Val) ([]component.Val, error) {
	req, err := getOutgoingRequest(ctx, args[0].Borrow())
	if err != nil {
		return []component.Val{component.ValOption(nil)}, nil
	}

	scheme := req.Scheme()
	if scheme == nil {
		return []component.Val{component.ValOption(nil)}, nil
	}

	// Convert scheme to variant
	schemeVal := schemeToVariant(scheme)
	return []component.Val{component.ValOption(&schemeVal)}, nil
}

// outgoingRequestSetScheme sets the scheme.
// Signature: func(self: borrow<outgoing-request>, scheme: option<scheme>) -> result<_, _>
func outgoingRequestSetScheme(ctx context.Context, args []component.Val) ([]component.Val, error) {
	req, err := getOutgoingRequest(ctx, args[0].Borrow())
	if err != nil {
		return []component.Val{component.ValResultOk(nil)}, nil
	}

	optVal := args[1].Option()
	if optVal == nil {
		req.SetScheme(nil)
	} else {
		scheme := schemeFromVariant(*optVal)
		req.SetScheme(&scheme)
	}
	return []component.Val{component.ValResultOk(nil)}, nil
}

// outgoingRequestAuthority returns the authority.
// Signature: func(self: borrow<outgoing-request>) -> option<string>
func outgoingRequestAuthority(ctx context.Context, args []component.Val) ([]component.Val, error) {
	req, err := getOutgoingRequest(ctx, args[0].Borrow())
	if err != nil {
		return []component.Val{component.ValOption(nil)}, nil
	}

	authority := req.Authority()
	if authority == nil {
		return []component.Val{component.ValOption(nil)}, nil
	}
	authorityVal := component.ValString(*authority)
	return []component.Val{component.ValOption(&authorityVal)}, nil
}

// outgoingRequestSetAuthority sets the authority.
// Signature: func(self: borrow<outgoing-request>, authority: option<string>) -> result<_, _>
func outgoingRequestSetAuthority(ctx context.Context, args []component.Val) ([]component.Val, error) {
	req, err := getOutgoingRequest(ctx, args[0].Borrow())
	if err != nil {
		return []component.Val{component.ValResultOk(nil)}, nil
	}

	optVal := args[1].Option()
	if optVal == nil {
		req.SetAuthority(nil)
	} else {
		authority := optVal.StringVal()
		req.SetAuthority(&authority)
	}
	return []component.Val{component.ValResultOk(nil)}, nil
}

// outgoingRequestHeaders returns the headers.
// Signature: func(self: borrow<outgoing-request>) -> own<fields>
func outgoingRequestHeaders(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := getOrCreateTable(ctx)
	req, err := getOutgoingRequest(ctx, args[0].Borrow())
	if err != nil || table == nil {
		return []component.Val{component.ValOwn(0)}, nil
	}

	// Return a handle to the headers (clone them so modifications don't affect request)
	headers := req.Headers()
	if headers == nil {
		headers = NewFields()
	}
	handle := table.New(headers, true)
	return []component.Val{component.ValOwn(uint32(handle))}, nil
}

// outgoingRequestBody returns the body.
// Signature: func(self: borrow<outgoing-request>) -> result<own<outgoing-body>, _>
func outgoingRequestBody(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := getOrCreateTable(ctx)
	req, err := getOutgoingRequest(ctx, args[0].Borrow())
	if err != nil || table == nil {
		body := component.ValOwn(0)
		return []component.Val{component.ValResultOk(&body)}, nil
	}

	body, err := req.Body()
	if err != nil {
		// Body already consumed
		errVal := component.ValVariant("body-already-consumed", nil)
		return []component.Val{component.ValResultError(&errVal)}, nil
	}

	handle := table.New(body, true)
	result := component.ValOwn(uint32(handle))
	return []component.Val{component.ValResultOk(&result)}, nil
}

// methodFromString converts a method name string to Method.
func methodFromString(name string) Method {
	switch name {
	case "get":
		return MethodGet
	case "head":
		return MethodHead
	case "post":
		return MethodPost
	case "put":
		return MethodPut
	case "delete":
		return MethodDelete
	case "connect":
		return MethodConnect
	case "options":
		return MethodOptions
	case "trace":
		return MethodTrace
	case "patch":
		return MethodPatch
	default:
		return MethodOther
	}
}

// schemeToVariant converts a Scheme to a component.Val variant.
func schemeToVariant(s *Scheme) component.Val {
	if s.IsHTTP() {
		return component.ValVariant("HTTP", nil)
	}
	if s.IsHTTPS() {
		return component.ValVariant("HTTPS", nil)
	}
	other := component.ValString(s.Other())
	return component.ValVariant("other", &other)
}

// schemeFromVariant converts a component.Val variant to a Scheme.
func schemeFromVariant(v component.Val) Scheme {
	name, payload := v.Variant()
	switch name {
	case "HTTP", "http":
		return NewSchemeHTTP()
	case "HTTPS", "https":
		return NewSchemeHTTPS()
	case "other":
		if payload != nil {
			return NewSchemeOther(payload.StringVal())
		}
		return NewSchemeHTTP()
	default:
		return NewSchemeHTTP()
	}
}

// ====================
// Incoming request functions
// ====================

// incomingRequestMethod returns the HTTP method.
// Signature: func(self: borrow<incoming-request>) -> method
func incomingRequestMethod(ctx context.Context, args []component.Val) ([]component.Val, error) {
	req, err := getIncomingRequest(ctx, args[0].Borrow())
	if err != nil {
		// Return GET as fallback for invalid handle
		return []component.Val{component.ValVariant("get", nil)}, nil
	}
	return []component.Val{component.ValVariant(req.Method().String(), nil)}, nil
}

// incomingRequestPathWithQuery returns the path with query.
// Signature: func(self: borrow<incoming-request>) -> option<string>
func incomingRequestPathWithQuery(ctx context.Context, args []component.Val) ([]component.Val, error) {
	req, err := getIncomingRequest(ctx, args[0].Borrow())
	if err != nil {
		return []component.Val{component.ValOption(nil)}, nil
	}

	path := req.PathWithQuery()
	if path == nil {
		return []component.Val{component.ValOption(nil)}, nil
	}
	pathVal := component.ValString(*path)
	return []component.Val{component.ValOption(&pathVal)}, nil
}

// incomingRequestScheme returns the scheme.
// Signature: func(self: borrow<incoming-request>) -> option<scheme>
func incomingRequestScheme(ctx context.Context, args []component.Val) ([]component.Val, error) {
	req, err := getIncomingRequest(ctx, args[0].Borrow())
	if err != nil {
		return []component.Val{component.ValOption(nil)}, nil
	}

	scheme := req.Scheme()
	if scheme == nil {
		return []component.Val{component.ValOption(nil)}, nil
	}

	// Convert scheme to variant
	schemeVal := schemeToVariant(scheme)
	return []component.Val{component.ValOption(&schemeVal)}, nil
}

// incomingRequestAuthority returns the authority.
// Signature: func(self: borrow<incoming-request>) -> option<string>
func incomingRequestAuthority(ctx context.Context, args []component.Val) ([]component.Val, error) {
	req, err := getIncomingRequest(ctx, args[0].Borrow())
	if err != nil {
		return []component.Val{component.ValOption(nil)}, nil
	}

	authority := req.Authority()
	if authority == nil {
		return []component.Val{component.ValOption(nil)}, nil
	}
	authorityVal := component.ValString(*authority)
	return []component.Val{component.ValOption(&authorityVal)}, nil
}

// incomingRequestHeaders returns the headers.
// Signature: func(self: borrow<incoming-request>) -> own<fields>
func incomingRequestHeaders(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := getOrCreateTable(ctx)
	req, err := getIncomingRequest(ctx, args[0].Borrow())
	if err != nil || table == nil {
		return []component.Val{component.ValOwn(0)}, nil
	}

	// Return a handle to the headers (clone them so modifications don't affect request)
	headers := req.Headers()
	if headers == nil {
		headers = NewFields()
	}
	handle := table.New(headers, true)
	return []component.Val{component.ValOwn(uint32(handle))}, nil
}

// incomingRequestConsume consumes the request body.
// Signature: func(self: borrow<incoming-request>) -> result<own<incoming-body>, _>
func incomingRequestConsume(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := getOrCreateTable(ctx)
	req, err := getIncomingRequest(ctx, args[0].Borrow())
	if err != nil || table == nil {
		body := component.ValOwn(0)
		return []component.Val{component.ValResultOk(&body)}, nil
	}

	body, consumeErr := req.Consume()
	if consumeErr != nil {
		errVal := component.ValVariant("body-already-consumed", nil)
		return []component.Val{component.ValResultError(&errVal)}, nil
	}

	handle := table.New(body, true)
	result := component.ValOwn(uint32(handle))
	return []component.Val{component.ValResultOk(&result)}, nil
}

// ====================
// Outgoing response functions
// ====================

func getOutgoingResponse(ctx context.Context, handle uint32) (*OutgoingResponse, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return nil, fmt.Errorf("no resource table in context")
	}
	entry, err := table.Get(component.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	resp, ok := entry.Rep.(*OutgoingResponse)
	if !ok {
		return nil, fmt.Errorf("handle %d is not an OutgoingResponse", handle)
	}
	return resp, nil
}

// outgoingResponseConstructor creates a new outgoing response.
// Signature: func(headers: own<fields>) -> own<outgoing-response>
func outgoingResponseConstructor(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return []component.Val{component.ValOwn(0)}, nil
	}

	headersHandle := component.Handle(args[0].Own())
	var headers *Fields
	headersEntry, err := table.Remove(headersHandle)
	if err == nil {
		if h, ok := headersEntry.Rep.(*Fields); ok {
			headers = h
		}
	}
	if headers == nil {
		headers = NewFields()
	}

	resp := NewOutgoingResponse(headers)
	handle := table.New(resp, true)
	return []component.Val{component.ValOwn(uint32(handle))}, nil
}

// outgoingResponseStatusCode returns the status code.
// Signature: func(self: borrow<outgoing-response>) -> status-code
func outgoingResponseStatusCode(ctx context.Context, args []component.Val) ([]component.Val, error) {
	resp, err := getOutgoingResponse(ctx, args[0].Borrow())
	if err != nil {
		return []component.Val{component.ValU16(200)}, nil
	}
	return []component.Val{component.ValU16(resp.StatusCode())}, nil
}

// outgoingResponseSetStatusCode sets the status code.
// Signature: func(self: borrow<outgoing-response>, status-code: status-code) -> result<_, _>
func outgoingResponseSetStatusCode(ctx context.Context, args []component.Val) ([]component.Val, error) {
	resp, err := getOutgoingResponse(ctx, args[0].Borrow())
	if err != nil {
		return []component.Val{component.ValResultOk(nil)}, nil
	}
	resp.SetStatusCode(args[1].U16())
	return []component.Val{component.ValResultOk(nil)}, nil
}

// outgoingResponseHeaders returns the headers.
// Signature: func(self: borrow<outgoing-response>) -> own<fields>
func outgoingResponseHeaders(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := getOrCreateTable(ctx)
	resp, err := getOutgoingResponse(ctx, args[0].Borrow())
	if err != nil || table == nil {
		return []component.Val{component.ValOwn(0)}, nil
	}
	headers := resp.Headers()
	if headers == nil {
		headers = NewFields()
	}
	handle := table.New(headers, true)
	return []component.Val{component.ValOwn(uint32(handle))}, nil
}

// outgoingResponseBody returns the body.
// Signature: func(self: borrow<outgoing-response>) -> result<own<outgoing-body>, _>
func outgoingResponseBody(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := getOrCreateTable(ctx)
	resp, err := getOutgoingResponse(ctx, args[0].Borrow())
	if err != nil || table == nil {
		body := component.ValOwn(0)
		return []component.Val{component.ValResultOk(&body)}, nil
	}

	body, bodyErr := resp.Body()
	if bodyErr != nil {
		errVal := component.ValVariant("body-already-consumed", nil)
		return []component.Val{component.ValResultError(&errVal)}, nil
	}

	handle := table.New(body, true)
	result := component.ValOwn(uint32(handle))
	return []component.Val{component.ValResultOk(&result)}, nil
}

// ====================
// Incoming response functions
// ====================

// incomingResponseStatus returns the status code.
// Signature: func(self: borrow<incoming-response>) -> status-code
func incomingResponseStatus(ctx context.Context, args []component.Val) ([]component.Val, error) {
	resp, err := getIncomingResponse(ctx, args[0].Borrow())
	if err != nil {
		return []component.Val{component.ValU16(200)}, nil
	}
	return []component.Val{component.ValU16(resp.Status())}, nil
}

// incomingResponseHeaders returns the headers.
// Signature: func(self: borrow<incoming-response>) -> own<fields>
func incomingResponseHeaders(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := getOrCreateTable(ctx)
	resp, err := getIncomingResponse(ctx, args[0].Borrow())
	if err != nil || table == nil {
		return []component.Val{component.ValOwn(0)}, nil
	}

	headers := resp.Headers()
	if headers == nil {
		headers = NewFields()
	}
	handle := table.New(headers, true)
	return []component.Val{component.ValOwn(uint32(handle))}, nil
}

// incomingResponseConsume consumes the response body.
// Signature: func(self: borrow<incoming-response>) -> result<own<incoming-body>, _>
func incomingResponseConsume(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := getOrCreateTable(ctx)
	resp, err := getIncomingResponse(ctx, args[0].Borrow())
	if err != nil || table == nil {
		body := component.ValOwn(0)
		return []component.Val{component.ValResultOk(&body)}, nil
	}

	body, err := resp.Consume()
	if err != nil {
		errVal := component.ValVariant("body-already-consumed", nil)
		return []component.Val{component.ValResultError(&errVal)}, nil
	}

	handle := table.New(body, true)
	result := component.ValOwn(uint32(handle))
	return []component.Val{component.ValResultOk(&result)}, nil
}

// ====================
// Incoming body functions
// ====================

// incomingBodyStream returns the input stream for reading the body.
// Signature: func(self: borrow<incoming-body>) -> result<own<input-stream>, _>
func incomingBodyStream(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := getOrCreateTable(ctx)
	body, err := getIncomingBody(ctx, args[0].Borrow())
	if err != nil || table == nil {
		stream := component.ValOwn(0)
		return []component.Val{component.ValResultOk(&stream)}, nil
	}

	stream, err := body.Stream()
	if err != nil {
		errVal := component.ValVariant("stream-already-consumed", nil)
		return []component.Val{component.ValResultError(&errVal)}, nil
	}

	handle := table.New(stream, true)
	result := component.ValOwn(uint32(handle))
	return []component.Val{component.ValResultOk(&result)}, nil
}

// incomingBodyFinish finishes consuming the body.
// Signature: func(body: own<incoming-body>) -> own<future-trailers>
func incomingBodyFinish(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return []component.Val{component.ValOwn(0)}, nil
	}

	// Consume the body handle
	bodyHandle := component.Handle(args[0].Own())
	bodyEntry, err := table.Remove(bodyHandle)
	if err == nil {
		if body, ok := bodyEntry.Rep.(*IncomingBody); ok {
			body.Close()
		}
	}

	// Create a future trailers that's immediately ready with no trailers
	futureTrailers := NewFutureTrailers()
	futureTrailers.ready = true
	handle := table.New(futureTrailers, true)
	return []component.Val{component.ValOwn(uint32(handle))}, nil
}

// ====================
// Outgoing body functions
// ====================

// outgoingBodyWrite returns the output stream for writing the body.
// Signature: func(self: borrow<outgoing-body>) -> result<own<output-stream>, _>
func outgoingBodyWrite(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := getOrCreateTable(ctx)
	body, err := getOutgoingBody(ctx, args[0].Borrow())
	if err != nil || table == nil {
		stream := component.ValOwn(0)
		return []component.Val{component.ValResultOk(&stream)}, nil
	}

	stream, err := body.Write()
	if err != nil {
		errVal := component.ValVariant("stream-already-consumed", nil)
		return []component.Val{component.ValResultError(&errVal)}, nil
	}

	handle := table.New(stream, true)
	result := component.ValOwn(uint32(handle))
	return []component.Val{component.ValResultOk(&result)}, nil
}

// outgoingBodyFinish finishes writing the body.
// Signature: func(body: own<outgoing-body>, trailers: option<own<fields>>) -> result<_, error-code>
func outgoingBodyFinish(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return []component.Val{component.ValResultOk(nil)}, nil
	}

	// Consume the body handle
	bodyHandle := component.Handle(args[0].Own())
	bodyEntry, err := table.Remove(bodyHandle)
	if err == nil {
		if body, ok := bodyEntry.Rep.(*OutgoingBody); ok {
			body.Finish()
		}
	}

	// Consume optional trailers
	trailersOpt := args[1].Option()
	if trailersOpt != nil {
		trailersHandle := component.Handle(trailersOpt.Own())
		table.Remove(trailersHandle)
	}

	return []component.Val{component.ValResultOk(nil)}, nil
}

// ====================
// Future incoming response functions
// ====================

// futureIncomingResponseGet polls for the response.
// Signature: func(self: borrow<future-incoming-response>) -> option<result<result<own<incoming-response>, error-code>, _>>
func futureIncomingResponseGet(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := getOrCreateTable(ctx)
	future, err := getFutureIncomingResponse(ctx, args[0].Borrow())
	if err != nil || table == nil {
		return []component.Val{component.ValOption(nil)}, nil
	}

	resp, errCode, ready := future.Get()
	if !ready {
		return []component.Val{component.ValOption(nil)}, nil
	}

	// Response is ready - build the nested result
	// Result structure: option<result<result<own<incoming-response>, error-code>, _>>
	if errCode != nil {
		// Error case: result<result<..., error-code>, _> -> ok(err(error-code))
		errVal := errorCodeToVariant(*errCode)
		innerResult := component.ValResultError(&errVal)
		outerResult := component.ValResultOk(&innerResult)
		return []component.Val{component.ValOption(&outerResult)}, nil
	}

	// Success case: create handle for incoming response
	respHandle := table.New(resp, true)
	respVal := component.ValOwn(uint32(respHandle))
	innerResult := component.ValResultOk(&respVal)
	outerResult := component.ValResultOk(&innerResult)
	return []component.Val{component.ValOption(&outerResult)}, nil
}

// futureIncomingResponseSubscribe returns a pollable for the future.
// Signature: func(self: borrow<future-incoming-response>) -> own<pollable>
func futureIncomingResponseSubscribe(ctx context.Context, args []component.Val) ([]component.Val, error) {
	future, err := getFutureIncomingResponse(ctx, args[0].Borrow())
	if err != nil {
		return []component.Val{component.ValOwn(0)}, nil
	}

	pollable := future.Subscribe()
	return []component.Val{createPollableHandle(ctx, pollable)}, nil
}

// getFutureIncomingResponse retrieves a FutureIncomingResponse from the resource table.
func getFutureIncomingResponse(ctx context.Context, handle uint32) (*FutureIncomingResponse, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return nil, fmt.Errorf("no resource table in context")
	}
	entry, err := table.Get(component.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	future, ok := entry.Rep.(*FutureIncomingResponse)
	if !ok {
		return nil, fmt.Errorf("handle %d is not a FutureIncomingResponse", handle)
	}
	return future, nil
}

// ====================
// Future trailers functions
// ====================

// getFutureTrailers retrieves FutureTrailers from the resource table.
func getFutureTrailers(ctx context.Context, handle uint32) (*FutureTrailers, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return nil, fmt.Errorf("no resource table in context")
	}
	entry, err := table.Get(component.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	future, ok := entry.Rep.(*FutureTrailers)
	if !ok {
		return nil, fmt.Errorf("handle %d is not a FutureTrailers", handle)
	}
	return future, nil
}

// futureTrailersGet polls for the trailers.
// Signature: func(self: borrow<future-trailers>) -> option<result<result<option<own<fields>>, error-code>, _>>
func futureTrailersGet(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := getOrCreateTable(ctx)
	future, err := getFutureTrailers(ctx, args[0].Borrow())
	if err != nil || table == nil {
		return []component.Val{component.ValOption(nil)}, nil
	}

	if !future.ready {
		return []component.Val{component.ValOption(nil)}, nil
	}

	// Trailers are ready
	if future.err != nil {
		errVal := errorCodeToVariant(*future.err)
		innerResult := component.ValResultError(&errVal)
		outerResult := component.ValResultOk(&innerResult)
		return []component.Val{component.ValOption(&outerResult)}, nil
	}

	// Return option<own<fields>> - typically None for most HTTP responses
	var innerResult component.Val
	if future.trailers != nil {
		handle := table.New(future.trailers, true)
		trailersVal := component.ValOwn(uint32(handle))
		trailersOpt := component.ValOption(&trailersVal)
		innerResult = component.ValResultOk(&trailersOpt)
	} else {
		noneOpt := component.ValOption(nil)
		innerResult = component.ValResultOk(&noneOpt)
	}
	outerResult := component.ValResultOk(&innerResult)
	return []component.Val{component.ValOption(&outerResult)}, nil
}

// futureTrailersSubscribe returns a pollable for the future.
// Signature: func(self: borrow<future-trailers>) -> own<pollable>
func futureTrailersSubscribe(ctx context.Context, args []component.Val) ([]component.Val, error) {
	future, err := getFutureTrailers(ctx, args[0].Borrow())
	if err != nil {
		// Return a ready pollable
		pollable := io.NewReadyPollable()
		return []component.Val{createPollableHandle(ctx, pollable)}, nil
	}

	// Create pollable that checks if future is ready
	pollable := io.NewPollable(
		func() bool { return future.ready },
		func() {}, // No blocking for future trailers
	)
	return []component.Val{createPollableHandle(ctx, pollable)}, nil
}

// ====================
// Response outparam functions
// ====================

// responseOutparamSet sets the response.
// Signature: func(param: own<response-outparam>, response: result<own<outgoing-response>, error-code>)
func responseOutparamSet(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return []component.Val{}, nil
	}

	// Consume the outparam (own handle)
	outparamHandle := component.Handle(args[0].Own())
	outparamEntry, err := table.Remove(outparamHandle)
	if err != nil {
		return []component.Val{}, nil
	}
	outparam, ok := outparamEntry.Rep.(*ResponseOutparam)
	if !ok {
		return []component.Val{}, nil
	}

	// Parse the result<own<outgoing-response>, error-code>
	isOk, okVal, errVal := args[1].Result()

	if isOk {
		// Success: extract outgoing-response
		respHandle := component.Handle(okVal.Own())
		respEntry, err := table.Remove(respHandle)
		if err == nil {
			if resp, ok := respEntry.Rep.(*OutgoingResponse); ok {
				select {
				case outparam.result <- ResponseResult{Response: resp}:
				default:
				}
				return []component.Val{}, nil
			}
		}
	} else {
		// Error: extract error-code variant
		caseName, _ := errVal.Variant()
		errCode := ErrorCode(caseName)
		select {
		case outparam.result <- ResponseResult{Err: &errCode}:
		default:
		}
	}

	return []component.Val{}, nil
}

// ====================
// Request options functions
// ====================

// requestOptionsConstructor creates new request options.
// Signature: func() -> own<request-options>
func requestOptionsConstructor(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return []component.Val{component.ValOwn(0)}, nil
	}

	opts := NewRequestOptions()
	handle := table.New(opts, true)
	return []component.Val{component.ValOwn(uint32(handle))}, nil
}

// requestOptionsConnectTimeout returns the connect timeout.
// Signature: func(self: borrow<request-options>) -> option<duration>
func requestOptionsConnectTimeout(ctx context.Context, args []component.Val) ([]component.Val, error) {
	opts, err := getRequestOptions(ctx, args[0].Borrow())
	if err != nil {
		return []component.Val{component.ValOption(nil)}, nil
	}

	timeout := opts.ConnectTimeout()
	if timeout == nil {
		return []component.Val{component.ValOption(nil)}, nil
	}
	timeoutVal := component.ValU64(*timeout)
	return []component.Val{component.ValOption(&timeoutVal)}, nil
}

// requestOptionsSetConnectTimeout sets the connect timeout.
// Signature: func(self: borrow<request-options>, timeout: option<duration>) -> result<_, _>
func requestOptionsSetConnectTimeout(ctx context.Context, args []component.Val) ([]component.Val, error) {
	opts, err := getRequestOptions(ctx, args[0].Borrow())
	if err != nil {
		return []component.Val{component.ValResultOk(nil)}, nil
	}

	optVal := args[1].Option()
	if optVal == nil {
		opts.SetConnectTimeout(nil)
	} else {
		timeout := optVal.U64()
		opts.SetConnectTimeout(&timeout)
	}
	return []component.Val{component.ValResultOk(nil)}, nil
}

// requestOptionsFirstByteTimeout returns the first byte timeout.
// Signature: func(self: borrow<request-options>) -> option<duration>
func requestOptionsFirstByteTimeout(ctx context.Context, args []component.Val) ([]component.Val, error) {
	opts, err := getRequestOptions(ctx, args[0].Borrow())
	if err != nil {
		return []component.Val{component.ValOption(nil)}, nil
	}

	timeout := opts.FirstByteTimeout()
	if timeout == nil {
		return []component.Val{component.ValOption(nil)}, nil
	}
	timeoutVal := component.ValU64(*timeout)
	return []component.Val{component.ValOption(&timeoutVal)}, nil
}

// requestOptionsSetFirstByteTimeout sets the first byte timeout.
// Signature: func(self: borrow<request-options>, timeout: option<duration>) -> result<_, _>
func requestOptionsSetFirstByteTimeout(ctx context.Context, args []component.Val) ([]component.Val, error) {
	opts, err := getRequestOptions(ctx, args[0].Borrow())
	if err != nil {
		return []component.Val{component.ValResultOk(nil)}, nil
	}

	optVal := args[1].Option()
	if optVal == nil {
		opts.SetFirstByteTimeout(nil)
	} else {
		timeout := optVal.U64()
		opts.SetFirstByteTimeout(&timeout)
	}
	return []component.Val{component.ValResultOk(nil)}, nil
}

// requestOptionsBetweenBytesTimeout returns the between bytes timeout.
// Signature: func(self: borrow<request-options>) -> option<duration>
func requestOptionsBetweenBytesTimeout(ctx context.Context, args []component.Val) ([]component.Val, error) {
	opts, err := getRequestOptions(ctx, args[0].Borrow())
	if err != nil {
		return []component.Val{component.ValOption(nil)}, nil
	}

	timeout := opts.BetweenBytesTimeout()
	if timeout == nil {
		return []component.Val{component.ValOption(nil)}, nil
	}
	timeoutVal := component.ValU64(*timeout)
	return []component.Val{component.ValOption(&timeoutVal)}, nil
}

// requestOptionsSetBetweenBytesTimeout sets the between bytes timeout.
// Signature: func(self: borrow<request-options>, timeout: option<duration>) -> result<_, _>
func requestOptionsSetBetweenBytesTimeout(ctx context.Context, args []component.Val) ([]component.Val, error) {
	opts, err := getRequestOptions(ctx, args[0].Borrow())
	if err != nil {
		return []component.Val{component.ValResultOk(nil)}, nil
	}

	optVal := args[1].Option()
	if optVal == nil {
		opts.SetBetweenBytesTimeout(nil)
	} else {
		timeout := optVal.U64()
		opts.SetBetweenBytesTimeout(&timeout)
	}
	return []component.Val{component.ValResultOk(nil)}, nil
}

// ====================
// HTTP error code function
// ====================

// httpErrorCode extracts an error code from an error.
// Signature: func(err: borrow<io-error>) -> option<error-code>
func httpErrorCode(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return None as placeholder - this is for extracting error codes from io errors
	return []component.Val{component.ValOption(nil)}, nil
}
