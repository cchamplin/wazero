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

// randShapeVal returns a random shape variant as apicomponent.Val.
func randShapeVal(rng *rand.Rand) apicomponent.Val {
	switch rng.IntN(3) {
	case 0:
		p := apicomponent.ValF64(rng.Float64() * 100)
		return apicomponent.ValVariant("circle", &p)
	case 1:
		p := apicomponent.ValF64(rng.Float64() * 100)
		return apicomponent.ValVariant("square", &p)
	default:
		return apicomponent.ValVariant("none", nil)
	}
}

// randEventDataVal returns a random event-data record as apicomponent.Val.
func randEventDataVal(rng *rand.Rand) apicomponent.Val {
	fields := map[string]apicomponent.Val{
		"event-type": apicomponent.ValEnum(randColor(rng)),
	}
	if rng.IntN(2) == 0 {
		bs := randBytes(rng, rng.IntN(32))
		elems := make([]apicomponent.Val, len(bs))
		for i, b := range bs {
			elems[i] = apicomponent.ValU8(b)
		}
		list := apicomponent.ValList(elems)
		fields["metadata"] = apicomponent.ValOption(&list)
	} else {
		fields["metadata"] = apicomponent.ValOption(nil)
	}
	return apicomponent.ValRecord(fields)
}

// randComplexInputVal returns a random complex-input record as apicomponent.Val.
func randComplexInputVal(rng *rand.Rand) apicomponent.Val {
	nTags := rng.IntN(5)
	tags := make([]apicomponent.Val, nTags)
	for i := range tags {
		tags[i] = apicomponent.ValString(randString(rng, rng.IntN(10)+1))
	}

	inner := apicomponent.ValRecord(map[string]apicomponent.Val{
		"label":  apicomponent.ValString(randString(rng, rng.IntN(20)+1)),
		"score":  apicomponent.ValF64(rng.Float64() * 1000),
		"active": apicomponent.ValBool(rng.IntN(2) == 0),
	})

	middleFields := map[string]apicomponent.Val{
		"inner": inner,
		"tags":  apicomponent.ValList(tags),
		"shape": randShapeVal(rng),
	}
	if rng.IntN(2) == 0 {
		p := apicomponent.ValU32(uint32(rng.IntN(100)))
		middleFields["priority"] = apicomponent.ValOption(&p)
	} else {
		middleFields["priority"] = apicomponent.ValOption(nil)
	}

	ciFields := map[string]apicomponent.Val{
		"id":     apicomponent.ValString(randString(rng, rng.IntN(15)+1)),
		"middle": apicomponent.ValRecord(middleFields),
		"color":  apicomponent.ValEnum(randColor(rng)),
	}
	if rng.IntN(2) == 0 {
		bs := randBytes(rng, rng.IntN(64))
		elems := make([]apicomponent.Val, len(bs))
		for i, b := range bs {
			elems[i] = apicomponent.ValU8(b)
		}
		list := apicomponent.ValList(elems)
		ciFields["metadata"] = apicomponent.ValOption(&list)
	} else {
		ciFields["metadata"] = apicomponent.ValOption(nil)
	}
	return apicomponent.ValRecord(ciFields)
}

// expectedProcessComplexVal computes the expected return value from process-complex.
func expectedProcessComplexVal(count uint32, input apicomponent.Val) uint32 {
	result := count
	rec := input.Record()
	result += uint32(len(rec["id"].StringVal()))
	middle := rec["middle"].Record()
	inner := middle["inner"].Record()
	result += uint32(len(inner["label"].StringVal()))
	tags := middle["tags"].List()
	result += uint32(len(tags))
	if p := middle["priority"].Option(); p != nil {
		result += p.U32()
	}
	if m := rec["metadata"].Option(); m != nil {
		result += uint32(len(m.List()))
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
		results, err := fn.CallAndPostReturn(ctx, apicomponent.ValString(input))
		if err != nil {
			t.Fatalf("iter %d: echo-string(%q) failed: %v", i, input, err)
		}
		got := results[0].StringVal()
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
		if r, err := fnBool.CallAndPostReturn(ctx, apicomponent.ValBool(bv)); err != nil {
			t.Fatalf("iter %d: echo-bool: %v", i, err)
		} else if r[0].Bool() != bv {
			t.Fatalf("iter %d: echo-bool mismatch", i)
		}

		// s8
		s8v := int8(rng.IntN(256) - 128)
		if r, err := fnS8.CallAndPostReturn(ctx, apicomponent.ValS8(s8v)); err != nil {
			t.Fatalf("iter %d: echo-s8: %v", i, err)
		} else if r[0].S8() != s8v {
			t.Fatalf("iter %d: echo-s8 mismatch: got %d want %d", i, r[0].S8(), s8v)
		}

		// u8
		u8v := uint8(rng.IntN(256))
		if r, err := fnU8.CallAndPostReturn(ctx, apicomponent.ValU8(u8v)); err != nil {
			t.Fatalf("iter %d: echo-u8: %v", i, err)
		} else if r[0].U8() != u8v {
			t.Fatalf("iter %d: echo-u8 mismatch", i)
		}

		// s16
		s16v := int16(rng.IntN(65536) - 32768)
		if r, err := fnS16.CallAndPostReturn(ctx, apicomponent.ValS16(s16v)); err != nil {
			t.Fatalf("iter %d: echo-s16: %v", i, err)
		} else if r[0].S16() != s16v {
			t.Fatalf("iter %d: echo-s16 mismatch", i)
		}

		// u16
		u16v := uint16(rng.IntN(65536))
		if r, err := fnU16.CallAndPostReturn(ctx, apicomponent.ValU16(u16v)); err != nil {
			t.Fatalf("iter %d: echo-u16: %v", i, err)
		} else if r[0].U16() != u16v {
			t.Fatalf("iter %d: echo-u16 mismatch", i)
		}

		// f32
		f32v := float32(rng.Float64()*2000 - 1000)
		if r, err := fnF32.CallAndPostReturn(ctx, apicomponent.ValF32(f32v)); err != nil {
			t.Fatalf("iter %d: echo-f32: %v", i, err)
		} else if math.Float32bits(r[0].F32()) != math.Float32bits(f32v) {
			t.Fatalf("iter %d: echo-f32 mismatch: got %v want %v", i, r[0].F32(), f32v)
		}

		// f64
		f64v := rng.Float64()*2000 - 1000
		if r, err := fnF64.CallAndPostReturn(ctx, apicomponent.ValF64(f64v)); err != nil {
			t.Fatalf("iter %d: echo-f64: %v", i, err)
		} else if math.Float64bits(r[0].F64()) != math.Float64bits(f64v) {
			t.Fatalf("iter %d: echo-f64 mismatch: got %v want %v", i, r[0].F64(), f64v)
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
		results, err := fn.CallAndPostReturn(ctx, apicomponent.ValU32(a), apicomponent.ValU32(b), apicomponent.ValU32(c))
		if err != nil {
			t.Fatalf("iter %d: add-three: %v", i, err)
		}
		got := results[0].U32()
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
		results, err := fn.CallAndPostReturn(ctx, apicomponent.ValString(a), apicomponent.ValString(b))
		if err != nil {
			t.Fatalf("iter %d: concat-strings: %v", i, err)
		}
		got := results[0].StringVal()
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
		results, err := fn.CallAndPostReturn(ctx, apicomponent.ValString(name), apicomponent.ValU32(count), apicomponent.ValBool(flag))
		if err != nil {
			t.Fatalf("iter %d: mixed-params: %v", i, err)
		}
		got := results[0].StringVal()
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
		results, err := fn.CallAndPostReturn(ctx, apicomponent.ValEnum(color))
		if err != nil {
			t.Fatalf("iter %d: echo-enum(%s): %v", i, color, err)
		}
		got := results[0].Enum()
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
		shape := randShapeVal(rng)
		results, err := fn.CallAndPostReturn(ctx, shape)
		if err != nil {
			t.Fatalf("iter %d: echo-variant(%v): %v", i, shape, err)
		}
		gotCase, gotPayload := results[0].Variant()
		wantCase, wantPayload := shape.Variant()
		if gotCase != wantCase {
			t.Fatalf("iter %d: echo-variant case mismatch: got %v want %v", i, gotCase, wantCase)
		}
		if wantCase != "none" {
			if gotPayload == nil || wantPayload == nil {
				t.Fatalf("iter %d: echo-variant payload nil mismatch", i)
			}
			if math.Float64bits(gotPayload.F64()) != math.Float64bits(wantPayload.F64()) {
				t.Fatalf("iter %d: echo-variant payload mismatch: got %v want %v", i, gotPayload.F64(), wantPayload.F64())
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
		elems := make([]apicomponent.Val, len(data))
		for j, b := range data {
			elems[j] = apicomponent.ValU8(b)
		}
		results, err := fn.CallAndPostReturn(ctx, apicomponent.ValString(id), apicomponent.ValList(elems))
		if err != nil {
			t.Fatalf("iter %d: handle-bytes: %v", i, err)
		}
		isOk, okVal, _ := results[0].Result()
		if !isOk {
			t.Fatalf("iter %d: handle-bytes ok = false", i)
		}
		got := okVal.U32()
		if got != uint32(len(data)) {
			t.Fatalf("iter %d: handle-bytes value = %d, want %d", i, got, len(data))
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
		event := randEventDataVal(rng)
		results, err := fn.CallAndPostReturn(ctx, apicomponent.ValString(id), event)
		if err != nil {
			t.Fatalf("iter %d: handle-event(%v): %v", i, event, err)
		}
		isOk, okVal, _ := results[0].Result()
		if !isOk {
			t.Fatalf("iter %d: handle-event ok = false", i)
		}
		if okVal.U32() != 1 {
			t.Fatalf("iter %d: handle-event value mismatch", i)
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
		input := randComplexInputVal(rng)
		want := expectedProcessComplexVal(count, input)

		results, err := fn.CallAndPostReturn(ctx, apicomponent.ValString(id), apicomponent.ValU32(count), input)
		if err != nil {
			t.Fatalf("iter %d: process-complex: %v", i, err)
		}
		got := results[0].U32()
		if got != want {
			t.Fatalf("iter %d: process-complex = %d, want %d (count=%d)", i, got, want, count)
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

		results, err := fn.CallAndPostReturn(ctx, apicomponent.ValString(name), apicomponent.ValU32(code))
		if err != nil {
			t.Fatalf("iter %d: transform-complex: %v", i, err)
		}
		rec := results[0].Record()

		// Verify top-level
		if rec["success"].Bool() != true {
			t.Fatalf("iter %d: success = false", i)
		}

		// Verify nested detail
		detail := rec["detail"].Record()
		if detail["code"].U32() != 200 {
			t.Fatalf("iter %d: detail.code = %v", i, detail["code"].U32())
		}
		wantMsg := "ok:" + name
		if detail["message"].StringVal() != wantMsg {
			t.Fatalf("iter %d: detail.message = %q, want %q", i, detail["message"].StringVal(), wantMsg)
		}

		// Verify list<u32>
		values := rec["values"].List()
		if len(values) != 3 {
			t.Fatalf("iter %d: values len = %d", i, len(values))
		}

		// Verify nested event
		event := rec["event"].Record()
		if event["event-type"].Enum() != "green" {
			t.Fatalf("iter %d: event-type = %v", i, event["event-type"].Enum())
		}

		// Verify label
		if rec["label"].StringVal() != name {
			t.Fatalf("iter %d: label = %v, want %v", i, rec["label"].StringVal(), name)
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
		results, err := fnOk.CallAndPostReturn(ctx, apicomponent.ValU32(v))
		if err != nil {
			t.Fatalf("iter %d: make-ok: %v", i, err)
		}
		isOk, okVal, _ := results[0].Result()
		if !isOk {
			t.Fatalf("iter %d: make-ok: ok = false", i)
		}
		if okVal.U32() != v {
			t.Fatalf("iter %d: make-ok: value = %v, want %v", i, okVal.U32(), v)
		}

		msg := randString(rng, rng.IntN(50)+1)
		results, err = fnErr.CallAndPostReturn(ctx, apicomponent.ValString(msg))
		if err != nil {
			t.Fatalf("iter %d: make-err: %v", i, err)
		}
		isOk2, _, errVal := results[0].Result()
		if isOk2 {
			t.Fatalf("iter %d: make-err: ok = true", i)
		}
		if errVal.StringVal() != msg {
			t.Fatalf("iter %d: make-err: error = %q, want %q", i, errVal.StringVal(), msg)
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
			_, err := fnEchoString.CallAndPostReturn(ctx, apicomponent.ValString(randString(rng, rng.IntN(100))))
			return err
		}},
		{"add-three", func() error {
			_, err := fnAddThree.CallAndPostReturn(ctx, apicomponent.ValU32(uint32(rng.IntN(1000))), apicomponent.ValU32(uint32(rng.IntN(1000))), apicomponent.ValU32(uint32(rng.IntN(1000))))
			return err
		}},
		{"mixed-params", func() error {
			_, err := fnMixed.CallAndPostReturn(ctx, apicomponent.ValString(randString(rng, 10)), apicomponent.ValU32(uint32(rng.IntN(100))), apicomponent.ValBool(rng.IntN(2) == 0))
			return err
		}},
		{"echo-enum", func() error {
			_, err := fnEchoEnum.CallAndPostReturn(ctx, apicomponent.ValEnum(randColor(rng)))
			return err
		}},
		{"echo-variant", func() error {
			_, err := fnEchoVariant.CallAndPostReturn(ctx, randShapeVal(rng))
			return err
		}},
		{"handle-bytes", func() error {
			bs := randBytes(rng, rng.IntN(128))
			elems := make([]apicomponent.Val, len(bs))
			for i, b := range bs {
				elems[i] = apicomponent.ValU8(b)
			}
			_, err := fnHandleBytes.CallAndPostReturn(ctx, apicomponent.ValString(randString(rng, 5)), apicomponent.ValList(elems))
			return err
		}},
		{"handle-event", func() error {
			_, err := fnHandleEvent.CallAndPostReturn(ctx, apicomponent.ValString(randString(rng, 5)), randEventDataVal(rng))
			return err
		}},
		{"process-complex", func() error {
			_, err := fnProcessComplex.CallAndPostReturn(ctx, apicomponent.ValString(randString(rng, 5)), apicomponent.ValU32(uint32(rng.IntN(50))), randComplexInputVal(rng))
			return err
		}},
		{"transform-complex", func() error {
			_, err := fnTransformComplex.CallAndPostReturn(ctx, apicomponent.ValString(randString(rng, 10)), apicomponent.ValU32(uint32(rng.IntN(100))))
			return err
		}},
		{"transform-complex-variant", func() error {
			_, err := fnTransformVariant.CallAndPostReturn(ctx, apicomponent.ValString(randString(rng, 10)), randShapeVal(rng))
			return err
		}},
		{"make-ok", func() error {
			_, err := fnMakeOk.CallAndPostReturn(ctx, apicomponent.ValU32(uint32(rng.IntN(10000))))
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
		if _, err := fn.CallAndPostReturn(ctx, apicomponent.ValString("benchmark-string-data")); err != nil {
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
		if _, err := fn.CallAndPostReturn(ctx, apicomponent.ValU32(1), apicomponent.ValU32(2), apicomponent.ValU32(3)); err != nil {
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
		if _, err := fn.CallAndPostReturn(ctx, apicomponent.ValEnum("green")); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEchoVariant(b *testing.B) {
	instance, ctx, cleanup := newBenchInstance(b)
	defer cleanup()
	fn := instance.ExportedInstance("test:repro/handler").ExportedFunction("echo-variant")
	p := apicomponent.ValF64(3.14)
	shape := apicomponent.ValVariant("circle", &p)
	b.ResetTimer()
	for b.Loop() {
		if _, err := fn.CallAndPostReturn(ctx, shape); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHandleBytes(b *testing.B) {
	instance, ctx, cleanup := newBenchInstance(b)
	defer cleanup()
	fn := instance.ExportedInstance("test:repro/handler").ExportedFunction("handle-bytes")
	elems := make([]apicomponent.Val, 64)
	for i := range elems {
		elems[i] = apicomponent.ValU8(0)
	}
	data := apicomponent.ValList(elems)
	b.ResetTimer()
	for b.Loop() {
		if _, err := fn.CallAndPostReturn(ctx, apicomponent.ValString("corr-id"), data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHandleEvent(b *testing.B) {
	instance, ctx, cleanup := newBenchInstance(b)
	defer cleanup()
	fn := instance.ExportedInstance("test:repro/handler").ExportedFunction("handle-event")
	event := apicomponent.ValRecord(map[string]apicomponent.Val{
		"event-type": apicomponent.ValEnum("red"),
		"metadata":   apicomponent.ValOption(nil),
	})
	b.ResetTimer()
	for b.Loop() {
		if _, err := fn.CallAndPostReturn(ctx, apicomponent.ValString("corr-id"), event); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProcessComplex(b *testing.B) {
	instance, ctx, cleanup := newBenchInstance(b)
	defer cleanup()
	fn := instance.ExportedInstance("test:repro/handler").ExportedFunction("process-complex")
	circlePayload := apicomponent.ValF64(2.0)
	priority := apicomponent.ValU32(10)
	metadata := apicomponent.ValList([]apicomponent.Val{apicomponent.ValU8(1), apicomponent.ValU8(2), apicomponent.ValU8(3)})
	input := apicomponent.ValRecord(map[string]apicomponent.Val{
		"id": apicomponent.ValString("bench-id"),
		"middle": apicomponent.ValRecord(map[string]apicomponent.Val{
			"inner": apicomponent.ValRecord(map[string]apicomponent.Val{
				"label": apicomponent.ValString("bench"), "score": apicomponent.ValF64(1.5), "active": apicomponent.ValBool(true),
			}),
			"tags":     apicomponent.ValList([]apicomponent.Val{apicomponent.ValString("a"), apicomponent.ValString("b")}),
			"priority": apicomponent.ValOption(&priority),
			"shape":    apicomponent.ValVariant("circle", &circlePayload),
		}),
		"color":    apicomponent.ValEnum("blue"),
		"metadata": apicomponent.ValOption(&metadata),
	})
	b.ResetTimer()
	for b.Loop() {
		if _, err := fn.CallAndPostReturn(ctx, apicomponent.ValString("ctx"), apicomponent.ValU32(5), input); err != nil {
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
		if _, err := fn.CallAndPostReturn(ctx, apicomponent.ValString("bench-name"), apicomponent.ValU32(42)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTransformComplexVariant(b *testing.B) {
	instance, ctx, cleanup := newBenchInstance(b)
	defer cleanup()
	fn := instance.ExportedInstance("test:repro/handler").ExportedFunction("transform-complex-variant")
	p := apicomponent.ValF64(9.0)
	shape := apicomponent.ValVariant("square", &p)
	b.ResetTimer()
	for b.Loop() {
		if _, err := fn.CallAndPostReturn(ctx, apicomponent.ValString("bench-name"), shape); err != nil {
			b.Fatal(err)
		}
	}
}
