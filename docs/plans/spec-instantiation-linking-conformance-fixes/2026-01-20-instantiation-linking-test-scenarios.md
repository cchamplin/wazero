# Component Model Instantiation & Linking Test Scenarios

**Date:** 2026-01-20
**Companion:** [Gap Analysis](../2026-01-20-instantiation-linking-gap-analysis.md)

## Test Categories

1. Multi-Component Tests
2. Import/Export Type Matching Tests
3. Alias Resolution Tests
4. Start Function Tests
5. Resource System Tests
6. Edge Case & Error Tests

---

## 1. Multi-Component Tests

### 1.1 Basic Nested Component

**Purpose:** Verify that a parent component can instantiate a child component.

**Component Structure:**
```wat
;; Child component - exports a simple function
(component $child
  (core module $m
    (func (export "add") (param i32 i32) (result i32)
      local.get 0
      local.get 1
      i32.add
    )
  )
  (core instance $i (instantiate $m))
  (func $add (param "a" s32) (param "b" s32) (result s32)
    (canon lift (core func $i "add"))
  )
  (export "add" (func $add))
)

;; Parent component - instantiates child and re-exports
(component $parent
  (import "child" (component $c
    (export "add" (func (param "a" s32) (param "b" s32) (result s32)))
  ))
  (instance $child-inst (instantiate $c))
  (alias export $child-inst "add" (func $add))
  (export "add" (func $add))
)
```

**Test:**
```go
func TestBasicNestedComponent(t *testing.T) {
    ctx := context.Background()
    rt := wazero.NewRuntime(ctx)
    defer rt.Close(ctx)

    // Compile child component
    childBytes := loadTestComponent("nested/child.wasm")
    childCompiled, err := rt.CompileComponent(ctx, childBytes)
    require.NoError(t, err)

    // Compile parent component
    parentBytes := loadTestComponent("nested/parent.wasm")
    parentCompiled, err := rt.CompileComponent(ctx, parentBytes)
    require.NoError(t, err)

    // Create linker and provide child as import
    linker := component.NewComponentLinker(rt)
    linker.DefineComponent("child", childCompiled)

    // Instantiate parent
    inst, err := linker.Instantiate(ctx, parentCompiled)
    require.NoError(t, err)

    // Call the re-exported add function
    addFunc := inst.ExportedFunction("add")
    require.NotNil(t, addFunc)

    result, err := addFunc.Call(ctx, component.ValS32(10), component.ValS32(20))
    require.NoError(t, err)
    assert.Equal(t, int32(30), result[0].S32())
}
```

### 1.2 Multi-Level Nesting

**Purpose:** Verify 3+ levels of component nesting.

```wat
;; Level 3 (innermost)
(component $level3
  (export "value" (func (result s32)))
)

;; Level 2
(component $level2
  (import "inner" (component $i (export "value" (func (result s32)))))
  (instance $inner-inst (instantiate $i))
  (export "inner" (instance $inner-inst))
)

;; Level 1 (outermost)
(component $level1
  (import "middle" (component $m
    (export "inner" (instance (export "value" (func (result s32)))))
  ))
  (instance $middle-inst (instantiate $m))
  (alias export $middle-inst "inner" (instance $inner))
  (export "nested-inner" (instance $inner))
)
```

**Test:**
```go
func TestMultiLevelNesting(t *testing.T) {
    // ... setup

    // Verify we can access the innermost function through nested exports
    inst, err := linker.Instantiate(ctx, level1Compiled)
    require.NoError(t, err)

    nestedInner := inst.ExportedInstance("nested-inner")
    require.NotNil(t, nestedInner)

    valueFunc := nestedInner.ExportedFunction("value")
    require.NotNil(t, valueFunc)
}
```

### 1.3 Shared Type Across Components

**Purpose:** Verify type sharing via outer aliases.

```wat
;; Parent defines a record type
(component $parent
  (type $point (record (field "x" s32) (field "y" s32)))

  ;; Child uses the type via outer alias
  (component $child
    (alias outer 1 0 (type $point))  ;; Outer alias to parent's type 0
    (export "origin" (func (result $point)))
  )

  (instance $child-inst (instantiate $child))
  (alias export $child-inst "origin" (func $origin))
  (export "get-origin" (func $origin))
)
```

**Test:**
```go
func TestSharedTypeAcrossComponents(t *testing.T) {
    // ... setup

    inst, err := linker.Instantiate(ctx, parentCompiled)
    require.NoError(t, err)

    originFunc := inst.ExportedFunction("get-origin")
    require.NotNil(t, originFunc)

    result, err := originFunc.Call(ctx)
    require.NoError(t, err)

    // Result should be a record with x and y fields
    record := result[0].Record()
    assert.Equal(t, int32(0), record["x"].S32())
    assert.Equal(t, int32(0), record["y"].S32())
}
```

### 1.4 Resource Passing Between Components

**Purpose:** Verify resource handles can be passed between parent and child.

```wat
(component $parent
  ;; Define a resource type
  (type $handle (resource (rep i32)))

  ;; Child component takes and returns handles
  (component $child
    (import "handle-type" (type $h (sub resource)))
    (export "process" (func (param "h" (own $h)) (result (own $h))))
  )

  ;; Instantiate child with our resource type
  (instance $child-inst (instantiate $child
    (with "handle-type" (type $handle))
  ))

  ;; Create and process a handle
  (func $create-and-process (result (own $handle))
    ;; Create handle, pass to child, get back
  )

  (export "test" (func $create-and-process))
)
```

**Test:**
```go
func TestResourcePassingBetweenComponents(t *testing.T) {
    // ... setup

    inst, err := linker.Instantiate(ctx, compiled)
    require.NoError(t, err)

    testFunc := inst.ExportedFunction("test")
    result, err := testFunc.Call(ctx)
    require.NoError(t, err)

    // Should get back a valid resource handle
    handle := result[0].Own()
    assert.NotNil(t, handle)
}
```

---

## 2. Import/Export Type Matching Tests

### 2.1 Function Parameter Contravariance

**Purpose:** Verify that a function accepting more params can satisfy import expecting fewer.

**Scenario:**
- Import expects: `(func (param s32 s32) (result s32))`
- Linker provides: `(func (param s32 s32 s32) (result s32))`
- Result: Should PASS (extra params ignored)

```go
func TestFunctionContravariance(t *testing.T) {
    linker := component.NewLinker()

    // Define function with extra parameter
    err := linker.DefineFunc("test", "add",
        &component.FuncType{
            Params: []component.NamedValType{
                {Name: "a", ValType: primitiveS32()},
                {Name: "b", ValType: primitiveS32()},
                {Name: "extra", ValType: primitiveS32()}, // Extra
            },
            Results: []component.NamedValType{
                {ValType: primitiveS32()},
            },
        },
        func(ctx context.Context, args []component.Val) ([]component.Val, error) {
            // Only use first two
            return []component.Val{component.ValS32(args[0].S32() + args[1].S32())}, nil
        },
    )
    require.NoError(t, err)

    // Component imports with fewer params
    compBytes := buildTestComponent(t, `
        (component
          (import "test/add" (func $add (param "a" s32) (param "b" s32) (result s32)))
          (export "add" (func $add))
        )
    `)

    compiled, err := rt.CompileComponent(ctx, compBytes)
    require.NoError(t, err)

    // Should instantiate successfully
    inst, err := linker.Instantiate(ctx, compiled)
    require.NoError(t, err)

    // Call should work
    addFunc := inst.ExportedFunction("add")
    result, err := addFunc.Call(ctx, component.ValS32(5), component.ValS32(7))
    require.NoError(t, err)
    assert.Equal(t, int32(12), result[0].S32())
}
```

### 2.2 Function Result Covariance

**Purpose:** Verify result type subtyping.

**Scenario:**
- Import expects: `(func (result (result s32 string)))`
- Linker provides: `(func (result s32))` (subtype of result.ok)
- Result: Implementation-dependent (may need explicit result wrapping)

```go
func TestFunctionCovariance(t *testing.T) {
    // Test that providing a more specific result type works
    // ...
}
```

### 2.3 Instance Width Subtyping

**Purpose:** Verify instance with extra exports can satisfy import expecting fewer.

```go
func TestInstanceWidthSubtyping(t *testing.T) {
    linker := component.NewLinker()

    // Define instance with extra exports
    err := linker.DefineInstance("wasi:test/extra").
        Func("required-fn", func(ctx context.Context, args []component.Val) ([]component.Val, error) {
            return nil, nil
        }).
        Func("optional-fn", func(ctx context.Context, args []component.Val) ([]component.Val, error) {
            return nil, nil
        }).
        Func("another-extra", func(ctx context.Context, args []component.Val) ([]component.Val, error) {
            return nil, nil
        }).
        Build()
    require.NoError(t, err)

    // Component only requires one export
    compBytes := buildTestComponent(t, `
        (component
          (import "wasi:test/extra" (instance
            (export "required-fn" (func))
          ))
        )
    `)

    compiled, err := rt.CompileComponent(ctx, compBytes)
    require.NoError(t, err)

    // Should instantiate - extra exports are OK
    inst, err := linker.Instantiate(ctx, compiled)
    require.NoError(t, err)
    assert.NotNil(t, inst)
}
```

### 2.4 Instance Missing Export

**Purpose:** Verify error when required export is missing.

```go
func TestInstanceMissingExport(t *testing.T) {
    linker := component.NewLinker()

    // Define instance WITHOUT required export
    err := linker.DefineInstance("wasi:test/incomplete").
        Func("some-fn", func(ctx context.Context, args []component.Val) ([]component.Val, error) {
            return nil, nil
        }).
        Build()
    require.NoError(t, err)

    // Component requires "required-fn"
    compBytes := buildTestComponent(t, `
        (component
          (import "wasi:test/incomplete" (instance
            (export "required-fn" (func))
          ))
        )
    `)

    compiled, err := rt.CompileComponent(ctx, compBytes)
    require.NoError(t, err)

    // Should FAIL - missing required export
    _, err = linker.Instantiate(ctx, compiled)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "missing required export")
}
```

### 2.5 Resource Type Equality

**Purpose:** Verify resource types must be exactly equal.

```go
func TestResourceTypeEquality(t *testing.T) {
    linker := component.NewLinker()

    // Define two separate resource types
    err := linker.DefineInstance("wasi:test/res1").
        Resource("handle", func(rep uint32) {}).
        Build()
    require.NoError(t, err)

    err = linker.DefineInstance("wasi:test/res2").
        Resource("handle", func(rep uint32) {}).
        Build()
    require.NoError(t, err)

    // Component uses same resource type in two places
    compBytes := buildTestComponent(t, `
        (component
          (import "wasi:test/res1" (instance $r1
            (export "handle" (type (sub resource)))
          ))
          (import "wasi:test/res2" (instance $r2
            (export "handle" (type (eq (type $r1 "handle"))))
          ))
        )
    `)

    compiled, err := rt.CompileComponent(ctx, compBytes)
    require.NoError(t, err)

    // Should FAIL - res1.handle != res2.handle
    _, err = linker.Instantiate(ctx, compiled)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "resource type mismatch")
}
```

### 2.6 Record Field Subtyping

**Purpose:** Verify record with extra fields satisfies record expecting fewer.

```go
func TestRecordWidthSubtyping(t *testing.T) {
    // Import expects: (record (field "x" s32))
    // Provide: (record (field "x" s32) (field "y" s32))
    // Should PASS
}
```

### 2.7 Variant Refinement

**Purpose:** Verify variant with fewer cases satisfies variant expecting more.

```go
func TestVariantRefinement(t *testing.T) {
    // Import expects: (variant (case "a" s32) (case "b" string))
    // Provide: (variant (case "a" s32))
    // Should PASS (fewer cases is refinement)
}
```

---

## 3. Alias Resolution Tests

### 3.1 Core Export Alias Chain

**Purpose:** Verify chained core export aliases resolve correctly.

```go
func TestCoreExportAliasChain(t *testing.T) {
    // Component with:
    // (core instance $i0 (instantiate ...))
    // (alias core export $i0 "mem" (core memory $m0))
    // (core instance $i1 (instantiate ... (with "memory" (memory $m0))))
    // (alias core export $i1 "fn" (core func $f))
    // Export $f

    // Verify the chain resolves and function is callable
}
```

### 3.2 Component Export Alias

**Purpose:** Verify aliasing from component instance exports.

```go
func TestComponentExportAlias(t *testing.T) {
    // Component with:
    // (import "provider" (instance $p (export "data" (func ...))))
    // (alias export $p "data" (func $get-data))
    // (export "get-data" (func $get-data))

    // Verify aliased function is callable
}
```

### 3.3 Outer Alias - Type

**Purpose:** Verify outer alias to parent's type works.

```go
func TestOuterAliasType(t *testing.T) {
    // Parent with type definition
    // Child with (alias outer 1 X (type ...))
    // Child uses aliased type in function signature

    // Verify function call with aliased type works
}
```

### 3.4 Outer Alias - Module

**Purpose:** Verify outer alias to parent's module works.

```go
func TestOuterAliasModule(t *testing.T) {
    // Parent with embedded module
    // Child with (alias outer 1 X (core module ...))
    // Child instantiates aliased module

    // Verify module is instantiated correctly
}
```

### 3.5 Invalid Outer Alias Depth

**Purpose:** Verify error on invalid outer alias depth.

```go
func TestOuterAliasInvalidDepth(t *testing.T) {
    // Component with outer alias depth > actual nesting
    // Should fail during instantiation
}
```

---

## 4. Start Function Tests

### 4.1 Basic Start Function

**Purpose:** Verify start function executes during instantiation.

```go
func TestBasicStartFunction(t *testing.T) {
    // Component with:
    // (import "name" (value $name string))
    // (func $init (param "name" string) (result string)
    //   ;; prepend "Hello, "
    // )
    // (start $init (value $name) (result (value $greeting)))
    // (export "greeting" (value $greeting))

    linker := component.NewLinker()
    linker.DefineValue("name", component.ValString("World"))

    inst, err := linker.Instantiate(ctx, compiled)
    require.NoError(t, err)

    greeting := inst.ExportedValue("greeting")
    assert.Equal(t, "Hello, World", greeting.StringVal())
}
```

### 4.2 Start Function with Multiple Results

**Purpose:** Verify start function can produce multiple values.

```go
func TestStartFunctionMultipleResults(t *testing.T) {
    // (start $init (result (value $v1)) (result (value $v2)) (result (value $v3)))
    // Verify all three values accessible after instantiation
}
```

### 4.3 Value Consumed Once

**Purpose:** Verify error when value used multiple times.

```go
func TestValueConsumedOnce(t *testing.T) {
    // Component tries to use same value in both start and export
    // Should fail validation
}
```

### 4.4 Start Function Error

**Purpose:** Verify instantiation fails if start function returns error.

```go
func TestStartFunctionError(t *testing.T) {
    linker := component.NewLinker()

    // Start function that always fails
    // ...

    _, err := linker.Instantiate(ctx, compiled)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "start function failed")
}
```

---

## 5. Resource System Tests

### 5.1 Resource New/Drop Lifecycle

**Purpose:** Verify resource creation and destruction.

```go
func TestResourceLifecycle(t *testing.T) {
    var dropped []uint32

    linker := component.NewLinker()
    linker.DefineInstance("wasi:test/res").
        Resource("handle", func(rep uint32) {
            dropped = append(dropped, rep)
        }).
        Func("create", func(ctx context.Context, args []component.Val) ([]component.Val, error) {
            // Create resource with rep=42
            table := component.ResourceTableFromContext(ctx)
            handle := table.New(42)
            return []component.Val{component.ValOwn(handle)}, nil
        }).
        Build()

    // Component creates, uses, and drops resource
    // ...

    // Verify destructor was called
    assert.Contains(t, dropped, uint32(42))
}
```

### 5.2 Resource Borrow Semantics

**Purpose:** Verify borrowed handles can't outlive owner.

```go
func TestResourceBorrowSemantics(t *testing.T) {
    // Function takes borrow, tries to store it
    // Should fail at runtime
}
```

### 5.3 Resource Rep Access

**Purpose:** Verify resource.rep returns correct representation.

```go
func TestResourceRep(t *testing.T) {
    // Create resource with known rep
    // Call resource.rep
    // Verify returned value matches
}
```

---

## 6. Edge Case & Error Tests

### 6.1 Empty Import Name

**Purpose:** Verify empty import module names work (common in components).

```go
func TestEmptyImportName(t *testing.T) {
    // Core module with (import "" "fn" ...)
    // Verify instantiation provides correct function
}
```

### 6.2 Circular Component Import

**Purpose:** Verify error on circular component dependencies.

```go
func TestCircularImport(t *testing.T) {
    // A imports B, B imports A
    // Should fail with cycle detection
}
```

### 6.3 Missing Import

**Purpose:** Verify clear error on missing import.

```go
func TestMissingImport(t *testing.T) {
    linker := component.NewLinker()
    // Don't define required import

    _, err := linker.Instantiate(ctx, compiled)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "import \"wasi:missing/thing\" not found")
}
```

### 6.4 Type Index Out of Range

**Purpose:** Verify error on invalid type index.

```go
func TestTypeIndexOutOfRange(t *testing.T) {
    // Component with invalid type index reference
    // Should fail during instantiation
}
```

### 6.5 Semver Mismatch

**Purpose:** Verify error on incompatible semver.

```go
func TestSemverMismatch(t *testing.T) {
    linker := component.NewLinker()
    linker.DefineInstance("wasi:test/thing@1.0.0", ...) // Major version 1

    // Component requires 2.0.0
    compBytes := buildTestComponent(t, `
        (component
          (import "wasi:test/thing@2.0.0" (instance ...))
        )
    `)

    _, err := linker.Instantiate(ctx, compiled)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "no compatible version")
}
```

### 6.6 Maximum Nesting Depth

**Purpose:** Verify deep nesting works or fails gracefully.

```go
func TestMaximumNestingDepth(t *testing.T) {
    // 10 levels of component nesting
    // Should either work or fail with clear error
}
```

---

## Test Helpers

### Build Test Component

```go
func buildTestComponent(t *testing.T, wat string) []byte {
    // Use wasm-tools or similar to convert WAT to binary
    // Return the component binary
}
```

### Primitive Type Helpers

```go
func primitiveS32() component.ValTypeRef {
    return component.ValTypeRef{IsPrimitive: true, Primitive: 0x7a}
}

func primitiveString() component.ValTypeRef {
    return component.ValTypeRef{IsPrimitive: true, Primitive: 0x73}
}
```

### Load Test Component

```go
func loadTestComponent(name string) []byte {
    path := filepath.Join("testdata", name)
    data, err := os.ReadFile(path)
    if err != nil {
        panic(err)
    }
    return data
}
```

---

## Test Data Organization

```
internal/component/
├── testdata/
│   ├── nested/
│   │   ├── child.wasm
│   │   ├── parent.wasm
│   │   └── multi-level.wasm
│   ├── types/
│   │   ├── func-contravariant.wasm
│   │   ├── instance-width.wasm
│   │   └── resource-eq.wasm
│   ├── aliases/
│   │   ├── core-chain.wasm
│   │   └── outer-type.wasm
│   ├── start/
│   │   ├── basic.wasm
│   │   └── multi-result.wasm
│   └── resources/
│       ├── lifecycle.wasm
│       └── borrow.wasm
├── type_checker_test.go
├── nested_component_test.go
├── alias_resolution_test.go
├── start_function_test.go
└── resource_system_test.go
```
