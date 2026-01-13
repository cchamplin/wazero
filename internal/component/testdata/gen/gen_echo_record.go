//go:build ignore

// This program generates the echo_record.wasm test fixture.
// Run with: go run gen_echo_record.go
//
// This component defines a record type `point { x: s32, y: s32 }`
// and exports a function `echo(p: point) -> point` that doubles the coordinates.
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
	// A minimal core wasm module that exports an echo function
	// The function takes (i32, i32) and returns (i32, i32) - doubled values
	coreModule := []byte{
		// Magic + version
		0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
		// Type section (1): 1 type - (i32, i32) -> (i32, i32)
		0x01, 0x09, 0x01, 0x60,
		0x02, 0x7f, 0x7f, // 2 params: i32, i32
		0x02, 0x7f, 0x7f, // 2 results: i32, i32
		// Function section (3): 1 function using type 0
		0x03, 0x02, 0x01, 0x00,
		// Export section (7): export "echo" as function 0
		0x07, 0x08, 0x01, 0x04, 0x65, 0x63, 0x68, 0x6f, 0x00, 0x00, // "echo"
		// Code section (10): 1 function body
		// Function body: local.get 0, i32.const 2, i32.mul, local.get 1, i32.const 2, i32.mul
		0x0a, 0x0f, 0x01, // code section, size 15, 1 function
		0x0d, 0x00, // function body size 13, 0 locals
		0x20, 0x00, // local.get 0
		0x41, 0x02, // i32.const 2
		0x6c, // i32.mul
		0x20, 0x01, // local.get 1
		0x41, 0x02, // i32.const 2
		0x6c,       // i32.mul
		0x0b,       // end
	}

	// Section ID 1 (core module) + size + content
	out = append(out, 0x01)
	out = appendLEB128(out, uint32(len(coreModule)))
	out = append(out, coreModule...)

	// === Section 7: Type Section ===
	// Type 0: record point { x: s32, y: s32 }
	// Type 1: functype (point) -> point
	typeSection := []byte{
		0x02, // 2 types

		// Type 0: record type
		0x72,             // record opcode
		0x02,             // 2 fields
		0x01, 'x', 0x7a,  // field "x": s32
		0x01, 'y', 0x7a,  // field "y": s32

		// Type 1: functype (point) -> point
		0x40,             // functype sync
		0x01,             // 1 param
		0x01, 'p',        // param name "p" (length 1)
		0x00,             // type index 0 (the record type)
		0x00,             // single result
		0x00,             // type index 0 (the record type)
	}
	out = append(out, 0x07)
	out = appendLEB128(out, uint32(len(typeSection)))
	out = append(out, typeSection...)

	// === Section 8: Canon Section ===
	// Lift core function 0 as component function type 1
	canonSection := []byte{
		0x01, // 1 canonical
		0x00, // canon.lift
		0x00, // core sort
		0x00, // core func index = 0
		0x00, // 0 options
		0x01, // type index = 1 (the functype)
	}
	out = append(out, 0x08)
	out = appendLEB128(out, uint32(len(canonSection)))
	out = append(out, canonSection...)

	// === Section 11: Export Section ===
	// Export "echo" as function 0
	exportSection := []byte{
		0x01,                         // 1 export
		0x00,                         // simple name
		0x04, 0x65, 0x63, 0x68, 0x6f, // name "echo"
		0x01,                         // sort = func
		0x00,                         // index = 0
	}
	out = append(out, 0x0b)
	out = appendLEB128(out, uint32(len(exportSection)))
	out = append(out, exportSection...)

	if err := os.WriteFile("../echo_record.wasm", out, 0644); err != nil {
		panic(err)
	}
	println("Generated echo_record.wasm")
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
