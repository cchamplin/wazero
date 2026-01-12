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
