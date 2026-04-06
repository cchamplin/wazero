// imports/wasip2/http/incoming.go

package http

import (
	"context"
	gohttp "net/http"

	"github.com/tetratelabs/wazero/internal/component"
)

// instantiateIncomingHandler registers wasi:http/incoming-handler@0.2.0
func instantiateIncomingHandler(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:http/incoming-handler@0.2.0")

	// handle: func(request: own<incoming-request>, response-out: own<response-outparam>)
	inst.FuncNoType("handle", incomingHandlerHandle)

	return inst.SkipValidation().Build()
}

// NewHTTPHandler creates a Go http.Handler that bridges to a WASI component's
// incoming-handler.handle export. The callHandle function should invoke the
// component's handle function with the given request and outparam handles.
func NewHTTPHandler(callHandle func(ctx context.Context, requestHandle, outparamHandle component.Handle) error) gohttp.Handler {
	return gohttp.HandlerFunc(func(w gohttp.ResponseWriter, r *gohttp.Request) {
		table := component.NewResourceTable()
		ctx := component.WithResourceTable(r.Context(), table)

		// Build IncomingRequest from Go request
		method := methodFromHTTPMethod(r.Method)
		scheme := schemeFromString(r.URL.Scheme)
		authority := r.Host
		pathWithQuery := r.URL.RequestURI()
		headers := NewFields()
		for name, values := range r.Header {
			for _, v := range values {
				headers.Append(name, []byte(v))
			}
		}

		req := NewIncomingRequest(method, scheme, &authority, &pathWithQuery, headers)
		if r.Body != nil {
			req.SetBody(NewIncomingBodyFromReader(r.Body))
		}
		requestHandle := table.New(req, true)

		// Create response outparam with channel
		outparam := NewResponseOutparam()
		outparamHandle := table.New(outparam, true)

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

		// Write body
		if resp.body != nil {
			w.Write(resp.body.Bytes())
		}
	})
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
func incomingHandlerHandle(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// This is a no-op placeholder - returns nothing (unit)
	return []component.Val{}, nil
}
