// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: WASI HTTP conformance tests verify that
// wazero's HTTP host module types (Fields, OutgoingRequest,
// OutgoingResponse, etc.) behave correctly.
package conformance

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero"
	wasiphttp "github.com/tetratelabs/wazero/imports/wasip2/http"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestWASIHTTP exercises the wasi:http host module API surface.
//
// No counterpart (justified): WASI P2 host module conformance invariant.
func TestWASIHTTP(t *testing.T) {

	// ------------------------------------------------------------------
	// Case 1: Instantiate registers all wasi:http interfaces.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. http.Instantiate must register the types, incoming-
	// handler, and outgoing-handler interfaces.
	t.Run("InstantiateRegistersInterfaces", func(t *testing.T) {
		rt := wazero.NewRuntime(context.TODO())
		defer rt.Close(context.TODO())
		linker := component.NewComponentLinker(rt)
		err := wasiphttp.Instantiate(linker)
		require.NoError(t, err)

		def, lookupErr := linker.MatchImport("wasi:http/types@0.2.0")
		require.NoError(t, lookupErr, "wasi:http/types@0.2.0 should be registered")
		_, ok := def.(*component.InstanceDef)
		require.True(t, ok)
	})

	// ------------------------------------------------------------------
	// Case 2: Fields CRUD operations work correctly.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. wasi:http/types fields resource must support get, set,
	// append, has, delete.
	t.Run("FieldsCRUD", func(t *testing.T) {
		f := wasiphttp.NewFields()

		// Initially empty
		require.False(t, f.Has("content-type"), "should not have content-type initially")
		require.Equal(t, 0, len(f.Get("content-type")))

		// Set and get
		f.Set("content-type", [][]byte{[]byte("text/plain")})
		require.True(t, f.Has("content-type"))
		vals := f.Get("content-type")
		require.Equal(t, 1, len(vals))
		require.Equal(t, "text/plain", string(vals[0]))

		// Append
		f.Append("accept", []byte("application/json"))
		f.Append("accept", []byte("text/html"))
		acceptVals := f.Get("accept")
		require.Equal(t, 2, len(acceptVals))

		// Delete
		f.Delete("content-type")
		require.False(t, f.Has("content-type"))
	})

	// ------------------------------------------------------------------
	// Case 3: Fields.Clone produces an independent deep copy.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. Cloned fields must be independent of the original.
	t.Run("FieldsClone", func(t *testing.T) {
		f := wasiphttp.NewFields()
		f.Set("x-test", [][]byte{[]byte("original")})

		clone := f.Clone()
		clone.Set("x-test", [][]byte{[]byte("modified")})

		// Original should be unchanged
		origVals := f.Get("x-test")
		require.Equal(t, 1, len(origVals))
		require.Equal(t, "original", string(origVals[0]))
	})

	// ------------------------------------------------------------------
	// Case 4: OutgoingRequest default method is GET.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. A freshly created outgoing-request must default to
	// the GET method per the WASI HTTP spec.
	t.Run("OutgoingRequestDefaults", func(t *testing.T) {
		req := wasiphttp.NewOutgoingRequest(wasiphttp.NewFields())
		require.Equal(t, wasiphttp.MethodGet, req.Method())
		require.True(t, req.Headers() != nil, "headers should not be nil")
	})

	// ------------------------------------------------------------------
	// Case 5: OutgoingResponse default status code is 200.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. A freshly created outgoing-response must default to
	// status code 200.
	t.Run("OutgoingResponseDefaults", func(t *testing.T) {
		resp := wasiphttp.NewOutgoingResponse(wasiphttp.NewFields())
		require.Equal(t, uint16(200), resp.StatusCode())
		resp.SetStatusCode(404)
		require.Equal(t, uint16(404), resp.StatusCode())
	})

	// ------------------------------------------------------------------
	// Case 6: Method.String returns correct WASI identifiers.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. HTTP method variant string conversion must match the
	// wasi:http/types WIT definitions.
	t.Run("MethodStrings", func(t *testing.T) {
		require.Equal(t, "get", wasiphttp.MethodGet.String())
		require.Equal(t, "post", wasiphttp.MethodPost.String())
		require.Equal(t, "put", wasiphttp.MethodPut.String())
		require.Equal(t, "delete", wasiphttp.MethodDelete.String())
		require.Equal(t, "patch", wasiphttp.MethodPatch.String())
	})
}
