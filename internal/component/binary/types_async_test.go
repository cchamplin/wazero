// internal/component/binary/types_async_test.go

package binary

import (
	"testing"
)

func TestDecodeStreamType(t *testing.T) {
	data := buildComponentWithTypeSection([]byte{
		0x66,       // stream opcode
		0x01, 0x7d, // has element type: u8
		0x00,       // no end type
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.Types[0].Stream == nil {
		t.Fatal("expected stream type def")
	}

	if c.Types[0].Stream.ElementType == nil {
		t.Fatal("expected element type")
	}

	if !c.Types[0].Stream.ElementType.IsPrimitive {
		t.Error("expected primitive element type")
	}

	if c.Types[0].Stream.ElementType.Primitive != 0x7d {
		t.Errorf("expected u8 (0x7d), got 0x%02x", c.Types[0].Stream.ElementType.Primitive)
	}

	if c.Types[0].Stream.EndType != nil {
		t.Error("expected nil end type")
	}
}

func TestDecodeStreamType_WithEndType(t *testing.T) {
	data := buildComponentWithTypeSection([]byte{
		0x66,       // stream opcode
		0x01, 0x7d, // has element type: u8
		0x01, 0x73, // has end type: string
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.Types[0].Stream == nil {
		t.Fatal("expected stream type def")
	}

	if c.Types[0].Stream.ElementType == nil {
		t.Fatal("expected element type")
	}

	if c.Types[0].Stream.EndType == nil {
		t.Fatal("expected end type")
	}

	if c.Types[0].Stream.EndType.Primitive != 0x73 {
		t.Errorf("expected string (0x73), got 0x%02x", c.Types[0].Stream.EndType.Primitive)
	}
}

func TestDecodeStreamType_NoElementNoEnd(t *testing.T) {
	data := buildComponentWithTypeSection([]byte{
		0x66, // stream opcode
		0x00, // no element type
		0x00, // no end type
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.Types[0].Stream == nil {
		t.Fatal("expected stream type def")
	}

	if c.Types[0].Stream.ElementType != nil {
		t.Error("expected nil element type")
	}

	if c.Types[0].Stream.EndType != nil {
		t.Error("expected nil end type")
	}
}

func TestDecodeFutureType(t *testing.T) {
	data := buildComponentWithTypeSection([]byte{
		0x65,       // future opcode
		0x01, 0x73, // has payload: string
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.Types[0].Future == nil {
		t.Fatal("expected future type def")
	}

	if c.Types[0].Future.PayloadType == nil {
		t.Fatal("expected payload type")
	}

	if !c.Types[0].Future.PayloadType.IsPrimitive {
		t.Error("expected primitive payload type")
	}

	if c.Types[0].Future.PayloadType.Primitive != 0x73 {
		t.Errorf("expected string (0x73), got 0x%02x", c.Types[0].Future.PayloadType.Primitive)
	}
}

func TestDecodeFutureType_NoPayload(t *testing.T) {
	data := buildComponentWithTypeSection([]byte{
		0x65, // future opcode
		0x00, // no payload
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.Types[0].Future == nil {
		t.Fatal("expected future type def")
	}

	if c.Types[0].Future.PayloadType != nil {
		t.Error("expected nil payload type")
	}
}

func TestDecodeFixedSizeListType(t *testing.T) {
	data := buildComponentWithTypeSection([]byte{
		0x67, // fixed-size list opcode
		0x7d, // element type: u8
		0x10, // size: 16
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.Types[0].FixedSizeList == nil {
		t.Fatal("expected fixed-size list type def")
	}

	if c.Types[0].FixedSizeList.Size != 16 {
		t.Errorf("expected size 16, got %d", c.Types[0].FixedSizeList.Size)
	}

	if !c.Types[0].FixedSizeList.ElementType.IsPrimitive {
		t.Error("expected primitive element type")
	}

	if c.Types[0].FixedSizeList.ElementType.Primitive != 0x7d {
		t.Errorf("expected u8 (0x7d), got 0x%02x", c.Types[0].FixedSizeList.ElementType.Primitive)
	}
}

func TestDecodeFixedSizeListType_LargeSize(t *testing.T) {
	data := buildComponentWithTypeSection([]byte{
		0x67,             // fixed-size list opcode
		0x7a,             // element type: s32
		0x80, 0x80, 0x04, // size: 65536 (LEB128)
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.Types[0].FixedSizeList == nil {
		t.Fatal("expected fixed-size list type def")
	}

	if c.Types[0].FixedSizeList.Size != 65536 {
		t.Errorf("expected size 65536, got %d", c.Types[0].FixedSizeList.Size)
	}
}
