//go:build ignore

// This program generates the list_sum.wasm test fixture.
// Run with: go run gen_list_sum.go
//
// This component exports a function `sum(l: list<s32>) -> s32` that
// sums all elements in the list.
//
// Lists require linear memory for the data. The component has:
// - A core module with memory and a sum function
// - Type section with list<s32> and functype
// - Canon lift with memory option
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
	// A core wasm module with memory that exports a sum function.
	// The function takes (ptr: i32, len: i32) -> i32 and sums the s32 values
	// in memory at ptr[0..len].
	coreModule := buildCoreModule()

	// Section ID 1 (core module) + size + content
	out = append(out, 0x01)
	out = appendLEB128(out, uint32(len(coreModule)))
	out = append(out, coreModule...)

	// === Section 7: Type Section ===
	// Type 0: list<s32>
	// Type 1: functype (list<s32>) -> s32
	typeSection := []byte{
		0x02, // 2 types

		// Type 0: list<s32>
		0x70, // list opcode
		0x7a, // s32 valtype

		// Type 1: functype (list<s32>) -> s32
		0x40,      // functype sync
		0x01,      // 1 param
		0x01, 'l', // param name "l" (length 1)
		0x00,      // type index 0 (the list type)
		0x00,      // single result
		0x7a,      // s32
	}
	out = append(out, 0x07)
	out = appendLEB128(out, uint32(len(typeSection)))
	out = append(out, typeSection...)

	// === Section 8: Canon Section ===
	// Lift core function 0 as component function type 1
	// With memory option pointing to core memory 0
	canonSection := []byte{
		0x01, // 1 canonical
		0x00, // canon.lift
		0x00, // core sort
		0x00, // core func index = 0
		0x01, // 1 option
		0x03, // memory option
		0x00, // memory index = 0
		0x01, // type index = 1 (the functype)
	}
	out = append(out, 0x08)
	out = appendLEB128(out, uint32(len(canonSection)))
	out = append(out, canonSection...)

	// === Section 11: Export Section ===
	// Export "sum" as function 0
	exportSection := []byte{
		0x01,                // 1 export
		0x00,                // simple name
		0x03, 's', 'u', 'm', // name "sum"
		0x01,                // sort = func
		0x00,                // index = 0
		0x00,                // no externdesc (REQUIRED)
	}
	out = append(out, 0x0b)
	out = appendLEB128(out, uint32(len(exportSection)))
	out = append(out, exportSection...)

	if err := os.WriteFile("../list_sum.wasm", out, 0644); err != nil {
		panic(err)
	}
	println("Generated list_sum.wasm")
}

// buildCoreModule creates a core wasm module with:
// - Memory (1 page minimum)
// - Function sum(ptr: i32, len: i32) -> i32 that sums s32 values
// - Memory export "memory"
// - Function export "sum"
func buildCoreModule() []byte {
	// Core module bytes
	module := []byte{
		// Magic + version
		0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
	}

	// Type section (1): 1 type - (i32, i32) -> i32
	typeSection := []byte{
		0x01,                         // 1 type
		0x60,                         // func
		0x02, 0x7f, 0x7f,             // 2 params: i32, i32
		0x01, 0x7f,                   // 1 result: i32
	}
	module = append(module, 0x01) // type section ID
	module = appendLEB128(module, uint32(len(typeSection)))
	module = append(module, typeSection...)

	// Function section (3): 1 function using type 0
	funcSection := []byte{0x01, 0x00}
	module = append(module, 0x03)
	module = appendLEB128(module, uint32(len(funcSection)))
	module = append(module, funcSection...)

	// Memory section (5): 1 memory with min 1 page
	memSection := []byte{
		0x01,       // 1 memory
		0x00, 0x01, // limits: min 1, no max
	}
	module = append(module, 0x05)
	module = appendLEB128(module, uint32(len(memSection)))
	module = append(module, memSection...)

	// Export section (7): export "sum" as function 0, "memory" as memory 0
	exportSection := []byte{
		0x02,                                   // 2 exports
		0x03, 's', 'u', 'm', 0x00, 0x00,        // "sum" as function 0
		0x06, 'm', 'e', 'm', 'o', 'r', 'y', 0x02, 0x00, // "memory" as memory 0
	}
	module = append(module, 0x07)
	module = appendLEB128(module, uint32(len(exportSection)))
	module = append(module, exportSection...)

	// Code section (10): 1 function body
	// Function: sum(ptr: i32, len: i32) -> i32
	// locals: sum (local 2), i (local 3)
	//
	// Algorithm:
	//   sum = 0
	//   i = 0
	//   while i < len:
	//     sum += mem[ptr + i*4]
	//     i++
	//   return sum
	funcBody := []byte{
		// Locals declaration: 2 locals of type i32
		0x01,       // 1 local group
		0x02, 0x7f, // 2 locals of type i32

		// Initialize sum = 0
		0x41, 0x00, // i32.const 0
		0x21, 0x02, // local.set 2 (sum)

		// Initialize i = 0
		0x41, 0x00, // i32.const 0
		0x21, 0x03, // local.set 3 (i)

		// Block for loop structure
		0x02, 0x40, // block (void)

		// Loop start
		0x03, 0x40, // loop (void)

		// Check: i >= len -> break
		0x20, 0x03, // local.get 3 (i)
		0x20, 0x01, // local.get 1 (len)
		0x4f,       // i32.ge_u (opcode 0x4f)
		0x0d, 0x01, // br_if 1 (break out of block)

		// sum += mem[ptr + i*4]
		0x20, 0x02, // local.get 2 (sum)
		0x20, 0x00, // local.get 0 (ptr)
		0x20, 0x03, // local.get 3 (i)
		0x41, 0x04, // i32.const 4
		0x6c,       // i32.mul
		0x6a,       // i32.add (ptr + i*4)
		0x28, 0x02, 0x00, // i32.load offset=0 align=4
		0x6a,       // i32.add (sum + mem value)
		0x21, 0x02, // local.set 2 (sum)

		// i++
		0x20, 0x03, // local.get 3 (i)
		0x41, 0x01, // i32.const 1
		0x6a,       // i32.add
		0x21, 0x03, // local.set 3 (i)

		// Continue loop
		0x0c, 0x00, // br 0 (back to loop start)

		0x0b, // end loop
		0x0b, // end block

		// Return sum
		0x20, 0x02, // local.get 2 (sum)
		0x0b,       // end function
	}

	codeSection := []byte{0x01} // 1 function
	codeSection = appendLEB128(codeSection, uint32(len(funcBody)))
	codeSection = append(codeSection, funcBody...)

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
