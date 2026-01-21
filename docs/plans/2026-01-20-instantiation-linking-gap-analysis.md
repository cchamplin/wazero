# Component Model Instantiation & Linking Gap Analysis

**Date:** 2026-01-20
**Status:** Analysis Complete
**Scope:** Instantiation, Linking, and Type Matching

## Executive Summary

The wazero component model has a functional implementation that passes the calculator test suite (add, subtract, multi, div plugins). However, comparing against the Component Model specification and wasmtime's reference implementation reveals several gaps that will affect more complex components, multi-component scenarios, and spec compliance.

### What Works Today

| Feature | Status | Evidence |
|---------|--------|----------|
| Core module instantiation | ✅ Working | Calculator tests pass |
| Canon lift/lower | ✅ Working | Function exports callable |
| Import semver matching | ✅ Working | Relaxed mode for WASI 0.2.x |
| Resource table basics | ✅ Working | WASI streams work |
| Core export aliases | ✅ Working | Memory/func aliases resolved |
| Instance imports | ✅ Working | WASI interfaces resolved |
| Memory sharing | ✅ Working | Incremental wiring works |

### Critical Gaps

| Gap | Priority | Impact |
|-----|----------|--------|
| Type subtyping validation | P1 | Spec compliance, incorrect components may instantiate |
| Nested component instantiation | P1 | Multi-component linking broken |
| Component instance aliasing | P1 | Component composition broken |
| Start function handling | P2 | Components with start won't execute initialization |
| Resource type equality checking | P2 | Type-unsafe resource sharing |
| Outer alias resolution | P2 | Nested component access to parent types |

---

## Detailed Gap Analysis

### 1. Core Module Instantiation

**Spec Requirements (Binary.md lines 63-67):**
```
core:instanceexpr ::= 0x00 m:<moduleidx> arg*:vec(<core:instantiatearg>)
core:instantiatearg ::= n:<core:name> 0x12 i:<instanceidx>
```

**Current Implementation (`component_linker.go:233-253`):**
- ✅ CoreInstanceExprInstantiate handled correctly
- ✅ CoreInstanceExprInline handled (placeholder instance)
- ✅ Args mapped via `buildImportResolver`
- ✅ Memory sharing wired incrementally

**Gaps:**
1. **Inline instance module creation incomplete** (`component_linker.go:775`)
   - `createInlineInstanceModule` builds host modules for canon operations
   - BUT: Mixed inline exports (some from instances, some from canon ops) not fully tested
   - Wasmtime builds these lazily during instantiation

**Test Coverage Needed:**
```go
// Complex inline instance with mixed export sources
(core instance (;5;)
  (export "mem" (memory 3 "mem"))           // From core instance 3
  (export "lowered_fn" (func 42))           // From canon lower
  (export "drop_resource" (func 45))        // From resource.drop
)
```

---

### 2. Component Instantiation

**Spec Requirements (Explainer.md, Binary.md lines 77-89):**
- Component instantiation requires matching all named imports with `with` arguments
- Type-compatibility required between supplied arguments and import types
- When type imports are supplied, the actual type **immediately replaces all uses**

**Current Implementation:**
- ❌ **Nested component instantiation NOT implemented**
- Component types parsed (`ComponentTypeDef`) but never instantiated
- `c.ComponentInstances` tracked but not processed during instantiation

**Evidence (`component_linker.go:176-180`):**
```go
// Component instance definitions also add to the instance index space
// (after imports)
for range c.ComponentInstances {
    compInstanceIdx++  // Just counting, never instantiating!
}
```

**Gap: No recursive component instantiation**

**Wasmtime Approach (`linker.rs`):**
- `instantiate_pre` resolves all imports including nested components
- Uses `TypeChecker` for recursive validation
- `InstancePre::instantiate` calls nested component instantiation recursively

**Required Implementation:**
```go
func (l *ComponentLinker) instantiateNestedComponent(
    ctx context.Context,
    inst *Instance,
    compInst *ComponentInstance,
    parentComponent *Component,
) (*Instance, error) {
    // 1. Get the nested component definition
    nestedComp := parentComponent.Components[compInst.ComponentIdx]

    // 2. Build with arguments from parent scope
    withArgs := make(map[string]Definition)
    for _, arg := range compInst.Args {
        // Resolve arg.Sort, arg.Idx from parent index spaces
        def := resolveFromParentScope(inst, arg.Sort, arg.Idx)
        withArgs[arg.Name] = def
    }

    // 3. Recursively instantiate
    return l.instantiateWithArgs(ctx, nestedComp, withArgs)
}
```

---

### 3. Import Resolution

**Spec Requirements (Linking.md, Explainer.md lines 2507-2634):**

| Import Type | Format | Resolution |
|-------------|--------|------------|
| Plain name | `"foo"` | Exact match |
| Interface | `"wasi:io/streams@0.2.0"` | Semver-compatible |
| Locked dep | `"locked-dep=<name>:*"` | Registry resolution |
| Unlocked dep | `"unlocked-dep=<name>:>=1.0.0"` | Version range |
| URL | `"url=https://..."` | Fetch and resolve |
| Hash | `"integrity=sha256:..."` | Content hash |

**Current Implementation (`linker.go:140-279`):**
- ✅ Plain name imports
- ✅ Interface imports with semver (including relaxed mode)
- ❌ Locked/unlocked dependency imports
- ❌ URL imports
- ❌ Hash imports

**Gap Details:**

1. **Dependency imports not parsed:**
   ```go
   // Should parse: "locked-dep=my-dep:0.1.0"
   // Currently: treated as plain name, fails to match
   ```

2. **Version range syntax not supported:**
   ```
   @*                        // All versions - NOT SUPPORTED
   @{>=1.0.0}               // Lower bound - NOT SUPPORTED
   @{>=1.0.0 <2.0.0}        // Range - NOT SUPPORTED
   ```

**Impact:** Components using locked-dep or unlocked-dep imports will fail.

**Wasmtime Reference (`linker.rs`):**
- Parses import names into structured types
- Supports all import name variants
- Uses semver crate for range matching

---

### 4. Type Matching & Subtyping

**Spec Requirements (Explainer.md lines 923-1072):**

**Structural Equality:**
- Non-resource types use structural equality (AST comparison)
- Index layout doesn't affect equality

**Subtyping Rules:**
| Type | Rule |
|------|------|
| Component | Export more, import less |
| Instance | Export more (extra exports OK) |
| Function | Params contravariant, results covariant |
| Record | Width subtyping with reordering |
| Variant | Refinement (fewer cases OK) |
| Resource | Exact equality only |

**Current Implementation:**
- ❌ **No subtyping validation at instantiation time**
- ❌ **No structural type comparison**
- ❌ **Resource equality not checked**

**Evidence (`linker.go:291-315`):**
```go
func (l *Linker) Instantiate(ctx context.Context, c *Component) (*Instance, error) {
    for _, imp := range c.Imports {
        _, err := l.MatchImport(imp.Name)  // Name matching only!
        if err != nil {
            return nil, fmt.Errorf("import %q: %w", imp.Name, err)
        }
        // NO TYPE CHECKING!
    }
}
```

**Wasmtime Approach (`matching.rs`):**

```rust
pub struct TypeChecker<'a> {
    pub engine: &'a Engine,
    pub types: &'a Arc<ComponentTypes>,
    pub strings: &'a Strings,
    pub imported_resources: Arc<PrimaryMap<ResourceIndex, ResourceType>>,
}

// Entry point for type checking
fn definition(&mut self, expected: &TypeDef, actual: Option<&Definition>) -> Result<()> {
    match expected {
        TypeDef::Module(expected) => self.module(expected, actual),
        TypeDef::ComponentInstance(expected) => self.instance(expected, actual),
        TypeDef::ComponentFunc(expected) => self.func(expected, actual),
        TypeDef::Resource(i) => self.resource(*i, actual),
    }
}
```

**Required Implementation:**
```go
type TypeChecker struct {
    component         *Component
    importedResources map[uint32]ResourceType // First-seen resource types
}

func (tc *TypeChecker) CheckDefinition(expected *TypeDef, actual Definition) error {
    switch expected.Kind {
    case TypeDefKindFunc:
        return tc.checkFunc(expected.Func, actual)
    case TypeDefKindInstance:
        return tc.checkInstance(expected.Instance, actual)
    case TypeDefKindResource:
        return tc.checkResource(expected, actual)
    }
}

func (tc *TypeChecker) checkFunc(expected *FuncType, actual Definition) error {
    funcDef, ok := actual.(*FuncDef)
    if !ok {
        return fmt.Errorf("expected function, got %T", actual)
    }

    // Check params (contravariant - actual can accept more)
    if len(funcDef.Type.Params) < len(expected.Params) {
        return fmt.Errorf("insufficient parameters")
    }
    for i, p := range expected.Params {
        if !tc.isSubtype(funcDef.Type.Params[i].ValType, p.ValType) {
            return fmt.Errorf("param %d type mismatch", i)
        }
    }

    // Check results (covariant - actual must be at least as specific)
    // ...
}
```

---

### 5. Export Handling

**Spec Requirements (Explainer.md lines 2481-2497):**
- Appends new element to index space of exported sort
- Optional `externdesc` allows type ascription (must be supertype of inferred)
- All exports strongly-unique

**Current Implementation (`component_linker.go:255-264`):**
- ✅ Function exports wired
- ❌ Instance exports NOT exposed
- ❌ Type exports NOT tracked
- ❌ Type ascription NOT validated

**Evidence (`linker_api.go:162-165`):**
```go
func (w *ComponentWrapper) ExportedInstance(name string) api.ComponentInstance {
    return nil // NOT IMPLEMENTED
}
```

**Gap: Cannot access exported instances from instantiated components**

---

### 6. Alias Resolution

**Spec Requirements (Explainer.md lines 340-406):**

| Alias Type | Syntax | Current Status |
|------------|--------|----------------|
| Export | `(alias export $i "name")` | ✅ Working |
| Core Export | `(alias core export $i "name")` | ✅ Working |
| Outer | `(alias outer $depth $idx)` | ⚠️ Parsed, not used |

**Current Implementation:**

```go
for i := range c.Aliases {
    alias := &c.Aliases[i]
    if alias.Kind == AliasKindCoreExport {
        switch alias.CoreSort {
        case CoreSortFunc:
            funcSpace.AddAlias(...)  // ✅
        case CoreSortMemory:
            memSpace.AddAlias(...)   // ✅
        }
    }
}
```

**Gap: Outer aliases parsed but not resolved during instantiation**

**Impact:** Nested components cannot access parent component's types/modules/components.

**Required for:** Components that share types across nested component boundaries.

---

### 7. Start Function Handling

**Spec Requirements (Binary.md lines 360-379, Explainer.md lines 2436-2476):**
- Start function called during instantiation
- Results appended to value index space
- Each value consumed exactly once (tracked via consumed flag)

**Current Implementation:**
- ❌ **Start function NOT executed**
- Start definition parsed (`Component.Start`) but never invoked

**Evidence:** No `Start` field usage in `component_linker.go`

**Required Implementation:**
```go
// In Instantiate, after core modules but before wiring exports:
if c.Start != nil {
    startFunc := inst.componentFuncs[c.Start.FuncIdx]

    // Gather value arguments
    args := make([]Val, len(c.Start.Args))
    for i, argIdx := range c.Start.Args {
        args[i] = inst.values[argIdx]
    }

    // Call start function
    results, err := startFunc.Call(ctx, args)
    if err != nil {
        return nil, fmt.Errorf("start function failed: %w", err)
    }

    // Append results to value index space
    for _, r := range results {
        inst.values = append(inst.values, r)
    }
}
```

---

### 8. Resource Type System

**Spec Requirements (Explainer.md lines 1020-1034, 1053-1072):**
- Each `(sub resource)` bound creates unique abstract type
- Resources use exact equality (no subtyping)
- Handles inherit freshness from abstract resources

**Current Implementation:**
- ✅ Resource table operations (new, drop, rep)
- ✅ Resource handles tracked
- ❌ Resource type identity NOT validated
- ❌ `eq` bounds NOT enforced
- ❌ Abstract type freshness NOT tracked

**Gap: Can mix incompatible resource types**

**Wasmtime Approach (`matching.rs`):**
```rust
TypeDef::Resource(i) => {
    match self.imported_resources.get(i) {
        None => {
            // First time - record it
            resources.push(*actual);
        }
        Some(expected) => {
            // Second occurrence - must be equal
            if expected != actual {
                bail!("mismatched resource types")
            }
        }
    }
}
```

---

### 9. Component Instance State

**Spec Requirements:**
- Resource tables per component instance
- Memory/realloc references for canonical options
- may_leave flag for async boundaries
- Parent-child relationships for nested components

**Current Implementation (`instance.go:16-32`):**
```go
type Instance struct {
    component      *Component
    coreInstances  []api.Module
    exports        map[string]*ExportedFunc
    componentFuncs map[uint32]ComponentFunc
    resourceTable  *ResourceTable
    destructors    map[uint32]func(any)
    callContext    *CallContext
}
```

**Gaps:**
1. ❌ No `may_leave` flag
2. ❌ No parent instance reference
3. ❌ No child instance tracking

---

## Implementation Plan

### Phase 1: Type Checking (P1)

**Goal:** Validate types at instantiation time

1. **Create TypeChecker struct**
   - `CheckDefinition(expected, actual) error`
   - Track imported resources for equality checks

2. **Implement subtyping rules**
   - Function contravariance/covariance
   - Instance width subtyping
   - Resource equality

3. **Integrate into Linker.Instantiate**
   - After `MatchImport`, call `TypeChecker.CheckDefinition`
   - Fail fast on mismatches

**Files:**
- NEW: `internal/component/type_checker.go`
- MODIFY: `internal/component/linker.go`
- MODIFY: `internal/component/component_linker.go`

**Dependency:** None

### Phase 2: Start Function (P2)

**Goal:** Execute start functions during instantiation

1. **Add value index space to Instance**
   ```go
   type Instance struct {
       // ...
       values []Val  // Value index space
   }
   ```

2. **Process value imports**
   - Add to value index space during import resolution

3. **Execute start function**
   - After core modules, before exports
   - Append results to value index space

4. **Track value consumption**
   - Ensure each value used exactly once

**Files:**
- MODIFY: `internal/component/instance.go`
- MODIFY: `internal/component/component_linker.go`

**Dependency:** None

### Phase 3: Nested Components (P1)

**Goal:** Support component-in-component instantiation

1. **Add parent reference to Instance**
   ```go
   type Instance struct {
       parent *Instance
       children []*Instance
   }
   ```

2. **Implement `instantiateNestedComponent`**
   - Resolve `with` arguments from parent scope
   - Recursive instantiation

3. **Process ComponentInstances in Instantiate**
   - After imports, before core modules

4. **Support outer aliases**
   - Resolve types/modules from parent scope

**Files:**
- MODIFY: `internal/component/instance.go`
- MODIFY: `internal/component/component_linker.go`
- NEW: `internal/component/nested_component.go`

**Dependency:** Phase 1 (type checking for args)

### Phase 4: Export Instance Access (P2)

**Goal:** Expose exported instances to API

1. **Implement `ExportedInstance`**
   ```go
   func (w *ComponentWrapper) ExportedInstance(name string) api.ComponentInstance
   ```

2. **Track instance exports during instantiation**
   - Store in Instance.exportedInstances

3. **Expose via API wrapper**

**Files:**
- MODIFY: `internal/component/linker_api.go`
- MODIFY: `internal/component/instance.go`

**Dependency:** Phase 3 (nested instances)

### Phase 5: Advanced Import Names (P3)

**Goal:** Support all import name variants

1. **Parse dependency imports**
   - `locked-dep=<name>:<version>`
   - `unlocked-dep=<name>:<range>`

2. **Support version ranges**
   - `@*`, `@{>=1.0.0}`, `@{>=1.0.0 <2.0.0}`

3. **URL/hash imports (optional)**
   - Lower priority, rarely used

**Files:**
- MODIFY: `internal/component/linker.go`
- NEW: `internal/component/import_name.go`
- MODIFY: `internal/component/semver.go`

**Dependency:** None

---

## Test Scenarios

### Multi-Component Tests

```go
// Test: Parent component instantiates child component
func TestNestedComponentInstantiation(t *testing.T) {
    // Component A exports a function
    // Component B imports A, exports wrapped function
    // Instantiate B, call exported function
}

// Test: Type sharing across components
func TestSharedTypeAcrossComponents(t *testing.T) {
    // Parent defines record type
    // Child imports type via outer alias
    // Function uses shared type
}

// Test: Resource sharing between components
func TestResourceAcrossComponents(t *testing.T) {
    // Parent creates resource
    // Passes handle to child
    // Child uses handle
}
```

### Type Matching Tests

```go
// Test: Function subtyping
func TestFunctionContravariance(t *testing.T) {
    // Import expects (i32, i32) -> i32
    // Provide (i32, i32, i32) -> i32  // Extra param OK (contravariant)
}

func TestFunctionCovariance(t *testing.T) {
    // Import expects () -> result<u32, string>
    // Provide () -> u32  // OK (subtype of result's ok case)
}

// Test: Instance width subtyping
func TestInstanceExtraExports(t *testing.T) {
    // Import expects {foo: func}
    // Provide {foo: func, bar: func}  // Extra export OK
}

// Test: Resource equality
func TestResourceMismatch(t *testing.T) {
    // Import expects resource type A
    // Provide resource type B
    // Should FAIL
}
```

### Import Resolution Tests

```go
// Test: Version range matching
func TestVersionRange(t *testing.T) {
    cases := []struct{
        importName string
        available  []string
        expected   string
    }{
        {"pkg@{>=1.0.0}", []string{"pkg@0.9.0", "pkg@1.2.0"}, "pkg@1.2.0"},
        {"pkg@{>=1.0.0 <2.0.0}", []string{"pkg@1.5.0", "pkg@2.0.0"}, "pkg@1.5.0"},
        {"pkg@*", []string{"pkg@0.1.0", "pkg@3.0.0"}, "pkg@3.0.0"},
    }
}

// Test: Locked dependency
func TestLockedDependency(t *testing.T) {
    // Import: "locked-dep=my-dep:1.2.3"
    // Provide: my-dep@1.2.3
    // Should match exactly
}
```

### Start Function Tests

```go
// Test: Start function execution
func TestStartFunction(t *testing.T) {
    // Component with:
    //   (import "name" (value $name string))
    //   (start $init (value $name) (result (value $greeting)))
    //   (export "greeting" (value $greeting))

    // Instantiate with name="World"
    // Export greeting should be "Hello, World!"
}

// Test: Value consumption tracking
func TestValueConsumedOnce(t *testing.T) {
    // Value used in both start and export
    // Should fail validation
}
```

---

## Regression Requirements

All changes MUST maintain passing status for:

```bash
go test -v ./internal/component/wasip2test/... -run TestCalculatorPlugins
```

Tests that must pass:
- `add` - Rust plugin with WASI, uses relaxed semver
- `subtract` - C plugin, no WASI
- `multi` - Go plugin with wit-bindgen
- `div` - Go plugin with tinygo wasip2

---

## References

- [Component Model Explainer](https://github.com/WebAssembly/component-model/blob/main/design/mvp/Explainer.md)
- [Component Model Binary Format](https://github.com/WebAssembly/component-model/blob/main/design/mvp/Binary.md)
- [Component Model Linking](https://github.com/WebAssembly/component-model/blob/main/design/mvp/Linking.md)
- [Wasmtime linker.rs](https://github.com/bytecodealliance/wasmtime/blob/main/crates/wasmtime/src/runtime/component/linker.rs)
- [Wasmtime matching.rs](https://github.com/bytecodealliance/wasmtime/blob/main/crates/wasmtime/src/runtime/component/matching.rs)
