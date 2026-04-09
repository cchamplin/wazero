// imports/wasip2/http/http.go

// WIT source of truth: debug-vendored/WASI/proposals/http/wit/types.wit
// Package version: wasi:http@0.2.9 (wazero targets wasi:http@0.2.0)
//
package http

import (
	"context"
	"fmt"
	"sync"

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
		if f := getFieldsFromRegistry(rep); f != nil {
			f.Destroy()
		}
		unregisterFields(rep)
	})

	inst.Func("[constructor]fields", fieldsConstructor)
	inst.Func("[static]fields.from-list", fieldsFromList)
	inst.Func("[method]fields.get", fieldsGet)
	inst.Func("[method]fields.has", fieldsHas)
	inst.Func("[method]fields.set", fieldsSet)
	inst.Func("[method]fields.delete", fieldsDelete)
	inst.Func("[method]fields.append", fieldsAppend)
	inst.Func("[method]fields.entries", fieldsEntries)
	inst.Func("[method]fields.clone", fieldsClone)

	// ==================
	// Incoming request resource
	// ==================
	inst.Resource("incoming-request", func(rep uint32) {
		unregisterIncomingRequest(rep)
	})

	inst.Func("[method]incoming-request.method", incomingRequestMethod)
	inst.Func("[method]incoming-request.path-with-query", incomingRequestPathWithQuery)
	inst.Func("[method]incoming-request.scheme", incomingRequestScheme)
	inst.Func("[method]incoming-request.authority", incomingRequestAuthority)
	inst.Func("[method]incoming-request.headers", incomingRequestHeaders)
	inst.Func("[method]incoming-request.consume", incomingRequestConsume)

	// ==================
	// Outgoing request resource
	// ==================
	inst.Resource("outgoing-request", func(rep uint32) {
		if r := getOutgoingRequestFromRegistry(rep); r != nil {
			r.Destroy()
		}
		unregisterOutgoingRequest(rep)
	})

	inst.Func("[constructor]outgoing-request", outgoingRequestConstructor)
	inst.Func("[method]outgoing-request.method", outgoingRequestMethod)
	inst.Func("[method]outgoing-request.set-method", outgoingRequestSetMethod)
	inst.Func("[method]outgoing-request.path-with-query", outgoingRequestPathWithQuery)
	inst.Func("[method]outgoing-request.set-path-with-query", outgoingRequestSetPathWithQuery)
	inst.Func("[method]outgoing-request.scheme", outgoingRequestScheme)
	inst.Func("[method]outgoing-request.set-scheme", outgoingRequestSetScheme)
	inst.Func("[method]outgoing-request.authority", outgoingRequestAuthority)
	inst.Func("[method]outgoing-request.set-authority", outgoingRequestSetAuthority)
	inst.Func("[method]outgoing-request.headers", outgoingRequestHeaders)
	inst.Func("[method]outgoing-request.body", outgoingRequestBody)

	// ==================
	// Incoming response resource
	// ==================
	inst.Resource("incoming-response", func(rep uint32) {
		if r := getIncomingResponseFromRegistry(rep); r != nil {
			r.Destroy()
		}
		unregisterIncomingResponse(rep)
	})

	inst.Func("[method]incoming-response.status", incomingResponseStatus)
	inst.Func("[method]incoming-response.headers", incomingResponseHeaders)
	inst.Func("[method]incoming-response.consume", incomingResponseConsume)

	// ==================
	// Outgoing response resource
	// ==================
	inst.Resource("outgoing-response", func(rep uint32) {
		unregisterOutgoingResponse(rep)
	})

	inst.Func("[constructor]outgoing-response", outgoingResponseConstructor)
	inst.Func("[method]outgoing-response.status-code", outgoingResponseStatusCode)
	inst.Func("[method]outgoing-response.set-status-code", outgoingResponseSetStatusCode)
	inst.Func("[method]outgoing-response.headers", outgoingResponseHeaders)
	inst.Func("[method]outgoing-response.body", outgoingResponseBody)

	// ==================
	// Incoming body resource
	// ==================
	inst.Resource("incoming-body", func(rep uint32) {
		if b := getIncomingBodyFromRegistry(rep); b != nil {
			b.Destroy()
		}
		unregisterIncomingBody(rep)
	})

	inst.Func("[method]incoming-body.stream", incomingBodyStream)
	inst.Func("[static]incoming-body.finish", incomingBodyFinish)

	// ==================
	// Outgoing body resource
	// ==================
	inst.Resource("outgoing-body", func(rep uint32) {
		if b := getOutgoingBodyFromRegistry(rep); b != nil {
			b.Destroy()
		}
		unregisterOutgoingBody(rep)
	})

	inst.Func("[method]outgoing-body.write", outgoingBodyWrite)
	inst.Func("[static]outgoing-body.finish", outgoingBodyFinish)

	// ==================
	// Future incoming response resource
	// ==================
	inst.Resource("future-incoming-response", func(rep uint32) {
		if f := getFutureIncomingResponseFromRegistry(rep); f != nil {
			f.Destroy()
		}
		unregisterFutureIncomingResponse(rep)
	})

	inst.Func("[method]future-incoming-response.get", futureIncomingResponseGet)
	inst.Func("[method]future-incoming-response.subscribe", futureIncomingResponseSubscribe)

	// ==================
	// Future trailers resource
	// ==================
	inst.Resource("future-trailers", func(rep uint32) {
		unregisterFutureTrailers(rep)
	})

	inst.Func("[method]future-trailers.get", futureTrailersGet)
	inst.Func("[method]future-trailers.subscribe", futureTrailersSubscribe)

	// ==================
	// Response outparam resource
	// ==================
	inst.Resource("response-outparam", func(rep uint32) {
		if p := getResponseOutparamFromRegistry(rep); p != nil {
			p.Destroy()
		}
		unregisterResponseOutparam(rep)
	})

	inst.Func("[static]response-outparam.set", responseOutparamSet)

	// ==================
	// Request options resource
	// ==================
	inst.Resource("request-options", func(rep uint32) {
		unregisterRequestOptions(rep)
	})

	inst.Func("[constructor]request-options", requestOptionsConstructor)
	inst.Func("[method]request-options.connect-timeout", requestOptionsConnectTimeout)
	inst.Func("[method]request-options.set-connect-timeout", requestOptionsSetConnectTimeout)
	inst.Func("[method]request-options.first-byte-timeout", requestOptionsFirstByteTimeout)
	inst.Func("[method]request-options.set-first-byte-timeout", requestOptionsSetFirstByteTimeout)
	inst.Func("[method]request-options.between-bytes-timeout", requestOptionsBetweenBytesTimeout)
	inst.Func("[method]request-options.set-between-bytes-timeout", requestOptionsSetBetweenBytesTimeout)

	// ==================
	// Functions
	// ==================
	inst.Func("http-error-code", httpErrorCode)

	return inst.SkipValidation().Build()
}

// ====================
// Helper functions
// ====================

// ===========================
// Per-module u32 registries for HTTP resource types
// ===========================

// Fields registry
var (
	fieldsRegistryMu sync.Mutex
	fieldsRegistry   []*Fields
	fieldsFreelist   []uint32
)

func registerFields(f *Fields) uint32 {
	fieldsRegistryMu.Lock()
	defer fieldsRegistryMu.Unlock()
	if n := len(fieldsFreelist); n > 0 {
		id := fieldsFreelist[n-1]
		fieldsFreelist = fieldsFreelist[:n-1]
		fieldsRegistry[id] = f
		return id
	}
	id := uint32(len(fieldsRegistry))
	fieldsRegistry = append(fieldsRegistry, f)
	return id
}

func getFieldsFromRegistry(id uint32) *Fields {
	fieldsRegistryMu.Lock()
	defer fieldsRegistryMu.Unlock()
	if int(id) >= len(fieldsRegistry) {
		return nil
	}
	return fieldsRegistry[id]
}

func unregisterFields(id uint32) {
	fieldsRegistryMu.Lock()
	defer fieldsRegistryMu.Unlock()
	if int(id) < len(fieldsRegistry) && fieldsRegistry[id] != nil {
		fieldsRegistry[id] = nil
		fieldsFreelist = append(fieldsFreelist, id)
	}
}

// OutgoingRequest registry
var (
	outgoingRequestRegistryMu sync.Mutex
	outgoingRequestRegistry   []*OutgoingRequest
	outgoingRequestFreelist   []uint32
)

func registerOutgoingRequest(r *OutgoingRequest) uint32 {
	outgoingRequestRegistryMu.Lock()
	defer outgoingRequestRegistryMu.Unlock()
	if n := len(outgoingRequestFreelist); n > 0 {
		id := outgoingRequestFreelist[n-1]
		outgoingRequestFreelist = outgoingRequestFreelist[:n-1]
		outgoingRequestRegistry[id] = r
		return id
	}
	id := uint32(len(outgoingRequestRegistry))
	outgoingRequestRegistry = append(outgoingRequestRegistry, r)
	return id
}

func getOutgoingRequestFromRegistry(id uint32) *OutgoingRequest {
	outgoingRequestRegistryMu.Lock()
	defer outgoingRequestRegistryMu.Unlock()
	if int(id) >= len(outgoingRequestRegistry) {
		return nil
	}
	return outgoingRequestRegistry[id]
}

func unregisterOutgoingRequest(id uint32) {
	outgoingRequestRegistryMu.Lock()
	defer outgoingRequestRegistryMu.Unlock()
	if int(id) < len(outgoingRequestRegistry) && outgoingRequestRegistry[id] != nil {
		outgoingRequestRegistry[id] = nil
		outgoingRequestFreelist = append(outgoingRequestFreelist, id)
	}
}

// IncomingResponse registry
var (
	incomingResponseRegistryMu sync.Mutex
	incomingResponseRegistry   []*IncomingResponse
	incomingResponseFreelist   []uint32
)

func registerIncomingResponse(r *IncomingResponse) uint32 {
	incomingResponseRegistryMu.Lock()
	defer incomingResponseRegistryMu.Unlock()
	if n := len(incomingResponseFreelist); n > 0 {
		id := incomingResponseFreelist[n-1]
		incomingResponseFreelist = incomingResponseFreelist[:n-1]
		incomingResponseRegistry[id] = r
		return id
	}
	id := uint32(len(incomingResponseRegistry))
	incomingResponseRegistry = append(incomingResponseRegistry, r)
	return id
}

func getIncomingResponseFromRegistry(id uint32) *IncomingResponse {
	incomingResponseRegistryMu.Lock()
	defer incomingResponseRegistryMu.Unlock()
	if int(id) >= len(incomingResponseRegistry) {
		return nil
	}
	return incomingResponseRegistry[id]
}

func unregisterIncomingResponse(id uint32) {
	incomingResponseRegistryMu.Lock()
	defer incomingResponseRegistryMu.Unlock()
	if int(id) < len(incomingResponseRegistry) && incomingResponseRegistry[id] != nil {
		incomingResponseRegistry[id] = nil
		incomingResponseFreelist = append(incomingResponseFreelist, id)
	}
}

// IncomingBody registry
var (
	incomingBodyRegistryMu sync.Mutex
	incomingBodyRegistry   []*IncomingBody
	incomingBodyFreelist   []uint32
)

func registerIncomingBody(b *IncomingBody) uint32 {
	incomingBodyRegistryMu.Lock()
	defer incomingBodyRegistryMu.Unlock()
	if n := len(incomingBodyFreelist); n > 0 {
		id := incomingBodyFreelist[n-1]
		incomingBodyFreelist = incomingBodyFreelist[:n-1]
		incomingBodyRegistry[id] = b
		return id
	}
	id := uint32(len(incomingBodyRegistry))
	incomingBodyRegistry = append(incomingBodyRegistry, b)
	return id
}

func getIncomingBodyFromRegistry(id uint32) *IncomingBody {
	incomingBodyRegistryMu.Lock()
	defer incomingBodyRegistryMu.Unlock()
	if int(id) >= len(incomingBodyRegistry) {
		return nil
	}
	return incomingBodyRegistry[id]
}

func unregisterIncomingBody(id uint32) {
	incomingBodyRegistryMu.Lock()
	defer incomingBodyRegistryMu.Unlock()
	if int(id) < len(incomingBodyRegistry) && incomingBodyRegistry[id] != nil {
		incomingBodyRegistry[id] = nil
		incomingBodyFreelist = append(incomingBodyFreelist, id)
	}
}

// OutgoingBody registry
var (
	outgoingBodyRegistryMu sync.Mutex
	outgoingBodyRegistry   []*OutgoingBody
	outgoingBodyFreelist   []uint32
)

func registerOutgoingBody(b *OutgoingBody) uint32 {
	outgoingBodyRegistryMu.Lock()
	defer outgoingBodyRegistryMu.Unlock()
	if n := len(outgoingBodyFreelist); n > 0 {
		id := outgoingBodyFreelist[n-1]
		outgoingBodyFreelist = outgoingBodyFreelist[:n-1]
		outgoingBodyRegistry[id] = b
		return id
	}
	id := uint32(len(outgoingBodyRegistry))
	outgoingBodyRegistry = append(outgoingBodyRegistry, b)
	return id
}

func getOutgoingBodyFromRegistry(id uint32) *OutgoingBody {
	outgoingBodyRegistryMu.Lock()
	defer outgoingBodyRegistryMu.Unlock()
	if int(id) >= len(outgoingBodyRegistry) {
		return nil
	}
	return outgoingBodyRegistry[id]
}

func unregisterOutgoingBody(id uint32) {
	outgoingBodyRegistryMu.Lock()
	defer outgoingBodyRegistryMu.Unlock()
	if int(id) < len(outgoingBodyRegistry) && outgoingBodyRegistry[id] != nil {
		outgoingBodyRegistry[id] = nil
		outgoingBodyFreelist = append(outgoingBodyFreelist, id)
	}
}

// RequestOptions registry
var (
	requestOptionsRegistryMu sync.Mutex
	requestOptionsRegistry   []*RequestOptions
	requestOptionsFreelist   []uint32
)

func registerRequestOptions(o *RequestOptions) uint32 {
	requestOptionsRegistryMu.Lock()
	defer requestOptionsRegistryMu.Unlock()
	if n := len(requestOptionsFreelist); n > 0 {
		id := requestOptionsFreelist[n-1]
		requestOptionsFreelist = requestOptionsFreelist[:n-1]
		requestOptionsRegistry[id] = o
		return id
	}
	id := uint32(len(requestOptionsRegistry))
	requestOptionsRegistry = append(requestOptionsRegistry, o)
	return id
}

func getRequestOptionsFromRegistry(id uint32) *RequestOptions {
	requestOptionsRegistryMu.Lock()
	defer requestOptionsRegistryMu.Unlock()
	if int(id) >= len(requestOptionsRegistry) {
		return nil
	}
	return requestOptionsRegistry[id]
}

func unregisterRequestOptions(id uint32) {
	requestOptionsRegistryMu.Lock()
	defer requestOptionsRegistryMu.Unlock()
	if int(id) < len(requestOptionsRegistry) && requestOptionsRegistry[id] != nil {
		requestOptionsRegistry[id] = nil
		requestOptionsFreelist = append(requestOptionsFreelist, id)
	}
}

// IncomingRequest registry
var (
	incomingRequestRegistryMu sync.Mutex
	incomingRequestRegistry   []*IncomingRequest
	incomingRequestFreelist   []uint32
)

func registerIncomingRequest(r *IncomingRequest) uint32 {
	incomingRequestRegistryMu.Lock()
	defer incomingRequestRegistryMu.Unlock()
	if n := len(incomingRequestFreelist); n > 0 {
		id := incomingRequestFreelist[n-1]
		incomingRequestFreelist = incomingRequestFreelist[:n-1]
		incomingRequestRegistry[id] = r
		return id
	}
	id := uint32(len(incomingRequestRegistry))
	incomingRequestRegistry = append(incomingRequestRegistry, r)
	return id
}

func getIncomingRequestFromRegistry(id uint32) *IncomingRequest {
	incomingRequestRegistryMu.Lock()
	defer incomingRequestRegistryMu.Unlock()
	if int(id) >= len(incomingRequestRegistry) {
		return nil
	}
	return incomingRequestRegistry[id]
}

func unregisterIncomingRequest(id uint32) {
	incomingRequestRegistryMu.Lock()
	defer incomingRequestRegistryMu.Unlock()
	if int(id) < len(incomingRequestRegistry) && incomingRequestRegistry[id] != nil {
		incomingRequestRegistry[id] = nil
		incomingRequestFreelist = append(incomingRequestFreelist, id)
	}
}

// OutgoingResponse registry
var (
	outgoingResponseRegistryMu sync.Mutex
	outgoingResponseRegistry   []*OutgoingResponse
	outgoingResponseFreelist   []uint32
)

func registerOutgoingResponse(r *OutgoingResponse) uint32 {
	outgoingResponseRegistryMu.Lock()
	defer outgoingResponseRegistryMu.Unlock()
	if n := len(outgoingResponseFreelist); n > 0 {
		id := outgoingResponseFreelist[n-1]
		outgoingResponseFreelist = outgoingResponseFreelist[:n-1]
		outgoingResponseRegistry[id] = r
		return id
	}
	id := uint32(len(outgoingResponseRegistry))
	outgoingResponseRegistry = append(outgoingResponseRegistry, r)
	return id
}

func getOutgoingResponseFromRegistry(id uint32) *OutgoingResponse {
	outgoingResponseRegistryMu.Lock()
	defer outgoingResponseRegistryMu.Unlock()
	if int(id) >= len(outgoingResponseRegistry) {
		return nil
	}
	return outgoingResponseRegistry[id]
}

func unregisterOutgoingResponse(id uint32) {
	outgoingResponseRegistryMu.Lock()
	defer outgoingResponseRegistryMu.Unlock()
	if int(id) < len(outgoingResponseRegistry) && outgoingResponseRegistry[id] != nil {
		outgoingResponseRegistry[id] = nil
		outgoingResponseFreelist = append(outgoingResponseFreelist, id)
	}
}

// FutureIncomingResponse registry
var (
	futureIncomingResponseRegistryMu sync.Mutex
	futureIncomingResponseRegistry   []*FutureIncomingResponse
	futureIncomingResponseFreelist   []uint32
)

func registerFutureIncomingResponse(f *FutureIncomingResponse) uint32 {
	futureIncomingResponseRegistryMu.Lock()
	defer futureIncomingResponseRegistryMu.Unlock()
	if n := len(futureIncomingResponseFreelist); n > 0 {
		id := futureIncomingResponseFreelist[n-1]
		futureIncomingResponseFreelist = futureIncomingResponseFreelist[:n-1]
		futureIncomingResponseRegistry[id] = f
		return id
	}
	id := uint32(len(futureIncomingResponseRegistry))
	futureIncomingResponseRegistry = append(futureIncomingResponseRegistry, f)
	return id
}

func getFutureIncomingResponseFromRegistry(id uint32) *FutureIncomingResponse {
	futureIncomingResponseRegistryMu.Lock()
	defer futureIncomingResponseRegistryMu.Unlock()
	if int(id) >= len(futureIncomingResponseRegistry) {
		return nil
	}
	return futureIncomingResponseRegistry[id]
}

func unregisterFutureIncomingResponse(id uint32) {
	futureIncomingResponseRegistryMu.Lock()
	defer futureIncomingResponseRegistryMu.Unlock()
	if int(id) < len(futureIncomingResponseRegistry) && futureIncomingResponseRegistry[id] != nil {
		futureIncomingResponseRegistry[id] = nil
		futureIncomingResponseFreelist = append(futureIncomingResponseFreelist, id)
	}
}

// FutureTrailers registry
var (
	futureTrailersRegistryMu sync.Mutex
	futureTrailersRegistry   []*FutureTrailers
	futureTrailersFreelist   []uint32
)

func registerFutureTrailers(f *FutureTrailers) uint32 {
	futureTrailersRegistryMu.Lock()
	defer futureTrailersRegistryMu.Unlock()
	if n := len(futureTrailersFreelist); n > 0 {
		id := futureTrailersFreelist[n-1]
		futureTrailersFreelist = futureTrailersFreelist[:n-1]
		futureTrailersRegistry[id] = f
		return id
	}
	id := uint32(len(futureTrailersRegistry))
	futureTrailersRegistry = append(futureTrailersRegistry, f)
	return id
}

func getFutureTrailersFromRegistry(id uint32) *FutureTrailers {
	futureTrailersRegistryMu.Lock()
	defer futureTrailersRegistryMu.Unlock()
	if int(id) >= len(futureTrailersRegistry) {
		return nil
	}
	return futureTrailersRegistry[id]
}

func unregisterFutureTrailers(id uint32) {
	futureTrailersRegistryMu.Lock()
	defer futureTrailersRegistryMu.Unlock()
	if int(id) < len(futureTrailersRegistry) && futureTrailersRegistry[id] != nil {
		futureTrailersRegistry[id] = nil
		futureTrailersFreelist = append(futureTrailersFreelist, id)
	}
}

// ResponseOutparam registry
var (
	responseOutparamRegistryMu sync.Mutex
	responseOutparamRegistry   []*ResponseOutparam
	responseOutparamFreelist   []uint32
)

func registerResponseOutparam(p *ResponseOutparam) uint32 {
	responseOutparamRegistryMu.Lock()
	defer responseOutparamRegistryMu.Unlock()
	if n := len(responseOutparamFreelist); n > 0 {
		id := responseOutparamFreelist[n-1]
		responseOutparamFreelist = responseOutparamFreelist[:n-1]
		responseOutparamRegistry[id] = p
		return id
	}
	id := uint32(len(responseOutparamRegistry))
	responseOutparamRegistry = append(responseOutparamRegistry, p)
	return id
}

func getResponseOutparamFromRegistry(id uint32) *ResponseOutparam {
	responseOutparamRegistryMu.Lock()
	defer responseOutparamRegistryMu.Unlock()
	if int(id) >= len(responseOutparamRegistry) {
		return nil
	}
	return responseOutparamRegistry[id]
}

func unregisterResponseOutparam(id uint32) {
	responseOutparamRegistryMu.Lock()
	defer responseOutparamRegistryMu.Unlock()
	if int(id) < len(responseOutparamRegistry) && responseOutparamRegistry[id] != nil {
		responseOutparamRegistry[id] = nil
		responseOutparamFreelist = append(responseOutparamFreelist, id)
	}
}

// Host-managed resource type singletons. One *ResourceType per host
// resource kind. Impl is nil because these resources are host-owned;
// destruction flows through ResourceType.HostDestructor.
var (
	httpPollableResourceType = &runtime.ResourceType{
		HostDestructor: func(rep uint32) error { io.UnregisterPollable(rep); return nil },
	}
	httpFieldsResourceType = &runtime.ResourceType{
		HostDestructor: func(rep uint32) error {
			if f := getFieldsFromRegistry(rep); f != nil {
				f.Destroy()
			}
			unregisterFields(rep)
			return nil
		},
	}
	httpOutgoingRequestResourceType = &runtime.ResourceType{
		HostDestructor: func(rep uint32) error {
			if r := getOutgoingRequestFromRegistry(rep); r != nil {
				r.Destroy()
			}
			unregisterOutgoingRequest(rep)
			return nil
		},
	}
	httpIncomingResponseResourceType = &runtime.ResourceType{
		HostDestructor: func(rep uint32) error {
			if r := getIncomingResponseFromRegistry(rep); r != nil {
				r.Destroy()
			}
			unregisterIncomingResponse(rep)
			return nil
		},
	}
	httpIncomingBodyResourceType = &runtime.ResourceType{
		HostDestructor: func(rep uint32) error {
			if b := getIncomingBodyFromRegistry(rep); b != nil {
				b.Destroy()
			}
			unregisterIncomingBody(rep)
			return nil
		},
	}
	httpOutgoingBodyResourceType = &runtime.ResourceType{
		HostDestructor: func(rep uint32) error {
			if b := getOutgoingBodyFromRegistry(rep); b != nil {
				b.Destroy()
			}
			unregisterOutgoingBody(rep)
			return nil
		},
	}
	httpRequestOptionsResourceType = &runtime.ResourceType{
		HostDestructor: func(rep uint32) error { unregisterRequestOptions(rep); return nil },
	}
	httpIncomingRequestResourceType = &runtime.ResourceType{
		HostDestructor: func(rep uint32) error { unregisterIncomingRequest(rep); return nil },
	}
	httpOutgoingResponseResourceType = &runtime.ResourceType{
		HostDestructor: func(rep uint32) error { unregisterOutgoingResponse(rep); return nil },
	}
	httpFutureIncomingResponseResourceType = &runtime.ResourceType{
		HostDestructor: func(rep uint32) error {
			if f := getFutureIncomingResponseFromRegistry(rep); f != nil {
				f.Destroy()
			}
			unregisterFutureIncomingResponse(rep)
			return nil
		},
	}
	httpFutureTrailersResourceType = &runtime.ResourceType{
		HostDestructor: func(rep uint32) error { unregisterFutureTrailers(rep); return nil },
	}
	httpInputStreamResourceType = &runtime.ResourceType{
		HostDestructor: func(rep uint32) error {
			if s := io.GetInputStream(rep); s != nil {
				s.Destroy()
			}
			io.UnregisterInputStream(rep)
			return nil
		},
	}
	httpOutputStreamResourceType = &runtime.ResourceType{
		HostDestructor: func(rep uint32) error {
			if s := io.GetOutputStream(rep); s != nil {
				s.Destroy()
			}
			io.UnregisterOutputStream(rep)
			return nil
		},
	}
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
	f := getFieldsFromRegistry(resEntry.Rep)
	if f == nil {
		return nil, fmt.Errorf("handle %d: Fields not found in registry (rep=%d)", handle, resEntry.Rep)
	}
	return f, nil
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
	r := getIncomingResponseFromRegistry(resEntry.Rep)
	if r == nil {
		return nil, fmt.Errorf("handle %d: IncomingResponse not found in registry (rep=%d)", handle, resEntry.Rep)
	}
	return r, nil
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
	b := getIncomingBodyFromRegistry(resEntry.Rep)
	if b == nil {
		return nil, fmt.Errorf("handle %d: IncomingBody not found in registry (rep=%d)", handle, resEntry.Rep)
	}
	return b, nil
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
	b := getOutgoingBodyFromRegistry(resEntry.Rep)
	if b == nil {
		return nil, fmt.Errorf("handle %d: OutgoingBody not found in registry (rep=%d)", handle, resEntry.Rep)
	}
	return b, nil
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
	o := getRequestOptionsFromRegistry(resEntry.Rep)
	if o == nil {
		return nil, fmt.Errorf("handle %d: RequestOptions not found in registry (rep=%d)", handle, resEntry.Rep)
	}
	return o, nil
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
	r := getOutgoingRequestFromRegistry(resEntry.Rep)
	if r == nil {
		return nil, fmt.Errorf("handle %d: OutgoingRequest not found in registry (rep=%d)", handle, resEntry.Rep)
	}
	return r, nil
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
	r := getIncomingRequestFromRegistry(resEntry.Rep)
	if r == nil {
		return nil, fmt.Errorf("handle %d: IncomingRequest not found in registry (rep=%d)", handle, resEntry.Rep)
	}
	return r, nil
}

// createPollableHandle creates a pollable handle in the resource table.
func createPollableHandle(ctx context.Context, pollable *io.Pollable) types.Val {
	table := getOrCreateTable(ctx)
	if table == nil {
		return types.ValOwn(0)
	}
	id := io.RegisterPollable(pollable)
	handle, hErr := table.NewResourceHandle(id, true, httpPollableResourceType)
	if hErr != nil {
		io.UnregisterPollable(id)
		return types.ValOwn(0)
	}
	return types.ValOwn(uint32(handle))
}

// ====================
// Fields functions
// ====================

// fieldsConstructor creates a new empty fields.
// Signature: func() -> own<fields>
func fieldsConstructor(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return []types.Val{types.ValOwn(0)}, nil
	}
	fields := NewFields()
	id := registerFields(fields)
	handle, err := table.NewResourceHandle(id, true, httpFieldsResourceType)
	if err != nil {
		unregisterFields(id)
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	return []types.Val{types.ValOwn(uint32(handle))}, nil
}

// fieldsFromList creates fields from a list of entries.
// Signature: func(entries: list<tuple<field-key, field-value>>) -> result<own<fields>, header-error>
func fieldsFromList(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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

	id := registerFields(fields)
	handle, err := table.NewResourceHandle(id, true, httpFieldsResourceType)
	if err != nil {
		unregisterFields(id)
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	result := types.ValOwn(uint32(handle))
	return []types.Val{types.ValResultOk(&result)}, nil
}

// fieldsGet returns the values for a field name.
// Signature: func(self: borrow<fields>, name: field-key) -> list<field-value>
func fieldsGet(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
func fieldsHas(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
func fieldsSet(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
func fieldsDelete(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
func fieldsAppend(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
func fieldsEntries(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
func fieldsClone(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	fields, err := getFields(ctx, args[0].Borrow())
	if err != nil || table == nil {
		return []types.Val{types.ValOwn(0)}, nil
	}

	clone := fields.Clone()
	id := registerFields(clone)
	handle, err := table.NewResourceHandle(id, true, httpFieldsResourceType)
	if err != nil {
		unregisterFields(id)
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	return []types.Val{types.ValOwn(uint32(handle))}, nil
}

// ====================
// Outgoing request functions
// ====================

// outgoingRequestConstructor creates a new outgoing request.
// Signature: func(headers: own<fields>) -> own<outgoing-request>
func outgoingRequestConstructor(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return []types.Val{types.ValOwn(0)}, nil
	}

	// Get the headers handle and retrieve the Fields
	headersHandle := runtime.Handle(args[0].Own())
	headersEntry, _ := table.Remove(headersHandle)
	headers := NewFields()
	if headersEntry != nil {
		if f := getFieldsFromRegistry(headersEntry.Rep); f != nil {
			headers = f
		}
	}

	req := NewOutgoingRequest(headers)
	id := registerOutgoingRequest(req)
	handle, err := table.NewResourceHandle(id, true, httpOutgoingRequestResourceType)
	if err != nil {
		unregisterOutgoingRequest(id)
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	return []types.Val{types.ValOwn(uint32(handle))}, nil
}

// outgoingRequestMethod returns the HTTP method.
// Signature: func(self: borrow<outgoing-request>) -> method
func outgoingRequestMethod(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	req, err := getOutgoingRequest(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValVariant("get", nil)}, nil
	}
	return []types.Val{types.ValVariant(req.Method().String(), nil)}, nil
}

// outgoingRequestSetMethod sets the HTTP method.
// Signature: func(self: borrow<outgoing-request>, method: method) -> result<_, _>
func outgoingRequestSetMethod(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
func outgoingRequestPathWithQuery(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
func outgoingRequestSetPathWithQuery(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
func outgoingRequestScheme(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
func outgoingRequestSetScheme(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
func outgoingRequestAuthority(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
func outgoingRequestSetAuthority(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
func outgoingRequestHeaders(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
	fid := registerFields(headers)
	handle, err := table.NewResourceHandle(fid, true, httpFieldsResourceType)
	if err != nil {
		unregisterFields(fid)
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	return []types.Val{types.ValOwn(uint32(handle))}, nil
}

// outgoingRequestBody returns the body.
// Signature: func(self: borrow<outgoing-request>) -> result<own<outgoing-body>, _>
func outgoingRequestBody(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	req, err := getOutgoingRequest(ctx, args[0].Borrow())
	if err != nil || table == nil {
		body := types.ValOwn(0)
		return []types.Val{types.ValResultOk(&body)}, nil
	}

	body, bodyErr := req.Body()
	if bodyErr != nil {
		errVal := types.ValVariant("body-already-consumed", nil)
		return []types.Val{types.ValResultError(&errVal)}, nil
	}

	bid := registerOutgoingBody(body)
	handle, err := table.NewResourceHandle(bid, true, httpOutgoingBodyResourceType)
	if err != nil {
		unregisterOutgoingBody(bid)
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
func incomingRequestMethod(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	req, err := getIncomingRequest(ctx, args[0].Borrow())
	if err != nil {
		// Return GET as fallback for invalid handle
		return []types.Val{types.ValVariant("get", nil)}, nil
	}
	return []types.Val{types.ValVariant(req.Method().String(), nil)}, nil
}

// incomingRequestPathWithQuery returns the path with query.
// Signature: func(self: borrow<incoming-request>) -> option<string>
func incomingRequestPathWithQuery(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
func incomingRequestScheme(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
func incomingRequestAuthority(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
func incomingRequestHeaders(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
	fid := registerFields(headers)
	handle, err := table.NewResourceHandle(fid, true, httpFieldsResourceType)
	if err != nil {
		unregisterFields(fid)
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	return []types.Val{types.ValOwn(uint32(handle))}, nil
}

// incomingRequestConsume consumes the request body.
// Signature: func(self: borrow<incoming-request>) -> result<own<incoming-body>, _>
func incomingRequestConsume(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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

	bid := registerIncomingBody(body)
	handle, err := table.NewResourceHandle(bid, true, httpIncomingBodyResourceType)
	if err != nil {
		unregisterIncomingBody(bid)
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
	r := getOutgoingResponseFromRegistry(resEntry.Rep)
	if r == nil {
		return nil, fmt.Errorf("handle %d: OutgoingResponse not found in registry (rep=%d)", handle, resEntry.Rep)
	}
	return r, nil
}

// outgoingResponseConstructor creates a new outgoing response.
// Signature: func(headers: own<fields>) -> own<outgoing-response>
func outgoingResponseConstructor(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return []types.Val{types.ValOwn(0)}, nil
	}

	headersHandle := runtime.Handle(args[0].Own())
	headersEntry, _ := table.Remove(headersHandle)
	headers := NewFields()
	if headersEntry != nil {
		if f := getFieldsFromRegistry(headersEntry.Rep); f != nil {
			headers = f
		}
	}

	resp := NewOutgoingResponse(headers)
	id := registerOutgoingResponse(resp)
	handle, err := table.NewResourceHandle(id, true, httpOutgoingResponseResourceType)
	if err != nil {
		unregisterOutgoingResponse(id)
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	return []types.Val{types.ValOwn(uint32(handle))}, nil
}

// outgoingResponseStatusCode returns the status code.
// Signature: func(self: borrow<outgoing-response>) -> status-code
func outgoingResponseStatusCode(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	resp, err := getOutgoingResponse(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValU16(200)}, nil
	}
	return []types.Val{types.ValU16(resp.StatusCode())}, nil
}

// outgoingResponseSetStatusCode sets the status code.
// Signature: func(self: borrow<outgoing-response>, status-code: status-code) -> result<_, _>
func outgoingResponseSetStatusCode(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	resp, err := getOutgoingResponse(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValResultOk(nil)}, nil
	}
	resp.SetStatusCode(args[1].U16())
	return []types.Val{types.ValResultOk(nil)}, nil
}

// outgoingResponseHeaders returns the headers.
// Signature: func(self: borrow<outgoing-response>) -> own<fields>
func outgoingResponseHeaders(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	resp, err := getOutgoingResponse(ctx, args[0].Borrow())
	if err != nil || table == nil {
		return []types.Val{types.ValOwn(0)}, nil
	}
	headers := resp.Headers()
	if headers == nil {
		headers = NewFields()
	}
	fid := registerFields(headers)
	handle, err := table.NewResourceHandle(fid, true, httpFieldsResourceType)
	if err != nil {
		unregisterFields(fid)
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	return []types.Val{types.ValOwn(uint32(handle))}, nil
}

// outgoingResponseBody returns the body.
// Signature: func(self: borrow<outgoing-response>) -> result<own<outgoing-body>, _>
func outgoingResponseBody(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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

	bid := registerOutgoingBody(body)
	handle, err := table.NewResourceHandle(bid, true, httpOutgoingBodyResourceType)
	if err != nil {
		unregisterOutgoingBody(bid)
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
func incomingResponseStatus(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	resp, err := getIncomingResponse(ctx, args[0].Borrow())
	if err != nil {
		return []types.Val{types.ValU16(200)}, nil
	}
	return []types.Val{types.ValU16(resp.Status())}, nil
}

// incomingResponseHeaders returns the headers.
// Signature: func(self: borrow<incoming-response>) -> own<fields>
func incomingResponseHeaders(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	resp, err := getIncomingResponse(ctx, args[0].Borrow())
	if err != nil || table == nil {
		return []types.Val{types.ValOwn(0)}, nil
	}

	headers := resp.Headers()
	if headers == nil {
		headers = NewFields()
	}
	fid := registerFields(headers)
	handle, err := table.NewResourceHandle(fid, true, httpFieldsResourceType)
	if err != nil {
		unregisterFields(fid)
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	return []types.Val{types.ValOwn(uint32(handle))}, nil
}

// incomingResponseConsume consumes the response body.
// Signature: func(self: borrow<incoming-response>) -> result<own<incoming-body>, _>
func incomingResponseConsume(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	resp, err := getIncomingResponse(ctx, args[0].Borrow())
	if err != nil || table == nil {
		body := types.ValOwn(0)
		return []types.Val{types.ValResultOk(&body)}, nil
	}

	body, consumeErr := resp.Consume()
	if consumeErr != nil {
		errVal := types.ValVariant("body-already-consumed", nil)
		return []types.Val{types.ValResultError(&errVal)}, nil
	}

	bid := registerIncomingBody(body)
	handle, err := table.NewResourceHandle(bid, true, httpIncomingBodyResourceType)
	if err != nil {
		unregisterIncomingBody(bid)
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
func incomingBodyStream(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	body, err := getIncomingBody(ctx, args[0].Borrow())
	if err != nil || table == nil {
		stream := types.ValOwn(0)
		return []types.Val{types.ValResultOk(&stream)}, nil
	}

	stream, streamErr := body.Stream()
	if streamErr != nil {
		errVal := types.ValVariant("stream-already-consumed", nil)
		return []types.Val{types.ValResultError(&errVal)}, nil
	}

	sid := io.RegisterInputStream(stream)
	handle, err := table.NewResourceHandle(sid, true, httpInputStreamResourceType)
	if err != nil {
		io.UnregisterInputStream(sid)
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	result := types.ValOwn(uint32(handle))
	return []types.Val{types.ValResultOk(&result)}, nil
}

// incomingBodyFinish finishes consuming the body.
// Signature: func(body: own<incoming-body>) -> own<future-trailers>
func incomingBodyFinish(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return []types.Val{types.ValOwn(0)}, nil
	}

	// Consume the body handle
	bodyHandle := runtime.Handle(args[0].Own())
	bodyEntry, _ := table.Remove(bodyHandle)
	if bodyEntry != nil {
		if b := getIncomingBodyFromRegistry(bodyEntry.Rep); b != nil {
			b.Close()
		}
		unregisterIncomingBody(bodyEntry.Rep)
	}

	// For simple cases (no trailer support), resolve immediately with no trailers
	ft := NewFutureTrailersReady(nil, nil)
	ftid := registerFutureTrailers(ft)
	handle, err := table.NewResourceHandle(ftid, true, httpFutureTrailersResourceType)
	if err != nil {
		unregisterFutureTrailers(ftid)
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	return []types.Val{types.ValOwn(uint32(handle))}, nil
}

// ====================
// Outgoing body functions
// ====================

// outgoingBodyWrite returns the output stream for writing the body.
// Signature: func(self: borrow<outgoing-body>) -> result<own<output-stream>, _>
func outgoingBodyWrite(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	body, err := getOutgoingBody(ctx, args[0].Borrow())
	if err != nil || table == nil {
		stream := types.ValOwn(0)
		return []types.Val{types.ValResultOk(&stream)}, nil
	}

	stream, streamErr := body.Write()
	if streamErr != nil {
		errVal := types.ValVariant("stream-already-consumed", nil)
		return []types.Val{types.ValResultError(&errVal)}, nil
	}

	sid := io.RegisterOutputStream(stream)
	handle, err := table.NewResourceHandle(sid, true, httpOutputStreamResourceType)
	if err != nil {
		io.UnregisterOutputStream(sid)
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	result := types.ValOwn(uint32(handle))
	return []types.Val{types.ValResultOk(&result)}, nil
}

// outgoingBodyFinish finishes writing the body.
// Signature: func(body: own<outgoing-body>, trailers: option<own<fields>>) -> result<_, error-code>
func outgoingBodyFinish(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return []types.Val{types.ValResultOk(nil)}, nil
	}

	// Consume the body handle
	bodyHandle := runtime.Handle(args[0].Own())
	bodyEntry, _ := table.Remove(bodyHandle)
	if bodyEntry != nil {
		if b := getOutgoingBodyFromRegistry(bodyEntry.Rep); b != nil {
			b.Finish()
		}
		unregisterOutgoingBody(bodyEntry.Rep)
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
func futureIncomingResponseGet(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
	rid := registerIncomingResponse(resp)
	respHandle, err := table.NewResourceHandle(rid, true, httpIncomingResponseResourceType)
	if err != nil {
		unregisterIncomingResponse(rid)
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	respVal := types.ValOwn(uint32(respHandle))
	innerResult := types.ValResultOk(&respVal)
	outerResult := types.ValResultOk(&innerResult)
	return []types.Val{types.ValOption(&outerResult)}, nil
}

// futureIncomingResponseSubscribe returns a pollable for the future.
// Signature: func(self: borrow<future-incoming-response>) -> own<pollable>
func futureIncomingResponseSubscribe(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
	f := getFutureIncomingResponseFromRegistry(resEntry.Rep)
	if f == nil {
		return nil, fmt.Errorf("handle %d: FutureIncomingResponse not found in registry (rep=%d)", handle, resEntry.Rep)
	}
	return f, nil
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
	f := getFutureTrailersFromRegistry(resEntry.Rep)
	if f == nil {
		return nil, fmt.Errorf("handle %d: FutureTrailers not found in registry (rep=%d)", handle, resEntry.Rep)
	}
	return f, nil
}

// futureTrailersGet polls for the trailers.
// Signature: func(self: borrow<future-trailers>) -> option<result<result<option<own<fields>>, error-code>, _>>
func futureTrailersGet(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
		fid := registerFields(ft.trailers)
		handle, err := table.NewResourceHandle(fid, true, httpFieldsResourceType)
		if err != nil {
			unregisterFields(fid)
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
func futureTrailersSubscribe(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
func responseOutparamSet(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
		var resp *OutgoingResponse
		if respEntry != nil {
			resp = getOutgoingResponseFromRegistry(respEntry.Rep)
			unregisterOutgoingResponse(respEntry.Rep)
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
	if outparamEntry != nil {
		if outparam := getResponseOutparamFromRegistry(outparamEntry.Rep); outparam != nil {
			outparam.mu.Lock()
			if !outparam.closed {
				outparam.result <- deliver
			}
			outparam.mu.Unlock()
		}
		unregisterResponseOutparam(outparamEntry.Rep)
	}

	return []types.Val{}, nil
}

// ====================
// Request options functions
// ====================

// requestOptionsConstructor creates new request options.
// Signature: func() -> own<request-options>
func requestOptionsConstructor(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return []types.Val{types.ValOwn(0)}, nil
	}

	opts := NewRequestOptions()
	oid := registerRequestOptions(opts)
	handle, err := table.NewResourceHandle(oid, true, httpRequestOptionsResourceType)
	if err != nil {
		unregisterRequestOptions(oid)
		return nil, fmt.Errorf("create resource handle: %w", err)
	}
	return []types.Val{types.ValOwn(uint32(handle))}, nil
}

// requestOptionsConnectTimeout returns the connect timeout.
// Signature: func(self: borrow<request-options>) -> option<duration>
func requestOptionsConnectTimeout(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
func requestOptionsSetConnectTimeout(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
func requestOptionsFirstByteTimeout(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
func requestOptionsSetFirstByteTimeout(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
func requestOptionsBetweenBytesTimeout(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
func requestOptionsSetBetweenBytesTimeout(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
func httpErrorCode(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
	ioErr := io.GetError(resEntry.Rep)
	if ioErr == nil {
		return []types.Val{types.ValOption(nil)}, nil
	}
	goErr := ioErr.Unwrap()
	if goErr == nil {
		return []types.Val{types.ValOption(nil)}, nil
	}
	if httpErr, ok := goErr.(*HTTPError); ok {
		errVal := types.ValEnum(string(httpErr.Code))
		return []types.Val{types.ValOption(&errVal)}, nil
	}
	return []types.Val{types.ValOption(nil)}, nil
}
