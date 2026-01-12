# Component Model Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement full WebAssembly Component Model and WASI Preview 2 support in wazero.

**Architecture:** Parallel `internal/component/` package structure with single-pass streaming binary parser, hybrid lift/lower (dynamic Val + interfaces), generation-counted resource handles, and engine-agnostic component orchestration layer.

**Tech Stack:** Go (zero external dependencies), wazero core wasm runtime, cargo-component/wasm-tools for test fixture generation.

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

### Task 12-30: Continue implementation...

*[The plan continues with detailed tasks for:]*
- Task 12: Parse core-module sections (delegate to wasm.DecodeModule)
- Task 13: Parse type section (primitive function types)
- Task 14: Parse canon section (lift/lower definitions)
- Task 15: Parse export section
- Task 16: Build Component struct with parsed data
- Task 17: Implement basic flat ABI for s32
- Task 18: Create component instance structure
- Task 19: Wire up core module instantiation
- Task 20: Implement exported function lookup
- Task 21: Implement function call with primitive lift/lower
- Task 22: Create add_i32.wasm test fixture using cargo-component
- Task 23: Integration test - call add(2, 3) == 5
- Task 24-30: Edge cases, error handling, cleanup

---

## Phase 2-6 Overview

Detailed task breakdowns for subsequent phases follow the same pattern. Key milestones:

**Phase 2: Complete Type System**
- Tasks 31-50: Record, variant, list, option, result, flags, enum, tuple types
- Tasks 51-60: Memory layout and composite lift/lower
- Tasks 61-70: String encoding support

**Phase 3: Resources**
- Tasks 71-80: ResourceTable implementation
- Tasks 81-90: Own/borrow semantics
- Tasks 91-100: Destructor invocation

**Phase 4: Full Instantiation & Linking**
- Tasks 101-120: Alias section, canonical definitions
- Tasks 121-140: Linker with semver matching
- Tasks 141-150: Nested components

**Phase 5: WASI Preview 2**
- Tasks 151-180: wasi:cli, wasi:filesystem, wasi:io
- Tasks 181-210: wasi:clocks, wasi:random, wasi:sockets
- Tasks 211-240: wasi:http

**Phase 6: Polish & Conformance**
- Tasks 241-260: Wasmtime test suite conformance
- Tasks 261-270: Performance optimization
- Tasks 271-280: Documentation and examples

---

## Running Tests

```bash
# Run all component tests
go test ./internal/component/... -v

# Run specific test
go test ./internal/component/binary/... -v -run TestDecodeComponent

# Run with race detector
go test ./internal/component/... -race -v
```

## References

- [Component Model Binary Format](https://github.com/WebAssembly/component-model/blob/main/design/mvp/Binary.md)
- [Canonical ABI](https://github.com/WebAssembly/component-model/blob/main/design/mvp/CanonicalABI.md)
- [wazero internal/wasm/binary/decoder.go](internal/wasm/binary/decoder.go) - Pattern reference
- [wazero internal/testing/require/require.go](internal/testing/require/require.go) - Assertion library
