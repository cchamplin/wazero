// imports/wasip2/http/types.go

// Package http implements the wasi:http interfaces for WASI Preview 2.
// It provides HTTP client and server capabilities.
package http

import (
	"bytes"
	"context"
	"fmt"
	goio "io"
	gohttp "net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tetratelabs/wazero/imports/wasip2/io"
)

// Method represents HTTP methods.
// Matches wasi:http/types method variant.
type Method uint8

const (
	MethodGet Method = iota
	MethodHead
	MethodPost
	MethodPut
	MethodDelete
	MethodConnect
	MethodOptions
	MethodTrace
	MethodPatch
	MethodOther
)

// String returns the WASI name for this HTTP method.
func (m Method) String() string {
	switch m {
	case MethodGet:
		return "get"
	case MethodHead:
		return "head"
	case MethodPost:
		return "post"
	case MethodPut:
		return "put"
	case MethodDelete:
		return "delete"
	case MethodConnect:
		return "connect"
	case MethodOptions:
		return "options"
	case MethodTrace:
		return "trace"
	case MethodPatch:
		return "patch"
	case MethodOther:
		return "other"
	default:
		return "get"
	}
}

// HTTPMethod returns the Go net/http method string (uppercase).
func (m Method) HTTPMethod() string {
	switch m {
	case MethodGet:
		return gohttp.MethodGet
	case MethodHead:
		return gohttp.MethodHead
	case MethodPost:
		return gohttp.MethodPost
	case MethodPut:
		return gohttp.MethodPut
	case MethodDelete:
		return gohttp.MethodDelete
	case MethodConnect:
		return gohttp.MethodConnect
	case MethodOptions:
		return gohttp.MethodOptions
	case MethodTrace:
		return gohttp.MethodTrace
	case MethodPatch:
		return gohttp.MethodPatch
	default:
		return gohttp.MethodGet
	}
}

// schemeKind represents the kind of scheme (http, https, or other).
type schemeKind uint8

const (
	schemeKindHTTP schemeKind = iota
	schemeKindHTTPS
	schemeKindOther
)

// Scheme represents the URI scheme (http, https, or other).
// Matches wasi:http/types scheme variant.
type Scheme struct {
	kind  schemeKind
	other string
}

// NewSchemeHTTP creates an HTTP scheme.
func NewSchemeHTTP() Scheme {
	return Scheme{kind: schemeKindHTTP}
}

// NewSchemeHTTPS creates an HTTPS scheme.
func NewSchemeHTTPS() Scheme {
	return Scheme{kind: schemeKindHTTPS}
}

// NewSchemeOther creates a custom scheme.
func NewSchemeOther(name string) Scheme {
	return Scheme{kind: schemeKindOther, other: name}
}

// IsHTTP returns true if this is an HTTP scheme.
func (s Scheme) IsHTTP() bool {
	return s.kind == schemeKindHTTP
}

// IsHTTPS returns true if this is an HTTPS scheme.
func (s Scheme) IsHTTPS() bool {
	return s.kind == schemeKindHTTPS
}

// IsOther returns true if this is a custom scheme.
func (s Scheme) IsOther() bool {
	return s.kind == schemeKindOther
}

// Other returns the custom scheme name (only valid if IsOther() is true).
func (s Scheme) Other() string {
	return s.other
}

// String returns the string representation of the scheme.
func (s Scheme) String() string {
	switch s.kind {
	case schemeKindHTTP:
		return "http"
	case schemeKindHTTPS:
		return "https"
	case schemeKindOther:
		return s.other
	default:
		return "http"
	}
}

// Fields represents HTTP headers or trailers.
// Matches wasi:http/types fields resource.
type Fields struct {
	entries map[string][][]byte
}

// NewFields creates a new empty Fields.
func NewFields() *Fields {
	return &Fields{
		entries: make(map[string][][]byte),
	}
}

// Get returns the values for a field name.
func (f *Fields) Get(name string) [][]byte {
	if values, ok := f.entries[name]; ok {
		return values
	}
	return [][]byte{}
}

// Has returns true if the field name exists.
func (f *Fields) Has(name string) bool {
	if f.entries == nil {
		return false
	}
	_, ok := f.entries[name]
	return ok
}

// Set sets the values for a field name.
func (f *Fields) Set(name string, values [][]byte) {
	f.entries[name] = values
}

// Delete removes a field by name.
func (f *Fields) Delete(name string) {
	delete(f.entries, name)
}

// Append adds a value to a field name.
func (f *Fields) Append(name string, value []byte) {
	f.entries[name] = append(f.entries[name], value)
}

// Entries returns all field entries.
func (f *Fields) Entries() []struct {
	Name   string
	Values [][]byte
} {
	result := make([]struct {
		Name   string
		Values [][]byte
	}, 0, len(f.entries))
	for name, values := range f.entries {
		result = append(result, struct {
			Name   string
			Values [][]byte
		}{Name: name, Values: values})
	}
	return result
}

// Clone creates a deep copy of the Fields.
func (f *Fields) Clone() *Fields {
	clone := NewFields()
	for name, values := range f.entries {
		clonedValues := make([][]byte, len(values))
		for i, v := range values {
			clonedValues[i] = make([]byte, len(v))
			copy(clonedValues[i], v)
		}
		clone.entries[name] = clonedValues
	}
	return clone
}

// Destroy clears all entries in the Fields.
// This handles resource cleanup.
// Safe to call multiple times (idempotent).
func (f *Fields) Destroy() {
	if f.entries != nil {
		// Clear the map
		for k := range f.entries {
			delete(f.entries, k)
		}
	}
}

// OutgoingRequest represents an outgoing HTTP request.
// Matches wasi:http/types outgoing-request resource.
type OutgoingRequest struct {
	method        Method
	otherMethod   string // For MethodOther
	scheme        *Scheme
	authority     *string
	pathWithQuery *string
	headers       *Fields
	body          *OutgoingBody
	bodyConsumed  bool // Track if body was already obtained
}

// NewOutgoingRequest creates a new outgoing request with the given headers.
func NewOutgoingRequest(headers *Fields) *OutgoingRequest {
	return &OutgoingRequest{
		method:  MethodGet,
		headers: headers,
	}
}

// Method returns the HTTP method.
func (r *OutgoingRequest) Method() Method {
	return r.method
}

// SetMethod sets the HTTP method.
func (r *OutgoingRequest) SetMethod(method Method) {
	r.method = method
}

// Scheme returns the URI scheme.
func (r *OutgoingRequest) Scheme() *Scheme {
	return r.scheme
}

// SetScheme sets the URI scheme.
func (r *OutgoingRequest) SetScheme(scheme *Scheme) {
	r.scheme = scheme
}

// Authority returns the authority (host:port).
func (r *OutgoingRequest) Authority() *string {
	return r.authority
}

// SetAuthority sets the authority.
func (r *OutgoingRequest) SetAuthority(authority *string) {
	r.authority = authority
}

// PathWithQuery returns the path with query string.
func (r *OutgoingRequest) PathWithQuery() *string {
	return r.pathWithQuery
}

// SetPathWithQuery sets the path with query string.
func (r *OutgoingRequest) SetPathWithQuery(path *string) {
	r.pathWithQuery = path
}

// Headers returns the request headers.
func (r *OutgoingRequest) Headers() *Fields {
	return r.headers
}

// Body returns the outgoing body for writing request content.
// Can only be called once per request.
func (r *OutgoingRequest) Body() (*OutgoingBody, error) {
	if r.bodyConsumed {
		return nil, fmt.Errorf("body already consumed")
	}
	if r.body == nil {
		r.body = NewOutgoingBody()
	}
	r.bodyConsumed = true
	return r.body, nil
}

// Destroy releases all resources held by the request.
// This handles resource cleanup.
// Safe to call multiple times (idempotent).
func (r *OutgoingRequest) Destroy() {
	if r.headers != nil {
		r.headers.Destroy()
	}
	if r.body != nil {
		r.body.Destroy()
	}
}

// ToHTTPRequest converts the OutgoingRequest to a Go net/http Request.
func (r *OutgoingRequest) ToHTTPRequest(ctx context.Context) (*gohttp.Request, error) {
	// Build the URL
	var scheme string
	if r.scheme != nil {
		scheme = r.scheme.String()
	} else {
		scheme = "http" // Default to http
	}

	var authority string
	if r.authority != nil {
		authority = *r.authority
	} else {
		return nil, fmt.Errorf("authority is required")
	}

	var pathWithQuery string
	if r.pathWithQuery != nil {
		pathWithQuery = *r.pathWithQuery
	} else {
		pathWithQuery = "/"
	}

	// Construct URL
	urlStr := fmt.Sprintf("%s://%s%s", scheme, authority, pathWithQuery)

	// Get the HTTP method
	method := r.method.HTTPMethod()
	if r.method == MethodOther && r.otherMethod != "" {
		method = strings.ToUpper(r.otherMethod)
	}

	// Get body reader
	var bodyReader goio.Reader
	if r.body != nil && r.body.buffer != nil {
		bodyReader = bytes.NewReader(r.body.buffer.Bytes())
	}

	// Create the request
	req, err := gohttp.NewRequestWithContext(ctx, method, urlStr, bodyReader)
	if err != nil {
		return nil, err
	}

	// Copy headers
	if r.headers != nil {
		for name, values := range r.headers.entries {
			for _, value := range values {
				req.Header.Add(name, string(value))
			}
		}
	}

	return req, nil
}

// IncomingRequest represents an incoming HTTP request.
// Matches wasi:http/types incoming-request resource.
type IncomingRequest struct {
	method        Method
	scheme        *Scheme
	authority     *string
	pathWithQuery *string
	headers       *Fields
	body          *IncomingBody
	bodyConsumed  atomic.Bool
}

// NewIncomingRequest creates a new incoming request.
func NewIncomingRequest(method Method, scheme *Scheme, authority *string, pathWithQuery *string, headers *Fields) *IncomingRequest {
	return &IncomingRequest{
		method:        method,
		scheme:        scheme,
		authority:     authority,
		pathWithQuery: pathWithQuery,
		headers:       headers,
	}
}

// Method returns the HTTP method.
func (r *IncomingRequest) Method() Method {
	return r.method
}

// Scheme returns the URI scheme.
func (r *IncomingRequest) Scheme() *Scheme {
	return r.scheme
}

// Authority returns the authority (host:port).
func (r *IncomingRequest) Authority() *string {
	return r.authority
}

// PathWithQuery returns the path with query string.
func (r *IncomingRequest) PathWithQuery() *string {
	return r.pathWithQuery
}

// Headers returns the request headers.
func (r *IncomingRequest) Headers() *Fields {
	return r.headers
}

// SetBody sets the body for the incoming request.
func (r *IncomingRequest) SetBody(body *IncomingBody) {
	r.body = body
}

// Consume returns the request body, ensuring it can only be consumed once.
// Thread-safe via atomic compare-and-swap.
func (r *IncomingRequest) Consume() (*IncomingBody, error) {
	if !r.bodyConsumed.CompareAndSwap(false, true) {
		return nil, fmt.Errorf("body already consumed")
	}
	if r.body != nil {
		return r.body, nil
	}
	return NewIncomingBody(), nil
}

// OutgoingResponse represents an outgoing HTTP response.
// Matches wasi:http/types outgoing-response resource.
type OutgoingResponse struct {
	statusCode   uint16
	headers      *Fields
	body         *OutgoingBody
	bodyConsumed bool
}

// NewOutgoingResponse creates a new outgoing response with the given headers.
func NewOutgoingResponse(headers *Fields) *OutgoingResponse {
	return &OutgoingResponse{
		statusCode: 200,
		headers:    headers,
	}
}

// StatusCode returns the status code.
func (r *OutgoingResponse) StatusCode() uint16 {
	return r.statusCode
}

// SetStatusCode sets the status code.
func (r *OutgoingResponse) SetStatusCode(code uint16) {
	r.statusCode = code
}

// Headers returns the response headers.
func (r *OutgoingResponse) Headers() *Fields {
	return r.headers
}

// Body returns the outgoing body for writing. Can only be called once.
func (r *OutgoingResponse) Body() (*OutgoingBody, error) {
	if r.bodyConsumed {
		return nil, fmt.Errorf("body already consumed")
	}
	r.bodyConsumed = true
	r.body = NewOutgoingBody()
	return r.body, nil
}

// BodyBytes returns the bytes that have been written to the body buffer,
// or nil if the body was never accessed. Read-only accessor that does not
// consume the body.
func (r *OutgoingResponse) BodyBytes() []byte {
	if r.body == nil {
		return nil
	}
	return r.body.Bytes()
}

// IncomingResponse represents an incoming HTTP response.
// Matches wasi:http/types incoming-response resource.
type IncomingResponse struct {
	statusCode   uint16
	headers      *Fields
	httpResponse *gohttp.Response // Original Go HTTP response
	body         *IncomingBody
	bodyConsumed bool
}

// NewIncomingResponse creates a new incoming response.
func NewIncomingResponse(statusCode uint16, headers *Fields) *IncomingResponse {
	return &IncomingResponse{
		statusCode: statusCode,
		headers:    headers,
	}
}

// NewIncomingResponseFromHTTP creates an IncomingResponse from a Go http.Response.
func NewIncomingResponseFromHTTP(resp *gohttp.Response) *IncomingResponse {
	// Convert headers
	headers := NewFields()
	for name, values := range resp.Header {
		for _, v := range values {
			headers.Append(name, []byte(v))
		}
	}

	return &IncomingResponse{
		statusCode:   uint16(resp.StatusCode),
		headers:      headers,
		httpResponse: resp,
	}
}

// Status returns the status code.
func (r *IncomingResponse) Status() uint16 {
	return r.statusCode
}

// Headers returns the response headers.
func (r *IncomingResponse) Headers() *Fields {
	return r.headers
}

// Consume returns the body of the response.
// Can only be called once per response.
func (r *IncomingResponse) Consume() (*IncomingBody, error) {
	if r.bodyConsumed {
		return nil, fmt.Errorf("body already consumed")
	}
	if r.body == nil {
		if r.httpResponse != nil && r.httpResponse.Body != nil {
			r.body = NewIncomingBodyFromReader(r.httpResponse.Body)
		} else {
			r.body = NewIncomingBody()
		}
	}
	r.bodyConsumed = true
	return r.body, nil
}

// Destroy releases all resources held by the response.
// This handles resource cleanup.
// Safe to call multiple times (idempotent).
func (r *IncomingResponse) Destroy() {
	if r.headers != nil {
		r.headers.Destroy()
	}
	if r.body != nil {
		r.body.Destroy()
	}
}

// IncomingBody represents the body of an incoming HTTP message.
// Matches wasi:http/types incoming-body resource.
type IncomingBody struct {
	reader         goio.ReadCloser
	stream         *io.InputStream
	streamConsumed bool
}

// NewIncomingBody creates a new incoming body.
func NewIncomingBody() *IncomingBody {
	return &IncomingBody{}
}

// NewIncomingBodyFromReader creates an incoming body from a reader.
func NewIncomingBodyFromReader(r goio.ReadCloser) *IncomingBody {
	return &IncomingBody{reader: r}
}

// Stream returns an InputStream for reading the body.
// Can only be called once per body.
func (b *IncomingBody) Stream() (*io.InputStream, error) {
	if b.streamConsumed {
		return nil, fmt.Errorf("stream already consumed")
	}
	if b.stream == nil {
		if b.reader != nil {
			b.stream = io.NewInputStream(b.reader)
		} else {
			// Empty body
			b.stream = io.NewInputStream(goio.NopCloser(bytes.NewReader(nil)))
		}
	}
	b.streamConsumed = true
	return b.stream, nil
}

// Close closes the body and releases resources.
func (b *IncomingBody) Close() {
	if b.reader != nil {
		b.reader.Close()
	}
}

// Destroy closes the reader and releases resources.
// This handles resource cleanup.
// Safe to call multiple times (idempotent).
func (b *IncomingBody) Destroy() {
	b.Close()
}

// OutgoingBody represents the body of an outgoing HTTP message.
// Matches wasi:http/types outgoing-body resource.
type OutgoingBody struct {
	buffer         *bytes.Buffer
	stream         *io.OutputStream
	streamConsumed bool
	finished       bool
}

// NewOutgoingBody creates a new outgoing body.
func NewOutgoingBody() *OutgoingBody {
	return &OutgoingBody{
		buffer: &bytes.Buffer{},
	}
}

// Write returns an OutputStream for writing the body.
// Can only be called once per body.
func (b *OutgoingBody) Write() (*io.OutputStream, error) {
	if b.streamConsumed {
		return nil, fmt.Errorf("stream already consumed")
	}
	if b.finished {
		return nil, fmt.Errorf("body already finished")
	}
	if b.stream == nil {
		b.stream = io.NewOutputStream(b.buffer)
	}
	b.streamConsumed = true
	return b.stream, nil
}

// Finish marks the body as complete.
func (b *OutgoingBody) Finish() error {
	if b.finished {
		return fmt.Errorf("body already finished")
	}
	b.finished = true
	return nil
}

// Bytes returns the written bytes.
func (b *OutgoingBody) Bytes() []byte {
	if b.buffer != nil {
		return b.buffer.Bytes()
	}
	return nil
}

// Destroy clears the buffer and releases resources.
// This handles resource cleanup.
// Safe to call multiple times (idempotent).
func (b *OutgoingBody) Destroy() {
	b.buffer = nil
	b.stream = nil
	b.finished = true
}

// FutureIncomingResponse represents an async incoming response.
// Matches wasi:http/types future-incoming-response resource.
type FutureIncomingResponse struct {
	done      chan struct{}
	response  *IncomingResponse
	errorCode *ErrorCode
	retrieved bool // Track if Get() returned the result
}

// NewFutureIncomingResponse creates a new future incoming response.
func NewFutureIncomingResponse() *FutureIncomingResponse {
	return &FutureIncomingResponse{
		done: make(chan struct{}),
	}
}

// SetResponse sets the successful response and signals completion.
func (f *FutureIncomingResponse) SetResponse(resp *IncomingResponse) {
	f.response = resp
	close(f.done)
}

// SetError sets the error and signals completion.
func (f *FutureIncomingResponse) SetError(code ErrorCode) {
	f.errorCode = &code
	close(f.done)
}

// Subscribe returns a Pollable that is ready when the response is available.
func (f *FutureIncomingResponse) Subscribe() *io.Pollable {
	return io.NewPollable(
		func() bool {
			select {
			case <-f.done:
				return true
			default:
				return false
			}
		},
		func() {
			<-f.done
		},
	)
}

// Get returns the response if ready, otherwise returns nil, nil, false.
// Once the response is returned, subsequent calls return nil, nil, false.
// Returns: (response, errorCode, ready)
func (f *FutureIncomingResponse) Get() (*IncomingResponse, *ErrorCode, bool) {
	// If already retrieved, return nothing per WASI spec
	if f.retrieved {
		return nil, nil, false
	}

	select {
	case <-f.done:
		f.retrieved = true
		return f.response, f.errorCode, true
	default:
		return nil, nil, false
	}
}

// IsReady returns true if the response is available.
func (f *FutureIncomingResponse) IsReady() bool {
	select {
	case <-f.done:
		return true
	default:
		return false
	}
}

// Destroy marks the future as consumed and destroys any held response.
// This handles resource cleanup.
// Safe to call multiple times (idempotent).
func (f *FutureIncomingResponse) Destroy() {
	f.retrieved = true
	if f.response != nil {
		f.response.Destroy()
	}
}

// futureTrailersState represents the state of a FutureTrailers resource.
type futureTrailersState int

const (
	futureTrailersWaiting futureTrailersState = iota
	futureTrailersDone
	futureTrailersConsumed
)

// FutureTrailers represents async trailers.
// Matches wasi:http/types future-trailers resource.
type FutureTrailers struct {
	mu       sync.Mutex
	state    futureTrailersState
	trailers *Fields
	err      *ErrorCode
	done     chan struct{}
}

// NewFutureTrailers creates a new future trailers in the waiting state.
func NewFutureTrailers() *FutureTrailers {
	return &FutureTrailers{
		state: futureTrailersWaiting,
		done:  make(chan struct{}),
	}
}

// NewFutureTrailersReady creates a new future trailers that is already resolved.
func NewFutureTrailersReady(trailers *Fields, err *ErrorCode) *FutureTrailers {
	done := make(chan struct{})
	close(done)
	return &FutureTrailers{
		state:    futureTrailersDone,
		trailers: trailers,
		err:      err,
		done:     done,
	}
}

// IsReady returns true if the future trailers have been resolved.
// Thread-safe: protected by mutex; advances state from waiting to done if the
// underlying done channel has been closed.
func (ft *FutureTrailers) IsReady() bool {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	if ft.state != futureTrailersWaiting {
		return true
	}
	select {
	case <-ft.done:
		ft.state = futureTrailersDone
		return true
	default:
		return false
	}
}

// State returns the current state. Thread-safe.
func (ft *FutureTrailers) State() futureTrailersState {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	return ft.state
}

// SetConsumed marks the future trailers as consumed.
// Returns true if the state successfully transitioned from done to consumed,
// false if it was already consumed or still waiting.
func (ft *FutureTrailers) SetConsumed() bool {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	// Advance from waiting to done if the channel has fired
	if ft.state == futureTrailersWaiting {
		select {
		case <-ft.done:
			ft.state = futureTrailersDone
		default:
			return false
		}
	}
	if ft.state == futureTrailersDone {
		ft.state = futureTrailersConsumed
		return true
	}
	return false
}

// ResponseResult holds the result delivered through a ResponseOutparam.
type ResponseResult struct {
	Response *OutgoingResponse
	Err      *ErrorCode
}

// ResponseOutparam represents a response outparam for server responses.
// Matches wasi:http/types response-outparam resource.
type ResponseOutparam struct {
	mu     sync.Mutex
	result chan ResponseResult
	closed bool
}

// NewResponseOutparam creates a new response outparam.
func NewResponseOutparam() *ResponseOutparam {
	return &ResponseOutparam{
		result: make(chan ResponseResult, 1),
	}
}

// WaitForResponse blocks until a response is delivered or the context is cancelled.
// This is a one-shot call: the channel buffers a single result. After the first
// successful return any subsequent call will block until context cancellation
// or the outparam is destroyed.
func (p *ResponseOutparam) WaitForResponse(ctx context.Context) (*OutgoingResponse, *ErrorCode, error) {
	select {
	case r, ok := <-p.result:
		if !ok {
			return nil, nil, fmt.Errorf("response-outparam destroyed before response was set")
		}
		return r.Response, r.Err, nil
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
}

// Destroy handles resource cleanup.
// It closes the result channel, unblocking any pending WaitForResponse caller.
func (p *ResponseOutparam) Destroy() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	p.closed = true
	close(p.result)
}

// RequestOptions represents options for HTTP requests.
// Matches wasi:http/types request-options resource.
type RequestOptions struct {
	connectTimeout      *uint64 // Duration in nanoseconds
	firstByteTimeout    *uint64 // Duration in nanoseconds
	betweenBytesTimeout *uint64 // Duration in nanoseconds
}

// NewRequestOptions creates new request options with default values.
func NewRequestOptions() *RequestOptions {
	return &RequestOptions{}
}

// ConnectTimeout returns the connect timeout.
func (o *RequestOptions) ConnectTimeout() *uint64 {
	return o.connectTimeout
}

// SetConnectTimeout sets the connect timeout.
func (o *RequestOptions) SetConnectTimeout(timeout *uint64) {
	o.connectTimeout = timeout
}

// FirstByteTimeout returns the first byte timeout.
func (o *RequestOptions) FirstByteTimeout() *uint64 {
	return o.firstByteTimeout
}

// SetFirstByteTimeout sets the first byte timeout.
func (o *RequestOptions) SetFirstByteTimeout(timeout *uint64) {
	o.firstByteTimeout = timeout
}

// BetweenBytesTimeout returns the between bytes timeout.
func (o *RequestOptions) BetweenBytesTimeout() *uint64 {
	return o.betweenBytesTimeout
}

// SetBetweenBytesTimeout sets the between bytes timeout.
func (o *RequestOptions) SetBetweenBytesTimeout(timeout *uint64) {
	o.betweenBytesTimeout = timeout
}

// HTTPError wraps an ErrorCode so it can be stored in io.Error and extracted
// by http-error-code.
type HTTPError struct {
	Code ErrorCode
}

func (e *HTTPError) Error() string {
	return string(e.Code)
}

// ErrorCode represents HTTP error codes.
// Matches wasi:http/types error-code variant.
type ErrorCode string

const (
	// DNS errors
	ErrorCodeDNSTimeout ErrorCode = "DNS-timeout"
	ErrorCodeDNSError   ErrorCode = "DNS-error"

	// Destination errors
	ErrorCodeDestinationNotFound     ErrorCode = "destination-not-found"
	ErrorCodeDestinationUnavailable  ErrorCode = "destination-unavailable"
	ErrorCodeDestinationIPProhibited ErrorCode = "destination-IP-prohibited"
	ErrorCodeDestinationIPUnroutable ErrorCode = "destination-IP-unroutable"

	// Connection errors
	ErrorCodeConnectionRefused      ErrorCode = "connection-refused"
	ErrorCodeConnectionTerminated   ErrorCode = "connection-terminated"
	ErrorCodeConnectionTimeout      ErrorCode = "connection-timeout"
	ErrorCodeConnectionReadTimeout  ErrorCode = "connection-read-timeout"
	ErrorCodeConnectionWriteTimeout ErrorCode = "connection-write-timeout"
	ErrorCodeConnectionLimitReached ErrorCode = "connection-limit-reached"

	// TLS errors
	ErrorCodeTLSProtocolError    ErrorCode = "TLS-protocol-error"
	ErrorCodeTLSCertificateError ErrorCode = "TLS-certificate-error"
	ErrorCodeTLSAlertReceived    ErrorCode = "TLS-alert-received"

	// HTTP request errors
	ErrorCodeHTTPRequestDenied             ErrorCode = "HTTP-request-denied"
	ErrorCodeHTTPRequestLengthRequired     ErrorCode = "HTTP-request-length-required"
	ErrorCodeHTTPRequestBodySize           ErrorCode = "HTTP-request-body-size"
	ErrorCodeHTTPRequestMethodInvalid      ErrorCode = "HTTP-request-method-invalid"
	ErrorCodeHTTPRequestURIInvalid         ErrorCode = "HTTP-request-URI-invalid"
	ErrorCodeHTTPRequestURITooLong         ErrorCode = "HTTP-request-URI-too-long"
	ErrorCodeHTTPRequestHeaderSectionSize  ErrorCode = "HTTP-request-header-section-size"
	ErrorCodeHTTPRequestHeaderSize         ErrorCode = "HTTP-request-header-size"
	ErrorCodeHTTPRequestTrailerSectionSize ErrorCode = "HTTP-request-trailer-section-size"
	ErrorCodeHTTPRequestTrailerSize        ErrorCode = "HTTP-request-trailer-size"

	// HTTP response errors
	ErrorCodeHTTPResponseIncomplete         ErrorCode = "HTTP-response-incomplete"
	ErrorCodeHTTPResponseHeaderSectionSize  ErrorCode = "HTTP-response-header-section-size"
	ErrorCodeHTTPResponseHeaderSize         ErrorCode = "HTTP-response-header-size"
	ErrorCodeHTTPResponseBodySize           ErrorCode = "HTTP-response-body-size"
	ErrorCodeHTTPResponseTrailerSectionSize ErrorCode = "HTTP-response-trailer-section-size"
	ErrorCodeHTTPResponseTrailerSize        ErrorCode = "HTTP-response-trailer-size"
	ErrorCodeHTTPResponseTransferCoding     ErrorCode = "HTTP-response-transfer-coding"
	ErrorCodeHTTPResponseContentCoding      ErrorCode = "HTTP-response-content-coding"
	ErrorCodeHTTPResponseTimeout            ErrorCode = "HTTP-response-timeout"
	ErrorCodeHTTPUpgradeFailed              ErrorCode = "HTTP-upgrade-failed"
	ErrorCodeHTTPProtocolError              ErrorCode = "HTTP-protocol-error"

	// General errors
	ErrorCodeLoopDetected       ErrorCode = "loop-detected"
	ErrorCodeConfigurationError ErrorCode = "configuration-error"
	ErrorCodeInternalError      ErrorCode = "internal-error"
)

// HeaderError represents errors that can occur when manipulating headers.
// Matches wasi:http/types header-error variant.
type HeaderError string

const (
	HeaderErrorInvalidSyntax HeaderError = "invalid-syntax"
	HeaderErrorForbidden     HeaderError = "forbidden"
	HeaderErrorImmutable     HeaderError = "immutable"
)

// ErrorCodeFromError maps a Go error to a WASI HTTP error code.
func ErrorCodeFromError(err error) ErrorCode {
	if err == nil {
		return ErrorCodeInternalError
	}

	errStr := err.Error()

	// Check for timeout errors
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
		if strings.Contains(errStr, "dial") || strings.Contains(errStr, "connect") {
			return ErrorCodeConnectionTimeout
		}
		return ErrorCodeHTTPResponseTimeout
	}

	// Check for DNS errors
	if strings.Contains(errStr, "no such host") || strings.Contains(errStr, "DNS") {
		return ErrorCodeDNSError
	}

	// Check for connection errors
	if strings.Contains(errStr, "connection refused") {
		return ErrorCodeConnectionRefused
	}
	if strings.Contains(errStr, "connection reset") {
		return ErrorCodeConnectionTerminated
	}

	// Check for TLS errors
	if strings.Contains(errStr, "certificate") || strings.Contains(errStr, "x509") {
		return ErrorCodeTLSCertificateError
	}
	if strings.Contains(errStr, "tls") || strings.Contains(errStr, "TLS") {
		return ErrorCodeTLSProtocolError
	}

	// Check for URL errors
	if strings.Contains(errStr, "invalid URL") || strings.Contains(errStr, "malformed") {
		return ErrorCodeHTTPRequestURIInvalid
	}

	// Default to internal error
	return ErrorCodeInternalError
}

// MakeHTTPClient creates an HTTP client configured with the given RequestOptions.
func MakeHTTPClient(opts *RequestOptions) *gohttp.Client {
	client := &gohttp.Client{}

	transport := &gohttp.Transport{}

	if opts != nil {
		// Connect timeout applies to the dial timeout
		if opts.connectTimeout != nil {
			transport.DialContext = (&gohttp.Transport{}).DialContext
			timeout := time.Duration(*opts.connectTimeout)
			transport.ResponseHeaderTimeout = timeout
		}
	}

	client.Transport = transport

	// Apply overall timeout based on first byte timeout
	if opts != nil && opts.firstByteTimeout != nil {
		client.Timeout = time.Duration(*opts.firstByteTimeout)
	}

	return client
}
