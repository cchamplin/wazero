//go:build ignore

// This program generates the add_s32.wasm test fixture.
// Run with: go run gen_add_s32.go
package main

import (
	"os"
)

func main() {
	// Component preamble
	out := []byte{
		// Magic number: \0asm
		0x00, 0x61, 0x73, 0x6d,
		// Version: 0x0d00
		0x0d, 0x00,
		// Layer: component (0x0100)
		0x01, 0x00,
	}

	// === Section 1: Core Module ===
	// A minimal core wasm module that exports an add function
	coreModule := []byte{
		// Magic + version
		0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
		// Type section (1): 1 type - (i32, i32) -> i32
		0x01, 0x07, 0x01, 0x60, 0x02, 0x7f, 0x7f, 0x01, 0x7f,
		// Function section (3): 1 function using type 0
		0x03, 0x02, 0x01, 0x00,
		// Export section (7): export "add" as function 0
		0x07, 0x07, 0x01, 0x03, 0x61, 0x64, 0x64, 0x00, 0x00,
		// Code section (10): 1 function body - local.get 0, local.get 1, i32.add
		0x0a, 0x09, 0x01, 0x07, 0x00, 0x20, 0x00, 0x20, 0x01, 0x6a, 0x0b,
	}

	// Section ID 1 (core module) + size + content
	out = append(out, 0x01)
	out = appendLEB128(out, uint32(len(coreModule)))
	out = append(out, coreModule...)

	// === Section 7: Type Section ===
	// Component function type: (s32, s32) -> s32
	typeSection := []byte{
		0x01,      // 1 type
		0x40,      // functype sync
		0x02,      // 2 params
		0x01, 'a', // param name "a" (length 1)
		0x7a,      // s32
		0x01, 'b', // param name "b" (length 1)
		0x7a,      // s32
		0x00,      // single result
		0x7a,      // s32
	}
	out = append(out, 0x07)
	out = appendLEB128(out, uint32(len(typeSection)))
	out = append(out, typeSection...)

	// === Section 8: Canon Section ===
	// Lift core function 0 as component function type 0
	canonSection := []byte{
		0x01, // 1 canonical
		0x00, // canon.lift
		0x00, // core sort
		0x00, // core func index = 0
		0x00, // 0 options
		0x00, // type index = 0
	}
	out = append(out, 0x08)
	out = appendLEB128(out, uint32(len(canonSection)))
	out = append(out, canonSection...)

	// === Section 11: Export Section ===
	// Export "add" as function 0
	exportSection := []byte{
		0x01,                   // 1 export
		0x00,                   // simple name
		0x03, 0x61, 0x64, 0x64, // name "add"
		0x01,                   // sort = func
		0x00,                   // index = 0
	}
	out = append(out, 0x0b)
	out = appendLEB128(out, uint32(len(exportSection)))
	out = append(out, exportSection...)

	if err := os.WriteFile("../add_s32.wasm", out, 0644); err != nil {
		panic(err)
	}
	println("Generated add_s32.wasm")
}

func appendLEB128(data []byte, v uint32) []byte {
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		data = append(data, b)
		if v == 0 {
			break
		}
	}
	return data
}
