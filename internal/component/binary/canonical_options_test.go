// internal/component/binary/canonical_options_test.go

package binary

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/types"
)

func TestDecodeCanonicalOptions(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte // includes LEB128 count prefix
		expected component.CanonicalOptions
		wantErr  bool
	}{
		{
			name:  "empty options defaults to utf8",
			input: []byte{0x00}, // count = 0
			expected: component.CanonicalOptions{
				StringEncoding: types.StringEncodingUTF8,
			},
		},
		{
			name:  "async option",
			input: []byte{0x01, 0x06}, // count = 1, async
			expected: component.CanonicalOptions{
				StringEncoding: types.StringEncodingUTF8,
				Async:          true,
			},
		},
		{
			name:  "callback option with index 5",
			input: []byte{0x01, 0x07, 0x05}, // count = 1, callback, index = 5
			expected: component.CanonicalOptions{
				StringEncoding: types.StringEncodingUTF8,
				CallbackIdx:    ptrUint32(5),
			},
		},
		{
			name:    "unknown option 0x08 (not in spec)",
			input:   []byte{0x01, 0x08, 0x03},
			wantErr: true,
		},
		{
			name:  "multiple options async memory realloc",
			input: []byte{0x03, 0x06, 0x03, 0x00, 0x04, 0x01}, // count = 3, async, memory idx=0, realloc idx=1
			expected: component.CanonicalOptions{
				StringEncoding: types.StringEncodingUTF8,
				Async:          true,
				MemoryIdx:      ptrUint32(0),
				ReallocIdx:     ptrUint32(1),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader(tt.input)
			var got component.CanonicalOptions
			err := decodeCanonicalOptions(r, &got)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// Compare fields
			if got.StringEncoding != tt.expected.StringEncoding {
				t.Errorf("StringEncoding: got %v, want %v", got.StringEncoding, tt.expected.StringEncoding)
			}
			if got.Async != tt.expected.Async {
				t.Errorf("Async: got %v, want %v", got.Async, tt.expected.Async)
			}
			if !ptrEqual(got.CallbackIdx, tt.expected.CallbackIdx) {
				t.Errorf("CallbackIdx: got %v, want %v", ptrVal(got.CallbackIdx), ptrVal(tt.expected.CallbackIdx))
			}
			if !ptrEqual(got.MemoryIdx, tt.expected.MemoryIdx) {
				t.Errorf("MemoryIdx: got %v, want %v", ptrVal(got.MemoryIdx), ptrVal(tt.expected.MemoryIdx))
			}
			if !ptrEqual(got.ReallocIdx, tt.expected.ReallocIdx) {
				t.Errorf("ReallocIdx: got %v, want %v", ptrVal(got.ReallocIdx), ptrVal(tt.expected.ReallocIdx))
			}
		})
	}
}

func ptrUint32(v uint32) *uint32 {
	return &v
}

func ptrEqual(a, b *uint32) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func ptrVal(p *uint32) string {
	if p == nil {
		return "nil"
	}
	return fmt.Sprintf("%d", *p)
}
