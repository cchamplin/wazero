//go:build ignore

// This program generates the option_roundtrip.wasm test fixture.
// Run with: go run gen_option_roundtrip.go
//
// This component defines an option type `option<s32>`
// and exports a function `echo(o: option<s32>) -> option<s32>` that returns the input unchanged.
//
// Since option<s32> results (2 i32s: discriminant + payload) exceed MaxFlatResults (1),
// the core ABI uses the retptr convention.
// Core ABI: (i32, i32, i32) -> () where third param is retptr.
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
	coreModule := buildOptionRoundtripCoreModule()

	out = append(out, 0x01)
	out = appendLEB128(out, uint32(len(coreModule)))
	out = append(out, coreModule...)

	// === Section 2: Core Instance Section ===
	coreInstanceSection := []byte{
		0x01, // 1 core instance
		0x00, // instantiate
		0x00, // module index = 0
		0x00, // 0 args
	}
	out = append(out, 0x02)
	out = appendLEB128(out, uint32(len(coreInstanceSection)))
	out = append(out, coreInstanceSection...)

	// === Section 6: Alias Section ===
	aliasSection := []byte{
		0x03, // 3 aliases

		// Alias 0: core func "echo" (core func idx 0)
		0x00,
		0x00,
		0x01,
		0x00,
		0x04, 'e', 'c', 'h', 'o',

		// Alias 1: core func "realloc" (core func idx 1)
		0x00,
		0x00,
		0x01,
		0x00,
		0x07, 'r', 'e', 'a', 'l', 'l', 'o', 'c',

		// Alias 2: core memory "memory" (core memory idx 0)
		0x00,
		0x02,
		0x01,
		0x00,
		0x06, 'm', 'e', 'm', 'o', 'r', 'y',
	}
	out = append(out, 0x06)
	out = appendLEB128(out, uint32(len(aliasSection)))
	out = append(out, aliasSection...)

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
	canonSection := []byte{
		0x01, // 1 canonical
		0x00, // canon.lift
		0x00, // core sort
		0x00, // core func index = 0 (aliased echo)
		0x02, // 2 options
		0x03, // memory option
		0x00, // memory index = 0
		0x04, // realloc option
		0x01, // realloc func index = 1
		0x01, // type index = 1 (the functype)
	}
	out = append(out, 0x08)
	out = appendLEB128(out, uint32(len(canonSection)))
	out = append(out, canonSection...)

	// === Section 11: Export Section ===
	exportSection := []byte{
		0x01,                         // 1 export
		0x00,                         // simple name
		0x04, 0x65, 0x63, 0x68, 0x6f, // name "echo"
		0x01, // sort = func
		0x00, // index = 0
		0x00, // no externdesc
	}
	out = append(out, 0x0b)
	out = appendLEB128(out, uint32(len(exportSection)))
	out = append(out, exportSection...)

	if err := os.WriteFile("../option_roundtrip.wasm", out, 0644); err != nil {
		panic(err)
	}
	println("Generated option_roundtrip.wasm")
}

// buildOptionRoundtripCoreModule creates a core wasm module:
// - Function 0: echo(disc: i32, payload: i32, retptr: i32) -> ()
//   Writes disc to mem[retptr], payload to mem[retptr+4]
// - Function 1: realloc(old_ptr, old_size, align, new_size) -> ptr
// - Memory, global for bump allocator
func buildOptionRoundtripCoreModule() []byte {
	module := []byte{
		0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
	}

	// Type section
	typeSection := []byte{
		0x02,
		0x60, 0x03, 0x7f, 0x7f, 0x7f, 0x00,             // type 0: (i32, i32, i32) -> ()
		0x60, 0x04, 0x7f, 0x7f, 0x7f, 0x7f, 0x01, 0x7f, // type 1: (i32, i32, i32, i32) -> i32
	}
	module = append(module, 0x01)
	module = appendLEB128(module, uint32(len(typeSection)))
	module = append(module, typeSection...)

	// Function section
	funcSection := []byte{0x02, 0x00, 0x01}
	module = append(module, 0x03)
	module = appendLEB128(module, uint32(len(funcSection)))
	module = append(module, funcSection...)

	// Memory section
	memSection := []byte{0x01, 0x00, 0x01}
	module = append(module, 0x05)
	module = appendLEB128(module, uint32(len(memSection)))
	module = append(module, memSection...)

	// Global section
	globalSection := []byte{
		0x01, 0x7f, 0x01, 0x41, 0x80, 0x08, 0x0b,
	}
	module = append(module, 0x06)
	module = appendLEB128(module, uint32(len(globalSection)))
	module = append(module, globalSection...)

	// Export section
	exportSection := []byte{
		0x03,
		0x04, 'e', 'c', 'h', 'o', 0x00, 0x00,
		0x07, 'r', 'e', 'a', 'l', 'l', 'o', 'c', 0x00, 0x01,
		0x06, 'm', 'e', 'm', 'o', 'r', 'y', 0x02, 0x00,
	}
	module = append(module, 0x07)
	module = appendLEB128(module, uint32(len(exportSection)))
	module = append(module, exportSection...)

	// Code section
	// Function 0: echo - write disc and payload to retptr
	echoBody := []byte{
		0x00, // 0 locals
		0x20, 0x02,       // local.get 2 (retptr)
		0x20, 0x00,       // local.get 0 (disc)
		0x36, 0x02, 0x00, // i32.store offset=0 align=4
		0x20, 0x02,       // local.get 2 (retptr)
		0x20, 0x01,       // local.get 1 (payload)
		0x36, 0x02, 0x04, // i32.store offset=4 align=4
		0x0b,
	}

	// Function 1: realloc
	reallocBody := []byte{
		0x01, 0x01, 0x7f, // 1 local: i32
		0x23, 0x00,       // global.get 0
		0x41, 0x03, 0x6a, // i32.const 3, i32.add
		0x41, 0x7c, 0x71, // i32.const -4, i32.and
		0x21, 0x04,       // local.set 4
		0x20, 0x04,       // local.get 4
		0x20, 0x03,       // local.get 3
		0x6a,             // i32.add
		0x24, 0x00,       // global.set 0
		0x20, 0x04,       // local.get 4
		0x0b,
	}

	codeSection := []byte{0x02}
	codeSection = appendLEB128(codeSection, uint32(len(echoBody)))
	codeSection = append(codeSection, echoBody...)
	codeSection = appendLEB128(codeSection, uint32(len(reallocBody)))
	codeSection = append(codeSection, reallocBody...)

	module = append(module, 0x0a)
	module = appendLEB128(module, uint32(len(codeSection)))
	module = append(module, codeSection...)

	return module
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
