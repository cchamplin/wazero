# Phase 5: wasi:http/outgoing-handler - HTTP Client

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Connect the HTTP outgoing-handler to Go's http.Client, enabling WASI components to make HTTP requests.

**Architecture:** Implement the handle function to convert WASI outgoing-request to Go http.Request, execute via http.Client, and stream response back through WASI incoming-response/body types.

**Tech Stack:** Go net/http package, http.Client, io streaming

**Gap Analysis Reference:** [wasip2-interface-gap-analysis.md](../wasip2-interface-gap-analysis.md) - Section 7.2 wasi:http/outgoing-handler@0.2.0

**Prerequisite:** Phase 4 (DNS) should be complete for hostname resolution.

---

## Reference Materials

### Specification
- **WIT File:** `debug-vendored/WASI/proposals/http/wit/handler.wit`
- **Key Functions:**
  - `handle: func(request: outgoing-request, options: option<request-options>) -> result<future-incoming-response, error-code>`

### Current Implementation
- **File:** `imports/wasip2/http/http.go`
- **Issues:**
  - handle function is a stub that returns error
  - HTTP types (request, response, body) exist but aren't connected to actual HTTP

### Wasmtime Reference
- **File:** `debug-vendored/wasmtime/crates/wasi-http/src/lib.rs`
- Look for outgoing_handler implementation

---

## Task 5.1: Create HTTP Client Wrapper

**Files:**
- Create: `imports/wasip2/http/client.go`
- Test: `imports/wasip2/http/client_test.go`

**Step 1: Write failing test**

Create `imports/wasip2/http/client_test.go`:

```go
package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPClient_Get(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(200)
		w.Write([]byte("Hello, World!"))
	}))
	defer server.Close()

	client := NewHTTPClient()

	// Create outgoing request
	req := &OutgoingRequest{
		method:    MethodGet,
		scheme:    SchemeHTTP,
		authority: server.Listener.Addr().String(),
		path:      "/test",
		headers:   NewFields(),
	}

	// Execute request
	futureResp, err := client.Handle(req, nil)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	// Wait for response
	resp, err := futureResp.Get()
	if err != nil {
		t.Fatalf("Get response failed: %v", err)
	}

	if resp.StatusCode() != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode())
	}
}

func TestHTTPClient_Post(t *testing.T) {
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, 1024)
		n, _ := r.Body.Read(body)
		receivedBody = body[:n]
		w.WriteHeader(201)
	}))
	defer server.Close()

	client := NewHTTPClient()

	req := &OutgoingRequest{
		method:    MethodPost,
		scheme:    SchemeHTTP,
		authority: server.Listener.Addr().String(),
		path:      "/submit",
		headers:   NewFields(),
	}

	// Set request body
	body := req.Body()
	body.Write([]byte("test body"))
	body.Finish(nil)

	futureResp, err := client.Handle(req, nil)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	resp, err := futureResp.Get()
	if err != nil {
		t.Fatalf("Get response failed: %v", err)
	}

	if resp.StatusCode() != 201 {
		t.Errorf("expected status 201, got %d", resp.StatusCode())
	}

	if string(receivedBody) != "test body" {
		t.Errorf("expected 'test body', got '%s'", string(receivedBody))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/http/... -run "TestHTTPClient"`
Expected: FAIL - HTTPClient, Handle don't exist

**Step 3: Create client.go with HTTPClient**

Create `imports/wasip2/http/client.go`:

```go
package http

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPClient wraps Go's http.Client for WASI compatibility
type HTTPClient struct {
	client *http.Client
}

// NewHTTPClient creates a new HTTP client
func NewHTTPClient() *HTTPClient {
	return &HTTPClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewHTTPClientWithTimeout creates a client with custom timeout
func NewHTTPClientWithTimeout(timeout time.Duration) *HTTPClient {
	return &HTTPClient{
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Handle executes an outgoing HTTP request
func (c *HTTPClient) Handle(req *OutgoingRequest, options *RequestOptions) (*FutureIncomingResponse, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}

	// Build URL
	scheme := "http"
	if req.scheme == SchemeHTTPS {
		scheme = "https"
	}

	path := req.path
	if path == "" {
		path = "/"
	}

	url := fmt.Sprintf("%s://%s%s", scheme, req.authority, path)

	// Get request body
	var bodyReader io.Reader
	if req.body != nil && req.body.outputStream != nil {
		bodyReader = bytes.NewReader(req.body.outputStream.Bytes())
	}

	// Create Go request
	goReq, err := http.NewRequest(methodToString(req.method), url, bodyReader)
	if err != nil {
		return nil, err
	}

	// Copy headers
	if req.headers != nil {
		for key, values := range req.headers.entries {
			for _, value := range values {
				goReq.Header.Add(key, string(value))
			}
		}
	}

	// Apply request options
	timeout := c.client.Timeout
	if options != nil && options.connectTimeout > 0 {
		timeout = options.connectTimeout
	}

	// Create future response
	future := &FutureIncomingResponse{
		ready: make(chan struct{}),
	}

	// Execute request in background
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		goReq = goReq.WithContext(ctx)

		resp, err := c.client.Do(goReq)
		if err != nil {
			future.err = err
			close(future.ready)
			return
		}

		// Convert to WASI response
		future.response = goResponseToWASI(resp)
		close(future.ready)
	}()

	return future, nil
}

func methodToString(m Method) string {
	switch m {
	case MethodGet:
		return "GET"
	case MethodHead:
		return "HEAD"
	case MethodPost:
		return "POST"
	case MethodPut:
		return "PUT"
	case MethodDelete:
		return "DELETE"
	case MethodConnect:
		return "CONNECT"
	case MethodOptions:
		return "OPTIONS"
	case MethodTrace:
		return "TRACE"
	case MethodPatch:
		return "PATCH"
	default:
		return "GET"
	}
}

func goResponseToWASI(resp *http.Response) *IncomingResponse {
	// Create WASI response
	wasiResp := &IncomingResponse{
		status: uint16(resp.StatusCode),
	}

	// Copy headers
	wasiResp.headers = NewFields()
	for key, values := range resp.Header {
		for _, value := range values {
			wasiResp.headers.Append(key, []byte(value))
		}
	}

	// Create body
	wasiResp.body = &IncomingBody{
		reader: resp.Body,
	}

	return wasiResp
}

// RequestOptions contains options for HTTP requests
type RequestOptions struct {
	connectTimeout  time.Duration
	firstByteTimeout time.Duration
	betweenBytesTimeout time.Duration
}

// NewRequestOptions creates new request options
func NewRequestOptions() *RequestOptions {
	return &RequestOptions{}
}

func (o *RequestOptions) SetConnectTimeout(d time.Duration) {
	o.connectTimeout = d
}

func (o *RequestOptions) SetFirstByteTimeout(d time.Duration) {
	o.firstByteTimeout = d
}

func (o *RequestOptions) SetBetweenBytesTimeout(d time.Duration) {
	o.betweenBytesTimeout = d
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/http/... -run "TestHTTPClient"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/http/client.go imports/wasip2/http/client_test.go
git commit -m "feat(wasip2): add HTTPClient wrapper for outgoing requests

Executes HTTP requests using Go's http.Client with WASI type conversion."
```

---

## Task 5.2: Implement FutureIncomingResponse

**Files:**
- Modify: `imports/wasip2/http/http.go`
- Test: `imports/wasip2/http/client_test.go`

**Step 1: Write failing test**

Add to `imports/wasip2/http/client_test.go`:

```go
func TestFutureIncomingResponse_Subscribe(t *testing.T) {
	future := &FutureIncomingResponse{
		ready: make(chan struct{}),
	}

	pollable := future.Subscribe()
	if pollable == nil {
		t.Fatal("expected pollable")
	}

	// Initially not ready
	if pollable.IsReady() {
		t.Error("pollable should not be ready initially")
	}

	// Signal ready
	close(future.ready)

	// Wait a moment for pollable to update
	time.Sleep(10 * time.Millisecond)

	// Now should be ready
	if !pollable.IsReady() {
		t.Error("pollable should be ready after response")
	}
}

func TestFutureIncomingResponse_Get(t *testing.T) {
	resp := &IncomingResponse{status: 200}

	future := &FutureIncomingResponse{
		ready:    make(chan struct{}),
		response: resp,
	}
	close(future.ready)

	result, err := future.Get()
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if result != resp {
		t.Error("expected same response object")
	}

	// Second call should return nil (consumed)
	result2, err := future.Get()
	if err != nil {
		t.Fatalf("second Get failed: %v", err)
	}
	if result2 != nil {
		t.Error("expected nil on second get")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/http/... -run "TestFutureIncomingResponse"`
Expected: FAIL - FutureIncomingResponse methods don't exist

**Step 3: Update FutureIncomingResponse in http.go**

Find and update `FutureIncomingResponse` in `imports/wasip2/http/http.go`:

```go
import (
	"sync"

	wasip2io "github.com/tetratelabs/wazero/imports/wasip2/io"
)

// FutureIncomingResponse represents an async HTTP response
type FutureIncomingResponse struct {
	mu       sync.Mutex
	ready    chan struct{}
	response *IncomingResponse
	err      error
	consumed bool
	pollable *wasip2io.Pollable
}

// Subscribe returns a pollable for response readiness
func (f *FutureIncomingResponse) Subscribe() *wasip2io.Pollable {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.pollable != nil {
		return f.pollable
	}

	f.pollable = wasip2io.NewPollable()

	// Watch for ready signal and update pollable
	go func() {
		<-f.ready
		f.pollable.SetReady()
	}()

	return f.pollable
}

// Get returns the response, consuming the future
// Returns (response, nil) on success
// Returns (nil, error) on failure
// Returns (nil, nil) if already consumed
func (f *FutureIncomingResponse) Get() (*IncomingResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.consumed {
		return nil, nil
	}

	// Wait for ready
	<-f.ready

	f.consumed = true

	if f.err != nil {
		return nil, f.err
	}

	return f.response, nil
}

// IsReady returns true if the response is available
func (f *FutureIncomingResponse) IsReady() bool {
	select {
	case <-f.ready:
		return true
	default:
		return false
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/http/... -run "TestFutureIncomingResponse"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/http/http.go imports/wasip2/http/client_test.go
git commit -m "feat(wasip2): implement FutureIncomingResponse with Subscribe/Get

Async response handling with pollable integration."
```

---

## Task 5.3: Implement Request/Response Body Streaming

**Files:**
- Modify: `imports/wasip2/http/http.go`
- Test: `imports/wasip2/http/client_test.go`

**Step 1: Write failing test**

Add to `imports/wasip2/http/client_test.go`:

```go
func TestIncomingBody_Stream(t *testing.T) {
	data := []byte("response body content")
	body := &IncomingBody{
		reader: io.NopCloser(bytes.NewReader(data)),
	}

	// Get stream
	stream := body.Stream()
	if stream == nil {
		t.Fatal("expected stream")
	}

	// Read via stream
	readData, err := stream.BlockingRead(100)
	if err != nil {
		t.Fatalf("BlockingRead failed: %v", err)
	}

	if string(readData) != string(data) {
		t.Errorf("expected '%s', got '%s'", string(data), string(readData))
	}
}

func TestOutgoingBody_Write(t *testing.T) {
	body := &OutgoingBody{
		outputStream: &bytes.Buffer{},
	}

	stream := body.Write()
	if stream == nil {
		t.Fatal("expected stream")
	}

	// Write via stream
	err := stream.BlockingWriteAndFlush([]byte("request body"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Verify content
	if body.outputStream.String() != "request body" {
		t.Errorf("expected 'request body', got '%s'", body.outputStream.String())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/http/... -run "TestIncomingBody|TestOutgoingBody"`
Expected: FAIL - Stream, Write, body types incomplete

**Step 3: Update body implementations in http.go**

Update `IncomingBody` and `OutgoingBody` in `imports/wasip2/http/http.go`:

```go
import (
	"bytes"
	"io"

	wasip2io "github.com/tetratelabs/wazero/imports/wasip2/io"
)

// IncomingBody wraps an HTTP response body
type IncomingBody struct {
	reader      io.ReadCloser
	inputStream *wasip2io.InputStream
	consumed    bool
}

// Stream returns the input stream for reading the body
func (b *IncomingBody) Stream() *wasip2io.InputStream {
	if b.inputStream != nil {
		return b.inputStream
	}

	b.inputStream = wasip2io.NewInputStream(b.reader)
	return b.inputStream
}

// Finish signals that body consumption is complete
// Returns future trailers if any
func (b *IncomingBody) Finish() *FutureTrailers {
	if b.reader != nil {
		b.reader.Close()
	}
	b.consumed = true

	// Return empty trailers future
	return &FutureTrailers{
		ready: make(chan struct{}),
	}
}

// OutgoingBody wraps an HTTP request body being built
type OutgoingBody struct {
	outputStream *bytes.Buffer
	wasiStream   *wasip2io.OutputStream
	finished     bool
	trailers     *Fields
}

// NewOutgoingBody creates a new outgoing body
func NewOutgoingBody() *OutgoingBody {
	buf := &bytes.Buffer{}
	return &OutgoingBody{
		outputStream: buf,
		wasiStream:   wasip2io.NewOutputStream(buf),
	}
}

// Write returns the output stream for writing the body
func (b *OutgoingBody) Write() *wasip2io.OutputStream {
	return b.wasiStream
}

// Finish signals that body writing is complete
func (b *OutgoingBody) Finish(trailers *Fields) error {
	b.finished = true
	b.trailers = trailers
	return nil
}

// Bytes returns the written body content
func (b *OutgoingBody) Bytes() []byte {
	if b.outputStream == nil {
		return nil
	}
	return b.outputStream.Bytes()
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/http/... -run "TestIncomingBody|TestOutgoingBody"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/http/http.go imports/wasip2/http/client_test.go
git commit -m "feat(wasip2): implement HTTP body streaming

IncomingBody returns InputStream, OutgoingBody provides OutputStream."
```

---

## Task 5.4: Wire outgoing-handler to WASI Interface

**Files:**
- Modify: `imports/wasip2/http/http.go`

**Step 1: Update handle function binding**

Find and update the `handle` function in `imports/wasip2/http/http.go`:

```go
// Global HTTP client instance
var defaultHTTPClient = NewHTTPClient()

// wasiHandle implements handle: func(request: outgoing-request, options: option<request-options>) -> result<future-incoming-response, error-code>
func wasiHandle(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return httpErrorResult(HTTPErrorCodeInternalError), nil
	}

	// Get request from resource table
	reqHandle := args[0].Own()
	resource := table.Get(int(reqHandle))
	req, ok := resource.(*OutgoingRequest)
	if !ok {
		return httpErrorResult(HTTPErrorCodeInternalError), nil
	}

	// Parse options if provided
	var options *RequestOptions
	if optVal := args[1].Option(); optVal != nil {
		// Extract request options from component value
		options = parseRequestOptions(*optVal)
	}

	// Execute request
	future, err := defaultHTTPClient.Handle(req, options)
	if err != nil {
		return httpErrorResult(HTTPErrorCodeInternalError), nil
	}

	// Register future in resource table
	futureHandle := table.New(future, true)

	return []component.Val{component.ValResultOk(component.ValOwn(uint32(futureHandle)))}, nil
}

func parseRequestOptions(val component.Val) *RequestOptions {
	// Request options is a record with optional timeout fields
	// For now, return nil to use defaults
	// TODO: Parse connect-timeout, first-byte-timeout, between-bytes-timeout
	return nil
}

func httpErrorResult(code HTTPErrorCode) []component.Val {
	return []component.Val{component.ValResultErr(component.ValU8(uint8(code)))}
}

type HTTPErrorCode uint8

const (
	HTTPErrorCodeDNSTimeout HTTPErrorCode = iota
	HTTPErrorCodeDNSError
	HTTPErrorCodeDestinationNotFound
	HTTPErrorCodeDestinationUnavailable
	HTTPErrorCodeDestinationIPProhibited
	HTTPErrorCodeDestinationIPUnroutable
	HTTPErrorCodeConnectionRefused
	HTTPErrorCodeConnectionTerminated
	HTTPErrorCodeConnectionTimeout
	HTTPErrorCodeConnectionReadTimeout
	HTTPErrorCodeConnectionWriteTimeout
	HTTPErrorCodeConnectionLimitReached
	HTTPErrorCodeTLSProtocolError
	HTTPErrorCodeTLSCertificateError
	HTTPErrorCodeTLSAlertReceived
	HTTPErrorCodeHTTPRequestDenied
	HTTPErrorCodeHTTPRequestLengthRequired
	HTTPErrorCodeHTTPRequestBodySize
	HTTPErrorCodeHTTPRequestMethodInvalid
	HTTPErrorCodeHTTPRequestURIInvalid
	HTTPErrorCodeHTTPRequestURITooLong
	HTTPErrorCodeHTTPRequestHeaderSectionSize
	HTTPErrorCodeHTTPRequestHeaderSize
	HTTPErrorCodeHTTPRequestTrailerSectionSize
	HTTPErrorCodeHTTPRequestTrailerSize
	HTTPErrorCodeHTTPResponseIncomplete
	HTTPErrorCodeHTTPResponseHeaderSectionSize
	HTTPErrorCodeHTTPResponseHeaderSize
	HTTPErrorCodeHTTPResponseBodySize
	HTTPErrorCodeHTTPResponseTrailerSectionSize
	HTTPErrorCodeHTTPResponseTrailerSize
	HTTPErrorCodeHTTPResponseTransferCoding
	HTTPErrorCodeHTTPResponseContentCoding
	HTTPErrorCodeHTTPResponseTimeout
	HTTPErrorCodeHTTPUpgradeFailed
	HTTPErrorCodeHTTPProtocolError
	HTTPErrorCodeLoopDetected
	HTTPErrorCodeConfigurationError
	HTTPErrorCodeInternalError
)
```

**Step 2: Update Instantiate to use new handle**

In `imports/wasip2/http/http.go`, ensure the outgoing-handler uses `wasiHandle`:

```go
func instantiateOutgoingHandler(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:http/outgoing-handler@0.2.0")

	inst.FuncNoType("handle", wasiHandle)

	return inst.Build()
}
```

**Step 3: Run tests**

Run: `go test -v ./imports/wasip2/http/...`
Expected: PASS

**Step 4: Commit**

```bash
git add imports/wasip2/http/http.go
git commit -m "feat(wasip2): connect outgoing-handler to HTTP client

WASI components can now make HTTP requests via wasi:http/outgoing-handler."
```

---

## Task 5.5: Add Integration Test

**Files:**
- Test: `imports/wasip2/http/integration_test.go`

**Step 1: Create integration test**

Create `imports/wasip2/http/integration_test.go`:

```go
//go:build integration

package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
)

func TestWASIHTTP_Integration(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "test-value")
		w.WriteHeader(200)
		w.Write([]byte("Integration test response"))
	}))
	defer server.Close()

	// Set up context with resource table
	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.ContextWithResourceTable(ctx, table)

	// Create outgoing request
	req := &OutgoingRequest{
		method:    MethodGet,
		scheme:    SchemeHTTP,
		authority: server.Listener.Addr().String(),
		path:      "/",
		headers:   NewFields(),
	}
	reqHandle := table.New(req, true)

	// Call WASI handle function
	args := []component.Val{
		component.ValOwn(uint32(reqHandle)),
		component.ValOption(nil), // No options
	}

	result, err := wasiHandle(ctx, args)
	if err != nil {
		t.Fatalf("wasiHandle failed: %v", err)
	}

	// Extract future handle from result
	isOk, futureVal, _ := result[0].Result()
	if !isOk {
		t.Fatal("expected ok result")
	}

	futureHandle := futureVal.Own()
	futureResource := table.Get(int(futureHandle))
	future, ok := futureResource.(*FutureIncomingResponse)
	if !ok {
		t.Fatal("expected FutureIncomingResponse")
	}

	// Get response
	resp, err := future.Get()
	if err != nil {
		t.Fatalf("Get response failed: %v", err)
	}

	if resp.StatusCode() != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode())
	}

	// Check custom header
	values := resp.headers.Get("X-Custom")
	if len(values) == 0 || string(values[0]) != "test-value" {
		t.Errorf("expected X-Custom header with 'test-value'")
	}

	// Read body
	stream := resp.body.Stream()
	bodyData, _ := stream.BlockingRead(1024)
	if string(bodyData) != "Integration test response" {
		t.Errorf("expected 'Integration test response', got '%s'", string(bodyData))
	}
}
```

**Step 2: Run integration test**

Run: `go test -v -tags=integration ./imports/wasip2/http/... -run "TestWASIHTTP_Integration"`
Expected: PASS

**Step 3: Commit**

```bash
git add imports/wasip2/http/integration_test.go
git commit -m "test(wasip2): add HTTP integration test

Verifies full WASI HTTP request/response flow."
```

---

## Phase 5 Checkpoint

**Run calculator tests to verify no regressions:**

```bash
go test -v ./internal/component/wasip2test/... -run "TestCalculator"
```

**Expected:** All tests pass for `add` and `subtract` plugins.

**If tests fail:**
1. Check that HTTP types don't conflict with existing imports
2. Verify the wasip2io import path is correct
3. Check component.Val methods exist
4. Debug and fix before proceeding to Phase 6

**Mark Phase 5 complete in README.md**
