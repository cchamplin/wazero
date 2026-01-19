//go:build ignore

// This program generates the option_roundtrip.wasm test fixture.
// Run with: go run gen_option_roundtrip.go
//
// This component defines an option type `option<s32>`
// and exports a function `echo(o: option<s32>) -> option<s32>` that returns the input unchanged.
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
	// The function takes (i32, i32) and returns (i32, i32) - unchanged values
	// For option<s32>, the flat ABI is (discriminant: i32, payload: i32)
	// where discriminant 0=None, 1=Some
	coreModule := []byte{
		// Magic + version
		0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
		// Type section (1): 1 type - (i32, i32) -> (i32, i32)
		0x01, 0x08, 0x01, 0x60,
		0x02, 0x7f, 0x7f, // 2 params: i32, i32
		0x02, 0x7f, 0x7f, // 2 results: i32, i32
		// Function section (3): 1 function using type 0
		0x03, 0x02, 0x01, 0x00,
		// Export section (7): export "echo" as function 0
		0x07, 0x08, 0x01, 0x04, 0x65, 0x63, 0x68, 0x6f, 0x00, 0x00, // "echo"
		// Code section (10): 1 function body
		// Function body: local.get 0, local.get 1 (just return inputs unchanged)
		0x0a, 0x08, 0x01, // code section, size 8, 1 function
		0x06, 0x00, // function body size 6, 0 locals
		0x20, 0x00, // local.get 0 (discriminant)
		0x20, 0x01, // local.get 1 (payload)
		0x0b, // end
	}

	// Section ID 1 (core module) + size + content
	out = append(out, 0x01)
	out = appendLEB128(out, uint32(len(coreModule)))
	out = append(out, coreModule...)

	// === Section 7: Type Section ===
	// Type 0: option<s32>
	// Type 1: functype (option<s32>) -> option<s32>
	typeSection := []byte{
		0x02, // 2 types

		// Type 0: option type
		0x6b, // option opcode
		0x7a, // s32 valtype

		// Type 1: functype (option<s32>) -> option<s32>
		0x40,      // functype sync
		0x01,      // 1 param
		0x01, 'o', // param name "o" (length 1)
		0x00,      // type index 0 (the option type)
		0x00,      // single result
		0x00,      // type index 0 (the option type)
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
		0x00,                         // no externdesc (REQUIRED)
	}
	out = append(out, 0x0b)
	out = appendLEB128(out, uint32(len(exportSection)))
	out = append(out, exportSection...)

	if err := os.WriteFile("../option_roundtrip.wasm", out, 0644); err != nil {
		panic(err)
	}
	println("Generated option_roundtrip.wasm")
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
