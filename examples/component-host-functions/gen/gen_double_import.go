//go:build ignore

// This program generates the double_import.wasm component binary.
// The component imports a function double(x: s32) -> s32 from host:util/calc
// and exports a function run(x: s32) -> s32 that calls the imported double.
//
// Run with: go run gen_double_import.go
package main

import (
	"os"
)

func main() {
	// Component preamble
	out := []byte{
		0x00, 0x61, 0x73, 0x6d, // magic: \0asm
		0x0d, 0x00,             // version
		0x01, 0x00,             // layer: component
	}

	// ==========================================================
	// Section 1: Type Section (section ID 0x07)
	// Define:
	//   type 0: functype (x: s32) -> s32
	//   type 1: instance type { export "double": func (type 0) }
	// ==========================================================
	typeSection := []byte{
		0x02, // 2 types

		// type 0: functype sync (s32) -> s32
		0x40,       // functype sync
		0x01,       // 1 param
		0x01, 0x78, // param name "x" (length 1)
		0x7a,       // s32
		0x00,       // single result
		0x7a,       // s32

		// type 1: instance type
		0x42, // instance type opcode
		0x02, // 2 declarations

		// declaration 0: type - define functype (s32) -> s32 locally (local type 0)
		0x01, // type declaration kind
		0x40, // functype sync
		0x01, // 1 param
		0x01, 0x78, // param name "x" (length 1)
		0x7a, // s32
		0x00, // single result
		0x7a, // s32

		// declaration 1: export "double" as func of local type 0
		0x04,                                     // export declaration kind
		0x00,                                     // simple export name
		0x06, 0x64, 0x6f, 0x75, 0x62, 0x6c, 0x65, // "double" (len=6)
		0x01, // externdesc = func
		0x00, // type index = 0 (local)
	}
	out = append(out, 0x07)
	out = appendLEB128(out, uint32(len(typeSection)))
	out = append(out, typeSection...)

	// ==========================================================
	// Section 2: Import Section (section ID 0x0a)
	// Import "host:util/calc" as instance of type 1
	// This creates component instance index 0.
	// ==========================================================
	importName := "host:util/calc"
	importSection := []byte{
		0x01, // 1 import
		0x00, // plain import name (prefix 0x00)
	}
	importSection = appendName(importSection, importName)
	importSection = append(importSection,
		0x05, // externdesc = instance
		0x01, // type index = 1
	)
	out = append(out, 0x0a)
	out = appendLEB128(out, uint32(len(importSection)))
	out = append(out, importSection...)

	// ==========================================================
	// Section 3: Alias Section (section ID 0x06)
	// Alias func "double" from component instance 0
	// This creates component func index 0.
	// ==========================================================
	aliasSection1 := []byte{
		0x01, // 1 alias

		// sort = func (0x01)
		0x01,
		// target = export (0x00)
		0x00,
		// instance index = 0
		0x00,
	}
	aliasSection1 = appendName(aliasSection1, "double")
	out = append(out, 0x06)
	out = appendLEB128(out, uint32(len(aliasSection1)))
	out = append(out, aliasSection1...)

	// ==========================================================
	// Section 4: Canon Section (section ID 0x08)
	// Canon lower: lower component func 0 -> core func 0
	// ==========================================================
	canonLowerSection := []byte{
		0x01, // 1 canonical

		0x01, // canon.lower
		0x00, // reserved zero byte
		0x00, // component func index = 0
		0x00, // 0 options
	}
	out = append(out, 0x08)
	out = appendLEB128(out, uint32(len(canonLowerSection)))
	out = append(out, canonLowerSection...)

	// ==========================================================
	// Section 5: Core Module Section (section ID 0x01)
	// A core wasm module that:
	//   - imports "" "double" (i32) -> i32
	//   - exports "run" which calls imported double
	// ==========================================================
	coreModule := []byte{
		// Magic + version
		0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,

		// Type section (1): 1 type - (i32) -> (i32)
		0x01, 0x06, 0x01,
		0x60,       // func type
		0x01, 0x7f, // 1 param: i32
		0x01, 0x7f, // 1 result: i32

		// Import section (2): import "" "double" as func type 0
		0x02, 0x0b, 0x01,
		0x00,                                           // module name "" (length 0)
		0x06, 0x64, 0x6f, 0x75, 0x62, 0x6c, 0x65,     // field name "double" (length 6)
		0x00,                                           // import kind = func
		0x00,                                           // type index = 0

		// Function section (3): 1 function using type 0
		0x03, 0x02, 0x01, 0x00,

		// Export section (7): export "run" as function 1 (func 0 is import, func 1 is our code)
		0x07, 0x07, 0x01,
		0x03, 0x72, 0x75, 0x6e, // "run" (length 3)
		0x00,                   // export kind = func
		0x01,                   // function index = 1

		// Code section (10): 1 function body - call imported double
		0x0a, 0x08, 0x01,
		0x06,       // function body size (includes locals + expr)
		0x00,       // 0 locals
		0x20, 0x00, // local.get 0
		0x10, 0x00, // call 0 (imported double)
		0x0b, // end
	}
	out = append(out, 0x01)
	out = appendLEB128(out, uint32(len(coreModule)))
	out = append(out, coreModule...)

	// ==========================================================
	// Section 6: Core Instance Section (section ID 0x02)
	// Two core instances:
	//   instance 0: inline, exports core func 0 as "double"
	//   instance 1: instantiate module 0, with arg "" = instance 0
	// ==========================================================
	coreInstanceSection := []byte{
		0x02, // 2 core instances

		// Core instance 0: inline
		0x01, // inline kind
		0x01, // 1 export
	}
	coreInstanceSection = appendName(coreInstanceSection, "double")
	coreInstanceSection = append(coreInstanceSection,
		0x00, // sort = func
		0x00, // core func index = 0
	)

	// Core instance 1: instantiate module 0
	coreInstanceSection = append(coreInstanceSection,
		0x00, // instantiate kind
		0x00, // module index = 0
		0x01, // 1 arg
	)
	coreInstanceSection = appendName(coreInstanceSection, "")
	coreInstanceSection = append(coreInstanceSection,
		0x12, // sort = instance (0x12)
		0x00, // instance index = 0
	)

	out = append(out, 0x02)
	out = appendLEB128(out, uint32(len(coreInstanceSection)))
	out = append(out, coreInstanceSection...)

	// ==========================================================
	// Section 7: Alias Section (section ID 0x06)
	// Alias core func "run" from core instance 1
	// -> core func index 1
	// ==========================================================
	aliasSection2 := []byte{
		0x01, // 1 alias

		// sort = core sort prefix (0x00) + core func (0x00)
		0x00, 0x00,
		// target = core export (0x01)
		0x01,
		// core instance index = 1
		0x01,
	}
	aliasSection2 = appendName(aliasSection2, "run")
	out = append(out, 0x06)
	out = appendLEB128(out, uint32(len(aliasSection2)))
	out = append(out, aliasSection2...)

	// ==========================================================
	// Section 8: Type Section (section ID 0x07)
	// type 2: functype (x: s32) -> s32 (for export)
	// ==========================================================
	typeSection2 := []byte{
		0x01, // 1 type

		// functype sync (s32) -> s32
		0x40,       // functype sync
		0x01,       // 1 param
		0x01, 0x78, // param name "x" (length 1)
		0x7a,       // s32
		0x00,       // single result
		0x7a,       // s32
	}
	out = append(out, 0x07)
	out = appendLEB128(out, uint32(len(typeSection2)))
	out = append(out, typeSection2...)

	// ==========================================================
	// Section 9: Canon Section (section ID 0x08)
	// Canon lift: lift core func 1 as component func type 2
	// -> component func index 1
	// ==========================================================
	canonLiftSection := []byte{
		0x01, // 1 canonical

		0x00, // canon.lift
		0x00, // core sort = func
		0x01, // core func index = 1
		0x00, // 0 options
		0x02, // type index = 2
	}
	out = append(out, 0x08)
	out = appendLEB128(out, uint32(len(canonLiftSection)))
	out = append(out, canonLiftSection...)

	// ==========================================================
	// Section 10: Export Section (section ID 0x0b)
	// Export "run" as component func 1
	// ==========================================================
	exportSection := []byte{
		0x01, // 1 export
		0x00, // simple name
	}
	exportSection = appendName(exportSection, "run")
	exportSection = append(exportSection,
		0x01, // sort = func
		0x01, // component func index = 1
		0x00, // no externdesc
	)
	out = append(out, 0x0b)
	out = appendLEB128(out, uint32(len(exportSection)))
	out = append(out, exportSection...)

	if err := os.WriteFile("../testdata/double_import.wasm", out, 0644); err != nil {
		panic(err)
	}
	println("Generated double_import.wasm")
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

func appendName(data []byte, name string) []byte {
	data = appendLEB128(data, uint32(len(name)))
	data = append(data, []byte(name)...)
	return data
}
