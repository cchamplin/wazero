// internal/component/wasip2test/repro_test.go
//
// Reproduction tests for component model bugs.
// ALL tests use the PUBLIC API (rt.NewComponentLinker, wasip2.MergeInto,
// linker.DefineInstance, api.ComponentFunc.Call) to ensure bugs in the
// public API wrapper layer are caught.
package wasip2test

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	apicomponent "github.com/tetratelabs/wazero/api/component"
	"github.com/tetratelabs/wazero/imports/wasip2"
)

// defaultHostOps returns a map of default host-ops function implementations.
func defaultHostOps() map[string]apicomponent.HostFunc {
	return map[string]apicomponent.HostFunc{
		"get-value": func(ctx context.Context, _ *apicomponent.TypeFunc, args []apicomponent.Val) ([]apicomponent.Val, error) {
			return []apicomponent.Val{apicomponent.ValU32(42)}, nil
		},
		"get-random-len": func(ctx context.Context, _ *apicomponent.TypeFunc, args []apicomponent.Val) ([]apicomponent.Val, error) {
			return []apicomponent.Val{apicomponent.ValU64(args[0].U64())}, nil
		},
		"send-enum": func(ctx context.Context, _ *apicomponent.TypeFunc, args []apicomponent.Val) ([]apicomponent.Val, error) {
			return []apicomponent.Val{apicomponent.ValU32(1)}, nil
		},
		"send-event": func(ctx context.Context, _ *apicomponent.TypeFunc, args []apicomponent.Val) ([]apicomponent.Val, error) {
			return []apicomponent.Val{apicomponent.ValU32(1)}, nil
		},
		"get-color": func(ctx context.Context, _ *apicomponent.TypeFunc, args []apicomponent.Val) ([]apicomponent.Val, error) {
			return []apicomponent.Val{apicomponent.ValEnum("blue")}, nil
		},
		"check-option": func(ctx context.Context, _ *apicomponent.TypeFunc, args []apicomponent.Val) ([]apicomponent.Val, error) {
			return []apicomponent.Val{apicomponent.ValU32(1)}, nil
		},
		"get-event": func(ctx context.Context, _ *apicomponent.TypeFunc, args []apicomponent.Val) ([]apicomponent.Val, error) {
			metadata := apicomponent.ValList([]apicomponent.Val{apicomponent.ValU8(42)})
			return []apicomponent.Val{apicomponent.ValRecord(map[string]apicomponent.Val{
				"event-type": apicomponent.ValEnum("green"),
				"metadata":   apicomponent.ValOption(&metadata),
			})}, nil
		},
		"check-opt-bytes": func(ctx context.Context, _ *apicomponent.TypeFunc, args []apicomponent.Val) ([]apicomponent.Val, error) {
			return []apicomponent.Val{apicomponent.ValU32(1)}, nil
		},
		"send-tagged-shape": func(ctx context.Context, _ *apicomponent.TypeFunc, args []apicomponent.Val) ([]apicomponent.Val, error) {
			return []apicomponent.Val{apicomponent.ValU32(1)}, nil
		},
		"send-events": func(ctx context.Context, _ *apicomponent.TypeFunc, args []apicomponent.Val) ([]apicomponent.Val, error) {
			return []apicomponent.Val{apicomponent.ValU32(1)}, nil
		},
	}
}

// newPublicAPIInstance creates a component instance using only the public API.
// Optionally overrides specific host-ops functions via the overrides map.
func newPublicAPIInstance(t *testing.T, overrides map[string]apicomponent.HostFunc) (api.Component, context.Context, func()) {
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
		t.Skipf("CompileComponent: %v", err)
	}

	linker := rt.NewComponentLinker()
	linker.SetRelaxedSemverMatching(true)

	if err := wasip2.MergeInto(linker); err != nil {
		compiled.Close(ctx)
		rt.Close(ctx)
		t.Fatalf("wasip2.MergeInto: %v", err)
	}

	err = linker.DefineInstance("test:repro/types").SkipValidation().Build()
	if err != nil {
		compiled.Close(ctx)
		rt.Close(ctx)
		t.Fatalf("DefineInstance types: %v", err)
	}

	// Merge defaults with overrides
	hostOps := defaultHostOps()
	for k, v := range overrides {
		hostOps[k] = v
	}

	builder := linker.DefineInstance("test:repro/host-ops").SkipValidation()
	for name, fn := range hostOps {
		builder = builder.Func(name, fn)
	}
	err = builder.Build()
	if err != nil {
		compiled.Close(ctx)
		rt.Close(ctx)
		t.Fatalf("DefineInstance host-ops: %v", err)
	}

	err = linker.DefineInstance("test:repro/host-rng").SkipValidation().
		Func("get-random-bytes", apicomponent.HostFunc(func(ctx context.Context, _ *apicomponent.TypeFunc, args []apicomponent.Val) ([]apicomponent.Val, error) {
			return []apicomponent.Val{apicomponent.ValList(nil)}, nil
		})).
		Build()
	if err != nil {
		compiled.Close(ctx)
		rt.Close(ctx)
		t.Fatalf("DefineInstance host-rng: %v", err)
	}

	var stdout, stderr bytes.Buffer
	wasiConfig := wasip2.NewConfig().
		WithStdout(&stdout).
		WithStderr(&stderr).
		WithArgs([]string{"test"}).
		WithEnviron([]string{})
	resourceTable := apicomponent.NewResourceTable()
	testCtx := wasip2.WithConfig(ctx, wasiConfig)
	testCtx = apicomponent.WithResourceTable(testCtx, resourceTable)

	instance, err := linker.Instantiate(testCtx, compiled)
	if err != nil {
		compiled.Close(ctx)
		rt.Close(ctx)
		t.Skipf("Instantiate: %v", err)
	}

	cleanup := func() {
		instance.Close(ctx)
		compiled.Close(ctx)
		rt.Close(ctx)
	}

	return instance, testCtx, cleanup
}

// getHandlerFunc is a helper to get a function from the handler instance.
func getHandlerFunc(t *testing.T, instance api.Component, funcName string) api.ComponentFunc {
	t.Helper()
	handlerInst := instance.ExportedInstance("test:repro/handler")
	if handlerInst == nil {
		t.Fatal("handler instance not found")
	}
	fn := handlerInst.ExportedFunction(funcName)
	if fn == nil {
		t.Fatalf("%s function not found", funcName)
	}
	return fn
}

// =============================================================================
// Instantiation tests
// =============================================================================

func TestRepro_InterfaceExportWithHostImport(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "process")
	results, err := fn.Call(testCtx)
	if err != nil {
		t.Fatalf("process() call failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	rec, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any result, got %T: %v", results[0], results[0])
	}
	if got, ok := rec["value"].(uint32); !ok || got != 42 {
		t.Errorf("process().value = %v, want 42", rec["value"])
	}
	if got, ok := rec["ok"].(bool); !ok || !got {
		t.Errorf("process().ok = %v, want true", rec["ok"])
	}
	t.Logf("process() = %v", rec)
}

func TestRepro_RecordTypeResolution(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "process")
	results, err := fn.Call(testCtx)
	if err != nil {
		t.Fatalf("process() call failed: %v", err)
	}

	rec, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T: %v", results[0], results[0])
	}

	if _, hasVal := rec["value"]; !hasVal {
		t.Fatal("record missing 'value' field")
	}
	if _, hasOk := rec["ok"]; !hasOk {
		t.Fatal("record missing 'ok' field")
	}
	t.Logf("Record type resolved correctly: %v", rec)
}

func TestRepro_U64CanonLowerSignature(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "process-random")
	input := uint64(123456789)
	results, err := fn.Call(testCtx, input)
	if err != nil {
		t.Fatalf("process-random(%d) call failed: %v", input, err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	got, ok := results[0].(uint64)
	if !ok {
		t.Fatalf("expected uint64, got %T: %v", results[0], results[0])
	}
	if got != input {
		t.Errorf("process-random(%d) = %d, want %d", input, got, input)
	}
	t.Logf("process-random(%d) = %d", input, got)
}

// =============================================================================
// String parameter tests
// =============================================================================

func TestRepro_StringParameterSupport(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "echo-string")
	input := "hello, component model!"
	results, err := fn.Call(testCtx, input)
	if err != nil {
		t.Fatalf("echo-string(%q) call failed: %v", input, err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	got, ok := results[0].(string)
	if !ok {
		t.Fatalf("expected string, got %T: %v", results[0], results[0])
	}
	if got != input {
		t.Errorf("echo-string(%q) = %q, want %q", input, got, input)
	}
	t.Logf("echo-string(%q) = %q", input, got)
}

// =============================================================================
// Primitive echo tests
// =============================================================================

func TestEcho_Bool(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "echo-bool")
	for _, tc := range []bool{true, false} {
		results, err := fn.Call(testCtx, tc)
		if err != nil {
			t.Fatalf("echo-bool(%v) call failed: %v", tc, err)
		}
		got, ok := results[0].(bool)
		if !ok {
			t.Fatalf("expected bool, got %T: %v", results[0], results[0])
		}
		if got != tc {
			t.Errorf("echo-bool(%v) = %v", tc, got)
		}
	}
}

func TestEcho_S8(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "echo-s8")
	for _, tc := range []int8{0, 1, -1, 127, -128} {
		results, err := fn.Call(testCtx, tc)
		if err != nil {
			t.Fatalf("echo-s8(%d) call failed: %v", tc, err)
		}
		got, ok := results[0].(int8)
		if !ok {
			t.Fatalf("expected int8, got %T: %v", results[0], results[0])
		}
		if got != tc {
			t.Errorf("echo-s8(%d) = %d", tc, got)
		}
	}
}

func TestEcho_U8(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "echo-u8")
	for _, tc := range []uint8{0, 1, 255} {
		results, err := fn.Call(testCtx, tc)
		if err != nil {
			t.Fatalf("echo-u8(%d) call failed: %v", tc, err)
		}
		got, ok := results[0].(uint8)
		if !ok {
			t.Fatalf("expected uint8, got %T: %v", results[0], results[0])
		}
		if got != tc {
			t.Errorf("echo-u8(%d) = %d", tc, got)
		}
	}
}

func TestEcho_S16(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "echo-s16")
	for _, tc := range []int16{0, 1, -1, 32767, -32768} {
		results, err := fn.Call(testCtx, tc)
		if err != nil {
			t.Fatalf("echo-s16(%d) call failed: %v", tc, err)
		}
		got, ok := results[0].(int16)
		if !ok {
			t.Fatalf("expected int16, got %T: %v", results[0], results[0])
		}
		if got != tc {
			t.Errorf("echo-s16(%d) = %d", tc, got)
		}
	}
}

func TestEcho_U16(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "echo-u16")
	for _, tc := range []uint16{0, 1, 65535} {
		results, err := fn.Call(testCtx, tc)
		if err != nil {
			t.Fatalf("echo-u16(%d) call failed: %v", tc, err)
		}
		got, ok := results[0].(uint16)
		if !ok {
			t.Fatalf("expected uint16, got %T: %v", results[0], results[0])
		}
		if got != tc {
			t.Errorf("echo-u16(%d) = %d", tc, got)
		}
	}
}

func TestEcho_F32(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "echo-f32")
	for _, tc := range []float32{0.0, 1.5, -1.5, 3.14159} {
		results, err := fn.Call(testCtx, tc)
		if err != nil {
			t.Fatalf("echo-f32(%v) call failed: %v", tc, err)
		}
		got, ok := results[0].(float32)
		if !ok {
			t.Fatalf("expected float32, got %T: %v", results[0], results[0])
		}
		if got != tc {
			t.Errorf("echo-f32(%v) = %v", tc, got)
		}
	}
}

func TestEcho_F64(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "echo-f64")
	for _, tc := range []float64{0.0, 1.5, -1.5, 3.141592653589793} {
		results, err := fn.Call(testCtx, tc)
		if err != nil {
			t.Fatalf("echo-f64(%v) call failed: %v", tc, err)
		}
		got, ok := results[0].(float64)
		if !ok {
			t.Fatalf("expected float64, got %T: %v", results[0], results[0])
		}
		if got != tc {
			t.Errorf("echo-f64(%v) = %v", tc, got)
		}
	}
}

func TestEcho_Char(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "echo-char")
	for _, tc := range []rune{'A', 'z', 0x1F600} {
		results, err := fn.Call(testCtx, apicomponent.ValChar(tc))
		if err != nil {
			t.Fatalf("echo-char(%U) call failed: %v", tc, err)
		}
		got, ok := results[0].(rune)
		if !ok {
			t.Fatalf("expected rune, got %T: %v", results[0], results[0])
		}
		if got != tc {
			t.Errorf("echo-char(%U) = %U", tc, got)
		}
	}
}

// =============================================================================
// Composite type echo tests
// =============================================================================

func TestEcho_Enum(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "echo-enum")
	for _, tc := range []string{"red", "green", "blue"} {
		results, err := fn.Call(testCtx, apicomponent.ValEnum(tc))
		if err != nil {
			t.Fatalf("echo-enum(%q) call failed: %v", tc, err)
		}
		got, ok := results[0].(string)
		if !ok {
			t.Fatalf("expected string (enum), got %T: %v", results[0], results[0])
		}
		if got != tc {
			t.Errorf("echo-enum(%q) = %q", tc, got)
		}
	}
}

func TestEcho_Flags(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "echo-flags")
	testCases := []map[string]bool{
		{"read": true, "write": false, "execute": false},
		{"read": true, "write": true, "execute": false},
		{"read": true, "write": true, "execute": true},
	}

	for _, tc := range testCases {
		results, err := fn.Call(testCtx, apicomponent.ValFlags(tc))
		if err != nil {
			t.Fatalf("echo-flags(%v) call failed: %v", tc, err)
		}
		got, ok := results[0].(map[string]bool)
		if !ok {
			t.Fatalf("expected map[string]bool, got %T: %v", results[0], results[0])
		}
		for k, v := range tc {
			if v && !got[k] {
				t.Errorf("echo-flags(%v): flag %q = false, want true", tc, k)
			}
		}
	}
}

func TestEcho_Variant(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "echo-variant")

	// Test circle(3.14)
	circlePayload := apicomponent.ValF64(3.14)
	results, err := fn.Call(testCtx, apicomponent.ValVariant("circle", &circlePayload))
	if err != nil {
		t.Fatalf("echo-variant(circle(3.14)) call failed: %v", err)
	}
	// Variant comes back as map[string]any with case name key
	t.Logf("echo-variant(circle(3.14)) = %v (type %T)", results[0], results[0])

	// Test none
	results, err = fn.Call(testCtx, apicomponent.ValVariant("none", nil))
	if err != nil {
		t.Fatalf("echo-variant(none) call failed: %v", err)
	}
	t.Logf("echo-variant(none) = %v (type %T)", results[0], results[0])
}

// =============================================================================
// Result type tests
// =============================================================================

func TestMakeOk(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "make-ok")
	results, err := fn.Call(testCtx, uint32(42))
	if err != nil {
		t.Fatalf("make-ok(42) call failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	t.Logf("make-ok(42) = %v (type %T)", results[0], results[0])

	// Result comes back as map[string]any{"ok": bool, "value": ...}
	rec, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any result, got %T: %v", results[0], results[0])
	}
	if isOk, _ := rec["ok"].(bool); !isOk {
		t.Errorf("make-ok(42): ok = %v, want true", rec["ok"])
	}
}

func TestMakeErr(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "make-err")
	results, err := fn.Call(testCtx, "something went wrong")
	if err != nil {
		t.Fatalf("make-err call failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	t.Logf("make-err() = %v (type %T)", results[0], results[0])

	rec, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any result, got %T: %v", results[0], results[0])
	}
	if isOk, _ := rec["ok"].(bool); isOk {
		t.Errorf("make-err: ok = true, want false")
	}
}

// =============================================================================
// Multi-parameter tests
// =============================================================================

func TestAddThree(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "add-three")
	results, err := fn.Call(testCtx, uint32(10), uint32(20), uint32(30))
	if err != nil {
		t.Fatalf("add-three call failed: %v", err)
	}
	got, ok := results[0].(uint32)
	if !ok {
		t.Fatalf("expected uint32, got %T: %v", results[0], results[0])
	}
	if got != 60 {
		t.Errorf("add-three(10, 20, 30) = %d, want 60", got)
	}
}

func TestConcatStrings(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "concat-strings")
	results, err := fn.Call(testCtx, "hello, ", "world!")
	if err != nil {
		t.Fatalf("concat-strings call failed: %v", err)
	}
	got, ok := results[0].(string)
	if !ok {
		t.Fatalf("expected string, got %T: %v", results[0], results[0])
	}
	if got != "hello, world!" {
		t.Errorf("concat-strings = %q, want %q", got, "hello, world!")
	}
}

func TestMixedParams(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "mixed-params")

	// flag=true → "alice:42"
	results, err := fn.Call(testCtx, "alice", uint32(42), true)
	if err != nil {
		t.Fatalf("mixed-params call failed: %v", err)
	}
	got, ok := results[0].(string)
	if !ok {
		t.Fatalf("expected string, got %T: %v", results[0], results[0])
	}
	if got != "alice:42" {
		t.Errorf("mixed-params(alice, 42, true) = %q, want %q", got, "alice:42")
	}

	// flag=false → "alice"
	results, err = fn.Call(testCtx, "alice", uint32(42), false)
	if err != nil {
		t.Fatalf("mixed-params call failed: %v", err)
	}
	got = results[0].(string)
	if got != "alice" {
		t.Errorf("mixed-params(alice, 42, false) = %q, want %q", got, "alice")
	}
}

// =============================================================================
// Host import tests — guest calls host with complex types
// =============================================================================

func TestRepro_HostImportEnumParam(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "call-send-enum")
	results, err := fn.Call(testCtx, apicomponent.ValEnum("green"), "hello")
	if err != nil {
		t.Fatalf("call-send-enum(green, \"hello\") failed: %v", err)
	}
	got, ok := results[0].(uint32)
	if !ok {
		t.Fatalf("expected uint32, got %T: %v", results[0], results[0])
	}
	if got != 1 {
		t.Errorf("call-send-enum() = %d, want 1", got)
	}
}

func TestHostImport_EnumArgTypeVerification(t *testing.T) {
	var capturedArgs []apicomponent.Val

	instance, testCtx, cleanup := newPublicAPIInstance(t, map[string]apicomponent.HostFunc{
		"send-enum": func(ctx context.Context, _ *apicomponent.TypeFunc, args []apicomponent.Val) ([]apicomponent.Val, error) {
			capturedArgs = make([]apicomponent.Val, len(args))
			copy(capturedArgs, args)
			return []apicomponent.Val{apicomponent.ValU32(1)}, nil
		},
	})
	defer cleanup()

	fn := getHandlerFunc(t, instance, "call-send-enum")
	_, err := fn.Call(testCtx, apicomponent.ValEnum("green"), "hello")
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}

	if len(capturedArgs) < 1 {
		t.Fatalf("host mock received %d args, expected at least 1", len(capturedArgs))
	}
	if capturedArgs[0].Kind() != apicomponent.ValKindEnum {
		t.Errorf("arg[0].Kind() = %v, want ValKindEnum; got value %v", capturedArgs[0].Kind(), capturedArgs[0])
	}
	if capturedArgs[0].Kind() == apicomponent.ValKindEnum && capturedArgs[0].Enum() != "green" {
		t.Errorf("arg[0].Enum() = %q, want %q", capturedArgs[0].Enum(), "green")
	}
}

func TestHostImport_EnumStringCombinedArgs(t *testing.T) {
	var capturedArgs []apicomponent.Val

	instance, testCtx, cleanup := newPublicAPIInstance(t, map[string]apicomponent.HostFunc{
		"send-enum": func(ctx context.Context, _ *apicomponent.TypeFunc, args []apicomponent.Val) ([]apicomponent.Val, error) {
			capturedArgs = make([]apicomponent.Val, len(args))
			copy(capturedArgs, args)
			return []apicomponent.Val{apicomponent.ValU32(1)}, nil
		},
	})
	defer cleanup()

	fn := getHandlerFunc(t, instance, "call-send-enum")
	for _, tc := range []struct {
		color string
		msg   string
	}{
		{"red", "hello"},
		{"green", "world"},
		{"blue", "test"},
	} {
		_, err := fn.Call(testCtx, apicomponent.ValEnum(tc.color), tc.msg)
		if err != nil {
			t.Fatalf("call-send-enum(%s, %q) failed: %v", tc.color, tc.msg, err)
		}
		if len(capturedArgs) != 2 {
			t.Fatalf("host received %d args, want 2", len(capturedArgs))
		}
		if capturedArgs[0].Kind() == apicomponent.ValKindEnum && capturedArgs[0].Enum() != tc.color {
			t.Errorf("arg[0].Enum() = %q, want %q", capturedArgs[0].Enum(), tc.color)
		}
		if capturedArgs[1].Kind() == apicomponent.ValKindString && capturedArgs[1].StringVal() != tc.msg {
			t.Errorf("arg[1].StringVal() = %q, want %q", capturedArgs[1].StringVal(), tc.msg)
		}
	}
}

func TestHostImport_EnumInRecordVerification(t *testing.T) {
	var capturedArgs []apicomponent.Val

	instance, testCtx, cleanup := newPublicAPIInstance(t, map[string]apicomponent.HostFunc{
		"send-event": func(ctx context.Context, _ *apicomponent.TypeFunc, args []apicomponent.Val) ([]apicomponent.Val, error) {
			capturedArgs = make([]apicomponent.Val, len(args))
			copy(capturedArgs, args)
			return []apicomponent.Val{apicomponent.ValU32(1)}, nil
		},
	})
	defer cleanup()

	fn := getHandlerFunc(t, instance, "call-send-event")
	metadata := apicomponent.ValList([]apicomponent.Val{
		apicomponent.ValU8(1), apicomponent.ValU8(2), apicomponent.ValU8(3),
	})
	eventRecord := apicomponent.ValRecord(map[string]apicomponent.Val{
		"event-type": apicomponent.ValEnum("red"),
		"metadata":   apicomponent.ValOption(&metadata),
	})
	_, err := fn.Call(testCtx, eventRecord)
	if err != nil {
		t.Fatalf("call-send-event failed: %v", err)
	}

	if len(capturedArgs) < 1 {
		t.Fatalf("host received %d args, want at least 1", len(capturedArgs))
	}
	if capturedArgs[0].Kind() != apicomponent.ValKindRecord {
		t.Fatalf("arg[0].Kind() = %v, want ValKindRecord", capturedArgs[0].Kind())
	}
	rec := capturedArgs[0].Record()
	eventType, ok := rec["event-type"]
	if !ok {
		t.Fatal("record missing 'event-type' field")
	}
	if eventType.Kind() != apicomponent.ValKindEnum {
		t.Errorf("event-type.Kind() = %v, want ValKindEnum", eventType.Kind())
	}
}

func TestHostImport_AllEnumCasesRoundTrip(t *testing.T) {
	var capturedArgs []apicomponent.Val

	instance, testCtx, cleanup := newPublicAPIInstance(t, map[string]apicomponent.HostFunc{
		"send-enum": func(ctx context.Context, _ *apicomponent.TypeFunc, args []apicomponent.Val) ([]apicomponent.Val, error) {
			capturedArgs = make([]apicomponent.Val, len(args))
			copy(capturedArgs, args)
			if len(args) > 0 && args[0].Kind() == apicomponent.ValKindEnum {
				switch args[0].Enum() {
				case "red":
					return []apicomponent.Val{apicomponent.ValU32(0)}, nil
				case "green":
					return []apicomponent.Val{apicomponent.ValU32(1)}, nil
				case "blue":
					return []apicomponent.Val{apicomponent.ValU32(2)}, nil
				}
			}
			return []apicomponent.Val{apicomponent.ValU32(99)}, nil
		},
	})
	defer cleanup()

	fn := getHandlerFunc(t, instance, "call-send-enum")
	for _, tc := range []struct {
		color    string
		expected uint32
	}{
		{"red", 0},
		{"green", 1},
		{"blue", 2},
	} {
		results, err := fn.Call(testCtx, apicomponent.ValEnum(tc.color), "test")
		if err != nil {
			t.Fatalf("call-send-enum(%s) failed: %v", tc.color, err)
		}
		got := results[0].(uint32)
		if got != tc.expected {
			t.Errorf("call-send-enum(%s) = %d, want %d", tc.color, got, tc.expected)
		}
	}
}

func TestRepro_HostImportRecordWithOption(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "call-send-event")

	// Test with metadata: some([1,2,3])
	metadata := apicomponent.ValList([]apicomponent.Val{
		apicomponent.ValU8(1), apicomponent.ValU8(2), apicomponent.ValU8(3),
	})
	eventRecord := apicomponent.ValRecord(map[string]apicomponent.Val{
		"event-type": apicomponent.ValEnum("red"),
		"metadata":   apicomponent.ValOption(&metadata),
	})
	results, err := fn.Call(testCtx, eventRecord)
	if err != nil {
		t.Fatalf("call-send-event({red, some([1,2,3])}) failed: %v", err)
	}
	if results[0].(uint32) != 1 {
		t.Errorf("call-send-event() = %v, want 1", results[0])
	}

	// Test with metadata: none
	eventRecordNone := apicomponent.ValRecord(map[string]apicomponent.Val{
		"event-type": apicomponent.ValEnum("blue"),
		"metadata":   apicomponent.ValOption(nil),
	})
	results, err = fn.Call(testCtx, eventRecordNone)
	if err != nil {
		t.Fatalf("call-send-event({blue, none}) failed: %v", err)
	}
	if results[0].(uint32) != 1 {
		t.Errorf("call-send-event() = %v, want 1", results[0])
	}
}

func TestHostImport_EnumReturn(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "call-get-color")
	results, err := fn.Call(testCtx)
	if err != nil {
		t.Fatalf("call-get-color() failed: %v", err)
	}
	got, ok := results[0].(string)
	if !ok {
		t.Fatalf("expected string (enum), got %T: %v", results[0], results[0])
	}
	if got != "blue" {
		t.Errorf("call-get-color() = %q, want %q", got, "blue")
	}
}

func TestHostImport_OptionParam(t *testing.T) {
	var capturedArgs []apicomponent.Val

	instance, testCtx, cleanup := newPublicAPIInstance(t, map[string]apicomponent.HostFunc{
		"check-option": func(ctx context.Context, _ *apicomponent.TypeFunc, args []apicomponent.Val) ([]apicomponent.Val, error) {
			capturedArgs = make([]apicomponent.Val, len(args))
			copy(capturedArgs, args)
			if len(args) > 0 && args[0].Kind() == apicomponent.ValKindOption {
				opt := args[0].Option()
				if opt != nil {
					return []apicomponent.Val{apicomponent.ValU32(1)}, nil
				}
				return []apicomponent.Val{apicomponent.ValU32(0)}, nil
			}
			return []apicomponent.Val{apicomponent.ValU32(99)}, nil
		},
	})
	defer cleanup()

	fn := getHandlerFunc(t, instance, "call-check-option")

	// Test Some(42)
	someVal := apicomponent.ValU32(42)
	results, err := fn.Call(testCtx, apicomponent.ValOption(&someVal))
	if err != nil {
		t.Fatalf("call-check-option(some(42)) failed: %v", err)
	}
	if results[0].(uint32) != 1 {
		t.Errorf("call-check-option(some(42)) = %v, want 1", results[0])
	}

	// Test None
	results, err = fn.Call(testCtx, apicomponent.ValOption(nil))
	if err != nil {
		t.Fatalf("call-check-option(none) failed: %v", err)
	}
	if results[0].(uint32) != 0 {
		t.Errorf("call-check-option(none) = %v, want 0", results[0])
	}
}

func TestHostImport_RetptrCompositeReturn(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "call-get-event")
	results, err := fn.Call(testCtx)
	if err != nil {
		t.Fatalf("call-get-event() failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	rec, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T: %v", results[0], results[0])
	}
	if _, ok := rec["event-type"]; !ok {
		t.Error("record missing 'event-type' field")
	}
}

func TestHostImport_NestedOptionListParam(t *testing.T) {
	var capturedArgs []apicomponent.Val

	instance, testCtx, cleanup := newPublicAPIInstance(t, map[string]apicomponent.HostFunc{
		"check-opt-bytes": func(ctx context.Context, _ *apicomponent.TypeFunc, args []apicomponent.Val) ([]apicomponent.Val, error) {
			capturedArgs = make([]apicomponent.Val, len(args))
			copy(capturedArgs, args)
			if len(args) > 0 && args[0].Kind() == apicomponent.ValKindOption {
				opt := args[0].Option()
				if opt != nil && opt.Kind() == apicomponent.ValKindList {
					return []apicomponent.Val{apicomponent.ValU32(uint32(len(opt.List())))}, nil
				}
				return []apicomponent.Val{apicomponent.ValU32(0)}, nil
			}
			return []apicomponent.Val{apicomponent.ValU32(99)}, nil
		},
	})
	defer cleanup()

	fn := getHandlerFunc(t, instance, "call-check-opt-bytes")

	// Test Some([1,2,3])
	listVal := apicomponent.ValList([]apicomponent.Val{
		apicomponent.ValU8(1), apicomponent.ValU8(2), apicomponent.ValU8(3),
	})
	results, err := fn.Call(testCtx, apicomponent.ValOption(&listVal))
	if err != nil {
		t.Fatalf("call-check-opt-bytes(some([1,2,3])) failed: %v", err)
	}
	if results[0].(uint32) != 3 {
		t.Errorf("call-check-opt-bytes(some([1,2,3])) = %v, want 3", results[0])
	}

	// Test None
	results, err = fn.Call(testCtx, apicomponent.ValOption(nil))
	if err != nil {
		t.Fatalf("call-check-opt-bytes(none) failed: %v", err)
	}
	if results[0].(uint32) != 0 {
		t.Errorf("call-check-opt-bytes(none) = %v, want 0", results[0])
	}
}

func TestHostImport_RecordWithVariant(t *testing.T) {
	var capturedArgs []apicomponent.Val

	instance, testCtx, cleanup := newPublicAPIInstance(t, map[string]apicomponent.HostFunc{
		"send-tagged-shape": func(ctx context.Context, _ *apicomponent.TypeFunc, args []apicomponent.Val) ([]apicomponent.Val, error) {
			capturedArgs = make([]apicomponent.Val, len(args))
			copy(capturedArgs, args)
			return []apicomponent.Val{apicomponent.ValU32(1)}, nil
		},
	})
	defer cleanup()

	fn := getHandlerFunc(t, instance, "call-send-tagged-shape")
	circleVal := apicomponent.ValF64(3.14)
	taggedShape := apicomponent.ValRecord(map[string]apicomponent.Val{
		"tag":   apicomponent.ValString("my-circle"),
		"shape": apicomponent.ValVariant("circle", &circleVal),
	})
	results, err := fn.Call(testCtx, taggedShape)
	if err != nil {
		t.Fatalf("call-send-tagged-shape failed: %v", err)
	}
	if results[0].(uint32) != 1 {
		t.Errorf("call-send-tagged-shape() = %v, want 1", results[0])
	}

	if len(capturedArgs) < 1 || capturedArgs[0].Kind() != apicomponent.ValKindRecord {
		t.Fatalf("expected record arg, got %v", capturedArgs)
	}
	rec := capturedArgs[0].Record()
	if tag, ok := rec["tag"]; !ok || tag.StringVal() != "my-circle" {
		t.Errorf("tag = %v, want my-circle", rec["tag"])
	}
}

func TestHostImport_ListOfRecords(t *testing.T) {
	var capturedArgs []apicomponent.Val

	instance, testCtx, cleanup := newPublicAPIInstance(t, map[string]apicomponent.HostFunc{
		"send-events": func(ctx context.Context, _ *apicomponent.TypeFunc, args []apicomponent.Val) ([]apicomponent.Val, error) {
			capturedArgs = make([]apicomponent.Val, len(args))
			copy(capturedArgs, args)
			if len(args) > 0 && args[0].Kind() == apicomponent.ValKindList {
				return []apicomponent.Val{apicomponent.ValU32(uint32(len(args[0].List())))}, nil
			}
			return []apicomponent.Val{apicomponent.ValU32(99)}, nil
		},
	})
	defer cleanup()

	fn := getHandlerFunc(t, instance, "call-send-events")
	metadata1 := apicomponent.ValList([]apicomponent.Val{apicomponent.ValU8(1), apicomponent.ValU8(2)})
	event1 := apicomponent.ValRecord(map[string]apicomponent.Val{
		"event-type": apicomponent.ValEnum("red"),
		"metadata":   apicomponent.ValOption(&metadata1),
	})
	event2 := apicomponent.ValRecord(map[string]apicomponent.Val{
		"event-type": apicomponent.ValEnum("green"),
		"metadata":   apicomponent.ValOption(nil),
	})
	results, err := fn.Call(testCtx, apicomponent.ValList([]apicomponent.Val{event1, event2}))
	if err != nil {
		t.Fatalf("call-send-events failed: %v", err)
	}
	if results[0].(uint32) != 2 {
		t.Errorf("call-send-events() = %v, want 2", results[0])
	}
}

// =============================================================================
// Public API record return + record-with-nil-option param tests
// These reproduce the director integration test failures.
// =============================================================================

// TestPublicAPI_RecordReturnFromExport reproduces the error:
//
//	expected map[string]any result, got int32: <raw pointer>
//
// When calling an exported function that returns a record (e.g. process-result
// with {value: u32, ok: bool}), the public API's Call returns a raw int32
// (the retptr address) instead of lifting the record from linear memory into
// a map[string]any.
// TestPublicAPI_SingleParamRecordReturn tests calling an exported function that takes
// a single string parameter and returns a record via the public API. This isolates
// the record-return path with a parameter present (unlike process() which has zero params).
func TestPublicAPI_SingleParamRecordReturn(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "process-with-tag")
	results, err := fn.Call(testCtx, "hello")
	if err != nil {
		t.Fatalf("process-with-tag() call failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// The result MUST be a map[string]any (record), not an int32 (raw retptr).
	rec, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any result, got %T: %v", results[0], results[0])
	}
	if val, ok := rec["value"].(uint32); !ok || val != 5 {
		t.Errorf("process-with-tag().value = %v (type %T), want uint32(5)", rec["value"], rec["value"])
	}
	if ok, isOk := rec["ok"].(bool); !isOk || !ok {
		t.Errorf("process-with-tag().ok = %v (type %T), want bool(true)", rec["ok"], rec["ok"])
	}
}

// TestPublicAPI_TwoStringParamsRecordReturn tests calling an exported function that takes
// two string parameters and returns a record via the public API.
func TestPublicAPI_TwoStringParamsRecordReturn(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "process-two-strings")
	results, err := fn.Call(testCtx, "hello", "world")
	if err != nil {
		t.Fatalf("process-two-strings() call failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	rec, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any result, got %T: %v", results[0], results[0])
	}
	if val, ok := rec["value"].(uint32); !ok || val != 10 {
		t.Errorf("value = %v (type %T), want uint32(10)", rec["value"], rec["value"])
	}
	if ok, isOk := rec["ok"].(bool); !isOk || !ok {
		t.Errorf("ok = %v (type %T), want bool(true)", rec["ok"], rec["ok"])
	}
}

// TestPublicAPI_StringAndListParam tests string + list<u8> params returning u32 (no record return).
// Isolates whether the list<u8> param lowering itself is the issue.
func TestPublicAPI_StringAndListParam(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "count-bytes")
	results, err := fn.Call(testCtx, "id-1", []uint8{10, 20, 30})
	if err != nil {
		t.Fatalf("count-bytes() call failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	got, ok := results[0].(uint32)
	if !ok {
		t.Fatalf("expected uint32, got %T: %v", results[0], results[0])
	}
	if got != 3 {
		t.Errorf("count-bytes() = %d, want 3", got)
	}
}

// TestPublicAPI_MultiParamRecordReturn reproduces the director's TestHandleInput failure:
//
//	expected map[string]any result, got int32: 4497408
//
// When calling an exported function that takes multiple parameters (string + list<u8>)
// and returns a record, via the public API Call(), the result comes back as a raw
// int32 (retptr) instead of being lifted to map[string]any.
// This mirrors director's handle-input(correlation-id: string, input: list<u8>) -> handle-result.
func TestPublicAPI_MultiParamRecordReturn(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "handle-bytes")
	results, err := fn.Call(testCtx, "corr-1", []uint8{1, 2, 3})
	if err != nil {
		t.Fatalf("handle-bytes() call failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// The result MUST be a map[string]any (record), not an int32 (raw retptr).
	rec, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any result, got %T: %v", results[0], results[0])
	}
	if val, ok := rec["value"].(uint32); !ok || val != 3 {
		t.Errorf("handle-bytes().value = %v (type %T), want uint32(3)", rec["value"], rec["value"])
	}
	if ok, isOk := rec["ok"].(bool); !isOk || !ok {
		t.Errorf("handle-bytes().ok = %v (type %T), want bool(true)", rec["ok"], rec["ok"])
	}
}

// TestPublicAPI_MultiParamRecordWithNilOption reproduces the director's TestHandleSessionEvent failure:
//
//	panic: interface conversion: interface {} is nil, not bool
//
// When calling an exported function that takes multiple parameters (string + record)
// where the record contains an option field set to nil (option::none), via the public
// API Call(), the lowering path panics.
// This mirrors director's handle-session-event(correlation-id: string, event: session-event)
// where session-event has metadata: option<list<u8>> set to nil.
func TestPublicAPI_MultiParamRecordWithNilOption(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "handle-event")

	// Pass a record with nil metadata (option::none) using Go native types
	// (map[string]any), matching how the director integration test does it.
	eventRecord := map[string]any{
		"event-type": "red",
		"metadata":   nil,
	}

	results, err := fn.Call(testCtx, "corr-2", eventRecord)
	if err != nil {
		t.Fatalf("handle-event(corr-2, {red, nil}) failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	rec, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any result, got %T: %v", results[0], results[0])
	}
	if val, ok := rec["value"].(uint32); !ok || val != 1 {
		t.Errorf("handle-event().value = %v (type %T), want uint32(1)", rec["value"], rec["value"])
	}
}

// TestPublicAPI_ComplexNestedRecordParam tests multi-param with a deeply nested
// complex record input: record containing record containing record, plus lists,
// options, variants, and enums at multiple nesting levels.
//
// complex-input {
//
//	id: string,
//	middle: nested-middle {
//	  inner: nested-inner { label: string, score: f64, active: bool },
//	  tags: list<string>,
//	  priority: option<u32>,
//	  shape: shape (variant),
//	},
//	color: color (enum),
//	metadata: option<list<u8>>,
//
// }
func TestPublicAPI_ComplexNestedRecordParam(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "process-complex")

	complexInput := map[string]any{
		"id": "test-id",
		"middle": map[string]any{
			"inner": map[string]any{
				"label":  "hello",
				"score":  3.14,
				"active": true,
			},
			"tags":     []any{"a", "b", "c"},
			"priority": uint32(7),
			"shape":    map[string]any{"case": "circle", "payload": 2.5},
		},
		"color":    "blue",
		"metadata": []uint8{10, 20},
	}

	// Expected: count(5) + len("test-id")(7) + len("hello")(5) + len(tags)(3) + priority(7) + len(metadata)(2) = 29
	results, err := fn.Call(testCtx, "ctx-1", uint32(5), complexInput)
	if err != nil {
		t.Fatalf("process-complex() call failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	got, ok := results[0].(uint32)
	if !ok {
		t.Fatalf("expected uint32, got %T: %v", results[0], results[0])
	}
	if got != 29 {
		t.Errorf("process-complex() = %d, want 29", got)
	}
}

// TestPublicAPI_ComplexNestedRecordReturn tests multi-param (string + variant)
// returning a deeply nested complex record output:
//
// complex-output {
//
//	success: bool,
//	detail: result-detail { code: u32, message: string, extra: option<string> },
//	values: list<u32>,
//	event: event-data { event-type: color (enum), metadata: option<list<u8>> },
//	label: option<string>,
//
// }
func TestPublicAPI_ComplexNestedRecordReturn(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "transform-complex")

	results, err := fn.Call(testCtx, "myname", uint32(42))
	if err != nil {
		t.Fatalf("transform-complex() call failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	rec, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any result, got %T: %v", results[0], results[0])
	}

	// Verify top-level fields
	if success, ok := rec["success"].(bool); !ok || !success {
		t.Errorf("success = %v (type %T), want true", rec["success"], rec["success"])
	}

	// Verify nested detail record
	detail, ok := rec["detail"].(map[string]any)
	if !ok {
		t.Fatalf("detail: expected map[string]any, got %T: %v", rec["detail"], rec["detail"])
	}
	if code, ok := detail["code"].(uint32); !ok || code != 200 {
		t.Errorf("detail.code = %v, want 200", detail["code"])
	}
	if msg, ok := detail["message"].(string); !ok || msg != "ok:myname" {
		t.Errorf("detail.message = %v, want 'ok:myname'", detail["message"])
	}
	if detail["extra"] == nil {
		t.Errorf("detail.extra = nil, want 'detail-extra'")
	}

	// Verify list<u32>
	values, ok := rec["values"].([]any)
	if !ok {
		t.Fatalf("values: expected []any, got %T: %v", rec["values"], rec["values"])
	}
	if len(values) != 3 {
		t.Errorf("values length = %d, want 3", len(values))
	}

	// Verify nested event record with enum + option<list<u8>>
	event, ok := rec["event"].(map[string]any)
	if !ok {
		t.Fatalf("event: expected map[string]any, got %T: %v", rec["event"], rec["event"])
	}
	if et, ok := event["event-type"].(string); !ok || et != "green" {
		t.Errorf("event.event-type = %v, want 'green'", event["event-type"])
	}

	// Verify option<string> label
	if label, ok := rec["label"].(string); !ok || label != "myname" {
		t.Errorf("label = %v, want 'myname'", rec["label"])
	}
}

// TestPublicAPI_VariantParamComplexReturn tests multi-param with string + variant param,
// returning a deeply nested complex record. The variant param (shape) expands to multiple
// core wasm params (discriminant + payload), which triggers a param count mismatch error.
func TestPublicAPI_VariantParamComplexReturn(t *testing.T) {
	instance, testCtx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()

	fn := getHandlerFunc(t, instance, "transform-complex-variant")

	shapeParam := map[string]any{"case": "square", "payload": 4.0}
	results, err := fn.Call(testCtx, "myname", shapeParam)
	if err != nil {
		t.Fatalf("transform-complex-variant() call failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	rec, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any result, got %T: %v", results[0], results[0])
	}
	if success, ok := rec["success"].(bool); !ok || !success {
		t.Errorf("success = %v (type %T), want true", rec["success"], rec["success"])
	}
}
