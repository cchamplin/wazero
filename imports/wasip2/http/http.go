// imports/wasip2/http/http.go

package http

import (
	"context"
	"errors"
	"fmt"

	"github.com/tetratelabs/wazero/imports/wasip2/io"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
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

// Host-managed resource type singletons. One *ResourceType per host
// resource kind. Impl is nil because these resources are host-owned;
// destruction flows through the existing Destroyable interface on Rep.
var (
	httpPollableResourceType               = &runtime.ResourceType{}
	httpFieldsResourceType                 = &runtime.ResourceType{}
	httpOutgoingRequestResourceType        = &runtime.ResourceType{}
	httpIncomingResponseResourceType       = &runtime.ResourceType{}
	httpIncomingBodyResourceType           = &runtime.ResourceType{}
	httpOutgoingBodyResourceType           = &runtime.ResourceType{}
	httpRequestOptionsResourceType         = &runtime.ResourceType{}
	httpIncomingRequestResourceType        = &runtime.ResourceType{}
	httpOutgoingResponseResourceType       = &runtime.ResourceType{}
	httpFutureIncomingResponseResourceType = &runtime.ResourceType{}
	httpFutureTrailersResourceType         = &runtime.ResourceType{}
	httpInputStreamResourceType            = &runtime.ResourceType{}
	httpOutputStreamResourceType           = &runtime.ResourceType{}
)

// getOrCreateTable returns the resource table from context.
func getOrCreateTable(ctx context.Context) *runtime.Table {
	return component.ResourceTableFromContext(ctx)
}

// getFields retrieves Fields from the resource table.
func getFields(ctx context.Context, handle uint32) (*Fields, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return nil, fmt.Errorf("no resource table in context")
	}
	entry, err := table.Get(runtime.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	resEntry, ok := entry.(*runtime.ResourceHandleEntry)
	if !ok {
		return nil, fmt.Errorf("handle %d is not a resource handle", handle)
	}
	fields, ok := resEntry.Rep.(*Fields)
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
	entry, err := table.Get(runtime.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	resEntry, ok := entry.(*runtime.ResourceHandleEntry)
	if !ok {
		return nil, fmt.Errorf("handle %d is not a resource handle", handle)
	}
	resp, ok := resEntry.Rep.(*IncomingResponse)
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
	entry, err := table.Get(runtime.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	resEntry, ok := entry.(*runtime.ResourceHandleEntry)
	if !ok {
		return nil, fmt.Errorf("handle %d is not a resource handle", handle)
	}
	body, ok := resEntry.Rep.(*IncomingBody)
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
	entry, err := table.Get(runtime.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	resEntry, ok := entry.(*runtime.ResourceHandleEntry)
	if !ok {
		return nil, fmt.Errorf("handle %d is not a resource handle", handle)
	}
	body, ok := resEntry.Rep.(*OutgoingBody)
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
	entry, err := table.Get(runtime.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	resEntry, ok := entry.(*runtime.ResourceHandleEntry)
	if !ok {
		return nil, fmt.Errorf("handle %d is not a resource handle", handle)
	}
	opts, ok := resEntry.Rep.(*RequestOptions)
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
	entry, err := table.Get(runtime.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	resEntry, ok := entry.(*runtime.ResourceHandleEntry)
	if !ok {
		return nil, fmt.Errorf("handle %d is not a resource handle", handle)
	}
	req, ok := resEntry.Rep.(*OutgoingRequest)
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
	entry, err := table.Get(runtime.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	resEntry, ok := entry.(*runtime.ResourceHandleEntry)
	if !ok {
		return nil, fmt.Errorf("handle %d is not a resource handle", handle)
	}
	req, ok := resEntry.Rep.(*IncomingRequest)
	if !ok {
		return nil, fmt.Errorf("handle %d is not an IncomingRequest", handle)
	}
	return req, nil
}

// createPollableHandle creates a pollable handle in the resource table.
func createPollableHandle(ctx context.Context, pollable *io.Pollable) types.Val {
	table := getOrCreateTable(ctx)
	if table == nil {
		return types.ValOwn(0)
	}
	handle, hErr := table.NewResourceHandle(pollable, true, httpPollableResourceType)
	if hErr != nil {
		return types.ValOwn(0)
	}
	return types.ValOwn(uint32(handle))
}

// ====================
// Fields functions
// ====================

// fieldsConstructor creates a new empty fields.
// Signature: func() -> own<fields>
func fieldsConstructor(ctx context.Context, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return []types.Val{types.ValOwn(0)}, nil
	}
	fields := NewFields()
	handle, err := table.NewResourceHandle(fields, true, httpFieldsResourceType)
	if err != nil {
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	return []types.Val{types.ValOwn(uint32(handle))}, nil
}

// fieldsFromList creates fields from a list of entries.
// Signature: func(entries: list<tuple<field-key, field-value>>) -> result<own<fields>, header-error>
func fieldsFromList(ctx context.Context, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		handle := types.ValOwn(0)
		return []types.Val{types.ValResultOk(&handle)}, nil
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

	handle, err := table.NewResourceHandle(fields, true, httpFieldsResourceType)
	if err != nil {
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	result := types.ValOwn(uint32(handle))
	return []types.Val{types.ValResultOk(&result)}, nil
}

// fieldsGet returns the values for a field name.
// Signature: func(self: borrow<fields>, name: field-key) -> list<field-value>
func fieldsGet(ctx context.Context, args []types.Val) ([]types.Val, error) {
	fields, err := getFields(ctx, args[0].Borrow())
	if err != nil {
		// Return empty list on error
		return []types.Val{types.ValList([]types.Val{})}, nil
	}

	name := args[1].StringVal()
	values := fields.Get(name)

	// Convert [][]byte to list<list<u8>>
	result := make([]types.Val, len(values))
	for i, v := range values {
		bytes := make([]types.Val, len(v))
		for j, b := range v {
			bytes[j] = types.ValU8(b)
		}
		result[i] = types.ValList(bytes)
	}
	return []types.Val{types.ValList(result)}, nil
}

// fieldsHas returns whether a field name exists.
// Signature: func(self: borrow<fields>, name: field-key) -> bool
func fieldsHas(ctx context.Context, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()
	name := args[1].StringVal()

	fields, err := getFields(ctx, handle)
	if err != nil {
		return []types.Val{types.ValBool(false)}, nil
	}

	return []types.Val{types.ValBool(fields.Has(name))}, nil
}

// fieldsSet sets the values for a field name.
// Signature: func(self: borrow<fields>, name: field-key, values: list<field-value>) -> result<_, header-error>
func fieldsSet(ctx context.Context, args []types.Val) ([]types.Val, error) {
	fields, err := getFields(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValResultOk(nil)}, nil
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
	return []types.Val{types.ValResultOk(nil)}, nil
}

// fieldsDelete removes a field by name.
// Signature: func(self: borrow<fields>, name: field-key) -> result<_, header-error>
func fieldsDelete(ctx context.Context, args []types.Val) ([]types.Val, error) {
	fields, err := getFields(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	name := args[1].StringVal()
	fields.Delete(name)
	return []types.Val{types.ValResultOk(nil)}, nil
}

// fieldsAppend appends a value to a field.
// Signature: func(self: borrow<fields>, name: field-key, value: field-value) -> result<_, header-error>
func fieldsAppend(ctx context.Context, args []types.Val) ([]types.Val, error) {
	fields, err := getFields(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	name := args[1].StringVal()
	valueList := args[2].List()
	value := make([]byte, len(valueList))
	for i, v := range valueList {
		value[i] = v.U8()
	}

	fields.Append(name, value)
	return []types.Val{types.ValResultOk(nil)}, nil
}

// fieldsEntries returns all field entries.
// Signature: func(self: borrow<fields>) -> list<tuple<field-key, field-value>>
func fieldsEntries(ctx context.Context, args []types.Val) ([]types.Val, error) {
	fields, err := getFields(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValList([]types.Val{})}, nil
	}

	entries := fields.Entries()
	result := make([]types.Val, 0)
	for _, entry := range entries {
		for _, value := range entry.Values {
			// Convert bytes to list<u8>
			bytes := make([]types.Val, len(value))
			for i, b := range value {
				bytes[i] = types.ValU8(b)
			}
			tuple := types.ValTuple([]types.Val{
				types.ValString(entry.Name),
				types.ValList(bytes),
			})
			result = append(result, tuple)
		}
	}
	return []types.Val{types.ValList(result)}, nil
}

// fieldsClone creates a copy of the fields.
// Signature: func(self: borrow<fields>) -> own<fields>
func fieldsClone(ctx context.Context, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	fields, err := getFields(ctx, args[0].Borrow())
	if err != nil || table == nil {
		return []types.Val{types.ValOwn(0)}, nil
	}

	clone := fields.Clone()
	handle, err := table.NewResourceHandle(clone, true, httpFieldsResourceType)
	if err != nil {
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	return []types.Val{types.ValOwn(uint32(handle))}, nil
}

// ====================
// Outgoing request functions
// ====================

// outgoingRequestConstructor creates a new outgoing request.
// Signature: func(headers: own<fields>) -> own<outgoing-request>
func outgoingRequestConstructor(ctx context.Context, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return []types.Val{types.ValOwn(0)}, nil
	}

	// Get the headers handle and retrieve the Fields
	headersHandle := runtime.Handle(args[0].Own())
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
	handle, err := table.NewResourceHandle(req, true, httpOutgoingRequestResourceType)
	if err != nil {
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	return []types.Val{types.ValOwn(uint32(handle))}, nil
}

// outgoingRequestMethod returns the HTTP method.
// Signature: func(self: borrow<outgoing-request>) -> method
func outgoingRequestMethod(ctx context.Context, args []types.Val) ([]types.Val, error) {
	req, err := getOutgoingRequest(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValVariant("get", nil)}, nil
	}
	return []types.Val{types.ValVariant(req.Method().String(), nil)}, nil
}

// outgoingRequestSetMethod sets the HTTP method.
// Signature: func(self: borrow<outgoing-request>, method: method) -> result<_, _>
func outgoingRequestSetMethod(ctx context.Context, args []types.Val) ([]types.Val, error) {
	req, err := getOutgoingRequest(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	methodName, _ := args[1].Variant()
	method := methodFromString(methodName)
	req.SetMethod(method)
	return []types.Val{types.ValResultOk(nil)}, nil
}

// outgoingRequestPathWithQuery returns the path with query.
// Signature: func(self: borrow<outgoing-request>) -> option<string>
func outgoingRequestPathWithQuery(ctx context.Context, args []types.Val) ([]types.Val, error) {
	req, err := getOutgoingRequest(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValOption(nil)}, nil
	}

	path := req.PathWithQuery()
	if path == nil {
		return []types.Val{types.ValOption(nil)}, nil
	}
	pathVal := types.ValString(*path)
	return []types.Val{types.ValOption(&pathVal)}, nil
}

// outgoingRequestSetPathWithQuery sets the path with query.
// Signature: func(self: borrow<outgoing-request>, path: option<string>) -> result<_, _>
func outgoingRequestSetPathWithQuery(ctx context.Context, args []types.Val) ([]types.Val, error) {
	req, err := getOutgoingRequest(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	optVal := args[1].Option()
	if optVal == nil {
		req.SetPathWithQuery(nil)
	} else {
		path := optVal.StringVal()
		req.SetPathWithQuery(&path)
	}
	return []types.Val{types.ValResultOk(nil)}, nil
}

// outgoingRequestScheme returns the scheme.
// Signature: func(self: borrow<outgoing-request>) -> option<scheme>
func outgoingRequestScheme(ctx context.Context, args []types.Val) ([]types.Val, error) {
	req, err := getOutgoingRequest(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValOption(nil)}, nil
	}

	scheme := req.Scheme()
	if scheme == nil {
		return []types.Val{types.ValOption(nil)}, nil
	}

	// Convert scheme to variant
	schemeVal := schemeToVariant(scheme)
	return []types.Val{types.ValOption(&schemeVal)}, nil
}

// outgoingRequestSetScheme sets the scheme.
// Signature: func(self: borrow<outgoing-request>, scheme: option<scheme>) -> result<_, _>
func outgoingRequestSetScheme(ctx context.Context, args []types.Val) ([]types.Val, error) {
	req, err := getOutgoingRequest(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	optVal := args[1].Option()
	if optVal == nil {
		req.SetScheme(nil)
	} else {
		scheme := schemeFromVariant(*optVal)
		req.SetScheme(&scheme)
	}
	return []types.Val{types.ValResultOk(nil)}, nil
}

// outgoingRequestAuthority returns the authority.
// Signature: func(self: borrow<outgoing-request>) -> option<string>
func outgoingRequestAuthority(ctx context.Context, args []types.Val) ([]types.Val, error) {
	req, err := getOutgoingRequest(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValOption(nil)}, nil
	}

	authority := req.Authority()
	if authority == nil {
		return []types.Val{types.ValOption(nil)}, nil
	}
	authorityVal := types.ValString(*authority)
	return []types.Val{types.ValOption(&authorityVal)}, nil
}

// outgoingRequestSetAuthority sets the authority.
// Signature: func(self: borrow<outgoing-request>, authority: option<string>) -> result<_, _>
func outgoingRequestSetAuthority(ctx context.Context, args []types.Val) ([]types.Val, error) {
	req, err := getOutgoingRequest(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	optVal := args[1].Option()
	if optVal == nil {
		req.SetAuthority(nil)
	} else {
		authority := optVal.StringVal()
		req.SetAuthority(&authority)
	}
	return []types.Val{types.ValResultOk(nil)}, nil
}

// outgoingRequestHeaders returns the headers.
// Signature: func(self: borrow<outgoing-request>) -> own<fields>
func outgoingRequestHeaders(ctx context.Context, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	req, err := getOutgoingRequest(ctx, args[0].Borrow())
	if err != nil || table == nil {
		return []types.Val{types.ValOwn(0)}, nil
	}

	// Return a handle to the headers (clone them so modifications don't affect request)
	headers := req.Headers()
	if headers == nil {
		headers = NewFields()
	}
	handle, err := table.NewResourceHandle(headers, true, httpFieldsResourceType)
	if err != nil {
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	return []types.Val{types.ValOwn(uint32(handle))}, nil
}

// outgoingRequestBody returns the body.
// Signature: func(self: borrow<outgoing-request>) -> result<own<outgoing-body>, _>
func outgoingRequestBody(ctx context.Context, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	req, err := getOutgoingRequest(ctx, args[0].Borrow())
	if err != nil || table == nil {
		body := types.ValOwn(0)
		return []types.Val{types.ValResultOk(&body)}, nil
	}

	body, err := req.Body()
	if err != nil {
		// Body already consumed
		errVal := types.ValVariant("body-already-consumed", nil)
		return []types.Val{types.ValResultError(&errVal)}, nil
	}

	handle, err := table.NewResourceHandle(body, true, httpOutgoingBodyResourceType)
	if err != nil {
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	result := types.ValOwn(uint32(handle))
	return []types.Val{types.ValResultOk(&result)}, nil
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

// schemeToVariant converts a Scheme to a types.Val variant.
func schemeToVariant(s *Scheme) types.Val {
	if s.IsHTTP() {
		return types.ValVariant("HTTP", nil)
	}
	if s.IsHTTPS() {
		return types.ValVariant("HTTPS", nil)
	}
	other := types.ValString(s.Other())
	return types.ValVariant("other", &other)
}

// schemeFromVariant converts a types.Val variant to a Scheme.
func schemeFromVariant(v types.Val) Scheme {
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
func incomingRequestMethod(ctx context.Context, args []types.Val) ([]types.Val, error) {
	req, err := getIncomingRequest(ctx, args[0].Borrow())
	if err != nil {
		// Return GET as fallback for invalid handle
		return []types.Val{types.ValVariant("get", nil)}, nil
	}
	return []types.Val{types.ValVariant(req.Method().String(), nil)}, nil
}

// incomingRequestPathWithQuery returns the path with query.
// Signature: func(self: borrow<incoming-request>) -> option<string>
func incomingRequestPathWithQuery(ctx context.Context, args []types.Val) ([]types.Val, error) {
	req, err := getIncomingRequest(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValOption(nil)}, nil
	}

	path := req.PathWithQuery()
	if path == nil {
		return []types.Val{types.ValOption(nil)}, nil
	}
	pathVal := types.ValString(*path)
	return []types.Val{types.ValOption(&pathVal)}, nil
}

// incomingRequestScheme returns the scheme.
// Signature: func(self: borrow<incoming-request>) -> option<scheme>
func incomingRequestScheme(ctx context.Context, args []types.Val) ([]types.Val, error) {
	req, err := getIncomingRequest(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValOption(nil)}, nil
	}

	scheme := req.Scheme()
	if scheme == nil {
		return []types.Val{types.ValOption(nil)}, nil
	}

	// Convert scheme to variant
	schemeVal := schemeToVariant(scheme)
	return []types.Val{types.ValOption(&schemeVal)}, nil
}

// incomingRequestAuthority returns the authority.
// Signature: func(self: borrow<incoming-request>) -> option<string>
func incomingRequestAuthority(ctx context.Context, args []types.Val) ([]types.Val, error) {
	req, err := getIncomingRequest(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValOption(nil)}, nil
	}

	authority := req.Authority()
	if authority == nil {
		return []types.Val{types.ValOption(nil)}, nil
	}
	authorityVal := types.ValString(*authority)
	return []types.Val{types.ValOption(&authorityVal)}, nil
}

// incomingRequestHeaders returns the headers.
// Signature: func(self: borrow<incoming-request>) -> own<fields>
func incomingRequestHeaders(ctx context.Context, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	req, err := getIncomingRequest(ctx, args[0].Borrow())
	if err != nil || table == nil {
		return []types.Val{types.ValOwn(0)}, nil
	}

	// Return a handle to the headers (clone them so modifications don't affect request)
	headers := req.Headers()
	if headers == nil {
		headers = NewFields()
	}
	handle, err := table.NewResourceHandle(headers, true, httpFieldsResourceType)
	if err != nil {
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	return []types.Val{types.ValOwn(uint32(handle))}, nil
}

// incomingRequestConsume consumes the request body.
// Signature: func(self: borrow<incoming-request>) -> result<own<incoming-body>, _>
func incomingRequestConsume(ctx context.Context, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	req, err := getIncomingRequest(ctx, args[0].Borrow())
	if err != nil || table == nil {
		body := types.ValOwn(0)
		return []types.Val{types.ValResultOk(&body)}, nil
	}

	body, consumeErr := req.Consume()
	if consumeErr != nil {
		errVal := types.ValVariant("body-already-consumed", nil)
		return []types.Val{types.ValResultError(&errVal)}, nil
	}

	handle, err := table.NewResourceHandle(body, true, httpIncomingBodyResourceType)
	if err != nil {
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	result := types.ValOwn(uint32(handle))
	return []types.Val{types.ValResultOk(&result)}, nil
}

// ====================
// Outgoing response functions
// ====================

func getOutgoingResponse(ctx context.Context, handle uint32) (*OutgoingResponse, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return nil, fmt.Errorf("no resource table in context")
	}
	entry, err := table.Get(runtime.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	resEntry, ok := entry.(*runtime.ResourceHandleEntry)
	if !ok {
		return nil, fmt.Errorf("handle %d is not a resource handle", handle)
	}
	resp, ok := resEntry.Rep.(*OutgoingResponse)
	if !ok {
		return nil, fmt.Errorf("handle %d is not an OutgoingResponse", handle)
	}
	return resp, nil
}

// outgoingResponseConstructor creates a new outgoing response.
// Signature: func(headers: own<fields>) -> own<outgoing-response>
func outgoingResponseConstructor(ctx context.Context, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return []types.Val{types.ValOwn(0)}, nil
	}

	headersHandle := runtime.Handle(args[0].Own())
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
	handle, err := table.NewResourceHandle(resp, true, httpOutgoingResponseResourceType)
	if err != nil {
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	return []types.Val{types.ValOwn(uint32(handle))}, nil
}

// outgoingResponseStatusCode returns the status code.
// Signature: func(self: borrow<outgoing-response>) -> status-code
func outgoingResponseStatusCode(ctx context.Context, args []types.Val) ([]types.Val, error) {
	resp, err := getOutgoingResponse(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValU16(200)}, nil
	}
	return []types.Val{types.ValU16(resp.StatusCode())}, nil
}

// outgoingResponseSetStatusCode sets the status code.
// Signature: func(self: borrow<outgoing-response>, status-code: status-code) -> result<_, _>
func outgoingResponseSetStatusCode(ctx context.Context, args []types.Val) ([]types.Val, error) {
	resp, err := getOutgoingResponse(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValResultOk(nil)}, nil
	}
	resp.SetStatusCode(args[1].U16())
	return []types.Val{types.ValResultOk(nil)}, nil
}

// outgoingResponseHeaders returns the headers.
// Signature: func(self: borrow<outgoing-response>) -> own<fields>
func outgoingResponseHeaders(ctx context.Context, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	resp, err := getOutgoingResponse(ctx, args[0].Borrow())
	if err != nil || table == nil {
		return []types.Val{types.ValOwn(0)}, nil
	}
	headers := resp.Headers()
	if headers == nil {
		headers = NewFields()
	}
	handle, err := table.NewResourceHandle(headers, true, httpFieldsResourceType)
	if err != nil {
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	return []types.Val{types.ValOwn(uint32(handle))}, nil
}

// outgoingResponseBody returns the body.
// Signature: func(self: borrow<outgoing-response>) -> result<own<outgoing-body>, _>
func outgoingResponseBody(ctx context.Context, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	resp, err := getOutgoingResponse(ctx, args[0].Borrow())
	if err != nil || table == nil {
		body := types.ValOwn(0)
		return []types.Val{types.ValResultOk(&body)}, nil
	}

	body, bodyErr := resp.Body()
	if bodyErr != nil {
		errVal := types.ValVariant("body-already-consumed", nil)
		return []types.Val{types.ValResultError(&errVal)}, nil
	}

	handle, err := table.NewResourceHandle(body, true, httpOutgoingBodyResourceType)
	if err != nil {
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	result := types.ValOwn(uint32(handle))
	return []types.Val{types.ValResultOk(&result)}, nil
}

// ====================
// Incoming response functions
// ====================

// incomingResponseStatus returns the status code.
// Signature: func(self: borrow<incoming-response>) -> status-code
func incomingResponseStatus(ctx context.Context, args []types.Val) ([]types.Val, error) {
	resp, err := getIncomingResponse(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValU16(200)}, nil
	}
	return []types.Val{types.ValU16(resp.Status())}, nil
}

// incomingResponseHeaders returns the headers.
// Signature: func(self: borrow<incoming-response>) -> own<fields>
func incomingResponseHeaders(ctx context.Context, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	resp, err := getIncomingResponse(ctx, args[0].Borrow())
	if err != nil || table == nil {
		return []types.Val{types.ValOwn(0)}, nil
	}

	headers := resp.Headers()
	if headers == nil {
		headers = NewFields()
	}
	handle, err := table.NewResourceHandle(headers, true, httpFieldsResourceType)
	if err != nil {
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	return []types.Val{types.ValOwn(uint32(handle))}, nil
}

// incomingResponseConsume consumes the response body.
// Signature: func(self: borrow<incoming-response>) -> result<own<incoming-body>, _>
func incomingResponseConsume(ctx context.Context, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	resp, err := getIncomingResponse(ctx, args[0].Borrow())
	if err != nil || table == nil {
		body := types.ValOwn(0)
		return []types.Val{types.ValResultOk(&body)}, nil
	}

	body, err := resp.Consume()
	if err != nil {
		errVal := types.ValVariant("body-already-consumed", nil)
		return []types.Val{types.ValResultError(&errVal)}, nil
	}

	handle, err := table.NewResourceHandle(body, true, httpIncomingBodyResourceType)
	if err != nil {
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	result := types.ValOwn(uint32(handle))
	return []types.Val{types.ValResultOk(&result)}, nil
}

// ====================
// Incoming body functions
// ====================

// incomingBodyStream returns the input stream for reading the body.
// Signature: func(self: borrow<incoming-body>) -> result<own<input-stream>, _>
func incomingBodyStream(ctx context.Context, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	body, err := getIncomingBody(ctx, args[0].Borrow())
	if err != nil || table == nil {
		stream := types.ValOwn(0)
		return []types.Val{types.ValResultOk(&stream)}, nil
	}

	stream, err := body.Stream()
	if err != nil {
		errVal := types.ValVariant("stream-already-consumed", nil)
		return []types.Val{types.ValResultError(&errVal)}, nil
	}

	handle, err := table.NewResourceHandle(stream, true, httpInputStreamResourceType)
	if err != nil {
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	result := types.ValOwn(uint32(handle))
	return []types.Val{types.ValResultOk(&result)}, nil
}

// incomingBodyFinish finishes consuming the body.
// Signature: func(body: own<incoming-body>) -> own<future-trailers>
func incomingBodyFinish(ctx context.Context, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return []types.Val{types.ValOwn(0)}, nil
	}

	// Consume the body handle
	bodyHandle := runtime.Handle(args[0].Own())
	bodyEntry, err := table.Remove(bodyHandle)
	if err == nil {
		if body, ok := bodyEntry.Rep.(*IncomingBody); ok {
			body.Close()
		}
	}

	// For simple cases (no trailer support), resolve immediately with no trailers
	ft := NewFutureTrailersReady(nil, nil)
	handle, err := table.NewResourceHandle(ft, true, httpFutureTrailersResourceType)
	if err != nil {
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	return []types.Val{types.ValOwn(uint32(handle))}, nil
}

// ====================
// Outgoing body functions
// ====================

// outgoingBodyWrite returns the output stream for writing the body.
// Signature: func(self: borrow<outgoing-body>) -> result<own<output-stream>, _>
func outgoingBodyWrite(ctx context.Context, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	body, err := getOutgoingBody(ctx, args[0].Borrow())
	if err != nil || table == nil {
		stream := types.ValOwn(0)
		return []types.Val{types.ValResultOk(&stream)}, nil
	}

	stream, err := body.Write()
	if err != nil {
		errVal := types.ValVariant("stream-already-consumed", nil)
		return []types.Val{types.ValResultError(&errVal)}, nil
	}

	handle, err := table.NewResourceHandle(stream, true, httpOutputStreamResourceType)
	if err != nil {
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	result := types.ValOwn(uint32(handle))
	return []types.Val{types.ValResultOk(&result)}, nil
}

// outgoingBodyFinish finishes writing the body.
// Signature: func(body: own<outgoing-body>, trailers: option<own<fields>>) -> result<_, error-code>
func outgoingBodyFinish(ctx context.Context, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	// Consume the body handle
	bodyHandle := runtime.Handle(args[0].Own())
	bodyEntry, err := table.Remove(bodyHandle)
	if err == nil {
		if body, ok := bodyEntry.Rep.(*OutgoingBody); ok {
			body.Finish()
		}
	}

	// Consume optional trailers
	trailersOpt := args[1].Option()
	if trailersOpt != nil {
		trailersHandle := runtime.Handle(trailersOpt.Own())
		table.Remove(trailersHandle)
	}

	return []types.Val{types.ValResultOk(nil)}, nil
}

// ====================
// Future incoming response functions
// ====================

// futureIncomingResponseGet polls for the response.
// Signature: func(self: borrow<future-incoming-response>) -> option<result<result<own<incoming-response>, error-code>, _>>
func futureIncomingResponseGet(ctx context.Context, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	future, err := getFutureIncomingResponse(ctx, args[0].Borrow())
	if err != nil || table == nil {
		return []types.Val{types.ValOption(nil)}, nil
	}

	resp, errCode, ready := future.Get()
	if !ready {
		return []types.Val{types.ValOption(nil)}, nil
	}

	// Response is ready - build the nested result
	// Result structure: option<result<result<own<incoming-response>, error-code>, _>>
	if errCode != nil {
		// Error case: result<result<..., error-code>, _> -> ok(err(error-code))
		errVal := errorCodeToVariant(*errCode)
		innerResult := types.ValResultError(&errVal)
		outerResult := types.ValResultOk(&innerResult)
		return []types.Val{types.ValOption(&outerResult)}, nil
	}

	// Success case: create handle for incoming response
	respHandle, err := table.NewResourceHandle(resp, true, httpIncomingResponseResourceType)
	if err != nil {
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	respVal := types.ValOwn(uint32(respHandle))
	innerResult := types.ValResultOk(&respVal)
	outerResult := types.ValResultOk(&innerResult)
	return []types.Val{types.ValOption(&outerResult)}, nil
}

// futureIncomingResponseSubscribe returns a pollable for the future.
// Signature: func(self: borrow<future-incoming-response>) -> own<pollable>
func futureIncomingResponseSubscribe(ctx context.Context, args []types.Val) ([]types.Val, error) {
	future, err := getFutureIncomingResponse(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValOwn(0)}, nil
	}

	pollable := future.Subscribe()
	return []types.Val{createPollableHandle(ctx, pollable)}, nil
}

// getFutureIncomingResponse retrieves a FutureIncomingResponse from the resource table.
func getFutureIncomingResponse(ctx context.Context, handle uint32) (*FutureIncomingResponse, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return nil, fmt.Errorf("no resource table in context")
	}
	entry, err := table.Get(runtime.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	resEntry, ok := entry.(*runtime.ResourceHandleEntry)
	if !ok {
		return nil, fmt.Errorf("handle %d is not a resource handle", handle)
	}
	future, ok := resEntry.Rep.(*FutureIncomingResponse)
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
	entry, err := table.Get(runtime.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	resEntry, ok := entry.(*runtime.ResourceHandleEntry)
	if !ok {
		return nil, fmt.Errorf("handle %d is not a resource handle", handle)
	}
	future, ok := resEntry.Rep.(*FutureTrailers)
	if !ok {
		return nil, fmt.Errorf("handle %d is not a FutureTrailers", handle)
	}
	return future, nil
}

// futureTrailersGet polls for the trailers.
// Signature: func(self: borrow<future-trailers>) -> option<result<result<option<own<fields>>, error-code>, _>>
func futureTrailersGet(ctx context.Context, args []types.Val) ([]types.Val, error) {
	ft, err := getFutureTrailers(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValOption(nil)}, nil
	}

	// SetConsumed atomically advances state to consumed if currently waiting+ready
	// or done. Returns false if still waiting (channel not closed) or already consumed.
	if !ft.SetConsumed() {
		return []types.Val{types.ValOption(nil)}, nil
	}

	// State has now transitioned to consumed; ft.err and ft.trailers are
	// safe to read since they were set before the done channel was closed
	// (happens-before relationship via the channel close).
	table := getOrCreateTable(ctx)
	if ft.err != nil {
		errVal := errorCodeToVariant(*ft.err)
		innerResult := types.ValResultError(&errVal)
		outerResult := types.ValResultOk(&innerResult)
		return []types.Val{types.ValOption(&outerResult)}, nil
	}

	// Ok case: option<trailers>
	var trailersOpt types.Val
	if ft.trailers != nil && table != nil {
		handle, err := table.NewResourceHandle(ft.trailers, true, httpFieldsResourceType)
		if err != nil {
			return nil, fmt.Errorf("create resource handle: %w", err)
		}
		trailersHandle := types.ValOwn(uint32(handle))
		trailersOpt = types.ValOption(&trailersHandle)
	} else {
		trailersOpt = types.ValOption(nil)
	}

	innerResult := types.ValResultOk(&trailersOpt)
	outerResult := types.ValResultOk(&innerResult)
	return []types.Val{types.ValOption(&outerResult)}, nil
}

// futureTrailersSubscribe returns a pollable for the future.
// Signature: func(self: borrow<future-trailers>) -> own<pollable>
func futureTrailersSubscribe(ctx context.Context, args []types.Val) ([]types.Val, error) {
	ft, err := getFutureTrailers(ctx, args[0].Borrow())
	if err != nil {
		// Return a ready pollable
		pollable := io.NewReadyPollable()
		return []types.Val{createPollableHandle(ctx, pollable)}, nil
	}

	// Create pollable that checks if future is ready and blocks on done channel
	pollable := io.NewPollable(
		func() bool { return ft.IsReady() },
		func() { <-ft.done },
	)
	return []types.Val{createPollableHandle(ctx, pollable)}, nil
}

// ====================
// Response outparam functions
// ====================

// responseOutparamSet sets the response.
// Signature: func(param: own<response-outparam>, response: result<own<outgoing-response>, error-code>)
//
// Per the WASI HTTP types.wit specification (lines 454-467) and the Component
// Model canonical ABI (CanonicalABI.md, lift_own at lines 2215-2220), an own
// handle that does not exist in the table MUST trap. Wazero signals a trap by
// returning a non-nil Go error from the host function — see
// internal/component/component_linker.go createCanonLowerFunc, where errors
// are propagated as panics that the wasm runtime catches as traps.
//
// In particular, when the guest passes a bad own<outgoing-response> handle in
// the Ok branch, this function MUST NOT fabricate a synthetic Err(internal-error)
// and surface it through the outparam channel — that would make a guest bug
// indistinguishable from the guest legitimately calling
// `set(outparam, Err(internal-error))`.
//
// Reference: wasmtime's equivalent in
// crates/wasi-http/src/types_impl.rs (HostResponseOutparam::set) uses
// `self.table().delete(resp)?` which propagates the error as a wasmtime trap.
func responseOutparamSet(ctx context.Context, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		// No resource table at all is a degenerate test/host configuration; in
		// production a table is always present. Returning unit here preserves
		// existing behavior for tests that exercise the function without a table.
		return []types.Val{}, nil
	}

	// Parse the result<own<outgoing-response>, error-code> first.
	// We process the response payload before the outparam handle so that, on
	// the failure path of an invalid outgoing-response handle, the outparam
	// remains in the table and is not orphaned. This matches wasmtime's
	// ordering in HostResponseOutparam::set.
	isOk, okVal, errVal := args[1].Result()

	var deliver ResponseResult
	if isOk {
		// Success branch: lift the own<outgoing-response>. Per CanonicalABI.md
		// lift_own, an invalid handle traps.
		respHandle := runtime.Handle(okVal.Own())
		respEntry, err := table.Remove(respHandle)
		if err != nil {
			return nil, fmt.Errorf("response-outparam.set: invalid outgoing-response handle %d: %w", respHandle, err)
		}
		resp, ok := respEntry.Rep.(*OutgoingResponse)
		if !ok {
			return nil, fmt.Errorf("response-outparam.set: handle %d is not an outgoing-response", respHandle)
		}
		deliver = ResponseResult{Response: resp}
	} else {
		// Error branch: extract the error-code variant case name.
		// Note: payload fields on variants like dns-error, tls-alert-received,
		// internal-error(option<string>) are currently discarded.
		caseName, _ := errVal.Variant()
		errCode := ErrorCode(caseName)
		deliver = ResponseResult{Err: &errCode}
	}

	// Lift the outparam own handle. Invalid handle traps per lift_own.
	outparamHandle := runtime.Handle(args[0].Own())
	outparamEntry, err := table.Remove(outparamHandle)
	if err != nil {
		return nil, fmt.Errorf("response-outparam.set: invalid response-outparam handle %d: %w", outparamHandle, err)
	}
	outparam, ok := outparamEntry.Rep.(*ResponseOutparam)
	if !ok {
		return nil, fmt.Errorf("response-outparam.set: handle %d is not a response-outparam", outparamHandle)
	}

	// Deliver the result through the outparam channel. The channel is buffered
	// with capacity 1; the spec mandates set is called at most once, so the
	// non-blocking select-default protects us against a misbehaving guest that
	// calls set twice without crashing the host.
	select {
	case outparam.result <- deliver:
	default:
	}

	return []types.Val{}, nil
}

// ====================
// Request options functions
// ====================

// requestOptionsConstructor creates new request options.
// Signature: func() -> own<request-options>
func requestOptionsConstructor(ctx context.Context, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return []types.Val{types.ValOwn(0)}, nil
	}

	opts := NewRequestOptions()
	handle, err := table.NewResourceHandle(opts, true, httpRequestOptionsResourceType)
	if err != nil {
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	return []types.Val{types.ValOwn(uint32(handle))}, nil
}

// requestOptionsConnectTimeout returns the connect timeout.
// Signature: func(self: borrow<request-options>) -> option<duration>
func requestOptionsConnectTimeout(ctx context.Context, args []types.Val) ([]types.Val, error) {
	opts, err := getRequestOptions(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValOption(nil)}, nil
	}

	timeout := opts.ConnectTimeout()
	if timeout == nil {
		return []types.Val{types.ValOption(nil)}, nil
	}
	timeoutVal := types.ValU64(*timeout)
	return []types.Val{types.ValOption(&timeoutVal)}, nil
}

// requestOptionsSetConnectTimeout sets the connect timeout.
// Signature: func(self: borrow<request-options>, timeout: option<duration>) -> result<_, _>
func requestOptionsSetConnectTimeout(ctx context.Context, args []types.Val) ([]types.Val, error) {
	opts, err := getRequestOptions(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	optVal := args[1].Option()
	if optVal == nil {
		opts.SetConnectTimeout(nil)
	} else {
		timeout := optVal.U64()
		opts.SetConnectTimeout(&timeout)
	}
	return []types.Val{types.ValResultOk(nil)}, nil
}

// requestOptionsFirstByteTimeout returns the first byte timeout.
// Signature: func(self: borrow<request-options>) -> option<duration>
func requestOptionsFirstByteTimeout(ctx context.Context, args []types.Val) ([]types.Val, error) {
	opts, err := getRequestOptions(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValOption(nil)}, nil
	}

	timeout := opts.FirstByteTimeout()
	if timeout == nil {
		return []types.Val{types.ValOption(nil)}, nil
	}
	timeoutVal := types.ValU64(*timeout)
	return []types.Val{types.ValOption(&timeoutVal)}, nil
}

// requestOptionsSetFirstByteTimeout sets the first byte timeout.
// Signature: func(self: borrow<request-options>, timeout: option<duration>) -> result<_, _>
func requestOptionsSetFirstByteTimeout(ctx context.Context, args []types.Val) ([]types.Val, error) {
	opts, err := getRequestOptions(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	optVal := args[1].Option()
	if optVal == nil {
		opts.SetFirstByteTimeout(nil)
	} else {
		timeout := optVal.U64()
		opts.SetFirstByteTimeout(&timeout)
	}
	return []types.Val{types.ValResultOk(nil)}, nil
}

// requestOptionsBetweenBytesTimeout returns the between bytes timeout.
// Signature: func(self: borrow<request-options>) -> option<duration>
func requestOptionsBetweenBytesTimeout(ctx context.Context, args []types.Val) ([]types.Val, error) {
	opts, err := getRequestOptions(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValOption(nil)}, nil
	}

	timeout := opts.BetweenBytesTimeout()
	if timeout == nil {
		return []types.Val{types.ValOption(nil)}, nil
	}
	timeoutVal := types.ValU64(*timeout)
	return []types.Val{types.ValOption(&timeoutVal)}, nil
}

// requestOptionsSetBetweenBytesTimeout sets the between bytes timeout.
// Signature: func(self: borrow<request-options>, timeout: option<duration>) -> result<_, _>
func requestOptionsSetBetweenBytesTimeout(ctx context.Context, args []types.Val) ([]types.Val, error) {
	opts, err := getRequestOptions(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	optVal := args[1].Option()
	if optVal == nil {
		opts.SetBetweenBytesTimeout(nil)
	} else {
		timeout := optVal.U64()
		opts.SetBetweenBytesTimeout(&timeout)
	}
	return []types.Val{types.ValResultOk(nil)}, nil
}

// ====================
// HTTP error code function
// ====================

// httpErrorCode extracts an error code from an error.
// Signature: func(err: borrow<io-error>) -> option<error-code>
func httpErrorCode(ctx context.Context, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	table := getOrCreateTable(ctx)
	if table == nil {
		return []types.Val{types.ValOption(nil)}, nil
	}

	entry, err := table.Get(runtime.Handle(handle))
	if err != nil {
		return []types.Val{types.ValOption(nil)}, nil
	}

	resEntry, ok := entry.(*runtime.ResourceHandleEntry)
	if !ok {
		return []types.Val{types.ValOption(nil)}, nil
	}
	ioErr, ok := resEntry.Rep.(*io.Error)
	if !ok {
		return []types.Val{types.ValOption(nil)}, nil
	}

	// Unwrap the Go error and check if it's an HTTPError
	var httpErr *HTTPError
	if errors.As(ioErr.Unwrap(), &httpErr) {
		codeVal := errorCodeToVariant(httpErr.Code)
		return []types.Val{types.ValOption(&codeVal)}, nil
	}

	return []types.Val{types.ValOption(nil)}, nil
}
