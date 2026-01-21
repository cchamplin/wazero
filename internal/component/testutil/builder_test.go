package testutil

import (
	"testing"
)

func TestBuildComponentFromWAT(t *testing.T) {
	wat := `
    (component
        (core module $m
            (func (export "test") (result i32)
                i32.const 42
            )
        )
        (core instance $i (instantiate $m))
        (alias core export $i "test" (core func $f))
        (type $ft (func (result s32)))
        (func (export "test") (type $ft)
            (canon lift (core func $f)))
    )`

	wasmBytes, err := BuildComponentFromWAT(wat)
	if err != nil {
		t.Fatalf("BuildComponentFromWAT: %v", err)
	}

	// Verify magic number
	if len(wasmBytes) < 8 {
		t.Fatal("Component too short")
	}
	// Component magic: \0asm\r\0\1\0
	if wasmBytes[0] != 0x00 || wasmBytes[1] != 0x61 || wasmBytes[2] != 0x73 || wasmBytes[3] != 0x6d {
		t.Error("Invalid magic number")
	}
}
