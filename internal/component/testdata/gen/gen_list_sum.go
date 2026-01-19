//go:build ignore

// This program generates the list_sum.wasm test fixture.
// Run with: go run gen_list_sum.go
//
// This component exports a function `sum(l: list<s32>) -> s32` that
// sums all elements in the list.
//
// Lists require linear memory for the data. The component has:
// - A core module with memory, a sum function, and a realloc function
// - Type section with list<s32> and functype
// - Canon lift with memory and realloc options
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
	// A core wasm module with memory that exports a sum function and realloc.
	// The function takes (ptr: i32, len: i32) -> i32 and sums the s32 values
	// in memory at ptr[0..len].
	coreModule := buildCoreModule()

	// Section ID 1 (core module) + size + content
	out = append(out, 0x01)
	out = appendLEB128(out, uint32(len(coreModule)))
	out = append(out, coreModule...)

	// === Section 2: Core Instance Section ===
	// Instantiate core module 0 with no arguments
	coreInstanceSection := []byte{
		0x01,       // 1 core instance
		0x00,       // instantiate
		0x00,       // module index = 0
		0x00,       // 0 args (no imports)
	}
	out = append(out, 0x02)
	out = appendLEB128(out, uint32(len(coreInstanceSection)))
	out = append(out, coreInstanceSection...)

	// === Section 5: Alias Section ===
	// Alias core exports into the component's core index spaces
	// This is required for canon lift to reference them by index
	// Format: sort aliastarget
	// For core exports: sort = 0x00 + core_sort_byte, aliastarget = 0x01 instanceidx name
	aliasSection := []byte{
		0x03, // 3 aliases

		// Alias 0: core export "sum" from instance 0 (becomes core func 0)
		0x00,                // sort prefix for core sort
		0x00,                // core sort = func
		0x01,                // alias target = core export
		0x00,                // instance index = 0
		0x03, 's', 'u', 'm', // name "sum"

		// Alias 1: core export "realloc" from instance 0 (becomes core func 1)
		0x00,                                     // sort prefix for core sort
		0x00,                                     // core sort = func
		0x01,                                     // alias target = core export
		0x00,                                     // instance index = 0
		0x07, 'r', 'e', 'a', 'l', 'l', 'o', 'c', // name "realloc"

		// Alias 2: core export "memory" from instance 0 (becomes core memory 0)
		0x00,                                  // sort prefix for core sort
		0x02,                                  // core sort = memory
		0x01,                                  // alias target = core export
		0x00,                                  // instance index = 0
		0x06, 'm', 'e', 'm', 'o', 'r', 'y',    // name "memory"
	}
	out = append(out, 0x06) // section ID = 6 (alias)
	out = appendLEB128(out, uint32(len(aliasSection)))
	out = append(out, aliasSection...)

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
	// Lift core function 0 (aliased sum) as component function type 1
	// With memory option pointing to core memory 0 (aliased memory)
	// and realloc option pointing to core function 1 (aliased realloc)
	canonSection := []byte{
		0x01, // 1 canonical
		0x00, // canon.lift
		0x00, // core sort
		0x00, // core func index = 0 (aliased sum)
		0x02, // 2 options
		0x03, // memory option
		0x00, // memory index = 0 (aliased memory)
		0x04, // realloc option
		0x01, // realloc func index = 1 (aliased realloc)
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
// - Global for bump allocator pointer
// - Function sum(ptr: i32, len: i32) -> i32 that sums s32 values
// - Function realloc(old_ptr: i32, old_size: i32, align: i32, new_size: i32) -> i32
// - Memory export "memory"
// - Function exports "sum" and "realloc"
func buildCoreModule() []byte {
	// Core module bytes
	module := []byte{
		// Magic + version
		0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
	}

	// Type section (1): 2 types
	// Type 0: (i32, i32) -> i32 (sum function)
	// Type 1: (i32, i32, i32, i32) -> i32 (realloc function)
	typeSection := []byte{
		0x02,                         // 2 types
		0x60,                         // func type 0
		0x02, 0x7f, 0x7f,             // 2 params: i32, i32
		0x01, 0x7f,                   // 1 result: i32
		0x60,                         // func type 1
		0x04, 0x7f, 0x7f, 0x7f, 0x7f, // 4 params: i32, i32, i32, i32
		0x01, 0x7f,                   // 1 result: i32
	}
	module = append(module, 0x01) // type section ID
	module = appendLEB128(module, uint32(len(typeSection)))
	module = append(module, typeSection...)

	// Function section (3): 2 functions
	// Function 0: type 0 (sum)
	// Function 1: type 1 (realloc)
	funcSection := []byte{0x02, 0x00, 0x01}
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

	// Global section (6): 1 mutable i32 global for bump allocator
	// Start at offset 1024 to leave room for static data
	globalSection := []byte{
		0x01,       // 1 global
		0x7f, 0x01, // i32, mutable
		0x41, 0x80, 0x08, // i32.const 1024
		0x0b,       // end
	}
	module = append(module, 0x06)
	module = appendLEB128(module, uint32(len(globalSection)))
	module = append(module, globalSection...)

	// Export section (7): export "sum" as function 0, "realloc" as function 1, "memory" as memory 0
	exportSection := []byte{
		0x03,                                              // 3 exports
		0x03, 's', 'u', 'm', 0x00, 0x00,                   // "sum" as function 0
		0x07, 'r', 'e', 'a', 'l', 'l', 'o', 'c', 0x00, 0x01, // "realloc" as function 1
		0x06, 'm', 'e', 'm', 'o', 'r', 'y', 0x02, 0x00,    // "memory" as memory 0
	}
	module = append(module, 0x07)
	module = appendLEB128(module, uint32(len(exportSection)))
	module = append(module, exportSection...)

	// Code section (10): 2 function bodies

	// Function 0: sum(ptr: i32, len: i32) -> i32
	// locals: sum (local 2), i (local 3)
	//
	// Algorithm:
	//   sum = 0
	//   i = 0
	//   while i < len:
	//     sum += mem[ptr + i*4]
	//     i++
	//   return sum
	sumBody := []byte{
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

	// Function 1: realloc(old_ptr: i32, old_size: i32, align: i32, new_size: i32) -> i32
	// Simple bump allocator - ignores old_ptr and old_size
	// Returns aligned pointer, updates global bump pointer
	reallocBody := []byte{
		// No locals needed
		0x00, // 0 local groups

		// Get current bump pointer
		0x23, 0x00, // global.get 0

		// Align: ptr = (ptr + align - 1) & ~(align - 1)
		// For simplicity, we'll just align to 4 bytes (most common case)
		// ptr = (ptr + 3) & ~3
		0x41, 0x03, // i32.const 3
		0x6a,       // i32.add
		0x41, 0x7c, // i32.const -4 (0xfffffffc as signed LEB128)
		0x71,       // i32.and

		// Store aligned pointer in a local (we need to duplicate it)
		// Since we can't use locals easily, we use tee pattern
		// Actually let's use the stack properly:
		// Stack: aligned_ptr

		// Duplicate the aligned ptr for return value
		// We'll compute: new_bump = aligned_ptr + new_size
		// and return aligned_ptr

		// Get new_size (param 3)
		0x20, 0x03, // local.get 3 (new_size)

		// Stack: aligned_ptr, new_size

		// Compute new bump = aligned_ptr + new_size
		// But we need aligned_ptr twice: once for return, once for addition
		// Use local to store it

		// Actually, let's be more explicit with a local
		0x0b, // end function (placeholder - will rewrite below)
	}

	// Rewrite realloc with a local for clarity
	reallocBody = []byte{
		// 1 local group: 1 local of type i32 (for aligned_ptr)
		0x01,       // 1 local group
		0x01, 0x7f, // 1 local of type i32

		// Get current bump pointer and align it
		0x23, 0x00, // global.get 0 (bump pointer)
		0x41, 0x03, // i32.const 3
		0x6a,       // i32.add
		0x41, 0x7c, // i32.const -4 (for & ~3)
		0x71,       // i32.and
		0x21, 0x04, // local.set 4 (aligned_ptr)

		// Update bump pointer: global = aligned_ptr + new_size
		0x20, 0x04, // local.get 4 (aligned_ptr)
		0x20, 0x03, // local.get 3 (new_size)
		0x6a,       // i32.add
		0x24, 0x00, // global.set 0 (new bump pointer)

		// Return aligned_ptr
		0x20, 0x04, // local.get 4
		0x0b,       // end function
	}

	codeSection := []byte{0x02} // 2 functions
	codeSection = appendLEB128(codeSection, uint32(len(sumBody)))
	codeSection = append(codeSection, sumBody...)
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
