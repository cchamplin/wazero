# Component Model Phase 4: Full Instantiation & Linking

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Parent Plan:** [2026-01-12-component-model-implementation.md](./2026-01-12-component-model-implementation.md)
**Design Doc:** [2026-01-12-component-model-design.md](./2026-01-12-component-model-design.md)
**Previous Phase:** [Phase 3: Resources](./2026-01-12-component-model-phase3-resources.md)
**Status:** **COMPLETE** (10/10 milestones complete)
**Tasks:** 101-150

---

## Overview

This phase implements the complete component instantiation and linking system, including alias handling, import section parsing, instance section parsing, semver-compatible import resolution, and nested component support.

**Goal:** Full component instantiation with import resolution, nested components, and proper linking.

**Prerequisites:**
- Phase 1 complete (binary parser)
- Phase 2 complete (all types)
- Phase 3 complete (resources)

---

## Phase 4 Milestones

| Milestone | Description | Tasks | Success Criteria |
|-----------|-------------|-------|------------------|
| 4.1 | Alias section parsing | 101-104 | All alias kinds (export, core export, outer) parsed correctly |
| 4.2 | Import section parsing | 105-109 | Component imports with externdesc parsed |
| 4.3 | Core instance section parsing | 110-113 | Core module instantiation with args |
| 4.4 | Component instance section parsing | 114-117 | Component instantiation with args |
| 4.5 | Linker core implementation | 118-124 | Linker with DefineFunc, DefineInstance, DefineResource |
| 4.6 | Semver import matching | 125-128 | Version-compatible import resolution (wasmtime: old_import_importing_new_item) |
| 4.7 | Nested component support | 129-135 | Recursive component parsing and instantiation |
| 4.8 | Full instantiation pipeline | 136-142 | Complete Linker.Instantiate with alias resolution |
| 4.9 | Instance exports | 143-146 | Export instances with proper scoping |
| 4.10 | Integration tests | 147-150 | Real component linking tests |

---

## Reference: Binary Format Specification

### Alias Section (Section ID 6)

```
alias       ::= s:<sort> t:<aliastarget>
aliastarget ::= 0x00 i:<instanceidx> n:<name>           => export i n
              | 0x01 i:<core:instanceidx> n:<core:name> => core export i n
              | 0x02 ct:<u32> idx:<u32>                 => outer ct idx
```

**Alias target opcodes:**
| Opcode | Target |
|--------|--------|
| 0x00 | Export alias (component instance) |
| 0x01 | Core export alias |
| 0x02 | Outer alias (depth + index) |

### Import Section (Section ID 10)

```
import      ::= in:<importname'> ed:<externdesc>
importname' ::= 0x00 len:<u32> in:<importname>
              | 0x01 len:<u32> in:<importname> vs:<versionsuffix'>

externdesc  ::= 0x00 0x11 i:<core:typeidx>  => (core module (type i))
              | 0x01 i:<typeidx>            => (func (type i))
              | 0x02 b:<valuebound>         => (value b)
              | 0x03 b:<typebound>          => (type b)
              | 0x04 i:<typeidx>            => (component (type i))
              | 0x05 i:<typeidx>            => (instance (type i))
```

### Instance Section (Section ID 5)

```
instance       ::= ie:<instanceexpr>
instanceexpr   ::= 0x00 c:<componentidx> arg*:vec(<instantiatearg>)
                 | 0x01 e*:vec(<inlineexport>)

instantiatearg ::= n:<name> si:<sortidx>
sortidx        ::= sort:<sort> idx:<u32>
```

### Core Instance Section (Section ID 2)

```
core:instance       ::= ie:<core:instanceexpr>
core:instanceexpr   ::= 0x00 m:<moduleidx> arg*:vec(<core:instantiatearg>)
                      | 0x01 e*:vec(<core:inlineexport>)

core:instantiatearg ::= n:<core:name> 0x12 i:<instanceidx>
```

### Sort Byte Values

| Value | Component Sort |
|-------|---------------|
| 0x00 (+ core:sort) | Core sort |
| 0x01 | Function |
| 0x02 | Value |
| 0x03 | Type |
| 0x04 | Component |
| 0x05 | Instance |

| Value | Core Sort |
|-------|-----------|
| 0x00 | Function |
| 0x01 | Table |
| 0x02 | Memory |
| 0x03 | Global |
| 0x10 | Type |
| 0x11 | Module |
| 0x12 | Instance |

---

## Reference: Wasmtime Test Scenarios to Port

Based on wasmtime's `tests/all/component_model/` test suite:

### linker.rs Tests
| Test | Description | Port Priority |
|------|-------------|---------------|
| `old_import_importing_new_item` | Linker provides v1.0.1 to satisfy v1.0.0 import | High |
| `new_import_importing_old_item` | v1.0.0 resource satisfies v1.0.1 import | High |
| `import_both_old_and_new` | Multiple versioned resources to single component | Medium |
| `missing_import_selects_max` | Linker selects highest available version | High |
| `linker_defines_unknown_imports_as_traps` | Unknown imports become trap functions | Medium |
| `linker_fails_to_define_unknown_core_module_imports_as_traps` | Core module imports cannot be traps | Medium |

### import.rs Tests
| Test | Description | Port Priority |
|------|-------------|---------------|
| `can_compile` | Various component configs compile successfully | High |
| `simple` | Static/dynamic APIs for importing functions | High |
| `functions_in_instances` | Importing functions within instances | High |
| `attempt_to_leave_during_malloc` | Trap on exit during realloc | Medium |
| `attempt_to_reenter_during_host` | Trap on reentry during host import | Medium |

### instance.rs Tests
| Test | Description | Port Priority |
|------|-------------|---------------|
| `instance_exports` | Retrieving exports from nested instances | High |
| `export_old_get_new` | v1.0.0 export satisfies v1.0.1 request | High |
| `export_new_get_old` | v1.0.1 export satisfies v1.0.0 request | High |
| `export_missing_get_max` | Fallback to max available version | Medium |

### nested.rs Tests
| Test | Description | Port Priority |
|------|-------------|---------------|
| `top_level_instance_two_level` | Two-level nested instance hierarchy | High |
| `nested_many_instantiations` | 4-level deep nested instantiation (16 calls) | Medium |
| `thread_options_through_inner` | Threading options through nested boundaries | Low |

---

## Tasks

### Task 101: Define Sort and CoreSort Types

**Files:**
- Modify: `internal/component/component.go`
- Create: `internal/component/component_test.go`

**Step 1: Write failing test**

```go
// internal/component/component_test.go

package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestSortString(t *testing.T) {
	tests := []struct {
		sort     Sort
		expected string
	}{
		{SortFunc, "func"},
		{SortValue, "value"},
		{SortType, "type"},
		{SortComponent, "component"},
		{SortInstance, "instance"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.expected, tt.sort.String())
	}
}

func TestCoreSortString(t *testing.T) {
	tests := []struct {
		sort     CoreSort
		expected string
	}{
		{CoreSortFunc, "func"},
		{CoreSortTable, "table"},
		{CoreSortMemory, "memory"},
		{CoreSortGlobal, "global"},
		{CoreSortType, "type"},
		{CoreSortModule, "module"},
		{CoreSortInstance, "instance"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.expected, tt.sort.String())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -v -run TestSortString`
Expected: FAIL with "undefined: Sort"

**Step 3: Implement Sort and CoreSort types**

```go
// Add to internal/component/component.go

// Sort identifies the kind of component-level item.
type Sort uint8

const (
	SortCoreSort  Sort = 0x00 // Followed by CoreSort
	SortFunc      Sort = 0x01
	SortValue     Sort = 0x02
	SortType      Sort = 0x03
	SortComponent Sort = 0x04
	SortInstance  Sort = 0x05
)

func (s Sort) String() string {
	switch s {
	case SortCoreSort:
		return "core"
	case SortFunc:
		return "func"
	case SortValue:
		return "value"
	case SortType:
		return "type"
	case SortComponent:
		return "component"
	case SortInstance:
		return "instance"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

// CoreSort identifies the kind of core wasm item.
type CoreSort uint8

const (
	CoreSortFunc     CoreSort = 0x00
	CoreSortTable    CoreSort = 0x01
	CoreSortMemory   CoreSort = 0x02
	CoreSortGlobal   CoreSort = 0x03
	CoreSortType     CoreSort = 0x10
	CoreSortModule   CoreSort = 0x11
	CoreSortInstance CoreSort = 0x12
)

func (s CoreSort) String() string {
	switch s {
	case CoreSortFunc:
		return "func"
	case CoreSortTable:
		return "table"
	case CoreSortMemory:
		return "memory"
	case CoreSortGlobal:
		return "global"
	case CoreSortType:
		return "type"
	case CoreSortModule:
		return "module"
	case CoreSortInstance:
		return "instance"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -v -run TestSortString`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/component.go internal/component/component_test.go
git commit -m "$(cat <<'EOF'
feat(component): add Sort and CoreSort types for linking

Implements sort byte values used in alias, import, and instance
sections per the component model binary format spec.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 102: Define Alias Types

**Files:**
- Modify: `internal/component/component.go`
- Modify: `internal/component/component_test.go`

**Step 1: Write failing test**

```go
// Add to internal/component/component_test.go

func TestAliasKindString(t *testing.T) {
	tests := []struct {
		kind     AliasKind
		expected string
	}{
		{AliasKindExport, "export"},
		{AliasKindCoreExport, "core-export"},
		{AliasKindOuter, "outer"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.expected, tt.kind.String())
	}
}

func TestAlias_ExportAlias(t *testing.T) {
	a := Alias{
		Kind:        AliasKindExport,
		Sort:        SortFunc,
		InstanceIdx: 1,
		ExportName:  "my-func",
	}
	require.Equal(t, AliasKindExport, a.Kind)
	require.Equal(t, SortFunc, a.Sort)
	require.Equal(t, uint32(1), a.InstanceIdx)
	require.Equal(t, "my-func", a.ExportName)
}

func TestAlias_OuterAlias(t *testing.T) {
	a := Alias{
		Kind:       AliasKindOuter,
		Sort:       SortType,
		OuterCount: 1,
		OuterIndex: 5,
	}
	require.Equal(t, AliasKindOuter, a.Kind)
	require.Equal(t, uint32(1), a.OuterCount)
	require.Equal(t, uint32(5), a.OuterIndex)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -v -run TestAliasKind`
Expected: FAIL with "undefined: AliasKind"

**Step 3: Implement Alias types**

```go
// Add to internal/component/component.go

// AliasKind identifies the kind of alias.
type AliasKind uint8

const (
	AliasKindExport     AliasKind = 0x00 // export alias from component instance
	AliasKindCoreExport AliasKind = 0x01 // core export alias from core instance
	AliasKindOuter      AliasKind = 0x02 // outer alias from enclosing scope
)

func (k AliasKind) String() string {
	switch k {
	case AliasKindExport:
		return "export"
	case AliasKindCoreExport:
		return "core-export"
	case AliasKindOuter:
		return "outer"
	default:
		return fmt.Sprintf("unknown(%d)", k)
	}
}

// Alias represents an alias definition in the component.
// Aliases create references to items in other scopes.
type Alias struct {
	Kind AliasKind
	Sort Sort // What kind of item is being aliased

	// For export aliases (Kind == AliasKindExport or AliasKindCoreExport)
	InstanceIdx uint32 // Instance to alias from
	ExportName  string // Name of the export

	// For outer aliases (Kind == AliasKindOuter)
	OuterCount uint32 // Number of enclosing scopes to traverse
	OuterIndex uint32 // Index within that scope

	// For core export aliases, the core sort
	CoreSort CoreSort
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -v -run TestAlias`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/component.go internal/component/component_test.go
git commit -m "$(cat <<'EOF'
feat(component): add Alias types for alias section parsing

Alias represents export, core export, and outer alias definitions
used to reference items across component scopes.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 103: Add Aliases to Component Structure

**Files:**
- Modify: `internal/component/component.go`

**Step 1: Write failing test**

```go
// Add to internal/component/component_test.go

func TestComponent_Aliases(t *testing.T) {
	c := &Component{
		Aliases: []Alias{
			{Kind: AliasKindExport, Sort: SortFunc, InstanceIdx: 0, ExportName: "test"},
		},
	}
	require.Len(t, c.Aliases, 1)
	require.Equal(t, AliasKindExport, c.Aliases[0].Kind)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -v -run TestComponent_Aliases`
Expected: FAIL (Aliases field not defined)

**Step 3: Add Aliases field to Component**

```go
// Modify Component struct in internal/component/component.go

type Component struct {
	// CoreModules contains embedded core wasm modules (section ID 1).
	CoreModules []*wasm.Module

	// CoreModuleData contains the raw bytes of each core module.
	CoreModuleData [][]byte

	// Types contains component type definitions (section ID 7).
	Types []TypeDef

	// Canonicals contains canonical function definitions (section ID 8).
	Canonicals []CanonicalDef

	// Exports contains component exports (section ID 11).
	Exports []Export

	// Aliases contains alias definitions (section ID 6).
	Aliases []Alias
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -v -run TestComponent_Aliases`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/component.go internal/component/component_test.go
git commit -m "$(cat <<'EOF'
feat(component): add Aliases field to Component structure

Components can now store alias definitions parsed from section ID 6.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 104: Implement Alias Section Decoder

**Files:**
- Modify: `internal/component/binary/decoder.go`
- Create: `internal/component/binary/alias.go`
- Create: `internal/component/binary/alias_test.go`

**Step 1: Write failing test**

```go
// internal/component/binary/alias_test.go

package binary

import (
	"bytes"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestDecodeAlias_ExportAlias(t *testing.T) {
	// sort=func(0x01), target=export(0x00), instance=0, name="test"
	data := []byte{
		0x01,                         // sort: func
		0x00,                         // target: export
		0x00,                         // instance index
		0x04, 't', 'e', 's', 't',     // name: "test"
	}

	alias, err := decodeAlias(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.AliasKindExport, alias.Kind)
	require.Equal(t, component.SortFunc, alias.Sort)
	require.Equal(t, uint32(0), alias.InstanceIdx)
	require.Equal(t, "test", alias.ExportName)
}

func TestDecodeAlias_CoreExportAlias(t *testing.T) {
	// sort=core(0x00)+func(0x00), target=core-export(0x01), instance=1, name="mem"
	data := []byte{
		0x00,                         // sort: core
		0x02,                         // core sort: memory
		0x01,                         // target: core export
		0x01,                         // instance index
		0x03, 'm', 'e', 'm',          // name: "mem"
	}

	alias, err := decodeAlias(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.AliasKindCoreExport, alias.Kind)
	require.Equal(t, component.CoreSortMemory, alias.CoreSort)
	require.Equal(t, uint32(1), alias.InstanceIdx)
	require.Equal(t, "mem", alias.ExportName)
}

func TestDecodeAlias_OuterAlias(t *testing.T) {
	// sort=type(0x03), target=outer(0x02), count=1, index=5
	data := []byte{
		0x03,                         // sort: type
		0x02,                         // target: outer
		0x01,                         // outer count
		0x05,                         // outer index
	}

	alias, err := decodeAlias(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.AliasKindOuter, alias.Kind)
	require.Equal(t, component.SortType, alias.Sort)
	require.Equal(t, uint32(1), alias.OuterCount)
	require.Equal(t, uint32(5), alias.OuterIndex)
}

func TestDecodeAliasSection(t *testing.T) {
	// Section with 2 aliases
	data := []byte{
		0x02,                         // count: 2
		// Alias 1: export func from instance 0
		0x01, 0x00, 0x00,
		0x04, 't', 'e', 's', 't',
		// Alias 2: outer type from depth 1, index 2
		0x03, 0x02, 0x01, 0x02,
	}

	c := &component.Component{}
	err := decodeAliasSection(c, bytes.NewReader(data))
	require.NoError(t, err)
	require.Len(t, c.Aliases, 2)
	require.Equal(t, component.AliasKindExport, c.Aliases[0].Kind)
	require.Equal(t, component.AliasKindOuter, c.Aliases[1].Kind)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/binary/... -v -run TestDecodeAlias`
Expected: FAIL with "undefined: decodeAlias"

**Step 3: Implement alias decoding**

```go
// internal/component/binary/alias.go

package binary

import (
	"bytes"
	"fmt"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/leb128"
)

// decodeAlias parses a single alias definition.
// Format: sort aliastarget
// aliastarget ::= 0x00 instanceidx name       (export)
//              | 0x01 core:instanceidx name  (core export)
//              | 0x02 count idx              (outer)
func decodeAlias(r *bytes.Reader) (component.Alias, error) {
	var alias component.Alias

	// Read sort byte
	sortByte, err := r.ReadByte()
	if err != nil {
		return alias, fmt.Errorf("reading sort: %w", err)
	}

	// Handle core sort prefix
	if sortByte == 0x00 {
		coreSortByte, err := r.ReadByte()
		if err != nil {
			return alias, fmt.Errorf("reading core sort: %w", err)
		}
		alias.CoreSort = component.CoreSort(coreSortByte)
		alias.Sort = component.SortCoreSort
	} else {
		alias.Sort = component.Sort(sortByte)
	}

	// Read alias target
	targetByte, err := r.ReadByte()
	if err != nil {
		return alias, fmt.Errorf("reading alias target: %w", err)
	}

	switch targetByte {
	case 0x00: // export alias
		alias.Kind = component.AliasKindExport
		alias.InstanceIdx, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return alias, fmt.Errorf("reading instance index: %w", err)
		}
		alias.ExportName, err = decodeName(r)
		if err != nil {
			return alias, fmt.Errorf("reading export name: %w", err)
		}

	case 0x01: // core export alias
		alias.Kind = component.AliasKindCoreExport
		alias.InstanceIdx, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return alias, fmt.Errorf("reading core instance index: %w", err)
		}
		alias.ExportName, err = decodeName(r)
		if err != nil {
			return alias, fmt.Errorf("reading core export name: %w", err)
		}

	case 0x02: // outer alias
		alias.Kind = component.AliasKindOuter
		alias.OuterCount, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return alias, fmt.Errorf("reading outer count: %w", err)
		}
		alias.OuterIndex, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return alias, fmt.Errorf("reading outer index: %w", err)
		}

	default:
		return alias, fmt.Errorf("unknown alias target: 0x%02x", targetByte)
	}

	return alias, nil
}

// decodeAliasSection parses the alias section (section ID 6).
func decodeAliasSection(c *component.Component, r *bytes.Reader) error {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("reading alias count: %w", err)
	}

	c.Aliases = make([]component.Alias, count)
	for i := uint32(0); i < count; i++ {
		alias, err := decodeAlias(r)
		if err != nil {
			return fmt.Errorf("decoding alias %d: %w", i, err)
		}
		c.Aliases[i] = alias
	}

	return nil
}
```

**Step 4: Integrate into decoder.go**

```go
// Add to switch in DecodeComponent in internal/component/binary/decoder.go

case SectionIDAlias:
	if err := decodeAliasSection(c, bytes.NewReader(sectionContent)); err != nil {
		return nil, fmt.Errorf("section %s: %w", sectionID, err)
	}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/component/binary/... -v -run TestDecodeAlias`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/component/binary/alias.go internal/component/binary/alias_test.go internal/component/binary/decoder.go
git commit -m "$(cat <<'EOF'
feat(component): implement alias section decoder

Parses alias section (ID 6) with export, core export, and outer
alias kinds per the component model binary format specification.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 105: Define Import Types

**Files:**
- Modify: `internal/component/component.go`
- Modify: `internal/component/component_test.go`

**Step 1: Write failing test**

```go
// Add to internal/component/component_test.go

func TestImportExternDescKind(t *testing.T) {
	tests := []struct {
		kind     ImportExternDescKind
		expected string
	}{
		{ImportExternDescCoreModule, "core-module"},
		{ImportExternDescFunc, "func"},
		{ImportExternDescValue, "value"},
		{ImportExternDescType, "type"},
		{ImportExternDescComponent, "component"},
		{ImportExternDescInstance, "instance"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.expected, tt.kind.String())
	}
}

func TestImport_FuncImport(t *testing.T) {
	imp := Import{
		Name: "wasi:cli/environment@0.2.0",
		ExternDesc: ImportExternDesc{
			Kind:    ImportExternDescFunc,
			TypeIdx: 5,
		},
	}
	require.Equal(t, "wasi:cli/environment@0.2.0", imp.Name)
	require.Equal(t, ImportExternDescFunc, imp.ExternDesc.Kind)
	require.Equal(t, uint32(5), imp.ExternDesc.TypeIdx)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -v -run TestImport`
Expected: FAIL with "undefined: ImportExternDescKind"

**Step 3: Implement Import types**

```go
// Add to internal/component/component.go

// ImportExternDescKind identifies the kind of imported item.
type ImportExternDescKind uint8

const (
	ImportExternDescCoreModule ImportExternDescKind = 0x00
	ImportExternDescFunc       ImportExternDescKind = 0x01
	ImportExternDescValue      ImportExternDescKind = 0x02
	ImportExternDescType       ImportExternDescKind = 0x03
	ImportExternDescComponent  ImportExternDescKind = 0x04
	ImportExternDescInstance   ImportExternDescKind = 0x05
)

func (k ImportExternDescKind) String() string {
	switch k {
	case ImportExternDescCoreModule:
		return "core-module"
	case ImportExternDescFunc:
		return "func"
	case ImportExternDescValue:
		return "value"
	case ImportExternDescType:
		return "type"
	case ImportExternDescComponent:
		return "component"
	case ImportExternDescInstance:
		return "instance"
	default:
		return fmt.Sprintf("unknown(%d)", k)
	}
}

// ImportExternDesc describes the type of an import.
type ImportExternDesc struct {
	Kind ImportExternDescKind

	// For func, component, instance: type index
	TypeIdx uint32

	// For core module: core type index (after 0x11 prefix)
	CoreTypeIdx uint32

	// For value: value bound
	// For type: type bound
}

// Import represents a component import.
type Import struct {
	Name       string           // Import name (kebab-name with optional version)
	ExternDesc ImportExternDesc // What is being imported
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -v -run TestImport`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/component.go internal/component/component_test.go
git commit -m "$(cat <<'EOF'
feat(component): add Import types for import section

Import and ImportExternDesc represent component import declarations
with their extern descriptors (func, value, type, component, instance).

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 106: Add Imports to Component Structure

**Files:**
- Modify: `internal/component/component.go`

**Step 1: Write failing test**

```go
// Add to internal/component/component_test.go

func TestComponent_Imports(t *testing.T) {
	c := &Component{
		Imports: []Import{
			{
				Name: "wasi:io/streams@0.2.0",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescInstance,
					TypeIdx: 3,
				},
			},
		},
	}
	require.Len(t, c.Imports, 1)
	require.Equal(t, "wasi:io/streams@0.2.0", c.Imports[0].Name)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -v -run TestComponent_Imports`
Expected: FAIL (Imports field not defined)

**Step 3: Add Imports field to Component**

```go
// Modify Component struct in internal/component/component.go

type Component struct {
	CoreModules    []*wasm.Module
	CoreModuleData [][]byte
	Types          []TypeDef
	Canonicals     []CanonicalDef
	Exports        []Export
	Aliases        []Alias

	// Imports contains component imports (section ID 10).
	Imports []Import
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -v -run TestComponent_Imports`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/component.go internal/component/component_test.go
git commit -m "$(cat <<'EOF'
feat(component): add Imports field to Component structure

Components can now store import declarations parsed from section ID 10.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 107: Implement Import Name Parsing

**Files:**
- Create: `internal/component/binary/import.go`
- Create: `internal/component/binary/import_test.go`

**Step 1: Write failing test**

```go
// internal/component/binary/import_test.go

package binary

import (
	"bytes"
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestDecodeImportName_Plain(t *testing.T) {
	// 0x00 prefix = plain name without version suffix
	data := []byte{
		0x00,                           // plain name
		0x08,                           // length
		't', 'e', 's', 't', '-', 'a', 'p', 'i',
	}

	name, err := decodeImportName(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, "test-api", name)
}

func TestDecodeImportName_WithVersion(t *testing.T) {
	// 0x01 prefix = name with version suffix
	data := []byte{
		0x01,                           // with version suffix
		0x14,                           // total length
		'w', 'a', 's', 'i', ':', 'c', 'l', 'i', '/', 'e', 'n', 'v', '@', '0', '.', '2', '.', '0', '-', 'r', 'c', '1',
	}
	// Actually let's use simpler test
	data = []byte{
		0x01,                           // with version suffix
		0x0c,                           // length
		't', 'e', 's', 't', '@', '1', '.', '0', '.', '0', '-', 'a',
	}
	// Let's use even simpler:
	data = []byte{
		0x01,                           // with version suffix
		0x0a,                           // length = 10
		't', 'e', 's', 't', '@', '1', '.', '2', '.', '3',
	}

	name, err := decodeImportName(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, "test@1.2.3", name)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/binary/... -v -run TestDecodeImportName`
Expected: FAIL with "undefined: decodeImportName"

**Step 3: Implement import name decoding**

```go
// internal/component/binary/import.go

package binary

import (
	"bytes"
	"fmt"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/leb128"
)

// decodeImportName decodes an import name with optional version suffix.
// Format: 0x00 len name       (plain name)
//       | 0x01 len name       (name with version suffix embedded)
func decodeImportName(r *bytes.Reader) (string, error) {
	prefix, err := r.ReadByte()
	if err != nil {
		return "", fmt.Errorf("reading import name prefix: %w", err)
	}

	switch prefix {
	case 0x00, 0x01:
		// Both cases: read length-prefixed name
		// The version suffix is embedded in the name string itself
		return decodeName(r)
	default:
		return "", fmt.Errorf("unknown import name prefix: 0x%02x", prefix)
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/binary/... -v -run TestDecodeImportName`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/binary/import.go internal/component/binary/import_test.go
git commit -m "$(cat <<'EOF'
feat(component): implement import name decoder

Parses import names with optional version suffixes.
Format supports both plain names (0x00) and versioned names (0x01).

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 108: Implement Import ExternDesc Parsing

**Files:**
- Modify: `internal/component/binary/import.go`
- Modify: `internal/component/binary/import_test.go`

**Step 1: Write failing test**

```go
// Add to internal/component/binary/import_test.go

func TestDecodeExternDesc_Func(t *testing.T) {
	// 0x01 = func, then type index
	data := []byte{0x01, 0x05}

	desc, err := decodeExternDesc(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.ImportExternDescFunc, desc.Kind)
	require.Equal(t, uint32(5), desc.TypeIdx)
}

func TestDecodeExternDesc_CoreModule(t *testing.T) {
	// 0x00 0x11 = core module, then core type index
	data := []byte{0x00, 0x11, 0x02}

	desc, err := decodeExternDesc(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.ImportExternDescCoreModule, desc.Kind)
	require.Equal(t, uint32(2), desc.CoreTypeIdx)
}

func TestDecodeExternDesc_Instance(t *testing.T) {
	// 0x05 = instance, then type index
	data := []byte{0x05, 0x03}

	desc, err := decodeExternDesc(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.ImportExternDescInstance, desc.Kind)
	require.Equal(t, uint32(3), desc.TypeIdx)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/binary/... -v -run TestDecodeExternDesc`
Expected: FAIL with "undefined: decodeExternDesc"

**Step 3: Implement extern desc decoding**

```go
// Add to internal/component/binary/import.go

// decodeExternDesc decodes an import extern descriptor.
// Format: 0x00 0x11 core:typeidx  (core module)
//       | 0x01 typeidx            (func)
//       | 0x02 valuebound         (value)
//       | 0x03 typebound          (type)
//       | 0x04 typeidx            (component)
//       | 0x05 typeidx            (instance)
func decodeExternDesc(r *bytes.Reader) (component.ImportExternDesc, error) {
	var desc component.ImportExternDesc

	kindByte, err := r.ReadByte()
	if err != nil {
		return desc, fmt.Errorf("reading externdesc kind: %w", err)
	}

	switch kindByte {
	case 0x00:
		// Core module: expect 0x11 prefix then core type index
		prefix, err := r.ReadByte()
		if err != nil {
			return desc, fmt.Errorf("reading core module prefix: %w", err)
		}
		if prefix != 0x11 {
			return desc, fmt.Errorf("expected 0x11 for core module, got 0x%02x", prefix)
		}
		desc.Kind = component.ImportExternDescCoreModule
		desc.CoreTypeIdx, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return desc, fmt.Errorf("reading core type index: %w", err)
		}

	case 0x01:
		desc.Kind = component.ImportExternDescFunc
		desc.TypeIdx, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return desc, fmt.Errorf("reading func type index: %w", err)
		}

	case 0x02:
		desc.Kind = component.ImportExternDescValue
		// TODO: decode valuebound
		return desc, fmt.Errorf("value imports not yet supported")

	case 0x03:
		desc.Kind = component.ImportExternDescType
		// TODO: decode typebound
		return desc, fmt.Errorf("type imports not yet supported")

	case 0x04:
		desc.Kind = component.ImportExternDescComponent
		desc.TypeIdx, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return desc, fmt.Errorf("reading component type index: %w", err)
		}

	case 0x05:
		desc.Kind = component.ImportExternDescInstance
		desc.TypeIdx, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return desc, fmt.Errorf("reading instance type index: %w", err)
		}

	default:
		return desc, fmt.Errorf("unknown externdesc kind: 0x%02x", kindByte)
	}

	return desc, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/binary/... -v -run TestDecodeExternDesc`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/binary/import.go internal/component/binary/import_test.go
git commit -m "$(cat <<'EOF'
feat(component): implement import extern descriptor decoder

Parses externdesc for imports including core module, func,
component, and instance kinds with their type indices.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 109: Implement Import Section Decoder

**Files:**
- Modify: `internal/component/binary/import.go`
- Modify: `internal/component/binary/import_test.go`
- Modify: `internal/component/binary/decoder.go`

**Step 1: Write failing test**

```go
// Add to internal/component/binary/import_test.go

func TestDecodeImportSection(t *testing.T) {
	// Section with 2 imports
	data := []byte{
		0x02,                                     // count: 2
		// Import 1: plain func import
		0x00, 0x04, 't', 'e', 's', 't',           // name
		0x01, 0x00,                               // func type 0
		// Import 2: versioned instance import
		0x01, 0x0a, 'm', 'y', '-', 'i', 'n', 's', 't', '@', '1', '.', '0', // name (10 chars)
		// Simplified - just do plain names
	}
	data = []byte{
		0x02,                                     // count: 2
		// Import 1: func
		0x00, 0x04, 't', 'e', 's', 't',           // name
		0x01, 0x00,                               // func type 0
		// Import 2: instance
		0x00, 0x05, 'o', 't', 'h', 'e', 'r',      // name
		0x05, 0x01,                               // instance type 1
	}

	c := &component.Component{}
	err := decodeImportSection(c, bytes.NewReader(data))
	require.NoError(t, err)
	require.Len(t, c.Imports, 2)
	require.Equal(t, "test", c.Imports[0].Name)
	require.Equal(t, component.ImportExternDescFunc, c.Imports[0].ExternDesc.Kind)
	require.Equal(t, "other", c.Imports[1].Name)
	require.Equal(t, component.ImportExternDescInstance, c.Imports[1].ExternDesc.Kind)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/binary/... -v -run TestDecodeImportSection`
Expected: FAIL with "undefined: decodeImportSection"

**Step 3: Implement import section decoding**

```go
// Add to internal/component/binary/import.go

// decodeImport decodes a single import.
func decodeImport(r *bytes.Reader) (component.Import, error) {
	var imp component.Import

	name, err := decodeImportName(r)
	if err != nil {
		return imp, fmt.Errorf("decoding import name: %w", err)
	}
	imp.Name = name

	desc, err := decodeExternDesc(r)
	if err != nil {
		return imp, fmt.Errorf("decoding externdesc: %w", err)
	}
	imp.ExternDesc = desc

	return imp, nil
}

// decodeImportSection parses the import section (section ID 10).
func decodeImportSection(c *component.Component, r *bytes.Reader) error {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("reading import count: %w", err)
	}

	c.Imports = make([]component.Import, count)
	for i := uint32(0); i < count; i++ {
		imp, err := decodeImport(r)
		if err != nil {
			return fmt.Errorf("decoding import %d: %w", i, err)
		}
		c.Imports[i] = imp
	}

	return nil
}
```

**Step 4: Integrate into decoder.go**

```go
// Add to switch in DecodeComponent in internal/component/binary/decoder.go

case SectionIDImport:
	if err := decodeImportSection(c, bytes.NewReader(sectionContent)); err != nil {
		return nil, fmt.Errorf("section %s: %w", sectionID, err)
	}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/component/binary/... -v -run TestDecodeImportSection`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/component/binary/import.go internal/component/binary/import_test.go internal/component/binary/decoder.go
git commit -m "$(cat <<'EOF'
feat(component): implement import section decoder

Parses import section (ID 10) with import names and extern descriptors.
Supports func, instance, component, and core module imports.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 110: Define CoreInstance Types

**Files:**
- Modify: `internal/component/component.go`
- Modify: `internal/component/component_test.go`

**Step 1: Write failing test**

```go
// Add to internal/component/component_test.go

func TestCoreInstanceExprKind(t *testing.T) {
	tests := []struct {
		kind     CoreInstanceExprKind
		expected string
	}{
		{CoreInstanceExprInstantiate, "instantiate"},
		{CoreInstanceExprInline, "inline"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.expected, tt.kind.String())
	}
}

func TestCoreInstance_Instantiate(t *testing.T) {
	ci := CoreInstance{
		Kind:      CoreInstanceExprInstantiate,
		ModuleIdx: 0,
		Args: []CoreInstantiateArg{
			{Name: "memory", InstanceIdx: 1},
		},
	}
	require.Equal(t, CoreInstanceExprInstantiate, ci.Kind)
	require.Equal(t, uint32(0), ci.ModuleIdx)
	require.Len(t, ci.Args, 1)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -v -run TestCoreInstance`
Expected: FAIL with "undefined: CoreInstanceExprKind"

**Step 3: Implement CoreInstance types**

```go
// Add to internal/component/component.go

// CoreInstanceExprKind identifies how a core instance is created.
type CoreInstanceExprKind uint8

const (
	CoreInstanceExprInstantiate CoreInstanceExprKind = 0x00 // Instantiate a module
	CoreInstanceExprInline      CoreInstanceExprKind = 0x01 // Inline exports
)

func (k CoreInstanceExprKind) String() string {
	switch k {
	case CoreInstanceExprInstantiate:
		return "instantiate"
	case CoreInstanceExprInline:
		return "inline"
	default:
		return fmt.Sprintf("unknown(%d)", k)
	}
}

// CoreInstantiateArg is an argument for core module instantiation.
type CoreInstantiateArg struct {
	Name        string   // Import name
	InstanceIdx uint32   // Instance to import from (prefixed with 0x12)
}

// CoreInlineExport is an inline export for a core instance.
type CoreInlineExport struct {
	Name     string
	Sort     CoreSort
	Idx      uint32
}

// CoreInstance represents a core instance definition (section ID 2).
type CoreInstance struct {
	Kind CoreInstanceExprKind

	// For Instantiate
	ModuleIdx uint32
	Args      []CoreInstantiateArg

	// For Inline
	InlineExports []CoreInlineExport
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -v -run TestCoreInstance`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/component.go internal/component/component_test.go
git commit -m "$(cat <<'EOF'
feat(component): add CoreInstance types for core instance section

CoreInstance represents core module instantiation with args
or inline exports per section ID 2 spec.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 111: Add CoreInstances to Component

**Files:**
- Modify: `internal/component/component.go`

**Step 1: Write failing test**

```go
// Add to internal/component/component_test.go

func TestComponent_CoreInstances(t *testing.T) {
	c := &Component{
		CoreInstances: []CoreInstance{
			{Kind: CoreInstanceExprInstantiate, ModuleIdx: 0},
		},
	}
	require.Len(t, c.CoreInstances, 1)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -v -run TestComponent_CoreInstances`
Expected: FAIL (CoreInstances field not defined)

**Step 3: Add CoreInstances field**

```go
// Modify Component struct in internal/component/component.go

type Component struct {
	CoreModules    []*wasm.Module
	CoreModuleData [][]byte
	Types          []TypeDef
	Canonicals     []CanonicalDef
	Exports        []Export
	Aliases        []Alias
	Imports        []Import

	// CoreInstances contains core instance definitions (section ID 2).
	CoreInstances []CoreInstance
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -v -run TestComponent_CoreInstances`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/component.go internal/component/component_test.go
git commit -m "$(cat <<'EOF'
feat(component): add CoreInstances field to Component

Components can now store core instance definitions from section ID 2.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 112: Implement Core Instance Section Decoder

**Files:**
- Create: `internal/component/binary/core_instance.go`
- Create: `internal/component/binary/core_instance_test.go`

**Step 1: Write failing test**

```go
// internal/component/binary/core_instance_test.go

package binary

import (
	"bytes"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestDecodeCoreInstance_Instantiate(t *testing.T) {
	// 0x00 = instantiate, module 0, 1 arg
	data := []byte{
		0x00,                         // instantiate
		0x00,                         // module index 0
		0x01,                         // 1 arg
		0x03, 'm', 'e', 'm',          // arg name "mem"
		0x12,                         // instance sort
		0x00,                         // instance index 0
	}

	ci, err := decodeCoreInstance(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.CoreInstanceExprInstantiate, ci.Kind)
	require.Equal(t, uint32(0), ci.ModuleIdx)
	require.Len(t, ci.Args, 1)
	require.Equal(t, "mem", ci.Args[0].Name)
	require.Equal(t, uint32(0), ci.Args[0].InstanceIdx)
}

func TestDecodeCoreInstance_Inline(t *testing.T) {
	// 0x01 = inline exports
	data := []byte{
		0x01,                         // inline
		0x02,                         // 2 exports
		0x04, 't', 'e', 's', 't',     // name "test"
		0x00,                         // sort: func
		0x05,                         // index 5
		0x03, 'm', 'e', 'm',          // name "mem"
		0x02,                         // sort: memory
		0x01,                         // index 1
	}

	ci, err := decodeCoreInstance(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.CoreInstanceExprInline, ci.Kind)
	require.Len(t, ci.InlineExports, 2)
	require.Equal(t, "test", ci.InlineExports[0].Name)
	require.Equal(t, component.CoreSortFunc, ci.InlineExports[0].Sort)
}

func TestDecodeCoreInstanceSection(t *testing.T) {
	// Section with 1 core instance
	data := []byte{
		0x01,                         // count: 1
		0x00,                         // instantiate
		0x00,                         // module 0
		0x00,                         // 0 args
	}

	c := &component.Component{}
	err := decodeCoreInstanceSection(c, bytes.NewReader(data))
	require.NoError(t, err)
	require.Len(t, c.CoreInstances, 1)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/binary/... -v -run TestDecodeCoreInstance`
Expected: FAIL with "undefined: decodeCoreInstance"

**Step 3: Implement core instance decoding**

```go
// internal/component/binary/core_instance.go

package binary

import (
	"bytes"
	"fmt"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/leb128"
)

// decodeCoreInstance decodes a single core instance definition.
func decodeCoreInstance(r *bytes.Reader) (component.CoreInstance, error) {
	var ci component.CoreInstance

	kindByte, err := r.ReadByte()
	if err != nil {
		return ci, fmt.Errorf("reading core instance kind: %w", err)
	}
	ci.Kind = component.CoreInstanceExprKind(kindByte)

	switch ci.Kind {
	case component.CoreInstanceExprInstantiate:
		ci.ModuleIdx, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return ci, fmt.Errorf("reading module index: %w", err)
		}

		argCount, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return ci, fmt.Errorf("reading arg count: %w", err)
		}

		ci.Args = make([]component.CoreInstantiateArg, argCount)
		for i := uint32(0); i < argCount; i++ {
			name, err := decodeName(r)
			if err != nil {
				return ci, fmt.Errorf("reading arg %d name: %w", i, err)
			}

			// Read sort byte (must be 0x12 for instance)
			sortByte, err := r.ReadByte()
			if err != nil {
				return ci, fmt.Errorf("reading arg %d sort: %w", i, err)
			}
			if sortByte != 0x12 {
				return ci, fmt.Errorf("expected instance sort 0x12, got 0x%02x", sortByte)
			}

			instanceIdx, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return ci, fmt.Errorf("reading arg %d instance index: %w", i, err)
			}

			ci.Args[i] = component.CoreInstantiateArg{
				Name:        name,
				InstanceIdx: instanceIdx,
			}
		}

	case component.CoreInstanceExprInline:
		exportCount, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return ci, fmt.Errorf("reading inline export count: %w", err)
		}

		ci.InlineExports = make([]component.CoreInlineExport, exportCount)
		for i := uint32(0); i < exportCount; i++ {
			name, err := decodeName(r)
			if err != nil {
				return ci, fmt.Errorf("reading export %d name: %w", i, err)
			}

			sortByte, err := r.ReadByte()
			if err != nil {
				return ci, fmt.Errorf("reading export %d sort: %w", i, err)
			}

			idx, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return ci, fmt.Errorf("reading export %d index: %w", i, err)
			}

			ci.InlineExports[i] = component.CoreInlineExport{
				Name: name,
				Sort: component.CoreSort(sortByte),
				Idx:  idx,
			}
		}

	default:
		return ci, fmt.Errorf("unknown core instance kind: 0x%02x", kindByte)
	}

	return ci, nil
}

// decodeCoreInstanceSection parses the core instance section (section ID 2).
func decodeCoreInstanceSection(c *component.Component, r *bytes.Reader) error {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("reading core instance count: %w", err)
	}

	c.CoreInstances = make([]component.CoreInstance, count)
	for i := uint32(0); i < count; i++ {
		ci, err := decodeCoreInstance(r)
		if err != nil {
			return fmt.Errorf("decoding core instance %d: %w", i, err)
		}
		c.CoreInstances[i] = ci
	}

	return nil
}
```

**Step 4: Integrate into decoder.go**

```go
// Add to switch in DecodeComponent in internal/component/binary/decoder.go

case SectionIDCoreInstance:
	if err := decodeCoreInstanceSection(c, bytes.NewReader(sectionContent)); err != nil {
		return nil, fmt.Errorf("section %s: %w", sectionID, err)
	}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/component/binary/... -v -run TestDecodeCoreInstance`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/component/binary/core_instance.go internal/component/binary/core_instance_test.go internal/component/binary/decoder.go
git commit -m "$(cat <<'EOF'
feat(component): implement core instance section decoder

Parses core instance section (ID 2) with instantiate and inline
export variants. Supports core module instantiation with instance args.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 113-117: Component Instance Section (Similar pattern to Tasks 110-112)

Tasks 113-117 follow the same TDD pattern for component instances:
- Task 113: Define ComponentInstance types
- Task 114: Add ComponentInstances to Component
- Task 115: Implement instance argument decoding
- Task 116: Implement component instance section decoder
- Task 117: Integration test for instance section

---

### Task 118: Create Linker Structure

**Files:**
- Create: `internal/component/linker.go`
- Create: `internal/component/linker_test.go`

**Step 1: Write failing test**

```go
// internal/component/linker_test.go

package component

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestNewLinker(t *testing.T) {
	l := NewLinker()
	require.NotNil(t, l)
	require.NotNil(t, l.definitions)
}

func TestLinker_DefineFunc(t *testing.T) {
	l := NewLinker()

	funcType := &FuncType{
		Params:  []NamedValType{{Name: "x", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7f}}},
		Results: []NamedValType{{Name: "", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7f}}},
	}

	err := l.DefineFunc("test:api", "add", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return []Val{ValS32(42)}, nil
	})
	require.NoError(t, err)

	// Check it was added
	def, ok := l.definitions["test:api/add"]
	require.True(t, ok)
	require.IsType(t, &FuncDef{}, def)
}

func TestLinker_DefineFunc_Duplicate(t *testing.T) {
	l := NewLinker()
	funcType := &FuncType{}

	err := l.DefineFunc("test", "fn", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	// Duplicate should error
	err = l.DefineFunc("test", "fn", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return nil, nil
	})
	require.Error(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -v -run TestNewLinker`
Expected: FAIL with "undefined: NewLinker"

**Step 3: Implement Linker**

```go
// internal/component/linker.go

package component

import (
	"context"
	"fmt"
)

// HostFunc is a host function that can be called from a component.
type HostFunc func(ctx context.Context, args []Val) ([]Val, error)

// Definition is an item that can satisfy a component import.
type Definition interface {
	definition()
}

// FuncDef is a function definition.
type FuncDef struct {
	Type     *FuncType
	Callback HostFunc
}

func (*FuncDef) definition() {}

// InstanceDef is an instance definition with multiple exports.
type InstanceDef struct {
	Exports map[string]Definition
}

func (*InstanceDef) definition() {}

// ResourceDef is a resource type definition.
type ResourceDef struct {
	Destructor func(rep uint32)
}

func (*ResourceDef) definition() {}

// Linker resolves component imports and instantiates components.
type Linker struct {
	definitions map[string]Definition
}

// NewLinker creates a new component linker.
func NewLinker() *Linker {
	return &Linker{
		definitions: make(map[string]Definition),
	}
}

// DefineFunc adds a host function definition.
func (l *Linker) DefineFunc(namespace, name string, typ *FuncType, fn HostFunc) error {
	key := namespace + "/" + name
	if _, exists := l.definitions[key]; exists {
		return fmt.Errorf("definition already exists: %s", key)
	}
	l.definitions[key] = &FuncDef{Type: typ, Callback: fn}
	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -v -run TestNewLinker`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/linker.go internal/component/linker_test.go
git commit -m "$(cat <<'EOF'
feat(component): implement Linker with DefineFunc

Linker provides import resolution for component instantiation.
DefineFunc registers host functions to satisfy component imports.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 119: Implement Linker.DefineInstance

**Files:**
- Modify: `internal/component/linker.go`
- Modify: `internal/component/linker_test.go`

**Step 1: Write failing test**

```go
// Add to internal/component/linker_test.go

func TestLinker_DefineInstance(t *testing.T) {
	l := NewLinker()

	funcType := &FuncType{}

	err := l.DefineInstance("wasi:io/streams@0.2.0").
		Func("read", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
			return nil, nil
		}).
		Func("write", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
			return nil, nil
		}).
		Build()
	require.NoError(t, err)

	// Check it was added
	def, ok := l.definitions["wasi:io/streams@0.2.0"]
	require.True(t, ok)
	instDef, ok := def.(*InstanceDef)
	require.True(t, ok)
	require.Len(t, instDef.Exports, 2)
	require.Contains(t, instDef.Exports, "read")
	require.Contains(t, instDef.Exports, "write")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -v -run TestLinker_DefineInstance`
Expected: FAIL (DefineInstance method not found)

**Step 3: Implement DefineInstance**

```go
// Add to internal/component/linker.go

// InstanceBuilder builds an instance definition with multiple exports.
type InstanceBuilder struct {
	linker    *Linker
	namespace string
	exports   map[string]Definition
}

// DefineInstance starts building an instance definition.
func (l *Linker) DefineInstance(namespace string) *InstanceBuilder {
	return &InstanceBuilder{
		linker:    l,
		namespace: namespace,
		exports:   make(map[string]Definition),
	}
}

// Func adds a function export to the instance.
func (b *InstanceBuilder) Func(name string, typ *FuncType, fn HostFunc) *InstanceBuilder {
	b.exports[name] = &FuncDef{Type: typ, Callback: fn}
	return b
}

// Build finalizes the instance definition and registers it with the linker.
func (b *InstanceBuilder) Build() error {
	if _, exists := b.linker.definitions[b.namespace]; exists {
		return fmt.Errorf("definition already exists: %s", b.namespace)
	}
	b.linker.definitions[b.namespace] = &InstanceDef{Exports: b.exports}
	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -v -run TestLinker_DefineInstance`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/linker.go internal/component/linker_test.go
git commit -m "$(cat <<'EOF'
feat(component): implement Linker.DefineInstance builder

InstanceBuilder provides fluent API for defining instance exports.
Supports adding multiple function exports to a single namespace.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 120-124: Continue Linker Implementation

Tasks 120-124 follow the same TDD pattern:
- Task 120: Implement Linker.DefineResource
- Task 121: Implement Linker.Get for lookups
- Task 122: Implement direct import name matching
- Task 123: Implement import lookup with prefix matching
- Task 124: Integration tests for linker lookups

---

### Task 125: Implement Semver Parsing

**Files:**
- Create: `internal/component/semver.go`
- Create: `internal/component/semver_test.go`

**Step 1: Write failing test**

```go
// internal/component/semver_test.go

package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestParseSemver(t *testing.T) {
	tests := []struct {
		input    string
		expected *Semver
	}{
		{"1.0.0", &Semver{Major: 1, Minor: 0, Patch: 0}},
		{"0.2.0", &Semver{Major: 0, Minor: 2, Patch: 0}},
		{"2.1.3", &Semver{Major: 2, Minor: 1, Patch: 3}},
	}

	for _, tt := range tests {
		result, err := ParseSemver(tt.input)
		require.NoError(t, err)
		require.Equal(t, tt.expected.Major, result.Major)
		require.Equal(t, tt.expected.Minor, result.Minor)
		require.Equal(t, tt.expected.Patch, result.Patch)
	}
}

func TestParseSemver_Invalid(t *testing.T) {
	tests := []string{
		"invalid",
		"1.0",
		"1",
		"",
	}

	for _, input := range tests {
		_, err := ParseSemver(input)
		require.Error(t, err)
	}
}

func TestSplitVersion(t *testing.T) {
	tests := []struct {
		input       string
		baseName    string
		version     string
		hasVersion  bool
	}{
		{"wasi:cli/env@0.2.0", "wasi:cli/env", "0.2.0", true},
		{"test-api", "test-api", "", false},
		{"pkg@1.0.0", "pkg", "1.0.0", true},
	}

	for _, tt := range tests {
		base, ver, hasVer := SplitVersion(tt.input)
		require.Equal(t, tt.baseName, base)
		require.Equal(t, tt.version, ver)
		require.Equal(t, tt.hasVersion, hasVer)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -v -run TestParseSemver`
Expected: FAIL with "undefined: Semver"

**Step 3: Implement Semver**

```go
// internal/component/semver.go

package component

import (
	"fmt"
	"strconv"
	"strings"
)

// Semver represents a semantic version.
type Semver struct {
	Major uint32
	Minor uint32
	Patch uint32
}

// ParseSemver parses a semver string like "1.2.3".
func ParseSemver(s string) (*Semver, error) {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid semver: %s", s)
	}

	major, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid major version: %w", err)
	}

	minor, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid minor version: %w", err)
	}

	patch, err := strconv.ParseUint(parts[2], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid patch version: %w", err)
	}

	return &Semver{
		Major: uint32(major),
		Minor: uint32(minor),
		Patch: uint32(patch),
	}, nil
}

// SplitVersion splits a name like "pkg@1.0.0" into base name and version.
func SplitVersion(name string) (baseName, version string, hasVersion bool) {
	idx := strings.LastIndex(name, "@")
	if idx == -1 {
		return name, "", false
	}
	return name[:idx], name[idx+1:], true
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -v -run TestParseSemver`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/semver.go internal/component/semver_test.go
git commit -m "$(cat <<'EOF'
feat(component): implement semver parsing utilities

Semver and SplitVersion support version extraction from import
names for semver-compatible import resolution.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 126: Implement Semver Compatibility Check

**Files:**
- Modify: `internal/component/semver.go`
- Modify: `internal/component/semver_test.go`

**Step 1: Write failing test**

```go
// Add to internal/component/semver_test.go

func TestSemverCompatible(t *testing.T) {
	tests := []struct {
		required   *Semver
		available  *Semver
		compatible bool
	}{
		// Same version is compatible
		{&Semver{1, 0, 0}, &Semver{1, 0, 0}, true},
		// Newer patch is compatible
		{&Semver{1, 0, 0}, &Semver{1, 0, 1}, true},
		// Newer minor is compatible
		{&Semver{1, 0, 0}, &Semver{1, 1, 0}, true},
		// Different major is not compatible
		{&Semver{1, 0, 0}, &Semver{2, 0, 0}, false},
		// Older version is not compatible
		{&Semver{1, 1, 0}, &Semver{1, 0, 0}, false},
		// Pre-1.0 versions: same minor required
		{&Semver{0, 2, 0}, &Semver{0, 2, 1}, true},
		{&Semver{0, 2, 0}, &Semver{0, 3, 0}, false},
	}

	for _, tt := range tests {
		result := SemverCompatible(tt.required, tt.available)
		require.Equal(t, tt.compatible, result,
			"required=%v available=%v", tt.required, tt.available)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -v -run TestSemverCompatible`
Expected: FAIL with "undefined: SemverCompatible"

**Step 3: Implement SemverCompatible**

```go
// Add to internal/component/semver.go

// SemverCompatible checks if available version satisfies required version.
// For major > 0: same major, available minor.patch >= required minor.patch
// For major == 0: same major.minor, available patch >= required patch
func SemverCompatible(required, available *Semver) bool {
	if required.Major != available.Major {
		return false
	}

	// Pre-1.0: breaking changes allowed in minor bumps
	if required.Major == 0 {
		if required.Minor != available.Minor {
			return false
		}
		return available.Patch >= required.Patch
	}

	// 1.0+: breaking changes only in major bumps
	if available.Minor > required.Minor {
		return true
	}
	if available.Minor == required.Minor {
		return available.Patch >= required.Patch
	}
	return false
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -v -run TestSemverCompatible`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/semver.go internal/component/semver_test.go
git commit -m "$(cat <<'EOF'
feat(component): implement semver compatibility checking

SemverCompatible follows semver rules: same major, available >= required.
Pre-1.0 versions require exact minor match.

Ref: wasmtime old_import_importing_new_item test

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 127: Implement Linker Import Matching with Semver

**Files:**
- Modify: `internal/component/linker.go`
- Modify: `internal/component/linker_test.go`

**Step 1: Write failing test (wasmtime: old_import_importing_new_item)**

```go
// Add to internal/component/linker_test.go

func TestLinker_MatchImport_OldImportNewItem(t *testing.T) {
	// Wasmtime test: old_import_importing_new_item
	// Component requires v1.0.0, linker provides v1.0.1
	l := NewLinker()
	funcType := &FuncType{}

	// Define v1.0.1
	err := l.DefineFunc("test:api@1.0.1", "fn", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	// Request v1.0.0 - should match v1.0.1
	def, err := l.MatchImport("test:api@1.0.0/fn")
	require.NoError(t, err)
	require.NotNil(t, def)
}

func TestLinker_MatchImport_NewImportOldItem(t *testing.T) {
	// Wasmtime test: new_import_importing_old_item
	// Component requires v1.0.1, linker provides v1.0.0 - should NOT match
	l := NewLinker()
	funcType := &FuncType{}

	// Define v1.0.0
	err := l.DefineFunc("test:api@1.0.0", "fn", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	// Request v1.0.1 - should NOT match v1.0.0
	_, err = l.MatchImport("test:api@1.0.1/fn")
	require.Error(t, err)
}

func TestLinker_MatchImport_SelectsMax(t *testing.T) {
	// Wasmtime test: missing_import_selects_max
	l := NewLinker()
	funcType := &FuncType{}

	// Define multiple versions
	l.DefineFunc("test:api@1.0.0", "fn", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return []Val{ValS32(100)}, nil
	})
	l.DefineFunc("test:api@1.0.2", "fn", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return []Val{ValS32(102)}, nil
	})
	l.DefineFunc("test:api@1.0.1", "fn", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return []Val{ValS32(101)}, nil
	})

	// Request v1.0.0 - should select highest compatible (v1.0.2)
	def, err := l.MatchImport("test:api@1.0.0/fn")
	require.NoError(t, err)
	funcDef := def.(*FuncDef)

	// Call to verify we got v1.0.2
	results, err := funcDef.Callback(context.Background(), nil)
	require.NoError(t, err)
	require.Equal(t, int32(102), results[0].S32())
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -v -run TestLinker_MatchImport`
Expected: FAIL with "MatchImport method not found"

**Step 3: Implement MatchImport**

```go
// Add to internal/component/linker.go

// MatchImport finds a definition that satisfies the import name.
// Supports semver-compatible matching per component model spec.
func (l *Linker) MatchImport(importName string) (Definition, error) {
	// Try direct match first
	if def, ok := l.definitions[importName]; ok {
		return def, nil
	}

	// Parse import name into namespace/name@version format
	// e.g., "test:api@1.0.0/fn" -> namespace="test:api@1.0.0", name="fn"
	lastSlash := strings.LastIndex(importName, "/")
	if lastSlash == -1 {
		return nil, fmt.Errorf("import not found: %s", importName)
	}

	namespace := importName[:lastSlash]
	name := importName[lastSlash+1:]

	// Split namespace into base and version
	baseNamespace, reqVersionStr, hasReqVersion := SplitVersion(namespace)
	if !hasReqVersion {
		return nil, fmt.Errorf("import not found: %s", importName)
	}

	reqVersion, err := ParseSemver(reqVersionStr)
	if err != nil {
		return nil, fmt.Errorf("invalid version in import: %w", err)
	}

	// Find best compatible match
	var bestDef Definition
	var bestVersion *Semver

	for defName, def := range l.definitions {
		// Check if it's the same function name
		defLastSlash := strings.LastIndex(defName, "/")
		if defLastSlash == -1 {
			continue
		}
		defNamespace := defName[:defLastSlash]
		defFuncName := defName[defLastSlash+1:]

		if defFuncName != name {
			continue
		}

		// Check namespace compatibility
		defBase, defVersionStr, hasDefVersion := SplitVersion(defNamespace)
		if defBase != baseNamespace {
			continue
		}
		if !hasDefVersion {
			continue
		}

		defVersion, err := ParseSemver(defVersionStr)
		if err != nil {
			continue
		}

		// Check semver compatibility
		if !SemverCompatible(reqVersion, defVersion) {
			continue
		}

		// Select highest compatible version
		if bestVersion == nil || semverGreater(defVersion, bestVersion) {
			bestDef = def
			bestVersion = defVersion
		}
	}

	if bestDef == nil {
		return nil, fmt.Errorf("no compatible definition for: %s", importName)
	}

	return bestDef, nil
}

func semverGreater(a, b *Semver) bool {
	if a.Major != b.Major {
		return a.Major > b.Major
	}
	if a.Minor != b.Minor {
		return a.Minor > b.Minor
	}
	return a.Patch > b.Patch
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -v -run TestLinker_MatchImport`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/linker.go internal/component/linker_test.go
git commit -m "$(cat <<'EOF'
feat(component): implement semver-compatible import matching

MatchImport resolves imports using semver compatibility rules.
Selects highest compatible version when multiple are available.

Ref: wasmtime tests old_import_importing_new_item, missing_import_selects_max

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Tasks 128-150: Remaining Implementation

The remaining tasks follow the same TDD pattern:

**Task 128-135: Nested Component Support**
- Task 128: Add Components field to Component struct
- Task 129: Implement nested component section decoder
- Task 130: Test two-level nested hierarchy (wasmtime: top_level_instance_two_level)
- Task 131: Test multiple instantiations (wasmtime: nested_many_instantiations)
- Task 132-135: Recursive instantiation support

**Task 136-142: Full Instantiation Pipeline**
- Task 136: Implement Linker.Instantiate basic flow
- Task 137: Implement alias resolution during instantiation
- Task 138: Implement core module instantiation
- Task 139: Implement component instance creation
- Task 140: Implement canonical operation wiring
- Task 141: Implement export wiring
- Task 142: Integration test full instantiation

**Task 143-146: Instance Exports**
- Task 143: Implement Instance.GetExport (wasmtime: instance_exports)
- Task 144: Implement semver export lookup (wasmtime: export_old_get_new)
- Task 145: Implement reverse semver lookup (wasmtime: export_new_get_old)
- Task 146: Implement max version fallback (wasmtime: export_missing_get_max)

**Task 147-150: Integration Tests**
- Task 147: Create test component with imports
- Task 148: Test component importing a function
- Task 149: Test component importing an instance
- Task 150: Test full linking scenario

---

## Running Tests

```bash
# Run all Phase 4 tests
go test ./internal/component/... -v -run "Test(Sort|Alias|Import|CoreInstance|Linker|Semver)"

# Run linker tests specifically
go test ./internal/component/... -v -run TestLinker

# Run alias parsing tests
go test ./internal/component/binary/... -v -run TestDecodeAlias

# Run import parsing tests
go test ./internal/component/binary/... -v -run TestDecodeImport

# Run semver tests
go test ./internal/component/... -v -run TestSemver

# Run with race detector
go test ./internal/component/... -race -v

# Run all component tests
go test ./internal/component/... -v
```

---

## Phase 4 Completion Checklist

- [ ] Sort and CoreSort types defined with String() methods
- [ ] Alias types defined (AliasKind, Alias)
- [ ] Alias section decoder implemented (export, core export, outer)
- [ ] Import types defined (ImportExternDescKind, ImportExternDesc, Import)
- [ ] Import section decoder implemented
- [ ] CoreInstance types defined
- [ ] Core instance section decoder implemented
- [ ] ComponentInstance types defined
- [ ] Component instance section decoder implemented
- [ ] Linker.DefineFunc implemented
- [ ] Linker.DefineInstance with builder pattern implemented
- [ ] Linker.DefineResource implemented
- [ ] Semver parsing and comparison implemented
- [ ] Linker.MatchImport with semver compatibility implemented
- [ ] Nested component parsing supported
- [ ] Linker.Instantiate implemented
- [ ] Alias resolution during instantiation
- [ ] Instance export retrieval with semver matching
- [ ] Integration tests pass with real components
- [ ] Wasmtime test scenarios ported:
  - [ ] old_import_importing_new_item
  - [ ] new_import_importing_old_item
  - [ ] missing_import_selects_max
  - [ ] instance_exports
  - [ ] export_old_get_new
  - [ ] top_level_instance_two_level

---

## References

- [Component Model Binary Format](https://github.com/WebAssembly/component-model/blob/main/design/mvp/Binary.md)
- [Component Model Explainer - Composition](https://github.com/WebAssembly/component-model/blob/main/design/mvp/Explainer.md#composition)
- [Wasmtime Component Tests](https://github.com/bytecodealliance/wasmtime/tree/main/tests/all/component_model)
  - [linker.rs](https://github.com/bytecodealliance/wasmtime/blob/main/tests/all/component_model/linker.rs)
  - [import.rs](https://github.com/bytecodealliance/wasmtime/blob/main/tests/all/component_model/import.rs)
  - [instance.rs](https://github.com/bytecodealliance/wasmtime/blob/main/tests/all/component_model/instance.rs)
  - [nested.rs](https://github.com/bytecodealliance/wasmtime/blob/main/tests/all/component_model/nested.rs)
