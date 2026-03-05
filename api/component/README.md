## api/component

This package provides the public types for the WebAssembly Component Model's
dynamic value system and host function interface.

### When do you need this package?

**Calling component-exported functions:** You don't need this package. Pass Go
primitives directly to `ComponentFunc.Call()`:

```go
results, err := instance.ExportedFunction("add").Call(ctx, int32(2), int32(3))
sum := results[0].(int32) // 5
```

Go types are converted automatically:

| Go type          | Component Model type |
|------------------|---------------------|
| `bool`           | `bool`              |
| `int8` / `uint8` | `s8` / `u8`         |
| `int16` / `uint16` | `s16` / `u16`     |
| `int32` / `uint32` | `s32` / `u32`     |
| `int64` / `uint64` | `s64` / `u64`     |
| `float32` / `float64` | `f32` / `f64` |
| `string`         | `string`            |
| `map[string]any` | `record`            |
| `[]any`          | `list`              |

Results are returned as Go native types using the same mapping. Result types
come back as `map[string]any{"ok": bool, "value": ..., "error": ...}`.

**Defining host functions:** You need `HostFunc` and `Val` from this package to
implement functions that satisfy component imports:

```go
import "github.com/tetratelabs/wazero/api/component"

linker := rt.NewComponentLinker()
err := linker.DefineInstance("my:app/math").
    Func("double", component.HostFunc(func(ctx context.Context, args []component.Val) ([]component.Val, error) {
        x := args[0].S32()
        return []component.Val{component.ValS32(x * 2)}, nil
    })).
    Build()
```

### Val constructors

| Constructor | Component type |
|-------------|---------------|
| `ValBool(b)` | `bool` |
| `ValS8(n)` ... `ValU64(n)` | integer types |
| `ValF32(f)`, `ValF64(f)` | floating point |
| `ValChar(r)` | `char` |
| `ValString(s)` | `string` |
| `ValRecord(map[string]Val)` | `record` |
| `ValList([]Val)` | `list<T>` |
| `ValTuple([]Val)` | `tuple<...>` |
| `ValOption(*Val)` | `option<T>` (`nil` = none) |
| `ValResultOk(*Val)` | `result<T, E>` (ok) |
| `ValResultErr(*Val)` | `result<T, E>` (error) |
| `ValVariant(name, *Val)` | `variant` |
| `ValEnum(name)` | `enum` |
| `ValFlags(map[string]bool)` | `flags` |
| `ValOwn(handle)`, `ValBorrow(handle)` | resource handles |

### Val accessors

Each Val has a `Kind()` method returning its `ValKind`, plus typed accessors:
`Bool()`, `S32()`, `StringVal()`, `Record()`, `RecordField(name)`, `List()`,
`Option()`, `Result()`, `Variant()`, `Enum()`, `Flags()`, `Own()`, `Borrow()`,
etc.

### Examples

See the [examples](../../examples/) directory:
- [component-basic](../../examples/component-basic/) — calling exported functions
- [component-types](../../examples/component-types/) — records, options, lists, results
- [component-host-functions](../../examples/component-host-functions/) — defining host functions
- [component-wasip2](../../examples/component-wasip2/) — WASI Preview 2 components
