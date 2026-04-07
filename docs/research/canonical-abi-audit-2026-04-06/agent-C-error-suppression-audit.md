# Error Suppression Audit — Agent C

**Date:** 2026-04-06
**Branch:** `feat/wasip2-complete-implementation`
**Scope:** `internal/component/`, `imports/wasip2/`, `internal/engine/`, `internal/sys/`, `internal/sysfs/`, `experimental/`, top-level `wazero` package
**Methodology:** Read-only static analysis. No code was modified, no builds/tests were run.

## Executive Summary

This audit found **pervasive, systemic error suppression** in the Component Model implementation — especially in `imports/wasip2/` and in `internal/component/component_linker.go`'s memory-writing/lifting helpers. These suppressions fall into three broad categories, in descending order of severity:

1. **Silent default-on-bad-handle in wasip2 host functions.** Virtually every host function in `imports/wasip2/sockets/tcp.go`, `udp.go`, and `imports/wasip2/http/http.go` swallows `getTcpSocket`/`getUdpSocket`/`getFields`/`getIncomingRequest`/`getOutgoingRequest`/etc. errors and returns a placeholder success value (`ResultOk(nil)`, `ValBool(false)`, `ValU16(200)`, `ValEnum("ipv4")`, empty list, etc.). Per Component Model `CanonicalABI.md` `lift_borrow`/`lift_own` (lines 2215-2238), a handle that does not exist in the table **must trap**. These host functions currently make guest bugs look like legitimate `Ok` results. The comments justifying these fallbacks ("Fallback for tests without resource table") are false: the behavior is not limited to tests; real guest calls with bad handles also hit this path and silently succeed.

2. **Silent memory-read/memory-write failures in `component_linker.go`.** The free (non-method) `writeResultsToMemory`, `writeValToMemory`, `writeRecordToMemory`, `liftValFromMemory`, `liftRecordFromMemory`, and `liftOptionFromMemory` functions (all in `component_linker.go`) systematically discard the `bool` return of `api.Memory.Write*` and `api.Memory.Read*` calls. When the guest passes a pointer that's out of bounds, the lift functions return zero values and the lower functions return success — no trap, no error.

3. **Silent return from non-trap resource operations.** `ResourceTable.CreateResourceDropFunc` and `CreateResourceRepFunc` exist as "silently ignore invalid handles per spec" variants alongside trap-emitting `*WithTrap` variants. The silent variants are still used by `createCanonResourceExport` and `createResourceDropExport`/`createResourceRepExport` in `component_linker.go`. This matches a known family of canonical ABI bugs: the trap variants exist but are not wired in.

A bug previously masked for weeks was `createCanonLowerFunc` silently discarding errors from `writeResultsToMemory` and `compFunc.Impl` — that has now been fixed (commits `d71ffbf3` and `3f91ed37`). **The patterns around those two call sites still exist throughout the same file and neighbouring files. The fix for those two lines did not generalise.**

---

## Findings by Package

Legend:
- **critical**: causes silent data corruption, wrong results returned to the guest, or security holes
- **high**: causes silent test failures, hard-to-debug symptoms, or hides real bugs
- **medium**: causes lost diagnostics in normal failure paths
- **low**: defensive coding that could be tightened but is not actively harmful
- **informational**: deliberate/documented suppression worth the human's attention

---

## `internal/component/`

### `internal/component/component_linker.go:2390-2392`
**Function:** `createAliasExport.<closure>` (lazy-resolution wrapper)
**Pattern:** silent return on error
**Severity:** high
**Code:**
```go
results, err := sourceFunc.Call(ctx, stack[:actualParamCount]...)
if err != nil {
    return
}
```
**Impact:** When a component aliases a function from a nested core instance and the nested function returns an error (e.g., trap), the wrapper silently returns without modifying the stack. The guest reads whatever bytes happened to be on the stack — a classic lift-from-uninitialized symptom. Errors in source functions are lost; no trap is surfaced.
**Recommendation:** Propagate via panic (matching the recent fix in `createCanonLowerFunc`) or return the error to the interpreter.
**Already fixed?** No
**Confidence:** high

---

### `internal/component/component_linker.go:2367-2385`
**Function:** `createAliasExport.<closure>` (lazy-resolution wrapper) — multiple nil checks
**Pattern:** silent return on nil / unresolved forward reference
**Severity:** high
**Code:**
```go
func(ctx context.Context, mod api.Module, stack []uint64) {
    if int(sourceInstanceIdx) >= len(inst.coreInstances) {
        // Source instance not available - this shouldn't happen at call time
        return
    }
    sourceInst := inst.coreInstances[sourceInstanceIdx]
    if sourceInst == nil {
        return
    }
    sourceFunc := sourceInst.ExportedFunction(sourceExportName)
    if sourceFunc == nil {
        return
    }
    defs := sourceInst.ExportedFunctionDefinitions()
    def := defs[sourceExportName]
    if def == nil {
        return
    }
```
**Impact:** Five separate "shouldn't happen" silent-return branches in one closure. If any wiring inconsistency leaves the source instance/export unresolved at call time, the guest sees uninitialized stack. The comment "this shouldn't happen at call time" literally describes a class of bugs this code is masking.
**Recommendation:** Each branch should panic with a descriptive error.
**Already fixed?** No
**Confidence:** high

---

### `internal/component/component_linker.go:1803-1806, 2054-2058, 2061-2065`
**Function:** `createHostModule`, `createCanonLowerHostModule`
**Pattern:** silent `return nil` on error
**Severity:** high
**Code:**
```go
mod, err := hmi.InstantiateHostModule(ctx, moduleName, exports)
if err != nil {
    return nil
}
```
```go
mod, err := hmi.InstantiateHostModuleWithResources(ctx, moduleName, exports, tableExports, memoryExports)
if err != nil {
    return nil
}
```
**Impact:** Host module instantiation errors are silently converted to a `nil` return, which cascades into `nil` entries in the instance's module lookup maps. The caller cannot distinguish between "runtime does not support host modules" and "instantiation failed with a real error." Symptoms show up later as "function N not found" or import resolution failures pointing at the wrong cause.
**Recommendation:** Return `(api.Module, error)` or at least log/panic.
**Already fixed?** No
**Confidence:** high

---

### `internal/component/component_linker.go:1818-1821`
**Function:** `createResourceDropExport.<closure>`
**Pattern:** silent return on error with "per spec" comment that doesn't match spec
**Severity:** critical
**Code:**
```go
Func: api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
    handle := uint32(stack[0])
    entry, err := inst.resourceTable.Remove(Handle(handle))
    if err != nil {
        return // Silently ignore invalid handles per spec
    }
```
**Impact:** Per Component Model `CanonicalABI.md` lines 2215-2220 (`lift_own`), an invalid resource handle on `resource.drop` **must trap**. The comment claims "per spec" but the spec requires the opposite behavior. Guest bugs (double-drop, use-after-drop) are invisible.
**Recommendation:** Panic/trap via the `CreateResourceDropFuncWithTrap` pattern already implemented in `resource_table.go:453`.
**Already fixed?** No
**Confidence:** high

---

### `internal/component/component_linker.go:1854-1862`
**Function:** `createResourceRepExport.<closure>`
**Pattern:** default fallback value on error (returns handle 0 to guest)
**Severity:** critical
**Code:**
```go
Func: api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
    handle := uint32(stack[0])
    rep, err := inst.resourceTable.Rep(Handle(handle))
    if err != nil {
        stack[0] = 0
        return
    }
    stack[0] = uint64(rep)
}),
```
**Impact:** Invalid handle on `resource.rep` returns 0 to the guest instead of trapping. Per spec `trap_if(not isinstance(h, ResourceHandle))`. Guest code that uses the result as a pointer will silently corrupt memory. A trap-emitting variant exists (`CreateResourceRepFuncWithTrap`) but is not used here.
**Recommendation:** Use the trap-emitting variant.
**Already fixed?** No
**Confidence:** high

---

### `internal/component/component_linker.go:2161-2167`
**Function:** `createResourceOpExport` (CanonKindResourceDrop branch)
**Pattern:** discarded error return
**Severity:** critical
**Code:**
```go
Func: api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
    handle := uint32(stack[0])
    if inst.resourceTable != nil {
        inst.resourceTable.Remove(Handle(handle))
    }
}),
```
**Impact:** `Remove` returns `(*ResourceTableEntry, error)` — both are dropped. Invalid handle on `canon resource.drop` silently succeeds. Same issue as `createResourceDropExport` but reached via a different code path (canonical op export vs resource-def op export). Same spec violation.
**Recommendation:** Check the error and trap.
**Already fixed?** No
**Confidence:** high

---

### `internal/component/component_linker.go:2189-2196`
**Function:** `createResourceOpExport` (CanonKindResourceRep branch)
**Pattern:** silent fall-through on error; stack[0] left as input handle
**Severity:** critical
**Code:**
```go
Func: api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
    handle := uint32(stack[0])
    if inst.resourceTable != nil {
        rep, err := inst.resourceTable.Rep(Handle(handle))
        if err == nil {
            stack[0] = uint64(rep)
        }
    }
}),
```
**Impact:** On `Rep` error, `stack[0]` is not overwritten — the guest receives **the input handle value** as if it were the rep. That's worse than returning 0: it's a silent identity mapping that will look correct in simple tests but produce garbage on real workloads. The one-line fix pattern `panic(err)` from `d71ffbf3` is exactly what's needed here.
**Recommendation:** Trap on error.
**Already fixed?** No
**Confidence:** high

---

### `internal/component/component_linker.go:1564, 1571`
**Function:** `wireMemorySharing`
**Pattern:** silent `return nil` (success) on impossible-to-proceed conditions
**Severity:** medium
**Code:**
```go
if primaryMemoryModule == nil {
    return nil
}
...
primaryMemoryInst, ok := primaryMemoryModule.(*wasm.ModuleInstance)
if !ok || primaryMemoryInst.MemoryInstance == nil {
    return nil
}
```
**Impact:** Memory-sharing setup silently skips when the primary memory module cannot be resolved. Modules that expected to import memory now have no memory, and subsequent accesses fail with confusing errors ("nil memory").
**Recommendation:** Return a descriptive error or log.
**Already fixed?** No
**Confidence:** medium

---

### `internal/component/component_linker.go:1440-1453, 1474-1487`
**Function:** `resolveCoreFunc`, `resolveCoreMemory`
**Pattern:** "Fallback" to first core instance on resolve error
**Severity:** high
**Code:**
```go
func (l *ComponentLinker) resolveCoreFunc(inst *Instance, c *Component, funcIdx uint32, funcSpace *CoreFuncIndexSpace) (api.Function, error) {
    instanceIdx, exportName, err := funcSpace.Resolve(funcIdx)
    if err != nil {
        // Fallback: first core instance
        if len(inst.coreInstances) == 0 {
            return nil, fmt.Errorf("no core instances available")
        }
        coreInst := inst.coreInstances[0]
        ...
        for name := range coreInst.ExportedFunctionDefinitions() {
            return coreInst.ExportedFunction(name), nil
        }
        ...
    }
```
**Impact:** When the index space lookup fails, the function silently returns `coreInstances[0]`'s first exported function — a **completely unrelated function**. This is extremely dangerous: a canonical lift/lower operation will execute against a function with a different signature. Test code with a single-module component happens to pass because "first core instance" equals "the module you wanted."
**Recommendation:** Remove the fallback entirely; return the original error.
**Already fixed?** No
**Confidence:** high

---

### `internal/component/component_linker.go:3290-3292`
**Function:** `writeResultsToMemory` (variant branch)
**Pattern:** hardcoded placeholder discriminant, known-broken
**Severity:** critical
**Code:**
```go
case ValKindVariant:
    caseName, payload := result.Variant()
    // For now, write discriminant as first i32 and payload recursively
    // Note: proper implementation needs type info for discriminant index
    _ = caseName // Would need type info to map name to index
    memory.WriteUint32Le(offset, 0) // Placeholder discriminant
```
**Impact:** **All variant-typed return values have their discriminant silently hardcoded to 0**, regardless of the actual case. This is the textbook "guest sees wrong result" bug. Any function returning a variant through `createCanonLowerFunc` will have its case ignored. This explains a whole class of spurious-zero reports.
**Recommendation:** Requires plumbing the resolved variant type into `writeResultsToMemory`; a short-term fix is to panic when the variant type is unavailable.
**Already fixed?** No
**Confidence:** high

---

### `internal/component/component_linker.go:3370-3383`
**Function:** `writeRecordToMemory`
**Pattern:** missing field silently written as zero bytes
**Severity:** critical
**Code:**
```go
func writeRecordToMemory(ctx context.Context, memory api.Memory, reallocFunc api.Function, offset uint32, rec map[string]Val, recordDef *RecordTypeDef, localTypes map[uint32]*TypeDef) (uint32, error) {
    for _, field := range recordDef.Fields {
        val, ok := rec[field.Name]
        if !ok {
            // Field missing - write zeros based on type size
            offset += fieldSizeForType(field.ValType, localTypes)
            continue
        }
```
**Impact:** If the Go host returns a record Val missing one or more declared fields, `writeRecordToMemory` silently writes zero bytes for them and skips ahead. The guest sees a zero-filled field where the host forgot to populate it. Contrast with `abi/lower.go:373-376` which correctly errors on missing fields.
**Recommendation:** Return `fmt.Errorf("missing field %q", field.Name)`.
**Already fixed?** No
**Confidence:** high

---

### `internal/component/component_linker.go:3170-3520`
**Function:** `writeResultsToMemory`, `writeValToMemory`
**Pattern:** systematically discarded `bool` from `api.Memory.Write*` calls
**Severity:** critical
**Code:**
```go
// Many dozens of calls like:
memory.WriteUint32Le(offset, result.U32())
memory.WriteByteAt(offset, 1)
memory.Write(ptr, []byte(str))
// ... none of them check the bool return value
```
**Impact:** The signature is `WriteUint32Le(offset, v uint32) bool` where `false` means "out of range." Every one of these writes is silently lost when the guest's retptr is invalid. The lift chain (`writeResultsToMemory` → `writeValToMemory` → `writeRecordToMemory`) contains ~40+ unchecked `memory.Write*` calls. Contrast with `instance.go:2019-2275` where the method `lowerToMemory` **does** check every write and returns an error. The free functions in `component_linker.go` were written independently and dropped the check.
**Recommendation:** Audit every `memory.Write*` call in `component_linker.go` and check the return, returning an error.
**Already fixed?** No
**Confidence:** high

---

### `internal/component/component_linker.go:2726-2793`
**Function:** `liftValFromMemory` (and downstream `liftRecordFromMemory`, `liftOptionFromMemory`)
**Pattern:** systematically discarded `ok` from `api.Memory.Read*` calls
**Severity:** critical
**Code:**
```go
case 0x7f: // bool
    b, _ := memory.ReadByteAt(offset)
    return ValBool(b != 0)
case 0x7e: // s8
    b, _ := memory.ReadByteAt(offset)
    return ValS8(int8(b))
...
// liftRecordFromMemory and liftOptionFromMemory transitively call this
// with no error return
```
**Impact:** When the guest passes a pointer to parameters that is out of bounds, every read silently returns a zero byte and the lift produces a zero-valued Val. A malicious or buggy guest is indistinguishable from a legitimate guest passing zero-valued args. No error path at all — the function signature returns `Val`, not `(Val, error)`.
**Recommendation:** Change the function signature to return an error, or panic on read failure.
**Already fixed?** No
**Confidence:** high

---

### `internal/component/instance.go:1242-1401`
**Function:** `ExportedFunc.liftFieldFromMemory`
**Pattern:** default fallback values (0, "", empty) on read failure
**Severity:** critical
**Code:** Over 20 separate branches like:
```go
case types.Bool:
    b, ok := f.memory.ReadByteAt(offset)
    if !ok {
        return ValBool(false), 1
    }
    return ValBool(b != 0), 1
case types.S8:
    b, ok := f.memory.ReadByteAt(offset)
    if !ok {
        return ValS8(0), 1
    }
    return ValS8(int8(b)), 1
// ...and so on for all primitive types, Enum, Flags, Option, List, Record
```
**Impact:** Same class as the `liftValFromMemory` issue above. This one is subtler because the code **does** check `ok` — but then returns a zero value. The function signature `(Val, uint32)` has no error channel. When a misbehaving guest writes a bad retptr for a record containing an s64 field, the lifted record has `ValS64(0)` for that field and nothing flags the failure.
**Recommendation:** Change signature to `(Val, uint32, error)` and propagate.
**Already fixed?** No
**Confidence:** high

---

### `internal/component/instance.go:1146, 1182`
**Function:** `ExportedFunc.liftResultsWithTypeRetptr` (variant and list branches)
**Pattern:** discarded error-channel-less `liftFieldFromMemory`
**Severity:** critical
**Code:**
```go
// Variant retptr case
if c.Type != nil {
    payloadOffset := t.PayloadOffset()
    v, _ := f.liftFieldFromMemory(retptr+payloadOffset, c.Type)
    payload = &v
}
...
// List retptr case
for i := uint32(0); i < length; i++ {
    elemOffset := ptr + i*elemSize
    val, _ := f.liftFieldFromMemory(elemOffset, t.Element)
    elems[i] = val
}
```
**Impact:** Even when `liftFieldFromMemory` could indicate failure (it already returns `(Val, uint32)`, no error), callers discard the size `_` and the caller has no way to know if the underlying memory read succeeded. Combined with the default-zero behavior of `liftFieldFromMemory`, variant payload and list element reads silently corrupt.
**Recommendation:** Ties into the `liftFieldFromMemory` fix above.
**Already fixed?** No
**Confidence:** high

---

### `internal/component/resource_table.go:320-352`
**Function:** `CreateResourceDropFunc`, `CreateResourceRepFunc`
**Pattern:** "Silently ignore invalid handles per spec" — same false-spec comment
**Severity:** high (library footgun)
**Code:**
```go
func (t *ResourceTable) CreateResourceDropFunc(resourceTypeIdx uint32, destructor func(rep uint32)) func(handle uint32) {
    return func(handle uint32) {
        entry, err := t.Remove(Handle(handle))
        if err != nil {
            return // Silently ignore invalid handles per spec
        }
        ...
    }
}

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
**Impact:** These are the library-provided helpers. They exist with a false "per spec" comment. The spec-compliant variants (`*WithTrap`, `*WithContext`) exist in the same file and are correctly implemented — but this pair lingers and is called from `kv_store_test.go` and is still present for any external consumer. Keeping the silent variants invites future wiring bugs.
**Recommendation:** Either delete these and migrate all callers to the trap variants, or document loudly that they are spec-violating legacy helpers.
**Already fixed?** No
**Confidence:** high

---

### `internal/component/call_context.go:83-85`
**Function:** `CallContext.ExitCall`
**Pattern:** discarded error return
**Severity:** low
**Code:**
```go
for _, h := range c.lenders {
    // Ignore errors from already-removed handles (can happen if
    // the source handle was transferred during the call)
    _ = table.DecrementLends(h)
}
```
**Impact:** Documented suppression. Legitimate use case per comment. Informational — but worth a note that if `DecrementLends` could ever fail for a reason other than "already removed", that reason would be lost.
**Recommendation:** Either distinguish error types (`ErrHandleAlreadyRemoved` vs others) or leave as-is with a tighter comment.
**Already fixed?** No
**Confidence:** high (finding is accurate; severity is low)

---

### `internal/component/instance.go:155`
**Function:** `ExportedFunc.Call` (deferred cleanup)
**Pattern:** comment-only acknowledgement
**Severity:** low
**Code:**
```go
defer func() {
    if subtask != nil && subtask.State() == SubtaskStatePending {
        subtask.DeliverResolve(nil)
        subtask.StartFinish()
        subtask.Finish() // Ignore errors in defer cleanup
    }
}()
```
**Impact:** Subtask cleanup errors in the defer path are lost. If `Finish` could reveal a tracking inconsistency, it would be invisible.
**Recommendation:** Consider logging via a debug hook.
**Already fixed?** No
**Confidence:** medium

---

### `internal/component/nested_component.go:259-265`
**Function:** `populateChildTypeSpace`
**Pattern:** comment-only skip on error + "shouldn't happen" comment
**Severity:** medium
**Code:**
```go
case AliasKindOuter:
    resolved, err := ResolveOuterAlias(inst, alias)
    if err == nil {
        if td, ok := resolved.(*TypeDef); ok {
            inst.typeSpace[alias.Idx] = td
        }
    }
```
**Impact:** Outer-alias resolution errors are discarded. When `ResolveOuterAlias` fails, the type slot is left as nil and subsequent type lookups will produce confusing "type not found" errors far from the real cause.
**Recommendation:** At least log or surface the error up the stack.
**Already fixed?** No
**Confidence:** high

---

### `internal/component/component_linker.go:470-500`
**Function:** `wireComponentFunctions` (alias-on-instance resolution)
**Pattern:** silent `continue` on 4 consecutive type-assertion failures
**Severity:** medium
**Code:**
```go
importName, ok := instanceToImport[alias.InstanceIdx]
if !ok {
    // Instance might be a ComponentInstance definition, not an import.
    continue
}
def, ok := resolvedImports[importName]
if !ok {
    continue
}
instDef, ok := def.(*InstanceDef)
if !ok {
    continue
}
exportDef, ok := instDef.Exports[alias.ExportName]
if !ok {
    continue
}
funcDef, ok := exportDef.(*FuncDef)
if !ok {
    continue
}
```
**Impact:** If a host provides an instance with an export of unexpected type (or a missing export), the alias wiring silently skips. The resulting component has fewer entries in `componentFuncs` than expected, and later lookups produce misleading errors ("func N not found").
**Recommendation:** Distinguish "instance is a ComponentInstance definition" (expected) from the other failure modes (bug/misconfiguration) and return or log for the latter.
**Already fixed?** No
**Confidence:** medium

---

### `internal/component/component_linker.go:1677-1679, 1697-1700, 1733-1736`
**Function:** `matchComponentImport` (semver resolution)
**Pattern:** silent return/continue on `ParseSemver` error
**Severity:** low
**Code:**
```go
reqVersion, err := ParseSemver(reqVersionStr)
if err != nil {
    return nil
}
...
availVersion, err := ParseSemver(availVersionStr)
if err != nil {
    continue
}
```
**Impact:** Unparseable version strings silently fall off the matching list. If the import was intended to match but the version string is malformed, the consumer gets "import not found" with no indication that a candidate existed.
**Recommendation:** Log at debug level.
**Already fixed?** No
**Confidence:** medium

---

### `internal/component/linker.go:240-252, 340-355, 400-417, 555-572`
**Function:** `Linker.matchImportWithStrategy` (multiple semver-resolution helpers)
**Pattern:** silent `continue` on `ParseSemver` error
**Severity:** low
**Code:** (similar pattern across four sites)
**Impact:** Same as above. Low severity.
**Already fixed?** No
**Confidence:** medium

---

### `internal/component/canon_lower.go:385`
**Function:** `LoweredFunc.lowerResultsTyped`
**Pattern:** cosmetic assignment-discard (NOT a bug, reviewed for completeness)
**Severity:** informational
**Code:**
```go
flat, err := f.lowerValToFlatTyped(results[i], f.funcType.Results[i].ValType)
if err != nil {
    return nil, fmt.Errorf("lower result %d: %w", i, err)
}
_ = result // use indexed access for consistency
```
**Impact:** None — this is a deliberate `_ = result` to acknowledge the loop variable while preferring indexed access. Not a suppression.
**Recommendation:** No action needed.
**Already fixed?** N/A
**Confidence:** high

---

### `internal/component/component_linker.go:3291`
**Function:** `writeResultsToMemory` — already listed above but flagging the specific discard
**Pattern:** `_ = caseName` with acknowledging comment
**Severity:** critical (see above finding)
**Code:**
```go
_ = caseName // Would need type info to map name to index
```
**Already documented.**

---

### `internal/component/component_linker.go:2318`
**Function:** `createAliasExport` (type-resolution helper)
**Pattern:** discarded third return (NOT an error — reviewed for completeness)
**Severity:** informational
**Code:**
```go
paramTypes, resultTypes, _ = coreSignature(compParamTypes, compResultTypes)
```
**Impact:** `coreSignature` returns `(params, results []api.ValueType, needsRetptr bool)`. The discarded value is a bool, not an error. Not a suppression.
**Recommendation:** No action needed.
**Confidence:** high

---

## `imports/wasip2/`

### `imports/wasip2/sockets/tcp.go` — systemic pattern
**Functions:** `tcpSocketStartBind`, `tcpSocketFinishBind`, `tcpSocketStartConnect`, `tcpSocketFinishConnect`, `tcpSocketStartListen`, `tcpSocketFinishListen`, `tcpSocketAccept`, `tcpSocketLocalAddress`, `tcpSocketRemoteAddress`, `tcpSocketIsListening`, `tcpSocketAddressFamily`, `tcpSocketSetListenBacklogSize`, `tcpSocketKeepAliveEnabled`, `tcpSocketSetKeepAliveEnabled`, `tcpSocketKeepAliveIdleTime`, `tcpSocketSetKeepAliveIdleTime`, `tcpSocketKeepAliveInterval`, `tcpSocketSetKeepAliveInterval`, `tcpSocketKeepAliveCount`, `tcpSocketSetKeepAliveCount`, `tcpSocketHopLimit`, `tcpSocketSetHopLimit`, `tcpSocketReceiveBufferSize`, `tcpSocketSetReceiveBufferSize`, `tcpSocketSendBufferSize`, `tcpSocketSetSendBufferSize`, `tcpSocketSubscribe`, `tcpSocketShutdown`
**Pattern:** "Fallback for tests without resource table" — silent success on any `getTcpSocket` error
**Severity:** critical
**Sample locations:** lines 101-105, 130-134, 169-173, 198-205, 275-279, 296-300, 323-331, 383-388, 403-408, 423-426, 436-439, 450-453, 464-468, 480-483, 497-501, 514-517, 529-533, 546-549, 561-565, 577-580, 591-595, 607-610, 621-625, 637-640, 654-657, 670-673, 703-706

**Code:**
```go
sock, err := getTcpSocket(ctx, handle)
if err != nil {
    // Fallback for tests without resource table
    return []component.Val{component.ValResultOk(nil)}, nil
}
```
or
```go
sock, err := getTcpSocket(ctx, handle)
if err != nil {
    return []component.Val{component.ValBool(false)}, nil
}
```
or
```go
sock, err := getTcpSocket(ctx, handle)
if err != nil {
    duration := component.ValU64(7200000000000) // 2 hours in nanoseconds
    return []component.Val{component.ValResultOk(&duration)}, nil
}
```

**Impact:** Per Component Model `CanonicalABI.md` `lift_borrow`, an invalid borrow handle must trap. These functions all swallow the error and return a placeholder success. Consequences:
- Guest bugs (use-after-drop on a tcp-socket handle) look like legitimate operations.
- Getter functions return lies: a bad handle returns a hardcoded default (2 hours for keep-alive idle, 9 for keep-alive count, 64 for hop limit, 65536 for buffer sizes). A guest written to probe socket state cannot distinguish "never set" from "bad handle."
- Setter functions silently no-op on bad handles, and the guest thinks the configuration was applied.
- Listen/bind/connect functions silently transition to "success", which makes the guest think the socket is operational when it isn't.
- The comment "Fallback for tests without resource table" is technically false: these branches trigger on **any** getTcpSocket failure, not just "no resource table." In a real component invocation with a real resource table, a bad handle still follows this path.

**Recommendation:** Return a non-nil Go error from every getter/setter on `getTcpSocket` failure. `createCanonLowerFunc` will then trap (per the recent `d71ffbf3` fix).
**Already fixed?** No
**Confidence:** high

---

### `imports/wasip2/sockets/udp.go` — systemic pattern (mirror of tcp.go)
**Functions:** `udpSocketStartBind`, `udpSocketFinishBind`, `udpSocketStream`, `udpSocketLocalAddress`, `udpSocketRemoteAddress`, `udpSocketAddressFamily`, `udpSocketUnicastHopLimit`, `udpSocketSetUnicastHopLimit`, `udpSocketReceiveBufferSize`, `udpSocketSetReceiveBufferSize`, `udpSocketSendBufferSize`, `udpSocketSetSendBufferSize`, `udpSocketSubscribe`, `udpSocketShutdown`, `incomingDatagramStreamReceive`, `outgoingDatagramStreamCheckSend`, `outgoingDatagramStreamSend`
**Pattern:** same "Fallback for tests without resource table" + "Fallback - return placeholder" pattern
**Severity:** critical
**Sample locations:** lines 102-104, 131-133, 169-173, 234-238, 254-258, 274-275, 287-289, 400, 507

**Impact:** Identical to the tcp.go pattern. Same spec violation. Same silent default values.
**Recommendation:** Same as tcp.go.
**Already fixed?** No
**Confidence:** high

---

### `imports/wasip2/sockets/udp.go:194-197`
**Function:** `udpSocketStream` (connect-on-stream path)
**Pattern:** comment-only "Ignore close error"
**Severity:** low
**Code:**
```go
if sock.conn != nil {
    netErr := sock.conn.Close()
    if netErr != nil {
        // Ignore close error
    }
    ...
}
```
**Impact:** UDP is connectionless; close error before a re-dial is low-consequence. But explicit.
**Recommendation:** Either log or remove the empty branch.
**Already fixed?** No
**Confidence:** medium

---

### `imports/wasip2/sockets/types.go:846-858`
**Function:** `TcpSocket.Close`
**Pattern:** error overwrite hiding the first close failure
**Severity:** medium
**Code:**
```go
func (s *TcpSocket) Close() error {
    var err error
    if s.listener != nil {
        err = s.listener.Close()
        s.listener = nil
    }
    if s.conn != nil {
        err = s.conn.Close()  // overwrites listener err
        s.conn = nil
    }
    s.state = tcpStateClosed
    return err
}
```
**Impact:** If both listener and conn exist and listener.Close fails, the listener error is silently discarded when conn.Close is called. Not a common case but a correctness bug.
**Recommendation:** Use `errors.Join`.
**Already fixed?** No
**Confidence:** high

---

### `imports/wasip2/sockets/types.go:878-882`
**Function:** `UdpSocket.Destroy`
**Pattern:** discarded error return
**Severity:** low
**Code:**
```go
func (s *UdpSocket) Destroy() {
    s.Close()  // error discarded
    s.localAddr = nil
    s.remoteAddr = nil
}
```
**Impact:** Destructor pattern — Go destructor cannot return errors. Low severity.
**Already fixed?** No
**Confidence:** high

---

### `imports/wasip2/sockets/tcp.go:218`
**Function:** `tcpSocketFinishConnect` (listener-to-connection transition)
**Pattern:** discarded `Close()` error
**Severity:** low
**Code:**
```go
if sock.listener != nil {
    sock.listener.Close()
    sock.listener = nil
}
```
**Impact:** Listener close error on transition to connection. Read-side cleanup, low severity.
**Already fixed?** No
**Confidence:** high

---

### `imports/wasip2/http/http.go` — systemic pattern
**Functions:** `fieldsGet`, `fieldsHas`, `fieldsSet`, `fieldsDelete`, `fieldsAppend`, `fieldsEntries`, `fieldsClone`, `outgoingRequestMethod`, `outgoingRequestSetMethod`, `outgoingRequestPathWithQuery`, `outgoingRequestSetPathWithQuery`, `outgoingRequestScheme`, `outgoingRequestSetScheme`, `outgoingRequestAuthority`, `outgoingRequestSetAuthority`, `outgoingRequestHeaders`, `outgoingRequestBody`, `incomingRequestMethod`, `incomingRequestPathWithQuery`, `incomingRequestScheme`, `incomingRequestAuthority`, `incomingRequestHeaders`, `incomingRequestConsume`, `outgoingResponseStatusCode`, `outgoingResponseSetStatusCode`, `outgoingResponseHeaders`, `outgoingResponseBody`, `incomingResponseStatus`, `incomingResponseHeaders`, `incomingResponseConsume`, `incomingBodyStream`, `outgoingBodyWrite`, `futureIncomingResponseGet`, `futureIncomingResponseSubscribe`, `futureTrailersGet`, `futureTrailersSubscribe`
**Pattern:** same "silent success on getXxx failure" — swallows `getFields`/`getIncomingRequest`/`getOutgoingRequest`/`getIncomingResponse`/`getOutgoingResponse`/`getIncomingBody`/`getOutgoingBody`/`getRequestOptions`/`getFutureIncomingResponse` errors
**Severity:** critical
**Sample locations:** lines 363-368, 391-394, 401-405, 427-430, 440-443, 459-462, 485-491, 530-533, 540-543, 554-557, 570-573, 588-591, 606-609, 623-627, 640-643, 659-661, 676-681, 758-762, 769-772, 785-788, 803-806, 819-823, 837-842, 903-906, 913-916, 925-928, 940-945, 964-968, 976-979, 992-997, 1018-1022, 1065-1069, 1118-1121, 1149-1152

**Code samples:**
```go
// Setter: silent Ok
func outgoingRequestSetMethod(ctx context.Context, args []component.Val) ([]component.Val, error) {
    req, err := getOutgoingRequest(ctx, args[0].Borrow())
    if err != nil {
        return []component.Val{component.ValResultOk(nil)}, nil
    }
    ...
}

// Getter: return hardcoded default
func incomingResponseStatus(ctx context.Context, args []component.Val) ([]component.Val, error) {
    resp, err := getIncomingResponse(ctx, args[0].Borrow())
    if err != nil {
        return []component.Val{component.ValU16(200)}, nil  // fake "200 OK"
    }
    ...
}

// Lookup: return false/empty
func fieldsHas(ctx context.Context, args []component.Val) ([]component.Val, error) {
    ...
    fields, err := getFields(ctx, handle)
    if err != nil {
        return []component.Val{component.ValBool(false)}, nil
    }
    ...
}

// Ownership transfer: return bogus handle 0
func outgoingRequestHeaders(ctx context.Context, args []component.Val) ([]component.Val, error) {
    ...
    if err != nil || table == nil {
        return []component.Val{component.ValOwn(0)}, nil
    }
    ...
}

// Variant getter: return "get" as default method
func incomingRequestMethod(ctx context.Context, args []component.Val) ([]component.Val, error) {
    req, err := getIncomingRequest(ctx, args[0].Borrow())
    if err != nil {
        // Return GET as fallback for invalid handle
        return []component.Val{component.ValVariant("get", nil)}, nil
    }
    ...
}
```

**Impact:** Identical to the sockets case, plus some unique twists:
- `incomingResponseStatus` returns a fake 200 OK for a bad handle, meaning a guest checking "was the response successful?" gets a false-positive on its own bug.
- `incomingRequestMethod` returns "get" for bad handles, so a guest misrouting requests may find itself handling a GET that was actually a POST.
- Ownership-transfer functions (`headers()`, `body()`) silently return handle 0. The guest now has a zero handle that will itself fail on any subsequent operation, but with a "bad handle 0" error that doesn't point back to the original cause.
- The `ValOwn(0)` return from headers/body is **particularly bad** because it looks like a valid handle from the guest's perspective until the guest tries to use it. The resulting "bad handle 0" error is cosmetically similar to "bad-handle-N" errors from unrelated bugs.

**Recommendation:** Every `get*` helper error should be returned as a non-nil Go error; `createCanonLowerFunc` then traps.
**Already fixed?** No
**Confidence:** high

---

### `imports/wasip2/http/http.go:1094-1098`
**Function:** `outgoingBodyFinish`
**Pattern:** silent skip of `body.Finish()` on invalid handle
**Severity:** high
**Code:**
```go
bodyHandle := component.Handle(args[0].Own())
bodyEntry, err := table.Remove(bodyHandle)
if err == nil {
    if body, ok := bodyEntry.Rep.(*OutgoingBody); ok {
        body.Finish()
    }
}

// Consume optional trailers
trailersOpt := args[1].Option()
if trailersOpt != nil {
    trailersHandle := component.Handle(trailersOpt.Own())
    table.Remove(trailersHandle)  // return value discarded
}

return []component.Val{component.ValResultOk(nil)}, nil
```
**Impact:** Specific to the **output body finish** path. If the handle is bad, `body.Finish()` is never called — buffered writes may never flush. Meanwhile the guest is told the operation succeeded. This is **data loss on the write path**. Also, `table.Remove(trailersHandle)` drops its return so a bad trailers handle is silently ignored.
**Recommendation:** Trap on bad body handle; check trailers removal.
**Already fixed?** No
**Confidence:** high

---

### `imports/wasip2/http/http.go:1045-1050`
**Function:** `incomingBodyFinish`
**Pattern:** silent skip of `body.Close()` on invalid handle
**Severity:** medium
**Code:**
```go
bodyHandle := component.Handle(args[0].Own())
bodyEntry, err := table.Remove(bodyHandle)
if err == nil {
    if body, ok := bodyEntry.Rep.(*IncomingBody); ok {
        body.Close()
    }
}
```
**Impact:** On the read path, so less severe than `outgoingBodyFinish`. But if the handle is bad, `body.Close()` is skipped and the underlying reader may leak.
**Recommendation:** Trap on bad handle.
**Already fixed?** No
**Confidence:** high

---

### `imports/wasip2/http/http.go:885-890`
**Function:** `outgoingResponseConstructor`
**Pattern:** silent ignore of `table.Remove` error for headers handle
**Severity:** medium
**Code:**
```go
headersHandle := component.Handle(args[0].Own())
var headers *Fields
headersEntry, err := table.Remove(headersHandle)
if err == nil {
    if h, ok := headersEntry.Rep.(*Fields); ok {
        headers = h
    }
}
if headers == nil {
    headers = NewFields()
}
```
**Impact:** A bad headers handle silently results in the response being constructed with **fresh empty headers** instead of the guest's supplied headers. The guest thinks it built a response with its prepared headers; it didn't.
**Recommendation:** Trap on bad handle.
**Already fixed?** No
**Confidence:** high

---

### `imports/wasip2/http/incoming.go:71`
**Function:** `NewHTTPHandler.<closure>` (cleanup path)
**Pattern:** explicit `_, _ = table.Remove(...)`
**Severity:** low
**Code:**
```go
defer func() {
    _, _ = table.Remove(requestHandle)
    if entry, err := table.Remove(outparamHandle); err == nil {
        if op, ok := entry.Rep.(*ResponseOutparam); ok {
            op.Destroy()
        }
    }
}()
```
**Impact:** Cleanup of the request handle during defer, after the call has returned. Low severity because the handle was created in this same closure and is being torn down. However, the second `table.Remove(outparamHandle)` error (via `if err == nil`) does skip Destroy, which could leak the outparam if the handle was invalid.
**Already fixed?** No
**Confidence:** high

---

### `imports/wasip2/http/incoming.go:115`
**Function:** `NewHTTPHandler.<closure>` (response body write to HTTP ResponseWriter)
**Pattern:** discarded `Write` return
**Severity:** medium
**Code:**
```go
if body := resp.BodyBytes(); body != nil {
    w.Write(body)
}
```
**Impact:** `http.ResponseWriter.Write` returns `(int, error)`. The error is discarded. If the client disconnected, the error is lost; if only a partial write succeeds, no retry is attempted. Real-world HTTP servers commonly log these.
**Recommendation:** Log the error.
**Already fixed?** No
**Confidence:** high

---

### `imports/wasip2/http/types.go:624`
**Function:** `IncomingBody.Close`
**Pattern:** discarded `Close` error
**Severity:** low
**Code:**
```go
func (b *IncomingBody) Close() {
    if b.reader != nil {
        b.reader.Close()
    }
}
```
**Impact:** Read-side cleanup; low severity.
**Already fixed?** No
**Confidence:** high

---

### `imports/wasip2/io/streams.go:123, 254`
**Function:** `InputStream.Close`, `OutputStream.Close`
**Pattern:** discarded `Close()` error
**Severity:** medium for OutputStream, low for InputStream
**Code:**
```go
// Input (line 123)
func (s *InputStream) Close() {
    s.closed = true
    if closer, ok := s.reader.(goio.Closer); ok {
        closer.Close()
    }
}

// Output (line 254)
func (s *OutputStream) Close() {
    s.closed = true
    if closer, ok := s.writer.(goio.Closer); ok {
        closer.Close()
    }
}
```
**Impact:** For OutputStream, a close error can indicate unflushed buffered data was lost. The WASI `output-stream.blocking-flush`/`drop` protocol should catch this at the guest level, but the underlying OS error is still lost.
**Recommendation:** Change `Close` to return `error`.
**Already fixed?** No
**Confidence:** high

---

### `imports/wasip2/io/streams.go:267-270`
**Function:** `OutputStream.Destroy`
**Pattern:** explicit "ignore errors during cleanup" with discarded `Flush` error
**Severity:** medium
**Code:**
```go
func (s *OutputStream) Destroy() {
    // Flush first if possible (ignore errors during cleanup)
    if f, ok := s.writer.(interface{ Flush() error }); ok && !s.closed {
        f.Flush()
    }
    s.Close()
}
```
**Impact:** Flush error on a buffered writer at destroy time means buffered data was silently lost. This is data loss in the write path. The comment ("ignore errors during cleanup") implies this is intentional, but the consequence is real data loss.
**Recommendation:** At least log; ideally propagate.
**Already fixed?** No
**Confidence:** high

---

### `imports/wasip2/filesystem/filesystem.go:230, 280`
**Function:** `descriptorReadViaStream`, `descriptorWriteViaStream`
**Pattern:** discarded `file.Close()` on seek failure
**Severity:** low for read, medium for write
**Code:**
```go
// read (230)
if offset > 0 {
    _, err = file.Seek(int64(offset), goio.SeekStart)
    if err != nil {
        file.Close()
        return errorResult(MapOSError(err)), nil
    }
}

// write (280) - identical pattern
```
**Impact:** On write path, a seek failure followed by an ignored close error means any prior buffered state may be lost without the guest knowing.
**Recommendation:** Check close; log.
**Already fixed?** No
**Confidence:** high

---

### `imports/wasip2/filesystem/filesystem.go:974, 993`
**Function:** `descriptorOpenAt` (error-cleanup paths)
**Pattern:** discarded `file.Close()` on stat error or missing resource table
**Severity:** medium
**Code:**
```go
fileInfo, statErr := file.Stat()
if statErr != nil {
    file.Close()
    return errorResult(MapOSError(statErr)), nil
}
...
table := component.ResourceTableFromContext(ctx)
if table == nil {
    file.Close()
    return errorResult(ErrorCodeIO), nil
}
```
**Impact:** File was opened in write/create mode (`osFlags` may include `O_CREATE|O_WRONLY`). On error cleanup, Close error is lost. If the file was freshly created and Close fails, data-consistency diagnostics are lost.
**Recommendation:** Log close errors; consider calling unlink on freshly created files to avoid partial-write pollution.
**Already fixed?** No
**Confidence:** medium

---

### `imports/wasip2/filesystem/preopens.go:60-70`
**Function:** `getPreopenedDirectories`
**Pattern:** silent `continue` on open/stat error
**Severity:** medium
**Code:**
```go
for guestPath, hostPath := range preopens {
    file, err := os.Open(hostPath)
    if err != nil {
        // Skip directories that can't be opened
        continue
    }
    info, err := file.Stat()
    if err != nil || !info.IsDir() {
        file.Close()
        continue
    }
    ...
}
```
**Impact:** Preopen entries that fail to open or aren't directories are silently dropped from the list returned to the guest. The guest sees a smaller preopen set than was configured, and no indication why. This is the literal symptom ("empty preopen list") the commit message for `3f91ed37` described as the user-visible failure mode.
**Recommendation:** Log and/or return an error if any preopen fails.
**Already fixed?** No
**Confidence:** high

---

### `imports/wasip2/filesystem/filesystem.go:610-614`
**Function:** `descriptorReadDirectory` (directory listing entry type resolution)
**Pattern:** silent fallback to `DescriptorTypeUnknown` on `entry.Info()` error
**Severity:** informational
**Code:**
```go
info, err := entry.Info()
var entryType DescriptorType
if err != nil {
    entryType = DescriptorTypeUnknown
} else {
    entryType = fileInfoToDescriptorType(info)
}
```
**Impact:** A stat failure on a directory entry is silently converted to "unknown." This is consistent with WASI semantics (unknown is a valid enum value), and some filesystems produce stat errors on symlinks that should be represented as unknown. Informational only.
**Already fixed?** No
**Confidence:** high

---

### `imports/wasip2/cli/cli.go:118-120`
**Function:** `initialCwd`
**Pattern:** silent fallback to None on `os.Getwd` error
**Severity:** informational
**Code:**
```go
cwd, err := os.Getwd()
if err != nil {
    // Unable to get cwd, return None
    return []component.Val{component.ValOption(nil)}, nil
}
```
**Impact:** The WIT return type is `option<string>` which natively expresses "cwd unavailable." Semantically correct.
**Already fixed?** No
**Confidence:** high

---

## `internal/engine/`

### `internal/engine/interpreter/interpreter.go:630-632, 789-805`
**Function:** `callEngine.call` (deferred recover), `callFunction` (nested snapshot handling)
**Pattern:** designed `recover()` — properly re-panics or translates to error
**Severity:** informational
**Impact:** These `recover()` sites correctly either (a) convert a panic to a returned `error` via `recoverOnCall`, or (b) re-panic non-snapshot recoveries. Not suppression.
**Already fixed?** N/A
**Confidence:** high

---

### `internal/engine/wazevo/module_engine.go:457-460`, `call_engine.go:265-276, 743-749`
**Function:** wazevo compiled engine recover sites
**Pattern:** designed `recover()` — builder-based error wrapping and snapshot handling
**Severity:** informational
**Impact:** Like the interpreter, these are the designed trap handlers. Not suppression.
**Already fixed?** N/A
**Confidence:** high

---

## `internal/sys/` and `internal/sysfs/`

### `internal/sys/fs.go:306`
**Function:** `FSContext.Renumber`
**Pattern:** discarded `Close` on file being replaced
**Severity:** low
**Code:**
```go
if toFile, ok := c.openedFiles.Lookup(to); ok {
    if toFile.IsPreopen {
        return sys.ENOTSUP
    }
    _ = toFile.File.Close()
}
```
**Impact:** `fd_renumber` closes the file being replaced. Spec note in the surrounding comment acknowledges this is best-effort to avoid Windows lock issues. If the file had unflushed buffered writes, they are silently lost. Low severity.
**Already fixed?** No
**Confidence:** high

---

### `internal/sys/fs.go:336`
**Function:** `FSContext.SockAccept`
**Pattern:** discarded `conn.Close` during nonblock-set failure
**Severity:** low
**Code:**
```go
if nonblock {
    if pf, ok := fe.File.(sys.PollableFile); ok {
        if errno = pf.SetNonblock(true); errno != 0 {
            _ = conn.Close()
            return 0, errno
        }
    }
}
```
**Impact:** Cleanup on error path; low severity.
**Already fixed?** No
**Confidence:** high

---

### `internal/sysfs/file.go:240, 490`
**Function:** `Seek` restore, directory swap Close
**Pattern:** discarded `Close` / `Seek` return
**Severity:** low
**Code:**
```go
defer func() { _, _ = rs.Seek(currentOffset, io.SeekStart) }()
...
_ = f.file.Close()
f.file = file
```
**Impact:** Read-path operations where the original file is about to be replaced. Low severity.
**Already fixed?** No
**Confidence:** high

---

### `internal/sysfs/osfile.go:128, 134`
**Function:** `reopen`
**Pattern:** discarded `Close` in reopen sequence
**Severity:** low
**Impact:** Same as sysfs/file.go.
**Already fixed?** No
**Confidence:** high

---

## `internal/wasm/`

### `internal/wasm/store.go:210-216`, `table.go:264-271`
**Function:** `ModuleInstance.applyElements`, element segment bounds check
**Pattern:** "Ignore error as it's already validated" — discarded LEB128 error
**Severity:** informational
**Code:**
```go
// Ignore error as it's already validated.
globalIdx, _, _ := leb128.LoadUint32(elem.OffsetExpr.Data)
```
**Impact:** Documented as safe because the LEB128 was validated at module compile time. This is legitimate post-validation suppression.
**Already fixed?** No
**Confidence:** high

---

### `internal/wasm/store.go:402`
**Function:** `Store.Instantiate`
**Pattern:** discarded `Close` on registration failure
**Severity:** low
**Code:**
```go
if err = s.registerModule(m); err != nil {
    _ = m.Close(ctx) // don't overwrite the error
    return nil, err
}
```
**Impact:** Intentional to preserve the original error. Low severity.
**Already fixed?** No
**Confidence:** high

---

### `internal/wasm/module_instance.go:59, 62, 77, 80`
**Function:** `closeModuleOnCanceledOrTimeout`, `CloseWithCtxErr`
**Pattern:** "TODO: figure out how to report error here" + discarded return
**Severity:** medium
**Code:**
```go
case errors.Is(ctx.Err(), context.Canceled):
    // TODO: figure out how to report error here.
    _ = m.closeWithExitCodeWithoutClosingResource(sys.ExitCodeContextCanceled)
case errors.Is(ctx.Err(), context.DeadlineExceeded):
    // TODO: figure out how to report error here.
    _ = m.closeWithExitCodeWithoutClosingResource(sys.ExitCodeDeadlineExceeded)
```
**Impact:** Context-cancellation close errors are lost because the surrounding goroutine has no error channel. Acknowledged TODO; medium severity.
**Already fixed?** No
**Confidence:** high

---

### `internal/wasm/module_instance.go:104, 117`
**Function:** `CloseWithExitCode`, `closeWithExitCodeWithoutClosingResource`
**Pattern:** discarded `deleteModule` error
**Severity:** low
**Code:**
```go
_ = m.s.deleteModule(m)
```
**Impact:** deleteModule error during close is intentional low-impact suppression.
**Already fixed?** No
**Confidence:** high

---

### `internal/wasm/module_instance.go:20`
**Function:** `FailIfClosed`
**Pattern:** discarded `ensureResourcesClosed` error
**Severity:** medium
**Code:**
```go
if closed := m.Closed.Load(); closed != 0 {
    switch closed & exitCodeFlagMask {
    case exitCodeFlagResourceNotClosed:
        _ = m.ensureResourcesClosed(context.Background())
    }
```
**Impact:** Deferred resource-close errors are lost. This path is taken when a module was closed async (via `CloseModuleOnCanceledOrTimeout`) and the actual resource close is deferred to the next access. If resource cleanup fails, the guest may leak file descriptors etc.
**Already fixed?** No
**Confidence:** medium

---

## `runtime.go` (top level)

### `runtime.go:383`
**Function:** `runtime.InstantiateModule`
**Pattern:** comment-documented discarded `Close`
**Severity:** low
**Code:**
```go
if err = s.registerModule(m); err != nil {
    _ = code.Close(ctx)  // don't overwrite the error
    ...
}
```
**Impact:** Intentional.
**Already fixed?** No
**Confidence:** high

---

### `runtime.go:405`
**Function:** `runtime.InstantiateModule` start-function error path
**Pattern:** discarded `Close`
**Severity:** low
**Code:**
```go
if _, err = start.Call(ctx); err != nil {
    _ = mod.Close(ctx) // Don't leak the module on error.
    ...
}
```
**Impact:** Intentional cleanup-on-error.
**Already fixed?** No
**Confidence:** high

---

## `experimental/`

### `experimental/features_example_test.go:167-169`
**Function:** runtime finalizer closure (example code)
**Pattern:** discarded errors in finalizer
**Severity:** informational (test/example)
**Impact:** Example code demonstrating runtime finalization. Not production.
**Already fixed?** N/A
**Confidence:** high

---

## `internal/emscripten/`

### `internal/emscripten/emscripten.go:138-151`
**Function:** `setjmp` longjmp handling
**Pattern:** intentional error filter — panic unless ThrowLongjmpError
**Severity:** informational
**Code:**
```go
// This is the equivalent of "_emscripten_stack_restore(sp);".
// Do not overwrite err here to preserve the original error.
callOrPanic(ctx, mod, "_emscripten_stack_restore", savedStack[:])

// If we encounter ThrowLongjmpError, this means that the C code did a
// longjmp, which in turn called _emscripten_throw_longjmp...
if !errors.Is(err, ThrowLongjmpError) {
    panic(err)
}
```
**Impact:** Designed longjmp handling. Correct.
**Already fixed?** N/A
**Confidence:** high

---

## `internal/component/binary/`

### `internal/component/binary/exports.go:101`
**Function:** `decodeExport`
**Pattern:** `_ = externKind` — decoder reads but does not store/validate
**Severity:** medium
**Code:**
```go
externKind, err := r.ReadByte()
if err != nil {
    return exp, fmt.Errorf("read extern desc kind: %w", err)
}
_ = externKind
```
**Impact:** The decoder reads the extern desc kind but doesn't validate that it matches the export sort. A malformed component could have mismatched kind/sort and the decoder wouldn't flag it.
**Recommendation:** Validate kind against sort.
**Already fixed?** No
**Confidence:** medium

---

### `internal/component/binary/exports.go:132`
**Function:** `decodeExportName` (versioned variant)
**Pattern:** "Skip version suffix for now" — version suffix discarded
**Severity:** medium
**Code:**
```go
case ExportNameVersioned:
    name, err := decodeName(r)
    if err != nil {
        return "", err
    }
    // Skip version suffix for now
    suffixLen, _, err := leb128.DecodeUint32(r)
    ...
```
**Impact:** Version-suffix semantic data is discarded. Exports with versioned names lose their version info. This could matter for multi-version import resolution.
**Recommendation:** Store the version.
**Already fixed?** No
**Confidence:** medium

---

### `internal/component/binary/core_type.go:142-144, 168-174, 185-188`
**Function:** `decodeCoreType` — module type import/export/alias declarations
**Pattern:** "Skip type index" with read-and-discard
**Severity:** medium
**Code:**
```go
// Skip type index based on kind
if _, _, err := leb128.DecodeUint32(r); err != nil {
    return nil, fmt.Errorf("read import %d type index: %w", i, err)
}
```
**Impact:** Core type declarations in module types have their type indices read but not stored. The component model supports module types for validation purposes; without type index info, type checks on imports/exports against module types would be incomplete.
**Recommendation:** Store the type index.
**Already fixed?** No
**Confidence:** high

---

### `internal/component/binary/core_type.go:326, 344`
**Function:** `decodeCoreCompositeType` — struct/array type
**Pattern:** "Skip struct fields for now (full GC support not required)" / "Skip array element type for now"
**Severity:** informational
**Impact:** Documented TODO tied to GC proposal support. Not a suppression bug.
**Already fixed?** No
**Confidence:** high

---

## Noteworthy Non-Findings (Reviewed but Clean)

For completeness, the following locations were inspected and are **not** suppressions:

- `internal/component/abi/lift.go`, `abi/lower.go` — these properly propagate errors through the whole chain. Contrast with the parallel implementations in `component_linker.go` and `instance.go`.
- `internal/component/canon_lower.go:385` — `_ = result` is cosmetic (indexed access preference), not error suppression.
- `internal/component/component_linker.go:2318` — `paramTypes, resultTypes, _ = coreSignature(...)` discards a `bool`, not an error.
- `internal/engine/interpreter/interpreter.go:822, 829` — `_, _ = ce.popValue(), ce.popValue()` is stack manipulation, not error suppression.
- `internal/engine/wazevo/frontend/lower.go:1600, 3867, 3872, 3873` — `_ = state.pop()` and similar are deliberate stack/SSA discards with no error returns.
- `internal/wasm/memory.go:80` — `_ = buffer[:minBytes]` is a bounds-check assertion trick, not an error discard.
- `internal/emscripten/emscripten.go` — longjmp/setjmp handling is intentional and documented.
- `internal/engine/interpreter/interpreter.go:630`, `wazevo/call_engine.go:265, 743` — `recover()` sites are the designed trap-handling mechanism.
- Resource-table trap variants (`CreateResourceDropFuncWithTrap`, `CreateResourceDropFuncWithContext`, `CreateResourceRepFuncWithTrap`) exist and are correct — the issue is that the non-trap variants are still wired in from `component_linker.go`.

---

## Summary of Patterns

| Pattern | Count | Severity |
|---|---|---|
| Silent default-on-bad-handle in wasip2 host functions (tcp/udp/http) | ~70+ call sites in 3 files | critical |
| `component_linker.go` free-function memory write/read discards | ~40+ call sites | critical |
| `createAliasExport` silent returns (5 branches + 1 Call error) | 6 | high |
| `createHostModule` silent `return nil` on error | 3 | high |
| Resource op exports using silent-ignore variants | 3 functions | critical |
| `liftFieldFromMemory` / `liftValFromMemory` default-zero on read failure | ~25 branches | critical |
| `writeRecordToMemory` silent-zero on missing field | 1 | critical |
| `writeResultsToMemory` hardcoded variant discriminant 0 | 1 | critical |
| `resolveCoreFunc` / `resolveCoreMemory` "fallback to first instance" | 2 | high |
| Silent `continue` on semver parse error | ~5 | low |
| Alias-wiring silent `continue` on !ok | 4 | medium |
| `table.Remove` return value discards in cleanup paths | ~5 | varies |
| `Close` return discards on output/write paths | ~10 | low-medium |
| Binary decoder "read and discard" (type index, extern kind, version suffix) | ~6 | medium |

## Key Takeaways

1. **The fix in `d71ffbf3` and `3f91ed37` addressed two specific lines inside `createCanonLowerFunc` but the same class of bug exists in dozens of related sites — particularly the free `writeResultsToMemory`, `liftValFromMemory`, `liftFieldFromMemory`, and all host functions in `imports/wasip2/sockets/{tcp,udp}.go` and `imports/wasip2/http/http.go`.** These are the exact files the user was asking about, and the pattern is systemic, not isolated.

2. **There is a parallel-implementation problem.** `internal/component/abi/lift.go`/`lower.go` do the right thing (propagate errors). `internal/component/component_linker.go` and `internal/component/instance.go` have their own reimplementations that don't. Any time the two diverge, the `component_linker.go`/`instance.go` versions are the ones actually used from the canon-lift/canon-lower dispatch paths. Unification would eliminate the worst findings.

3. **The sockets/HTTP host-function silent-fallback pattern is spec-violating but consistent.** It's reproduced in ~70+ functions with the same shape. A single sweep that converts every `getXxx, err := get…(...); if err != nil { return [default-val], nil }` into `if err != nil { return nil, err }` would close most of the wasip2 findings at once.

4. **Resource-op silent variants should be deleted, not kept alongside trap variants.** The existence of `CreateResourceDropFunc` and `CreateResourceRepFunc` (silent) alongside `CreateResourceDropFuncWithTrap`/`CreateResourceRepFuncWithTrap`/`CreateResourceDropFuncWithContext` is a footgun. The silent ones are still used from the production `component_linker.go` path.

5. **`writeResultsToMemory`'s hardcoded-zero variant discriminant is an acute wrong-result bug** — any guest function returning a variant type will get a zero discriminant regardless of the actual case. It is not merely "defensive coding"; it is actively producing the wrong answer. Likely cause of mysterious "wrong variant case" symptoms during integration tests.
