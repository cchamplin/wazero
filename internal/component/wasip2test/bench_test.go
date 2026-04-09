// internal/component/wasip2test/bench_test.go
//
// Benchmark and property tests for the component model public API.
// Exercises all major type paths (primitives, strings, lists, records,
// enums, variants, options, nested records) with randomized data,
// verifying correctness and measuring throughput.
package wasip2test

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"testing"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	apicomponent "github.com/tetratelabs/wazero/api/component"
	"github.com/tetratelabs/wazero/imports/wasip2"
)

// newBenchInstance creates a component instance for benchmarks, panicking on error.
func newBenchInstance(b *testing.B) (api.Component, context.Context, func()) {
	b.Helper()

	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)

	wasmBytes, err := os.ReadFile("go-repro-plugin/component.wasm")
	if err != nil {
		b.Fatalf("ReadFile: %v", err)
	}

	compiled, err := rt.CompileComponent(ctx, wasmBytes)
	if err != nil {
		b.Fatalf("CompileComponent: %v", err)
	}

	linker := rt.NewComponentLinker()
	linker.SetRelaxedSemverMatching(true)

	if err := wasip2.MergeInto(linker); err != nil {
		b.Fatalf("wasip2.MergeInto: %v", err)
	}

	_ = linker.DefineInstance("test:repro/types").SkipValidation().Build()

	hostOps := defaultHostOps()
	builder := linker.DefineInstance("test:repro/host-ops").SkipValidation()
	for name, fn := range hostOps {
		builder = builder.Func(name, fn)
	}
	_ = builder.Build()

	_ = linker.DefineInstance("test:repro/host-rng").SkipValidation().
		Func("get-random-bytes", apicomponent.HostFunc(func(ctx context.Context, _ *apicomponent.TypeFunc, args []apicomponent.Val) ([]apicomponent.Val, error) {
			return []apicomponent.Val{apicomponent.ValList(nil)}, nil
		})).
		Build()

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
		b.Fatalf("Instantiate: %v", err)
	}

	cleanup := func() {
		instance.Close(ctx)
		compiled.Close(ctx)
		rt.Close(ctx)
	}

	return instance, testCtx, cleanup
}

// randString returns a random string of length n.
func randString(rng *rand.Rand, n int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 !@#$%"
	b := make([]byte, n)
	for i := range b {
		b[i] = charset[rng.IntN(len(charset))]
	}
	return string(b)
}

// randBytes returns a random byte slice of length n.
func randBytes(rng *rand.Rand, n int) []uint8 {
	b := make([]uint8, n)
	for i := range b {
		b[i] = uint8(rng.IntN(256))
	}
	return b
}

// randColor returns a random color enum string.
func randColor(rng *rand.Rand) string {
	colors := []string{"red", "green", "blue"}
	return colors[rng.IntN(len(colors))]
}

// randShape returns a random shape variant as map[string]any.
func randShape(rng *rand.Rand) map[string]any {
	switch rng.IntN(3) {
	case 0:
		return map[string]any{"case": "circle", "payload": rng.Float64() * 100}
	case 1:
		return map[string]any{"case": "square", "payload": rng.Float64() * 100}
	default:
		return map[string]any{"case": "none"}
	}
}

// randEventData returns a random event-data record as map[string]any.
func randEventData(rng *rand.Rand) map[string]any {
	m := map[string]any{
		"event-type": randColor(rng),
	}
	if rng.IntN(2) == 0 {
		m["metadata"] = randBytes(rng, rng.IntN(32))
	} else {
		m["metadata"] = nil
	}
	return m
}

// randComplexInput returns a random complex-input record as map[string]any.
func randComplexInput(rng *rand.Rand) map[string]any {
	nTags := rng.IntN(5)
	tags := make([]any, nTags)
	for i := range tags {
		tags[i] = randString(rng, rng.IntN(10)+1)
	}

	inner := map[string]any{
		"label":  randString(rng, rng.IntN(20)+1),
		"score":  rng.Float64() * 1000,
		"active": rng.IntN(2) == 0,
	}

	middle := map[string]any{
		"inner": inner,
		"tags":  tags,
		"shape": randShape(rng),
	}
	if rng.IntN(2) == 0 {
		middle["priority"] = uint32(rng.IntN(100))
	} else {
		middle["priority"] = nil
	}

	ci := map[string]any{
		"id":     randString(rng, rng.IntN(15)+1),
		"middle": middle,
		"color":  randColor(rng),
	}
	if rng.IntN(2) == 0 {
		ci["metadata"] = randBytes(rng, rng.IntN(64))
	} else {
		ci["metadata"] = nil
	}
	return ci
}

// expectedProcessComplex computes the expected return value from process-complex.
func expectedProcessComplex(count uint32, input map[string]any) uint32 {
	result := count
	result += uint32(len(input["id"].(string)))
	middle := input["middle"].(map[string]any)
	inner := middle["inner"].(map[string]any)
	result += uint32(len(inner["label"].(string)))
	tags := middle["tags"].([]any)
	result += uint32(len(tags))
	if p := middle["priority"]; p != nil {
		result += p.(uint32)
	}
	if m := input["metadata"]; m != nil {
		result += uint32(len(m.([]uint8)))
	}
	return result
}

// =============================================================================
// Property tests: randomized correctness verification
// =============================================================================

func TestProperty_EchoString(t *testing.T) {
	instance, ctx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()
	fn := getHandlerFunc(t, instance, "echo-string")
	rng := rand.New(rand.NewPCG(42, 99))

	for i := 0; i < 1000; i++ {
		input := randString(rng, rng.IntN(200))
		results, err := fn.Call(ctx, input)
		if err != nil {
			t.Fatalf("iter %d: echo-string(%q) failed: %v", i, input, err)
		}
		got, ok := results[0].(string)
		if !ok {
			t.Fatalf("iter %d: expected string, got %T", i, results[0])
		}
		if got != input {
			t.Fatalf("iter %d: echo-string mismatch: got %q, want %q", i, got, input)
		}
	}
}

func TestProperty_EchoPrimitives(t *testing.T) {
	instance, ctx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()
	rng := rand.New(rand.NewPCG(42, 99))

	fnBool := getHandlerFunc(t, instance, "echo-bool")
	fnS8 := getHandlerFunc(t, instance, "echo-s8")
	fnU8 := getHandlerFunc(t, instance, "echo-u8")
	fnS16 := getHandlerFunc(t, instance, "echo-s16")
	fnU16 := getHandlerFunc(t, instance, "echo-u16")
	fnF32 := getHandlerFunc(t, instance, "echo-f32")
	fnF64 := getHandlerFunc(t, instance, "echo-f64")

	for i := 0; i < 500; i++ {
		// bool
		bv := rng.IntN(2) == 0
		if r, err := fnBool.Call(ctx, bv); err != nil {
			t.Fatalf("iter %d: echo-bool: %v", i, err)
		} else if r[0].(bool) != bv {
			t.Fatalf("iter %d: echo-bool mismatch", i)
		}

		// s8
		s8v := int8(rng.IntN(256) - 128)
		if r, err := fnS8.Call(ctx, s8v); err != nil {
			t.Fatalf("iter %d: echo-s8: %v", i, err)
		} else if r[0].(int8) != s8v {
			t.Fatalf("iter %d: echo-s8 mismatch: got %d want %d", i, r[0], s8v)
		}

		// u8
		u8v := uint8(rng.IntN(256))
		if r, err := fnU8.Call(ctx, u8v); err != nil {
			t.Fatalf("iter %d: echo-u8: %v", i, err)
		} else if r[0].(uint8) != u8v {
			t.Fatalf("iter %d: echo-u8 mismatch", i)
		}

		// s16
		s16v := int16(rng.IntN(65536) - 32768)
		if r, err := fnS16.Call(ctx, s16v); err != nil {
			t.Fatalf("iter %d: echo-s16: %v", i, err)
		} else if r[0].(int16) != s16v {
			t.Fatalf("iter %d: echo-s16 mismatch", i)
		}

		// u16
		u16v := uint16(rng.IntN(65536))
		if r, err := fnU16.Call(ctx, u16v); err != nil {
			t.Fatalf("iter %d: echo-u16: %v", i, err)
		} else if r[0].(uint16) != u16v {
			t.Fatalf("iter %d: echo-u16 mismatch", i)
		}

		// f32
		f32v := float32(rng.Float64()*2000 - 1000)
		if r, err := fnF32.Call(ctx, f32v); err != nil {
			t.Fatalf("iter %d: echo-f32: %v", i, err)
		} else if math.Float32bits(r[0].(float32)) != math.Float32bits(f32v) {
			t.Fatalf("iter %d: echo-f32 mismatch: got %v want %v", i, r[0], f32v)
		}

		// f64
		f64v := rng.Float64()*2000 - 1000
		if r, err := fnF64.Call(ctx, f64v); err != nil {
			t.Fatalf("iter %d: echo-f64: %v", i, err)
		} else if math.Float64bits(r[0].(float64)) != math.Float64bits(f64v) {
			t.Fatalf("iter %d: echo-f64 mismatch: got %v want %v", i, r[0], f64v)
		}
	}
}

func TestProperty_AddThree(t *testing.T) {
	instance, ctx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()
	fn := getHandlerFunc(t, instance, "add-three")
	rng := rand.New(rand.NewPCG(42, 99))

	for i := 0; i < 1000; i++ {
		a := uint32(rng.IntN(1000000))
		b := uint32(rng.IntN(1000000))
		c := uint32(rng.IntN(1000000))
		results, err := fn.Call(ctx, a, b, c)
		if err != nil {
			t.Fatalf("iter %d: add-three: %v", i, err)
		}
		got := results[0].(uint32)
		if got != a+b+c {
			t.Fatalf("iter %d: add-three(%d,%d,%d) = %d, want %d", i, a, b, c, got, a+b+c)
		}
	}
}

func TestProperty_ConcatStrings(t *testing.T) {
	instance, ctx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()
	fn := getHandlerFunc(t, instance, "concat-strings")
	rng := rand.New(rand.NewPCG(42, 99))

	for i := 0; i < 1000; i++ {
		a := randString(rng, rng.IntN(50))
		b := randString(rng, rng.IntN(50))
		results, err := fn.Call(ctx, a, b)
		if err != nil {
			t.Fatalf("iter %d: concat-strings: %v", i, err)
		}
		got := results[0].(string)
		if got != a+b {
			t.Fatalf("iter %d: concat-strings mismatch", i)
		}
	}
}

func TestProperty_MixedParams(t *testing.T) {
	instance, ctx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()
	fn := getHandlerFunc(t, instance, "mixed-params")
	rng := rand.New(rand.NewPCG(42, 99))

	for i := 0; i < 1000; i++ {
		name := randString(rng, rng.IntN(20)+1)
		count := uint32(rng.IntN(10000))
		flag := rng.IntN(2) == 0
		results, err := fn.Call(ctx, name, count, flag)
		if err != nil {
			t.Fatalf("iter %d: mixed-params: %v", i, err)
		}
		got := results[0].(string)
		var want string
		if flag {
			want = fmt.Sprintf("%s:%d", name, count)
		} else {
			want = name
		}
		if got != want {
			t.Fatalf("iter %d: mixed-params(%q,%d,%v) = %q, want %q", i, name, count, flag, got, want)
		}
	}
}

func TestProperty_EchoEnum(t *testing.T) {
	instance, ctx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()
	fn := getHandlerFunc(t, instance, "echo-enum")
	rng := rand.New(rand.NewPCG(42, 99))

	for i := 0; i < 1000; i++ {
		color := randColor(rng)
		results, err := fn.Call(ctx, color)
		if err != nil {
			t.Fatalf("iter %d: echo-enum(%s): %v", i, color, err)
		}
		got := results[0].(string)
		if got != color {
			t.Fatalf("iter %d: echo-enum(%s) = %s", i, color, got)
		}
	}
}

func TestProperty_EchoVariant(t *testing.T) {
	instance, ctx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()
	fn := getHandlerFunc(t, instance, "echo-variant")
	rng := rand.New(rand.NewPCG(42, 99))

	for i := 0; i < 1000; i++ {
		shape := randShape(rng)
		results, err := fn.Call(ctx, shape)
		if err != nil {
			t.Fatalf("iter %d: echo-variant(%v): %v", i, shape, err)
		}
		got := results[0].(map[string]any)
		if got["case"] != shape["case"] {
			t.Fatalf("iter %d: echo-variant case mismatch: got %v want %v", i, got["case"], shape["case"])
		}
		if shape["case"] != "none" {
			gotPayload := got["payload"].(float64)
			wantPayload := shape["payload"].(float64)
			if math.Float64bits(gotPayload) != math.Float64bits(wantPayload) {
				t.Fatalf("iter %d: echo-variant payload mismatch: got %v want %v", i, gotPayload, wantPayload)
			}
		}
	}
}

func TestProperty_HandleBytes(t *testing.T) {
	instance, ctx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()
	fn := getHandlerFunc(t, instance, "handle-bytes")
	rng := rand.New(rand.NewPCG(42, 99))

	for i := 0; i < 1000; i++ {
		id := randString(rng, rng.IntN(20)+1)
		data := randBytes(rng, rng.IntN(256))
		results, err := fn.Call(ctx, id, data)
		if err != nil {
			t.Fatalf("iter %d: handle-bytes: %v", i, err)
		}
		rec := results[0].(map[string]any)
		got := rec["value"].(uint32)
		if got != uint32(len(data)) {
			t.Fatalf("iter %d: handle-bytes value = %d, want %d", i, got, len(data))
		}
		if rec["ok"].(bool) != true {
			t.Fatalf("iter %d: handle-bytes ok = false", i)
		}
	}
}

func TestProperty_HandleEvent(t *testing.T) {
	instance, ctx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()
	fn := getHandlerFunc(t, instance, "handle-event")
	rng := rand.New(rand.NewPCG(42, 99))

	for i := 0; i < 1000; i++ {
		id := randString(rng, rng.IntN(20)+1)
		event := randEventData(rng)
		results, err := fn.Call(ctx, id, event)
		if err != nil {
			t.Fatalf("iter %d: handle-event(%v): %v", i, event, err)
		}
		rec := results[0].(map[string]any)
		if rec["value"].(uint32) != 1 {
			t.Fatalf("iter %d: handle-event value mismatch", i)
		}
		if rec["ok"].(bool) != true {
			t.Fatalf("iter %d: handle-event ok = false", i)
		}
	}
}

func TestProperty_ProcessComplex(t *testing.T) {
	instance, ctx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()
	fn := getHandlerFunc(t, instance, "process-complex")
	rng := rand.New(rand.NewPCG(42, 99))

	for i := 0; i < 500; i++ {
		id := randString(rng, rng.IntN(10)+1)
		count := uint32(rng.IntN(100))
		input := randComplexInput(rng)
		want := expectedProcessComplex(count, input)

		results, err := fn.Call(ctx, id, count, input)
		if err != nil {
			t.Fatalf("iter %d: process-complex: %v", i, err)
		}
		got := results[0].(uint32)
		if got != want {
			t.Fatalf("iter %d: process-complex = %d, want %d (count=%d, input=%v)", i, got, want, count, input)
		}
	}
}

func TestProperty_TransformComplex(t *testing.T) {
	instance, ctx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()
	fn := getHandlerFunc(t, instance, "transform-complex")
	rng := rand.New(rand.NewPCG(42, 99))

	for i := 0; i < 500; i++ {
		name := randString(rng, rng.IntN(30)+1)
		code := uint32(rng.IntN(1000))

		results, err := fn.Call(ctx, name, code)
		if err != nil {
			t.Fatalf("iter %d: transform-complex: %v", i, err)
		}
		rec := results[0].(map[string]any)

		// Verify top-level
		if rec["success"].(bool) != true {
			t.Fatalf("iter %d: success = false", i)
		}

		// Verify nested detail
		detail := rec["detail"].(map[string]any)
		if detail["code"].(uint32) != 200 {
			t.Fatalf("iter %d: detail.code = %v", i, detail["code"])
		}
		wantMsg := "ok:" + name
		if detail["message"].(string) != wantMsg {
			t.Fatalf("iter %d: detail.message = %q, want %q", i, detail["message"], wantMsg)
		}

		// Verify list<u32>
		values := rec["values"].([]any)
		if len(values) != 3 {
			t.Fatalf("iter %d: values len = %d", i, len(values))
		}

		// Verify nested event
		event := rec["event"].(map[string]any)
		if event["event-type"].(string) != "green" {
			t.Fatalf("iter %d: event-type = %v", i, event["event-type"])
		}

		// Verify label
		if rec["label"].(string) != name {
			t.Fatalf("iter %d: label = %v, want %v", i, rec["label"], name)
		}
	}
}

func TestProperty_ResultTypes(t *testing.T) {
	instance, ctx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()
	fnOk := getHandlerFunc(t, instance, "make-ok")
	fnErr := getHandlerFunc(t, instance, "make-err")
	rng := rand.New(rand.NewPCG(42, 99))

	for i := 0; i < 500; i++ {
		v := uint32(rng.IntN(1000000))
		results, err := fnOk.Call(ctx, v)
		if err != nil {
			t.Fatalf("iter %d: make-ok: %v", i, err)
		}
		rec := results[0].(map[string]any)
		if rec["ok"].(bool) != true {
			t.Fatalf("iter %d: make-ok: ok = false", i)
		}
		if rec["value"].(uint32) != v {
			t.Fatalf("iter %d: make-ok: value = %v, want %v", i, rec["value"], v)
		}

		msg := randString(rng, rng.IntN(50)+1)
		results, err = fnErr.Call(ctx, msg)
		if err != nil {
			t.Fatalf("iter %d: make-err: %v", i, err)
		}
		rec = results[0].(map[string]any)
		if rec["ok"].(bool) != false {
			t.Fatalf("iter %d: make-err: ok = true", i)
		}
		if rec["error"].(string) != msg {
			t.Fatalf("iter %d: make-err: error = %q, want %q", i, rec["error"], msg)
		}
	}
}

// =============================================================================
// Synthetic throughput test: exercises all paths, reports timing
// =============================================================================

func TestSyntheticThroughput(t *testing.T) {
	instance, ctx, cleanup := newPublicAPIInstance(t, nil)
	defer cleanup()
	rng := rand.New(rand.NewPCG(42, 99))

	type callSpec struct {
		name string
		fn   func() error
	}

	fnEchoString := getHandlerFunc(t, instance, "echo-string")
	fnAddThree := getHandlerFunc(t, instance, "add-three")
	fnMixed := getHandlerFunc(t, instance, "mixed-params")
	fnEchoEnum := getHandlerFunc(t, instance, "echo-enum")
	fnEchoVariant := getHandlerFunc(t, instance, "echo-variant")
	fnHandleBytes := getHandlerFunc(t, instance, "handle-bytes")
	fnHandleEvent := getHandlerFunc(t, instance, "handle-event")
	fnProcessComplex := getHandlerFunc(t, instance, "process-complex")
	fnTransformComplex := getHandlerFunc(t, instance, "transform-complex")
	fnTransformVariant := getHandlerFunc(t, instance, "transform-complex-variant")
	fnMakeOk := getHandlerFunc(t, instance, "make-ok")

	specs := []callSpec{
		{"echo-string", func() error {
			_, err := fnEchoString.Call(ctx, randString(rng, rng.IntN(100)))
			return err
		}},
		{"add-three", func() error {
			_, err := fnAddThree.Call(ctx, uint32(rng.IntN(1000)), uint32(rng.IntN(1000)), uint32(rng.IntN(1000)))
			return err
		}},
		{"mixed-params", func() error {
			_, err := fnMixed.Call(ctx, randString(rng, 10), uint32(rng.IntN(100)), rng.IntN(2) == 0)
			return err
		}},
		{"echo-enum", func() error {
			_, err := fnEchoEnum.Call(ctx, randColor(rng))
			return err
		}},
		{"echo-variant", func() error {
			_, err := fnEchoVariant.Call(ctx, randShape(rng))
			return err
		}},
		{"handle-bytes", func() error {
			_, err := fnHandleBytes.Call(ctx, randString(rng, 5), randBytes(rng, rng.IntN(128)))
			return err
		}},
		{"handle-event", func() error {
			_, err := fnHandleEvent.Call(ctx, randString(rng, 5), randEventData(rng))
			return err
		}},
		{"process-complex", func() error {
			_, err := fnProcessComplex.Call(ctx, randString(rng, 5), uint32(rng.IntN(50)), randComplexInput(rng))
			return err
		}},
		{"transform-complex", func() error {
			_, err := fnTransformComplex.Call(ctx, randString(rng, 10), uint32(rng.IntN(100)))
			return err
		}},
		{"transform-complex-variant", func() error {
			_, err := fnTransformVariant.Call(ctx, randString(rng, 10), randShape(rng))
			return err
		}},
		{"make-ok", func() error {
			_, err := fnMakeOk.Call(ctx, uint32(rng.IntN(10000)))
			return err
		}},
	}

	const totalCalls = 5000
	callsPerSpec := totalCalls / len(specs)

	t.Logf("Running %d total calls (%d per function, %d functions)", callsPerSpec*len(specs), callsPerSpec, len(specs))

	overallStart := time.Now()
	for _, spec := range specs {
		start := time.Now()
		for j := 0; j < callsPerSpec; j++ {
			if err := spec.fn(); err != nil {
				t.Fatalf("%s iter %d: %v", spec.name, j, err)
			}
		}
		elapsed := time.Since(start)
		avg := elapsed / time.Duration(callsPerSpec)
		t.Logf("  %-30s %d calls in %v  (avg %v/call)", spec.name, callsPerSpec, elapsed.Round(time.Millisecond), avg)
	}
	overallElapsed := time.Since(overallStart)
	totalCallCount := callsPerSpec * len(specs)
	avgOverall := overallElapsed / time.Duration(totalCallCount)
	t.Logf("  %-30s %d calls in %v  (avg %v/call)", "TOTAL", totalCallCount, overallElapsed.Round(time.Millisecond), avgOverall)
}

// =============================================================================
// Go benchmarks: use `go test -bench=. -benchtime=5s` for stable results
// =============================================================================

func BenchmarkEchoString(b *testing.B) {
	instance, ctx, cleanup := newBenchInstance(b)
	defer cleanup()
	fn := instance.ExportedInstance("test:repro/handler").ExportedFunction("echo-string")
	b.ResetTimer()
	for b.Loop() {
		if _, err := fn.Call(ctx, "benchmark-string-data"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAddThree(b *testing.B) {
	instance, ctx, cleanup := newBenchInstance(b)
	defer cleanup()
	fn := instance.ExportedInstance("test:repro/handler").ExportedFunction("add-three")
	b.ResetTimer()
	for b.Loop() {
		if _, err := fn.Call(ctx, uint32(1), uint32(2), uint32(3)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEchoEnum(b *testing.B) {
	instance, ctx, cleanup := newBenchInstance(b)
	defer cleanup()
	fn := instance.ExportedInstance("test:repro/handler").ExportedFunction("echo-enum")
	b.ResetTimer()
	for b.Loop() {
		if _, err := fn.Call(ctx, "green"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEchoVariant(b *testing.B) {
	instance, ctx, cleanup := newBenchInstance(b)
	defer cleanup()
	fn := instance.ExportedInstance("test:repro/handler").ExportedFunction("echo-variant")
	shape := map[string]any{"case": "circle", "payload": 3.14}
	b.ResetTimer()
	for b.Loop() {
		if _, err := fn.Call(ctx, shape); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHandleBytes(b *testing.B) {
	instance, ctx, cleanup := newBenchInstance(b)
	defer cleanup()
	fn := instance.ExportedInstance("test:repro/handler").ExportedFunction("handle-bytes")
	data := make([]uint8, 64)
	b.ResetTimer()
	for b.Loop() {
		if _, err := fn.Call(ctx, "corr-id", data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHandleEvent(b *testing.B) {
	instance, ctx, cleanup := newBenchInstance(b)
	defer cleanup()
	fn := instance.ExportedInstance("test:repro/handler").ExportedFunction("handle-event")
	event := map[string]any{"event-type": "red", "metadata": nil}
	b.ResetTimer()
	for b.Loop() {
		if _, err := fn.Call(ctx, "corr-id", event); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProcessComplex(b *testing.B) {
	instance, ctx, cleanup := newBenchInstance(b)
	defer cleanup()
	fn := instance.ExportedInstance("test:repro/handler").ExportedFunction("process-complex")
	input := map[string]any{
		"id": "bench-id",
		"middle": map[string]any{
			"inner": map[string]any{
				"label": "bench", "score": 1.5, "active": true,
			},
			"tags":     []any{"a", "b"},
			"priority": uint32(10),
			"shape":    map[string]any{"case": "circle", "payload": 2.0},
		},
		"color":    "blue",
		"metadata": []uint8{1, 2, 3},
	}
	b.ResetTimer()
	for b.Loop() {
		if _, err := fn.Call(ctx, "ctx", uint32(5), input); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTransformComplex(b *testing.B) {
	instance, ctx, cleanup := newBenchInstance(b)
	defer cleanup()
	fn := instance.ExportedInstance("test:repro/handler").ExportedFunction("transform-complex")
	b.ResetTimer()
	for b.Loop() {
		if _, err := fn.Call(ctx, "bench-name", uint32(42)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTransformComplexVariant(b *testing.B) {
	instance, ctx, cleanup := newBenchInstance(b)
	defer cleanup()
	fn := instance.ExportedInstance("test:repro/handler").ExportedFunction("transform-complex-variant")
	shape := map[string]any{"case": "square", "payload": 9.0}
	b.ResetTimer()
	for b.Loop() {
		if _, err := fn.Call(ctx, "bench-name", shape); err != nil {
			b.Fatal(err)
		}
	}
}
