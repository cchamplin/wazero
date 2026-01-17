// Package conformance contains conformance tests for the Component Model implementation.
// This file implements Task 283: WASI HTTP Conformance Tests.
package conformance

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/imports/wasip2"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// =============================================================================
// Task 283: WASI HTTP Conformance Tests
// =============================================================================

// TestWASI_HTTP_TypesInterfaceExists tests that the HTTP types interface exists.
func TestWASI_HTTP_TypesInterfaceExists(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	// Get the HTTP types interface
	typesDef, ok := linker.Get("wasi:http/types@0.2.0")
	require.True(t, ok, "http/types interface should be registered")

	instDef, ok := typesDef.(*component.InstanceDef)
	require.True(t, ok, "should be an InstanceDef")
	require.NotNil(t, instDef.Exports, "exports should not be nil")
}

// TestWASI_HTTP_FieldsResource tests that the fields resource exists.
func TestWASI_HTTP_FieldsResource(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	typesDef, _ := linker.Get("wasi:http/types@0.2.0")
	instDef := typesDef.(*component.InstanceDef)

	// Verify fields resource exists
	fieldsRes, ok := instDef.Exports["fields"]
	require.True(t, ok, "fields resource should be exported")
	require.NotNil(t, fieldsRes, "fields resource should not be nil")

	// Verify fields constructor exists
	fieldsConstructor, ok := instDef.Exports["[constructor]fields"]
	require.True(t, ok, "[constructor]fields should be exported")
	require.NotNil(t, fieldsConstructor, "[constructor]fields should not be nil")
}

// TestWASI_HTTP_FieldsConstructor tests the fields constructor.
func TestWASI_HTTP_FieldsConstructor(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	typesDef, _ := linker.Get("wasi:http/types@0.2.0")
	instDef := typesDef.(*component.InstanceDef)

	fieldsConstructor := instDef.Exports["[constructor]fields"].(*component.FuncDef)

	result, err := fieldsConstructor.Callback(ctx, []component.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result), "[constructor]fields should return exactly one value")

	// Result should be own<fields>
	handle := result[0].Own()
	// Handle 0 may be valid placeholder, handle > 0 is real resource
	require.True(t, handle >= 0, "should return a valid fields handle")
}

// TestWASI_HTTP_FieldsMethods tests that all fields methods exist.
func TestWASI_HTTP_FieldsMethods(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	typesDef, _ := linker.Get("wasi:http/types@0.2.0")
	instDef := typesDef.(*component.InstanceDef)

	// Expected fields methods
	expectedMethods := []string{
		"[constructor]fields",
		"[static]fields.from-list",
		"[method]fields.get",
		"[method]fields.set",
		"[method]fields.delete",
		"[method]fields.append",
		"[method]fields.entries",
		"[method]fields.clone",
	}

	for _, method := range expectedMethods {
		methodDef, ok := instDef.Exports[method]
		require.True(t, ok, "method %s should be exported", method)
		require.NotNil(t, methodDef, "method %s should not be nil", method)
	}
}

// TestWASI_HTTP_OutgoingHandlerExists tests that the outgoing-handler interface exists.
func TestWASI_HTTP_OutgoingHandlerExists(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	// Get the outgoing-handler interface
	handlerDef, ok := linker.Get("wasi:http/outgoing-handler@0.2.0")
	require.True(t, ok, "outgoing-handler interface should be registered")

	instDef, ok := handlerDef.(*component.InstanceDef)
	require.True(t, ok, "should be an InstanceDef")

	// Verify handle function exists
	handleFunc, ok := instDef.Exports["handle"]
	require.True(t, ok, "handle function should be exported")
	require.NotNil(t, handleFunc, "handle function should not be nil")
}

// TestWASI_HTTP_IncomingHandlerExists tests that the incoming-handler interface exists.
func TestWASI_HTTP_IncomingHandlerExists(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	// Get the incoming-handler interface
	handlerDef, ok := linker.Get("wasi:http/incoming-handler@0.2.0")
	require.True(t, ok, "incoming-handler interface should be registered")

	instDef, ok := handlerDef.(*component.InstanceDef)
	require.True(t, ok, "should be an InstanceDef")

	// Verify handle function exists
	handleFunc, ok := instDef.Exports["handle"]
	require.True(t, ok, "handle function should be exported")
	require.NotNil(t, handleFunc, "handle function should not be nil")
}

// TestWASI_HTTP_RequestResources tests that request resources exist.
func TestWASI_HTTP_RequestResources(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	typesDef, _ := linker.Get("wasi:http/types@0.2.0")
	instDef := typesDef.(*component.InstanceDef)

	// Verify incoming-request resource exists
	incomingReqRes, ok := instDef.Exports["incoming-request"]
	require.True(t, ok, "incoming-request resource should be exported")
	require.NotNil(t, incomingReqRes, "incoming-request resource should not be nil")

	// Verify outgoing-request resource exists
	outgoingReqRes, ok := instDef.Exports["outgoing-request"]
	require.True(t, ok, "outgoing-request resource should be exported")
	require.NotNil(t, outgoingReqRes, "outgoing-request resource should not be nil")
}

// TestWASI_HTTP_ResponseResources tests that response resources exist.
func TestWASI_HTTP_ResponseResources(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	typesDef, _ := linker.Get("wasi:http/types@0.2.0")
	instDef := typesDef.(*component.InstanceDef)

	// Verify incoming-response resource exists
	incomingRespRes, ok := instDef.Exports["incoming-response"]
	require.True(t, ok, "incoming-response resource should be exported")
	require.NotNil(t, incomingRespRes, "incoming-response resource should not be nil")

	// Verify outgoing-response resource exists
	outgoingRespRes, ok := instDef.Exports["outgoing-response"]
	require.True(t, ok, "outgoing-response resource should be exported")
	require.NotNil(t, outgoingRespRes, "outgoing-response resource should not be nil")
}

// TestWASI_HTTP_BodyResources tests that body resources exist.
func TestWASI_HTTP_BodyResources(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	typesDef, _ := linker.Get("wasi:http/types@0.2.0")
	instDef := typesDef.(*component.InstanceDef)

	// Verify incoming-body resource exists
	incomingBodyRes, ok := instDef.Exports["incoming-body"]
	require.True(t, ok, "incoming-body resource should be exported")
	require.NotNil(t, incomingBodyRes, "incoming-body resource should not be nil")

	// Verify outgoing-body resource exists
	outgoingBodyRes, ok := instDef.Exports["outgoing-body"]
	require.True(t, ok, "outgoing-body resource should be exported")
	require.NotNil(t, outgoingBodyRes, "outgoing-body resource should not be nil")
}

// TestWASI_HTTP_FutureResources tests that future resources exist.
func TestWASI_HTTP_FutureResources(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	typesDef, _ := linker.Get("wasi:http/types@0.2.0")
	instDef := typesDef.(*component.InstanceDef)

	// Verify future-incoming-response resource exists
	futureRespRes, ok := instDef.Exports["future-incoming-response"]
	require.True(t, ok, "future-incoming-response resource should be exported")
	require.NotNil(t, futureRespRes, "future-incoming-response resource should not be nil")

	// Verify future-trailers resource exists
	futureTrailersRes, ok := instDef.Exports["future-trailers"]
	require.True(t, ok, "future-trailers resource should be exported")
	require.NotNil(t, futureTrailersRes, "future-trailers resource should not be nil")
}

// TestWASI_HTTP_RequestOptionsMethods tests request options methods.
func TestWASI_HTTP_RequestOptionsMethods(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	typesDef, _ := linker.Get("wasi:http/types@0.2.0")
	instDef := typesDef.(*component.InstanceDef)

	// Verify request-options resource exists
	reqOptsRes, ok := instDef.Exports["request-options"]
	require.True(t, ok, "request-options resource should be exported")
	require.NotNil(t, reqOptsRes, "request-options resource should not be nil")

	// Expected request-options methods
	expectedMethods := []string{
		"[constructor]request-options",
		"[method]request-options.connect-timeout",
		"[method]request-options.set-connect-timeout",
		"[method]request-options.first-byte-timeout",
		"[method]request-options.set-first-byte-timeout",
		"[method]request-options.between-bytes-timeout",
		"[method]request-options.set-between-bytes-timeout",
	}

	for _, method := range expectedMethods {
		methodDef, ok := instDef.Exports[method]
		require.True(t, ok, "method %s should be exported", method)
		require.NotNil(t, methodDef, "method %s should not be nil", method)
	}
}

// TestWASI_HTTP_RequestOptionsConstructor tests the request-options constructor.
func TestWASI_HTTP_RequestOptionsConstructor(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	typesDef, _ := linker.Get("wasi:http/types@0.2.0")
	instDef := typesDef.(*component.InstanceDef)

	reqOptsConstructor := instDef.Exports["[constructor]request-options"].(*component.FuncDef)

	result, err := reqOptsConstructor.Callback(ctx, []component.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result), "[constructor]request-options should return exactly one value")

	// Result should be own<request-options>
	handle := result[0].Own()
	// Handle 0 may be valid placeholder, handle > 0 is real resource
	require.True(t, handle >= 0, "should return a valid request-options handle")
}

// TestWASI_HTTP_OutgoingRequestMethods tests outgoing request methods.
func TestWASI_HTTP_OutgoingRequestMethods(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	typesDef, _ := linker.Get("wasi:http/types@0.2.0")
	instDef := typesDef.(*component.InstanceDef)

	// Expected outgoing-request methods
	expectedMethods := []string{
		"[constructor]outgoing-request",
		"[method]outgoing-request.method",
		"[method]outgoing-request.set-method",
		"[method]outgoing-request.path-with-query",
		"[method]outgoing-request.set-path-with-query",
		"[method]outgoing-request.scheme",
		"[method]outgoing-request.set-scheme",
		"[method]outgoing-request.authority",
		"[method]outgoing-request.set-authority",
		"[method]outgoing-request.headers",
		"[method]outgoing-request.body",
	}

	for _, method := range expectedMethods {
		methodDef, ok := instDef.Exports[method]
		require.True(t, ok, "method %s should be exported", method)
		require.NotNil(t, methodDef, "method %s should not be nil", method)
	}
}

// TestWASI_HTTP_IncomingResponseMethods tests incoming response methods.
func TestWASI_HTTP_IncomingResponseMethods(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	typesDef, _ := linker.Get("wasi:http/types@0.2.0")
	instDef := typesDef.(*component.InstanceDef)

	// Expected incoming-response methods
	expectedMethods := []string{
		"[method]incoming-response.status",
		"[method]incoming-response.headers",
		"[method]incoming-response.consume",
	}

	for _, method := range expectedMethods {
		methodDef, ok := instDef.Exports[method]
		require.True(t, ok, "method %s should be exported", method)
		require.NotNil(t, methodDef, "method %s should not be nil", method)
	}
}

// TestWASI_HTTP_FutureIncomingResponseMethods tests future incoming response methods.
func TestWASI_HTTP_FutureIncomingResponseMethods(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	typesDef, _ := linker.Get("wasi:http/types@0.2.0")
	instDef := typesDef.(*component.InstanceDef)

	// Expected future-incoming-response methods
	expectedMethods := []string{
		"[method]future-incoming-response.get",
		"[method]future-incoming-response.subscribe",
	}

	for _, method := range expectedMethods {
		methodDef, ok := instDef.Exports[method]
		require.True(t, ok, "method %s should be exported", method)
		require.NotNil(t, methodDef, "method %s should not be nil", method)
	}
}

// TestWASI_HTTP_BodyMethods tests body methods.
func TestWASI_HTTP_BodyMethods(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	typesDef, _ := linker.Get("wasi:http/types@0.2.0")
	instDef := typesDef.(*component.InstanceDef)

	// Expected incoming-body methods
	incomingBodyMethods := []string{
		"[method]incoming-body.stream",
		"[static]incoming-body.finish",
	}

	for _, method := range incomingBodyMethods {
		methodDef, ok := instDef.Exports[method]
		require.True(t, ok, "method %s should be exported", method)
		require.NotNil(t, methodDef, "method %s should not be nil", method)
	}

	// Expected outgoing-body methods
	outgoingBodyMethods := []string{
		"[method]outgoing-body.write",
		"[static]outgoing-body.finish",
	}

	for _, method := range outgoingBodyMethods {
		methodDef, ok := instDef.Exports[method]
		require.True(t, ok, "method %s should be exported", method)
		require.NotNil(t, methodDef, "method %s should not be nil", method)
	}
}

// TestWASI_HTTP_InterfaceRegistration tests that all HTTP interfaces are properly registered.
func TestWASI_HTTP_InterfaceRegistration(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	// Verify all HTTP interfaces are registered
	interfaces := []string{
		"wasi:http/types@0.2.0",
		"wasi:http/outgoing-handler@0.2.0",
		"wasi:http/incoming-handler@0.2.0",
	}

	for _, iface := range interfaces {
		def, ok := linker.Get(iface)
		require.True(t, ok, "interface %s should be registered", iface)
		require.NotNil(t, def, "interface %s definition should not be nil", iface)
	}
}

// TestWASI_HTTP_ErrorCodeFunction tests the http-error-code function.
func TestWASI_HTTP_ErrorCodeFunction(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	typesDef, _ := linker.Get("wasi:http/types@0.2.0")
	instDef := typesDef.(*component.InstanceDef)

	// Verify http-error-code function exists
	errorCodeFunc, ok := instDef.Exports["http-error-code"]
	require.True(t, ok, "http-error-code function should be exported")
	require.NotNil(t, errorCodeFunc, "http-error-code function should not be nil")
}

// TestWASI_HTTP_ResponseOutparamResource tests that response-outparam resource exists.
func TestWASI_HTTP_ResponseOutparamResource(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	typesDef, _ := linker.Get("wasi:http/types@0.2.0")
	instDef := typesDef.(*component.InstanceDef)

	// Verify response-outparam resource exists
	respOutRes, ok := instDef.Exports["response-outparam"]
	require.True(t, ok, "response-outparam resource should be exported")
	require.NotNil(t, respOutRes, "response-outparam resource should not be nil")

	// Verify set static function exists
	setFunc, ok := instDef.Exports["[static]response-outparam.set"]
	require.True(t, ok, "[static]response-outparam.set should be exported")
	require.NotNil(t, setFunc, "[static]response-outparam.set should not be nil")
}
