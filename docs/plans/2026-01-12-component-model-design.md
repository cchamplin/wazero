# WebAssembly Component Model Implementation Design

**Date:** 2026-01-12
**Status:** Draft
**Goal:** Full specification compliance with the WebAssembly Component Model and WASI Preview 2

## Overview

This document describes the design for implementing the WebAssembly Component Model in wazero. The implementation will provide complete support for the Component Model binary format, Canonical ABI, resource management, and all WASI Preview 2 interfaces.

## Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Architecture | Parallel `internal/component/` package | Clean separation, component-specific concerns isolated |
| Parser | Single-pass streaming | Matches existing wazero pattern, lower memory |
| Type marshalling | Hybrid (dynamic Val + interfaces) | Flexibility for tooling, performance for known types |
| Resource handles | Generation-counted | Prevents use-after-free, matches spec semantics |
| Testing | Port wasmtime tests | Reference implementation conformance |
| First milestone | Primitives + simple calls | End-to-end proof with minimal complexity |
| Public API | Unified Runtime | Consistent UX, shared engine/store |
| WASI P2 | Pluggable with defaults, complete | Works out of box, customizable, no gaps |
| Engine integration | Engine-agnostic layer | Components orchestrate, don't execute directly |

## Project Structure

```
internal/component/
├── binary/
│   ├── decoder.go          # Component binary parser
│   ├── section.go          # Section type definitions
│   └── types.go            # Binary format type parsing
├── types/
│   ├── primitive.go        # Bool, integers, floats, char, string
│   ├── composite.go        # Record, variant, tuple, list, option, result, flags, enum
│   ├── resource.go         # Resource, own, borrow handle types
│   └── function.go         # Component function types
├── abi/
│   ├── canonical.go        # Lift/lower coordination
│   ├── lift.go             # Component → Go value conversion
│   ├── lower.go            # Go → component value conversion
│   ├── memory.go           # Memory layout calculations
│   └── strings.go          # UTF-8/UTF-16/Latin1 encoding
├── component.go            # Component struct (parsed representation)
├── instance.go             # ComponentInstance (runtime state)
├── linker.go               # Import resolution and linking
├── resource_table.go       # Generation-counted resource handles
├── val.go                  # Dynamic Val type
└── store.go                # Component store integration

imports/wasip2/
├── cli/                    # wasi:cli interfaces
├── filesystem/             # wasi:filesystem interfaces
├── io/                     # wasi:io interfaces
├── clocks/                 # wasi:clocks interfaces
├── random/                 # wasi:random interfaces
├── sockets/                # wasi:sockets interfaces
└── http/                   # wasi:http interfaces
```

## Binary Format Parser

Components are distinguished from core modules by the layer field after the version:

```go
// internal/component/binary/decoder.go

const (
    Magic          = "\x00asm"
    VersionPre     = []byte{0x0d, 0x00}  // Pre-standard version
    LayerModule    = []byte{0x00, 0x00}  // Core module
    LayerComponent = []byte{0x01, 0x00}  // Component
)

// Component section IDs (different from core module sections)
const (
    SectionCoreCustom   = 0
    SectionCoreModule   = 1   // Embedded core wasm module
    SectionCoreInstance = 2
    SectionCoreType     = 3
    SectionComponent    = 4   // Nested component
    SectionInstance     = 5
    SectionAlias        = 6
    SectionType         = 7   // Component types
    SectionCanon        = 8   // Canonical definitions
    SectionStart        = 9
    SectionImport       = 10
    SectionExport       = 11
    SectionValue        = 12
)

func DecodeComponent(r io.Reader) (*Component, error) {
    // 1. Validate magic + version + layer
    // 2. Stream through sections in order
    // 3. For SectionCoreModule: delegate to wasm.DecodeModule()
    // 4. For SectionComponent: recurse into DecodeComponent()
    // 5. Build Component struct with all parsed content
}
```

Nested core modules are parsed using the existing `internal/wasm/binary` decoder.

## Type System

The component type system represents WIT types in Go:

```go
// internal/component/types/types.go

// ValType represents any component model value type
type ValType interface {
    valType()
    Size() uint32      // Size in bytes when stored in memory
    Align() uint32     // Alignment requirement
    FlattenCount() int // Number of core wasm values when flattened
}

// Primitives
type Bool struct{}
type S8 struct{}
type U8 struct{}
type S16 struct{}
type U16 struct{}
type S32 struct{}
type U32 struct{}
type S64 struct{}
type U64 struct{}
type F32 struct{}
type F64 struct{}
type Char struct{}    // Unicode scalar value
type String struct{}  // ptr + len when lowered

// Composites
type Record struct {
    Fields []Field  // Named fields in order
}

type Variant struct {
    Cases []Case    // Discriminant + optional payload
}

type List struct {
    Element ValType  // Element type
}

type Option struct {
    Some ValType     // None represented as discriminant 0
}

type Result struct {
    Ok    ValType    // May be nil (no payload)
    Error ValType    // May be nil (no payload)
}

type Flags struct {
    Names []string   // Flag names, packed into u8/u16/u32
}

type Enum struct {
    Cases []string   // Discriminant-only variant
}

type Tuple struct {
    Types []ValType  // Positional types
}
```

## Resource Types and Handle Management

Resources use generation counting to prevent use-after-free:

```go
// internal/component/resource_table.go

// Handle is a 64-bit value: upper 32 = generation, lower 32 = index
type Handle uint64

func (h Handle) Index() uint32      { return uint32(h) }
func (h Handle) Generation() uint32 { return uint32(h >> 32) }
func MakeHandle(idx, gen uint32) Handle {
    return Handle(uint64(gen)<<32 | uint64(idx))
}

type ResourceTable struct {
    entries    []resourceEntry
    freeHead   int32  // Head of free list, -1 if empty
    generation uint32 // Monotonically increasing
}

type resourceEntry struct {
    state       entryState
    generation  uint32
    data        any           // The actual resource value
    nextFree    int32         // -1 if end of free list
    borrowCount uint32        // Active borrows (must be 0 to drop)
}

func (t *ResourceTable) New(data any) Handle
func (t *ResourceTable) Rep(h Handle) (any, error)
func (t *ResourceTable) Drop(h Handle) (any, error)
func (t *ResourceTable) Borrow(h Handle) (Handle, error)
func (t *ResourceTable) EndBorrow(h Handle) error

// Borrow tracking per call
func (t *ResourceTable) EnterCall()
func (t *ResourceTable) ExitCall()
```

## Dynamic Val Type

The `Val` type provides runtime-typed values:

```go
// internal/component/val.go

type Val struct {
    kind ValKind
    v    any
}

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

// Constructors
func ValBool(v bool) Val
func ValS32(v int32) Val
func ValString(v string) Val
func ValList(vals []Val) Val
func ValRecord(fields []Val) Val
func ValVariant(disc uint32, payload *Val) Val
func ValOwn(h Handle) Val

// Accessors
func (v Val) Bool() bool
func (v Val) S32() int32
func (v Val) String() string
func (v Val) List() []Val
```

## Canonical ABI

The Canonical ABI converts between component values and core wasm representation:

```go
// internal/component/abi/canonical.go

const MaxFlatParams = 16
const MaxFlatResults = 1

type Options struct {
    Memory      api.Memory
    Realloc     api.Function
    PostReturn  api.Function
    StringEnc   StringEncoding
}

type StringEncoding uint8
const (
    StringEncodingUTF8 StringEncoding = iota
    StringEncodingUTF16
    StringEncodingLatin1UTF16
)

// internal/component/abi/lower.go

func Lower(opts *Options, typ ValType, val Val) ([]uint64, error) {
    flat := typ.FlattenCount()
    if flat <= MaxFlatParams {
        return lowerFlat(typ, val)
    }
    ptr, err := lowerToMemory(opts, typ, val)
    return []uint64{uint64(ptr)}, err
}

// internal/component/abi/lift.go

func Lift(opts *Options, typ ValType, vals []uint64) (Val, error) {
    flat := typ.FlattenCount()
    if flat <= MaxFlatResults {
        return liftFlat(typ, vals)
    }
    return liftFromMemory(opts, typ, uint32(vals[0]))
}
```

## Component and Instance Structures

```go
// internal/component/component.go

type Component struct {
    CoreModules    []*wasm.Module
    Components     []*Component
    CoreTypes      []CoreTypeDef
    Types          []ComponentTypeDef
    Imports        []Import
    Exports        []Export
    Canonicals     []CanonicalDef
    Aliases        []Alias
    Start          *StartDef
}

// internal/component/instance.go

type Instance struct {
    component      *Component
    coreInstances  []*wasm.ModuleInstance
    subInstances   []*Instance
    resources      map[ResourceTypeID]*ResourceTable
    exports        map[string]ExportInstance
    options        *Options
}

type ExportInstance struct {
    Kind     ExportKind
    Func     *Func
    Instance *Instance
}

type Func struct {
    funcType   *FuncType
    liftedCore api.Function
    options    *Options
}
```

## Linker

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

## Public API

```go
// wazero.go additions

type Runtime interface {
    // Existing module methods...
    CompileModule(ctx context.Context, binary []byte) (CompiledModule, error)
    InstantiateModule(ctx context.Context, compiled CompiledModule, config ModuleConfig) (api.Module, error)

    // New component methods
    CompileComponent(ctx context.Context, binary []byte) (CompiledComponent, error)
    InstantiateComponent(ctx context.Context, compiled CompiledComponent) (Component, error)
    NewComponentLinker() ComponentLinker
}

// api/component.go

type CompiledComponent interface {
    Imports() []ComponentImport
    Exports() []ComponentExport
    Close(context.Context) error
}

type Component interface {
    ExportedFunction(name string) ComponentFunc
    ExportedInstance(name string) Component
    Close(context.Context) error
}

type ComponentFunc interface {
    Type() ComponentFuncType
    Call(ctx context.Context, params ...Val) ([]Val, error)
}

type ComponentLinker interface {
    DefineFunc(namespace, name string, fn HostFunc) error
    DefineInstance(namespace string) InstanceBuilder
    DefineResource(namespace, name string, dtor ResourceDestructor) error
    Instantiate(ctx context.Context, compiled CompiledComponent) (Component, error)
}

type HostFunc func(context.Context, []Val) ([]Val, error)
type ResourceDestructor func(rep uint32)
```

## WASI Preview 2 Implementation

```go
// imports/wasip2/wasip2.go

func Instantiate(ctx context.Context, linker wazero.ComponentLinker) error {
    if err := cli.Instantiate(ctx, linker); err != nil {
        return err
    }
    if err := filesystem.Instantiate(ctx, linker); err != nil {
        return err
    }
    if err := io.Instantiate(ctx, linker); err != nil {
        return err
    }
    if err := clocks.Instantiate(ctx, linker); err != nil {
        return err
    }
    if err := random.Instantiate(ctx, linker); err != nil {
        return err
    }
    if err := sockets.Instantiate(ctx, linker); err != nil {
        return err
    }
    if err := http.Instantiate(ctx, linker); err != nil {
        return err
    }
    return nil
}
```

WASI P2 interfaces to implement:
- `wasi:cli` (environment, exit, stdin, stdout, stderr, terminal)
- `wasi:filesystem` (types, preopens)
- `wasi:io` (streams, poll, error)
- `wasi:clocks` (monotonic, wall)
- `wasi:random` (random, insecure, insecure-seed)
- `wasi:sockets` (tcp, udp, ip-name-lookup, network)
- `wasi:http` (types, incoming-handler, outgoing-handler)

## Testing Strategy

Test components are committed as binary fixtures, built from WIT using cargo-component:

```
internal/component/testdata/
├── primitives/
│   ├── add_i32.wasm
│   ├── primitives_all.wasm
│   └── primitives.wit
├── strings/
│   ├── echo_string.wasm
│   ├── string_encodings.wasm
│   └── strings.wit
├── composites/
│   ├── record_simple.wasm
│   ├── variant_cases.wasm
│   ├── list_operations.wasm
│   └── composites.wit
├── resources/
│   ├── resource_own.wasm
│   ├── resource_borrow.wasm
│   ├── resource_drop.wasm
│   └── resources.wit
└── wasip2/
    ├── cli_args.wasm
    ├── fs_read.wasm
    ├── http_fetch.wasm
    └── ...
```

Example test:

```go
func TestPrimitiveAdd(t *testing.T) {
    ctx := context.Background()
    rt := wazero.NewRuntime(ctx)
    defer rt.Close(ctx)

    binary, _ := testdata.ReadFile("testdata/primitives/add_i32.wasm")
    compiled, err := rt.CompileComponent(ctx, binary)
    require.NoError(t, err)

    linker := rt.NewComponentLinker()
    instance, err := linker.Instantiate(ctx, compiled)
    require.NoError(t, err)

    add := instance.ExportedFunction("add")
    results, err := add.Call(ctx, ValS32(2), ValS32(3))
    require.NoError(t, err)
    require.Equal(t, int32(5), results[0].S32())
}
```

## Implementation Phases

### Phase 1: Binary Parser & Primitives
- Component binary detection and section parsing
- Type section parsing (primitives only)
- Core module extraction (delegate to existing decoder)
- First test: parse a minimal component, verify structure
- Second test: instantiate component, call `add(s32, s32) -> s32`

### Phase 2: Complete Type System
- All WIT types: record, variant, list, option, result, flags, enum, tuple
- Canonical ABI lift/lower for all types
- Memory layout calculations
- String encoding (UTF-8, UTF-16, Latin1)
- Tests: round-trip all composite types

### Phase 3: Resources
- Resource type parsing
- Generation-counted handle tables
- Own/borrow semantics
- Destructor invocation
- Borrow tracking per call scope
- Tests: resource lifecycle, borrow validation

### Phase 4: Full Instantiation & Linking
- Alias section handling
- Canonical definitions (lift/lower wrappers)
- Component imports/exports
- Linker with semver matching
- Nested components and instances
- Tests: multi-component linking

### Phase 5: WASI Preview 2
- All interfaces: cli, filesystem, io, clocks, random, sockets, http
- Pluggable configuration
- Integration tests with real wasi-cli components
- Tests: cargo-component-built binaries run correctly

### Phase 6: Polish & Conformance
- Edge cases from wasmtime test suite
- Performance optimization
- Documentation and examples

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Spec evolution (Component Model is pre-1.0) | Track wasmtime closely, version our implementation |
| Performance (Canonical ABI overhead) | Optimize hot paths, consider code-gen later |
| Complexity (large surface area) | Strict phasing, comprehensive tests at each phase |

## Dependencies

- No new external Go dependencies (maintains wazero's zero-dep philosophy)
- Build-time: `cargo-component` and `wasm-tools` for generating test fixtures

## References

- [Component Model Introduction](https://component-model.bytecodealliance.org/introduction.html)
- [WIT Reference](https://component-model.bytecodealliance.org/design/wit.html)
- [Component Model Binary Format](https://github.com/WebAssembly/component-model/blob/main/design/mvp/Binary.md)
- [Canonical ABI Specification](https://github.com/WebAssembly/component-model/blob/main/design/mvp/CanonicalABI.md)
- [Component Model Explainer](https://github.com/WebAssembly/component-model/blob/main/design/mvp/Explainer.md)
- [Wasmtime Component Model Implementation](https://github.com/bytecodealliance/wasmtime/tree/main/crates/wasmtime/src/runtime/component)
