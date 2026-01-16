// internal/component/binary/start_test.go
package binary

import (
	"testing"
)

func TestDecodeStartSection(t *testing.T) {
	data := buildComponentWithSection(SectionIDStart, []byte{
		0x05, // func index
		0x00, // arg count
		0x00, // result count
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.Start == nil {
		t.Fatal("expected start definition")
	}

	if c.Start.FuncIdx != 5 {
		t.Errorf("expected func index 5, got %d", c.Start.FuncIdx)
	}
}

func TestDecodeStartSectionWithArgs(t *testing.T) {
	data := buildComponentWithSection(SectionIDStart, []byte{
		0x03,       // func index
		0x02,       // 2 args
		0x00, 0x01, // arg value indices
		0x01,       // 1 result
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.Start == nil {
		t.Fatal("expected start definition")
	}

	if len(c.Start.ArgValueIdx) != 2 {
		t.Errorf("expected 2 args, got %d", len(c.Start.ArgValueIdx))
	}

	if c.Start.ResultCount != 1 {
		t.Errorf("expected 1 result, got %d", c.Start.ResultCount)
	}
}
