# Binary Parser Completion Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Complete the component model binary parser to successfully parse real-world WASI Preview 2 components like `add.wasm` and `subtract.wasm`.

**Architecture:** Extend `internal/component/binary/` with missing type decoders (instance type 0x42, component type 0x41), add missing opcodes (ErrorContext 0x64, async types), and fix incomplete implementations (core module type, export externdesc).

**Tech Stack:** Go, existing binary parsing infrastructure in `internal/component/binary/`, test files in `internal/component/wasip2test/plugins/`.

**Prerequisites:** Existing binary parser foundation (sections 0-12, primitive types, composite types, canonicals, aliases).

**Produces:** Binary parser capable of parsing production WASI P2 components.

---

## Phase 1: Critical - Instance Type (0x42) Support

This is the highest priority gap. WASI P2 components use instance types extensively for imports.

---

### Task 1: Add Instance Type Data Structures

**Files:**
- Modify: `internal/component/component.go:62-106`
- Test: `internal/component/component_test.go`

**Step 1: Write the failing test**

```go
// internal/component/component_test.go (add)
func TestInstanceTypeDef(t *testing.T) {
	// Verify instance type definition can be created
	decl := InstanceDecl{
		Kind: InstanceDeclKindExport,
		Export: &InstanceExport{
			Name: "test",
			Kind: ExportKindFunc,
			Idx:  0,
		},
	}
	instType := InstanceTypeDef{
		Declarations: []InstanceDecl{decl},
	}
	if len(instType.Declarations) != 1 {
		t.Errorf("expected 1 declaration, got %d", len(instType.Declarations))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/component/... -v -run TestInstanceTypeDef`
Expected: FAIL with "undefined: InstanceTypeDef"

**Step 3: Write minimal implementation**

Add to `internal/component/component.go` after `TypeDef` struct:

```go
// InstanceDeclKind identifies the kind of instance declaration.
type InstanceDeclKind uint8

const (
	InstanceDeclKindCoreType InstanceDeclKind = 0x00
	InstanceDeclKindType     InstanceDeclKind = 0x01
	InstanceDeclKindAlias    InstanceDeclKind = 0x02
	InstanceDeclKindExport   InstanceDeclKind = 0x04
)

// InstanceDecl represents a declaration within an instance type.
type InstanceDecl struct {
	Kind     InstanceDeclKind
	CoreType *CoreTypeDef
	Type     *TypeDef
	Alias    *Alias
	Export   *InstanceExport
}

// InstanceExport represents an export declaration in an instance type.
type InstanceExport struct {
	Name     string
	Kind     ExportKind
	Idx      uint32
	TypeIdx  *uint32 // Optional type annotation
}

// InstanceTypeDef represents an instance type (0x42).
type InstanceTypeDef struct {
	Declarations []InstanceDecl
}
```

Update `TypeDef` struct to add:

```go
type TypeDef struct {
	// ... existing fields ...
	Instance *InstanceTypeDef  // For 0x42
}
```

**Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/component/... -v -run TestInstanceTypeDef`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/component.go internal/component/component_test.go
git commit -m "$(cat <<'EOF'
feat(component): add instance type definition data structures

Add InstanceTypeDef and InstanceDecl types to support parsing instance
types (0x42) which are used extensively in WASI P2 component imports.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: Add Instance Type Binary Decoder

**Files:**
- Create: `internal/component/binary/instance_type.go`
- Modify: `internal/component/binary/decoder.go:165-291`
- Test: `internal/component/binary/instance_type_test.go`

**Step 1: Write the failing test**

```go
// internal/component/binary/instance_type_test.go
package binary

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
)

func TestDecodeInstanceType(t *testing.T) {
	// Instance type with one export declaration
	// (instance (export "test" (func (type 0))))
	data := buildComponentWithTypeSection([]byte{
		0x42,       // instance type opcode
		0x01,       // 1 declaration
		0x04,       // export declaration
		0x00,       // simple name
		0x04, 't', 'e', 's', 't', // name "test"
		0x01,       // func sort
		0x00,       // type index 0
		0x00,       // no type annotation
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(c.Types) != 1 {
		t.Fatalf("expected 1 type, got %d", len(c.Types))
	}

	if c.Types[0].Kind != component.TypeDefKindInstance {
		t.Fatalf("expected instance type, got kind %d", c.Types[0].Kind)
	}

	if c.Types[0].Instance == nil {
		t.Fatal("expected instance type def")
	}

	if len(c.Types[0].Instance.Declarations) != 1 {
		t.Errorf("expected 1 declaration, got %d", len(c.Types[0].Instance.Declarations))
	}
}

func TestDecodeInstanceTypeWithAlias(t *testing.T) {
	// Instance type with alias declaration
	// (instance (alias outer 0 1 (type)))
	data := buildComponentWithTypeSection([]byte{
		0x42,       // instance type opcode
		0x01,       // 1 declaration
		0x02,       // alias declaration
		0x03,       // type sort
		0x02,       // outer alias target
		0x00,       // outer count 0
		0x01,       // outer index 1
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.Types[0].Instance.Declarations[0].Kind != component.InstanceDeclKindAlias {
		t.Errorf("expected alias declaration")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/component/binary/... -v -run TestDecodeInstanceType`
Expected: FAIL with "unsupported type opcode 0x42"

**Step 3: Write minimal implementation**

Create `internal/component/binary/instance_type.go`:

```go
package binary

import (
	"bytes"
	"fmt"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/leb128"
)

// TypeOpInstance is the opcode for instance types.
const TypeOpInstance byte = 0x42

// decodeInstanceTypeDef decodes an instance type definition.
// Format: 0x42 vec(instancetypedecl)
// instancetypedecl ::= 0x00 core:type         (core type)
//                    | 0x01 type              (type)
//                    | 0x02 alias             (alias)
//                    | 0x04 export            (export)
func decodeInstanceTypeDef(r *bytes.Reader) (*component.InstanceTypeDef, error) {
	declCount, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read declaration count: %w", err)
	}

	decls := make([]component.InstanceDecl, declCount)
	for i := uint32(0); i < declCount; i++ {
		decl, err := decodeInstanceDecl(r)
		if err != nil {
			return nil, fmt.Errorf("decode declaration %d: %w", i, err)
		}
		decls[i] = decl
	}

	return &component.InstanceTypeDef{Declarations: decls}, nil
}

// decodeInstanceDecl decodes a single instance type declaration.
func decodeInstanceDecl(r *bytes.Reader) (component.InstanceDecl, error) {
	var decl component.InstanceDecl

	kindByte, err := r.ReadByte()
	if err != nil {
		return decl, fmt.Errorf("read declaration kind: %w", err)
	}
	decl.Kind = component.InstanceDeclKind(kindByte)

	switch decl.Kind {
	case component.InstanceDeclKindCoreType:
		// Core type declaration
		coreType, err := decodeCoreTypeDef(r)
		if err != nil {
			return decl, fmt.Errorf("decode core type: %w", err)
		}
		decl.CoreType = coreType

	case component.InstanceDeclKindType:
		// Nested type declaration - recursively decode
		typeDef, err := decodeTypeDef(r)
		if err != nil {
			return decl, fmt.Errorf("decode type: %w", err)
		}
		decl.Type = typeDef

	case component.InstanceDeclKindAlias:
		// Alias declaration
		alias, err := decodeAliasInType(r)
		if err != nil {
			return decl, fmt.Errorf("decode alias: %w", err)
		}
		decl.Alias = alias

	case component.InstanceDeclKindExport:
		// Export declaration
		export, err := decodeInstanceExport(r)
		if err != nil {
			return decl, fmt.Errorf("decode export: %w", err)
		}
		decl.Export = export

	default:
		return decl, fmt.Errorf("unknown instance declaration kind: 0x%02x", kindByte)
	}

	return decl, nil
}

// decodeInstanceExport decodes an export declaration within an instance type.
func decodeInstanceExport(r *bytes.Reader) (*component.InstanceExport, error) {
	name, err := decodeExportName(r)
	if err != nil {
		return nil, fmt.Errorf("decode export name: %w", err)
	}

	sortByte, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("read sort: %w", err)
	}

	idx, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read index: %w", err)
	}

	export := &component.InstanceExport{
		Name: name,
		Idx:  idx,
	}

	// Map sort to ExportKind
	switch sortByte {
	case 0x00:
		// Core sort - read nested core sort
		coreSortByte, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("read core sort: %w", err)
		}
		_ = coreSortByte // Store if needed
		export.Kind = component.ExportKindFunc
	case 0x01:
		export.Kind = component.ExportKindFunc
	case 0x02:
		export.Kind = component.ExportKindValue
	case 0x03:
		export.Kind = component.ExportKindType
	case 0x04:
		export.Kind = component.ExportKindComponent
	case 0x05:
		export.Kind = component.ExportKindInstance
	default:
		return nil, fmt.Errorf("unknown sort: 0x%02x", sortByte)
	}

	// Check for optional type annotation
	hasType, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("read type annotation flag: %w", err)
	}
	if hasType == 0x01 {
		typeIdx, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read type index: %w", err)
		}
		export.TypeIdx = &typeIdx
	}

	return export, nil
}

// decodeAliasInType decodes an alias declaration within a type definition.
func decodeAliasInType(r *bytes.Reader) (*component.Alias, error) {
	// Alias format: sort aliastarget
	return decodeAlias(r)
}

// decodeCoreTypeDef decodes a core type definition for use in instance/component types.
func decodeCoreTypeDef(r *bytes.Reader) (*component.CoreTypeDef, error) {
	opcode, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("read core type opcode: %w", err)
	}

	switch opcode {
	case 0x60: // func type
		funcType, err := decodeCoreFunc(r)
		if err != nil {
			return nil, fmt.Errorf("decode core func: %w", err)
		}
		return &component.CoreTypeDef{
			Kind: component.CoreTypeDefKindFunc,
			Func: funcType,
		}, nil
	case 0x50: // module type
		moduleType, err := decodeCoreModuleType(r)
		if err != nil {
			return nil, fmt.Errorf("decode core module: %w", err)
		}
		return &component.CoreTypeDef{
			Kind:   component.CoreTypeDefKindModule,
			Module: moduleType,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported core type opcode: 0x%02x", opcode)
	}
}

// decodeTypeDef decodes a type definition (for nested types in instance/component types).
func decodeTypeDef(r *bytes.Reader) (*component.TypeDef, error) {
	opcode, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("read type opcode: %w", err)
	}

	typeDef := &component.TypeDef{}

	switch opcode {
	case TypeOpFuncSync, TypeOpFuncAsync:
		if err := r.UnreadByte(); err != nil {
			return nil, err
		}
		ft, err := decodeFuncType(r)
		if err != nil {
			return nil, err
		}
		typeDef.Kind = component.TypeDefKindFunc
		typeDef.Func = ft

	case TypeOpInstance:
		inst, err := decodeInstanceTypeDef(r)
		if err != nil {
			return nil, err
		}
		typeDef.Kind = component.TypeDefKindInstance
		typeDef.Instance = inst

	case ValTypeOpcodeRecord:
		record, err := decodeRecordTypeDef(r)
		if err != nil {
			return nil, err
		}
		typeDef.Kind = component.TypeDefKindDefined
		typeDef.Record = convertRecordTypeDef(record)

	case ValTypeOpcodeVariant:
		variant, err := decodeVariantTypeDef(r)
		if err != nil {
			return nil, err
		}
		typeDef.Kind = component.TypeDefKindDefined
		typeDef.Variant = convertVariantTypeDef(variant)

	case ValTypeOpcodeList:
		list, err := decodeListTypeDef(r)
		if err != nil {
			return nil, err
		}
		typeDef.Kind = component.TypeDefKindDefined
		typeDef.List = convertListTypeDef(list)

	case ValTypeOpcodeTuple:
		tuple, err := decodeTupleTypeDef(r)
		if err != nil {
			return nil, err
		}
		typeDef.Kind = component.TypeDefKindDefined
		typeDef.Tuple = convertTupleTypeDef(tuple)

	case ValTypeOpcodeFlags:
		flags, err := decodeFlagsTypeDef(r)
		if err != nil {
			return nil, err
		}
		typeDef.Kind = component.TypeDefKindDefined
		typeDef.Flags = convertFlagsTypeDef(flags)

	case ValTypeOpcodeEnum:
		enum, err := decodeEnumTypeDef(r)
		if err != nil {
			return nil, err
		}
		typeDef.Kind = component.TypeDefKindDefined
		typeDef.Enum = convertEnumTypeDef(enum)

	case ValTypeOpcodeOption:
		option, err := decodeOptionTypeDef(r)
		if err != nil {
			return nil, err
		}
		typeDef.Kind = component.TypeDefKindDefined
		typeDef.Option = convertOptionTypeDef(option)

	case ValTypeOpcodeResult:
		result, err := decodeResultTypeDef(r)
		if err != nil {
			return nil, err
		}
		typeDef.Kind = component.TypeDefKindDefined
		typeDef.Result = convertResultTypeDef(result)

	case TypeOpResourceSync:
		resourceDef, err := decodeResourceTypeDef(r)
		if err != nil {
			return nil, err
		}
		typeDef.Kind = component.TypeDefKindResource
		typeDef.Resource = resourceDef

	default:
		return nil, fmt.Errorf("unsupported nested type opcode: 0x%02x", opcode)
	}

	return typeDef, nil
}
```

Update `internal/component/binary/decoder.go` in `decodeTypeSection` switch to add:

```go
case TypeOpInstance:
	inst, err := decodeInstanceTypeDef(r)
	if err != nil {
		return fmt.Errorf("decode instance type %d: %w", i, err)
	}
	c.Types[i] = component.TypeDef{
		Kind:     component.TypeDefKindInstance,
		Instance: inst,
	}
```

**Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/component/binary/... -v -run TestDecodeInstanceType`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/binary/instance_type.go internal/component/binary/instance_type_test.go internal/component/binary/decoder.go
git commit -m "$(cat <<'EOF'
feat(binary): add instance type (0x42) parsing

Implement parsing for instance types which define the shape of component
imports. This is critical for WASI P2 support where all imports use
instance types.

Supports all declaration kinds:
- Core type declarations (0x00)
- Type declarations (0x01)
- Alias declarations (0x02)
- Export declarations (0x04)

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Add Component Type (0x41) Data Structures and Decoder

**Files:**
- Modify: `internal/component/component.go`
- Create: `internal/component/binary/component_type.go`
- Test: `internal/component/binary/component_type_test.go`

**Step 1: Write the failing test**

```go
// internal/component/binary/component_type_test.go
package binary

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
)

func TestDecodeComponentType(t *testing.T) {
	// Component type with an import declaration
	data := buildComponentWithTypeSection([]byte{
		0x41,       // component type opcode
		0x01,       // 1 declaration
		0x03,       // import declaration
		0x00,       // simple name
		0x04, 't', 'e', 's', 't', // name "test"
		0x01,       // func extern desc
		0x00,       // type index 0
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(c.Types) != 1 {
		t.Fatalf("expected 1 type, got %d", len(c.Types))
	}

	if c.Types[0].Kind != component.TypeDefKindComponent {
		t.Fatalf("expected component type, got kind %d", c.Types[0].Kind)
	}

	if c.Types[0].Component == nil {
		t.Fatal("expected component type def")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/component/binary/... -v -run TestDecodeComponentType`
Expected: FAIL with "unsupported type opcode 0x41"

**Step 3: Write minimal implementation**

Add to `internal/component/component.go`:

```go
// ComponentDeclKind identifies the kind of component declaration.
type ComponentDeclKind uint8

const (
	ComponentDeclKindCoreType ComponentDeclKind = 0x00
	ComponentDeclKindType     ComponentDeclKind = 0x01
	ComponentDeclKindAlias    ComponentDeclKind = 0x02
	ComponentDeclKindImport   ComponentDeclKind = 0x03
	ComponentDeclKindExport   ComponentDeclKind = 0x04
)

// ComponentDecl represents a declaration within a component type.
type ComponentDecl struct {
	Kind     ComponentDeclKind
	CoreType *CoreTypeDef
	Type     *TypeDef
	Alias    *Alias
	Import   *Import
	Export   *InstanceExport
}

// ComponentTypeDef represents a component type (0x41).
type ComponentTypeDef struct {
	Declarations []ComponentDecl
}
```

Update `TypeDef` to add:

```go
Component *ComponentTypeDef  // For 0x41
```

Create `internal/component/binary/component_type.go`:

```go
package binary

import (
	"bytes"
	"fmt"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/leb128"
)

// TypeOpComponent is the opcode for component types.
const TypeOpComponent byte = 0x41

// decodeComponentTypeDef decodes a component type definition.
// Format: 0x41 vec(componenttypedecl)
// componenttypedecl ::= 0x00 core:type      (core type)
//                     | 0x01 type           (type)
//                     | 0x02 alias          (alias)
//                     | 0x03 import         (import)
//                     | 0x04 export         (export)
func decodeComponentTypeDef(r *bytes.Reader) (*component.ComponentTypeDef, error) {
	declCount, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read declaration count: %w", err)
	}

	decls := make([]component.ComponentDecl, declCount)
	for i := uint32(0); i < declCount; i++ {
		decl, err := decodeComponentDecl(r)
		if err != nil {
			return nil, fmt.Errorf("decode declaration %d: %w", i, err)
		}
		decls[i] = decl
	}

	return &component.ComponentTypeDef{Declarations: decls}, nil
}

// decodeComponentDecl decodes a single component type declaration.
func decodeComponentDecl(r *bytes.Reader) (component.ComponentDecl, error) {
	var decl component.ComponentDecl

	kindByte, err := r.ReadByte()
	if err != nil {
		return decl, fmt.Errorf("read declaration kind: %w", err)
	}
	decl.Kind = component.ComponentDeclKind(kindByte)

	switch decl.Kind {
	case component.ComponentDeclKindCoreType:
		coreType, err := decodeCoreTypeDef(r)
		if err != nil {
			return decl, fmt.Errorf("decode core type: %w", err)
		}
		decl.CoreType = coreType

	case component.ComponentDeclKindType:
		typeDef, err := decodeTypeDef(r)
		if err != nil {
			return decl, fmt.Errorf("decode type: %w", err)
		}
		decl.Type = typeDef

	case component.ComponentDeclKindAlias:
		alias, err := decodeAliasInType(r)
		if err != nil {
			return decl, fmt.Errorf("decode alias: %w", err)
		}
		decl.Alias = alias

	case component.ComponentDeclKindImport:
		imp, err := decodeImport(r)
		if err != nil {
			return decl, fmt.Errorf("decode import: %w", err)
		}
		decl.Import = &imp

	case component.ComponentDeclKindExport:
		export, err := decodeInstanceExport(r)
		if err != nil {
			return decl, fmt.Errorf("decode export: %w", err)
		}
		decl.Export = export

	default:
		return decl, fmt.Errorf("unknown component declaration kind: 0x%02x", kindByte)
	}

	return decl, nil
}
```

Update `decoder.go` to add component type case:

```go
case TypeOpComponent:
	comp, err := decodeComponentTypeDef(r)
	if err != nil {
		return fmt.Errorf("decode component type %d: %w", i, err)
	}
	c.Types[i] = component.TypeDef{
		Kind:      component.TypeDefKindComponent,
		Component: comp,
	}
```

**Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/component/binary/... -v -run TestDecodeComponentType`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/component.go internal/component/binary/component_type.go internal/component/binary/component_type_test.go internal/component/binary/decoder.go
git commit -m "$(cat <<'EOF'
feat(binary): add component type (0x41) parsing

Implement parsing for component types which define the shape of
nested components. Supports all declaration kinds including imports.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Phase 2: Missing Type Opcodes

---

### Task 4: Add ErrorContext Primitive Type (0x64)

**Files:**
- Modify: `internal/component/binary/valtype.go:12-26`
- Modify: `internal/component/binary/types.go:140-145`
- Test: `internal/component/binary/valtype_test.go`

**Step 1: Write the failing test**

```go
// internal/component/binary/valtype_test.go (add)
func TestIsPrimValTypeErrorContext(t *testing.T) {
	if !IsPrimValType(0x64) {
		t.Error("0x64 (error-context) should be a primitive valtype")
	}
}

func TestPrimValTypeErrorContextString(t *testing.T) {
	if PrimValType(0x64).String() != "error-context" {
		t.Errorf("expected 'error-context', got %s", PrimValType(0x64).String())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/component/binary/... -v -run TestIsPrimValTypeErrorContext`
Expected: FAIL

**Step 3: Write minimal implementation**

Update `internal/component/binary/valtype.go`:

```go
const (
	PrimValTypeBool         PrimValType = 0x7f
	PrimValTypeS8           PrimValType = 0x7e
	PrimValTypeU8           PrimValType = 0x7d
	PrimValTypeS16          PrimValType = 0x7c
	PrimValTypeU16          PrimValType = 0x7b
	PrimValTypeS32          PrimValType = 0x7a
	PrimValTypeU32          PrimValType = 0x79
	PrimValTypeS64          PrimValType = 0x78
	PrimValTypeU64          PrimValType = 0x77
	PrimValTypeF32          PrimValType = 0x76
	PrimValTypeF64          PrimValType = 0x75
	PrimValTypeChar         PrimValType = 0x74
	PrimValTypeString       PrimValType = 0x73
	PrimValTypeErrorContext PrimValType = 0x64  // Add
)

func (p PrimValType) String() string {
	switch p {
	// ... existing cases ...
	case PrimValTypeErrorContext:
		return "error-context"
	default:
		return fmt.Sprintf("unknown(0x%02x)", byte(p))
	}
}

// IsPrimValType returns true if the byte is a valid primitive valtype opcode.
func IsPrimValType(b byte) bool {
	return (b >= 0x73 && b <= 0x7f) || b == 0x64
}
```

**Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/component/binary/... -v -run TestIsPrimValTypeErrorContext`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/binary/valtype.go internal/component/binary/valtype_test.go
git commit -m "$(cat <<'EOF'
feat(binary): add error-context primitive type (0x64)

Add support for the error-context primitive type used in WASI P2
error handling.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: Add Async Type Opcodes (Stream 0x66, Future 0x65, FixedSizeList 0x67)

**Files:**
- Modify: `internal/component/binary/valtype.go`
- Modify: `internal/component/binary/types.go`
- Modify: `internal/component/component.go`
- Test: `internal/component/binary/types_async_test.go`

**Step 1: Write the failing test**

```go
// internal/component/binary/types_async_test.go
package binary

import (
	"testing"
)

func TestDecodeStreamType(t *testing.T) {
	data := buildComponentWithTypeSection([]byte{
		0x66,       // stream opcode
		0x01, 0x7d, // has element type: u8
		0x00,       // no end type
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.Types[0].Stream == nil {
		t.Fatal("expected stream type def")
	}
}

func TestDecodeFutureType(t *testing.T) {
	data := buildComponentWithTypeSection([]byte{
		0x65,       // future opcode
		0x01, 0x73, // has payload: string
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.Types[0].Future == nil {
		t.Fatal("expected future type def")
	}
}

func TestDecodeFixedSizeListType(t *testing.T) {
	data := buildComponentWithTypeSection([]byte{
		0x67,       // fixed-size list opcode
		0x7d,       // element type: u8
		0x10,       // size: 16
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.Types[0].FixedSizeList == nil {
		t.Fatal("expected fixed-size list type def")
	}

	if c.Types[0].FixedSizeList.Size != 16 {
		t.Errorf("expected size 16, got %d", c.Types[0].FixedSizeList.Size)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/component/binary/... -v -run "TestDecode(Stream|Future|FixedSizeList)Type"`
Expected: FAIL

**Step 3: Write minimal implementation**

Add to `internal/component/component.go`:

```go
// StreamTypeDef represents a stream type (0x66).
type StreamTypeDef struct {
	ElementType *ValTypeRef // nil if no element type
	EndType     *ValTypeRef // nil if no end type
}

// FutureTypeDef represents a future type (0x65).
type FutureTypeDef struct {
	PayloadType *ValTypeRef // nil if no payload
}

// FixedSizeListTypeDef represents a fixed-size list type (0x67).
type FixedSizeListTypeDef struct {
	ElementType ValTypeRef
	Size        uint32
}
```

Update `TypeDef` to add:

```go
Stream        *StreamTypeDef        // For 0x66
Future        *FutureTypeDef        // For 0x65
FixedSizeList *FixedSizeListTypeDef // For 0x67
```

Add to `internal/component/binary/valtype.go`:

```go
const (
	ValTypeOpcodeStream        byte = 0x66
	ValTypeOpcodeFuture        byte = 0x65
	ValTypeOpcodeFixedSizeList byte = 0x67
)
```

Add to `internal/component/binary/types.go`:

```go
// decodeStreamTypeDef decodes a stream type definition.
// Format: 0x66 <has_element> [element_type] <has_end> [end_type]
func decodeStreamTypeDef(r *bytes.Reader) (*component.StreamTypeDef, error) {
	stream := &component.StreamTypeDef{}

	hasElement, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("read has element: %w", err)
	}
	if hasElement == 0x01 {
		elemType, err := decodeValType(r)
		if err != nil {
			return nil, fmt.Errorf("read element type: %w", err)
		}
		stream.ElementType = &elemType
	}

	hasEnd, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("read has end: %w", err)
	}
	if hasEnd == 0x01 {
		endType, err := decodeValType(r)
		if err != nil {
			return nil, fmt.Errorf("read end type: %w", err)
		}
		stream.EndType = &endType
	}

	return stream, nil
}

// decodeFutureTypeDef decodes a future type definition.
// Format: 0x65 <has_payload> [payload_type]
func decodeFutureTypeDef(r *bytes.Reader) (*component.FutureTypeDef, error) {
	future := &component.FutureTypeDef{}

	hasPayload, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("read has payload: %w", err)
	}
	if hasPayload == 0x01 {
		payloadType, err := decodeValType(r)
		if err != nil {
			return nil, fmt.Errorf("read payload type: %w", err)
		}
		future.PayloadType = &payloadType
	}

	return future, nil
}

// decodeFixedSizeListTypeDef decodes a fixed-size list type definition.
// Format: 0x67 <element_type> <size>
func decodeFixedSizeListTypeDef(r *bytes.Reader) (*component.FixedSizeListTypeDef, error) {
	elemType, err := decodeValType(r)
	if err != nil {
		return nil, fmt.Errorf("read element type: %w", err)
	}

	size, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read size: %w", err)
	}

	return &component.FixedSizeListTypeDef{
		ElementType: elemType,
		Size:        size,
	}, nil
}
```

Add cases to `decoder.go`:

```go
case ValTypeOpcodeStream:
	stream, err := decodeStreamTypeDef(r)
	if err != nil {
		return fmt.Errorf("decode stream type %d: %w", i, err)
	}
	c.Types[i] = component.TypeDef{
		Kind:   component.TypeDefKindDefined,
		Stream: stream,
	}

case ValTypeOpcodeFuture:
	future, err := decodeFutureTypeDef(r)
	if err != nil {
		return fmt.Errorf("decode future type %d: %w", i, err)
	}
	c.Types[i] = component.TypeDef{
		Kind:   component.TypeDefKindDefined,
		Future: future,
	}

case ValTypeOpcodeFixedSizeList:
	fixedList, err := decodeFixedSizeListTypeDef(r)
	if err != nil {
		return fmt.Errorf("decode fixed-size list type %d: %w", i, err)
	}
	c.Types[i] = component.TypeDef{
		Kind:          component.TypeDefKindDefined,
		FixedSizeList: fixedList,
	}
```

**Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/component/binary/... -v -run "TestDecode(Stream|Future|FixedSizeList)Type"`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/component.go internal/component/binary/valtype.go internal/component/binary/types.go internal/component/binary/decoder.go internal/component/binary/types_async_test.go
git commit -m "$(cat <<'EOF'
feat(binary): add async type opcodes (stream, future, fixed-size list)

Add support for async-related types:
- stream (0x66): Async data streams
- future (0x65): Async single values
- fixed-size list (0x67): Fixed-length lists

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Phase 3: Fix Incomplete Implementations

---

### Task 6: Implement Full Core Module Type (0x50) Parsing

**Files:**
- Modify: `internal/component/binary/core_type.go:86-89`
- Test: `internal/component/binary/core_type_test.go`

**Step 1: Write the failing test**

```go
// internal/component/binary/core_type_test.go (add)
func TestDecodeCoreModuleType(t *testing.T) {
	data := buildComponentWithSection(SectionIDCoreType, []byte{
		0x01,       // count = 1
		0x50,       // module type
		0x02,       // 2 declarations
		// Import declaration
		0x00,       // import
		0x04, 't', 'e', 's', 't', // module name
		0x03, 'f', 'o', 'o',     // import name
		0x00,                     // func kind
		0x00,                     // type index
		// Export declaration
		0x03,       // export
		0x03, 'b', 'a', 'r',     // export name
		0x00,                     // func kind
		0x00,                     // type index
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.CoreTypes[0].Module == nil {
		t.Fatal("expected core module type")
	}

	if len(c.CoreTypes[0].Module.Imports) != 1 {
		t.Errorf("expected 1 import, got %d", len(c.CoreTypes[0].Module.Imports))
	}

	if len(c.CoreTypes[0].Module.Exports) != 1 {
		t.Errorf("expected 1 export, got %d", len(c.CoreTypes[0].Module.Exports))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/component/binary/... -v -run TestDecodeCoreModuleType`
Expected: FAIL (returns empty module type)

**Step 3: Write minimal implementation**

Replace `decodeCoreModuleType` in `internal/component/binary/core_type.go`:

```go
// decodeCoreModuleType decodes a core module type.
// Format: vec(moduletypedecl)
// moduletypedecl ::= 0x00 import              (import)
//                  | 0x01 core:type           (type)
//                  | 0x02 alias               (outer alias)
//                  | 0x03 export              (export)
func decodeCoreModuleType(r *bytes.Reader) (*component.CoreModuleTypeDef, error) {
	declCount, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read declaration count: %w", err)
	}

	moduleType := &component.CoreModuleTypeDef{}

	for i := uint32(0); i < declCount; i++ {
		declKind, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("read declaration %d kind: %w", i, err)
		}

		switch declKind {
		case 0x00: // import
			moduleName, err := decodeName(r)
			if err != nil {
				return nil, fmt.Errorf("read import %d module name: %w", i, err)
			}
			importName, err := decodeName(r)
			if err != nil {
				return nil, fmt.Errorf("read import %d name: %w", i, err)
			}
			kind, err := r.ReadByte()
			if err != nil {
				return nil, fmt.Errorf("read import %d kind: %w", i, err)
			}
			// Skip type index based on kind
			if _, _, err := leb128.DecodeUint32(r); err != nil {
				return nil, fmt.Errorf("read import %d type index: %w", i, err)
			}
			moduleType.Imports = append(moduleType.Imports, component.CoreImportType{
				Module: moduleName,
				Name:   importName,
				Kind:   kind,
			})

		case 0x01: // type
			// Skip nested core type
			if _, err := decodeCoreFunc(r); err != nil {
				// Try as module type if func fails
				return nil, fmt.Errorf("read type declaration %d: %w", i, err)
			}

		case 0x02: // outer alias
			// Skip outer alias
			if _, _, err := leb128.DecodeUint32(r); err != nil { // count
				return nil, fmt.Errorf("read alias %d count: %w", i, err)
			}
			if _, _, err := leb128.DecodeUint32(r); err != nil { // index
				return nil, fmt.Errorf("read alias %d index: %w", i, err)
			}

		case 0x03: // export
			exportName, err := decodeName(r)
			if err != nil {
				return nil, fmt.Errorf("read export %d name: %w", i, err)
			}
			kind, err := r.ReadByte()
			if err != nil {
				return nil, fmt.Errorf("read export %d kind: %w", i, err)
			}
			// Skip type index
			if _, _, err := leb128.DecodeUint32(r); err != nil {
				return nil, fmt.Errorf("read export %d type index: %w", i, err)
			}
			moduleType.Exports = append(moduleType.Exports, component.CoreExportType{
				Name: exportName,
				Kind: kind,
			})

		default:
			return nil, fmt.Errorf("unknown module type declaration kind: 0x%02x", declKind)
		}
	}

	return moduleType, nil
}
```

**Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/component/binary/... -v -run TestDecodeCoreModuleType`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/binary/core_type.go internal/component/binary/core_type_test.go
git commit -m "$(cat <<'EOF'
feat(binary): implement full core module type (0x50) parsing

Replace stub implementation with full module type parsing that handles:
- Import declarations (0x00)
- Type declarations (0x01)
- Outer alias declarations (0x02)
- Export declarations (0x03)

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: Fix Export ExternDesc Parsing

**Files:**
- Modify: `internal/component/binary/exports.go:73-76`
- Modify: `internal/component/component.go` (Export struct)
- Test: `internal/component/binary/exports_test.go`

**Step 1: Write the failing test**

```go
// internal/component/binary/exports_test.go (add)
func TestDecodeExportWithExternDesc(t *testing.T) {
	data := buildComponentWithSection(SectionIDExport, []byte{
		0x01,                   // count = 1
		0x00,                   // simple name
		0x04, 't', 'e', 's', 't',
		0x01,                   // func sort
		0x00,                   // index
		0x01,                   // has extern desc
		0x01,                   // func type
		0x05,                   // type index
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.Exports[0].TypeIdx == nil {
		t.Fatal("expected type index")
	}

	if *c.Exports[0].TypeIdx != 5 {
		t.Errorf("expected type index 5, got %d", *c.Exports[0].TypeIdx)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/component/binary/... -v -run TestDecodeExportWithExternDesc`
Expected: FAIL

**Step 3: Write minimal implementation**

Update `internal/component/component.go` Export struct:

```go
// Export represents a component export.
type Export struct {
	Name    string
	Kind    ExportKind
	Idx     uint32   // Index into the appropriate index space
	TypeIdx *uint32  // Optional type annotation
}
```

Update `internal/component/binary/exports.go`:

```go
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

	// Handle core sort prefix
	if sort == SortCore {
		coreSortByte, err := r.ReadByte()
		if err != nil {
			return exp, fmt.Errorf("read core sort: %w", err)
		}
		// Map core sort to export kind
		switch coreSortByte {
		case 0x00:
			exp.Kind = component.ExportKindFunc
		case 0x01:
			exp.Kind = component.ExportKindFunc // table
		case 0x02:
			exp.Kind = component.ExportKindFunc // memory
		case 0x03:
			exp.Kind = component.ExportKindFunc // global
		default:
			exp.Kind = component.ExportKindFunc
		}
	} else {
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
		default:
			return exp, fmt.Errorf("unknown sort: 0x%02x", sort)
		}
	}

	idx, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return exp, fmt.Errorf("read index: %w", err)
	}
	exp.Idx = idx

	// Check for optional externdesc
	if r.Len() > 0 {
		hasExternDesc, err := r.ReadByte()
		if err != nil {
			return exp, fmt.Errorf("read extern desc flag: %w", err)
		}
		if hasExternDesc == 0x01 {
			// Read extern desc kind
			externKind, err := r.ReadByte()
			if err != nil {
				return exp, fmt.Errorf("read extern desc kind: %w", err)
			}
			_ = externKind // Could store this

			// Read type index
			typeIdx, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return exp, fmt.Errorf("read extern desc type index: %w", err)
			}
			exp.TypeIdx = &typeIdx
		}
	}

	return exp, nil
}
```

**Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/component/binary/... -v -run TestDecodeExportWithExternDesc`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/component.go internal/component/binary/exports.go internal/component/binary/exports_test.go
git commit -m "$(cat <<'EOF'
feat(binary): implement export externdesc parsing

Parse the optional type annotation on exports which provides type
information for linking and validation.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Phase 4: Integration Testing

---

### Task 8: Test Parsing Real WASI P2 Components

**Files:**
- Create: `internal/component/binary/wasip2_test.go`

**Step 1: Write the integration test**

```go
// internal/component/binary/wasip2_test.go
package binary

import (
	"os"
	"testing"
)

func TestParseAddWasm(t *testing.T) {
	data, err := os.ReadFile("../wasip2test/plugins/add.wasm")
	if err != nil {
		t.Skip("add.wasm not found")
	}

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("failed to parse add.wasm: %v", err)
	}

	// Verify expected structure
	if len(c.CoreModules) != 1 {
		t.Errorf("expected 1 core module, got %d", len(c.CoreModules))
	}

	if len(c.Imports) < 5 {
		t.Errorf("expected at least 5 imports, got %d", len(c.Imports))
	}

	if len(c.Exports) != 2 {
		t.Errorf("expected 2 exports, got %d", len(c.Exports))
	}

	// Check import names
	expectedImports := []string{
		"wasi:cli/environment@0.2.3",
		"wasi:cli/exit@0.2.3",
		"wasi:io/error@0.2.3",
	}
	for i, expected := range expectedImports {
		if i < len(c.Imports) && c.Imports[i].Name != expected {
			t.Errorf("import %d: expected %s, got %s", i, expected, c.Imports[i].Name)
		}
	}

	// Check export names
	foundGetPluginName := false
	foundEvaluate := false
	for _, exp := range c.Exports {
		if exp.Name == "get-plugin-name" {
			foundGetPluginName = true
		}
		if exp.Name == "evaluate" {
			foundEvaluate = true
		}
	}
	if !foundGetPluginName {
		t.Error("missing export 'get-plugin-name'")
	}
	if !foundEvaluate {
		t.Error("missing export 'evaluate'")
	}
}

func TestParseSubtractWasm(t *testing.T) {
	data, err := os.ReadFile("../wasip2test/plugins/subtract.wasm")
	if err != nil {
		t.Skip("subtract.wasm not found")
	}

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("failed to parse subtract.wasm: %v", err)
	}

	// subtract.wasm is simpler - verify basic structure
	if len(c.CoreModules) < 1 {
		t.Errorf("expected at least 1 core module, got %d", len(c.CoreModules))
	}

	if len(c.Exports) < 2 {
		t.Errorf("expected at least 2 exports, got %d", len(c.Exports))
	}
}
```

**Step 2: Run integration tests**

Run: `CGO_ENABLED=0 go test ./internal/component/binary/... -v -run "TestParse(Add|Subtract)Wasm"`
Expected: PASS if all previous tasks completed successfully

**Step 3: Commit**

```bash
git add internal/component/binary/wasip2_test.go
git commit -m "$(cat <<'EOF'
test(binary): add integration tests for WASI P2 components

Add tests that parse real-world WASI Preview 2 components (add.wasm,
subtract.wasm) to verify the binary parser handles production components.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Verification Checklist

After completing all tasks, run:

```bash
# Run all binary parser tests
CGO_ENABLED=0 go test ./internal/component/binary/... -v

# Run all component tests
CGO_ENABLED=0 go test ./internal/component/... -v

# Verify no regressions in wazero
CGO_ENABLED=0 go test ./... -short
```

**Expected Results:**
- All binary parser tests pass
- add.wasm and subtract.wasm parse successfully
- No regressions in existing tests

---

## Summary of Changes

| Task | Priority | Component | Description |
|------|----------|-----------|-------------|
| 1 | Critical | Data structures | Instance type definition types |
| 2 | Critical | Decoder | Instance type (0x42) parser |
| 3 | Critical | Decoder | Component type (0x41) parser |
| 4 | High | Valtype | ErrorContext (0x64) primitive |
| 5 | Medium | Decoder | Async types (stream, future, fixed-list) |
| 6 | High | Decoder | Core module type full implementation |
| 7 | Medium | Decoder | Export externdesc parsing |
| 8 | Critical | Test | WASI P2 integration tests |

**Total estimated implementation**: 8 tasks across 4 phases
