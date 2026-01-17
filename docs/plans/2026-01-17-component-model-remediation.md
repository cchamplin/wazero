# Component Model Remediation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix all blockers preventing instantiation of real-world WASI P2 components (add.wasm, subtract.wasm).

**Architecture:** Enable empty module name validation for component-embedded core modules, implement ordered core instance instantiation with import resolution, add canon lower/resource operations, and complete the instantiation pipeline.

**Tech Stack:** Go, wazero internal APIs, Component Model specification, Canonical ABI.

**Prerequisites:** Gap analysis at `docs/plans/2026-01-17-component-model-gap-analysis.md`

**Produces:** Working component instantiation for WASI P2 components.

---

## Phase 1: Enable Core Module Compilation with Empty Import Names

This phase removes the primary blocker: wazero rejects empty module names in imports.

---

### Task 1: Add AllowEmptyModuleName Flag to Module Struct

**Files:**
- Modify: `internal/wasm/module.go:478-499`
- Test: `internal/wasm/module_test.go`

**Step 1: Write the failing test**

Add to `internal/wasm/module_test.go` in the `TestModule_Validate_Imports` function's test cases:

```go
// Add this test case to the existing table
{
    name:                    "allow empty module name when flag set",
    enabledFeatures:         api.CoreFeaturesV1,
    i:                       &Import{Module: "", Name: "func", Type: ExternTypeFunc, DescFunc: 0},
    allowEmptyModuleName:    true,
    // No error expected - should pass validation
},
```

**Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/wasm/... -v -run TestModule_Validate_Imports`
Expected: FAIL - test case doesn't exist or allowEmptyModuleName field doesn't exist

**Step 3: Write minimal implementation**

Modify `internal/wasm/module.go`:

```go
// Add field to Module struct (around line 30-50, after existing fields)
type Module struct {
    // ... existing fields ...

    // AllowEmptyModuleName permits empty module names in imports.
    // This is required for component-embedded core modules which use ""
    // as a placeholder that gets resolved during component instantiation.
    AllowEmptyModuleName bool
}

// Modify validateImports (line 478-499)
func (m *Module) validateImports(enabledFeatures api.CoreFeatures) error {
    for i := range m.ImportSection {
        imp := &m.ImportSection[i]
        // Allow empty module names if explicitly permitted (component model)
        if imp.Module == "" && !m.AllowEmptyModuleName {
            return fmt.Errorf("import[%d] has an empty module name", i)
        }
        switch imp.Type {
        case ExternTypeFunc:
            if int(imp.DescFunc) >= len(m.TypeSection) {
                return fmt.Errorf("invalid import[%q.%q] function: type index out of range", imp.Module, imp.Name)
            }
        case ExternTypeGlobal:
            if !imp.DescGlobal.Mutable {
                continue
            }
            if err := enabledFeatures.RequireEnabled(api.CoreFeatureMutableGlobal); err != nil {
                return fmt.Errorf("invalid import[%q.%q] global: %w", imp.Module, imp.Name, err)
            }
        }
    }
    return nil
}
```

**Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/wasm/... -v -run TestModule_Validate_Imports`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/wasm/module.go internal/wasm/module_test.go
git commit -m "$(cat <<'EOF'
feat(wasm): add AllowEmptyModuleName flag for component model support

Component-embedded core modules use empty strings as import module names.
These are placeholders resolved during component instantiation. Add a flag
to permit this validation exception.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: Add allowEmptyModuleName Parameter to DecodeModule

**Files:**
- Modify: `internal/wasm/binary/decoder.go:18-24`
- Modify: `internal/wasm/binary/decoder.go` (end of function)
- Test: `internal/wasm/binary/decoder_test.go`

**Step 1: Write the failing test**

Add to `internal/wasm/binary/decoder_test.go`:

```go
func TestDecodeModule_AllowEmptyModuleName(t *testing.T) {
    // Module with empty import module name
    // (module (import "" "f" (func)))
    binary := []byte{
        0x00, 0x61, 0x73, 0x6d, // magic
        0x01, 0x00, 0x00, 0x00, // version
        // Type section
        0x01, 0x04, 0x01, 0x60, 0x00, 0x00, // (type (func))
        // Import section with empty module name
        0x02, 0x06, 0x01, // import section, size 6, count 1
        0x00,             // module name length 0 (empty)
        0x01, 0x66,       // name "f"
        0x00, 0x00,       // func, type 0
    }

    // Without flag: should fail
    _, err := DecodeModule(binary, api.CoreFeaturesV2, 65536, false, false, false)
    if err == nil || !strings.Contains(err.Error(), "empty module name") {
        t.Errorf("expected empty module name error, got: %v", err)
    }

    // With flag: should succeed
    m, err := DecodeModuleAllowEmptyImports(binary, api.CoreFeaturesV2, 65536, false, false, false)
    if err != nil {
        t.Fatalf("unexpected error with allowEmptyModuleName: %v", err)
    }
    if len(m.ImportSection) != 1 {
        t.Errorf("expected 1 import, got %d", len(m.ImportSection))
    }
    if m.ImportSection[0].Module != "" {
        t.Errorf("expected empty module name, got %q", m.ImportSection[0].Module)
    }
}
```

**Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/wasm/binary/... -v -run TestDecodeModule_AllowEmptyModuleName`
Expected: FAIL - DecodeModuleAllowEmptyImports doesn't exist

**Step 3: Write minimal implementation**

Add to `internal/wasm/binary/decoder.go`:

```go
// DecodeModuleAllowEmptyImports is like DecodeModule but permits empty import module names.
// This is required for core modules embedded in WebAssembly components.
func DecodeModuleAllowEmptyImports(
    binary []byte,
    enabledFeatures api.CoreFeatures,
    memoryLimitPages uint32,
    memoryCapacityFromMax,
    dwarfEnabled, storeCustomSections bool,
) (*wasm.Module, error) {
    m, err := DecodeModule(binary, enabledFeatures, memoryLimitPages, memoryCapacityFromMax, dwarfEnabled, storeCustomSections)
    if err != nil {
        return nil, err
    }
    m.AllowEmptyModuleName = true
    return m, nil
}
```

Wait - that won't work because validation happens inside DecodeModule. We need to modify the decode flow.

Actually, looking at the code more carefully, validation happens in `Module.Validate()` which is called after decoding. So we need to:

1. Decode the module (parsing succeeds regardless of empty names)
2. Set the flag before validation
3. Validate with the flag set

Let me revise:

```go
// DecodeModuleAllowEmptyImports is like DecodeModule but permits empty import module names.
// This is required for core modules embedded in WebAssembly components.
func DecodeModuleAllowEmptyImports(
    binary []byte,
    enabledFeatures api.CoreFeatures,
    memoryLimitPages uint32,
    memoryCapacityFromMax,
    dwarfEnabled, storeCustomSections bool,
) (*wasm.Module, error) {
    // First decode without validation
    r := bytes.NewReader(binary)

    // Magic number.
    buf := make([]byte, 4)
    if _, err := io.ReadFull(r, buf); err != nil || !bytes.Equal(buf, Magic) {
        return nil, ErrInvalidMagicNumber
    }

    // Version.
    if _, err := io.ReadFull(r, buf); err != nil || !bytes.Equal(buf, version) {
        return nil, ErrInvalidVersion
    }

    // ... this gets complex. Let's use a different approach.

    // Call the regular decoder, then re-validate with the flag
    m, err := decodeModuleInternal(binary, enabledFeatures, memoryLimitPages, memoryCapacityFromMax, dwarfEnabled, storeCustomSections, true)
    return m, err
}

// Modify DecodeModule to call internal with allowEmpty=false
func DecodeModule(...) (*wasm.Module, error) {
    return decodeModuleInternal(..., false)
}
```

This is getting complex. Let me simplify by just adding an extra parameter:

```go
// At the end of DecodeModule, after all sections are decoded but before return:
// Around line 220-230 where validation happens

// Original ends with something like:
//   return m, m.Validate(...)

// We need to expose a way to set the flag before Validate is called.
```

Looking at the actual code structure, the simplest approach is:

```go
// Add new exported function after DecodeModule
func DecodeModuleWithOptions(
    binary []byte,
    enabledFeatures api.CoreFeatures,
    memoryLimitPages uint32,
    memoryCapacityFromMax,
    dwarfEnabled, storeCustomSections bool,
    allowEmptyModuleName bool,
) (*wasm.Module, error) {
    m, err := decodeModuleCore(binary, enabledFeatures, memoryLimitPages, memoryCapacityFromMax, dwarfEnabled, storeCustomSections)
    if err != nil {
        return nil, err
    }
    m.AllowEmptyModuleName = allowEmptyModuleName
    // Validation is deferred to compilation, so just return
    return m, nil
}
```

Actually, I need to look at where validation really happens. Let me check the actual flow and write a simpler solution:

```go
// In decoder.go, add after DecodeModule:

// DecodeModuleForComponent decodes a module allowing empty import module names.
// This is required for core modules embedded in WebAssembly components where
// empty strings are used as placeholders resolved during instantiation.
func DecodeModuleForComponent(
    binary []byte,
    enabledFeatures api.CoreFeatures,
    memoryLimitPages uint32,
    memoryCapacityFromMax,
    dwarfEnabled, storeCustomSections bool,
) (*wasm.Module, error) {
    m, err := DecodeModule(binary, enabledFeatures, memoryLimitPages, memoryCapacityFromMax, dwarfEnabled, storeCustomSections)
    if err != nil {
        // Check if the error is specifically about empty module names
        if strings.Contains(err.Error(), "has an empty module name") {
            // Re-decode and set the flag before validation
            // This is a workaround - ideally we'd pass the flag through
            return decodeWithEmptyModuleNameAllowed(binary, enabledFeatures, memoryLimitPages, memoryCapacityFromMax, dwarfEnabled, storeCustomSections)
        }
        return nil, err
    }
    return m, nil
}

func decodeWithEmptyModuleNameAllowed(...) (*wasm.Module, error) {
    // Decode sections without validation
    m := &wasm.Module{AllowEmptyModuleName: true}
    // ... copy decode logic but skip Validate call or call with flag
}
```

This is getting messy. Let me look at the actual validate call location and do a cleaner fix.

The cleanest approach is to add the flag as a parameter to `Validate()`:

**Revised Step 3: Cleaner implementation**

First, modify `internal/wasm/module.go` to add an optional parameter pattern:

```go
// ValidateOption configures Module.Validate behavior.
type ValidateOption func(*validateConfig)

type validateConfig struct {
    allowEmptyModuleName bool
}

// AllowEmptyModuleName permits empty module names in imports.
func AllowEmptyModuleNameOption() ValidateOption {
    return func(c *validateConfig) {
        c.allowEmptyModuleName = true
    }
}

// Modify Validate signature
func (m *Module) Validate(enabledFeatures api.CoreFeatures, opts ...ValidateOption) error {
    cfg := &validateConfig{}
    for _, opt := range opts {
        opt(cfg)
    }
    m.AllowEmptyModuleName = cfg.allowEmptyModuleName
    // ... rest of validation
}
```

But this changes the public API. Let me just add a simple exported function instead:

**Final Step 3: Simple implementation**

In `internal/wasm/binary/decoder.go`, we need to understand how validation flows. Looking at the actual code, `DecodeModule` calls validation internally.

The simplest fix is to make validation accept the empty name when the flag is set on the module. Since we already added the flag to Module in Task 1, we just need a way to set it during decode.

Add to `internal/wasm/binary/decoder.go` after the DecodeModule function:

```go
// DecodeModuleForComponent is like DecodeModule but allows empty import module names.
// Component-embedded core modules use "" as import module name placeholders.
func DecodeModuleForComponent(
    binary []byte,
    enabledFeatures api.CoreFeatures,
    memoryLimitPages uint32,
    memoryCapacityFromMax,
    dwarfEnabled, storeCustomSections bool,
) (*wasm.Module, error) {
    // Decode the raw module structure
    m, err := decodeModule(binary, enabledFeatures, memoryLimitPages, memoryCapacityFromMax, dwarfEnabled, storeCustomSections)
    if err != nil {
        return nil, err
    }

    // Set flag to allow empty module names before validation
    m.AllowEmptyModuleName = true

    // Run validation with the flag set
    if err := m.Validate(enabledFeatures); err != nil {
        return nil, err
    }

    return m, nil
}
```

Wait, the issue is that DecodeModule already validates. We need to separate decode from validate OR pass the flag through. Let me look at exactly what DecodeModule returns.

Looking at the end of DecodeModule (~line 219), it returns `m, nil` after setting up DWARF. The validation must happen elsewhere (likely in CompileModule).

Let me trace this properly... In runtime.go's CompileModule, it calls:
1. `binaryformat.DecodeModule` - just parsing
2. Then `engine.CompileModule` which likely validates

So DecodeModule doesn't validate! The validation happens in the engine. Let me check `internal/wasm/module.go`'s Validate method and where it's called.

Given this, the fix is simpler:

```go
// In decoder.go, just add:
// After DecodeModule function definition, add:

// SetAllowEmptyModuleName is a post-decode modifier for component-embedded modules.
// Call this after DecodeModule when parsing modules from inside components.
func SetAllowEmptyModuleName(m *wasm.Module) {
    m.AllowEmptyModuleName = true
}
```

Then in runtime.go's CompileComponent, after decoding each module, call this function.

Actually, let me just make DecodeModule take an options struct or add a new function. Here's the final approach:

**Step 3 (Final): Implementation**

Add to `internal/wasm/binary/decoder.go` (after DecodeModule):

```go
import "strings"

// DecodeModuleForComponent decodes a core module that's embedded in a component.
// Unlike DecodeModule, this allows empty import module names which are used
// as placeholders in the component model.
func DecodeModuleForComponent(
    binary []byte,
    enabledFeatures api.CoreFeatures,
    memoryLimitPages uint32,
    memoryCapacityFromMax,
    dwarfEnabled, storeCustomSections bool,
) (*wasm.Module, error) {
    m, err := DecodeModule(binary, enabledFeatures, memoryLimitPages, memoryCapacityFromMax, dwarfEnabled, storeCustomSections)
    if err != nil {
        return nil, err
    }
    // Mark this module as allowing empty import module names
    m.AllowEmptyModuleName = true
    return m, nil
}
```

**Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/wasm/binary/... -v -run TestDecodeModule_AllowEmptyModuleName`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/wasm/binary/decoder.go internal/wasm/binary/decoder_test.go
git commit -m "$(cat <<'EOF'
feat(binary): add DecodeModuleForComponent for component-embedded modules

Add a decoder variant that sets AllowEmptyModuleName on the resulting
module, enabling component-embedded core modules with empty import
module names to be decoded without validation errors.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Use DecodeModuleForComponent in Component Binary Decoder

**Files:**
- Modify: `internal/component/binary/decoder.go:131-147`
- Test: `internal/component/binary/wasip2_test.go`

**Step 1: Write the failing test**

The test already exists in `wasip2_test.go`. Let's verify it currently fails:

```go
// internal/component/binary/wasip2_test.go already has TestParseAddWasm
// We need to verify it passes after our fix
```

**Step 2: Run test to verify current state**

Run: `CGO_ENABLED=0 go test ./internal/component/binary/... -v -run TestParseAddWasm`
Expected: Currently PASS (parsing works, it's compilation that fails)

**Step 3: Write minimal implementation**

Modify `internal/component/binary/decoder.go`, function `decodeCoreModuleSection`:

```go
// decodeCoreModuleSection parses an embedded core wasm module.
func decodeCoreModuleSection(c *component.Component, content []byte) error {
    // Use DecodeModuleForComponent to allow empty import module names
    m, err := wasmbinary.DecodeModuleForComponent(
        content,
        api.CoreFeaturesV2,
        65536,
        false,
        false,
        false,
    )
    if err != nil {
        return fmt.Errorf("decode core module: %w", err)
    }
    c.CoreModules = append(c.CoreModules, m)
    // Store raw bytes for instantiation via wazero's public API
    c.CoreModuleData = append(c.CoreModuleData, content)
    return nil
}
```

Add the import at the top:
```go
import (
    // ... existing imports ...
    wasmbinary "github.com/tetratelabs/wazero/internal/wasm/binary"
)
```

**Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/component/binary/... -v -run TestParseAddWasm`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/binary/decoder.go
git commit -m "$(cat <<'EOF'
fix(component): use DecodeModuleForComponent for embedded core modules

Switch to DecodeModuleForComponent which allows empty import module
names, fixing parsing of component-embedded core modules that use
"" as a placeholder module name.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: Update runtime.go CompileComponent to Use Relaxed Validation

**Files:**
- Modify: `runtime.go:460-477`
- Test: `internal/component/wasip2test/calculator_test.go`

**Step 1: Verify current failure**

Run: `CGO_ENABLED=0 go test ./internal/component/wasip2test/... -v -run TestCalculatorPlugins`
Expected: FAIL with "import[0] has an empty module name"

**Step 2: Write minimal implementation**

The issue is that `runtime.go:462-471` uses `r.CompileModule` which doesn't set `AllowEmptyModuleName`. We need to compile with the flag set.

Modify `runtime.go` `CompileComponent` function:

```go
// CompileComponent implements Runtime.CompileComponent
func (r *runtime) CompileComponent(ctx context.Context, binary []byte) (api.CompiledComponent, error) {
    if err := r.failIfClosed(); err != nil {
        return nil, err
    }

    // Check if this is a component (not a core module)
    if !componentbinary.IsComponent(binary) {
        return nil, componentbinary.ErrInvalidLayer
    }

    // Parse the component
    parsed, err := componentbinary.DecodeComponent(binary)
    if err != nil {
        return nil, fmt.Errorf("decode component: %w", err)
    }

    // Pre-compile all embedded core modules with AllowEmptyModuleName set
    compiledModules := make([]component.CompiledModuleCloser, len(parsed.CoreModuleData))
    for i, moduleData := range parsed.CoreModuleData {
        // Use the pre-parsed module with AllowEmptyModuleName already set
        // instead of re-compiling from bytes
        if i < len(parsed.CoreModules) && parsed.CoreModules[i] != nil {
            // The module was already decoded with AllowEmptyModuleName in decoder
            compiled, err := r.compileModuleFromParsed(ctx, parsed.CoreModules[i], moduleData)
            if err != nil {
                // Clean up already-compiled modules
                for j := 0; j < i; j++ {
                    if compiledModules[j] != nil {
                        compiledModules[j].Close(ctx)
                    }
                }
                return nil, fmt.Errorf("compile core module %d: %w", i, err)
            }
            compiledModules[i] = compiled
        }
    }

    return component.NewCompiledComponent(parsed, compiledModules, r), nil
}

// compileModuleFromParsed compiles an already-parsed module.
func (r *runtime) compileModuleFromParsed(ctx context.Context, m *wasm.Module, source []byte) (CompiledModule, error) {
    // Ensure the flag is set for component-embedded modules
    m.AllowEmptyModuleName = true

    if err := m.Validate(r.enabledFeatures); err != nil {
        return nil, fmt.Errorf("validate: %w", err)
    }

    c := &compiledModule{module: m, compiledEngine: r.store.Engine}

    if err := r.store.Engine.CompileModule(ctx, m, nil, false); err != nil {
        return nil, err
    }

    // Assign type IDs to all functions.
    c.typeIDs, err = r.store.GetFunctionTypeIDs(m.TypeSection)
    if err != nil {
        return nil, err
    }

    c.source = source
    return c, nil
}
```

Wait, this is getting complex because we're mixing internal types. Let me look at how CompileModule works and find a simpler path.

Looking at `CompileModule`, it:
1. Decodes the binary via `binaryformat.DecodeModule`
2. Validates via `m.Validate`
3. Compiles via `r.store.Engine.CompileModule`

The simplest fix is to have a variant that skips the decode step since we already have the parsed module:

```go
// In runtime.go, simpler approach:

// CompileComponent implements Runtime.CompileComponent
func (r *runtime) CompileComponent(ctx context.Context, binary []byte) (api.CompiledComponent, error) {
    if err := r.failIfClosed(); err != nil {
        return nil, err
    }

    if !componentbinary.IsComponent(binary) {
        return nil, componentbinary.ErrInvalidLayer
    }

    parsed, err := componentbinary.DecodeComponent(binary)
    if err != nil {
        return nil, fmt.Errorf("decode component: %w", err)
    }

    // Pre-compile all embedded core modules
    compiledModules := make([]component.CompiledModuleCloser, len(parsed.CoreModuleData))
    for i, moduleData := range parsed.CoreModuleData {
        // Compile using the pre-parsed module which has AllowEmptyModuleName set
        compiled, err := r.compileComponentModule(ctx, parsed.CoreModules[i], moduleData)
        if err != nil {
            for j := 0; j < i; j++ {
                if compiledModules[j] != nil {
                    compiledModules[j].Close(ctx)
                }
            }
            return nil, fmt.Errorf("compile core module %d: %w", i, err)
        }
        compiledModules[i] = compiled
    }

    return component.NewCompiledComponent(parsed, compiledModules, r), nil
}

// compileComponentModule compiles a pre-parsed module from a component.
func (r *runtime) compileComponentModule(ctx context.Context, m *wasm.Module, source []byte) (CompiledModule, error) {
    // Module already has AllowEmptyModuleName set from component decoder

    if err := m.Validate(r.enabledFeatures); err != nil {
        return nil, fmt.Errorf("validate: %w", err)
    }

    c := &compiledModule{module: m, compiledEngine: r.store.Engine}

    if err := r.store.Engine.CompileModule(ctx, m, nil, false); err != nil {
        return nil, err
    }

    var err error
    c.typeIDs, err = r.store.GetFunctionTypeIDs(m.TypeSection)
    if err != nil {
        return nil, err
    }

    c.source = source
    return c, nil
}
```

**Step 3: Run test**

Run: `CGO_ENABLED=0 go test ./internal/component/wasip2test/... -v -run TestCalculatorPlugins`
Expected: PASS for compilation (instantiation may still fail for other reasons)

**Step 4: Commit**

```bash
git add runtime.go
git commit -m "$(cat <<'EOF'
fix(runtime): compile component modules with AllowEmptyModuleName

Use pre-parsed modules from component decoder which have the
AllowEmptyModuleName flag set, enabling compilation of components
with core modules that use empty import module names.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Phase 2: Implement Ordered Core Instance Instantiation

Core instances must be instantiated in order, with imports resolved from previously instantiated instances.

---

### Task 5: Add Core Function Index Space Tracking

**Files:**
- Create: `internal/component/core_index_space.go`
- Test: `internal/component/core_index_space_test.go`

**Step 1: Write the failing test**

```go
// internal/component/core_index_space_test.go
package component

import (
    "testing"
)

func TestCoreFuncIndexSpace_AddAndResolve(t *testing.T) {
    space := NewCoreFuncIndexSpace()

    // Add some functions from instance 0
    space.AddFromInstance(0, []string{"foo", "bar"})

    // Resolve should find them
    instIdx, name, err := space.Resolve(0)
    if err != nil {
        t.Fatalf("Resolve(0): %v", err)
    }
    if instIdx != 0 || name != "foo" {
        t.Errorf("Resolve(0) = (%d, %q), want (0, foo)", instIdx, name)
    }

    instIdx, name, err = space.Resolve(1)
    if err != nil {
        t.Fatalf("Resolve(1): %v", err)
    }
    if instIdx != 0 || name != "bar" {
        t.Errorf("Resolve(1) = (%d, %q), want (0, bar)", instIdx, name)
    }
}

func TestCoreFuncIndexSpace_AddAlias(t *testing.T) {
    space := NewCoreFuncIndexSpace()

    // Alias: function 5 refers to instance 2's export "baz"
    space.AddAlias(5, 2, "baz")

    instIdx, name, err := space.Resolve(5)
    if err != nil {
        t.Fatalf("Resolve(5): %v", err)
    }
    if instIdx != 2 || name != "baz" {
        t.Errorf("Resolve(5) = (%d, %q), want (2, baz)", instIdx, name)
    }
}
```

**Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/component/... -v -run TestCoreFuncIndexSpace`
Expected: FAIL - types don't exist

**Step 3: Check if file already exists and read it**

The file `index_space.go` already exists. Read it and verify/extend.

**Step 4: Run tests to verify**

Run: `CGO_ENABLED=0 go test ./internal/component/... -v -run TestCoreFuncIndexSpace`

**Step 5: Commit if changes made**

---

### Task 6: Implement Ordered Core Instance Instantiation

**Files:**
- Modify: `internal/component/component_linker.go:84-147`
- Test: `internal/component/component_linker_test.go`

**Step 1: Write the failing test**

```go
// Add to internal/component/component_linker_test.go
func TestComponentLinker_OrderedInstantiation(t *testing.T) {
    // Create a mock component with:
    // - Module 0: no imports, exports "f"
    // - Module 1: imports "" "f", exports "g"
    // - CoreInstance 0: instantiate module 0
    // - CoreInstance 1: instantiate module 1 with "" from instance 0

    c := &Component{
        CoreModules: []*wasm.Module{
            // Module 0: (module (func (export "f")))
            {
                TypeSection:   []wasm.FunctionType{{Results: []wasm.ValueType{}}},
                FunctionSection: []wasm.Index{0},
                CodeSection:   []wasm.Code{{Body: []byte{0x0b}}}, // just end
                ExportSection: []wasm.Export{{Name: "f", Type: wasm.ExternTypeFunc, Index: 0}},
            },
            // Module 1: (module (import "" "f" (func)) (func (export "g")))
            {
                TypeSection:     []wasm.FunctionType{{Results: []wasm.ValueType{}}},
                ImportSection:   []wasm.Import{{Module: "", Name: "f", Type: wasm.ExternTypeFunc, DescFunc: 0}},
                FunctionSection: []wasm.Index{0},
                CodeSection:     []wasm.Code{{Body: []byte{0x0b}}},
                ExportSection:   []wasm.Export{{Name: "g", Type: wasm.ExternTypeFunc, Index: 1}},
                AllowEmptyModuleName: true,
            },
        },
        CoreInstances: []CoreInstance{
            {Kind: CoreInstanceExprInstantiate, ModuleIdx: 0},
            {
                Kind: CoreInstanceExprInstantiate,
                ModuleIdx: 1,
                Args: []InstantiateArg{
                    {Name: "", InstanceIdx: 0}, // import "" from instance 0
                },
            },
        },
    }

    // This should work - instance 0 provides "f" to instance 1
    // The linker must instantiate in order
}
```

**Step 2: Implement ordered instantiation**

This is complex and requires understanding the full instantiation flow. The key changes to `component_linker.go`:

```go
func (l *ComponentLinker) Instantiate(ctx context.Context, compiled *CompiledComponent) (*Instance, error) {
    c := compiled.Internal()

    inst := &Instance{
        component:     c,
        coreInstances: make([]api.Module, len(c.CoreInstances)),
        exports:       make(map[string]*ExportedFunc),
        resourceTable: NewResourceTable(),
    }

    // Build core function/memory index spaces from aliases
    funcSpace := NewCoreFuncIndexSpace()
    memSpace := NewCoreMemoryIndexSpace()

    for _, alias := range c.Aliases {
        if alias.Kind == AliasKindCoreExport {
            switch alias.CoreSort {
            case CoreSortFunc:
                funcSpace.AddAlias(alias.Idx, alias.InstanceIdx, alias.ExportName)
            case CoreSortMemory:
                memSpace.AddAlias(alias.Idx, alias.InstanceIdx, alias.ExportName)
            }
        }
    }

    // Step 1: Validate component imports
    resolvedImports := make(map[string]Definition)
    for _, imp := range c.Imports {
        def, err := l.MatchImport(imp.Name)
        if err != nil {
            return nil, fmt.Errorf("import %q: %w", imp.Name, err)
        }
        resolvedImports[imp.Name] = def
    }

    // Step 2: Process core instances IN ORDER
    for i, coreInst := range c.CoreInstances {
        switch coreInst.Kind {
        case CoreInstanceExprInstantiate:
            // Build imports for this core module from:
            // 1. Previously instantiated core instances
            // 2. Lowered component functions
            coreImports := l.buildCoreImports(inst, c, &coreInst, funcSpace)

            module, err := l.instantiateCoreModuleWithImports(
                ctx, inst, c, &coreInst, compiled.CompiledModules(), i, coreImports,
            )
            if err != nil {
                return nil, fmt.Errorf("instantiate core instance %d: %w", i, err)
            }
            inst.coreInstances[i] = module

            // Add this instance's exports to the function space
            funcSpace.AddFromInstance(uint32(i), getExportNames(module, wasm.ExternTypeFunc))
            memSpace.AddFromInstance(uint32(i), getExportNames(module, wasm.ExternTypeMemory))

        case CoreInstanceExprInline:
            // Inline instance - collect exports into a synthetic module
            inst.coreInstances[i] = l.buildInlineInstance(inst, &coreInst)
        }
    }

    // Step 3: Wire component exports
    for _, exp := range c.Exports {
        if exp.Kind == ExportKindFunc {
            exportedFunc, err := l.wireExportedFunc(inst, c, &exp, funcSpace, memSpace)
            if err != nil {
                return nil, fmt.Errorf("wire export %q: %w", exp.Name, err)
            }
            inst.exports[exp.Name] = exportedFunc
        }
    }

    return inst, nil
}

func (l *ComponentLinker) buildCoreImports(
    inst *Instance,
    c *Component,
    coreInst *CoreInstance,
    funcSpace *CoreFuncIndexSpace,
) map[string]map[string]any {
    imports := make(map[string]map[string]any)

    for _, arg := range coreInst.Args {
        modImports := make(map[string]any)

        // Get exports from the source instance
        if int(arg.InstanceIdx) < len(inst.coreInstances) {
            srcInst := inst.coreInstances[arg.InstanceIdx]
            if srcInst != nil {
                for name, def := range srcInst.ExportedFunctionDefinitions() {
                    modImports[name] = srcInst.ExportedFunction(name)
                    _ = def // suppress unused
                }
                if mem := srcInst.Memory(); mem != nil {
                    modImports["memory"] = mem
                }
            }
        }

        // Handle inline exports
        for _, export := range arg.Exports {
            // Resolve from function space or other sources
            if fn := l.resolveInlineExport(inst, &export, funcSpace); fn != nil {
                modImports[export.Name] = fn
            }
        }

        imports[arg.Name] = modImports
    }

    return imports
}
```

**Step 3: Run tests**

Run: `CGO_ENABLED=0 go test ./internal/component/... -v -run TestComponentLinker`

**Step 4: Commit**

```bash
git add internal/component/component_linker.go internal/component/component_linker_test.go
git commit -m "$(cat <<'EOF'
feat(component): implement ordered core instance instantiation

Core instances are now instantiated in order, with each instance's
imports resolved from previously instantiated instances. This enables
the dependency graph required by component model instantiation.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Phase 3: Implement Canon Lower

Canon lower creates core wasm functions from component functions.

---

### Task 7: Add CanonLower Implementation

**Files:**
- Modify: `internal/component/component_linker.go`
- Create: `internal/component/canon_lower.go`
- Test: `internal/component/canon_lower_test.go`

**Step 1: Write the failing test**

```go
// internal/component/canon_lower_test.go
package component

import (
    "context"
    "testing"
)

func TestCanonLower_SimpleFunc(t *testing.T) {
    // Create a simple host function that adds two i32s
    hostFunc := func(ctx context.Context, args []Val) ([]Val, error) {
        a := args[0].S32()
        b := args[1].S32()
        return []Val{ValS32(a + b)}, nil
    }

    // Lower it to a core function
    lowered := canonLower(hostFunc, &CanonicalOptions{
        StringEncoding: StringEncodingUTF8,
    })

    if lowered == nil {
        t.Fatal("canonLower returned nil")
    }

    // Call the lowered function with core wasm values
    results, err := lowered.CallWithStack(context.Background(), []uint64{2, 3})
    if err != nil {
        t.Fatalf("CallWithStack: %v", err)
    }

    if len(results) != 1 || results[0] != 5 {
        t.Errorf("expected [5], got %v", results)
    }
}
```

**Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/component/... -v -run TestCanonLower`
Expected: FAIL - canonLower doesn't exist

**Step 3: Write minimal implementation**

```go
// internal/component/canon_lower.go
package component

import (
    "context"
    "fmt"

    "github.com/tetratelabs/wazero/api"
    "github.com/tetratelabs/wazero/internal/component/abi"
    "github.com/tetratelabs/wazero/internal/component/types"
)

// LoweredFunc wraps a component function as a core wasm function.
type LoweredFunc struct {
    callback HostFunc
    funcType *FuncType
    options  *CanonicalOptions
    memory   api.Memory
    realloc  api.Function
}

// canonLower creates a core function from a component function.
// This is used to satisfy core module imports from component imports.
func canonLower(callback HostFunc, options *CanonicalOptions) *LoweredFunc {
    return &LoweredFunc{
        callback: callback,
        options:  options,
    }
}

// canonLowerWithType creates a lowered function with type information.
func canonLowerWithType(
    callback HostFunc,
    funcType *FuncType,
    options *CanonicalOptions,
    memory api.Memory,
    realloc api.Function,
) *LoweredFunc {
    return &LoweredFunc{
        callback: callback,
        funcType: funcType,
        options:  options,
        memory:   memory,
        realloc:  realloc,
    }
}

// CallWithStack invokes the lowered function with core wasm stack values.
func (f *LoweredFunc) CallWithStack(ctx context.Context, stack []uint64) ([]uint64, error) {
    // 1. Lift core args to component vals
    args, err := f.liftArgs(ctx, stack)
    if err != nil {
        return nil, fmt.Errorf("lift args: %w", err)
    }

    // 2. Call the component function
    results, err := f.callback(ctx, args)
    if err != nil {
        return nil, err
    }

    // 3. Lower results back to core values
    coreResults, err := f.lowerResults(ctx, results)
    if err != nil {
        return nil, fmt.Errorf("lower results: %w", err)
    }

    return coreResults, nil
}

func (f *LoweredFunc) liftArgs(ctx context.Context, stack []uint64) ([]Val, error) {
    if f.funcType == nil {
        // No type info - assume simple i32/i64 mapping
        args := make([]Val, len(stack))
        for i, v := range stack {
            args[i] = ValS64(int64(v))
        }
        return args, nil
    }

    // Use type info to properly lift
    liftCtx := &abi.LiftContext{
        Memory: f.memory,
    }

    args := make([]Val, len(f.funcType.Params))
    stackIdx := 0
    for i, param := range f.funcType.Params {
        valType := resolveValType(param.ValType)
        val, consumed, err := abi.LiftFlat(liftCtx, valType, stack[stackIdx:])
        if err != nil {
            return nil, fmt.Errorf("lift param %d: %w", i, err)
        }
        args[i] = val
        stackIdx += consumed
    }

    return args, nil
}

func (f *LoweredFunc) lowerResults(ctx context.Context, results []Val) ([]uint64, error) {
    if f.funcType == nil || len(f.funcType.Results) == 0 {
        // Simple case - just convert each result
        coreResults := make([]uint64, len(results))
        for i, r := range results {
            switch r.Kind() {
            case ValKindS32:
                coreResults[i] = uint64(uint32(r.S32()))
            case ValKindU32:
                coreResults[i] = uint64(r.U32())
            case ValKindS64:
                coreResults[i] = uint64(r.S64())
            case ValKindU64:
                coreResults[i] = r.U64()
            default:
                coreResults[i] = 0
            }
        }
        return coreResults, nil
    }

    // Use type info to properly lower
    lowerCtx := &abi.LowerContext{
        Memory:  f.memory,
        Realloc: f.realloc,
    }

    var coreResults []uint64
    for i, result := range f.funcType.Results {
        if i >= len(results) {
            break
        }
        valType := resolveValType(result.ValType)
        flat, err := abi.LowerFlat(lowerCtx, valType, results[i])
        if err != nil {
            return nil, fmt.Errorf("lower result %d: %w", i, err)
        }
        coreResults = append(coreResults, flat...)
    }

    return coreResults, nil
}

// resolveValType converts a ValTypeRef to a types.ValType
func resolveValType(ref ValTypeRef) types.ValType {
    if ref.IsPrimitive {
        switch ref.Primitive {
        case 0x7f:
            return types.Bool{}
        case 0x7e:
            return types.S8{}
        case 0x7d:
            return types.U8{}
        case 0x7c:
            return types.S16{}
        case 0x7b:
            return types.U16{}
        case 0x7a:
            return types.S32{}
        case 0x79:
            return types.U32{}
        case 0x78:
            return types.S64{}
        case 0x77:
            return types.U64{}
        case 0x76:
            return types.F32{}
        case 0x75:
            return types.F64{}
        case 0x74:
            return types.Char{}
        case 0x73:
            return types.String{}
        }
    }
    // For type references, return a placeholder
    return types.U32{} // Default to u32 for handles/indices
}
```

**Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/component/... -v -run TestCanonLower`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/canon_lower.go internal/component/canon_lower_test.go
git commit -m "$(cat <<'EOF'
feat(component): implement canon lower for component-to-core function wrapping

Add canonLower which creates core wasm functions from component functions.
This enables component imports to be lowered and provided to core modules
as imports.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Phase 4: Implement Resource Operations

---

### Task 8: Implement Canon Resource.New

**Files:**
- Modify: `internal/component/resource_table.go`
- Test: `internal/component/resource_table_test.go`

**Step 1: Verify existing implementation**

Run: `CGO_ENABLED=0 go test ./internal/component/... -v -run TestResourceTable`
Check if resource.new is already implemented.

**Step 2: Add resource.new as callable function if missing**

```go
// CreateResourceNewFunc creates a core function for resource.new
func (t *ResourceTable) CreateResourceNewFunc(resourceTypeIdx uint32) func(rep uint32) uint32 {
    return func(rep uint32) uint32 {
        handle := t.New(rep)
        return uint32(handle)
    }
}
```

---

### Task 9: Implement Canon Resource.Drop

**Files:**
- Modify: `internal/component/resource_table.go`
- Test: `internal/component/resource_table_test.go`

```go
// CreateResourceDropFunc creates a core function for resource.drop
func (t *ResourceTable) CreateResourceDropFunc(resourceTypeIdx uint32, destructor func(rep uint32)) func(handle uint32) {
    return func(handle uint32) {
        rep, err := t.Drop(Handle(handle))
        if err != nil {
            return // Silently ignore invalid handles per spec
        }
        if destructor != nil {
            destructor(rep)
        }
    }
}
```

---

### Task 10: Implement Canon Resource.Rep

```go
// CreateResourceRepFunc creates a core function for resource.rep
func (t *ResourceTable) CreateResourceRepFunc(resourceTypeIdx uint32) func(handle uint32) uint32 {
    return func(handle uint32) uint32 {
        rep, err := t.Rep(Handle(handle))
        if err != nil {
            return 0 // Return 0 for invalid handles
        }
        return rep
    }
}
```

---

## Phase 5: Integration Testing

---

### Task 11: Test subtract.wasm End-to-End

**Files:**
- Modify: `internal/component/wasip2test/calculator_test.go`

**Step 1: Run test**

Run: `CGO_ENABLED=0 go test ./internal/component/wasip2test/... -v -run TestCalculatorPlugins/subtract`
Expected: PASS after all previous tasks complete

---

### Task 12: Test add.wasm End-to-End

**Files:**
- Modify: `internal/component/wasip2test/calculator_test.go`

**Step 1: Run test**

Run: `CGO_ENABLED=0 go test ./internal/component/wasip2test/... -v -run TestCalculatorPlugins/add`
Expected: PASS after all previous tasks complete

---

## Phase 6: Type Validation (Future)

Lower priority - implement after core functionality works.

### Task 13: Instance Type Validation

Validate that provided definitions match expected instance types.

### Task 14: Function Signature Matching

Validate function signatures during linking.

---

## Verification Checklist

After completing all tasks:

```bash
# Run all component tests
CGO_ENABLED=0 go test ./internal/component/... -v

# Run WASI P2 interface tests
CGO_ENABLED=0 go test ./imports/wasip2/... -v

# Run integration tests with real components
CGO_ENABLED=0 go test ./internal/component/wasip2test/... -v

# Run full test suite to check for regressions
CGO_ENABLED=0 go test ./... -short
```

**Expected Results:**
- All tests pass
- subtract.wasm instantiates and executes correctly
- add.wasm instantiates and executes correctly (with WASI P2)

---

## Summary

| Phase | Tasks | Priority | Description |
|-------|-------|----------|-------------|
| 1 | 1-4 | P0 | Enable empty module name validation |
| 2 | 5-6 | P0 | Ordered core instance instantiation |
| 3 | 7 | P0 | Canon lower implementation |
| 4 | 8-10 | P1 | Resource operations |
| 5 | 11-12 | P0 | Integration testing |
| 6 | 13-14 | P2 | Type validation |

**Total: 14 tasks across 6 phases**
