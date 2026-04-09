// imports/wasip2/http/incoming.go

// WIT source of truth: debug-vendored/WASI/proposals/http/wit/{types,handler,proxy}.wit
// Package version: wasi:http@0.2.9 (wazero targets wasi:http@0.2.0)
//
package http

import (
	"context"
	"fmt"
	gohttp "net/http"
	"strings"
	"time"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// defaultHandlerTimeout is the maximum time NewHTTPHandler will wait for a
// component-side response. Without this, a misbehaving component that never
// writes to the outparam channel would hang the host handler indefinitely.
const defaultHandlerTimeout = 30 * time.Second

// instantiateIncomingHandler registers wasi:http/incoming-handler@0.2.0
func instantiateIncomingHandler(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:http/incoming-handler@0.2.0")

	// handle: func(request: own<incoming-request>, response-out: own<response-outparam>)
	inst.Func("handle", incomingHandlerHandle)

	return inst.SkipValidation().Build()
}

// NewHTTPHandler creates a Go http.Handler that bridges to a WASI component's
// incoming-handler.handle export. The callHandle function should invoke the
// component's handle function with the given request and outparam handles.
func NewHTTPHandler(callHandle func(ctx context.Context, requestHandle, outparamHandle runtime.Handle) error) gohttp.Handler {
	return gohttp.HandlerFunc(func(w gohttp.ResponseWriter, r *gohttp.Request) {
		table := runtime.NewTable()

		// Apply a default deadline so a misbehaving component cannot hang the
		// host handler indefinitely. Callers wanting a different deadline can
		// wrap the request with a custom context before passing it in.
		ctx, cancel := context.WithTimeout(r.Context(), defaultHandlerTimeout)
		defer cancel()
		ctx = component.WithResourceTable(ctx, table)

		// Build IncomingRequest from Go request
		method := methodFromHTTPMethod(r.Method)
		scheme := schemeForRequest(r)
		authority := r.Host
		pathWithQuery := r.URL.RequestURI()
		headers := NewFields()
		for name, values := range r.Header {
			// WASI HTTP uses lowercase header names; Go canonicalizes to
			// Title-Case form. Normalize so component lookups match.
			lowerName := strings.ToLower(name)
			for _, v := range values {
				headers.Append(lowerName, []byte(v))
			}
		}

		req := NewIncomingRequest(method, scheme, &authority, &pathWithQuery, headers)
		// r.Body is always non-nil for server requests; it returns EOF when
		// there is no body.
		req.SetBody(NewIncomingBodyFromReader(r.Body))
		reqID := registerIncomingRequest(req)
		requestHandle, err := table.NewResourceHandle(reqID, true, httpIncomingRequestResourceType)
		if err != nil {
			unregisterIncomingRequest(reqID)
			panic(fmt.Errorf("create resource handle: %w", err))
		}

		// Create response outparam with channel
		outparam := NewResponseOutparam()
		outparamID := registerResponseOutparam(outparam)
		outparamHandle, err := table.NewResourceHandle(outparamID, true, httpOutgoingResponseResourceType)
		if err != nil {
			unregisterResponseOutparam(outparamID)
			panic(fmt.Errorf("create resource handle: %w", err))
		}

		// Ensure resources are cleaned up on every exit path. Delete is a
		// no-op if the component already consumed the handle.
		defer func() {
			if entry, err := table.Remove(requestHandle); err == nil && entry != nil {
				unregisterIncomingRequest(entry.Rep)
			}
			if entry, err := table.Remove(outparamHandle); err == nil && entry != nil {
				if p := getResponseOutparamFromRegistry(entry.Rep); p != nil {
					p.Destroy()
				}
				unregisterResponseOutparam(entry.Rep)
			}
		}()

		// Call the component's handle function
		if err := callHandle(ctx, requestHandle, outparamHandle); err != nil {
			gohttp.Error(w, "handler error", gohttp.StatusInternalServerError)
			return
		}

		// Wait for response
		resp, errCode, err := outparam.WaitForResponse(ctx)
		if err != nil {
			gohttp.Error(w, "timeout waiting for response", gohttp.StatusGatewayTimeout)
			return
		}

		if errCode != nil {
			gohttp.Error(w, string(*errCode), gohttp.StatusBadGateway)
			return
		}

		if resp == nil {
			gohttp.Error(w, "no response", gohttp.StatusInternalServerError)
			return
		}

		// Write response headers
		respHeaders := resp.Headers()
		if respHeaders != nil {
			for _, entry := range respHeaders.Entries() {
				for _, v := range entry.Values {
					w.Header().Add(entry.Name, string(v))
				}
			}
		}
		w.WriteHeader(int(resp.StatusCode()))

		// Write body via the public accessor (no private field reach-in).
		if body := resp.BodyBytes(); body != nil {
			w.Write(body)
		}
	})
}

// schemeForRequest derives a WASI Scheme from a Go HTTP request, defaulting
// to http or https based on TLS state when the URL scheme is empty (which it
// always is for server-side requests).
func schemeForRequest(r *gohttp.Request) *Scheme {
	if r.URL.Scheme != "" {
		return schemeFromString(r.URL.Scheme)
	}
	if r.TLS != nil {
		sch := NewSchemeHTTPS()
		return &sch
	}
	sch := NewSchemeHTTP()
	return &sch
}

func schemeFromString(s string) *Scheme {
	switch s {
	case "http":
		sch := NewSchemeHTTP()
		return &sch
	case "https":
		sch := NewSchemeHTTPS()
		return &sch
	case "":
		return nil
	default:
		sch := NewSchemeOther(s)
		return &sch
	}
}

func methodFromHTTPMethod(s string) Method {
	switch s {
	case "GET":
		return MethodGet
	case "HEAD":
		return MethodHead
	case "POST":
		return MethodPost
	case "PUT":
		return MethodPut
	case "DELETE":
		return MethodDelete
	case "CONNECT":
		return MethodConnect
	case "OPTIONS":
		return MethodOptions
	case "TRACE":
		return MethodTrace
	case "PATCH":
		return MethodPatch
	default:
		return MethodOther
	}
}

// incomingHandlerHandle handles an incoming HTTP request.
// Signature: func(request: own<incoming-request>, response-out: own<response-outparam>)
func incomingHandlerHandle(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	// This is a no-op placeholder - returns nothing (unit)
	return []types.Val{}, nil
}
