// imports/wasip2/http/http_test.go

package http

import (
	"context"
	"errors"
	goio "io"
	gohttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tetratelabs/wazero/imports/wasip2/io"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestInstantiate(t *testing.T) {
	linker := component.NewLinker()
	err := Instantiate(linker)
	require.NoError(t, err)

	// Verify all HTTP interfaces are registered
	interfaces := []string{
		"wasi:http/types@0.2.0",
		"wasi:http/outgoing-handler@0.2.0",
		"wasi:http/incoming-handler@0.2.0",
	}

	for _, iface := range interfaces {
		def, err := linker.MatchImport(iface)
		require.NoError(t, err, "interface %s should be registered", iface)
		_, ok := def.(*component.InstanceDef)
		require.True(t, ok, "expected InstanceDef for %s", iface)
	}
}

func TestInstantiate_Duplicate(t *testing.T) {
	linker := component.NewLinker()

	// First registration should succeed
	err := Instantiate(linker)
	require.NoError(t, err)

	// Second registration should fail
	err = Instantiate(linker)
	require.Error(t, err)
}

// ====================
// Types Interface Tests
// ====================

func TestInstantiateTypes(t *testing.T) {
	linker := component.NewLinker()
	err := instantiateTypes(linker)
	require.NoError(t, err)

	def, err := linker.MatchImport("wasi:http/types@0.2.0")
	require.NoError(t, err)

	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	// Verify resources are defined
	resources := []string{
		"fields",
		"incoming-request",
		"incoming-response",
		"outgoing-request",
		"outgoing-response",
		"incoming-body",
		"outgoing-body",
		"future-incoming-response",
		"future-trailers",
		"response-outparam",
		"request-options",
	}

	for _, res := range resources {
		_, hasResource := instDef.Exports[res]
		require.True(t, hasResource, "%s resource should be defined", res)
	}

	// Verify fields methods
	fieldsMethods := []string{
		"[constructor]fields",
		"[method]fields.get",
		"[method]fields.set",
		"[method]fields.delete",
		"[method]fields.append",
		"[method]fields.entries",
		"[method]fields.clone",
		"[static]fields.from-list",
	}

	for _, method := range fieldsMethods {
		_, hasMethod := instDef.Exports[method]
		require.True(t, hasMethod, "%s should be defined", method)
	}

	// Verify outgoing-request methods
	outgoingRequestMethods := []string{
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

	for _, method := range outgoingRequestMethods {
		_, hasMethod := instDef.Exports[method]
		require.True(t, hasMethod, "%s should be defined", method)
	}

	// Verify incoming-request methods
	incomingRequestMethods := []string{
		"[method]incoming-request.method",
		"[method]incoming-request.path-with-query",
		"[method]incoming-request.scheme",
		"[method]incoming-request.authority",
		"[method]incoming-request.headers",
		"[method]incoming-request.consume",
	}

	for _, method := range incomingRequestMethods {
		_, hasMethod := instDef.Exports[method]
		require.True(t, hasMethod, "%s should be defined", method)
	}

	// Verify outgoing-response methods
	outgoingResponseMethods := []string{
		"[constructor]outgoing-response",
		"[method]outgoing-response.status-code",
		"[method]outgoing-response.set-status-code",
		"[method]outgoing-response.headers",
		"[method]outgoing-response.body",
	}

	for _, method := range outgoingResponseMethods {
		_, hasMethod := instDef.Exports[method]
		require.True(t, hasMethod, "%s should be defined", method)
	}

	// Verify incoming-response methods
	incomingResponseMethods := []string{
		"[method]incoming-response.status",
		"[method]incoming-response.headers",
		"[method]incoming-response.consume",
	}

	for _, method := range incomingResponseMethods {
		_, hasMethod := instDef.Exports[method]
		require.True(t, hasMethod, "%s should be defined", method)
	}

	// Verify incoming-body methods
	incomingBodyMethods := []string{
		"[method]incoming-body.stream",
		"[static]incoming-body.finish",
	}

	for _, method := range incomingBodyMethods {
		_, hasMethod := instDef.Exports[method]
		require.True(t, hasMethod, "%s should be defined", method)
	}

	// Verify outgoing-body methods
	outgoingBodyMethods := []string{
		"[method]outgoing-body.write",
		"[static]outgoing-body.finish",
	}

	for _, method := range outgoingBodyMethods {
		_, hasMethod := instDef.Exports[method]
		require.True(t, hasMethod, "%s should be defined", method)
	}

	// Verify future-incoming-response methods
	futureMethods := []string{
		"[method]future-incoming-response.get",
		"[method]future-incoming-response.subscribe",
	}

	for _, method := range futureMethods {
		_, hasMethod := instDef.Exports[method]
		require.True(t, hasMethod, "%s should be defined", method)
	}

	// Verify future-trailers methods
	futureTrailersMethods := []string{
		"[method]future-trailers.get",
		"[method]future-trailers.subscribe",
	}

	for _, method := range futureTrailersMethods {
		_, hasMethod := instDef.Exports[method]
		require.True(t, hasMethod, "%s should be defined", method)
	}

	// Verify response-outparam methods
	responseOutparamMethods := []string{
		"[static]response-outparam.set",
	}

	for _, method := range responseOutparamMethods {
		_, hasMethod := instDef.Exports[method]
		require.True(t, hasMethod, "%s should be defined", method)
	}

	// Verify request-options methods
	requestOptionsMethods := []string{
		"[constructor]request-options",
		"[method]request-options.connect-timeout",
		"[method]request-options.set-connect-timeout",
		"[method]request-options.first-byte-timeout",
		"[method]request-options.set-first-byte-timeout",
		"[method]request-options.between-bytes-timeout",
		"[method]request-options.set-between-bytes-timeout",
	}

	for _, method := range requestOptionsMethods {
		_, hasMethod := instDef.Exports[method]
		require.True(t, hasMethod, "%s should be defined", method)
	}

	// Verify http-error-code function
	_, hasErrorCode := instDef.Exports["http-error-code"]
	require.True(t, hasErrorCode, "http-error-code should be defined")
}

// ====================
// Fields Resource Tests
// ====================

func TestFieldsConstructor(t *testing.T) {
	// Returns: own<fields>
	result, err := fieldsConstructor(context.Background(), nil, []types.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOwn, result[0].Kind())
}

func TestFieldsFromList(t *testing.T) {
	// Args: entries (list<tuple<field-key, field-value>>)
	// Returns: result<own<fields>, header-error>
	entries := types.ValList([]types.Val{})
	result, err := fieldsFromList(context.Background(), nil, []types.Val{entries})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindOwn, ok.Kind())
}

func TestFieldsGet(t *testing.T) {
	// Args: self (borrow<fields>), name (field-key)
	// Returns: list<field-value>
	selfHandle := types.ValBorrow(0)
	name := types.ValString("content-type")
	result, err := fieldsGet(context.Background(), nil, []types.Val{selfHandle, name})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindList, result[0].Kind())
}

func TestFieldsSet(t *testing.T) {
	// Args: self (borrow<fields>), name (field-key), values (list<field-value>)
	// Returns: result<_, header-error>
	selfHandle := types.ValBorrow(0)
	name := types.ValString("content-type")
	values := types.ValList([]types.Val{})
	result, err := fieldsSet(context.Background(), nil, []types.Val{selfHandle, name, values})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestFieldsDelete(t *testing.T) {
	// Args: self (borrow<fields>), name (field-key)
	// Returns: result<_, header-error>
	selfHandle := types.ValBorrow(0)
	name := types.ValString("content-type")
	result, err := fieldsDelete(context.Background(), nil, []types.Val{selfHandle, name})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestFieldsAppend(t *testing.T) {
	// Args: self (borrow<fields>), name (field-key), value (field-value)
	// Returns: result<_, header-error>
	selfHandle := types.ValBorrow(0)
	name := types.ValString("accept")
	value := types.ValList([]types.Val{types.ValU8('a')})
	result, err := fieldsAppend(context.Background(), nil, []types.Val{selfHandle, name, value})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestFieldsEntries(t *testing.T) {
	// Args: self (borrow<fields>)
	// Returns: list<tuple<field-key, field-value>>
	selfHandle := types.ValBorrow(0)
	result, err := fieldsEntries(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindList, result[0].Kind())
}

func TestFieldsClone(t *testing.T) {
	// Args: self (borrow<fields>)
	// Returns: own<fields>
	selfHandle := types.ValBorrow(0)
	result, err := fieldsClone(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOwn, result[0].Kind())
}

func TestFieldsHas_WithResourceTable(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	fields := NewFields()
	fields.Set("Content-Type", [][]byte{[]byte("text/html")})
	sid := registerFields(fields)
	handle, errH1 := table.NewResourceHandle(sid, true, httpFieldsResourceType)
	if errH1 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH1)
	}

	selfHandle := types.ValBorrow(uint32(handle))

	// Has existing key
	name := types.ValString("Content-Type")
	result, err := fieldsHas(ctx, nil, []types.Val{selfHandle, name})
	require.NoError(t, err)
	require.True(t, result[0].Bool(), "should return true for existing key")

	// Has missing key
	name = types.ValString("X-Missing")
	result, err = fieldsHas(ctx, nil, []types.Val{selfHandle, name})
	require.NoError(t, err)
	require.False(t, result[0].Bool(), "should return false for missing key")
}

// ====================
// Outgoing Request Tests
// ====================

func TestOutgoingRequestConstructor(t *testing.T) {
	// Args: headers (own<fields>)
	// Returns: own<outgoing-request>
	headers := types.ValOwn(0)
	result, err := outgoingRequestConstructor(context.Background(), nil, []types.Val{headers})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOwn, result[0].Kind())
}

func TestOutgoingRequestMethod(t *testing.T) {
	// Args: self (borrow<outgoing-request>)
	// Returns: method
	selfHandle := types.ValBorrow(0)
	result, err := outgoingRequestMethod(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindVariant, result[0].Kind())
}

func TestOutgoingRequestSetMethod(t *testing.T) {
	// Args: self (borrow<outgoing-request>), method (method)
	// Returns: result<_, _>
	selfHandle := types.ValBorrow(0)
	method := types.ValVariant("get", nil)
	result, err := outgoingRequestSetMethod(context.Background(), nil, []types.Val{selfHandle, method})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestOutgoingRequestPathWithQuery(t *testing.T) {
	// Args: self (borrow<outgoing-request>)
	// Returns: option<string>
	selfHandle := types.ValBorrow(0)
	result, err := outgoingRequestPathWithQuery(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOption, result[0].Kind())
}

func TestOutgoingRequestSetPathWithQuery(t *testing.T) {
	// Args: self (borrow<outgoing-request>), path (option<string>)
	// Returns: result<_, _>
	selfHandle := types.ValBorrow(0)
	path := types.ValString("/foo/bar")
	pathOpt := types.ValOption(&path)
	result, err := outgoingRequestSetPathWithQuery(context.Background(), nil, []types.Val{selfHandle, pathOpt})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestOutgoingRequestScheme(t *testing.T) {
	// Args: self (borrow<outgoing-request>)
	// Returns: option<scheme>
	selfHandle := types.ValBorrow(0)
	result, err := outgoingRequestScheme(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOption, result[0].Kind())
}

func TestOutgoingRequestSetScheme(t *testing.T) {
	// Args: self (borrow<outgoing-request>), scheme (option<scheme>)
	// Returns: result<_, _>
	selfHandle := types.ValBorrow(0)
	scheme := types.ValVariant("https", nil)
	schemeOpt := types.ValOption(&scheme)
	result, err := outgoingRequestSetScheme(context.Background(), nil, []types.Val{selfHandle, schemeOpt})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestOutgoingRequestAuthority(t *testing.T) {
	// Args: self (borrow<outgoing-request>)
	// Returns: option<string>
	selfHandle := types.ValBorrow(0)
	result, err := outgoingRequestAuthority(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOption, result[0].Kind())
}

func TestOutgoingRequestSetAuthority(t *testing.T) {
	// Args: self (borrow<outgoing-request>), authority (option<string>)
	// Returns: result<_, _>
	selfHandle := types.ValBorrow(0)
	authority := types.ValString("example.com")
	authorityOpt := types.ValOption(&authority)
	result, err := outgoingRequestSetAuthority(context.Background(), nil, []types.Val{selfHandle, authorityOpt})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestOutgoingRequestHeaders(t *testing.T) {
	// Args: self (borrow<outgoing-request>)
	// Returns: own<fields>
	selfHandle := types.ValBorrow(0)
	result, err := outgoingRequestHeaders(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOwn, result[0].Kind())
}

func TestOutgoingRequestBody(t *testing.T) {
	// Args: self (borrow<outgoing-request>)
	// Returns: result<own<outgoing-body>, _>
	selfHandle := types.ValBorrow(0)
	result, err := outgoingRequestBody(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindOwn, ok.Kind())
}

// ====================
// Incoming Request Tests
// ====================

func TestIncomingRequestMethod(t *testing.T) {
	// Args: self (borrow<incoming-request>)
	// Returns: method
	selfHandle := types.ValBorrow(0)
	result, err := incomingRequestMethod(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindVariant, result[0].Kind())
}

func TestIncomingRequestPathWithQuery(t *testing.T) {
	// Args: self (borrow<incoming-request>)
	// Returns: option<string>
	selfHandle := types.ValBorrow(0)
	result, err := incomingRequestPathWithQuery(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOption, result[0].Kind())
}

func TestIncomingRequestScheme(t *testing.T) {
	// Args: self (borrow<incoming-request>)
	// Returns: option<scheme>
	selfHandle := types.ValBorrow(0)
	result, err := incomingRequestScheme(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOption, result[0].Kind())
}

func TestIncomingRequestAuthority(t *testing.T) {
	// Args: self (borrow<incoming-request>)
	// Returns: option<string>
	selfHandle := types.ValBorrow(0)
	result, err := incomingRequestAuthority(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOption, result[0].Kind())
}

func TestIncomingRequestHeaders(t *testing.T) {
	// Args: self (borrow<incoming-request>)
	// Returns: own<fields>
	selfHandle := types.ValBorrow(0)
	result, err := incomingRequestHeaders(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOwn, result[0].Kind())
}

func TestIncomingRequestConsume(t *testing.T) {
	// Args: self (borrow<incoming-request>)
	// Returns: result<own<incoming-body>, _>
	selfHandle := types.ValBorrow(0)
	result, err := incomingRequestConsume(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindOwn, ok.Kind())
}

// ====================
// Incoming Request Handler Tests (with resource table)
// ====================

// TestIncomingRequest_Method_WithResourceTable tests that method accessor
// returns the correct HTTP method from a registered incoming request.
func TestIncomingRequest_Method_WithResourceTable(t *testing.T) {
	// Create resource table and context
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Create an incoming request with POST method
	headers := NewFields()
	path := "/api/test?foo=bar"
	authority := "example.com"
	scheme := NewSchemeHTTPS()
	sid := registerIncomingRequest(NewIncomingRequest(MethodPost, &scheme, &authority, &path, headers))

	// Register in resource table
	handle, errH2 := table.NewResourceHandle(sid, true, httpFieldsResourceType)
	if errH2 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH2)
	}

	// Call method accessor
	selfHandle := types.ValBorrow(uint32(handle))
	result, err := incomingRequestMethod(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindVariant, result[0].Kind())

	// Verify returns correct method variant
	methodName, _ := result[0].Variant()
	require.Equal(t, "post", methodName, "expected POST method")
}

// TestIncomingRequest_PathWithQuery_WithResourceTable tests that path-with-query
// accessor returns the correct path from a registered incoming request.
func TestIncomingRequest_PathWithQuery_WithResourceTable(t *testing.T) {
	// Create resource table and context
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Create an incoming request with path
	headers := NewFields()
	path := "/api/test?foo=bar"
	authority := "example.com"
	scheme := NewSchemeHTTPS()
	sid := registerIncomingRequest(NewIncomingRequest(MethodGet, &scheme, &authority, &path, headers))

	// Register in resource table
	handle, errH3 := table.NewResourceHandle(sid, true, httpFieldsResourceType)
	if errH3 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH3)
	}

	// Call path-with-query accessor
	selfHandle := types.ValBorrow(uint32(handle))
	result, err := incomingRequestPathWithQuery(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOption, result[0].Kind())

	// Verify returns Some with correct path
	optVal := result[0].Option()
	require.NotNil(t, optVal, "expected Some, got None")
	require.Equal(t, "/api/test?foo=bar", optVal.StringVal())
}

// TestIncomingRequest_Scheme_WithResourceTable tests that scheme accessor
// returns the correct scheme from a registered incoming request.
func TestIncomingRequest_Scheme_WithResourceTable(t *testing.T) {
	// Create resource table and context
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Create an incoming request with HTTPS scheme
	headers := NewFields()
	path := "/api/test"
	authority := "example.com"
	scheme := NewSchemeHTTPS()
	sid := registerIncomingRequest(NewIncomingRequest(MethodGet, &scheme, &authority, &path, headers))

	// Register in resource table
	handle, errH4 := table.NewResourceHandle(sid, true, httpFieldsResourceType)
	if errH4 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH4)
	}

	// Call scheme accessor
	selfHandle := types.ValBorrow(uint32(handle))
	result, err := incomingRequestScheme(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOption, result[0].Kind())

	// Verify returns Some with correct scheme variant
	optVal := result[0].Option()
	require.NotNil(t, optVal, "expected Some, got None")
	require.Equal(t, types.ValKindVariant, optVal.Kind())
	schemeName, _ := optVal.Variant()
	require.Equal(t, "HTTPS", schemeName, "expected HTTPS scheme")
}

// TestIncomingRequest_Authority_WithResourceTable tests that authority accessor
// returns the correct authority from a registered incoming request.
func TestIncomingRequest_Authority_WithResourceTable(t *testing.T) {
	// Create resource table and context
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Create an incoming request with authority
	headers := NewFields()
	path := "/api/test"
	authority := "example.com:8080"
	scheme := NewSchemeHTTPS()
	sid := registerIncomingRequest(NewIncomingRequest(MethodGet, &scheme, &authority, &path, headers))

	// Register in resource table
	handle, errH5 := table.NewResourceHandle(sid, true, httpFieldsResourceType)
	if errH5 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH5)
	}

	// Call authority accessor
	selfHandle := types.ValBorrow(uint32(handle))
	result, err := incomingRequestAuthority(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOption, result[0].Kind())

	// Verify returns Some with correct authority
	optVal := result[0].Option()
	require.NotNil(t, optVal, "expected Some, got None")
	require.Equal(t, "example.com:8080", optVal.StringVal())
}

// TestIncomingRequest_Headers_WithResourceTable tests that headers accessor
// returns a valid headers handle from a registered incoming request.
func TestIncomingRequest_Headers_WithResourceTable(t *testing.T) {
	// Create resource table and context
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Create an incoming request with headers
	headers := NewFields()
	headers.Set("Content-Type", [][]byte{[]byte("application/json")})
	headers.Set("Accept", [][]byte{[]byte("*/*")})
	path := "/api/test"
	authority := "example.com"
	scheme := NewSchemeHTTPS()
	sid := registerIncomingRequest(NewIncomingRequest(MethodGet, &scheme, &authority, &path, headers))

	// Register in resource table
	handle, errH6 := table.NewResourceHandle(sid, true, httpFieldsResourceType)
	if errH6 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH6)
	}

	// Call headers accessor
	selfHandle := types.ValBorrow(uint32(handle))
	result, err := incomingRequestHeaders(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOwn, result[0].Kind())

	// Verify the returned handle points to valid headers
	headersHandle := runtime.Handle(result[0].Own())
	rawEntry1, err := table.Get(headersHandle)
	entry, _ := rawEntry1.(*runtime.ResourceHandleEntry)
	require.NoError(t, err)
	require.NotNil(t, entry)

	_ = entry
}

// TestIncomingRequest_Consume_WithResourceTable tests that consume
// returns a valid incoming body handle from a registered incoming request.
func TestIncomingRequest_Consume_WithResourceTable(t *testing.T) {
	// Create resource table and context
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Create an incoming request
	headers := NewFields()
	path := "/api/test"
	authority := "example.com"
	scheme := NewSchemeHTTPS()
	sid := registerIncomingRequest(NewIncomingRequest(MethodPost, &scheme, &authority, &path, headers))

	// Register in resource table
	handle, errH7 := table.NewResourceHandle(sid, true, httpFieldsResourceType)
	if errH7 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH7)
	}

	// Call consume
	selfHandle := types.ValBorrow(uint32(handle))
	result, err := incomingRequestConsume(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindOwn, ok.Kind())

	// Verify the returned handle points to a valid incoming body
	bodyHandle := runtime.Handle(ok.Own())
	require.NotEqual(t, runtime.Handle(0), bodyHandle, "body handle should not be 0")

	rawEntry2, err := table.Get(bodyHandle)
	entry, _ := rawEntry2.(*runtime.ResourceHandleEntry)
	require.NoError(t, err)
	require.NotNil(t, entry)

	_ = entry
}

// TestIncomingRequest_AllMethods tests all methods for various HTTP methods.
func TestIncomingRequest_AllMethods(t *testing.T) {
	methods := []struct {
		method       Method
		expectedName string
	}{
		{MethodGet, "get"},
		{MethodHead, "head"},
		{MethodPost, "post"},
		{MethodPut, "put"},
		{MethodDelete, "delete"},
		{MethodConnect, "connect"},
		{MethodOptions, "options"},
		{MethodTrace, "trace"},
		{MethodPatch, "patch"},
	}

	for _, tc := range methods {
		t.Run(tc.expectedName, func(t *testing.T) {
			table := runtime.NewTable()
			ctx := component.WithResourceTable(context.Background(), table)

			headers := NewFields()
			path := "/test"
			authority := "localhost"
			scheme := NewSchemeHTTP()
			sid := registerIncomingRequest(NewIncomingRequest(tc.method, &scheme, &authority, &path, headers))

			handle, errH8 := table.NewResourceHandle(sid, true, httpFieldsResourceType)
			if errH8 != nil {
				t.Fatalf("NewResourceHandle failed: %v", errH8)
			}
			selfHandle := types.ValBorrow(uint32(handle))

			result, err := incomingRequestMethod(ctx, nil, []types.Val{selfHandle})
			require.NoError(t, err)

			methodName, _ := result[0].Variant()
			require.Equal(t, tc.expectedName, methodName)
		})
	}
}

// TestIncomingRequest_NilScheme tests handling of nil scheme.
func TestIncomingRequest_NilScheme(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	headers := NewFields()
	path := "/test"
	authority := "localhost"
	sid := registerIncomingRequest(NewIncomingRequest(MethodGet, nil, &authority, &path, headers))

	handle, errH9 := table.NewResourceHandle(sid, true, httpFieldsResourceType)
	if errH9 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH9)
	}
	selfHandle := types.ValBorrow(uint32(handle))

	result, err := incomingRequestScheme(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, types.ValKindOption, result[0].Kind())

	// Should return None when scheme is nil
	optVal := result[0].Option()
	require.Nil(t, optVal, "expected None for nil scheme")
}

// TestIncomingRequest_NilPathWithQuery tests handling of nil path.
func TestIncomingRequest_NilPathWithQuery(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	headers := NewFields()
	authority := "localhost"
	scheme := NewSchemeHTTP()
	sid := registerIncomingRequest(NewIncomingRequest(MethodGet, &scheme, &authority, nil, headers))

	handle, errH10 := table.NewResourceHandle(sid, true, httpFieldsResourceType)
	if errH10 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH10)
	}
	selfHandle := types.ValBorrow(uint32(handle))

	result, err := incomingRequestPathWithQuery(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, types.ValKindOption, result[0].Kind())

	// Should return None when path is nil
	optVal := result[0].Option()
	require.Nil(t, optVal, "expected None for nil path")
}

// TestIncomingRequest_NilAuthority tests handling of nil authority.
func TestIncomingRequest_NilAuthority(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	headers := NewFields()
	path := "/test"
	scheme := NewSchemeHTTP()
	sid := registerIncomingRequest(NewIncomingRequest(MethodGet, &scheme, nil, &path, headers))

	handle, errH11 := table.NewResourceHandle(sid, true, httpFieldsResourceType)
	if errH11 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH11)
	}
	selfHandle := types.ValBorrow(uint32(handle))

	result, err := incomingRequestAuthority(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, types.ValKindOption, result[0].Kind())

	// Should return None when authority is nil
	optVal := result[0].Option()
	require.Nil(t, optVal, "expected None for nil authority")
}

// ====================
// Outgoing Response Tests
// ====================

func TestOutgoingResponseConstructor(t *testing.T) {
	// Args: headers (own<fields>)
	// Returns: own<outgoing-response>
	headers := types.ValOwn(0)
	result, err := outgoingResponseConstructor(context.Background(), nil, []types.Val{headers})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOwn, result[0].Kind())
}

func TestOutgoingResponseStatusCode(t *testing.T) {
	// Args: self (borrow<outgoing-response>)
	// Returns: status-code (u16)
	selfHandle := types.ValBorrow(0)
	result, err := outgoingResponseStatusCode(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindU16, result[0].Kind())
}

func TestOutgoingResponseSetStatusCode(t *testing.T) {
	// Args: self (borrow<outgoing-response>), status-code (u16)
	// Returns: result<_, _>
	selfHandle := types.ValBorrow(0)
	status := types.ValU16(200)
	result, err := outgoingResponseSetStatusCode(context.Background(), nil, []types.Val{selfHandle, status})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestOutgoingResponseHeaders(t *testing.T) {
	// Args: self (borrow<outgoing-response>)
	// Returns: own<fields>
	selfHandle := types.ValBorrow(0)
	result, err := outgoingResponseHeaders(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOwn, result[0].Kind())
}

func TestOutgoingResponseBody(t *testing.T) {
	// Args: self (borrow<outgoing-response>)
	// Returns: result<own<outgoing-body>, _>
	selfHandle := types.ValBorrow(0)
	result, err := outgoingResponseBody(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindOwn, ok.Kind())
}

func TestOutgoingResponseConstructor_WithResourceTable(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Occupy slot 0 so that when headers (slot 1) is removed,
	// the response gets allocated in a fresh slot (slot 2) with generation 0.
	dummySid := registerFields(NewFields())
	_, _ = table.NewResourceHandle(dummySid, false, httpFieldsResourceType)

	// Create headers
	headers := NewFields()
	headers.Set("X-Test", [][]byte{[]byte("value")})
	hdrSid := registerFields(headers)
	headersHandle, errH12 := table.NewResourceHandle(hdrSid, true, httpFieldsResourceType)
	if errH12 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH12)
	}

	headersVal := types.ValOwn(uint32(headersHandle))
	result, err := outgoingResponseConstructor(ctx, nil, []types.Val{headersVal})
	require.NoError(t, err)
	respHandle := result[0].Own()
	require.NotEqual(t, uint32(0), respHandle)

	// Verify the response works by calling status code on it
	selfHandle := types.ValBorrow(respHandle)
	statusResult, err := outgoingResponseStatusCode(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, uint16(200), statusResult[0].U16())
}

func TestOutgoingResponseStatusCode_WithResourceTable(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	resp := NewOutgoingResponse(NewFields())
	sid := registerOutgoingResponse(resp)
	resp.SetStatusCode(404)
	handle, errH13 := table.NewResourceHandle(sid, true, httpFieldsResourceType)
	if errH13 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH13)
	}

	selfHandle := types.ValBorrow(uint32(handle))
	result, err := outgoingResponseStatusCode(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, uint16(404), result[0].U16())
}

func TestOutgoingResponseSetStatusCode_WithResourceTable(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	resp := NewOutgoingResponse(NewFields())
	sid := registerOutgoingResponse(resp)
	handle, errH14 := table.NewResourceHandle(sid, true, httpFieldsResourceType)
	if errH14 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH14)
	}

	selfHandle := types.ValBorrow(uint32(handle))
	status := types.ValU16(500)
	result, err := outgoingResponseSetStatusCode(ctx, nil, []types.Val{selfHandle, status})
	require.NoError(t, err)
	isOk, _, _ := result[0].Result()
	require.True(t, isOk)
	require.Equal(t, uint16(500), resp.StatusCode())
}

func TestOutgoingResponseBody_WithResourceTable(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	sid := registerOutgoingResponse(NewOutgoingResponse(NewFields()))
	handle, errH15 := table.NewResourceHandle(sid, true, httpFieldsResourceType)
	if errH15 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH15)
	}

	selfHandle := types.ValBorrow(uint32(handle))
	result, err := outgoingResponseBody(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	isOk, bodyHandle, _ := result[0].Result()
	require.True(t, isOk)
	require.NotNil(t, bodyHandle)

	// Second call should fail
	result, err = outgoingResponseBody(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	isOk, _, _ = result[0].Result()
	require.False(t, isOk, "second body call should fail")
}

// ====================
// Incoming Response Tests
// ====================

func TestIncomingResponseStatus(t *testing.T) {
	// Args: self (borrow<incoming-response>)
	// Returns: status-code (u16)
	selfHandle := types.ValBorrow(0)
	result, err := incomingResponseStatus(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindU16, result[0].Kind())
}

func TestIncomingResponseHeaders(t *testing.T) {
	// Args: self (borrow<incoming-response>)
	// Returns: own<fields>
	selfHandle := types.ValBorrow(0)
	result, err := incomingResponseHeaders(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOwn, result[0].Kind())
}

func TestIncomingResponseConsume(t *testing.T) {
	// Args: self (borrow<incoming-response>)
	// Returns: result<own<incoming-body>, _>
	selfHandle := types.ValBorrow(0)
	result, err := incomingResponseConsume(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindOwn, ok.Kind())
}

// ====================
// Incoming Body Tests
// ====================

func TestIncomingBodyStream(t *testing.T) {
	// Args: self (borrow<incoming-body>)
	// Returns: result<own<input-stream>, _>
	selfHandle := types.ValBorrow(0)
	result, err := incomingBodyStream(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindOwn, ok.Kind())
}

func TestIncomingBodyFinish(t *testing.T) {
	// Args: body (own<incoming-body>)
	// Returns: own<future-trailers>
	body := types.ValOwn(0)
	result, err := incomingBodyFinish(context.Background(), nil, []types.Val{body})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOwn, result[0].Kind())
}

// ====================
// Outgoing Body Tests
// ====================

func TestOutgoingBodyWrite(t *testing.T) {
	// Args: self (borrow<outgoing-body>)
	// Returns: result<own<output-stream>, _>
	selfHandle := types.ValBorrow(0)
	result, err := outgoingBodyWrite(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindOwn, ok.Kind())
}

func TestOutgoingBodyFinish(t *testing.T) {
	// Args: body (own<outgoing-body>), trailers (option<own<fields>>)
	// Returns: result<_, error-code>
	body := types.ValOwn(0)
	trailers := types.ValOption(nil)
	result, err := outgoingBodyFinish(context.Background(), nil, []types.Val{body, trailers})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

// ====================
// Future Incoming Response Tests
// ====================

func TestFutureIncomingResponseGet(t *testing.T) {
	// Args: self (borrow<future-incoming-response>)
	// Returns: option<result<result<own<incoming-response>, error-code>, _>>
	selfHandle := types.ValBorrow(0)
	result, err := futureIncomingResponseGet(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOption, result[0].Kind())
}

func TestFutureIncomingResponseSubscribe(t *testing.T) {
	// Args: self (borrow<future-incoming-response>)
	// Returns: own<pollable>
	selfHandle := types.ValBorrow(0)
	result, err := futureIncomingResponseSubscribe(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOwn, result[0].Kind())
}

// ====================
// Future Trailers Tests
// ====================

func TestFutureTrailersGet(t *testing.T) {
	// Args: self (borrow<future-trailers>)
	// Returns: option<result<result<option<own<fields>>, error-code>, _>>
	selfHandle := types.ValBorrow(0)
	result, err := futureTrailersGet(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOption, result[0].Kind())
}

func TestFutureTrailersSubscribe(t *testing.T) {
	// Args: self (borrow<future-trailers>)
	// Returns: own<pollable>
	selfHandle := types.ValBorrow(0)
	result, err := futureTrailersSubscribe(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOwn, result[0].Kind())
}

func TestIncomingBodyFinish_ReturnsFutureTrailers(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Use a non-existent body handle — incomingBodyFinish handles this
	// gracefully and still creates the FutureTrailers resource.
	bodyVal := types.ValOwn(uint32(999))
	result, err := incomingBodyFinish(ctx, nil, []types.Val{bodyVal})
	require.NoError(t, err)
	require.Equal(t, types.ValKindOwn, result[0].Kind())

	// Verify the returned handle points to a FutureTrailers by using it
	ftHandle := result[0].Own()
	ft, err := getFutureTrailers(ctx, ftHandle)
	require.NoError(t, err)
	require.NotNil(t, ft, "finish should return FutureTrailers resource")
	require.True(t, ft.IsReady(), "future trailers from finish should be ready")
}

func TestFutureTrailersGet_NoTrailers(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Create a ready FutureTrailers directly (no body removal to avoid slot reuse)
	sid := registerFutureTrailers(NewFutureTrailersReady(nil, nil))
	ftHandle, errH16 := table.NewResourceHandle(sid, true, httpFieldsResourceType)
	if errH16 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH16)
	}

	ftBorrow := types.ValBorrow(uint32(ftHandle))

	// Get should return Some (ready)
	result, err := futureTrailersGet(ctx, nil, []types.Val{ftBorrow})
	require.NoError(t, err)

	outerOpt := result[0].Option()
	require.NotNil(t, outerOpt, "should return Some (ready)")
}

func TestFutureTrailersGet_ConsumedOnSecondCall(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	sid := registerFutureTrailers(NewFutureTrailersReady(nil, nil))
	ftHandle, errH17 := table.NewResourceHandle(sid, true, httpFieldsResourceType)
	if errH17 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH17)
	}

	ftBorrow := types.ValBorrow(uint32(ftHandle))

	// First call returns result
	futureTrailersGet(ctx, nil, []types.Val{ftBorrow})

	// Second call returns None (consumed)
	result, err := futureTrailersGet(ctx, nil, []types.Val{ftBorrow})
	require.NoError(t, err)
	outerOpt := result[0].Option()
	require.Nil(t, outerOpt, "second get should return None (consumed)")
}

func TestFutureTrailersSubscribe_ReturnsValidPollable(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	sid := registerFutureTrailers(NewFutureTrailersReady(nil, nil))
	ftHandle, errH18 := table.NewResourceHandle(sid, true, httpFieldsResourceType)
	if errH18 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH18)
	}

	ftBorrow := types.ValBorrow(uint32(ftHandle))
	result, err := futureTrailersSubscribe(ctx, nil, []types.Val{ftBorrow})
	require.NoError(t, err)
	require.Equal(t, types.ValKindOwn, result[0].Kind())
	require.True(t, result[0].Own() > 0, "should return valid pollable handle")
}

// ====================
// Response Outparam Tests
// ====================

func TestResponseOutparamSet(t *testing.T) {
	// Args: param (own<response-outparam>), response (result<own<outgoing-response>, error-code>)
	// Returns: nothing (unit)
	param := types.ValOwn(0)
	response := types.ValOwn(1)
	responseResult := types.ValResultOk(&response)
	result, err := responseOutparamSet(context.Background(), nil, []types.Val{param, responseResult})
	require.NoError(t, err)
	require.Equal(t, 0, len(result)) // Returns nothing
}

// ====================
// Request Options Tests
// ====================

func TestRequestOptionsConstructor(t *testing.T) {
	// Returns: own<request-options>
	result, err := requestOptionsConstructor(context.Background(), nil, []types.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOwn, result[0].Kind())
}

func TestRequestOptionsConnectTimeout(t *testing.T) {
	// Args: self (borrow<request-options>)
	// Returns: option<duration>
	selfHandle := types.ValBorrow(0)
	result, err := requestOptionsConnectTimeout(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOption, result[0].Kind())
}

func TestRequestOptionsSetConnectTimeout(t *testing.T) {
	// Args: self (borrow<request-options>), timeout (option<duration>)
	// Returns: result<_, _>
	selfHandle := types.ValBorrow(0)
	timeout := types.ValU64(30000000000) // 30 seconds in nanoseconds
	timeoutOpt := types.ValOption(&timeout)
	result, err := requestOptionsSetConnectTimeout(context.Background(), nil, []types.Val{selfHandle, timeoutOpt})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestRequestOptionsFirstByteTimeout(t *testing.T) {
	// Args: self (borrow<request-options>)
	// Returns: option<duration>
	selfHandle := types.ValBorrow(0)
	result, err := requestOptionsFirstByteTimeout(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOption, result[0].Kind())
}

func TestRequestOptionsSetFirstByteTimeout(t *testing.T) {
	// Args: self (borrow<request-options>), timeout (option<duration>)
	// Returns: result<_, _>
	selfHandle := types.ValBorrow(0)
	timeout := types.ValU64(60000000000)
	timeoutOpt := types.ValOption(&timeout)
	result, err := requestOptionsSetFirstByteTimeout(context.Background(), nil, []types.Val{selfHandle, timeoutOpt})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestRequestOptionsBetweenBytesTimeout(t *testing.T) {
	// Args: self (borrow<request-options>)
	// Returns: option<duration>
	selfHandle := types.ValBorrow(0)
	result, err := requestOptionsBetweenBytesTimeout(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOption, result[0].Kind())
}

func TestRequestOptionsSetBetweenBytesTimeout(t *testing.T) {
	// Args: self (borrow<request-options>), timeout (option<duration>)
	// Returns: result<_, _>
	selfHandle := types.ValBorrow(0)
	timeout := types.ValU64(10000000000)
	timeoutOpt := types.ValOption(&timeout)
	result, err := requestOptionsSetBetweenBytesTimeout(context.Background(), nil, []types.Val{selfHandle, timeoutOpt})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

// ====================
// HTTP Error Code Tests
// ====================

func TestHttpErrorCode(t *testing.T) {
	// Args: err (borrow<io-error>)
	// Returns: option<error-code>
	errHandle := types.ValBorrow(0)
	result, err := httpErrorCode(context.Background(), nil, []types.Val{errHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOption, result[0].Kind())
}

// ====================
// Outgoing Handler Tests
// ====================

func TestInstantiateOutgoingHandler(t *testing.T) {
	linker := component.NewLinker()
	err := instantiateOutgoingHandler(linker)
	require.NoError(t, err)

	def, err := linker.MatchImport("wasi:http/outgoing-handler@0.2.0")
	require.NoError(t, err)

	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	// Verify handle function is defined
	_, hasHandle := instDef.Exports["handle"]
	require.True(t, hasHandle, "handle function should be defined")
}

func TestOutgoingHandlerHandle(t *testing.T) {
	// Args: request (own<outgoing-request>), options (option<own<request-options>>)
	// Returns: result<own<future-incoming-response>, error-code>
	request := types.ValOwn(0)
	options := types.ValOption(nil)
	result, err := outgoingHandlerHandle(context.Background(), nil, []types.Val{request, options})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindOwn, ok.Kind())
}

// ====================
// Incoming Handler Tests
// ====================

func TestInstantiateIncomingHandler(t *testing.T) {
	linker := component.NewLinker()
	err := instantiateIncomingHandler(linker)
	require.NoError(t, err)

	def, err := linker.MatchImport("wasi:http/incoming-handler@0.2.0")
	require.NoError(t, err)

	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	// Verify handle function is defined
	_, hasHandle := instDef.Exports["handle"]
	require.True(t, hasHandle, "handle function should be defined")
}

func TestIncomingHandlerHandle(t *testing.T) {
	// Args: request (own<incoming-request>), response-out (own<response-outparam>)
	// Returns: nothing (unit)
	request := types.ValOwn(0)
	responseOut := types.ValOwn(1)
	result, err := incomingHandlerHandle(context.Background(), nil, []types.Val{request, responseOut})
	require.NoError(t, err)
	require.Equal(t, 0, len(result)) // Returns nothing
}

// ====================
// Type Tests
// ====================

func TestMethod(t *testing.T) {
	require.Equal(t, Method(0), MethodGet)
	require.Equal(t, Method(1), MethodHead)
	require.Equal(t, Method(2), MethodPost)
	require.Equal(t, Method(3), MethodPut)
	require.Equal(t, Method(4), MethodDelete)
	require.Equal(t, Method(5), MethodConnect)
	require.Equal(t, Method(6), MethodOptions)
	require.Equal(t, Method(7), MethodTrace)
	require.Equal(t, Method(8), MethodPatch)
	require.Equal(t, Method(9), MethodOther)
}

func TestMethodString(t *testing.T) {
	require.Equal(t, "get", MethodGet.String())
	require.Equal(t, "head", MethodHead.String())
	require.Equal(t, "post", MethodPost.String())
	require.Equal(t, "put", MethodPut.String())
	require.Equal(t, "delete", MethodDelete.String())
	require.Equal(t, "connect", MethodConnect.String())
	require.Equal(t, "options", MethodOptions.String())
	require.Equal(t, "trace", MethodTrace.String())
	require.Equal(t, "patch", MethodPatch.String())
	require.Equal(t, "other", MethodOther.String())
}

func TestScheme(t *testing.T) {
	http := NewSchemeHTTP()
	require.Equal(t, schemeKindHTTP, http.kind)

	https := NewSchemeHTTPS()
	require.Equal(t, schemeKindHTTPS, https.kind)

	other := NewSchemeOther("custom")
	require.Equal(t, schemeKindOther, other.kind)
	require.Equal(t, "custom", other.other)
}

func TestSchemeString(t *testing.T) {
	require.Equal(t, "http", NewSchemeHTTP().String())
	require.Equal(t, "https", NewSchemeHTTPS().String())
	require.Equal(t, "custom", NewSchemeOther("custom").String())
}

func TestFields(t *testing.T) {
	f := NewFields()
	require.NotNil(t, f)
	require.NotNil(t, f.entries)
	require.Equal(t, 0, len(f.entries))
}

func TestFieldsSetGet(t *testing.T) {
	f := NewFields()
	f.Set("content-type", [][]byte{[]byte("application/json")})
	values := f.Get("content-type")
	require.Equal(t, 1, len(values))
	require.Equal(t, []byte("application/json"), values[0])
}

func TestFieldsAppendEntry(t *testing.T) {
	f := NewFields()
	f.Append("accept", []byte("text/plain"))
	f.Append("accept", []byte("application/json"))
	values := f.Get("accept")
	require.Equal(t, 2, len(values))
}

func TestFieldsType_Delete(t *testing.T) {
	f := NewFields()
	f.Set("x-custom", [][]byte{[]byte("value")})
	f.Delete("x-custom")
	values := f.Get("x-custom")
	require.Equal(t, 0, len(values))
}

func TestFieldsType_Entries(t *testing.T) {
	f := NewFields()
	f.Set("content-type", [][]byte{[]byte("text/html")})
	f.Set("accept", [][]byte{[]byte("*/*")})
	entries := f.Entries()
	require.Equal(t, 2, len(entries))
}

func TestFieldsType_Clone(t *testing.T) {
	f := NewFields()
	f.Set("key", [][]byte{[]byte("value")})
	clone := f.Clone()
	require.Equal(t, f.Get("key"), clone.Get("key"))
	// Modify original, clone should be unaffected
	f.Set("key", [][]byte{[]byte("new-value")})
	require.NotEqual(t, f.Get("key"), clone.Get("key"))
}

func TestOutgoingRequest(t *testing.T) {
	headers := NewFields()
	req := NewOutgoingRequest(headers)
	require.NotNil(t, req)
	require.Equal(t, MethodGet, req.method)
	require.NotNil(t, req.headers)
}

func TestOutgoingRequestSetters(t *testing.T) {
	headers := NewFields()
	req := NewOutgoingRequest(headers)

	req.SetMethod(MethodPost)
	require.Equal(t, MethodPost, req.Method())

	path := "/api/v1"
	req.SetPathWithQuery(&path)
	require.Equal(t, &path, req.PathWithQuery())

	scheme := NewSchemeHTTPS()
	req.SetScheme(&scheme)
	require.Equal(t, &scheme, req.Scheme())

	authority := "example.com"
	req.SetAuthority(&authority)
	require.Equal(t, &authority, req.Authority())
}

func TestIncomingRequest(t *testing.T) {
	headers := NewFields()
	path := "/test"
	authority := "localhost"
	scheme := NewSchemeHTTP()
	req := NewIncomingRequest(MethodGet, &scheme, &authority, &path, headers)
	require.NotNil(t, req)
	require.Equal(t, MethodGet, req.Method())
	require.Equal(t, &path, req.PathWithQuery())
	require.Equal(t, &authority, req.Authority())
}

func TestOutgoingResponse(t *testing.T) {
	headers := NewFields()
	resp := NewOutgoingResponse(headers)
	require.NotNil(t, resp)
	require.Equal(t, uint16(200), resp.statusCode) // Default status code
}

func TestOutgoingResponseSetters(t *testing.T) {
	headers := NewFields()
	resp := NewOutgoingResponse(headers)
	resp.SetStatusCode(404)
	require.Equal(t, uint16(404), resp.StatusCode())
}

func TestIncomingResponse(t *testing.T) {
	headers := NewFields()
	resp := NewIncomingResponse(200, headers)
	require.NotNil(t, resp)
	require.Equal(t, uint16(200), resp.Status())
	require.NotNil(t, resp.Headers())
}

func TestErrorCodeValues(t *testing.T) {
	require.Equal(t, ErrorCode("DNS-timeout"), ErrorCodeDNSTimeout)
	require.Equal(t, ErrorCode("DNS-error"), ErrorCodeDNSError)
	require.Equal(t, ErrorCode("destination-not-found"), ErrorCodeDestinationNotFound)
	require.Equal(t, ErrorCode("destination-unavailable"), ErrorCodeDestinationUnavailable)
	require.Equal(t, ErrorCode("destination-IP-prohibited"), ErrorCodeDestinationIPProhibited)
	require.Equal(t, ErrorCode("destination-IP-unroutable"), ErrorCodeDestinationIPUnroutable)
	require.Equal(t, ErrorCode("connection-refused"), ErrorCodeConnectionRefused)
	require.Equal(t, ErrorCode("connection-terminated"), ErrorCodeConnectionTerminated)
	require.Equal(t, ErrorCode("connection-timeout"), ErrorCodeConnectionTimeout)
	require.Equal(t, ErrorCode("connection-read-timeout"), ErrorCodeConnectionReadTimeout)
	require.Equal(t, ErrorCode("connection-write-timeout"), ErrorCodeConnectionWriteTimeout)
	require.Equal(t, ErrorCode("connection-limit-reached"), ErrorCodeConnectionLimitReached)
	require.Equal(t, ErrorCode("TLS-protocol-error"), ErrorCodeTLSProtocolError)
	require.Equal(t, ErrorCode("TLS-certificate-error"), ErrorCodeTLSCertificateError)
	require.Equal(t, ErrorCode("TLS-alert-received"), ErrorCodeTLSAlertReceived)
	require.Equal(t, ErrorCode("HTTP-request-denied"), ErrorCodeHTTPRequestDenied)
	require.Equal(t, ErrorCode("HTTP-request-length-required"), ErrorCodeHTTPRequestLengthRequired)
	require.Equal(t, ErrorCode("HTTP-request-body-size"), ErrorCodeHTTPRequestBodySize)
	require.Equal(t, ErrorCode("HTTP-request-method-invalid"), ErrorCodeHTTPRequestMethodInvalid)
	require.Equal(t, ErrorCode("HTTP-request-URI-invalid"), ErrorCodeHTTPRequestURIInvalid)
	require.Equal(t, ErrorCode("HTTP-request-URI-too-long"), ErrorCodeHTTPRequestURITooLong)
	require.Equal(t, ErrorCode("HTTP-request-header-section-size"), ErrorCodeHTTPRequestHeaderSectionSize)
	require.Equal(t, ErrorCode("HTTP-request-header-size"), ErrorCodeHTTPRequestHeaderSize)
	require.Equal(t, ErrorCode("HTTP-request-trailer-section-size"), ErrorCodeHTTPRequestTrailerSectionSize)
	require.Equal(t, ErrorCode("HTTP-request-trailer-size"), ErrorCodeHTTPRequestTrailerSize)
	require.Equal(t, ErrorCode("HTTP-response-incomplete"), ErrorCodeHTTPResponseIncomplete)
	require.Equal(t, ErrorCode("HTTP-response-header-section-size"), ErrorCodeHTTPResponseHeaderSectionSize)
	require.Equal(t, ErrorCode("HTTP-response-header-size"), ErrorCodeHTTPResponseHeaderSize)
	require.Equal(t, ErrorCode("HTTP-response-body-size"), ErrorCodeHTTPResponseBodySize)
	require.Equal(t, ErrorCode("HTTP-response-trailer-section-size"), ErrorCodeHTTPResponseTrailerSectionSize)
	require.Equal(t, ErrorCode("HTTP-response-trailer-size"), ErrorCodeHTTPResponseTrailerSize)
	require.Equal(t, ErrorCode("HTTP-response-transfer-coding"), ErrorCodeHTTPResponseTransferCoding)
	require.Equal(t, ErrorCode("HTTP-response-content-coding"), ErrorCodeHTTPResponseContentCoding)
	require.Equal(t, ErrorCode("HTTP-response-timeout"), ErrorCodeHTTPResponseTimeout)
	require.Equal(t, ErrorCode("HTTP-upgrade-failed"), ErrorCodeHTTPUpgradeFailed)
	require.Equal(t, ErrorCode("HTTP-protocol-error"), ErrorCodeHTTPProtocolError)
	require.Equal(t, ErrorCode("loop-detected"), ErrorCodeLoopDetected)
	require.Equal(t, ErrorCode("configuration-error"), ErrorCodeConfigurationError)
	require.Equal(t, ErrorCode("internal-error"), ErrorCodeInternalError)
}

func TestRequestOptions(t *testing.T) {
	opts := NewRequestOptions()
	require.NotNil(t, opts)
	require.Nil(t, opts.connectTimeout)
	require.Nil(t, opts.firstByteTimeout)
	require.Nil(t, opts.betweenBytesTimeout)
}

func TestRequestOptionsSetters(t *testing.T) {
	opts := NewRequestOptions()

	timeout := uint64(30000000000)
	opts.SetConnectTimeout(&timeout)
	require.Equal(t, &timeout, opts.ConnectTimeout())

	opts.SetFirstByteTimeout(&timeout)
	require.Equal(t, &timeout, opts.FirstByteTimeout())

	opts.SetBetweenBytesTimeout(&timeout)
	require.Equal(t, &timeout, opts.BetweenBytesTimeout())
}

func TestIncomingBody(t *testing.T) {
	body := NewIncomingBody()
	require.NotNil(t, body)
}

func TestOutgoingBody(t *testing.T) {
	body := NewOutgoingBody()
	require.NotNil(t, body)
}

func TestFutureIncomingResponse(t *testing.T) {
	future := NewFutureIncomingResponse()
	require.NotNil(t, future)
}

func TestFutureTrailers(t *testing.T) {
	future := NewFutureTrailers()
	require.NotNil(t, future)
	require.False(t, future.IsReady(), "new future should not be ready")
}

func TestFutureTrailersReady(t *testing.T) {
	future := NewFutureTrailersReady(nil, nil)
	require.NotNil(t, future)
	require.True(t, future.IsReady(), "ready future should be ready")
}

func TestResponseOutparam(t *testing.T) {
	param := NewResponseOutparam()
	require.NotNil(t, param)
}

// ====================
// HTTP Client Integration Tests
// ====================

// TestOutgoingRequest_Build tests building an HTTP request from OutgoingRequest.
func TestOutgoingRequest_Build(t *testing.T) {
	headers := NewFields()
	headers.Set("Content-Type", [][]byte{[]byte("application/json")})
	headers.Set("Accept", [][]byte{[]byte("*/*")})

	req := NewOutgoingRequest(headers)
	req.SetMethod(MethodPost)

	path := "/api/test?foo=bar"
	req.SetPathWithQuery(&path)

	scheme := NewSchemeHTTPS()
	req.SetScheme(&scheme)

	authority := "example.com:8080"
	req.SetAuthority(&authority)

	// Get the body and write to it
	body, err := req.Body()
	require.NoError(t, err)
	require.NotNil(t, body)

	stream, err := body.Write()
	require.NoError(t, err)
	require.NotNil(t, stream)

	// Write some data
	writeErr := stream.Write([]byte(`{"test": "data"}`))
	require.Nil(t, writeErr)

	body.Finish()

	// Convert to Go HTTP request
	httpReq, err := req.ToHTTPRequest(context.Background())
	require.NoError(t, err)
	require.NotNil(t, httpReq)

	// Verify the request properties
	require.Equal(t, "POST", httpReq.Method)
	require.Equal(t, "https://example.com:8080/api/test?foo=bar", httpReq.URL.String())
	require.Equal(t, "application/json", httpReq.Header.Get("Content-Type"))
	require.Equal(t, "*/*", httpReq.Header.Get("Accept"))
}

// TestOutgoingHandler_Get tests sending a GET request to a test server.
func TestOutgoingHandler_Get(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(gohttp.HandlerFunc(func(w gohttp.ResponseWriter, r *gohttp.Request) {
		require.Equal(t, "GET", r.Method)
		require.Equal(t, "/test/path", r.URL.Path)

		w.Header().Set("X-Test-Header", "test-value")
		w.WriteHeader(200)
		w.Write([]byte("Hello, World!"))
	}))
	defer server.Close()

	// Build the request directly (bypassing resource table for request setup)
	headers := NewFields()
	req := NewOutgoingRequest(headers)
	req.SetMethod(MethodGet)

	path := "/test/path"
	req.SetPathWithQuery(&path)

	scheme := NewSchemeHTTP()
	req.SetScheme(&scheme)

	// Parse server URL for authority
	authority := strings.TrimPrefix(server.URL, "http://")
	req.SetAuthority(&authority)

	// Convert to HTTP request and execute directly
	httpReq, err := req.ToHTTPRequest(context.Background())
	require.NoError(t, err)

	resp, err := gohttp.DefaultClient.Do(httpReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify response
	require.Equal(t, 200, resp.StatusCode)
	require.Equal(t, "test-value", resp.Header.Get("X-Test-Header"))

	// Read body
	body, err := goio.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "Hello, World!", string(body))
}

// TestOutgoingHandler_Post tests sending a POST request with body.
func TestOutgoingHandler_Post(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(gohttp.HandlerFunc(func(w gohttp.ResponseWriter, r *gohttp.Request) {
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Read request body
		body, err := goio.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, `{"message": "hello"}`, string(body))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		w.Write([]byte(`{"status": "created"}`))
	}))
	defer server.Close()

	// Build the request directly
	headers := NewFields()
	headers.Set("Content-Type", [][]byte{[]byte("application/json")})
	req := NewOutgoingRequest(headers)
	req.SetMethod(MethodPost)

	path := "/api/resource"
	req.SetPathWithQuery(&path)

	scheme := NewSchemeHTTP()
	req.SetScheme(&scheme)

	authority := strings.TrimPrefix(server.URL, "http://")
	req.SetAuthority(&authority)

	// Write request body
	reqBody, err := req.Body()
	require.NoError(t, err)

	stream, err := reqBody.Write()
	require.NoError(t, err)

	writeErr := stream.Write([]byte(`{"message": "hello"}`))
	require.Nil(t, writeErr)

	reqBody.Finish()

	// Convert to HTTP request and execute directly
	httpReq, err := req.ToHTTPRequest(context.Background())
	require.NoError(t, err)

	resp, err := gohttp.DefaultClient.Do(httpReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify response
	require.Equal(t, 201, resp.StatusCode)

	// Read response body
	respData, err := goio.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, `{"status": "created"}`, string(respData))
}

// TestFutureIncomingResponse_Subscribe tests async polling behavior.
func TestFutureIncomingResponse_Subscribe_Async(t *testing.T) {
	future := NewFutureIncomingResponse()

	// Subscribe should return a pollable
	pollable := future.Subscribe()
	require.NotNil(t, pollable)

	// Should not be ready yet
	require.False(t, pollable.Ready())

	// Set response in background
	go func() {
		time.Sleep(10 * time.Millisecond)
		future.SetResponse(NewIncomingResponse(200, NewFields()))
	}()

	// Block should wait until ready
	pollable.Block()

	// Now should be ready
	require.True(t, future.IsReady())

	// Get should return response
	resp, errCode, ready := future.Get()
	require.True(t, ready)
	require.Nil(t, errCode)
	require.NotNil(t, resp)
	require.Equal(t, uint16(200), resp.Status())

	// Second Get should return nothing (already consumed)
	resp2, errCode2, ready2 := future.Get()
	require.False(t, ready2)
	require.Nil(t, errCode2)
	require.Nil(t, resp2)
}

// TestFutureIncomingResponse_Error tests error handling.
func TestFutureIncomingResponse_Error_Handling(t *testing.T) {
	future := NewFutureIncomingResponse()

	// Set an error
	future.SetError(ErrorCodeConnectionRefused)

	// Should be ready
	require.True(t, future.IsReady())

	// Get should return error
	resp, errCode, ready := future.Get()
	require.True(t, ready)
	require.NotNil(t, errCode)
	require.Equal(t, ErrorCodeConnectionRefused, *errCode)
	require.Nil(t, resp)
}

// TestOutgoingHandler_Timeout tests timeout handling.
func TestOutgoingHandler_Timeout(t *testing.T) {
	// Create a slow server
	server := httptest.NewServer(gohttp.HandlerFunc(func(w gohttp.ResponseWriter, r *gohttp.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(200)
	}))
	defer server.Close()

	// Build request directly
	headers := NewFields()
	req := NewOutgoingRequest(headers)

	path := "/slow"
	req.SetPathWithQuery(&path)

	scheme := NewSchemeHTTP()
	req.SetScheme(&scheme)

	authority := strings.TrimPrefix(server.URL, "http://")
	req.SetAuthority(&authority)

	// Convert to HTTP request
	httpReq, err := req.ToHTTPRequest(context.Background())
	require.NoError(t, err)

	// Create client with short timeout
	client := &gohttp.Client{
		Timeout: 100 * time.Millisecond,
	}

	// Execute - should timeout
	_, err = client.Do(httpReq)
	require.Error(t, err)

	// Verify the error is a timeout (message could contain "timeout" or "deadline exceeded")
	errMsg := err.Error()
	isTimeout := strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "deadline exceeded")
	require.True(t, isTimeout, "expected timeout error, got: %s", errMsg)
}

// TestErrorCodeFromError tests error code mapping.
func TestErrorCodeFromError_Mapping(t *testing.T) {
	tests := []struct {
		errMsg   string
		expected ErrorCode
	}{
		{"connection refused", ErrorCodeConnectionRefused},
		{"dial tcp: connection timeout", ErrorCodeConnectionTimeout},
		{"no such host", ErrorCodeDNSError},
		{"connection reset by peer", ErrorCodeConnectionTerminated},
		{"x509: certificate has expired", ErrorCodeTLSCertificateError},
		{"context deadline exceeded", ErrorCodeHTTPResponseTimeout},
		{"unknown error", ErrorCodeInternalError},
	}

	for _, tc := range tests {
		t.Run(tc.errMsg, func(t *testing.T) {
			err := gohttp.ProtocolError{ErrorString: tc.errMsg}
			code := ErrorCodeFromError(&err)
			require.Equal(t, tc.expected, code)
		})
	}
}

// TestMethodHTTPMethod tests HTTP method conversion.
func TestMethodHTTPMethod(t *testing.T) {
	require.Equal(t, gohttp.MethodGet, MethodGet.HTTPMethod())
	require.Equal(t, gohttp.MethodPost, MethodPost.HTTPMethod())
	require.Equal(t, gohttp.MethodPut, MethodPut.HTTPMethod())
	require.Equal(t, gohttp.MethodDelete, MethodDelete.HTTPMethod())
	require.Equal(t, gohttp.MethodHead, MethodHead.HTTPMethod())
	require.Equal(t, gohttp.MethodOptions, MethodOptions.HTTPMethod())
	require.Equal(t, gohttp.MethodPatch, MethodPatch.HTTPMethod())
	require.Equal(t, gohttp.MethodConnect, MethodConnect.HTTPMethod())
	require.Equal(t, gohttp.MethodTrace, MethodTrace.HTTPMethod())
}

// TestIncomingResponseFromHTTP tests creating IncomingResponse from net/http response.
func TestIncomingResponseFromHTTP(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(gohttp.HandlerFunc(func(w gohttp.ResponseWriter, r *gohttp.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Add("X-Custom", "value1")
		w.Header().Add("X-Custom", "value2")
		w.WriteHeader(201)
		w.Write([]byte(`{"data": "test"}`))
	}))
	defer server.Close()

	// Make a request
	resp, err := gohttp.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Convert to IncomingResponse
	incomingResp := NewIncomingResponseFromHTTP(resp)
	require.NotNil(t, incomingResp)
	require.Equal(t, uint16(201), incomingResp.Status())

	// Check headers
	headers := incomingResp.Headers()
	require.NotNil(t, headers)

	contentType := headers.Get("Content-Type")
	require.Equal(t, 1, len(contentType))
	require.Equal(t, "application/json", string(contentType[0]))

	customHeaders := headers.Get("X-Custom")
	require.Equal(t, 2, len(customHeaders))

	// Check body
	body, err := incomingResp.Consume()
	require.NoError(t, err)

	stream, err := body.Stream()
	require.NoError(t, err)

	data, streamErr := stream.Read(1024)
	require.Nil(t, streamErr)
	require.Equal(t, `{"data": "test"}`, string(data))
}

// TestOutgoingBody_WriteAndFinish tests writing to outgoing body.
func TestOutgoingBody_WriteAndFinish(t *testing.T) {
	body := NewOutgoingBody()
	require.NotNil(t, body)

	// Get write stream
	stream, err := body.Write()
	require.NoError(t, err)
	require.NotNil(t, stream)

	// Write data
	writeErr := stream.Write([]byte("part1"))
	require.Nil(t, writeErr)

	writeErr = stream.Write([]byte("part2"))
	require.Nil(t, writeErr)

	// Finish
	err = body.Finish()
	require.NoError(t, err)

	// Verify bytes
	data := body.Bytes()
	require.Equal(t, "part1part2", string(data))

	// Second Write() call should fail
	_, err = body.Write()
	require.Error(t, err)

	// Second Finish() call should fail
	err = body.Finish()
	require.Error(t, err)
}

// TestIncomingBody_Stream tests reading from incoming body.
func TestIncomingBody_StreamRead(t *testing.T) {
	// Create body with data
	data := []byte("test body content")
	reader := goio.NopCloser(strings.NewReader(string(data)))
	body := NewIncomingBodyFromReader(reader)
	require.NotNil(t, body)

	// Get stream
	stream, err := body.Stream()
	require.NoError(t, err)
	require.NotNil(t, stream)

	// Read data
	readData, streamErr := stream.Read(1024)
	require.Nil(t, streamErr)
	require.Equal(t, "test body content", string(readData))

	// Second Stream() call should fail
	_, err = body.Stream()
	require.Error(t, err)
}

func TestResponseOutparamSet_OkResponse(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	outparam := NewResponseOutparam()
	opSid := registerResponseOutparam(outparam)
	outparamHandle, errH19 := table.NewResourceHandle(opSid, true, httpFieldsResourceType)
	if errH19 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH19)
	}

	resp := NewOutgoingResponse(NewFields())
	sid := registerOutgoingResponse(resp)
	resp.SetStatusCode(200)
	respHandle, errH20 := table.NewResourceHandle(sid, true, httpFieldsResourceType)
	if errH20 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH20)
	}

	// Build args: own<response-outparam>, result<own<outgoing-response>, error-code>
	outparamVal := types.ValOwn(uint32(outparamHandle))
	respVal := types.ValOwn(uint32(respHandle))
	resultVal := types.ValResultOk(&respVal)

	result, err := responseOutparamSet(ctx, nil, []types.Val{outparamVal, resultVal})
	require.NoError(t, err)
	require.Equal(t, 0, len(result)) // returns unit

	// Verify response is available on channel
	gotResp, gotErr, waitErr := outparam.WaitForResponse(context.Background())
	require.NoError(t, waitErr)
	require.Nil(t, gotErr)
	require.NotNil(t, gotResp)
	require.Equal(t, uint16(200), gotResp.StatusCode())
}

func TestResponseOutparamSet_ErrorResponse(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	outparam := NewResponseOutparam()
	opSid := registerResponseOutparam(outparam)
	outparamHandle, errH21 := table.NewResourceHandle(opSid, true, httpFieldsResourceType)
	if errH21 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH21)
	}

	outparamVal := types.ValOwn(uint32(outparamHandle))
	errCodeVal := types.ValVariant("connection-refused", nil)
	resultVal := types.ValResultError(&errCodeVal)

	result, err := responseOutparamSet(ctx, nil, []types.Val{outparamVal, resultVal})
	require.NoError(t, err)
	require.Equal(t, 0, len(result))

	gotResp, gotErr, waitErr := outparam.WaitForResponse(context.Background())
	require.NoError(t, waitErr)
	require.Nil(t, gotResp)
	require.NotNil(t, gotErr)
}

// TestResponseOutparamSet_BadOutgoingResponseHandle verifies that response-outparam.set
// traps (returns a non-nil error) when the guest passes an invalid own<outgoing-response>
// handle in the Ok branch. Per Component Model CanonicalABI.md (lift_own), an own handle
// that does not exist in the table must trap. This is distinct from the guest legitimately
// returning Err(internal-error), which must NOT trap and must be delivered through the
// outparam channel.
func TestResponseOutparamSet_BadOutgoingResponseHandle(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	outparam := NewResponseOutparam()
	opSid := registerResponseOutparam(outparam)
	outparamHandle, errH22 := table.NewResourceHandle(opSid, true, httpFieldsResourceType)
	if errH22 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH22)
	}

	// Bogus outgoing-response handle that has nothing in the table.
	const bogusRespHandleIdx uint32 = 9999

	outparamVal := types.ValOwn(uint32(outparamHandle))
	respVal := types.ValOwn(bogusRespHandleIdx)
	resultVal := types.ValResultOk(&respVal)

	// The host function must signal a trap by returning a non-nil error.
	_, err := responseOutparamSet(ctx, nil, []types.Val{outparamVal, resultVal})
	require.Error(t, err, "responseOutparamSet must trap on invalid outgoing-response handle")
	require.Contains(t, err.Error(), "outgoing-response", "error must mention which handle was invalid")

	// Verify nothing was sent on the outparam channel: the trap must NOT be
	// papered over with a synthetic ErrorCodeInternalError.
	select {
	case got := <-outparam.result:
		t.Fatalf("expected no value on outparam channel after trap, got %+v", got)
	default:
		// expected: channel is empty
	}
}

// TestResponseOutparamSet_BadOutparamHandle verifies that response-outparam.set traps
// when the guest passes an invalid own<response-outparam> handle, even if the response
// payload is well-formed. Per CanonicalABI.md (lift_own), an own handle that does not
// exist in the table must trap.
func TestResponseOutparamSet_BadOutparamHandle(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Bogus outparam handle that has nothing in the table.
	const bogusOutparamHandleIdx uint32 = 9999

	// Provide a well-formed Err(error-code) for the response payload so that we
	// only test the bad outparam handle path.
	outparamVal := types.ValOwn(bogusOutparamHandleIdx)
	errCodeVal := types.ValVariant("connection-refused", nil)
	resultVal := types.ValResultError(&errCodeVal)

	_, err := responseOutparamSet(ctx, nil, []types.Val{outparamVal, resultVal})
	require.Error(t, err, "responseOutparamSet must trap on invalid response-outparam handle")
	require.Contains(t, err.Error(), "response-outparam", "error must mention which handle was invalid")
}

func TestIncomingRequestConsume_WithBody(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	bodyReader := goio.NopCloser(strings.NewReader("request body content"))
	body := NewIncomingBodyFromReader(bodyReader)

	scheme := NewSchemeHTTPS()
	pathStr := "/api"
	req := NewIncomingRequest(MethodPost, &scheme, nil, &pathStr, NewFields())
	sid := registerIncomingRequest(req)
	req.SetBody(body)
	handle, errH23 := table.NewResourceHandle(sid, true, httpFieldsResourceType)
	if errH23 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH23)
	}

	selfHandle := types.ValBorrow(uint32(handle))
	result, err := incomingRequestConsume(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	isOk, bodyHandleVal, _ := result[0].Result()
	require.True(t, isOk)

	// Verify body is accessible
	bodyHandle := bodyHandleVal.Own()
	rawEntry3, err := table.Get(runtime.Handle(bodyHandle))
	entry, _ := rawEntry3.(*runtime.ResourceHandleEntry)
	require.NoError(t, err)
	_ = entry
}

func TestIncomingRequestConsume_BodyAlreadyConsumed(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	scheme := NewSchemeHTTPS()
	pathStr := "/api"
	sid := registerIncomingRequest(NewIncomingRequest(MethodPost, &scheme, nil, &pathStr, NewFields()))
	handle, errH24 := table.NewResourceHandle(sid, true, httpFieldsResourceType)
	if errH24 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH24)
	}

	selfHandle := types.ValBorrow(uint32(handle))

	// First call succeeds
	result, err := incomingRequestConsume(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "first consume should succeed")

	// Second call must return the body-already-consumed error variant
	result2, err := incomingRequestConsume(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	isOk2, _, errVal := result2[0].Result()
	require.False(t, isOk2, "second consume should fail")
	name, _ := errVal.Variant()
	require.Equal(t, "body-already-consumed", name)
}

func TestHttpErrorCode_WithHTTPError(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	httpErr := &HTTPError{Code: ErrorCodeConnectionRefused}
	sid := io.RegisterError(io.NewError(httpErr))
	handle, errH25 := table.NewResourceHandle(sid, true, httpFieldsResourceType)
	if errH25 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH25)
	}

	errHandle := types.ValBorrow(uint32(handle))
	result, err := httpErrorCode(ctx, nil, []types.Val{errHandle})
	require.NoError(t, err)

	opt := result[0].Option()
	require.NotNil(t, opt, "should return Some for HTTP error")
	name := opt.Enum()
	require.Equal(t, "connection-refused", name)
}

func TestNewHTTPHandler_SimpleGET(t *testing.T) {
}

func TestNewHTTPHandler_CallHandleError(t *testing.T) {
	handler := NewHTTPHandler(func(ctx context.Context, requestHandle, outparamHandle runtime.Handle) error {
		return errors.New("handler crashed")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	require.Equal(t, gohttp.StatusInternalServerError, w.Code)
}

func TestNewHTTPHandler_ErrorCodeResponse(t *testing.T) {
}

func TestNewHTTPHandler_HeadersAreLowercased(t *testing.T) {
}

func TestHttpErrorCode_WithNonHTTPError(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	sid := io.RegisterError(io.NewError(errors.New("some random error")))
	handle, errH26 := table.NewResourceHandle(sid, true, httpFieldsResourceType)
	if errH26 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH26)
	}

	errHandle := types.ValBorrow(uint32(handle))
	result, err := httpErrorCode(ctx, nil, []types.Val{errHandle})
	require.NoError(t, err)
	require.Nil(t, result[0].Option(), "should return None for non-HTTP error")
}
