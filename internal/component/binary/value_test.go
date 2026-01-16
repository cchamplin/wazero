// internal/component/binary/value_test.go
package binary

import (
	"testing"
)

func TestDecodeValueSection(t *testing.T) {
	data := buildComponentWithSection(SectionIDValue, []byte{
		0x01, // count = 1
		0x7a, // s32 type
		0x2a, // value = 42
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(c.Values) != 1 {
		t.Fatalf("expected 1 value, got %d", len(c.Values))
	}

	if !c.Values[0].Type.IsPrimitive {
		t.Error("expected primitive type")
	}
}
