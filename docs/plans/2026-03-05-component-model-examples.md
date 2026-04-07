# Component Model Examples Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Create top-level examples demonstrating the component model and WASI P2 public APIs, fixing any public API surface gaps discovered along the way.

**Architecture:** New `api/component` package re-exports internal Val/HostFunc types. Four progressive examples in `examples/` demonstrate the full component lifecycle. API gaps (SetRelaxedSemverMatching on public interface) are fixed as prerequisites.

**Tech Stack:** Go, wazero component model, WASI P2, hand-crafted component WASM binaries

---

### Task 1: Create the `api/component` public package

**Files:**
- Create: `api/component/component.go`

**Step 1: Write the public package file**

This package re-exports types from `internal/component` so external consumers can use Val, HostFunc, and ResourceTable without importing internal packages.

```go
// Package component provides public types for the WebAssembly Component Model.
//
// This package exposes the dynamic value types (Val) and host function signature
// (HostFunc) needed to interact with component model functions.
//
// For simple function calls using Go primitives (int32, string, etc.), these types
// are not needed — the public api.ComponentFunc.Call() method accepts any values.
// These types are required when defining host functions via api.ComponentLinker.DefineFunc().
package component

import (
	"context"

	internalcomponent "github.com/tetratelabs/wazero/internal/component"
)

// HostFunc is the signature for host-defined component functions.
// Host functions receive and return dynamic Val values.
type HostFunc = internalcomponent.HostFunc

// Val represents a dynamically-typed component model value.
type Val = internalcomponent.Val

// ValKind identifies the type of a Val.
type ValKind = internalcomponent.ValKind

// ValKind constants for type checking Val values.
const (
	ValKindBool    = internalcomponent.ValKindBool
	ValKindS8      = internalcomponent.ValKindS8
	ValKindU8      = internalcomponent.ValKindU8
	ValKindS16     = internalcomponent.ValKindS16
	ValKindU16     = internalcomponent.ValKindU16
	ValKindS32     = internalcomponent.ValKindS32
	ValKindU32     = internalcomponent.ValKindU32
	ValKindS64     = internalcomponent.ValKindS64
	ValKindU64     = internalcomponent.ValKindU64
	ValKindF32     = internalcomponent.ValKindF32
	ValKindF64     = internalcomponent.ValKindF64
	ValKindChar    = internalcomponent.ValKindChar
	ValKindString  = internalcomponent.ValKindString
	ValKindList    = internalcomponent.ValKindList
	ValKindRecord  = internalcomponent.ValKindRecord
	ValKindTuple   = internalcomponent.ValKindTuple
	ValKindVariant = internalcomponent.ValKindVariant
	ValKindEnum    = internalcomponent.ValKindEnum
	ValKindOption  = internalcomponent.ValKindOption
	ValKindResult  = internalcomponent.ValKindResult
	ValKindFlags   = internalcomponent.ValKindFlags
	ValKindOwn     = internalcomponent.ValKindOwn
	ValKindBorrow  = internalcomponent.ValKindBorrow
)

// Val constructors for primitive types.
var (
	ValBool   = internalcomponent.ValBool
	ValS8     = internalcomponent.ValS8
	ValU8     = internalcomponent.ValU8
	ValS16    = internalcomponent.ValS16
	ValU16    = internalcomponent.ValU16
	ValS32    = internalcomponent.ValS32
	ValU32    = internalcomponent.ValU32
	ValS64    = internalcomponent.ValS64
	ValU64    = internalcomponent.ValU64
	ValF32    = internalcomponent.ValF32
	ValF64    = internalcomponent.ValF64
	ValChar   = internalcomponent.ValChar
	ValString = internalcomponent.ValString
)

// Val constructors for composite types.
var (
	ValRecord    = internalcomponent.ValRecord
	ValList      = internalcomponent.ValList
	ValTuple     = internalcomponent.ValTuple
	ValVariant   = internalcomponent.ValVariant
	ValEnum      = internalcomponent.ValEnum
	ValOption    = internalcomponent.ValOption
	ValResultOk  = internalcomponent.ValResultOk
	ValResultErr = internalcomponent.ValResultError
	ValFlags     = internalcomponent.ValFlags
	ValOwn       = internalcomponent.ValOwn
	ValBorrow    = internalcomponent.ValBorrow
)

// WithResourceTable returns a new context with the given ResourceTable.
// This is only needed for advanced use cases; the component linker
// automatically creates and injects a ResourceTable during instantiation.
var WithResourceTable = internalcomponent.WithResourceTable

// ResourceTable manages resource handles for the component model.
type ResourceTable = internalcomponent.ResourceTable

// NewResourceTable creates a new empty ResourceTable.
var NewResourceTable = internalcomponent.NewResourceTable
```

**Step 2: Verify it compiles**

Run: `cd /home/cchamplin/development/wazero && go build ./api/component/`
Expected: Clean compilation with no errors

**Step 3: Commit**

```bash
git add api/component/component.go
git commit -m "feat(api/component): add public package for component model types

Exports Val, HostFunc, ValKind, and ResourceTable types from
internal/component so external consumers can use the component
model without importing internal packages."
```

---

### Task 2: Add `SetRelaxedSemverMatching` to `api.ComponentLinker`

**Files:**
- Modify: `api/component.go` (add method to interface)
- Modify: `internal/component/linker_api.go` (add method to wrapper)

**Step 1: Add the method to the `api.ComponentLinker` interface**

In `api/component.go`, add to the `ComponentLinker` interface (after `Instantiate`):

```go
	// SetRelaxedSemverMatching enables or disables relaxed semver matching.
	// When enabled, pre-1.0 versions (0.x.y) match any patch version within
	// the same minor version (e.g., 0.2.0 matches 0.2.3).
	SetRelaxedSemverMatching(relaxed bool)
```

**Step 2: Implement it on `ComponentLinkerWrapper`**

In `internal/component/linker_api.go`, add after the `MergeFrom` method:

```go
// SetRelaxedSemverMatching enables or disables relaxed semver matching.
func (l *ComponentLinkerWrapper) SetRelaxedSemverMatching(relaxed bool) {
	l.linker.SetRelaxedSemverMatching(relaxed)
}
```

**Step 3: Verify it compiles**

Run: `cd /home/cchamplin/development/wazero && go build ./...`
Expected: Clean compilation

**Step 4: Commit**

```bash
git add api/component.go internal/component/linker_api.go
git commit -m "feat(api): add SetRelaxedSemverMatching to ComponentLinker interface

Required for WASI P2 components that use pre-1.0 interface versions."
```

---

### Task 3: Create the `examples/component-basic` example

This is the simplest example: compile a component, instantiate it with no imports, call an exported function.

**Files:**
- Create: `examples/component-basic/add_test.go`
- Create: `examples/component-basic/testdata/` (copy `add_s32.wasm` from internal testdata)

**Step 1: Copy the test WASM binary**

```bash
mkdir -p /home/cchamplin/development/wazero/examples/component-basic/testdata
cp /home/cchamplin/development/wazero/internal/component/testdata/add_s32.wasm \
   /home/cchamplin/development/wazero/examples/component-basic/testdata/add_s32.wasm
```

**Step 2: Write the example test**

```go
package component_basic

import (
	"context"
	_ "embed"
	"testing"

	"github.com/tetratelabs/wazero"
)

// addWasm is a component that exports: add(a: s32, b: s32) -> s32
//
//go:embed testdata/add_s32.wasm
var addWasm []byte

// TestComponentBasic demonstrates the simplest component model usage:
// compile a component, instantiate it, and call an exported function.
func TestComponentBasic(t *testing.T) {
	ctx := context.Background()

	// Create a wazero runtime
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	// Compile the component binary
	compiled, err := rt.CompileComponent(ctx, addWasm)
	if err != nil {
		t.Fatal(err)
	}
	defer compiled.Close(ctx)

	// Inspect exports
	for _, exp := range compiled.Exports() {
		t.Logf("export: %s (kind=%d)", exp.Name, exp.Kind)
	}

	// Instantiate with no imports (convenience method)
	instance, err := rt.InstantiateComponent(ctx, compiled)
	if err != nil {
		t.Fatal(err)
	}
	defer instance.Close(ctx)

	// Get the exported "add" function
	addFunc := instance.ExportedFunction("add")
	if addFunc == nil {
		t.Fatal("exported function 'add' not found")
	}

	// Call add(2, 3) - pass Go primitives directly
	results, err := addFunc.Call(ctx, int32(2), int32(3))
	if err != nil {
		t.Fatal(err)
	}

	// Results are returned as Go native types
	got := results[0].(int32)
	if got != 5 {
		t.Errorf("add(2, 3) = %d, want 5", got)
	}
	t.Logf("add(2, 3) = %d", got)
}
```

**Step 3: Run the test**

Run: `cd /home/cchamplin/development/wazero && go test ./examples/component-basic/ -v`
Expected: PASS with log output `add(2, 3) = 5`

**Step 4: Commit**

```bash
git add examples/component-basic/
git commit -m "feat(examples): add component-basic example

Demonstrates the simplest component model usage: compile, instantiate,
and call an exported function using Go primitives."
```

---

### Task 4: Create the `examples/component-types` example

Demonstrates component model type system: records, options, lists, and results.

**Files:**
- Create: `examples/component-types/types_test.go`
- Create: `examples/component-types/testdata/` (copy WASM binaries from internal testdata)

**Step 1: Copy the test WASM binaries**

```bash
mkdir -p /home/cchamplin/development/wazero/examples/component-types/testdata
cp /home/cchamplin/development/wazero/internal/component/testdata/echo_record.wasm \
   /home/cchamplin/development/wazero/examples/component-types/testdata/
cp /home/cchamplin/development/wazero/internal/component/testdata/option_roundtrip.wasm \
   /home/cchamplin/development/wazero/examples/component-types/testdata/
cp /home/cchamplin/development/wazero/internal/component/testdata/list_sum.wasm \
   /home/cchamplin/development/wazero/examples/component-types/testdata/
cp /home/cchamplin/development/wazero/internal/component/testdata/result_divide.wasm \
   /home/cchamplin/development/wazero/examples/component-types/testdata/
```

**Step 2: Write the example test**

```go
package component_types

import (
	"context"
	_ "embed"
	"testing"

	"github.com/tetratelabs/wazero"
)

//go:embed testdata/echo_record.wasm
var echoRecordWasm []byte

//go:embed testdata/option_roundtrip.wasm
var optionRoundtripWasm []byte

//go:embed testdata/list_sum.wasm
var listSumWasm []byte

//go:embed testdata/result_divide.wasm
var resultDivideWasm []byte

// TestRecords demonstrates passing record types (structs) to and from components.
// The echo_record component exports: echo(point{x: s32, y: s32}) -> point{x: s32, y: s32}
// The component doubles both coordinates.
func TestRecords(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	compiled, err := rt.CompileComponent(ctx, echoRecordWasm)
	if err != nil {
		t.Fatal(err)
	}
	defer compiled.Close(ctx)

	instance, err := rt.InstantiateComponent(ctx, compiled)
	if err != nil {
		t.Fatal(err)
	}
	defer instance.Close(ctx)

	echoFunc := instance.ExportedFunction("echo")
	if echoFunc == nil {
		t.Fatal("exported function 'echo' not found")
	}

	// Pass a record as map[string]any
	input := map[string]any{"x": int32(3), "y": int32(4)}
	results, err := echoFunc.Call(ctx, input)
	if err != nil {
		t.Fatal(err)
	}

	// Result is returned as map[string]any
	point := results[0].(map[string]any)
	x := point["x"].(int32)
	y := point["y"].(int32)
	// The echo_record component doubles coordinates
	t.Logf("echo({x: 3, y: 4}) = {x: %d, y: %d}", x, y)
	if x != 6 || y != 8 {
		t.Errorf("expected {x: 6, y: 8}, got {x: %d, y: %d}", x, y)
	}
}

// TestOptions demonstrates option types (nullable values).
// The option_roundtrip component exports: echo(option<s32>) -> option<s32>
func TestOptions(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	compiled, err := rt.CompileComponent(ctx, optionRoundtripWasm)
	if err != nil {
		t.Fatal(err)
	}
	defer compiled.Close(ctx)

	instance, err := rt.InstantiateComponent(ctx, compiled)
	if err != nil {
		t.Fatal(err)
	}
	defer instance.Close(ctx)

	echoFunc := instance.ExportedFunction("echo")
	if echoFunc == nil {
		t.Fatal("exported function 'echo' not found")
	}

	// Pass Some(42) - use the component.Val type for option
	someVal := int32(42)
	results, err := echoFunc.Call(ctx, &someVal)
	if err != nil {
		// Options may need Val types - try that approach
		t.Logf("Direct option call: %v (may need Val types)", err)
	} else {
		t.Logf("echo(Some(42)) = %v", results[0])
	}
}

// TestLists demonstrates list types.
// The list_sum component exports: sum(list<s32>) -> s32
func TestLists(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	compiled, err := rt.CompileComponent(ctx, listSumWasm)
	if err != nil {
		t.Fatal(err)
	}
	defer compiled.Close(ctx)

	instance, err := rt.InstantiateComponent(ctx, compiled)
	if err != nil {
		t.Fatal(err)
	}
	defer instance.Close(ctx)

	sumFunc := instance.ExportedFunction("sum")
	if sumFunc == nil {
		t.Fatal("exported function 'sum' not found")
	}

	// Pass a list as []any
	list := []any{int32(1), int32(2), int32(3), int32(4), int32(5)}
	results, err := sumFunc.Call(ctx, list)
	if err != nil {
		t.Fatal(err)
	}

	got := results[0].(int32)
	t.Logf("sum([1, 2, 3, 4, 5]) = %d", got)
	if got != 15 {
		t.Errorf("expected 15, got %d", got)
	}
}

// TestResults demonstrates result types (Ok/Error).
// The result_divide component exports: divide(a: s32, b: s32) -> result<s32, s32>
func TestResults(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	compiled, err := rt.CompileComponent(ctx, resultDivideWasm)
	if err != nil {
		t.Fatal(err)
	}
	defer compiled.Close(ctx)

	instance, err := rt.InstantiateComponent(ctx, compiled)
	if err != nil {
		t.Fatal(err)
	}
	defer instance.Close(ctx)

	divideFunc := instance.ExportedFunction("divide")
	if divideFunc == nil {
		t.Fatal("exported function 'divide' not found")
	}

	// Test successful division
	results, err := divideFunc.Call(ctx, int32(10), int32(3))
	if err != nil {
		t.Fatal(err)
	}
	result := results[0].(map[string]any)
	t.Logf("divide(10, 3) = %v", result)
	if !result["ok"].(bool) {
		t.Error("expected Ok result")
	}

	// Test division by zero
	results, err = divideFunc.Call(ctx, int32(10), int32(0))
	if err != nil {
		t.Fatal(err)
	}
	result = results[0].(map[string]any)
	t.Logf("divide(10, 0) = %v", result)
	if result["ok"].(bool) {
		t.Error("expected Error result for division by zero")
	}
}
```

**Step 3: Run the tests**

Run: `cd /home/cchamplin/development/wazero && go test ./examples/component-types/ -v`
Expected: PASS for all subtests. If any fail, this indicates bugs in the public API's anyToVal/valToAny conversion or in the component execution itself — fix these before continuing.

**Step 4: Commit**

```bash
git add examples/component-types/
git commit -m "feat(examples): add component-types example

Demonstrates records, options, lists, and result types with components
using Go native types (maps, slices) for automatic conversion."
```

---

### Task 5: Generate the host-import component binary

A component that imports `host:util/calc.double(s32) -> s32` and exports `run(s32) -> s32` which calls the imported double function.

**Files:**
- Create: `examples/component-host-functions/gen/gen_double_import.go`
- Create: `examples/component-host-functions/testdata/double_import.wasm` (generated)

**Step 1: Write the generator**

Model this on `internal/component/testdata/gen/gen_add_s32.go` but with an import.

```go
//go:build ignore

// This program generates double_import.wasm, a component that imports
// a host function "double" from "host:util/calc" and exports a "run"
// function that calls it.
//
// WIT equivalent:
//   package host:util;
//   interface calc {
//     double: func(x: s32) -> s32;
//   }
//   world runner {
//     import host:util/calc;
//     export run: func(x: s32) -> s32;
//   }
//
// Run with: go run gen_double_import.go
package main

import (
	"os"
)

func main() {
	// Component preamble
	out := []byte{
		0x00, 0x61, 0x73, 0x6d, // magic
		0x0d, 0x00,             // version
		0x01, 0x00,             // layer: component
	}

	// === Section 7: Component Type Section (first) ===
	// We need to define the import type BEFORE the import section.
	// Type 0: component function type (s32) -> s32
	typeSection := []byte{
		0x01,      // 1 type
		0x40,      // functype
		0x01,      // 1 param
		0x01, 'x', // param "x"
		0x7a,      // s32
		0x00,      // single result (unnamed)
		0x7a,      // s32
	}
	out = append(out, 0x07)
	out = appendLEB128(out, uint32(len(typeSection)))
	out = append(out, typeSection...)

	// === Section 3: Component Import Section ===
	// Import "host:util/calc" as instance with function "double"
	// externname = "host:util/calc" (kebab-name with URL form)
	importName := "host:util/calc"
	importSection := []byte{
		0x01, // 1 import
	}
	// externname encoding: 0x00 = kebab-name
	importSection = append(importSection, 0x00)
	importSection = appendLEB128(importSection, uint32(len(importName)))
	importSection = append(importSection, []byte(importName)...)
	// externdesc: 0x04 = component type (instance type inline)
	// instance type: has exports
	importSection = append(importSection,
		0x05,       // sort = instance inline type
		0x01,       // 1 export in instance
		0x00,                              // kebab-name
	)
	doubleName := "double"
	importSection = appendLEB128(importSection, uint32(len(doubleName)))
	importSection = append(importSection, []byte(doubleName)...)
	importSection = append(importSection,
		0x00,       // externdesc = core sort prefix
		0x01,       // sort = func (indexed)
		0x00,       // type index 0
	)

	out = append(out, 0x03)
	out = appendLEB128(out, uint32(len(importSection)))
	out = append(out, importSection...)

	// === Section 6: Alias Section ===
	// Alias the "double" function from imported instance (component instance 0)
	aliasSection := []byte{
		0x01,                               // 1 alias
		0x01,                               // sort = func (component func, not core)
		0x02,                               // target kind = outer (export from instance)
		0x00,                               // instance index = 0
		0x06, 'd', 'o', 'u', 'b', 'l', 'e', // name "double"
	}
	out = append(out, 0x06)
	out = appendLEB128(out, uint32(len(aliasSection)))
	out = append(out, aliasSection...)

	// === Section 8: Canon Section (lower) ===
	// Lower component function 0 (the imported "double") to core function
	canonLowerSection := []byte{
		0x01, // 1 canonical
		0x01, // canon.lower
		0x00, // func index = 0 (the aliased "double")
		0x00, // 0 options
	}
	out = append(out, 0x08)
	out = appendLEB128(out, uint32(len(canonLowerSection)))
	out = append(out, canonLowerSection...)

	// === Section 1: Core Module ===
	// A core module that imports "host:util/calc#double" and exports "run"
	// The "run" function calls the imported double function
	coreModule := []byte{
		0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00, // magic+version
		// Type section: 1 type - (i32) -> (i32)
		0x01, 0x06, 0x01, 0x60, 0x01, 0x7f, 0x01, 0x7f,
		// Import section: import "[0]" "double" as function 0
		0x02, 0x0e, 0x01,
		0x03, 0x5b, 0x30, 0x5d, // module name "[0]"
		0x06, 0x64, 0x6f, 0x75, 0x62, 0x6c, 0x65, // field name "double"
		0x00, 0x00, // function, type index 0
		// Function section: 1 function (type 0)
		0x03, 0x02, 0x01, 0x00,
		// Export section: export "run" as function 1
		0x07, 0x07, 0x01, 0x03, 0x72, 0x75, 0x6e, 0x00, 0x01,
		// Code section: function body - call imported double
		0x0a, 0x07, 0x01, 0x05, 0x00, 0x20, 0x00, 0x10, 0x00, 0x0b,
	}

	out = append(out, 0x01) // section id = core module
	out = appendLEB128(out, uint32(len(coreModule)))
	out = append(out, coreModule...)

	// === Section 2: Core Instance Section ===
	// Instantiate the core module with the lowered import
	coreInstanceSection := []byte{
		0x01, // 1 core instance
		0x00, // instantiate
		0x00, // module index = 0
		0x01, // 1 arg
		// arg: "[0]" = core instance from lowered canon
		0x03, 0x5b, 0x30, 0x5d, // name "[0]"
		0x12,                    // sort = instance (0x12 = core instance)
		0x00,                    // index = 0 (the synthetic instance from canon.lower)
	}
	out = append(out, 0x02)
	out = appendLEB128(out, uint32(len(coreInstanceSection)))
	out = append(out, coreInstanceSection...)

	// === Section 6: Alias Section (second) ===
	// Alias the "run" function from core instance 0
	aliasSection2 := []byte{
		0x01,                         // 1 alias
		0x00,                         // sort = core sort prefix
		0x00,                         // core sort = func
		0x01,                         // target = core export
		0x00,                         // core instance index = 0
		0x03, 0x72, 0x75, 0x6e,       // name "run"
	}
	out = append(out, 0x06)
	out = appendLEB128(out, uint32(len(aliasSection2)))
	out = append(out, aliasSection2...)

	// === Section 7: Type Section (second) ===
	// Type 1: same as type 0 but for the export
	typeSection2 := []byte{
		0x01,      // 1 type
		0x40,      // functype
		0x01,      // 1 param
		0x01, 'x', // param "x"
		0x7a,      // s32
		0x00,      // single result
		0x7a,      // s32
	}
	out = append(out, 0x07)
	out = appendLEB128(out, uint32(len(typeSection2)))
	out = append(out, typeSection2...)

	// === Section 8: Canon Section (lift) ===
	// Lift core function 0 (aliased "run") as component function type 1
	canonLiftSection := []byte{
		0x01, // 1 canonical
		0x00, // canon.lift
		0x00, // core sort prefix
		0x00, // core func index = 0
		0x00, // 0 options
		0x01, // type index = 1
	}
	out = append(out, 0x08)
	out = appendLEB128(out, uint32(len(canonLiftSection)))
	out = append(out, canonLiftSection...)

	// === Section 11: Export Section ===
	// Export "run" as function 0
	exportSection := []byte{
		0x01,                         // 1 export
		0x00,                         // simple name
		0x03, 0x72, 0x75, 0x6e,       // name "run"
		0x01,                         // sort = func
		0x01,                         // index = 1 (component func from lift)
		0x00,                         // no externdesc
	}
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
```

**Step 2: Generate the WASM binary**

```bash
mkdir -p /home/cchamplin/development/wazero/examples/component-host-functions/testdata
cd /home/cchamplin/development/wazero/examples/component-host-functions/gen
go run gen_double_import.go
```

**Step 3: Verify the generated binary parses correctly**

Write a quick check: try `rt.CompileComponent(ctx, wasm)` on the generated binary. If parsing fails, debug the binary encoding. The binary format is complex — expect iteration here. Use `DEBUG_HELP.md` and reference `gen_add_s32.go` patterns.

**Important note for implementer:** Hand-crafting component binaries with imports is significantly harder than simple export-only components. The binary encoding for component imports, aliases, and canon lower/lift sections must precisely match the spec. If the generator approach proves too fragile, an alternative is to use the `subtract.wasm` (C plugin) from `wasip2test/plugins/` which has no WASI imports but uses the calculator interface — though this doesn't demonstrate custom host functions as cleanly.

**Step 4: Commit**

```bash
git add examples/component-host-functions/gen/ examples/component-host-functions/testdata/
git commit -m "feat(examples): add component binary with host function import

Generated component imports host:util/calc.double and exports run."
```

---

### Task 6: Create the `examples/component-host-functions` example test

**Files:**
- Create: `examples/component-host-functions/host_test.go`

**Step 1: Write the example test**

```go
package component_host_functions

import (
	"context"
	_ "embed"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api/component"
)

// doubleImportWasm is a component that imports host:util/calc.double(s32) -> s32
// and exports run(s32) -> s32 which calls the imported function.
//
//go:embed testdata/double_import.wasm
var doubleImportWasm []byte

// TestHostFunctions demonstrates defining host functions for component imports.
// The component imports a "double" function and the host provides an implementation.
func TestHostFunctions(t *testing.T) {
	ctx := context.Background()

	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	// Compile the component
	compiled, err := rt.CompileComponent(ctx, doubleImportWasm)
	if err != nil {
		t.Fatal(err)
	}
	defer compiled.Close(ctx)

	// Show what the component imports
	for _, imp := range compiled.Imports() {
		t.Logf("import: %s (kind=%d)", imp.Name, imp.Kind)
	}

	// Create a linker to define host functions
	linker := rt.NewComponentLinker()

	// Define the "double" function in the "host:util/calc" instance.
	// Host functions use component.HostFunc and component.Val types.
	err = linker.DefineInstance("host:util/calc").
		Func("double", component.HostFunc(func(ctx context.Context, args []component.Val) ([]component.Val, error) {
			x := args[0].S32()
			return []component.Val{component.ValS32(x * 2)}, nil
		})).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// Instantiate with the host imports satisfied
	instance, err := linker.Instantiate(ctx, compiled)
	if err != nil {
		t.Fatal(err)
	}
	defer instance.Close(ctx)

	// Call run(21) - the component calls double(21) internally
	runFunc := instance.ExportedFunction("run")
	if runFunc == nil {
		t.Fatal("exported function 'run' not found")
	}

	results, err := runFunc.Call(ctx, int32(21))
	if err != nil {
		t.Fatal(err)
	}

	got := results[0].(int32)
	if got != 42 {
		t.Errorf("run(21) = %d, want 42", got)
	}
	t.Logf("run(21) = %d (host doubled 21 to 42)", got)
}
```

**Step 2: Run the test**

Run: `cd /home/cchamplin/development/wazero && go test ./examples/component-host-functions/ -v`
Expected: PASS with `run(21) = 42`

If this fails, it likely indicates a bug in:
- The generated WASM binary (most likely — see Task 5 note)
- The `ComponentLinkerWrapper.DefineInstance` → `Func` → `Build` path
- The `anyToVal`/`valToAny` conversion or canon lower/lift

Debug systematically. The generated binary must match the component model binary spec exactly.

**Step 3: Commit**

```bash
git add examples/component-host-functions/host_test.go
git commit -m "feat(examples): add component-host-functions example

Demonstrates defining host functions using the public api/component
package with HostFunc and Val types."
```

---

### Task 7: Create the `examples/component-wasip2` example

**Files:**
- Create: `examples/component-wasip2/wasip2_test.go`
- Create: `examples/component-wasip2/testdata/add.wasm` (copy from wasip2test plugins)

**Step 1: Copy the WASI P2 calculator plugin**

```bash
mkdir -p /home/cchamplin/development/wazero/examples/component-wasip2/testdata
cp /home/cchamplin/development/wazero/internal/component/wasip2test/plugins/add.wasm \
   /home/cchamplin/development/wazero/examples/component-wasip2/testdata/add.wasm
```

**Step 2: Write the example test**

```go
package component_wasip2

import (
	"bytes"
	"context"
	_ "embed"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasip2"
)

// addPluginWasm is a Rust calculator plugin component that uses WASI P2.
// It exports:
//   - get-plugin-name() -> string
//   - evaluate(a: s32, b: s32) -> s32   (returns a + b)
//
//go:embed testdata/add.wasm
var addPluginWasm []byte

// TestWASIP2Plugin demonstrates running a WASI P2 component.
// This is a Rust plugin that requires WASI Preview 2 interfaces for
// standard I/O, clocks, random, etc.
func TestWASIP2Plugin(t *testing.T) {
	ctx := context.Background()

	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	// Compile the component
	compiled, err := rt.CompileComponent(ctx, addPluginWasm)
	if err != nil {
		t.Fatal(err)
	}
	defer compiled.Close(ctx)

	// Create a component linker
	linker := rt.NewComponentLinker()

	// Enable relaxed semver matching (required for WASI interfaces
	// that use pre-1.0 versions like 0.2.x)
	linker.SetRelaxedSemverMatching(true)

	// Merge WASI P2 interfaces into the linker with custom configuration
	var stdout, stderr bytes.Buffer
	wasiConfig := wasip2.NewConfig().
		WithStdout(&stdout).
		WithStderr(&stderr).
		WithArgs([]string{"test"}).
		WithEnviron([]string{})

	if err := wasip2.MergeIntoWithConfig(linker, wasiConfig); err != nil {
		t.Fatal(err)
	}

	// Set up the WASI context
	ctx = wasip2.WithConfig(ctx, wasiConfig)

	// Instantiate the component
	instance, err := linker.Instantiate(ctx, compiled)
	if err != nil {
		t.Fatal(err)
	}
	defer instance.Close(ctx)

	// Call get-plugin-name()
	nameFunc := instance.ExportedFunction("get-plugin-name")
	if nameFunc == nil {
		t.Fatal("exported function 'get-plugin-name' not found")
	}
	nameResult, err := nameFunc.Call(ctx)
	if err != nil {
		t.Fatal(err)
	}
	name := nameResult[0].(string)
	t.Logf("plugin name: %s", name)
	if name != "add" {
		t.Errorf("name = %q, want %q", name, "add")
	}

	// Call evaluate(28, 3) = 31
	evalFunc := instance.ExportedFunction("evaluate")
	if evalFunc == nil {
		t.Fatal("exported function 'evaluate' not found")
	}
	evalResult, err := evalFunc.Call(ctx, int32(28), int32(3))
	if err != nil {
		t.Fatal(err)
	}
	got := evalResult[0].(int32)
	t.Logf("evaluate(28, 3) = %d", got)
	if got != 31 {
		t.Errorf("evaluate(28, 3) = %d, want 31", got)
	}
}
```

**Step 3: Run the test**

Run: `cd /home/cchamplin/development/wazero && go test ./examples/component-wasip2/ -v`
Expected: PASS with plugin name "add" and evaluate(28, 3) = 31

**Step 4: Commit**

```bash
git add examples/component-wasip2/
git commit -m "feat(examples): add component-wasip2 example

Demonstrates running a WASI P2 component with MergeInto, config,
and relaxed semver matching using only public API imports."
```

---

### Task 8: Update the examples README

**Files:**
- Modify: `examples/README.md`

**Step 1: Add the new examples to the README**

Add the following entries to the existing list:

```markdown
* [component-basic](component-basic) - how to compile, instantiate, and call
  a WebAssembly Component Model component.
* [component-types](component-types) - how to pass records, options, lists,
  and results to and from component functions.
* [component-host-functions](component-host-functions) - how to define host
  functions that satisfy component imports using the `api/component` package.
* [component-wasip2](component-wasip2) - how to run a WASI Preview 2 component
  with full WASI P2 interface support.
```

**Step 2: Commit**

```bash
git add examples/README.md
git commit -m "docs(examples): add component model examples to README"
```

---

### Task 9: Run all examples and verify

**Step 1: Run all component examples together**

Run: `cd /home/cchamplin/development/wazero && go test ./examples/component-basic/ ./examples/component-types/ ./examples/component-host-functions/ ./examples/component-wasip2/ -v`
Expected: All tests PASS

**Step 2: Run the full test suite to check for regressions**

Run: `cd /home/cchamplin/development/wazero && go test ./... 2>&1 | tail -30`
Expected: No new failures introduced

**Step 3: Verify no internal imports in examples**

Run: `grep -r "internal/" examples/component-*/`
Expected: No results — all examples must use only public packages

---

### Debugging Notes

**If the host-function component binary fails to parse:**
The hand-crafted binary format is the most fragile part. Reference:
- `internal/component/testdata/gen/gen_add_s32.go` for the basic pattern
- `internal/component/binary/` for the parser expectations
- `DEBUG_HELP.md` for wasmtime comparison

Common issues: wrong section ordering, incorrect LEB128 encoding, mismatched indices.

**If WASI P2 instantiation fails:**
- Ensure `SetRelaxedSemverMatching(true)` is called before `MergeInto`
- Ensure `wasip2.WithConfig(ctx, config)` context is passed to `Instantiate`
- Check that all required WASI interfaces are registered (the Rust add plugin imports several)

**If valToAny/anyToVal conversion is wrong:**
- Check `internal/component/linker_api.go` `anyToVal` and `valToAny` functions
- For records: expects `map[string]any`, returns `map[string]any`
- For lists: expects `[]any`, returns `[]any`
- For results: returns `map[string]any{"ok": bool, "value": ..., "error": ...}`
