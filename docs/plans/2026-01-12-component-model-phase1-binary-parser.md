# Component Model Phase 1: Binary Parser & Primitives

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Parent Plan:** [2026-01-12-component-model-implementation.md](./2026-01-12-component-model-implementation.md)  
**Design Doc:** [2026-01-12-component-model-design.md](./2026-01-12-component-model-design.md)  
**Status:** IN PROGRESS (Tasks 1-25 completed through commit 00c3ae05)  
**Next Task:** Phase 1 complete - proceed to Phase 2

---

## Overview

This phase establishes the foundation: detecting component binaries, parsing sections, and executing a simple `add(s32, s32) -> s32` function.

**Goal:** Parse component binaries and call a simple s32 add function.

**Architecture:** Parallel `internal/component/` package structure with single-pass streaming binary parser, hybrid lift/lower (dynamic Val + interfaces), generation-counted resource handles, and engine-agnostic component orchestration layer.

**Tech Stack:** Go (zero external dependencies), wazero core wasm runtime, cargo-component/wasm-tools for test fixture generation.

---

## Completed Implementation Summary

Based on recent commits, the following has been implemented:
- Component package structure (`internal/component/`)
- Binary format parser with preamble validation
- Section header parsing and core module extraction
- Primitive value types and dynamic Val type
- Type section parsing (function types)
- Canonical section parsing (lift/lower)
- Export section parsing
- Basic component instantiation
- Working `add(s32, s32) -> s32` test

---

## Phase 1: Binary Parser & Primitives

This phase establishes the foundation: detecting component binaries, parsing sections, and executing a simple `add(s32, s32) -> s32` function.

---

### Task 1: Create Component Package Structure

**Files:**
- Create: `internal/component/component.go`
- Create: `internal/component/doc.go`

**Step 1: Create the component package with doc.go**

```go
// internal/component/doc.go

// Package component implements the WebAssembly Component Model.
//
// The Component Model extends core WebAssembly with:
//   - Rich types (records, variants, lists, options, results, resources)
//   - Interface-based composition via WIT (WebAssembly Interface Types)
//   - Canonical ABI for cross-component communication
//   - WASI Preview 2 interfaces
//
// See https://component-model.bytecodealliance.org/
package component
```

**Step 2: Create minimal component.go placeholder**

```go
// internal/component/component.go

package component

// Component represents a parsed WebAssembly component.
// Unlike core wasm modules, components can contain nested modules
// and components, and use rich interface types.
type Component struct {
	// Will be populated as we implement parsing
}
```

**Step 3: Verify package compiles**

Run: `go build ./internal/component/...`
Expected: No output (success)

**Step 4: Commit**

```bash
git add internal/component/
git commit -m "feat(component): create component package structure"
```

---

### Task 2: Define Component Binary Constants

**Files:**
- Create: `internal/component/binary/binary.go`

**Step 1: Write the failing test for magic/version detection**

```go
// internal/component/binary/binary_test.go

package binary

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestComponentMagic(t *testing.T) {
	require.Equal(t, []byte{0x00, 0x61, 0x73, 0x6d}, Magic[:])
}

func TestComponentVersion(t *testing.T) {
	// Pre-standard component version
	require.Equal(t, []byte{0x0d, 0x00}, Version[:])
}

func TestLayerComponent(t *testing.T) {
	require.Equal(t, []byte{0x01, 0x00}, LayerComponent[:])
}

func TestLayerModule(t *testing.T) {
	require.Equal(t, []byte{0x00, 0x00}, LayerModule[:])
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/binary/... -v`
Expected: FAIL - package does not exist

**Step 3: Write the implementation**

```go
// internal/component/binary/binary.go

package binary

// Magic is the 4-byte preamble for all WebAssembly binaries (modules and components).
// See https://github.com/WebAssembly/component-model/blob/main/design/mvp/Binary.md
var Magic = [4]byte{0x00, 0x61, 0x73, 0x6d} // "\0asm"

// Version is the component model version (pre-standard).
// This will change to [4]byte{0x01, 0x00, 0x01, 0x00} at 1.0.
var Version = [2]byte{0x0d, 0x00}

// LayerComponent identifies a binary as a component (vs core module).
var LayerComponent = [2]byte{0x01, 0x00}

// LayerModule identifies a binary as a core module (vs component).
var LayerModule = [2]byte{0x00, 0x00}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/binary/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/binary/
git commit -m "feat(component): add component binary magic and version constants"
```

---

### Task 3: Define Component Section IDs

**Files:**
- Modify: `internal/component/binary/binary.go`
- Modify: `internal/component/binary/binary_test.go`

**Step 1: Write the failing test for section IDs**

```go
// Add to internal/component/binary/binary_test.go

func TestSectionIDs(t *testing.T) {
	// Verify section IDs match the Component Model spec
	// https://github.com/WebAssembly/component-model/blob/main/design/mvp/Binary.md
	tests := []struct {
		name     string
		id       SectionID
		expected byte
	}{
		{"CoreCustom", SectionIDCoreCustom, 0},
		{"CoreModule", SectionIDCoreModule, 1},
		{"CoreInstance", SectionIDCoreInstance, 2},
		{"CoreType", SectionIDCoreType, 3},
		{"Component", SectionIDComponent, 4},
		{"Instance", SectionIDInstance, 5},
		{"Alias", SectionIDAlias, 6},
		{"Type", SectionIDType, 7},
		{"Canon", SectionIDCanon, 8},
		{"Start", SectionIDStart, 9},
		{"Import", SectionIDImport, 10},
		{"Export", SectionIDExport, 11},
		{"Value", SectionIDValue, 12},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, byte(tc.id))
		})
	}
}

func TestSectionIDName(t *testing.T) {
	require.Equal(t, "core-module", SectionIDCoreModule.String())
	require.Equal(t, "type", SectionIDType.String())
	require.Equal(t, "unknown(255)", SectionID(255).String())
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/binary/... -v -run TestSectionID`
Expected: FAIL - SectionID undefined

**Step 3: Write the implementation**

```go
// Add to internal/component/binary/binary.go

import "fmt"

// SectionID identifies a component section.
// Component sections have different IDs than core wasm sections.
type SectionID byte

const (
	// SectionIDCoreCustom is for custom sections within components.
	SectionIDCoreCustom SectionID = 0

	// SectionIDCoreModule contains an embedded core wasm module.
	SectionIDCoreModule SectionID = 1

	// SectionIDCoreInstance instantiates a core module.
	SectionIDCoreInstance SectionID = 2

	// SectionIDCoreType defines core types (for use in aliases).
	SectionIDCoreType SectionID = 3

	// SectionIDComponent contains a nested component.
	SectionIDComponent SectionID = 4

	// SectionIDInstance instantiates a component.
	SectionIDInstance SectionID = 5

	// SectionIDAlias creates aliases to items in other scopes.
	SectionIDAlias SectionID = 6

	// SectionIDType defines component types (functions, resources, etc).
	SectionIDType SectionID = 7

	// SectionIDCanon defines canonical functions (lift/lower).
	SectionIDCanon SectionID = 8

	// SectionIDStart specifies the component start function.
	SectionIDStart SectionID = 9

	// SectionIDImport declares component imports.
	SectionIDImport SectionID = 10

	// SectionIDExport declares component exports.
	SectionIDExport SectionID = 11

	// SectionIDValue defines component values (gated feature).
	SectionIDValue SectionID = 12
)

// String returns a human-readable section name.
func (s SectionID) String() string {
	switch s {
	case SectionIDCoreCustom:
		return "core-custom"
	case SectionIDCoreModule:
		return "core-module"
	case SectionIDCoreInstance:
		return "core-instance"
	case SectionIDCoreType:
		return "core-type"
	case SectionIDComponent:
		return "component"
	case SectionIDInstance:
		return "instance"
	case SectionIDAlias:
		return "alias"
	case SectionIDType:
		return "type"
	case SectionIDCanon:
		return "canon"
	case SectionIDStart:
		return "start"
	case SectionIDImport:
		return "import"
	case SectionIDExport:
		return "export"
	case SectionIDValue:
		return "value"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/binary/... -v -run TestSectionID`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/binary/
git commit -m "feat(component): add component section ID constants"
```

---

### Task 4: Implement Binary Detection (IsComponent)

**Files:**
- Modify: `internal/component/binary/binary.go`
- Modify: `internal/component/binary/binary_test.go`

**Step 1: Write the failing test**

```go
// Add to internal/component/binary/binary_test.go

func TestIsComponent(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected bool
	}{
		{
			name: "valid component",
			input: append(append(Magic[:], Version[:]...), LayerComponent[:]...),
			expected: true,
		},
		{
			name: "core module",
			input: append(append(Magic[:], 0x01, 0x00, 0x00, 0x00), LayerModule[:]...),
			expected: false,
		},
		{
			name: "too short",
			input: Magic[:],
			expected: false,
		},
		{
			name: "wrong magic",
			input: []byte{0x00, 0x00, 0x00, 0x00, 0x0d, 0x00, 0x01, 0x00},
			expected: false,
		},
		{
			name: "empty",
			input: []byte{},
			expected: false,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			result := IsComponent(tc.input)
			require.Equal(t, tc.expected, result)
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/binary/... -v -run TestIsComponent`
Expected: FAIL - IsComponent undefined

**Step 3: Write the implementation**

```go
// Add to internal/component/binary/binary.go

import "bytes"

// IsComponent returns true if the binary appears to be a component
// (as opposed to a core wasm module).
//
// This checks:
//   - Magic number matches "\0asm"
//   - Version is component version (0x0d 0x00)
//   - Layer byte is component layer (0x01 0x00)
func IsComponent(binary []byte) bool {
	// Need at least: magic(4) + version(2) + layer(2) = 8 bytes
	if len(binary) < 8 {
		return false
	}

	// Check magic
	if !bytes.Equal(binary[0:4], Magic[:]) {
		return false
	}

	// Check version (component model pre-standard version)
	if !bytes.Equal(binary[4:6], Version[:]) {
		return false
	}

	// Check layer (component vs module)
	if !bytes.Equal(binary[6:8], LayerComponent[:]) {
		return false
	}

	return true
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/binary/... -v -run TestIsComponent`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/binary/
git commit -m "feat(component): add IsComponent binary detection"
```

---

### Task 5: Define Decoder Errors

**Files:**
- Create: `internal/component/binary/errors.go`
- Create: `internal/component/binary/errors_test.go`

**Step 1: Write the failing test**

```go
// internal/component/binary/errors_test.go

package binary

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestErrors(t *testing.T) {
	require.EqualError(t, ErrInvalidMagic, "invalid component: bad magic number")
	require.EqualError(t, ErrInvalidVersion, "invalid component: bad version")
	require.EqualError(t, ErrInvalidLayer, "invalid component: not a component (core module?)")
	require.EqualError(t, ErrUnexpectedEOF, "invalid component: unexpected end of file")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/binary/... -v -run TestErrors`
Expected: FAIL - ErrInvalidMagic undefined

**Step 3: Write the implementation**

```go
// internal/component/binary/errors.go

package binary

import "errors"

// Decoder errors
var (
	// ErrInvalidMagic is returned when the magic number is not "\0asm".
	ErrInvalidMagic = errors.New("invalid component: bad magic number")

	// ErrInvalidVersion is returned when the version is not recognized.
	ErrInvalidVersion = errors.New("invalid component: bad version")

	// ErrInvalidLayer is returned when the layer byte indicates a core module.
	ErrInvalidLayer = errors.New("invalid component: not a component (core module?)")

	// ErrUnexpectedEOF is returned when the binary ends unexpectedly.
	ErrUnexpectedEOF = errors.New("invalid component: unexpected end of file")
)
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/binary/... -v -run TestErrors`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/binary/
git commit -m "feat(component): add decoder error definitions"
```

---

### Task 6: Implement Basic Decoder Preamble

**Files:**
- Create: `internal/component/binary/decoder.go`
- Create: `internal/component/binary/decoder_test.go`

**Step 1: Write the failing test for preamble decoding**

```go
// internal/component/binary/decoder_test.go

package binary

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestDecodeComponent_Preamble(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		expectedErr error
	}{
		{
			name:        "empty",
			input:       []byte{},
			expectedErr: ErrInvalidMagic,
		},
		{
			name:        "magic only",
			input:       Magic[:],
			expectedErr: ErrUnexpectedEOF,
		},
		{
			name:        "wrong magic",
			input:       []byte{0x00, 0x00, 0x00, 0x00, 0x0d, 0x00, 0x01, 0x00},
			expectedErr: ErrInvalidMagic,
		},
		{
			name:        "wrong version",
			input:       append(Magic[:], 0x01, 0x00, 0x00, 0x00, 0x01, 0x00),
			expectedErr: ErrInvalidVersion,
		},
		{
			name:        "core module layer",
			input:       append(append(Magic[:], Version[:]...), LayerModule[:]...),
			expectedErr: ErrInvalidLayer,
		},
		{
			name:        "valid empty component",
			input:       append(append(Magic[:], Version[:]...), LayerComponent[:]...),
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeComponent(tc.input)
			if tc.expectedErr != nil {
				require.Equal(t, tc.expectedErr, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDecodeComponent_EmptyComponent(t *testing.T) {
	input := append(append(Magic[:], Version[:]...), LayerComponent[:]...)

	c, err := DecodeComponent(input)
	require.NoError(t, err)
	require.NotNil(t, c)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/binary/... -v -run TestDecodeComponent`
Expected: FAIL - DecodeComponent undefined

**Step 3: Write the implementation**

```go
// internal/component/binary/decoder.go

package binary

import (
	"bytes"
	"io"

	"github.com/tetratelabs/wazero/internal/component"
)

// DecodeComponent parses a WebAssembly component from binary format.
func DecodeComponent(binary []byte) (*component.Component, error) {
	r := bytes.NewReader(binary)

	// Read and validate magic number
	magic := make([]byte, 4)
	if _, err := io.ReadFull(r, magic); err != nil {
		return nil, ErrInvalidMagic
	}
	if !bytes.Equal(magic, Magic[:]) {
		return nil, ErrInvalidMagic
	}

	// Read and validate version
	version := make([]byte, 2)
	if _, err := io.ReadFull(r, version); err != nil {
		return nil, ErrUnexpectedEOF
	}
	if !bytes.Equal(version, Version[:]) {
		return nil, ErrInvalidVersion
	}

	// Read and validate layer
	layer := make([]byte, 2)
	if _, err := io.ReadFull(r, layer); err != nil {
		return nil, ErrUnexpectedEOF
	}
	if !bytes.Equal(layer, LayerComponent[:]) {
		return nil, ErrInvalidLayer
	}

	// For now, return an empty component
	// Sections will be parsed in subsequent tasks
	return &component.Component{}, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/binary/... -v -run TestDecodeComponent`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/binary/
git commit -m "feat(component): implement decoder preamble parsing"
```

---

### Task 7: Define Primitive Value Types

**Files:**
- Create: `internal/component/types/types.go`
- Create: `internal/component/types/types_test.go`

**Step 1: Write the failing test**

```go
// internal/component/types/types_test.go

package types

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestPrimitiveValType(t *testing.T) {
	tests := []struct {
		name         string
		typ          ValType
		size         uint32
		align        uint32
		flattenCount int
	}{
		{"Bool", Bool{}, 1, 1, 1},
		{"S8", S8{}, 1, 1, 1},
		{"U8", U8{}, 1, 1, 1},
		{"S16", S16{}, 2, 2, 1},
		{"U16", U16{}, 2, 2, 1},
		{"S32", S32{}, 4, 4, 1},
		{"U32", U32{}, 4, 4, 1},
		{"S64", S64{}, 8, 8, 1},
		{"U64", U64{}, 8, 8, 1},
		{"F32", F32{}, 4, 4, 1},
		{"F64", F64{}, 8, 8, 1},
		{"Char", Char{}, 4, 4, 1},
		{"String", String{}, 8, 4, 2}, // ptr + len, align 4
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.size, tc.typ.Size())
			require.Equal(t, tc.align, tc.typ.Align())
			require.Equal(t, tc.flattenCount, tc.typ.FlattenCount())
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/types/... -v`
Expected: FAIL - package does not exist

**Step 3: Write the implementation**

```go
// internal/component/types/types.go

package types

// ValType represents any component model value type.
type ValType interface {
	valType() // marker method

	// Size returns the byte size when stored in linear memory.
	Size() uint32

	// Align returns the alignment requirement in bytes.
	Align() uint32

	// FlattenCount returns the number of core wasm values when flattened.
	// Used to determine if values pass in registers (flat) or memory (heap).
	FlattenCount() int
}

// Primitive types

// Bool represents a boolean value.
type Bool struct{}

func (Bool) valType()         {}
func (Bool) Size() uint32     { return 1 }
func (Bool) Align() uint32    { return 1 }
func (Bool) FlattenCount() int { return 1 }

// S8 represents a signed 8-bit integer.
type S8 struct{}

func (S8) valType()         {}
func (S8) Size() uint32     { return 1 }
func (S8) Align() uint32    { return 1 }
func (S8) FlattenCount() int { return 1 }

// U8 represents an unsigned 8-bit integer.
type U8 struct{}

func (U8) valType()         {}
func (U8) Size() uint32     { return 1 }
func (U8) Align() uint32    { return 1 }
func (U8) FlattenCount() int { return 1 }

// S16 represents a signed 16-bit integer.
type S16 struct{}

func (S16) valType()         {}
func (S16) Size() uint32     { return 2 }
func (S16) Align() uint32    { return 2 }
func (S16) FlattenCount() int { return 1 }

// U16 represents an unsigned 16-bit integer.
type U16 struct{}

func (U16) valType()         {}
func (U16) Size() uint32     { return 2 }
func (U16) Align() uint32    { return 2 }
func (U16) FlattenCount() int { return 1 }

// S32 represents a signed 32-bit integer.
type S32 struct{}

func (S32) valType()         {}
func (S32) Size() uint32     { return 4 }
func (S32) Align() uint32    { return 4 }
func (S32) FlattenCount() int { return 1 }

// U32 represents an unsigned 32-bit integer.
type U32 struct{}

func (U32) valType()         {}
func (U32) Size() uint32     { return 4 }
func (U32) Align() uint32    { return 4 }
func (U32) FlattenCount() int { return 1 }

// S64 represents a signed 64-bit integer.
type S64 struct{}

func (S64) valType()         {}
func (S64) Size() uint32     { return 8 }
func (S64) Align() uint32    { return 8 }
func (S64) FlattenCount() int { return 1 }

// U64 represents an unsigned 64-bit integer.
type U64 struct{}

func (U64) valType()         {}
func (U64) Size() uint32     { return 8 }
func (U64) Align() uint32    { return 8 }
func (U64) FlattenCount() int { return 1 }

// F32 represents a 32-bit floating point number.
type F32 struct{}

func (F32) valType()         {}
func (F32) Size() uint32     { return 4 }
func (F32) Align() uint32    { return 4 }
func (F32) FlattenCount() int { return 1 }

// F64 represents a 64-bit floating point number.
type F64 struct{}

func (F64) valType()         {}
func (F64) Size() uint32     { return 8 }
func (F64) Align() uint32    { return 8 }
func (F64) FlattenCount() int { return 1 }

// Char represents a Unicode scalar value (code point).
type Char struct{}

func (Char) valType()         {}
func (Char) Size() uint32     { return 4 }
func (Char) Align() uint32    { return 4 }
func (Char) FlattenCount() int { return 1 }

// String represents a UTF-8 encoded string.
// In memory: (ptr: i32, len: i32)
type String struct{}

func (String) valType()         {}
func (String) Size() uint32     { return 8 } // ptr + len
func (String) Align() uint32    { return 4 } // aligned to i32
func (String) FlattenCount() int { return 2 } // ptr, len
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/types/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/types/
git commit -m "feat(component): add primitive value types with size/align/flatten"
```

---

### Task 8: Define Dynamic Val Type

**Files:**
- Create: `internal/component/val.go`
- Create: `internal/component/val_test.go`

**Step 1: Write the failing test**

```go
// internal/component/val_test.go

package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestValConstructorsAndAccessors(t *testing.T) {
	t.Run("Bool", func(t *testing.T) {
		v := ValBool(true)
		require.Equal(t, ValKindBool, v.Kind())
		require.True(t, v.Bool())

		v = ValBool(false)
		require.False(t, v.Bool())
	})

	t.Run("S32", func(t *testing.T) {
		v := ValS32(-42)
		require.Equal(t, ValKindS32, v.Kind())
		require.Equal(t, int32(-42), v.S32())
	})

	t.Run("U32", func(t *testing.T) {
		v := ValU32(42)
		require.Equal(t, ValKindU32, v.Kind())
		require.Equal(t, uint32(42), v.U32())
	})

	t.Run("S64", func(t *testing.T) {
		v := ValS64(-123456789)
		require.Equal(t, ValKindS64, v.Kind())
		require.Equal(t, int64(-123456789), v.S64())
	})

	t.Run("U64", func(t *testing.T) {
		v := ValU64(123456789)
		require.Equal(t, ValKindU64, v.Kind())
		require.Equal(t, uint64(123456789), v.U64())
	})

	t.Run("F32", func(t *testing.T) {
		v := ValF32(3.14)
		require.Equal(t, ValKindF32, v.Kind())
		require.Equal(t, float32(3.14), v.F32())
	})

	t.Run("F64", func(t *testing.T) {
		v := ValF64(3.14159265359)
		require.Equal(t, ValKindF64, v.Kind())
		require.Equal(t, float64(3.14159265359), v.F64())
	})

	t.Run("Char", func(t *testing.T) {
		v := ValChar('A')
		require.Equal(t, ValKindChar, v.Kind())
		require.Equal(t, rune('A'), v.Char())
	})

	t.Run("String", func(t *testing.T) {
		v := ValString("hello")
		require.Equal(t, ValKindString, v.Kind())
		require.Equal(t, "hello", v.String())
	})
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -v -run TestValConstructors`
Expected: FAIL - ValBool undefined

**Step 3: Write the implementation**

```go
// internal/component/val.go

package component

// Val represents a dynamically-typed component model value.
// Used when function signatures aren't known at compile time.
type Val struct {
	kind ValKind
	v    any
}

// ValKind identifies the type of a Val.
type ValKind uint8

const (
	ValKindBool ValKind = iota
	ValKindS8
	ValKindU8
	ValKindS16
	ValKindU16
	ValKindS32
	ValKindU32
	ValKindS64
	ValKindU64
	ValKindF32
	ValKindF64
	ValKindChar
	ValKindString
	ValKindList
	ValKindRecord
	ValKindTuple
	ValKindVariant
	ValKindEnum
	ValKindOption
	ValKindResult
	ValKindFlags
	ValKindOwn
	ValKindBorrow
)

// Kind returns the type of this value.
func (v Val) Kind() ValKind { return v.kind }

// Constructors

// ValBool creates a boolean Val.
func ValBool(b bool) Val { return Val{ValKindBool, b} }

// ValS8 creates a signed 8-bit integer Val.
func ValS8(n int8) Val { return Val{ValKindS8, n} }

// ValU8 creates an unsigned 8-bit integer Val.
func ValU8(n uint8) Val { return Val{ValKindU8, n} }

// ValS16 creates a signed 16-bit integer Val.
func ValS16(n int16) Val { return Val{ValKindS16, n} }

// ValU16 creates an unsigned 16-bit integer Val.
func ValU16(n uint16) Val { return Val{ValKindU16, n} }

// ValS32 creates a signed 32-bit integer Val.
func ValS32(n int32) Val { return Val{ValKindS32, n} }

// ValU32 creates an unsigned 32-bit integer Val.
func ValU32(n uint32) Val { return Val{ValKindU32, n} }

// ValS64 creates a signed 64-bit integer Val.
func ValS64(n int64) Val { return Val{ValKindS64, n} }

// ValU64 creates an unsigned 64-bit integer Val.
func ValU64(n uint64) Val { return Val{ValKindU64, n} }

// ValF32 creates a 32-bit floating point Val.
func ValF32(f float32) Val { return Val{ValKindF32, f} }

// ValF64 creates a 64-bit floating point Val.
func ValF64(f float64) Val { return Val{ValKindF64, f} }

// ValChar creates a Unicode character Val.
func ValChar(c rune) Val { return Val{ValKindChar, c} }

// ValString creates a string Val.
func ValString(s string) Val { return Val{ValKindString, s} }

// Accessors

// Bool returns the boolean value. Panics if Kind() != ValKindBool.
func (v Val) Bool() bool { return v.v.(bool) }

// S8 returns the int8 value. Panics if Kind() != ValKindS8.
func (v Val) S8() int8 { return v.v.(int8) }

// U8 returns the uint8 value. Panics if Kind() != ValKindU8.
func (v Val) U8() uint8 { return v.v.(uint8) }

// S16 returns the int16 value. Panics if Kind() != ValKindS16.
func (v Val) S16() int16 { return v.v.(int16) }

// U16 returns the uint16 value. Panics if Kind() != ValKindU16.
func (v Val) U16() uint16 { return v.v.(uint16) }

// S32 returns the int32 value. Panics if Kind() != ValKindS32.
func (v Val) S32() int32 { return v.v.(int32) }

// U32 returns the uint32 value. Panics if Kind() != ValKindU32.
func (v Val) U32() uint32 { return v.v.(uint32) }

// S64 returns the int64 value. Panics if Kind() != ValKindS64.
func (v Val) S64() int64 { return v.v.(int64) }

// U64 returns the uint64 value. Panics if Kind() != ValKindU64.
func (v Val) U64() uint64 { return v.v.(uint64) }

// F32 returns the float32 value. Panics if Kind() != ValKindF32.
func (v Val) F32() float32 { return v.v.(float32) }

// F64 returns the float64 value. Panics if Kind() != ValKindF64.
func (v Val) F64() float64 { return v.v.(float64) }

// Char returns the rune value. Panics if Kind() != ValKindChar.
func (v Val) Char() rune { return v.v.(rune) }

// String returns the string value. Panics if Kind() != ValKindString.
func (v Val) String() string { return v.v.(string) }
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -v -run TestValConstructors`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/
git commit -m "feat(component): add dynamic Val type for primitive values"
```

---

### Task 9: Create Test Fixture - Minimal Component Binary

**Files:**
- Create: `internal/component/testdata/empty.wasm`
- Create: `internal/component/testdata/testdata.go`

**Step 1: Create testdata directory and embed file**

```go
// internal/component/testdata/testdata.go

package testdata

import (
	_ "embed"
)

// EmptyComponent is a minimal valid component with no content.
// Binary: magic(4) + version(2) + layer(2) = 8 bytes
//
//go:embed empty.wasm
var EmptyComponent []byte
```

**Step 2: Create the empty.wasm binary file**

Run the following to create a minimal component:

```bash
mkdir -p internal/component/testdata
printf '\x00\x61\x73\x6d\x0d\x00\x01\x00' > internal/component/testdata/empty.wasm
```

**Step 3: Verify the file exists and is correct**

Run: `xxd internal/component/testdata/empty.wasm`
Expected: `00000000: 0061 736d 0d00 0100`

**Step 4: Verify the testdata package compiles**

Run: `go build ./internal/component/testdata/...`
Expected: No output (success)

**Step 5: Commit**

```bash
git add internal/component/testdata/
git commit -m "feat(component): add minimal empty component test fixture"
```

---

### Task 10: Write Integration Test - Decode Empty Component

**Files:**
- Modify: `internal/component/binary/decoder_test.go`

**Step 1: Write the integration test**

```go
// Add to internal/component/binary/decoder_test.go

import (
	"github.com/tetratelabs/wazero/internal/component/testdata"
)

func TestDecodeComponent_EmptyFixture(t *testing.T) {
	// Use the embedded test fixture
	c, err := DecodeComponent(testdata.EmptyComponent)
	require.NoError(t, err)
	require.NotNil(t, c)
}
```

**Step 2: Run test to verify it passes**

Run: `go test ./internal/component/binary/... -v -run TestDecodeComponent_EmptyFixture`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/component/binary/
git commit -m "test(component): add integration test using empty component fixture"
```

---

## Phase 1 Summary

After completing Tasks 1-10, we have:

1. Package structure: `internal/component/`, `internal/component/binary/`, `internal/component/types/`
2. Binary constants: magic, version, layer, section IDs
3. Binary detection: `IsComponent()`
4. Decoder errors: `ErrInvalidMagic`, etc.
5. Decoder preamble: validates magic/version/layer
6. Primitive types: `Bool`, `S8`..`S64`, `U8`..`U64`, `F32`, `F64`, `Char`, `String`
7. Dynamic Val: constructors and accessors for primitives
8. Test fixture: minimal empty component

---

## Phase 1 Continued: Section Parsing & First Function Call

The following tasks continue Phase 1 to achieve the first milestone: calling `add(s32, s32) -> s32`.

---

### Task 11: Parse Section Headers

**Files:**
- Modify: `internal/component/binary/decoder.go`
- Modify: `internal/component/binary/decoder_test.go`

**Step 1: Write the failing test**

```go
// Add to internal/component/binary/decoder_test.go

func TestDecodeComponent_SectionHeader(t *testing.T) {
	// Component with one empty core-custom section
	// magic + version + layer + section(id=0, size=0)
	input := append(
		append(append(Magic[:], Version[:]...), LayerComponent[:]...),
		0x00, // section ID: core-custom
		0x00, // section size: 0 (LEB128)
	)

	c, err := DecodeComponent(input)
	require.NoError(t, err)
	require.NotNil(t, c)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/binary/... -v -run TestDecodeComponent_SectionHeader`
Expected: PASS (current decoder ignores sections)

**Step 3: Add section parsing loop to decoder**

```go
// Update internal/component/binary/decoder.go

import (
	"github.com/tetratelabs/wazero/internal/leb128"
)

func DecodeComponent(binary []byte) (*component.Component, error) {
	r := bytes.NewReader(binary)

	// ... (existing preamble validation) ...

	c := &component.Component{}

	// Parse sections
	for r.Len() > 0 {
		// Read section ID
		sectionIDByte, err := r.ReadByte()
		if err != nil {
			return nil, ErrUnexpectedEOF
		}
		sectionID := SectionID(sectionIDByte)

		// Read section size (LEB128)
		sectionSize, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("section %s: %w", sectionID, ErrUnexpectedEOF)
		}

		// For now, skip section content
		if _, err := r.Seek(int64(sectionSize), io.SeekCurrent); err != nil {
			return nil, fmt.Errorf("section %s: %w", sectionID, ErrUnexpectedEOF)
		}
	}

	return c, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/binary/... -v -run TestDecodeComponent_SectionHeader`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/binary/
git commit -m "feat(component): add section header parsing loop"
```

---

### Task 12: Define Primitive Valtype Byte Opcodes

**Files:**
- Create: `internal/component/binary/valtype.go`
- Create: `internal/component/binary/valtype_test.go`

**Step 1: Write the failing test for valtype opcodes**

```go
// internal/component/binary/valtype_test.go

package binary

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestPrimValTypeOpcodes(t *testing.T) {
	tests := []struct {
		name     string
		opcode   byte
		expected PrimValType
	}{
		{"bool", 0x7f, PrimValTypeBool},
		{"s8", 0x7e, PrimValTypeS8},
		{"u8", 0x7d, PrimValTypeU8},
		{"s16", 0x7c, PrimValTypeS16},
		{"u16", 0x7b, PrimValTypeU16},
		{"s32", 0x7a, PrimValTypeS32},
		{"u32", 0x79, PrimValTypeU32},
		{"s64", 0x78, PrimValTypeS64},
		{"u64", 0x77, PrimValTypeU64},
		{"f32", 0x76, PrimValTypeF32},
		{"f64", 0x75, PrimValTypeF64},
		{"char", 0x74, PrimValTypeChar},
		{"string", 0x73, PrimValTypeString},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.opcode, byte(tc.expected))
		})
	}
}

func TestPrimValTypeName(t *testing.T) {
	require.Equal(t, "bool", PrimValTypeBool.String())
	require.Equal(t, "s32", PrimValTypeS32.String())
	require.Equal(t, "string", PrimValTypeString.String())
	require.Equal(t, "unknown(0x50)", PrimValType(0x50).String())
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/binary/... -v -run TestPrimValType`
Expected: FAIL - PrimValType undefined

**Step 3: Write the implementation**

```go
// internal/component/binary/valtype.go

package binary

import "fmt"

// PrimValType represents primitive value type opcodes in the component binary format.
// These are encoded as negative SLEB128 values starting at 0x7f.
// See: https://github.com/WebAssembly/component-model/blob/main/design/mvp/Binary.md
type PrimValType byte

const (
	PrimValTypeBool   PrimValType = 0x7f
	PrimValTypeS8     PrimValType = 0x7e
	PrimValTypeU8     PrimValType = 0x7d
	PrimValTypeS16    PrimValType = 0x7c
	PrimValTypeU16    PrimValType = 0x7b
	PrimValTypeS32    PrimValType = 0x7a
	PrimValTypeU32    PrimValType = 0x79
	PrimValTypeS64    PrimValType = 0x78
	PrimValTypeU64    PrimValType = 0x77
	PrimValTypeF32    PrimValType = 0x76
	PrimValTypeF64    PrimValType = 0x75
	PrimValTypeChar   PrimValType = 0x74
	PrimValTypeString PrimValType = 0x73
)

// String returns a human-readable name for the primitive value type.
func (p PrimValType) String() string {
	switch p {
	case PrimValTypeBool:
		return "bool"
	case PrimValTypeS8:
		return "s8"
	case PrimValTypeU8:
		return "u8"
	case PrimValTypeS16:
		return "s16"
	case PrimValTypeU16:
		return "u16"
	case PrimValTypeS32:
		return "s32"
	case PrimValTypeU32:
		return "u32"
	case PrimValTypeS64:
		return "s64"
	case PrimValTypeU64:
		return "u64"
	case PrimValTypeF32:
		return "f32"
	case PrimValTypeF64:
		return "f64"
	case PrimValTypeChar:
		return "char"
	case PrimValTypeString:
		return "string"
	default:
		return fmt.Sprintf("unknown(0x%02x)", byte(p))
	}
}

// IsPrimValType returns true if the byte is a valid primitive valtype opcode.
// Primitive valtypes are in the range 0x73-0x7f (negative SLEB128).
func IsPrimValType(b byte) bool {
	return b >= 0x73 && b <= 0x7f
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/binary/... -v -run TestPrimValType`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/binary/
git commit -m "feat(component): add primitive valtype byte opcodes"
```

---

### Task 13: Define Component Struct Fields

**Files:**
- Modify: `internal/component/component.go`
- Create: `internal/component/component_test.go`

**Step 1: Write the failing test**

```go
// internal/component/component_test.go

package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
	"github.com/tetratelabs/wazero/internal/wasm"
)

func TestComponentStructure(t *testing.T) {
	c := &Component{}

	// Verify all slice fields are nil by default
	require.Nil(t, c.CoreModules)
	require.Nil(t, c.Types)
	require.Nil(t, c.Canonicals)
	require.Nil(t, c.Exports)
}

func TestComponentWithCoreModule(t *testing.T) {
	m := &wasm.Module{}
	c := &Component{
		CoreModules: []*wasm.Module{m},
	}

	require.Equal(t, 1, len(c.CoreModules))
	require.Same(t, m, c.CoreModules[0])
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -v -run TestComponent`
Expected: FAIL - CoreModules field does not exist

**Step 3: Write the implementation**

```go
// internal/component/component.go

package component

import "github.com/tetratelabs/wazero/internal/wasm"

// Component represents a parsed WebAssembly component.
// Unlike core wasm modules, components can contain nested modules
// and components, and use rich interface types.
type Component struct {
	// CoreModules contains embedded core wasm modules (section ID 1).
	// These are the raw modules that will be instantiated.
	CoreModules []*wasm.Module

	// Types contains component type definitions (section ID 7).
	// This includes function types, component types, instance types, etc.
	Types []TypeDef

	// Canonicals contains canonical function definitions (section ID 8).
	// These define lift/lower wrappers around core functions.
	Canonicals []CanonicalDef

	// Exports contains component exports (section ID 11).
	// These expose functions and instances to the outside world.
	Exports []Export
}

// TypeDef represents a component type definition.
// This is a discriminated union of different type kinds.
type TypeDef struct {
	Kind TypeDefKind

	// For FuncType
	Func *FuncType
}

// TypeDefKind identifies the kind of type definition.
type TypeDefKind uint8

const (
	TypeDefKindFunc TypeDefKind = iota
	TypeDefKindComponent
	TypeDefKindInstance
	TypeDefKindResource
	TypeDefKindDefined
)

// FuncType represents a component function type.
// Format: 0x40 paramlist resultlist
type FuncType struct {
	Params  []NamedValType // Named parameters
	Results []NamedValType // Named results (may be unnamed for single result)
}

// NamedValType is a (name, type) pair used in function parameters/results.
type NamedValType struct {
	Name    string
	ValType ValTypeRef
}

// ValTypeRef is a reference to a value type.
// Either a primitive type opcode or a type index.
type ValTypeRef struct {
	// IsPrimitive is true if this is a primitive type (0x73-0x7f).
	IsPrimitive bool

	// Primitive is the primitive type opcode (if IsPrimitive).
	Primitive byte

	// TypeIdx is the type index (if !IsPrimitive).
	TypeIdx uint32
}

// CanonicalDef represents a canonical function definition.
type CanonicalDef struct {
	Kind CanonKind

	// For Lift: core function index, options, and component function type
	CoreFuncIdx uint32
	TypeIdx     uint32

	// For Lower: component function index and options
	FuncIdx uint32

	// Options for both Lift and Lower
	Options CanonicalOptions
}

// CanonKind identifies the kind of canonical definition.
type CanonKind uint8

const (
	CanonKindLift CanonKind = iota
	CanonKindLower
	CanonKindResourceNew
	CanonKindResourceDrop
	CanonKindResourceRep
)

// CanonicalOptions holds optional parameters for canonical operations.
type CanonicalOptions struct {
	StringEncoding StringEncoding
	MemoryIdx      *uint32 // nil if not specified
	ReallocIdx     *uint32 // nil if not specified
	PostReturnIdx  *uint32 // nil if not specified
}

// StringEncoding specifies how strings are encoded.
type StringEncoding uint8

const (
	StringEncodingUTF8 StringEncoding = iota
	StringEncodingUTF16
	StringEncodingLatin1UTF16
)

// Export represents a component export.
type Export struct {
	Name string
	Kind ExportKind
	Idx  uint32 // Index into the appropriate index space
}

// ExportKind identifies what kind of item is being exported.
type ExportKind uint8

const (
	ExportKindFunc ExportKind = iota
	ExportKindValue
	ExportKindType
	ExportKindComponent
	ExportKindInstance
)
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -v -run TestComponent`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/
git commit -m "feat(component): define Component struct with types, canonicals, exports"
```

---

### Task 14: Parse Core Module Section

**Files:**
- Modify: `internal/component/binary/decoder.go`
- Modify: `internal/component/binary/decoder_test.go`

**Step 1: Write the failing test**

```go
// Add to internal/component/binary/decoder_test.go

func TestDecodeComponent_CoreModule(t *testing.T) {
	// Build a component with an embedded minimal core module
	// Component preamble: magic(4) + version(2) + layer(2) = 8 bytes
	// Section: id(1) + size(LEB128) + content

	// Minimal valid core module: magic + version = 8 bytes
	coreModule := []byte{
		0x00, 0x61, 0x73, 0x6d, // magic
		0x01, 0x00, 0x00, 0x00, // version
	}

	// Build component
	input := append(append(Magic[:], Version[:]...), LayerComponent[:]...)
	input = append(input, byte(SectionIDCoreModule)) // section ID = 1
	input = append(input, byte(len(coreModule)))     // section size (fits in 1 byte)
	input = append(input, coreModule...)

	c, err := DecodeComponent(input)
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Equal(t, 1, len(c.CoreModules))
	require.NotNil(t, c.CoreModules[0])
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/binary/... -v -run TestDecodeComponent_CoreModule`
Expected: FAIL - c.CoreModules is nil or empty

**Step 3: Update decoder to parse core modules**

```go
// Update internal/component/binary/decoder.go

import (
	"bytes"
	"fmt"
	"io"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/leb128"
	wasmbinary "github.com/tetratelabs/wazero/internal/wasm/binary"
)

// DecodeComponent parses a WebAssembly component from binary format.
func DecodeComponent(binary []byte) (*component.Component, error) {
	r := bytes.NewReader(binary)

	// Read and validate magic number
	magic := make([]byte, 4)
	if _, err := io.ReadFull(r, magic); err != nil {
		return nil, ErrInvalidMagic
	}
	if !bytes.Equal(magic, Magic[:]) {
		return nil, ErrInvalidMagic
	}

	// Read and validate version
	version := make([]byte, 2)
	if _, err := io.ReadFull(r, version); err != nil {
		return nil, ErrUnexpectedEOF
	}
	if !bytes.Equal(version, Version[:]) {
		return nil, ErrInvalidVersion
	}

	// Read and validate layer
	layer := make([]byte, 2)
	if _, err := io.ReadFull(r, layer); err != nil {
		return nil, ErrUnexpectedEOF
	}
	if !bytes.Equal(layer, LayerComponent[:]) {
		return nil, ErrInvalidLayer
	}

	c := &component.Component{}

	// Parse sections
	for r.Len() > 0 {
		// Read section ID
		sectionIDByte, err := r.ReadByte()
		if err != nil {
			return nil, ErrUnexpectedEOF
		}
		sectionID := SectionID(sectionIDByte)

		// Read section size (LEB128)
		sectionSize, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("section %s: %w", sectionID, ErrUnexpectedEOF)
		}

		// Read section content
		sectionContent := make([]byte, sectionSize)
		if _, err := io.ReadFull(r, sectionContent); err != nil {
			return nil, fmt.Errorf("section %s: %w", sectionID, ErrUnexpectedEOF)
		}

		// Parse section based on ID
		switch sectionID {
		case SectionIDCoreModule:
			if err := decodeCoreModuleSection(c, sectionContent); err != nil {
				return nil, fmt.Errorf("section %s: %w", sectionID, err)
			}
		default:
			// Skip unknown sections for now
		}
	}

	return c, nil
}

// decodeCoreModuleSection parses an embedded core wasm module.
func decodeCoreModuleSection(c *component.Component, content []byte) error {
	// The section content is a complete core wasm module binary
	m, err := wasmbinary.DecodeModule(
		content,
		api.CoreFeaturesV2,    // Enable all standard features
		65536,                  // Default memory limit
		false,                  // memoryCapacityFromMax
		false,                  // dwarfEnabled
		false,                  // storeCustomSections
	)
	if err != nil {
		return fmt.Errorf("decode core module: %w", err)
	}

	c.CoreModules = append(c.CoreModules, m)
	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/binary/... -v -run TestDecodeComponent_CoreModule`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/binary/
git commit -m "feat(component): parse embedded core module sections"
```

---

### Task 15: Parse Type Section - Function Types

**Files:**
- Create: `internal/component/binary/types.go`
- Create: `internal/component/binary/types_test.go`
- Modify: `internal/component/binary/decoder.go`

**Step 1: Write the failing test for type parsing**

```go
// internal/component/binary/types_test.go

package binary

import (
	"bytes"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestDecodeValType_Primitives(t *testing.T) {
	tests := []struct {
		name     string
		input    byte
		expected component.ValTypeRef
	}{
		{"bool", 0x7f, component.ValTypeRef{IsPrimitive: true, Primitive: 0x7f}},
		{"s32", 0x7a, component.ValTypeRef{IsPrimitive: true, Primitive: 0x7a}},
		{"string", 0x73, component.ValTypeRef{IsPrimitive: true, Primitive: 0x73}},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			r := bytes.NewReader([]byte{tc.input})
			result, err := decodeValType(r)
			require.NoError(t, err)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestDecodeFuncType(t *testing.T) {
	// Encode: 0x40 (sync func) + params + results
	// params: vec(labelvaltype) = count + (name + valtype)*
	// results: 0x00 valtype (single result) or 0x01 0x00 (no results)

	// Function: (param "a" s32) (param "b" s32) -> s32
	input := []byte{
		0x40,       // sync functype
		0x02,       // 2 params
		0x01, 'a',  // param name "a"
		0x7a,       // s32
		0x01, 'b',  // param name "b"
		0x7a,       // s32
		0x00,       // single result
		0x7a,       // s32
	}

	r := bytes.NewReader(input)
	ft, err := decodeFuncType(r)
	require.NoError(t, err)
	require.NotNil(t, ft)
	require.Equal(t, 2, len(ft.Params))
	require.Equal(t, "a", ft.Params[0].Name)
	require.Equal(t, byte(0x7a), ft.Params[0].ValType.Primitive)
	require.Equal(t, 1, len(ft.Results))
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/binary/... -v -run TestDecode.*Type`
Expected: FAIL - decodeValType undefined

**Step 3: Write the implementation**

```go
// internal/component/binary/types.go

package binary

import (
	"fmt"
	"io"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/leb128"
)

// Type definition opcodes
const (
	TypeOpFuncSync     byte = 0x40 // Sync function type
	TypeOpFuncAsync    byte = 0x43 // Async function type
	TypeOpComponent    byte = 0x41 // Component type
	TypeOpInstance     byte = 0x42 // Instance type
	TypeOpResourceSync byte = 0x3f // Resource type (sync destructor)
)

// Result encoding
const (
	ResultSingle byte = 0x00 // Single result value
	ResultNamed  byte = 0x01 // Named results (or empty)
)

// decodeValType reads a valtype from the reader.
// valtypes are either primitive opcodes (0x73-0x7f) or type indices (LEB128).
func decodeValType(r io.ByteReader) (component.ValTypeRef, error) {
	b, err := r.ReadByte()
	if err != nil {
		return component.ValTypeRef{}, err
	}

	// Check if it's a primitive type (negative SLEB128 range)
	if IsPrimValType(b) {
		return component.ValTypeRef{
			IsPrimitive: true,
			Primitive:   b,
		}, nil
	}

	// Otherwise, it's a type index (need to unread and decode as LEB128)
	// For now, assume single-byte indices (< 128)
	return component.ValTypeRef{
		IsPrimitive: false,
		TypeIdx:     uint32(b),
	}, nil
}

// decodeFuncType reads a component function type.
// Format: 0x40 paramlist resultlist (sync) or 0x43 paramlist resultlist (async)
func decodeFuncType(r *bytes.Reader) (*component.FuncType, error) {
	opcode, err := r.ReadByte()
	if err != nil {
		return nil, err
	}

	if opcode != TypeOpFuncSync && opcode != TypeOpFuncAsync {
		return nil, fmt.Errorf("expected functype opcode 0x40 or 0x43, got 0x%02x", opcode)
	}

	ft := &component.FuncType{}

	// Parse params: vec(labelvaltype)
	paramCount, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read param count: %w", err)
	}

	ft.Params = make([]component.NamedValType, paramCount)
	for i := uint32(0); i < paramCount; i++ {
		name, err := decodeName(r)
		if err != nil {
			return nil, fmt.Errorf("read param %d name: %w", i, err)
		}

		valType, err := decodeValType(r)
		if err != nil {
			return nil, fmt.Errorf("read param %d type: %w", i, err)
		}

		ft.Params[i] = component.NamedValType{
			Name:    name,
			ValType: valType,
		}
	}

	// Parse results
	resultTag, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("read result tag: %w", err)
	}

	switch resultTag {
	case ResultSingle:
		// Single unnamed result
		valType, err := decodeValType(r)
		if err != nil {
			return nil, fmt.Errorf("read single result type: %w", err)
		}
		ft.Results = []component.NamedValType{{ValType: valType}}

	case ResultNamed:
		// Named results (vec) - count of 0 means no results
		resultCount, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read result count: %w", err)
		}

		ft.Results = make([]component.NamedValType, resultCount)
		for i := uint32(0); i < resultCount; i++ {
			name, err := decodeName(r)
			if err != nil {
				return nil, fmt.Errorf("read result %d name: %w", i, err)
			}

			valType, err := decodeValType(r)
			if err != nil {
				return nil, fmt.Errorf("read result %d type: %w", i, err)
			}

			ft.Results[i] = component.NamedValType{
				Name:    name,
				ValType: valType,
			}
		}

	default:
		return nil, fmt.Errorf("invalid result tag: 0x%02x", resultTag)
	}

	return ft, nil
}

// decodeName reads a length-prefixed UTF-8 name.
func decodeName(r *bytes.Reader) (string, error) {
	length, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return "", err
	}

	if length == 0 {
		return "", nil
	}

	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}

	return string(buf), nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/binary/... -v -run TestDecode.*Type`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/binary/
git commit -m "feat(component): parse component function types from type section"
```

---

### Task 16: Integrate Type Section Parsing into Decoder

**Files:**
- Modify: `internal/component/binary/decoder.go`
- Modify: `internal/component/binary/decoder_test.go`

**Step 1: Write the failing test**

```go
// Add to internal/component/binary/decoder_test.go

func TestDecodeComponent_TypeSection(t *testing.T) {
	// Build a component with a type section containing one function type
	// Function type: (func (param "a" s32) (param "b" s32) (result s32))

	typeSection := []byte{
		0x01,       // 1 type definition
		0x40,       // sync functype
		0x02,       // 2 params
		0x01, 'a',  // param name "a" (length 1)
		0x7a,       // s32
		0x01, 'b',  // param name "b" (length 1)
		0x7a,       // s32
		0x00,       // single result
		0x7a,       // s32
	}

	// Build component
	input := append(append(Magic[:], Version[:]...), LayerComponent[:]...)
	input = append(input, byte(SectionIDType))       // section ID = 7
	input = append(input, byte(len(typeSection)))    // section size
	input = append(input, typeSection...)

	c, err := DecodeComponent(input)
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Equal(t, 1, len(c.Types))
	require.Equal(t, component.TypeDefKindFunc, c.Types[0].Kind)
	require.NotNil(t, c.Types[0].Func)
	require.Equal(t, 2, len(c.Types[0].Func.Params))
	require.Equal(t, "a", c.Types[0].Func.Params[0].Name)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/binary/... -v -run TestDecodeComponent_TypeSection`
Expected: FAIL - c.Types is nil or empty

**Step 3: Update decoder**

```go
// Add to internal/component/binary/decoder.go switch statement

case SectionIDType:
	if err := decodeTypeSection(c, bytes.NewReader(sectionContent)); err != nil {
		return nil, fmt.Errorf("section %s: %w", sectionID, err)
	}

// Add new function to decoder.go

// decodeTypeSection parses the type section (section ID 7).
func decodeTypeSection(c *component.Component, r *bytes.Reader) error {
	// Type section is: vec(typedef)
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("read type count: %w", err)
	}

	c.Types = make([]component.TypeDef, count)
	for i := uint32(0); i < count; i++ {
		// Peek at the opcode to determine type kind
		opcode, err := r.ReadByte()
		if err != nil {
			return fmt.Errorf("read type %d opcode: %w", i, err)
		}

		switch opcode {
		case TypeOpFuncSync, TypeOpFuncAsync:
			// Unread the opcode so decodeFuncType can read it
			if err := r.UnreadByte(); err != nil {
				return err
			}

			ft, err := decodeFuncType(r)
			if err != nil {
				return fmt.Errorf("decode functype %d: %w", i, err)
			}

			c.Types[i] = component.TypeDef{
				Kind: component.TypeDefKindFunc,
				Func: ft,
			}

		default:
			return fmt.Errorf("unsupported type opcode 0x%02x at index %d", opcode, i)
		}
	}

	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/binary/... -v -run TestDecodeComponent_TypeSection`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/binary/
git commit -m "feat(component): integrate type section parsing into decoder"
```

---

### Task 17: Parse Canonical Section (Lift/Lower)

**Files:**
- Create: `internal/component/binary/canonical.go`
- Create: `internal/component/binary/canonical_test.go`
- Modify: `internal/component/binary/decoder.go`

**Step 1: Write the failing test**

```go
// internal/component/binary/canonical_test.go

package binary

import (
	"bytes"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestDecodeCanonicalLift(t *testing.T) {
	// canon lift: 0x00 0x00 core:funcidx opts typeidx
	// Minimal: lift core function 0 as component function type 0
	input := []byte{
		0x00, // canon.lift
		0x00, // core sort (always 0x00 for funcs)
		0x00, // core:funcidx = 0
		0x00, // opts count = 0
		0x00, // typeidx = 0
	}

	r := bytes.NewReader(input)
	def, err := decodeCanonical(r)
	require.NoError(t, err)
	require.Equal(t, component.CanonKindLift, def.Kind)
	require.Equal(t, uint32(0), def.CoreFuncIdx)
	require.Equal(t, uint32(0), def.TypeIdx)
}

func TestDecodeCanonicalLower(t *testing.T) {
	// canon lower: 0x01 0x00 funcidx opts
	input := []byte{
		0x01, // canon.lower
		0x00, // always 0x00
		0x00, // funcidx = 0
		0x00, // opts count = 0
	}

	r := bytes.NewReader(input)
	def, err := decodeCanonical(r)
	require.NoError(t, err)
	require.Equal(t, component.CanonKindLower, def.Kind)
	require.Equal(t, uint32(0), def.FuncIdx)
}

func TestDecodeCanonicalLiftWithOptions(t *testing.T) {
	// canon lift with memory option
	input := []byte{
		0x00, // canon.lift
		0x00, // core sort
		0x00, // core:funcidx = 0
		0x01, // opts count = 1
		0x03, // memory option
		0x00, // memory index = 0
		0x00, // typeidx = 0
	}

	r := bytes.NewReader(input)
	def, err := decodeCanonical(r)
	require.NoError(t, err)
	require.NotNil(t, def.Options.MemoryIdx)
	require.Equal(t, uint32(0), *def.Options.MemoryIdx)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/binary/... -v -run TestDecodeCanonical`
Expected: FAIL - decodeCanonical undefined

**Step 3: Write the implementation**

```go
// internal/component/binary/canonical.go

package binary

import (
	"fmt"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/leb128"
)

// Canonical opcodes
const (
	CanonOpLift         byte = 0x00
	CanonOpLower        byte = 0x01
	CanonOpResourceNew  byte = 0x02
	CanonOpResourceDrop byte = 0x03
	CanonOpResourceRep  byte = 0x04
)

// Canonical option opcodes
const (
	CanonOptStringUTF8       byte = 0x00
	CanonOptStringUTF16      byte = 0x01
	CanonOptStringLatin1UTF16 byte = 0x02
	CanonOptMemory           byte = 0x03
	CanonOptRealloc          byte = 0x04
	CanonOptPostReturn       byte = 0x05
)

// decodeCanonical reads a single canonical definition.
func decodeCanonical(r *bytes.Reader) (component.CanonicalDef, error) {
	def := component.CanonicalDef{}

	opcode, err := r.ReadByte()
	if err != nil {
		return def, err
	}

	switch opcode {
	case CanonOpLift:
		def.Kind = component.CanonKindLift

		// Read core sort (always 0x00 for funcs)
		sort, err := r.ReadByte()
		if err != nil {
			return def, fmt.Errorf("read core sort: %w", err)
		}
		if sort != 0x00 {
			return def, fmt.Errorf("expected core sort 0x00, got 0x%02x", sort)
		}

		// Read core function index
		def.CoreFuncIdx, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return def, fmt.Errorf("read core funcidx: %w", err)
		}

		// Read options
		if err := decodeCanonicalOptions(r, &def.Options); err != nil {
			return def, fmt.Errorf("read options: %w", err)
		}

		// Read component function type index
		def.TypeIdx, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return def, fmt.Errorf("read typeidx: %w", err)
		}

	case CanonOpLower:
		def.Kind = component.CanonKindLower

		// Read always-zero byte
		zero, err := r.ReadByte()
		if err != nil {
			return def, err
		}
		if zero != 0x00 {
			return def, fmt.Errorf("expected 0x00 after lower, got 0x%02x", zero)
		}

		// Read component function index
		def.FuncIdx, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return def, fmt.Errorf("read funcidx: %w", err)
		}

		// Read options
		if err := decodeCanonicalOptions(r, &def.Options); err != nil {
			return def, fmt.Errorf("read options: %w", err)
		}

	case CanonOpResourceNew:
		def.Kind = component.CanonKindResourceNew
		def.TypeIdx, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return def, fmt.Errorf("read typeidx: %w", err)
		}

	case CanonOpResourceDrop:
		def.Kind = component.CanonKindResourceDrop
		def.TypeIdx, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return def, fmt.Errorf("read typeidx: %w", err)
		}

	case CanonOpResourceRep:
		def.Kind = component.CanonKindResourceRep
		def.TypeIdx, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return def, fmt.Errorf("read typeidx: %w", err)
		}

	default:
		return def, fmt.Errorf("unknown canonical opcode: 0x%02x", opcode)
	}

	return def, nil
}

// decodeCanonicalOptions reads canonical options vector.
func decodeCanonicalOptions(r *bytes.Reader, opts *component.CanonicalOptions) error {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return err
	}

	for i := uint32(0); i < count; i++ {
		optCode, err := r.ReadByte()
		if err != nil {
			return err
		}

		switch optCode {
		case CanonOptStringUTF8:
			opts.StringEncoding = component.StringEncodingUTF8
		case CanonOptStringUTF16:
			opts.StringEncoding = component.StringEncodingUTF16
		case CanonOptStringLatin1UTF16:
			opts.StringEncoding = component.StringEncodingLatin1UTF16
		case CanonOptMemory:
			idx, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return err
			}
			opts.MemoryIdx = &idx
		case CanonOptRealloc:
			idx, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return err
			}
			opts.ReallocIdx = &idx
		case CanonOptPostReturn:
			idx, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return err
			}
			opts.PostReturnIdx = &idx
		default:
			return fmt.Errorf("unknown canonical option: 0x%02x", optCode)
		}
	}

	return nil
}
```

**Step 4: Add missing bytes import and run test**

Run: `go test ./internal/component/binary/... -v -run TestDecodeCanonical`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/binary/
git commit -m "feat(component): parse canonical lift/lower definitions"
```

---

### Task 18: Integrate Canonical Section into Decoder

**Files:**
- Modify: `internal/component/binary/decoder.go`
- Modify: `internal/component/binary/decoder_test.go`

**Step 1: Write the failing test**

```go
// Add to internal/component/binary/decoder_test.go

func TestDecodeComponent_CanonSection(t *testing.T) {
	// Build a component with a canon section containing one lift
	canonSection := []byte{
		0x01, // 1 canonical definition
		0x00, // canon.lift
		0x00, // core sort
		0x00, // core:funcidx = 0
		0x00, // opts count = 0
		0x00, // typeidx = 0
	}

	input := append(append(Magic[:], Version[:]...), LayerComponent[:]...)
	input = append(input, byte(SectionIDCanon))
	input = append(input, byte(len(canonSection)))
	input = append(input, canonSection...)

	c, err := DecodeComponent(input)
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Equal(t, 1, len(c.Canonicals))
	require.Equal(t, component.CanonKindLift, c.Canonicals[0].Kind)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/binary/... -v -run TestDecodeComponent_CanonSection`
Expected: FAIL - c.Canonicals is nil or empty

**Step 3: Update decoder**

```go
// Add to internal/component/binary/decoder.go switch statement

case SectionIDCanon:
	if err := decodeCanonSection(c, bytes.NewReader(sectionContent)); err != nil {
		return nil, fmt.Errorf("section %s: %w", sectionID, err)
	}

// Add new function

// decodeCanonSection parses the canonical section (section ID 8).
func decodeCanonSection(c *component.Component, r *bytes.Reader) error {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("read canon count: %w", err)
	}

	c.Canonicals = make([]component.CanonicalDef, count)
	for i := uint32(0); i < count; i++ {
		def, err := decodeCanonical(r)
		if err != nil {
			return fmt.Errorf("decode canonical %d: %w", i, err)
		}
		c.Canonicals[i] = def
	}

	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/binary/... -v -run TestDecodeComponent_CanonSection`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/binary/
git commit -m "feat(component): integrate canonical section parsing into decoder"
```

---

### Task 19: Parse Export Section

**Files:**
- Create: `internal/component/binary/exports.go`
- Create: `internal/component/binary/exports_test.go`
- Modify: `internal/component/binary/decoder.go`

**Step 1: Write the failing test**

```go
// internal/component/binary/exports_test.go

package binary

import (
	"bytes"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestDecodeExport(t *testing.T) {
	// Export format: exportname' sortidx [externdesc?]
	// exportname': 0x00 len name (simple, no version suffix)
	// sortidx: sort u32

	// Export function index 0 with name "add"
	input := []byte{
		0x00,             // simple name (no version)
		0x03, 'a', 'd', 'd', // name length=3, "add"
		0x01,             // sort = func (0x01)
		0x00,             // index = 0
		// No optional externdesc
	}

	r := bytes.NewReader(input)
	exp, err := decodeExport(r)
	require.NoError(t, err)
	require.Equal(t, "add", exp.Name)
	require.Equal(t, component.ExportKindFunc, exp.Kind)
	require.Equal(t, uint32(0), exp.Idx)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/binary/... -v -run TestDecodeExport`
Expected: FAIL - decodeExport undefined

**Step 3: Write the implementation**

```go
// internal/component/binary/exports.go

package binary

import (
	"fmt"
	"io"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/leb128"
)

// Export name discriminators
const (
	ExportNameSimple     byte = 0x00 // No version suffix
	ExportNameVersioned  byte = 0x01 // Has version suffix
)

// Sort indicators (for sortidx)
const (
	SortCore      byte = 0x00 // Core definition
	SortFunc      byte = 0x01 // Component function
	SortValue     byte = 0x02 // Value (gated)
	SortType      byte = 0x03 // Type
	SortComponent byte = 0x04 // Component
	SortInstance  byte = 0x05 // Instance
)

// decodeExport reads a single export definition.
func decodeExport(r *bytes.Reader) (component.Export, error) {
	exp := component.Export{}

	// Read export name
	name, err := decodeExportName(r)
	if err != nil {
		return exp, fmt.Errorf("read export name: %w", err)
	}
	exp.Name = name

	// Read sortidx (sort + index)
	sort, err := r.ReadByte()
	if err != nil {
		return exp, fmt.Errorf("read sort: %w", err)
	}

	idx, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return exp, fmt.Errorf("read index: %w", err)
	}
	exp.Idx = idx

	// Map sort to ExportKind
	switch sort {
	case SortFunc:
		exp.Kind = component.ExportKindFunc
	case SortValue:
		exp.Kind = component.ExportKindValue
	case SortType:
		exp.Kind = component.ExportKindType
	case SortComponent:
		exp.Kind = component.ExportKindComponent
	case SortInstance:
		exp.Kind = component.ExportKindInstance
	case SortCore:
		// Core exports have a nested sort byte
		// For simplicity, treat as func for now
		exp.Kind = component.ExportKindFunc
	default:
		return exp, fmt.Errorf("unknown sort: 0x%02x", sort)
	}

	// Note: optional externdesc is skipped for Phase 1

	return exp, nil
}

// decodeExportName reads an export name with optional version suffix.
func decodeExportName(r *bytes.Reader) (string, error) {
	discriminator, err := r.ReadByte()
	if err != nil {
		return "", err
	}

	switch discriminator {
	case ExportNameSimple:
		// Just read the name
		return decodeName(r)

	case ExportNameVersioned:
		// Read name, then skip version suffix
		name, err := decodeName(r)
		if err != nil {
			return "", err
		}
		// Skip version suffix for now
		suffixLen, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return "", err
		}
		if _, err := io.CopyN(io.Discard, r, int64(suffixLen)); err != nil {
			return "", err
		}
		return name, nil

	default:
		return "", fmt.Errorf("unknown export name discriminator: 0x%02x", discriminator)
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/binary/... -v -run TestDecodeExport`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/binary/
git commit -m "feat(component): parse export definitions"
```

---

### Task 20: Integrate Export Section into Decoder

**Files:**
- Modify: `internal/component/binary/decoder.go`
- Modify: `internal/component/binary/decoder_test.go`

**Step 1: Write the failing test**

```go
// Add to internal/component/binary/decoder_test.go

func TestDecodeComponent_ExportSection(t *testing.T) {
	exportSection := []byte{
		0x01,             // 1 export
		0x00,             // simple name
		0x03, 'a', 'd', 'd', // name "add"
		0x01,             // sort = func
		0x00,             // index = 0
	}

	input := append(append(Magic[:], Version[:]...), LayerComponent[:]...)
	input = append(input, byte(SectionIDExport))
	input = append(input, byte(len(exportSection)))
	input = append(input, exportSection...)

	c, err := DecodeComponent(input)
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Equal(t, 1, len(c.Exports))
	require.Equal(t, "add", c.Exports[0].Name)
	require.Equal(t, component.ExportKindFunc, c.Exports[0].Kind)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/binary/... -v -run TestDecodeComponent_ExportSection`
Expected: FAIL - c.Exports is nil or empty

**Step 3: Update decoder**

```go
// Add to internal/component/binary/decoder.go switch statement

case SectionIDExport:
	if err := decodeExportSection(c, bytes.NewReader(sectionContent)); err != nil {
		return nil, fmt.Errorf("section %s: %w", sectionID, err)
	}

// Add new function

// decodeExportSection parses the export section (section ID 11).
func decodeExportSection(c *component.Component, r *bytes.Reader) error {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("read export count: %w", err)
	}

	c.Exports = make([]component.Export, count)
	for i := uint32(0); i < count; i++ {
		exp, err := decodeExport(r)
		if err != nil {
			return fmt.Errorf("decode export %d: %w", i, err)
		}
		c.Exports[i] = exp
	}

	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/binary/... -v -run TestDecodeComponent_ExportSection`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/binary/
git commit -m "feat(component): integrate export section parsing into decoder"
```

---

### Task 21: Create Simple add_s32 Test Component with cargo-component

**Files:**
- Create: `internal/component/testdata/primitives/add.wit`
- Create: `internal/component/testdata/primitives/src/lib.rs`
- Create: `internal/component/testdata/primitives/Cargo.toml`
- Build: `internal/component/testdata/primitives/add_s32.wasm`

**Step 1: Create the WIT interface definition**

```wit
// internal/component/testdata/primitives/add.wit

package test:primitives;

world primitives {
    export add: func(a: s32, b: s32) -> s32;
}
```

**Step 2: Create Cargo.toml for the component**

```toml
# internal/component/testdata/primitives/Cargo.toml

[package]
name = "primitives"
version = "0.1.0"
edition = "2021"

[lib]
crate-type = ["cdylib"]

[dependencies]
wit-bindgen = "0.36"

[package.metadata.component]
package = "test:primitives"

[package.metadata.component.target]
path = "add.wit"
world = "primitives"
```

**Step 3: Create the Rust implementation**

```rust
// internal/component/testdata/primitives/src/lib.rs

wit_bindgen::generate!({
    world: "primitives",
    path: "../add.wit",
});

struct Component;

impl Guest for Component {
    fn add(a: i32, b: i32) -> i32 {
        a + b
    }
}

export!(Component);
```

**Step 4: Build the component**

```bash
cd internal/component/testdata/primitives
cargo component build --release
cp target/wasm32-wasip1/release/primitives.wasm ../add_s32.wasm
```

**Step 5: Verify the component binary**

Run: `wasm-tools print internal/component/testdata/add_s32.wasm | head -50`
Expected: Should show component structure with core module, types, and exports

**Step 6: Update testdata.go to embed the new fixture**

```go
// Update internal/component/testdata/testdata.go

package testdata

import (
	_ "embed"
)

// EmptyComponent is a minimal valid component with no content.
//go:embed empty.wasm
var EmptyComponent []byte

// AddS32Component exports an add function: (s32, s32) -> s32
//go:embed add_s32.wasm
var AddS32Component []byte
```

**Step 7: Commit**

```bash
git add internal/component/testdata/
git commit -m "feat(component): add add_s32 test component fixture"
```

---

### Task 22: Parse Complete add_s32 Component

**Files:**
- Modify: `internal/component/binary/decoder_test.go`

**Step 1: Write the integration test**

```go
// Add to internal/component/binary/decoder_test.go

func TestDecodeComponent_AddS32Fixture(t *testing.T) {
	c, err := DecodeComponent(testdata.AddS32Component)
	require.NoError(t, err)
	require.NotNil(t, c)

	// Should have at least one core module
	require.GreaterOrEqual(t, len(c.CoreModules), 1)

	// Should have at least one type (the add function type)
	require.GreaterOrEqual(t, len(c.Types), 1)

	// Should have at least one canonical definition (lift for add)
	require.GreaterOrEqual(t, len(c.Canonicals), 1)

	// Should export "add"
	require.GreaterOrEqual(t, len(c.Exports), 1)

	// Find the "add" export
	var addExport *component.Export
	for i := range c.Exports {
		if c.Exports[i].Name == "add" {
			addExport = &c.Exports[i]
			break
		}
	}
	require.NotNil(t, addExport, "expected 'add' export")
	require.Equal(t, component.ExportKindFunc, addExport.Kind)
}
```

**Step 2: Run test and fix any parsing issues**

Run: `go test ./internal/component/binary/... -v -run TestDecodeComponent_AddS32Fixture`
Expected: PASS (may require debugging to handle all sections in the fixture)

**Step 3: Commit**

```bash
git add internal/component/binary/
git commit -m "test(component): verify parsing of add_s32 component fixture"
```

---

### Task 23: Implement ComponentInstance Structure

**Files:**
- Create: `internal/component/instance.go`
- Create: `internal/component/instance_test.go`

**Step 1: Write the failing test**

```go
// internal/component/instance_test.go

package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestInstanceStructure(t *testing.T) {
	c := &Component{}
	inst := &Instance{
		component: c,
	}

	require.Same(t, c, inst.Component())
	require.Nil(t, inst.ExportedFunction("nonexistent"))
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -v -run TestInstance`
Expected: FAIL - Instance undefined

**Step 3: Write the implementation**

```go
// internal/component/instance.go

package component

import (
	"context"

	"github.com/tetratelabs/wazero/api"
)

// Instance represents an instantiated component.
type Instance struct {
	component     *Component
	coreInstances []api.Module
	exports       map[string]*ExportedFunc
}

// Component returns the component this instance was created from.
func (i *Instance) Component() *Component {
	return i.component
}

// ExportedFunction returns the exported function with the given name,
// or nil if not found.
func (i *Instance) ExportedFunction(name string) *ExportedFunc {
	if i.exports == nil {
		return nil
	}
	return i.exports[name]
}

// ExportedFunc represents an exported component function.
type ExportedFunc struct {
	name       string
	funcType   *FuncType
	coreFunc   api.Function
	canonical  *CanonicalDef
}

// Name returns the export name of this function.
func (f *ExportedFunc) Name() string {
	return f.name
}

// Call invokes the exported function with the given arguments.
// For Phase 1, this only supports primitive types (especially s32).
func (f *ExportedFunc) Call(ctx context.Context, params ...Val) ([]Val, error) {
	// Phase 1: Primitive flat ABI only
	// Convert component Vals to core wasm values
	coreParams := make([]uint64, len(params))
	for i, p := range params {
		switch p.Kind() {
		case ValKindS32:
			coreParams[i] = uint64(uint32(p.S32()))
		case ValKindU32:
			coreParams[i] = uint64(p.U32())
		case ValKindS64:
			coreParams[i] = uint64(p.S64())
		case ValKindU64:
			coreParams[i] = p.U64()
		default:
			// For now, only support integers
			coreParams[i] = 0
		}
	}

	// Call the core function
	coreResults, err := f.coreFunc.Call(ctx, coreParams...)
	if err != nil {
		return nil, err
	}

	// Convert core results back to component Vals
	// Phase 1: Assume single s32 result
	results := make([]Val, len(coreResults))
	for i, r := range coreResults {
		results[i] = ValS32(int32(r))
	}

	return results, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -v -run TestInstance`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/
git commit -m "feat(component): add Instance and ExportedFunc structures"
```

---

### Task 24: Implement Basic Instantiator

**Files:**
- Create: `internal/component/instantiate.go`
- Create: `internal/component/instantiate_test.go`

**Step 1: Write the failing test**

```go
// internal/component/instantiate_test.go

package component

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/internal/component/binary"
	"github.com/tetratelabs/wazero/internal/component/testdata"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestInstantiate_AddS32(t *testing.T) {
	ctx := context.Background()

	// Create wazero runtime
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	// Parse the component
	c, err := binary.DecodeComponent(testdata.AddS32Component)
	require.NoError(t, err)

	// Instantiate the component
	inst, err := Instantiate(ctx, rt, c)
	require.NoError(t, err)
	require.NotNil(t, inst)

	// Get the "add" export
	add := inst.ExportedFunction("add")
	require.NotNil(t, add, "expected 'add' export")

	// Call add(2, 3) and expect 5
	results, err := add.Call(ctx, ValS32(2), ValS32(3))
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, int32(5), results[0].S32())
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -v -run TestInstantiate_AddS32`
Expected: FAIL - Instantiate undefined

**Step 3: Write the implementation**

```go
// internal/component/instantiate.go

package component

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero"
)

// Instantiate creates an Instance from a parsed Component.
// It instantiates all embedded core modules and wires up exports.
func Instantiate(ctx context.Context, rt wazero.Runtime, c *Component) (*Instance, error) {
	inst := &Instance{
		component:     c,
		coreInstances: make([]api.Module, len(c.CoreModules)),
		exports:       make(map[string]*ExportedFunc),
	}

	// Instantiate each core module
	for i, m := range c.CoreModules {
		compiled, err := rt.CompileModule(ctx, m.Source)
		if err != nil {
			return nil, fmt.Errorf("compile core module %d: %w", i, err)
		}

		modInst, err := rt.InstantiateModule(ctx, compiled, wazero.NewModuleConfig())
		if err != nil {
			return nil, fmt.Errorf("instantiate core module %d: %w", i, err)
		}

		inst.coreInstances[i] = modInst
	}

	// Wire up exports based on canonical definitions
	for _, exp := range c.Exports {
		if exp.Kind != ExportKindFunc {
			continue
		}

		// Find the canonical definition for this export
		if exp.Idx >= uint32(len(c.Canonicals)) {
			continue
		}
		canon := &c.Canonicals[exp.Idx]

		if canon.Kind != CanonKindLift {
			continue
		}

		// Find the core function
		if len(inst.coreInstances) == 0 {
			continue
		}
		coreModule := inst.coreInstances[0]

		// Get the core function by index
		// Note: This is simplified; real impl needs to track function index space
		coreFuncs := coreModule.ExportedFunctionDefinitions()
		var coreFunc api.Function
		for name := range coreFuncs {
			// For now, find any exported function
			coreFunc = coreModule.ExportedFunction(name)
			break
		}

		if coreFunc == nil {
			continue
		}

		// Find the function type
		var funcType *FuncType
		if canon.TypeIdx < uint32(len(c.Types)) {
			td := &c.Types[canon.TypeIdx]
			if td.Kind == TypeDefKindFunc {
				funcType = td.Func
			}
		}

		inst.exports[exp.Name] = &ExportedFunc{
			name:      exp.Name,
			funcType:  funcType,
			coreFunc:  coreFunc,
			canonical: canon,
		}
	}

	return inst, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -v -run TestInstantiate_AddS32`
Expected: PASS (may need debugging)

**Step 5: Commit**

```bash
git add internal/component/
git commit -m "feat(component): implement basic component instantiation"
```

---

### Task 25: Integration Test - Full add(2, 3) = 5

**Files:**
- Modify: `internal/component/instantiate_test.go`

**Step 1: Extend the integration test with edge cases**

```go
// Add to internal/component/instantiate_test.go

func TestInstantiate_AddS32_EdgeCases(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	c, err := binary.DecodeComponent(testdata.AddS32Component)
	require.NoError(t, err)

	inst, err := Instantiate(ctx, rt, c)
	require.NoError(t, err)

	add := inst.ExportedFunction("add")
	require.NotNil(t, add)

	tests := []struct {
		name     string
		a, b     int32
		expected int32
	}{
		{"zero plus zero", 0, 0, 0},
		{"positive plus positive", 2, 3, 5},
		{"negative plus negative", -2, -3, -5},
		{"positive plus negative", 5, -3, 2},
		{"max int32", 2147483646, 1, 2147483647},
		{"min int32", -2147483647, -1, -2147483648},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			results, err := add.Call(ctx, ValS32(tc.a), ValS32(tc.b))
			require.NoError(t, err)
			require.Equal(t, 1, len(results))
			require.Equal(t, tc.expected, results[0].S32())
		})
	}
}
```

**Step 2: Run test to verify all edge cases pass**

Run: `go test ./internal/component/... -v -run TestInstantiate_AddS32`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/component/
git commit -m "test(component): add edge case tests for add_s32 function"
```

---

