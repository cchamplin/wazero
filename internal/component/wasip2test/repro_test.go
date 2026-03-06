// internal/component/wasip2test/repro_test.go
//
// Reproduction tests for component model instantiation bugs.
//
// These tests use a Go WASM component (built via wasip1 adapter) that has:
//   - A shared types interface with a record type
//   - An interface import with a function the guest calls
//   - An interface export (not world-level) returning the record type
//
// Bug 1 (func not found): canon-lifted functions are not registered in the
// parent Instance's componentFuncs map during component instantiation.
//
// Bug 2 (type not found): component-defined types (records, enums, etc.) used
// across interfaces are not tracked in the parent Instance's type space, so
// resolveFromParentScope fails when a nested core module references them.
//
// Bug 3 (signature mismatch): canon lower generates i32 for u64 params instead
// of i64, causing core module instantiation to fail with signature mismatch.
package wasip2test

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/tetratelabs/wazero"
	apicomponent "github.com/tetratelabs/wazero/api/component"
	"github.com/tetratelabs/wazero/imports/wasip2"
	"github.com/tetratelabs/wazero/internal/component"
)

// newReproInstance creates a wazero runtime, registers WASI + mock host-ops,
// compiles and instantiates the go-repro-plugin component.
func newReproInstance(t *testing.T) (*component.Instance, context.Context, func()) {
	t.Helper()

	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)

	wasmBytes, err := os.ReadFile("go-repro-plugin/component.wasm")
	if err != nil {
		rt.Close(ctx)
		t.Fatalf("ReadFile: %v", err)
	}

	compiled, err := rt.CompileComponent(ctx, wasmBytes)
	if err != nil {
		rt.Close(ctx)
		t.Fatalf("CompileComponent: %v", err)
	}

	linker := component.NewComponentLinker(rt)
	linker.SetRelaxedSemverMatching(true)

	// Register WASI P2 interfaces (needed for wasip1 adapter)
	wasiLinker := component.NewLinker()
	if err := wasip2.Instantiate(wasiLinker); err != nil {
		compiled.Close(ctx)
		rt.Close(ctx)
		t.Fatalf("wasip2.Instantiate: %v", err)
	}
	linker.MergeFrom(wasiLinker)

	// Register the types interface (no functions, just type definitions)
	hostLinker := component.NewLinker()
	err = hostLinker.DefineInstance("test:repro/types").
		SkipValidation().
		Build()
	if err != nil {
		compiled.Close(ctx)
		rt.Close(ctx)
		t.Fatalf("DefineInstance types: %v", err)
	}

	// Register the host-ops import
	err = hostLinker.DefineInstance("test:repro/host-ops").
		FuncNoType("get-value", func(ctx context.Context, args []component.Val) ([]component.Val, error) {
			return []component.Val{component.ValU32(42)}, nil
		}).
		FuncNoType("get-random-len", func(ctx context.Context, args []component.Val) ([]component.Val, error) {
			// Echo back the u64 input
			return []component.Val{component.ValU64(args[0].U64())}, nil
		}).
		SkipValidation().
		Build()
	if err != nil {
		compiled.Close(ctx)
		rt.Close(ctx)
		t.Fatalf("DefineInstance host-ops: %v", err)
	}
	// Register the host-rng import (separate interface, same function name as wasi:random)
	err = hostLinker.DefineInstance("test:repro/host-rng").
		FuncNoType("get-random-bytes", func(ctx context.Context, args []component.Val) ([]component.Val, error) {
			return []component.Val{component.ValList(nil)}, nil
		}).
		SkipValidation().
		Build()
	if err != nil {
		compiled.Close(ctx)
		rt.Close(ctx)
		t.Fatalf("DefineInstance host-rng: %v", err)
	}
	linker.MergeFrom(hostLinker)

	// Set up WASI context
	var stdout, stderr bytes.Buffer
	wasiConfig := wasip2.NewConfig().
		WithStdout(&stdout).
		WithStderr(&stderr).
		WithArgs([]string{"test"}).
		WithEnviron([]string{})
	resourceTable := component.NewResourceTable()
	testCtx := wasip2.WithConfig(ctx, wasiConfig)
	testCtx = component.WithResourceTable(testCtx, resourceTable)

	instance, err := linker.Instantiate(testCtx, compiled.(*component.CompiledComponent))
	if err != nil {
		compiled.Close(ctx)
		rt.Close(ctx)
		t.Fatalf("Instantiate: %v", err)
	}

	cleanup := func() {
		compiled.Close(ctx)
		rt.Close(ctx)
	}

	return instance, testCtx, cleanup
}

// TestRepro_InterfaceExportWithHostImport reproduces the "func N not found in
// parent" bug. Canon-lifted functions (exports) must be registered in the
// parent Instance's componentFuncs map so nested core modules can reference
// them via instantiation args.
func TestRepro_InterfaceExportWithHostImport(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()

	handlerInst := instance.GetExportedInstance("test:repro/handler")
	if handlerInst == nil {
		t.Fatal("handler instance not found in component exports")
	}
	processFunc := handlerInst.ExportedFunction("process")
	if processFunc == nil {
		t.Fatal("process function not found")
	}

	results, err := processFunc.Call(testCtx)
	if err != nil {
		t.Fatalf("process() call failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// process() returns a process-result record {value: u32, ok: bool}.
	// The guest calls host-ops.get-value() (returns 42) and wraps it.
	rec := results[0].Record()
	if rec == nil {
		t.Fatalf("expected record, got kind=%v", results[0].Kind())
	}
	if got := rec["value"].U32(); got != 42 {
		t.Errorf("process().value = %d, want 42", got)
	}
	if got := rec["ok"].Bool(); !got {
		t.Errorf("process().ok = %v, want true", got)
	}
	t.Logf("process() = {value: %d, ok: %v}", rec["value"].U32(), rec["ok"].Bool())
}

// TestRepro_RecordTypeResolution reproduces the "type N not found in parent"
// bug. When a component defines record types in a shared types interface and
// uses them across import/export interfaces, those types must be tracked in
// the parent Instance's type space so resolveFromParentScope can find them
// when nested core modules reference them via instantiation args.
func TestRepro_RecordTypeResolution(t *testing.T) {
	// This test exercises the same instantiation path but focuses on the
	// type resolution aspect. If instantiation succeeds, the type was found.
	// The test deliberately calls the export to verify the record type is
	// correctly lifted back from the core module's linear memory.
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()

	handlerInst := instance.GetExportedInstance("test:repro/handler")
	if handlerInst == nil {
		t.Fatal("handler instance not found")
	}
	processFunc := handlerInst.ExportedFunction("process")
	if processFunc == nil {
		t.Fatal("process function not found")
	}

	results, err := processFunc.Call(testCtx)
	if err != nil {
		t.Fatalf("process() call failed: %v", err)
	}

	// Verify the record structure is correctly decoded
	rec := results[0].Record()
	if rec == nil {
		t.Fatalf("expected record type, got kind=%v", results[0].Kind())
	}

	// Verify both fields exist and have correct types
	valField, hasVal := rec["value"]
	okField, hasOk := rec["ok"]
	if !hasVal {
		t.Fatal("record missing 'value' field")
	}
	if !hasOk {
		t.Fatal("record missing 'ok' field")
	}

	t.Logf("Record type resolved correctly: {value: %d (kind=%v), ok: %v (kind=%v)}",
		valField.U32(), valField.Kind(), okField.Bool(), okField.Kind())
}

// TestRepro_U64CanonLowerSignature reproduces the canon lower signature mismatch
// for u64 parameters. When a host function takes u64 args, canon lower must
// produce a core function with i64 params. The bug generates i32 instead,
// causing: "signature mismatch: i64i32_v != i32i32_v" during core module
// instantiation.
func TestRepro_U64CanonLowerSignature(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()

	handlerInst := instance.GetExportedInstance("test:repro/handler")
	if handlerInst == nil {
		t.Fatal("handler instance not found")
	}
	processRandomFunc := handlerInst.ExportedFunction("process-random")
	if processRandomFunc == nil {
		t.Fatal("process-random function not found")
	}

	// Call process-random with a u64 value
	// The guest forwards to host-ops.get-random-len which echoes it back
	input := uint64(123456789)
	results, err := processRandomFunc.Call(testCtx, component.ValU64(input))
	if err != nil {
		t.Fatalf("process-random(%d) call failed: %v", input, err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	got := results[0].U64()
	if got != input {
		t.Errorf("process-random(%d) = %d, want %d", input, got, input)
	}
	t.Logf("process-random(%d) = %d", input, got)
}

// TestRepro_PublicAPISignatureMismatch reproduces the WASI random signature
// mismatch that occurs when using the public API (rt.NewComponentLinker() +
// wasip2.MergeInto + linker.DefineInstance via api.ComponentInstanceBuilder).
//
// The wasip1 adapter's core module expects get-random-bytes with signature
// (i64, i32) -> void but canon lower produces (i32, i32) -> void.
// This matches the director integration test setup.
func TestRepro_PublicAPISignatureMismatch(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	wasmBytes, err := os.ReadFile("go-repro-plugin/component.wasm")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	compiled, err := rt.CompileComponent(ctx, wasmBytes)
	if err != nil {
		t.Fatalf("CompileComponent: %v", err)
	}
	defer compiled.Close(ctx)

	// Use the PUBLIC API path — same as director integration test
	linker := rt.NewComponentLinker()
	linker.SetRelaxedSemverMatching(true)

	// Register WASI P2 via public API (MergeInto, not Instantiate+merge)
	if err := wasip2.MergeInto(linker); err != nil {
		t.Fatalf("wasip2.MergeInto: %v", err)
	}

	// Register host imports via public API DefineInstance
	err = linker.DefineInstance("test:repro/types").SkipValidation().Build()
	if err != nil {
		t.Fatalf("DefineInstance types: %v", err)
	}

	err = linker.DefineInstance("test:repro/host-ops").SkipValidation().
		Func("get-value", apicomponent.HostFunc(func(ctx context.Context, args []apicomponent.Val) ([]apicomponent.Val, error) {
			return []apicomponent.Val{apicomponent.ValU32(42)}, nil
		})).
		Func("get-random-len", apicomponent.HostFunc(func(ctx context.Context, args []apicomponent.Val) ([]apicomponent.Val, error) {
			return []apicomponent.Val{apicomponent.ValU64(args[0].U64())}, nil
		})).
		Build()
	if err != nil {
		t.Fatalf("DefineInstance host-ops: %v", err)
	}

	err = linker.DefineInstance("test:repro/host-rng").SkipValidation().
		Func("get-random-bytes", apicomponent.HostFunc(func(ctx context.Context, args []apicomponent.Val) ([]apicomponent.Val, error) {
			return []apicomponent.Val{apicomponent.ValList(nil)}, nil
		})).
		Build()
	if err != nil {
		t.Fatalf("DefineInstance host-rng: %v", err)
	}

	// Set up WASI context
	var stdout, stderr bytes.Buffer
	wasiConfig := wasip2.NewConfig().
		WithStdout(&stdout).
		WithStderr(&stderr).
		WithArgs([]string{"test"}).
		WithEnviron([]string{})
	resourceTable := apicomponent.NewResourceTable()
	testCtx := wasip2.WithConfig(ctx, wasiConfig)
	testCtx = apicomponent.WithResourceTable(testCtx, resourceTable)

	// THIS IS WHERE THE BUG MANIFESTS when using public API:
	// instantiate core instance N: import func[wasi:random/random@0.2.6.get-random-bytes]:
	// signature mismatch: i64i32_v != i32i32_v
	instance, err := linker.Instantiate(testCtx, compiled)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	defer instance.Close(ctx)

	// If instantiation succeeds, verify exports work
	handlerInst := instance.ExportedInstance("test:repro/handler")
	if handlerInst == nil {
		t.Fatal("handler instance not found")
	}
	processFunc := handlerInst.ExportedFunction("process")
	if processFunc == nil {
		t.Fatal("process function not found")
	}

	results, err := processFunc.Call(testCtx)
	if err != nil {
		t.Fatalf("process() call failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	t.Logf("process() returned: %v", results[0])
}

// TestRepro_StringParameterSupport reproduces the "unsupported parameter type:
// string" bug. When a component export takes a string parameter, canon lift
// must lower the string (ptr+len) into the component's linear memory before
// calling the core function. The bug is that the call path doesn't handle
// string types when preparing arguments for canon-lifted exports.
func TestRepro_StringParameterSupport(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()

	handlerInst := instance.GetExportedInstance("test:repro/handler")
	if handlerInst == nil {
		t.Fatal("handler instance not found")
	}
	echoFunc := handlerInst.ExportedFunction("echo-string")
	if echoFunc == nil {
		t.Fatal("echo-string function not found")
	}

	input := "hello, component model!"
	results, err := echoFunc.Call(testCtx, component.ValString(input))
	if err != nil {
		t.Fatalf("echo-string(%q) call failed: %v", input, err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	got := results[0].StringVal()
	if got != input {
		t.Errorf("echo-string(%q) = %q, want %q", input, got, input)
	}
	t.Logf("echo-string(%q) = %q", input, got)
}

// --- Primitive echo tests ---

func TestEcho_Bool(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()

	handlerInst := instance.GetExportedInstance("test:repro/handler")
	if handlerInst == nil {
		t.Fatal("handler instance not found")
	}
	fn := handlerInst.ExportedFunction("echo-bool")
	if fn == nil {
		t.Fatal("echo-bool function not found")
	}

	for _, tc := range []bool{true, false} {
		results, err := fn.Call(testCtx, component.ValBool(tc))
		if err != nil {
			t.Fatalf("echo-bool(%v) call failed: %v", tc, err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		got := results[0].Bool()
		if got != tc {
			t.Errorf("echo-bool(%v) = %v, want %v", tc, got, tc)
		}
		t.Logf("echo-bool(%v) = %v", tc, got)
	}
}

func TestEcho_S8(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()

	handlerInst := instance.GetExportedInstance("test:repro/handler")
	if handlerInst == nil {
		t.Fatal("handler instance not found")
	}
	fn := handlerInst.ExportedFunction("echo-s8")
	if fn == nil {
		t.Fatal("echo-s8 function not found")
	}

	for _, tc := range []int8{0, 1, -1, 127, -128} {
		results, err := fn.Call(testCtx, component.ValS8(tc))
		if err != nil {
			t.Fatalf("echo-s8(%d) call failed: %v", tc, err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		got := results[0].S8()
		if got != tc {
			t.Errorf("echo-s8(%d) = %d, want %d", tc, got, tc)
		}
		t.Logf("echo-s8(%d) = %d", tc, got)
	}
}

func TestEcho_U8(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()

	handlerInst := instance.GetExportedInstance("test:repro/handler")
	if handlerInst == nil {
		t.Fatal("handler instance not found")
	}
	fn := handlerInst.ExportedFunction("echo-u8")
	if fn == nil {
		t.Fatal("echo-u8 function not found")
	}

	for _, tc := range []uint8{0, 1, 255} {
		results, err := fn.Call(testCtx, component.ValU8(tc))
		if err != nil {
			t.Fatalf("echo-u8(%d) call failed: %v", tc, err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		got := results[0].U8()
		if got != tc {
			t.Errorf("echo-u8(%d) = %d, want %d", tc, got, tc)
		}
		t.Logf("echo-u8(%d) = %d", tc, got)
	}
}

func TestEcho_S16(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()

	handlerInst := instance.GetExportedInstance("test:repro/handler")
	if handlerInst == nil {
		t.Fatal("handler instance not found")
	}
	fn := handlerInst.ExportedFunction("echo-s16")
	if fn == nil {
		t.Fatal("echo-s16 function not found")
	}

	for _, tc := range []int16{0, 1, -1, 32767, -32768} {
		results, err := fn.Call(testCtx, component.ValS16(tc))
		if err != nil {
			t.Fatalf("echo-s16(%d) call failed: %v", tc, err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		got := results[0].S16()
		if got != tc {
			t.Errorf("echo-s16(%d) = %d, want %d", tc, got, tc)
		}
		t.Logf("echo-s16(%d) = %d", tc, got)
	}
}

func TestEcho_U16(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()

	handlerInst := instance.GetExportedInstance("test:repro/handler")
	if handlerInst == nil {
		t.Fatal("handler instance not found")
	}
	fn := handlerInst.ExportedFunction("echo-u16")
	if fn == nil {
		t.Fatal("echo-u16 function not found")
	}

	for _, tc := range []uint16{0, 1, 65535} {
		results, err := fn.Call(testCtx, component.ValU16(tc))
		if err != nil {
			t.Fatalf("echo-u16(%d) call failed: %v", tc, err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		got := results[0].U16()
		if got != tc {
			t.Errorf("echo-u16(%d) = %d, want %d", tc, got, tc)
		}
		t.Logf("echo-u16(%d) = %d", tc, got)
	}
}

func TestEcho_F32(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()

	handlerInst := instance.GetExportedInstance("test:repro/handler")
	if handlerInst == nil {
		t.Fatal("handler instance not found")
	}
	fn := handlerInst.ExportedFunction("echo-f32")
	if fn == nil {
		t.Fatal("echo-f32 function not found")
	}

	for _, tc := range []float32{0.0, 1.5, -1.5, 3.14159} {
		results, err := fn.Call(testCtx, component.ValF32(tc))
		if err != nil {
			t.Fatalf("echo-f32(%v) call failed: %v", tc, err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		got := results[0].F32()
		if got != tc {
			t.Errorf("echo-f32(%v) = %v, want %v", tc, got, tc)
		}
		t.Logf("echo-f32(%v) = %v", tc, got)
	}
}

func TestEcho_F64(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()

	handlerInst := instance.GetExportedInstance("test:repro/handler")
	if handlerInst == nil {
		t.Fatal("handler instance not found")
	}
	fn := handlerInst.ExportedFunction("echo-f64")
	if fn == nil {
		t.Fatal("echo-f64 function not found")
	}

	for _, tc := range []float64{0.0, 1.5, -1.5, 3.141592653589793} {
		results, err := fn.Call(testCtx, component.ValF64(tc))
		if err != nil {
			t.Fatalf("echo-f64(%v) call failed: %v", tc, err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		got := results[0].F64()
		if got != tc {
			t.Errorf("echo-f64(%v) = %v, want %v", tc, got, tc)
		}
		t.Logf("echo-f64(%v) = %v", tc, got)
	}
}

func TestEcho_Char(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()

	handlerInst := instance.GetExportedInstance("test:repro/handler")
	if handlerInst == nil {
		t.Fatal("handler instance not found")
	}
	fn := handlerInst.ExportedFunction("echo-char")
	if fn == nil {
		t.Fatal("echo-char function not found")
	}

	for _, tc := range []rune{'A', 'z', 0x1F600} {
		results, err := fn.Call(testCtx, component.ValChar(tc))
		if err != nil {
			t.Fatalf("echo-char(%U) call failed: %v", tc, err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		got := results[0].Char()
		if got != tc {
			t.Errorf("echo-char(%U) = %U, want %U", tc, got, tc)
		}
		t.Logf("echo-char(%U) = %U", tc, got)
	}
}

// --- Composite type tests ---

func TestEcho_Enum(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()

	handlerInst := instance.GetExportedInstance("test:repro/handler")
	if handlerInst == nil {
		t.Fatal("handler instance not found")
	}
	fn := handlerInst.ExportedFunction("echo-enum")
	if fn == nil {
		t.Fatal("echo-enum function not found")
	}

	for _, tc := range []string{"red", "green", "blue"} {
		results, err := fn.Call(testCtx, component.ValEnum(tc))
		if err != nil {
			t.Fatalf("echo-enum(%q) call failed: %v", tc, err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		got := results[0].Enum()
		if got != tc {
			t.Errorf("echo-enum(%q) = %q, want %q", tc, got, tc)
		}
		t.Logf("echo-enum(%q) = %q", tc, got)
	}
}

func TestEcho_Flags(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()

	handlerInst := instance.GetExportedInstance("test:repro/handler")
	if handlerInst == nil {
		t.Fatal("handler instance not found")
	}
	fn := handlerInst.ExportedFunction("echo-flags")
	if fn == nil {
		t.Fatal("echo-flags function not found")
	}

	testCases := []map[string]bool{
		{"read": true, "write": false, "execute": false},
		{"read": true, "write": true, "execute": false},
		{"read": true, "write": true, "execute": true},
		{"read": false, "write": false, "execute": false},
	}

	for _, tc := range testCases {
		results, err := fn.Call(testCtx, component.ValFlags(tc))
		if err != nil {
			t.Fatalf("echo-flags(%v) call failed: %v", tc, err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		got := results[0].Flags()
		for k, v := range tc {
			if got[k] != v {
				t.Errorf("echo-flags(%v): flag %q = %v, want %v", tc, k, got[k], v)
			}
		}
		t.Logf("echo-flags(%v) = %v", tc, got)
	}
}

func TestEcho_Variant(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()

	handlerInst := instance.GetExportedInstance("test:repro/handler")
	if handlerInst == nil {
		t.Fatal("handler instance not found")
	}
	fn := handlerInst.ExportedFunction("echo-variant")
	if fn == nil {
		t.Fatal("echo-variant function not found")
	}

	// Test circle(3.14)
	circlePayload := component.ValF64(3.14)
	results, err := fn.Call(testCtx, component.ValVariant("circle", &circlePayload))
	if err != nil {
		t.Fatalf("echo-variant(circle(3.14)) call failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	caseName, payload := results[0].Variant()
	if caseName != "circle" {
		t.Errorf("echo-variant(circle): case = %q, want %q", caseName, "circle")
	}
	if payload == nil || payload.F64() != 3.14 {
		t.Errorf("echo-variant(circle): payload = %v, want 3.14", payload)
	}
	t.Logf("echo-variant(circle(3.14)) = %s(%v)", caseName, payload.F64())

	// Test square(2.0)
	squarePayload := component.ValF64(2.0)
	results, err = fn.Call(testCtx, component.ValVariant("square", &squarePayload))
	if err != nil {
		t.Fatalf("echo-variant(square(2.0)) call failed: %v", err)
	}
	caseName, payload = results[0].Variant()
	if caseName != "square" {
		t.Errorf("echo-variant(square): case = %q, want %q", caseName, "square")
	}
	if payload == nil || payload.F64() != 2.0 {
		t.Errorf("echo-variant(square): payload = %v, want 2.0", payload)
	}
	t.Logf("echo-variant(square(2.0)) = %s(%v)", caseName, payload.F64())

	// Test none
	results, err = fn.Call(testCtx, component.ValVariant("none", nil))
	if err != nil {
		t.Fatalf("echo-variant(none) call failed: %v", err)
	}
	caseName, payload = results[0].Variant()
	if caseName != "none" {
		t.Errorf("echo-variant(none): case = %q, want %q", caseName, "none")
	}
	if payload != nil {
		t.Errorf("echo-variant(none): payload = %v, want nil", payload)
	}
	t.Logf("echo-variant(none) = %s", caseName)
}

// --- Result type tests ---

func TestMakeOk(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()

	handlerInst := instance.GetExportedInstance("test:repro/handler")
	if handlerInst == nil {
		t.Fatal("handler instance not found")
	}
	fn := handlerInst.ExportedFunction("make-ok")
	if fn == nil {
		t.Fatal("make-ok function not found")
	}

	results, err := fn.Call(testCtx, component.ValU32(42))
	if err != nil {
		t.Fatalf("make-ok(42) call failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	isOk, okVal, errVal := results[0].Result()
	if !isOk {
		t.Fatalf("make-ok(42): expected Ok result, got Err(%v)", errVal)
	}
	if okVal == nil || okVal.U32() != 42 {
		t.Errorf("make-ok(42): ok value = %v, want 42", okVal)
	}
	t.Logf("make-ok(42) = Ok(%d)", okVal.U32())
}

func TestMakeErr(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()

	handlerInst := instance.GetExportedInstance("test:repro/handler")
	if handlerInst == nil {
		t.Fatal("handler instance not found")
	}
	fn := handlerInst.ExportedFunction("make-err")
	if fn == nil {
		t.Fatal("make-err function not found")
	}

	results, err := fn.Call(testCtx, component.ValString("something went wrong"))
	if err != nil {
		t.Fatalf("make-err(%q) call failed: %v", "something went wrong", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	isOk, okVal, errVal := results[0].Result()
	if isOk {
		t.Fatalf("make-err: expected Err result, got Ok(%v)", okVal)
	}
	if errVal == nil || errVal.StringVal() != "something went wrong" {
		t.Errorf("make-err: err value = %v, want %q", errVal, "something went wrong")
	}
	t.Logf("make-err(%q) = Err(%q)", "something went wrong", errVal.StringVal())
}

// --- Multi-parameter tests ---

func TestAddThree(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()

	handlerInst := instance.GetExportedInstance("test:repro/handler")
	if handlerInst == nil {
		t.Fatal("handler instance not found")
	}
	fn := handlerInst.ExportedFunction("add-three")
	if fn == nil {
		t.Fatal("add-three function not found")
	}

	results, err := fn.Call(testCtx, component.ValU32(10), component.ValU32(20), component.ValU32(30))
	if err != nil {
		t.Fatalf("add-three(10, 20, 30) call failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	got := results[0].U32()
	if got != 60 {
		t.Errorf("add-three(10, 20, 30) = %d, want 60", got)
	}
	t.Logf("add-three(10, 20, 30) = %d", got)
}

func TestConcatStrings(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()

	handlerInst := instance.GetExportedInstance("test:repro/handler")
	if handlerInst == nil {
		t.Fatal("handler instance not found")
	}
	fn := handlerInst.ExportedFunction("concat-strings")
	if fn == nil {
		t.Fatal("concat-strings function not found")
	}

	results, err := fn.Call(testCtx, component.ValString("hello, "), component.ValString("world!"))
	if err != nil {
		t.Fatalf("concat-strings call failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	got := results[0].StringVal()
	if got != "hello, world!" {
		t.Errorf("concat-strings(\"hello, \", \"world!\") = %q, want %q", got, "hello, world!")
	}
	t.Logf("concat-strings(\"hello, \", \"world!\") = %q", got)
}

func TestMixedParams(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()

	handlerInst := instance.GetExportedInstance("test:repro/handler")
	if handlerInst == nil {
		t.Fatal("handler instance not found")
	}
	fn := handlerInst.ExportedFunction("mixed-params")
	if fn == nil {
		t.Fatal("mixed-params function not found")
	}

	// With flag=true, expect "alice:42"
	results, err := fn.Call(testCtx, component.ValString("alice"), component.ValU32(42), component.ValBool(true))
	if err != nil {
		t.Fatalf("mixed-params(\"alice\", 42, true) call failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	got := results[0].StringVal()
	if got != "alice:42" {
		t.Errorf("mixed-params(\"alice\", 42, true) = %q, want %q", got, "alice:42")
	}
	t.Logf("mixed-params(\"alice\", 42, true) = %q", got)

	// With flag=false, expect "alice"
	results, err = fn.Call(testCtx, component.ValString("alice"), component.ValU32(42), component.ValBool(false))
	if err != nil {
		t.Fatalf("mixed-params(\"alice\", 42, false) call failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	got = results[0].StringVal()
	if got != "alice" {
		t.Errorf("mixed-params(\"alice\", 42, false) = %q, want %q", got, "alice")
	}
	t.Logf("mixed-params(\"alice\", 42, false) = %q", got)
}
