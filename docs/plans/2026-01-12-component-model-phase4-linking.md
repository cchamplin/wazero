# Component Model Phase 4: Full Instantiation & Linking

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Parent Plan:** [2026-01-12-component-model-implementation.md](./2026-01-12-component-model-implementation.md)
**Design Doc:** [2026-01-12-component-model-design.md](./2026-01-12-component-model-design.md)
**Previous Phase:** [Phase 3: Resources](./2026-01-12-component-model-phase3-resources.md)
**Status:** NOT STARTED
**Estimated Tasks:** 101-150

---

## Overview

This phase implements the complete component instantiation and linking system, including alias handling, canonical definitions, semver import matching, and nested component support.

**Goal:** Full component instantiation with import resolution, nested components, and proper linking.

**Prerequisites:**
- Phase 1 complete (binary parser)
- Phase 2 complete (all types)
- Phase 3 complete (resources)

---

## Phase 4 Milestones

| Milestone | Description | Success Criteria |
|-----------|-------------|------------------|
| 4.1 | Alias section parsing | All alias kinds parsed and resolved |
| 4.2 | Complete canonical definitions | Lift/lower/resource ops fully wired |
| 4.3 | Import section parsing | Component imports with type checking |
| 4.4 | Linker with semver matching | Imports resolve with semver compatibility |
| 4.5 | Nested component support | Components can contain and instantiate components |
| 4.6 | Instance exports | Export instances with proper scoping |

---

## Linker Architecture

From the design doc:

```go
// internal/component/linker.go

type Linker struct {
    definitions map[string]Definition
    engine      wasm.Engine
}

type Definition interface {
    definition()
}

type FuncDef struct {
    Type     *FuncType
    Callback func(ctx context.Context, args []Val) ([]Val, error)
}

type InstanceDef struct {
    Exports map[string]Definition
}

type ResourceDef struct {
    Type       *ResourceType
    Destructor func(rep uint32)
}

func (l *Linker) DefineFunc(namespace, name string, fn func(context.Context, []Val) ([]Val, error)) error
func (l *Linker) DefineInstance(namespace string) *InstanceBuilder
func (l *Linker) DefineResource(namespace, name string, dtor func(rep uint32)) error
func (l *Linker) Instantiate(ctx context.Context, c *Component) (*Instance, error)
```

---

## Tasks

### Task 101: Parse Alias Section

**Files:**
- Modify: `internal/component/binary/decoder.go`
- Modify: `internal/component/component.go`
- Create: `internal/component/binary/alias_test.go`

**Step 1: Write failing test**

```go
// internal/component/binary/alias_test.go

package binary

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestDecodeAlias(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected Alias
	}{
		{
			name: "outer core type alias",
			input: []byte{
				0x00,       // kind: outer
				0x01,       // outer alias kind: core type
				0x01,       // count: 1 level up
				0x00,       // index: 0
			},
			expected: Alias{
				Kind:     AliasKindOuter,
				Target:   AliasTargetCoreType,
				Count:    1,
				Index:    0,
			},
		},
		{
			name: "export alias",
			input: []byte{
				0x01,       // kind: export
				0x00,       // instance index
				0x04, 'n', 'a', 'm', 'e', // export name
			},
			expected: Alias{
				Kind:         AliasKindExport,
				InstanceIdx:  0,
				ExportName:   "name",
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			alias, err := decodeAlias(bytes.NewReader(tc.input))
			require.NoError(t, err)
			require.Equal(t, tc.expected, alias)
		})
	}
}
```

**Step 2: Define alias types**

```go
// Add to internal/component/component.go

// Alias represents an alias definition.
type Alias struct {
	Kind AliasKind

	// For outer aliases
	Target AliasTarget
	Count  uint32 // Number of enclosing scopes
	Index  uint32

	// For export aliases
	InstanceIdx uint32
	ExportName  string
}

type AliasKind uint8

const (
	AliasKindOuter  AliasKind = 0x00
	AliasKindExport AliasKind = 0x01
)

type AliasTarget uint8

const (
	AliasTargetCoreModule   AliasTarget = 0x00
	AliasTargetCoreType     AliasTarget = 0x01
	AliasTargetType         AliasTarget = 0x02
	AliasTargetComponent    AliasTarget = 0x03
	AliasTargetInstance     AliasTarget = 0x04
	AliasTargetFunc         AliasTarget = 0x05
	AliasTargetValue        AliasTarget = 0x06
	AliasTargetTable        AliasTarget = 0x07
	AliasTargetMemory       AliasTarget = 0x08
	AliasTargetGlobal       AliasTarget = 0x09
)
```

**Step 3: Implement alias decoding**

```go
// Add to internal/component/binary/decoder.go

func decodeAlias(r *bytes.Reader) (component.Alias, error) {
	kindByte, err := r.ReadByte()
	if err != nil {
		return component.Alias{}, err
	}

	alias := component.Alias{Kind: component.AliasKind(kindByte)}

	switch alias.Kind {
	case component.AliasKindOuter:
		targetByte, _ := r.ReadByte()
		alias.Target = component.AliasTarget(targetByte)
		alias.Count, _, _ = leb128.DecodeUint32(r)
		alias.Index, _, _ = leb128.DecodeUint32(r)

	case component.AliasKindExport:
		alias.InstanceIdx, _, _ = leb128.DecodeUint32(r)
		alias.ExportName, _ = decodeName(r)
	}

	return alias, nil
}
```

---

### Task 102-105: Parse Import Section

Parse component imports with kebab-name format and type references.

**Import format:**
```
import ::= kebab-name importdesc
importdesc ::= 0x00 typeidx     ; func
             | 0x01 typeidx     ; value
             | 0x02 typebound   ; type
             | 0x03 typeidx     ; component
             | 0x04 typeidx     ; instance
```

---

### Task 106-110: Implement Linker Core

**Files:**
- Create: `internal/component/linker.go`
- Create: `internal/component/linker_test.go`

```go
// internal/component/linker.go

package component

import (
	"context"
	"fmt"
)

// Linker resolves component imports.
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

// DefineInstance adds a namespace with multiple exports.
func (l *Linker) DefineInstance(namespace string) *InstanceBuilder {
	return &InstanceBuilder{
		linker:    l,
		namespace: namespace,
		exports:   make(map[string]Definition),
	}
}

// InstanceBuilder builds an instance definition.
type InstanceBuilder struct {
	linker    *Linker
	namespace string
	exports   map[string]Definition
}

func (b *InstanceBuilder) Func(name string, typ *FuncType, fn HostFunc) *InstanceBuilder {
	b.exports[name] = &FuncDef{Type: typ, Callback: fn}
	return b
}

func (b *InstanceBuilder) Build() error {
	b.linker.definitions[b.namespace] = &InstanceDef{Exports: b.exports}
	return nil
}
```

---

### Task 111-115: Semver Import Matching

Implement semver-compatible import resolution:

```go
// matchImport checks if an import can be satisfied by a definition.
// Import names follow kebab-name with optional version:
//   "wasi:filesystem/types@0.2.0"
func matchImport(importName string, definitions map[string]Definition) (Definition, error) {
	// Direct match
	if def, ok := definitions[importName]; ok {
		return def, nil
	}

	// Try without version for semver matching
	baseName, version := parseVersion(importName)
	if version == nil {
		return nil, fmt.Errorf("import not found: %s", importName)
	}

	// Find compatible version
	for name, def := range definitions {
		defBase, defVersion := parseVersion(name)
		if defBase != baseName {
			continue
		}
		if semverCompatible(version, defVersion) {
			return def, nil
		}
	}

	return nil, fmt.Errorf("no compatible definition for: %s", importName)
}

func semverCompatible(required, available *semver) bool {
	// Same major version, available >= required
	return available.Major == required.Major &&
		(available.Minor > required.Minor ||
			(available.Minor == required.Minor && available.Patch >= required.Patch))
}
```

---

### Task 116-120: Instance Section Parsing

Parse instance sections that instantiate core modules or components:

```
instance ::= instantiate moduleidx (with name idx)*
          | inline export*
```

---

### Task 121-130: Nested Component Support

**Files:**
- Modify: `internal/component/component.go`
- Modify: `internal/component/binary/decoder.go`
- Create: `internal/component/instance_test.go`

Support recursive component parsing:

```go
// Component contains nested components
type Component struct {
	// ...existing fields...

	// Components contains nested component definitions (section ID 4).
	Components []*Component

	// Instances contains component instance definitions (section ID 5).
	Instances []InstanceDef
}

// For SectionIDComponent, recurse:
case SectionIDComponent:
	nested, err := DecodeComponent(sectionData)
	if err != nil {
		return nil, fmt.Errorf("nested component: %w", err)
	}
	c.Components = append(c.Components, nested)
```

---

### Task 131-140: Complete Instantiation

Wire together all the pieces for full instantiation:

```go
// Instantiate creates a live instance from a compiled component.
func (l *Linker) Instantiate(ctx context.Context, rt wazero.Runtime, c *Component) (*Instance, error) {
	inst := &Instance{
		component:     c,
		coreInstances: make([]*wasm.ModuleInstance, 0),
		subInstances:  make([]*Instance, 0),
		resources:     make(map[uint32]*ResourceTable),
		exports:       make(map[string]ExportInstance),
	}

	// 1. Instantiate core modules
	for i, mod := range c.CoreModules {
		coreInst, err := rt.InstantiateModule(ctx, mod, wazero.NewModuleConfig())
		if err != nil {
			return nil, fmt.Errorf("core module %d: %w", i, err)
		}
		inst.coreInstances = append(inst.coreInstances, coreInst)
	}

	// 2. Instantiate nested components
	for i, nested := range c.Components {
		subInst, err := l.Instantiate(ctx, rt, nested)
		if err != nil {
			return nil, fmt.Errorf("nested component %d: %w", i, err)
		}
		inst.subInstances = append(inst.subInstances, subInst)
	}

	// 3. Resolve aliases
	// 4. Apply canonical operations
	// 5. Wire exports

	return inst, nil
}
```

---

### Task 141-150: Integration Tests

Test components:

```
internal/component/testdata/
├── linking/
│   ├── import_func.wasm       # Component importing a function
│   ├── import_instance.wasm   # Component importing an instance
│   ├── nested_simple.wasm     # Simple nested component
│   ├── nested_compose.wasm    # Component composition
│   └── linking.wit
```

---

## Running Tests

```bash
# Run linker tests
go test ./internal/component/... -v -run TestLinker

# Run instantiation tests
go test ./internal/component/... -v -run TestInstantiate

# Run alias parsing tests
go test ./internal/component/binary/... -v -run TestAlias
```

---

## References

- [Component Model Binary Format - Aliases](https://github.com/WebAssembly/component-model/blob/main/design/mvp/Binary.md#alias-definitions)
- [Component Model Binary Format - Imports](https://github.com/WebAssembly/component-model/blob/main/design/mvp/Binary.md#import-definitions)
- [Component Model Explainer - Composition](https://github.com/WebAssembly/component-model/blob/main/design/mvp/Explainer.md#composition)
