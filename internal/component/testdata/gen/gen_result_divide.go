//go:build ignore

// This program generates the result_divide.wasm test fixture.
// Run with: go run gen_result_divide.go
//
// This component defines a result type `result<s32, s32>`
// and exports a function `divide(a: s32, b: s32) -> result<s32, s32>`.
// If b == 0, returns Error(1) (division by zero).
// Otherwise, returns Ok(a / b).
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
	// A minimal core wasm module that exports a divide function
	// The function takes (i32, i32) and returns (i32, i32)
	// For result<s32, s32>, the flat ABI is (discriminant: i32, payload: i32)
	// where discriminant 0=Ok, 1=Error
	//
	// Pseudocode:
	//   if b == 0:
	//       return (1, 1)  ; Error discriminant, error code 1
	//   else:
	//       return (0, a / b)  ; Ok discriminant, quotient
	coreModule := []byte{
		// Magic + version
		0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
		// Type section (1): 1 type - (i32, i32) -> (i32, i32)
		0x01, 0x08, 0x01, 0x60,
		0x02, 0x7f, 0x7f, // 2 params: i32, i32
		0x02, 0x7f, 0x7f, // 2 results: i32, i32
		// Function section (3): 1 function using type 0
		0x03, 0x02, 0x01, 0x00,
		// Export section (7): export "divide" as function 0
		0x07, 0x0a, 0x01, 0x06, 'd', 'i', 'v', 'i', 'd', 'e', 0x00, 0x00,
		// Code section (10): 1 function body
		// Function body implements:
		//   if (local.get 1 == 0)
		//       return (1, 1)   ; Error(1)
		//   else
		//       return (0, local.get 0 / local.get 1)  ; Ok(a/b)
		0x0a, // code section
		0x17, // section size: 23 bytes
		0x01, // 1 function
		0x15, // function body size: 21 bytes
		0x00, // 0 locals
		// if (local.get 1 == 0)
		0x20, 0x01, // local.get 1 (b)
		0x45,       // i32.eqz
		0x04, 0x7f, // if with i32 result (block type for first result)
		// then branch: return Error(1)
		0x41, 0x01, // i32.const 1 (error discriminant)
		0x05, // else
		// else branch: return Ok(a/b)
		0x41, 0x00, // i32.const 0 (ok discriminant)
		0x0b,       // end if
		// Now we need to compute the second value based on the branch taken
		// Actually, we need a different approach - the if only returns one value
		// Let's restructure to use two separate if blocks
	}

	// Let me reconstruct the core module with proper multi-value handling
	// Function body bytes (after local declarations):
	// local.get 1 (2), i32.eqz (1), if void (2), i32.const 1 (2), local.set 2 (2),
	// i32.const 1 (2), local.set 3 (2), else (1), i32.const 0 (2), local.set 2 (2),
	// local.get 0 (2), local.get 1 (2), i32.div_s (1), local.set 3 (2), end (1),
	// local.get 2 (2), local.get 3 (2), end (1) = 31 bytes
	// + local decl: 1 + 2 = 3 bytes
	// Total body: 34 bytes
	coreModule = []byte{
		// Magic + version
		0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
		// Type section (1): 1 type - (i32, i32) -> (i32, i32)
		0x01, 0x08, 0x01, 0x60,
		0x02, 0x7f, 0x7f, // 2 params: i32, i32
		0x02, 0x7f, 0x7f, // 2 results: i32, i32
		// Function section (3): 1 function using type 0
		0x03, 0x02, 0x01, 0x00,
		// Export section (7): export "divide" as function 0
		0x07, 0x0a, 0x01, 0x06, 'd', 'i', 'v', 'i', 'd', 'e', 0x00, 0x00,
		// Code section (10): 1 function body
		// Using a simpler approach with local variables and single-value ifs
		0x0a,       // code section
		0x24,       // section size: 36 bytes (1 func count + 1 body size + 34 body)
		0x01,       // 1 function
		0x22,       // function body size: 34 bytes
		0x01,       // 1 local declaration
		0x02, 0x7f, // 2 locals of type i32 (result discriminant and payload)
		// Check if b == 0
		0x20, 0x01, // local.get 1 (b)
		0x45,       // i32.eqz
		0x04, 0x40, // if (void block type)
		// then: set locals to Error(1)
		0x41, 0x01, // i32.const 1
		0x21, 0x02, // local.set 2 (discriminant = 1 = Error)
		0x41, 0x01, // i32.const 1
		0x21, 0x03, // local.set 3 (payload = 1 = error code)
		0x05, // else
		// else: set locals to Ok(a/b)
		0x41, 0x00, // i32.const 0
		0x21, 0x02, // local.set 2 (discriminant = 0 = Ok)
		0x20, 0x00, // local.get 0 (a)
		0x20, 0x01, // local.get 1 (b)
		0x6d,       // i32.div_s
		0x21, 0x03, // local.set 3 (payload = a/b)
		0x0b,       // end if
		// Return both values
		0x20, 0x02, // local.get 2 (discriminant)
		0x20, 0x03, // local.get 3 (payload)
		0x0b, // end function
	}

	// Section ID 1 (core module) + size + content
	out = append(out, 0x01)
	out = appendLEB128(out, uint32(len(coreModule)))
	out = append(out, coreModule...)

	// === Section 7: Type Section ===
	// Type 0: result<s32, s32>
	// Type 1: functype (s32, s32) -> result<s32, s32>
	typeSection := []byte{
		0x02, // 2 types

		// Type 0: result type
		// result<T, E> ::= 0x6a [ok_type] [err_type]
		// where each is: 0x00 (no type) or 0x01 followed by valtype
		0x6a,       // result opcode
		0x01, 0x7a, // has ok type: s32
		0x01, 0x7a, // has err type: s32

		// Type 1: functype (s32, s32) -> result<s32, s32>
		0x40,           // functype sync
		0x02,           // 2 params
		0x01, 'a',      // param name "a" (length 1)
		0x7a,           // s32 (primitive)
		0x01, 'b',      // param name "b" (length 1)
		0x7a,           // s32 (primitive)
		0x00,           // single result (not named)
		0x00,           // type index 0 (the result type)
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
	// Export "divide" as function 0
	exportSection := []byte{
		0x01,                               // 1 export
		0x00,                               // simple name
		0x06, 'd', 'i', 'v', 'i', 'd', 'e', // name "divide"
		0x01, // sort = func
		0x00, // index = 0
		0x00, // no externdesc (REQUIRED)
	}
	out = append(out, 0x0b)
	out = appendLEB128(out, uint32(len(exportSection)))
	out = append(out, exportSection...)

	if err := os.WriteFile("../result_divide.wasm", out, 0644); err != nil {
		panic(err)
	}
	println("Generated result_divide.wasm")
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
