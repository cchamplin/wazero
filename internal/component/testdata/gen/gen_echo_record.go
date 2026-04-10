//go:build ignore

// This program generates the echo_record.wasm test fixture.
// Run with: go run gen_echo_record.go
//
// This component defines a record type `point { x: s32, y: s32 }`
// and exports a function `echo(p: point) -> point` that doubles the coordinates.
//
// Since record results (2 i32s) exceed MaxFlatResults (1), the core ABI
// uses the retptr convention in the lift context: the core function allocates
// a result buffer in linear memory, writes results there, and returns the
// pointer as a single i32.
// Core ABI (lift context): (i32, i32) -> (i32) where result is retptr.
//
// The component structure follows the required pattern for record types used
// in exports: the record type is defined in a type section, then imported
// via a type import with (eq 0) to create an importable type alias. The
// functype references the imported alias. This satisfies the component model
// validation rule that types referenced by exported functions must be
// importable.
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

	// === Section 7: Type Section (first) ===
	// Type 0: record point { x: s32, y: s32 }
	typeSection0 := []byte{
		0x01, // 1 type

		// Type 0: record type
		0x72,            // record opcode
		0x02,            // 2 fields
		0x01, 'x', 0x7a, // field "x": s32
		0x01, 'y', 0x7a, // field "y": s32
	}
	out = append(out, 0x07)
	out = appendLEB128(out, uint32(len(typeSection0)))
	out = append(out, typeSection0...)

	// === Section 10: Import Section ===
	// Import "point" as type (eq 0) → creates type 1
	// This is required by the component model validation: types used in
	// exported functions must be importable (imported or exported).
	importSection := []byte{
		0x01, // 1 import
		// Import name: simple name "point"
		0x00,                                  // simple name discriminator
		0x05, 'p', 'o', 'i', 'n', 't',       // name "point" (len=5)
		// ExternDesc: type (eq 0)
		0x03, // externdesc kind = type
		0x00, // typebound tag = eq
		0x00, // typeidx = 0
	}
	out = append(out, 0x0a)
	out = appendLEB128(out, uint32(len(importSection)))
	out = append(out, importSection...)

	// === Section 1: Core Module ===
	// Core module with memory, echo function (retptr ABI), and realloc.
	coreModule := buildEchoRecordCoreModule()

	// Section ID 1 (core module) + size + content
	out = append(out, 0x01)
	out = appendLEB128(out, uint32(len(coreModule)))
	out = append(out, coreModule...)

	// === Section 2: Core Instance Section ===
	// Instantiate core module 0 with no imports
	coreInstanceSection := []byte{
		0x01, // 1 core instance
		0x00, // instantiate
		0x00, // module index = 0
		0x00, // 0 args
	}
	out = append(out, 0x02)
	out = appendLEB128(out, uint32(len(coreInstanceSection)))
	out = append(out, coreInstanceSection...)

	// === Section 6: Alias Section (memory) ===
	// Alias core memory "memory" from instance 0 (core memory idx 0)
	aliasSection0 := []byte{
		0x01, // 1 alias

		// Alias 0: core memory "memory" from instance 0
		0x00,                                  // sort prefix for core sort
		0x02,                                  // core sort = memory
		0x01,                                  // alias target = core export
		0x00,                                  // core instance index = 0
		0x06, 'm', 'e', 'm', 'o', 'r', 'y', // export name "memory" (len=6)
	}
	out = append(out, 0x06)
	out = appendLEB128(out, uint32(len(aliasSection0)))
	out = append(out, aliasSection0...)

	// === Section 7: Type Section (second) ===
	// Type 2: functype (param "p" type_1) (result type_1)
	// References type 1 (the imported alias of the record type).
	typeSection1 := []byte{
		0x01, // 1 type

		// Type 2: functype (point) -> point
		0x40,      // functype sync
		0x01,      // 1 param
		0x01, 'p', // param name "p" (length 1)
		0x01,      // type index 1 (the imported record type alias)
		0x00,      // single result
		0x01,      // type index 1 (the imported record type alias)
	}
	out = append(out, 0x07)
	out = appendLEB128(out, uint32(len(typeSection1)))
	out = append(out, typeSection1...)

	// === Section 6: Alias Section (core funcs) ===
	// Alias core funcs "echo" and "realloc" from instance 0
	aliasSection1 := []byte{
		0x02, // 2 aliases

		// Core func 0: "echo" from instance 0
		0x00,                      // sort prefix for core sort
		0x00,                      // core sort = func
		0x01,                      // alias target = core export
		0x00,                      // core instance index = 0
		0x04, 'e', 'c', 'h', 'o', // export name "echo" (len=4)

		// Core func 1: "realloc" from instance 0
		0x00,                                      // sort prefix for core sort
		0x00,                                      // core sort = func
		0x01,                                      // alias target = core export
		0x00,                                      // core instance index = 0
		0x07, 'r', 'e', 'a', 'l', 'l', 'o', 'c', // export name "realloc" (len=7)
	}
	out = append(out, 0x06)
	out = appendLEB128(out, uint32(len(aliasSection1)))
	out = append(out, aliasSection1...)

	// === Section 8: Canon Section ===
	// Lift core function 0 as component function type 2
	// With memory option (core memory 0) and realloc option (core func 1)
	canonSection := []byte{
		0x01, // 1 canonical
		0x00, // canon.lift
		0x00, // core sort
		0x00, // core func index = 0 (aliased echo)
		0x02, // 2 options
		0x03, // memory option
		0x00, // memory index = 0 (aliased memory)
		0x04, // realloc option
		0x01, // realloc func index = 1 (aliased realloc)
		0x02, // type index = 2 (the functype referencing imported type)
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
		0x01, // sort = func
		0x00, // index = 0
		0x00, // no externdesc
	}
	out = append(out, 0x0b)
	out = appendLEB128(out, uint32(len(exportSection)))
	out = append(out, exportSection...)

	if err := os.WriteFile("../echo_record.wasm", out, 0644); err != nil {
		panic(err)
	}
	println("Generated echo_record.wasm")
}

// buildEchoRecordCoreModule creates a core wasm module with:
// - Memory (1 page)
// - Global for bump allocator pointer (starts at 1024)
// - Function 0: echo(x: i32, y: i32) -> (i32)
//   Allocates 8 bytes via bump allocator, writes x*2 and y*2, returns ptr.
//   In the lift context, the core function returns the retptr (not receives it).
// - Function 1: realloc(old_ptr: i32, old_size: i32, align: i32, new_size: i32) -> i32
//   Simple bump allocator
// - Exports: "echo", "realloc", "memory"
func buildEchoRecordCoreModule() []byte {
	module := []byte{
		// Magic + version
		0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
	}

	// Type section: 2 types
	// Type 0: (i32, i32) -> (i32) [echo — lift context retptr: returns ptr]
	// Type 1: (i32, i32, i32, i32) -> i32 [realloc]
	typeSection := []byte{
		0x02,                          // 2 types
		0x60,                          // func type 0
		0x02, 0x7f, 0x7f,             // 2 params: i32, i32
		0x01, 0x7f,                    // 1 result: i32
		0x60,                          // func type 1
		0x04, 0x7f, 0x7f, 0x7f, 0x7f, // 4 params: i32, i32, i32, i32
		0x01, 0x7f,                    // 1 result: i32
	}
	module = append(module, 0x01) // type section ID
	module = appendLEB128(module, uint32(len(typeSection)))
	module = append(module, typeSection...)

	// Function section: 2 functions
	funcSection := []byte{0x02, 0x00, 0x01} // func 0 = type 0, func 1 = type 1
	module = append(module, 0x03)
	module = appendLEB128(module, uint32(len(funcSection)))
	module = append(module, funcSection...)

	// Memory section: 1 memory with min 1 page
	memSection := []byte{
		0x01,       // 1 memory
		0x00, 0x01, // limits: min 1, no max
	}
	module = append(module, 0x05)
	module = appendLEB128(module, uint32(len(memSection)))
	module = append(module, memSection...)

	// Global section: 1 mutable i32 for bump allocator (start at 1024)
	globalSection := []byte{
		0x01,             // 1 global
		0x7f, 0x01,       // i32, mutable
		0x41, 0x80, 0x08, // i32.const 1024
		0x0b,             // end
	}
	module = append(module, 0x06)
	module = appendLEB128(module, uint32(len(globalSection)))
	module = append(module, globalSection...)

	// Export section: echo, realloc, memory
	exportSection := []byte{
		0x03, // 3 exports
		0x04, 'e', 'c', 'h', 'o', 0x00, 0x00, // "echo" as function 0
		0x07, 'r', 'e', 'a', 'l', 'l', 'o', 'c', 0x00, 0x01, // "realloc" as function 1
		0x06, 'm', 'e', 'm', 'o', 'r', 'y', 0x02, 0x00, // "memory" as memory 0
	}
	module = append(module, 0x07)
	module = appendLEB128(module, uint32(len(exportSection)))
	module = append(module, exportSection...)

	// Code section: 2 functions

	// Function 0: echo(x: i32, y: i32) -> (i32)
	// Allocates 8 bytes via bump allocator, writes x*2 and y*2, returns ptr.
	// In the lift context, the core function returns the retptr.
	echoBody := []byte{
		0x01,       // 1 local group
		0x01, 0x7f, // 1 local of type i32 (local 2 = retptr)

		// Allocate 8 bytes: retptr = align4(global[0]); global[0] = retptr + 8
		0x23, 0x00,       // global.get 0
		0x41, 0x03,       // i32.const 3
		0x6a,             // i32.add
		0x41, 0x7c,       // i32.const -4
		0x71,             // i32.and
		0x21, 0x02,       // local.set 2 (retptr)
		0x20, 0x02,       // local.get 2
		0x41, 0x08,       // i32.const 8
		0x6a,             // i32.add
		0x24, 0x00,       // global.set 0

		// mem[retptr+0] = x * 2
		0x20, 0x02,       // local.get 2 (retptr)
		0x20, 0x00,       // local.get 0 (x)
		0x41, 0x02,       // i32.const 2
		0x6c,             // i32.mul
		0x36, 0x02, 0x00, // i32.store offset=0 align=4

		// mem[retptr+4] = y * 2
		0x20, 0x02,       // local.get 2 (retptr)
		0x20, 0x01,       // local.get 1 (y)
		0x41, 0x02,       // i32.const 2
		0x6c,             // i32.mul
		0x36, 0x02, 0x04, // i32.store offset=4 align=4

		// return retptr
		0x20, 0x02, // local.get 2
		0x0b,       // end
	}

	// Function 1: realloc(old_ptr, old_size, align, new_size) -> ptr
	// Simple bump allocator
	reallocBody := []byte{
		0x01,       // 1 local group
		0x01, 0x7f, // 1 local of type i32

		// Get current bump pointer and align it
		0x23, 0x00, // global.get 0
		0x41, 0x03, // i32.const 3
		0x6a,       // i32.add
		0x41, 0x7c, // i32.const -4
		0x71,       // i32.and
		0x21, 0x04, // local.set 4 (aligned_ptr)

		// Update bump pointer: global = aligned_ptr + new_size
		0x20, 0x04, // local.get 4
		0x20, 0x03, // local.get 3 (new_size)
		0x6a,       // i32.add
		0x24, 0x00, // global.set 0

		// Return aligned_ptr
		0x20, 0x04, // local.get 4
		0x0b,       // end
	}

	codeSection := []byte{0x02} // 2 functions
	codeSection = appendLEB128(codeSection, uint32(len(echoBody)))
	codeSection = append(codeSection, echoBody...)
	codeSection = appendLEB128(codeSection, uint32(len(reallocBody)))
	codeSection = append(codeSection, reallocBody...)

	module = append(module, 0x0a) // code section ID
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
