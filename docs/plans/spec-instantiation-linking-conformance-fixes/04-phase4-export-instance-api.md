# Phase 4: Export Instance API

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Expose exported instances through the public API so users can access nested component exports.

**Architecture:** Track exported instances during instantiation, implement `ExportedInstance` method in API wrapper, create `ComponentInstanceWrapper` for nested access.

**Tech Stack:** Go

**Parent Plan:** [00-root.md](./00-root.md)

**Prerequisite:** Phase 3 (Nested Components) must be complete.

**Gap Analysis Reference:** [Section 5: Export Handling](../2026-01-20-instantiation-linking-gap-analysis.md#5-export-handling)

---

## Spec References

Read these before starting:
- `debug-vendored/component-model/design/mvp/Explainer.md` lines 2481-2497 (Export semantics)

Key requirements:
- Exports append new elements to index space of exported sort
- Instance exports should be accessible for calling their functions

---

## Task 1: Track Exported Instances

**Files:**
- Modify: `internal/component/instance.go`
- Test: `internal/component/instance_test.go`

### Step 1: Write the failing test for exported instances

```go
// Add to internal/component/instance_test.go

func TestInstance_ExportedInstances(t *testing.T) {
	inst := &Instance{}
	childInst := &Instance{}

	inst.AddExportedInstance("my-service", childInst)

	retrieved := inst.GetExportedInstance("my-service")
	if retrieved != childInst {
		t.Error("exported instance should match")
	}

	// Non-existent should return nil
	missing := inst.GetExportedInstance("not-found")
	if missing != nil {
		t.Error("non-existent export should return nil")
	}
}
```

### Step 2: Run test to verify it fails

Run: `CGO_ENABLED=0 go test ./internal/component/... -run TestInstance_ExportedInstances -v`

Expected: FAIL with "undefined: inst.AddExportedInstance"

### Step 3: Write minimal implementation

```go
// Add to internal/component/instance.go

// In the Instance struct, add:
//
// exportedInstances map[string]*Instance

// AddExportedInstance adds an instance to the exported instances map.
func (i *Instance) AddExportedInstance(name string, inst *Instance) {
	if i.exportedInstances == nil {
		i.exportedInstances = make(map[string]*Instance)
	}
	i.exportedInstances[name] = inst
}

// GetExportedInstance retrieves an exported instance by name.
func (i *Instance) GetExportedInstance(name string) *Instance {
	if i.exportedInstances == nil {
		return nil
	}
	return i.exportedInstances[name]
}
```

### Step 4: Run test to verify it passes

Run: `CGO_ENABLED=0 go test ./internal/component/... -run TestInstance_ExportedInstances -v`

Expected: PASS

### Step 5: Commit

```bash
git add internal/component/instance.go internal/component/instance_test.go
git commit -m "feat(component): add exported instances tracking

Instances can be exported by name for API access.

Refs: docs/plans/spec-instantiation-linking-conformance-fixes"
```

---

## Task 2: Wire Instance Exports During Instantiation

**Files:**
- Modify: `internal/component/component_linker.go`

### Step 1: Find the export wiring section

Locate in `component_linker.go`:
```go
// Step 3: Wire exports
for _, exp := range c.Exports {
    if exp.Kind == ExportKindFunc {
```

### Step 2: Add instance export handling

```go
// Modify the export wiring loop to handle instance exports:

// Step 3: Wire exports
for _, exp := range c.Exports {
	switch exp.Kind {
	case ExportKindFunc:
		exportedFunc, err := l.wireExportedFunc(inst, c, &exp, funcSpace, memSpace)
		if err != nil {
			return nil, fmt.Errorf("wire export %q: %w", exp.Name, err)
		}
		inst.exports[exp.Name] = exportedFunc

	case ExportKindInstance:
		// Look up instance from instance index space
		exportedInst := inst.GetInstanceFromSpace(exp.Idx)
		if exportedInst != nil {
			inst.AddExportedInstance(exp.Name, exportedInst)
		}
	}
}
```

### Step 3: Run tests to verify no regression

Run: `CGO_ENABLED=0 go test ./internal/component/... -v`

Expected: All PASS

### Step 4: Commit

```bash
git add internal/component/component_linker.go
git commit -m "feat(component): wire instance exports during instantiation

Instance exports are now tracked and accessible.

Refs: docs/plans/spec-instantiation-linking-conformance-fixes"
```

---

## Task 3: Create ComponentInstanceWrapper

**Files:**
- Modify: `internal/component/linker_api.go`
- Test: `internal/component/linker_api_test.go`

### Step 1: Write the failing test for instance wrapper

```go
// internal/component/linker_api_test.go (create if doesn't exist)
package component

import (
	"context"
	"testing"
)

func TestComponentInstanceWrapper(t *testing.T) {
	// Create an instance with an exported function
	inst := &Instance{
		exports: map[string]*ExportedFunc{
			"greet": {
				name: "greet",
				funcType: &FuncType{
					Results: []NamedValType{{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x73}}},
				},
			},
		},
	}

	wrapper := &ComponentInstanceWrapper{instance: inst}

	// Should find the function
	fn := wrapper.ExportedFunction("greet")
	if fn == nil {
		t.Error("ExportedFunction should find 'greet'")
	}

	// Should return nil for missing
	missing := wrapper.ExportedFunction("not-found")
	if missing != nil {
		t.Error("ExportedFunction should return nil for missing")
	}
}
```

### Step 2: Run test to verify it fails

Run: `CGO_ENABLED=0 go test ./internal/component/... -run TestComponentInstanceWrapper -v`

Expected: FAIL with "undefined: ComponentInstanceWrapper"

### Step 3: Write minimal implementation

```go
// Add to internal/component/linker_api.go

// ComponentInstanceWrapper wraps an exported component instance for API access.
type ComponentInstanceWrapper struct {
	instance *Instance
}

// ExportedFunction returns an exported function by name.
func (w *ComponentInstanceWrapper) ExportedFunction(name string) api.ComponentFunc {
	if w.instance == nil {
		return nil
	}
	fn, ok := w.instance.exports[name]
	if !ok || fn == nil {
		return nil
	}
	return &ComponentFuncWrapper{fn: fn}
}

// ExportedInstance returns a nested exported instance by name.
func (w *ComponentInstanceWrapper) ExportedInstance(name string) api.ComponentInstance {
	if w.instance == nil {
		return nil
	}
	nested := w.instance.GetExportedInstance(name)
	if nested == nil {
		return nil
	}
	return &ComponentInstanceWrapper{instance: nested}
}
```

### Step 4: Run test to verify it passes

Run: `CGO_ENABLED=0 go test ./internal/component/... -run TestComponentInstanceWrapper -v`

Expected: PASS

### Step 5: Commit

```bash
git add internal/component/linker_api.go internal/component/linker_api_test.go
git commit -m "feat(component): add ComponentInstanceWrapper for API access

Wraps nested instances for function and instance access.

Refs: docs/plans/spec-instantiation-linking-conformance-fixes"
```

---

## Task 4: Implement ComponentWrapper.ExportedInstance

**Files:**
- Modify: `internal/component/linker_api.go`
- Test: `internal/component/linker_api_test.go`

### Step 1: Write the failing test for ExportedInstance

```go
// Add to internal/component/linker_api_test.go

func TestComponentWrapper_ExportedInstance(t *testing.T) {
	// Create a component instance with an exported instance
	nestedInst := &Instance{
		exports: map[string]*ExportedFunc{
			"do-thing": {name: "do-thing"},
		},
	}

	mainInst := &Instance{
		exportedInstances: map[string]*Instance{
			"service": nestedInst,
		},
	}

	wrapper := &ComponentWrapper{instance: mainInst}

	// Should find the exported instance
	service := wrapper.ExportedInstance("service")
	if service == nil {
		t.Fatal("ExportedInstance should find 'service'")
	}

	// Should be able to get functions from nested instance
	fn := service.ExportedFunction("do-thing")
	if fn == nil {
		t.Error("should find 'do-thing' on nested instance")
	}
}
```

### Step 2: Run test to verify it fails

Run: `CGO_ENABLED=0 go test ./internal/component/... -run TestComponentWrapper_ExportedInstance -v`

Expected: FAIL (method returns nil)

### Step 3: Implement ExportedInstance

Find the existing `ExportedInstance` method in `linker_api.go` that returns `nil` and replace it:

```go
// ExportedInstance returns an exported instance by name.
func (w *ComponentWrapper) ExportedInstance(name string) api.ComponentInstance {
	if w.instance == nil {
		return nil
	}
	nested := w.instance.GetExportedInstance(name)
	if nested == nil {
		return nil
	}
	return &ComponentInstanceWrapper{instance: nested}
}
```

### Step 4: Run test to verify it passes

Run: `CGO_ENABLED=0 go test ./internal/component/... -run TestComponentWrapper_ExportedInstance -v`

Expected: PASS

### Step 5: Commit

```bash
git add internal/component/linker_api.go internal/component/linker_api_test.go
git commit -m "feat(component): implement ComponentWrapper.ExportedInstance

Exposes nested instances through the public API.
Returns ComponentInstanceWrapper for further access.

Refs: docs/plans/spec-instantiation-linking-conformance-fixes"
```

---

## Task 5: Add ComponentInstance Interface to API

**Files:**
- Modify: `api/component.go`

### Step 1: Check if ComponentInstance interface exists

Look for `ComponentInstance` in `api/component.go`. If it doesn't exist, add it.

### Step 2: Add or verify the interface

```go
// Add to api/component.go if not present

// ComponentInstance represents an exported component instance.
type ComponentInstance interface {
	// ExportedFunction returns an exported function by name, or nil if not found.
	ExportedFunction(name string) ComponentFunc

	// ExportedInstance returns a nested exported instance by name, or nil if not found.
	ExportedInstance(name string) ComponentInstance
}
```

### Step 3: Verify ComponentInstanceWrapper implements the interface

Add a compile-time check in `linker_api.go`:

```go
// Compile-time interface check
var _ api.ComponentInstance = (*ComponentInstanceWrapper)(nil)
```

### Step 4: Run tests to verify

Run: `CGO_ENABLED=0 go build ./...`

Expected: Build succeeds

### Step 5: Commit

```bash
git add api/component.go internal/component/linker_api.go
git commit -m "feat(api): add ComponentInstance interface

Defines ExportedFunction and ExportedInstance for nested access.
ComponentInstanceWrapper implements the interface.

Refs: docs/plans/spec-instantiation-linking-conformance-fixes"
```

---

## Task 6: Run Phase 4 Regression Tests

**Files:** None (verification only)

### Step 1: Run all export instance tests

Run: `CGO_ENABLED=0 go test ./internal/component/... -run "ExportedInstance|ComponentInstanceWrapper" -v`

Expected: All PASS

### Step 2: Run calculator regression tests

Run: `CGO_ENABLED=0 go test ./internal/component/wasip2test/... -run TestCalculatorPlugins/add -v`

Expected: PASS

Run: `CGO_ENABLED=0 go test ./internal/component/wasip2test/... -run TestCalculatorPlugins/subtract -v`

Expected: PASS

### Step 3: Update progress tracker

Edit `docs/plans/spec-instantiation-linking-conformance-fixes/00-root.md`:
- Mark Phase 4 status as `[x] Complete`
- Mark Phase 4 regression as `[x] Verified`

### Step 4: Commit

```bash
git add docs/plans/spec-instantiation-linking-conformance-fixes/00-root.md
git commit -m "docs: mark Phase 4 (Export Instance API) complete

All export instance tests pass.
Calculator add/subtract regression tests pass.

Refs: docs/plans/spec-instantiation-linking-conformance-fixes"
```

---

## Phase 4 Complete

**Summary of changes:**
- Added exported instances tracking to Instance
- Wired instance exports during instantiation
- Created ComponentInstanceWrapper for API access
- Implemented ComponentWrapper.ExportedInstance
- Added/verified ComponentInstance interface in API

**Next steps:**
- Proceed to Phase 5 (Advanced Import Names) if needed
- Phase 5 is independent and can be done at any time
