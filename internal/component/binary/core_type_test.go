// internal/component/binary/core_type_test.go
package binary

import (
	"testing"
)

func TestDecodeCoreTypeSection(t *testing.T) {
	data := buildComponentWithSection(SectionIDCoreType, []byte{
		0x01,       // count = 1
		0x60,       // func type
		0x02, 0x7f, 0x7f, // 2 params: i32, i32
		0x01, 0x7f, // 1 result: i32
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(c.CoreTypes) != 1 {
		t.Fatalf("expected 1 core type, got %d", len(c.CoreTypes))
	}

	if c.CoreTypes[0].Func == nil {
		t.Fatal("expected core func type")
	}

	if len(c.CoreTypes[0].Func.Params) != 2 {
		t.Errorf("expected 2 params, got %d", len(c.CoreTypes[0].Func.Params))
	}
}

func buildComponentWithSection(sectionID SectionID, content []byte) []byte {
	result := make([]byte, 0, 20+len(content))
	result = append(result, Magic[:]...)
	result = append(result, Version[:]...)
	result = append(result, LayerComponent[:]...)
	result = append(result, byte(sectionID))
	result = append(result, byte(len(content)))
	result = append(result, content...)
	return result
}
