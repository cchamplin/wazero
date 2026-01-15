// imports/wasip2/http/types.go

// Package http implements the wasi:http interfaces for WASI Preview 2.
// It provides HTTP client and server capabilities.
package http

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

// OutgoingRequest represents an outgoing HTTP request.
// Matches wasi:http/types outgoing-request resource.
type OutgoingRequest struct {
	method        Method
	scheme        *Scheme
	authority     *string
	pathWithQuery *string
	headers       *Fields
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

// IncomingRequest represents an incoming HTTP request.
// Matches wasi:http/types incoming-request resource.
type IncomingRequest struct {
	method        Method
	scheme        *Scheme
	authority     *string
	pathWithQuery *string
	headers       *Fields
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

// OutgoingResponse represents an outgoing HTTP response.
// Matches wasi:http/types outgoing-response resource.
type OutgoingResponse struct {
	statusCode uint16
	headers    *Fields
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

// IncomingResponse represents an incoming HTTP response.
// Matches wasi:http/types incoming-response resource.
type IncomingResponse struct {
	statusCode uint16
	headers    *Fields
}

// NewIncomingResponse creates a new incoming response.
func NewIncomingResponse(statusCode uint16, headers *Fields) *IncomingResponse {
	return &IncomingResponse{
		statusCode: statusCode,
		headers:    headers,
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

// IncomingBody represents the body of an incoming HTTP message.
// Matches wasi:http/types incoming-body resource.
type IncomingBody struct {
	consumed bool
}

// NewIncomingBody creates a new incoming body.
func NewIncomingBody() *IncomingBody {
	return &IncomingBody{}
}

// OutgoingBody represents the body of an outgoing HTTP message.
// Matches wasi:http/types outgoing-body resource.
type OutgoingBody struct {
	written bool
}

// NewOutgoingBody creates a new outgoing body.
func NewOutgoingBody() *OutgoingBody {
	return &OutgoingBody{}
}

// FutureIncomingResponse represents an async incoming response.
// Matches wasi:http/types future-incoming-response resource.
type FutureIncomingResponse struct {
	ready    bool
	response *IncomingResponse
	err      *ErrorCode
}

// NewFutureIncomingResponse creates a new future incoming response.
func NewFutureIncomingResponse() *FutureIncomingResponse {
	return &FutureIncomingResponse{}
}

// FutureTrailers represents async trailers.
// Matches wasi:http/types future-trailers resource.
type FutureTrailers struct {
	ready    bool
	trailers *Fields
	err      *ErrorCode
}

// NewFutureTrailers creates a new future trailers.
func NewFutureTrailers() *FutureTrailers {
	return &FutureTrailers{}
}

// ResponseOutparam represents a response outparam for server responses.
// Matches wasi:http/types response-outparam resource.
type ResponseOutparam struct {
	set bool
}

// NewResponseOutparam creates a new response outparam.
func NewResponseOutparam() *ResponseOutparam {
	return &ResponseOutparam{}
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
	ErrorCodeHTTPResponseIncomplete          ErrorCode = "HTTP-response-incomplete"
	ErrorCodeHTTPResponseHeaderSectionSize   ErrorCode = "HTTP-response-header-section-size"
	ErrorCodeHTTPResponseHeaderSize          ErrorCode = "HTTP-response-header-size"
	ErrorCodeHTTPResponseBodySize            ErrorCode = "HTTP-response-body-size"
	ErrorCodeHTTPResponseTrailerSectionSize  ErrorCode = "HTTP-response-trailer-section-size"
	ErrorCodeHTTPResponseTrailerSize         ErrorCode = "HTTP-response-trailer-size"
	ErrorCodeHTTPResponseTransferCoding      ErrorCode = "HTTP-response-transfer-coding"
	ErrorCodeHTTPResponseContentCoding       ErrorCode = "HTTP-response-content-coding"
	ErrorCodeHTTPResponseTimeout             ErrorCode = "HTTP-response-timeout"
	ErrorCodeHTTPUpgradeFailed               ErrorCode = "HTTP-upgrade-failed"
	ErrorCodeHTTPProtocolError               ErrorCode = "HTTP-protocol-error"

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
