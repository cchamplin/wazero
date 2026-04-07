// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestPrimitiveValType(t *testing.T) {
	tests := []struct {
		name         string
		typ          ValType
		size         uint32
		align        uint32
		flattenCount int
	}{
		{"Bool", Bool{}, 1, 1, 1},
		{"S8", S8{}, 1, 1, 1},
		{"U8", U8{}, 1, 1, 1},
		{"S16", S16{}, 2, 2, 1},
		{"U16", U16{}, 2, 2, 1},
		{"S32", S32{}, 4, 4, 1},
		{"U32", U32{}, 4, 4, 1},
		{"S64", S64{}, 8, 8, 1},
		{"U64", U64{}, 8, 8, 1},
		{"F32", F32{}, 4, 4, 1},
		{"F64", F64{}, 8, 8, 1},
		{"Char", Char{}, 4, 4, 1},
		{"String", String{}, 8, 4, 2}, // ptr + len, align 4
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.size, tc.typ.Size())
			require.Equal(t, tc.align, tc.typ.Align())
			require.Equal(t, tc.flattenCount, tc.typ.FlattenCount())
		})
	}
}

// TestAsyncValTypes asserts the existence and i32-handle shape of the async
// value types Stream, Future, and ErrorContext. All three flatten to a single
// i32 handle per the canonical ABI spec:
//   - alignment(): definitions.py:1074, 1080 — ErrorContextType=4, StreamType|FutureType=4
//   - elem_size(): definitions.py:1132, 1138 — ErrorContextType=4, StreamType|FutureType=4
//   - flatten_type(): definitions.py:1713, 1719 — all three flatten to ['i32']
//
// Lift/lower of these types is intentionally unimplemented and traps; that
// behavior is asserted in internal/component/abi/lift_test.go and lower_test.go.
func TestAsyncValTypes(t *testing.T) {
	tests := []struct {
		name         string
		typ          ValType
		size         uint32
		align        uint32
		flattenCount int
	}{
		{"Stream_no_element", Stream{}, 4, 4, 1},
		{"Stream_with_element", Stream{Element: U32{}}, 4, 4, 1},
		{"Future_no_element", Future{}, 4, 4, 1},
		{"Future_with_element", Future{Element: U32{}}, 4, 4, 1},
		{"ErrorContext", ErrorContext{}, 4, 4, 1},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.size, tc.typ.Size())
			require.Equal(t, tc.align, tc.typ.Align())
			require.Equal(t, tc.flattenCount, tc.typ.FlattenCount())
		})
	}
}

// TestAsyncValTypesImplementValType compile-time-checks that Stream, Future,
// and ErrorContext satisfy the ValType interface.
func TestAsyncValTypesImplementValType(t *testing.T) {
	var _ ValType = Stream{}
	var _ ValType = Stream{Element: U32{}}
	var _ ValType = Future{}
	var _ ValType = Future{Element: U32{}}
	var _ ValType = ErrorContext{}
}
