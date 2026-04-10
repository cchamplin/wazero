package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component/types"
)

// buildTestTypes creates a ComponentTypes bag with a variety of interned
// types useful for testing introspection.
func buildTestTypes(t *testing.T) (*types.ComponentTypes, testTypeHandles) {
	t.Helper()
	b := types.NewComponentTypesBuilder()

	// Scalars — no interning needed.
	u32 := types.U32
	s32 := types.S32
	str := types.String_
	boolVT := types.Bool

	// List<u32>
	listU32 := b.InternList(u32)

	// Record{ name: string, age: u32 }
	rec := b.InternRecord([]types.RecordField{
		{Name: "name", Type: str},
		{Name: "age", Type: u32},
	})

	// Tuple<u32, string>
	tup := b.InternTuple([]types.ValType{u32, str})

	// Variant{ ok(u32), err(string), none }
	variant := b.InternVariant([]types.VariantCase{
		{Name: "ok", Payload: u32, HasPayload: true},
		{Name: "err", Payload: str, HasPayload: true},
		{Name: "none", HasPayload: false},
	})

	// Enum{ red, green, blue }
	enum := b.InternEnum([]string{"red", "green", "blue"})

	// Option<u32>
	option := b.InternOption(u32)

	// Result<u32, string>
	result := b.InternResult(u32, str, true, true)

	// Result<_, string> (no ok payload)
	resultErrOnly := b.InternResult(types.ValType{}, str, false, true)

	// Result<u32, _> (no err payload)
	resultOkOnly := b.InternResult(u32, types.ValType{}, true, false)

	// Flags{ read, write, execute }
	flags := b.InternFlags([]string{"read", "write", "execute"})

	// FixedLengthList<u32, 5>
	fixedList := b.InternFixedLengthList(u32, 5)

	// Function: (a: u32, b: s32) -> (bool)
	paramsTup := b.InternTuple([]types.ValType{u32, s32})
	resultsTup := b.InternTuple([]types.ValType{boolVT})
	funcIdx := b.InternFunc(false, []string{"a", "b"}, paramsTup, resultsTup)

	// Function with no results: (msg: string) -> ()
	noResultsTup := b.InternTuple([]types.ValType{})
	funcNoResultsIdx := b.InternFunc(false, []string{"msg"}, b.InternTuple([]types.ValType{str}), noResultsTup)

	ct := b.Finish()

	return ct, testTypeHandles{
		listU32:          listU32,
		rec:              rec,
		tup:              tup,
		variant:          variant,
		enum:             enum,
		option:           option,
		result:           result,
		resultErrOnly:    resultErrOnly,
		resultOkOnly:     resultOkOnly,
		flags:            flags,
		fixedList:        fixedList,
		funcIdx:          funcIdx,
		funcNoResultsIdx: funcNoResultsIdx,
	}
}

type testTypeHandles struct {
	listU32          types.ValType
	rec              types.ValType
	tup              types.ValType
	variant          types.ValType
	enum             types.ValType
	option           types.ValType
	result           types.ValType
	resultErrOnly    types.ValType
	resultOkOnly     types.ValType
	flags            types.ValType
	fixedList        types.ValType
	funcIdx          types.FuncTypeIdx
	funcNoResultsIdx types.FuncTypeIdx
}

// --- FuncTypeInfo tests ---

func TestFuncTypeInfoNumParams(t *testing.T) {
	ct, h := buildTestTypes(t)
	ft := &ct.Funcs[h.funcIdx]
	info := NewFuncTypeInfo(ft, ct)
	if got := info.NumParams(); got != 2 {
		t.Errorf("NumParams() = %d, want 2", got)
	}
}

func TestFuncTypeInfoNumResults(t *testing.T) {
	ct, h := buildTestTypes(t)
	ft := &ct.Funcs[h.funcIdx]
	info := NewFuncTypeInfo(ft, ct)
	if got := info.NumResults(); got != 1 {
		t.Errorf("NumResults() = %d, want 1", got)
	}
}

func TestFuncTypeInfoParams(t *testing.T) {
	ct, h := buildTestTypes(t)
	ft := &ct.Funcs[h.funcIdx]
	info := NewFuncTypeInfo(ft, ct)
	params := info.Params()
	if len(params) != 2 {
		t.Fatalf("len(Params()) = %d, want 2", len(params))
	}
	if params[0].Name != "a" {
		t.Errorf("Params()[0].Name = %q, want %q", params[0].Name, "a")
	}
	if params[0].Type.Kind() != TypeKindU32 {
		t.Errorf("Params()[0].Type.Kind() = %v, want TypeKindU32", params[0].Type.Kind())
	}
	if params[1].Name != "b" {
		t.Errorf("Params()[1].Name = %q, want %q", params[1].Name, "b")
	}
	if params[1].Type.Kind() != TypeKindS32 {
		t.Errorf("Params()[1].Type.Kind() = %v, want TypeKindS32", params[1].Type.Kind())
	}
}

func TestFuncTypeInfoResults(t *testing.T) {
	ct, h := buildTestTypes(t)
	ft := &ct.Funcs[h.funcIdx]
	info := NewFuncTypeInfo(ft, ct)
	results := info.Results()
	if len(results) != 1 {
		t.Fatalf("len(Results()) = %d, want 1", len(results))
	}
	if results[0].Name != "" {
		t.Errorf("Results()[0].Name = %q, want empty", results[0].Name)
	}
	if results[0].Type.Kind() != TypeKindBool {
		t.Errorf("Results()[0].Type.Kind() = %v, want TypeKindBool", results[0].Type.Kind())
	}
}

func TestFuncTypeInfoNoResults(t *testing.T) {
	ct, h := buildTestTypes(t)
	ft := &ct.Funcs[h.funcNoResultsIdx]
	info := NewFuncTypeInfo(ft, ct)
	if got := info.NumResults(); got != 0 {
		t.Errorf("NumResults() = %d, want 0", got)
	}
	results := info.Results()
	if len(results) != 0 {
		t.Errorf("len(Results()) = %d, want 0", len(results))
	}
}

// --- ValTypeInfo scalar tests ---

func TestValTypeInfoScalarKind(t *testing.T) {
	ct, _ := buildTestTypes(t)
	tests := []struct {
		name string
		vt   types.ValType
		want TypeKind
	}{
		{"bool", types.Bool, TypeKindBool},
		{"u32", types.U32, TypeKindU32},
		{"s64", types.S64, TypeKindS64},
		{"f32", types.F32, TypeKindF32},
		{"f64", types.F64, TypeKindF64},
		{"string", types.String_, TypeKindString},
		{"char", types.Char, TypeKindChar},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			info := NewValTypeInfo(tc.vt, ct)
			if got := info.Kind(); got != tc.want {
				t.Errorf("Kind() = %v, want %v", got, tc.want)
			}
		})
	}
}

// --- ValTypeInfo composite tests ---

func TestValTypeInfoListElement(t *testing.T) {
	ct, h := buildTestTypes(t)
	info := NewValTypeInfo(h.listU32, ct)
	if info.Kind() != TypeKindList {
		t.Fatalf("Kind() = %v, want TypeKindList", info.Kind())
	}
	elem := info.ListElement()
	if elem.Kind() != TypeKindU32 {
		t.Errorf("ListElement().Kind() = %v, want TypeKindU32", elem.Kind())
	}
}

func TestValTypeInfoFixedList(t *testing.T) {
	ct, h := buildTestTypes(t)
	info := NewValTypeInfo(h.fixedList, ct)
	if info.Kind() != TypeKindFixedList {
		t.Fatalf("Kind() = %v, want TypeKindFixedList", info.Kind())
	}
	elem := info.FixedListElement()
	if elem.Kind() != TypeKindU32 {
		t.Errorf("FixedListElement().Kind() = %v, want TypeKindU32", elem.Kind())
	}
	if got := info.FixedListLength(); got != 5 {
		t.Errorf("FixedListLength() = %d, want 5", got)
	}
}

func TestValTypeInfoRecordFields(t *testing.T) {
	ct, h := buildTestTypes(t)
	info := NewValTypeInfo(h.rec, ct)
	if info.Kind() != TypeKindRecord {
		t.Fatalf("Kind() = %v, want TypeKindRecord", info.Kind())
	}
	fields := info.RecordFields()
	if len(fields) != 2 {
		t.Fatalf("len(RecordFields()) = %d, want 2", len(fields))
	}
	if fields[0].Name != "name" {
		t.Errorf("RecordFields()[0].Name = %q, want %q", fields[0].Name, "name")
	}
	if fields[0].Type.Kind() != TypeKindString {
		t.Errorf("RecordFields()[0].Type.Kind() = %v, want TypeKindString", fields[0].Type.Kind())
	}
	if fields[1].Name != "age" {
		t.Errorf("RecordFields()[1].Name = %q, want %q", fields[1].Name, "age")
	}
	if fields[1].Type.Kind() != TypeKindU32 {
		t.Errorf("RecordFields()[1].Type.Kind() = %v, want TypeKindU32", fields[1].Type.Kind())
	}
}

func TestValTypeInfoTupleTypes(t *testing.T) {
	ct, h := buildTestTypes(t)
	info := NewValTypeInfo(h.tup, ct)
	if info.Kind() != TypeKindTuple {
		t.Fatalf("Kind() = %v, want TypeKindTuple", info.Kind())
	}
	elems := info.TupleTypes()
	if len(elems) != 2 {
		t.Fatalf("len(TupleTypes()) = %d, want 2", len(elems))
	}
	if elems[0].Kind() != TypeKindU32 {
		t.Errorf("TupleTypes()[0].Kind() = %v, want TypeKindU32", elems[0].Kind())
	}
	if elems[1].Kind() != TypeKindString {
		t.Errorf("TupleTypes()[1].Kind() = %v, want TypeKindString", elems[1].Kind())
	}
}

func TestValTypeInfoVariantCases(t *testing.T) {
	ct, h := buildTestTypes(t)
	info := NewValTypeInfo(h.variant, ct)
	if info.Kind() != TypeKindVariant {
		t.Fatalf("Kind() = %v, want TypeKindVariant", info.Kind())
	}
	cases := info.VariantCases()
	if len(cases) != 3 {
		t.Fatalf("len(VariantCases()) = %d, want 3", len(cases))
	}
	// case "ok" with u32 payload
	if cases[0].Name != "ok" {
		t.Errorf("VariantCases()[0].Name = %q, want %q", cases[0].Name, "ok")
	}
	if cases[0].Type == nil {
		t.Fatal("VariantCases()[0].Type is nil, want non-nil")
	}
	if cases[0].Type.Kind() != TypeKindU32 {
		t.Errorf("VariantCases()[0].Type.Kind() = %v, want TypeKindU32", cases[0].Type.Kind())
	}
	// case "err" with string payload
	if cases[1].Name != "err" {
		t.Errorf("VariantCases()[1].Name = %q, want %q", cases[1].Name, "err")
	}
	if cases[1].Type == nil {
		t.Fatal("VariantCases()[1].Type is nil, want non-nil")
	}
	if cases[1].Type.Kind() != TypeKindString {
		t.Errorf("VariantCases()[1].Type.Kind() = %v, want TypeKindString", cases[1].Type.Kind())
	}
	// case "none" with no payload
	if cases[2].Name != "none" {
		t.Errorf("VariantCases()[2].Name = %q, want %q", cases[2].Name, "none")
	}
	if cases[2].Type != nil {
		t.Errorf("VariantCases()[2].Type = %v, want nil", cases[2].Type)
	}
}

func TestValTypeInfoEnumCases(t *testing.T) {
	ct, h := buildTestTypes(t)
	info := NewValTypeInfo(h.enum, ct)
	if info.Kind() != TypeKindEnum {
		t.Fatalf("Kind() = %v, want TypeKindEnum", info.Kind())
	}
	names := info.EnumCases()
	if len(names) != 3 {
		t.Fatalf("len(EnumCases()) = %d, want 3", len(names))
	}
	expected := []string{"red", "green", "blue"}
	for i, want := range expected {
		if names[i] != want {
			t.Errorf("EnumCases()[%d] = %q, want %q", i, names[i], want)
		}
	}
}

func TestValTypeInfoOptionSome(t *testing.T) {
	ct, h := buildTestTypes(t)
	info := NewValTypeInfo(h.option, ct)
	if info.Kind() != TypeKindOption {
		t.Fatalf("Kind() = %v, want TypeKindOption", info.Kind())
	}
	some := info.OptionSome()
	if some.Kind() != TypeKindU32 {
		t.Errorf("OptionSome().Kind() = %v, want TypeKindU32", some.Kind())
	}
}

func TestValTypeInfoResultBoth(t *testing.T) {
	ct, h := buildTestTypes(t)
	info := NewValTypeInfo(h.result, ct)
	if info.Kind() != TypeKindResult {
		t.Fatalf("Kind() = %v, want TypeKindResult", info.Kind())
	}
	ok := info.ResultOk()
	if ok == nil {
		t.Fatal("ResultOk() is nil, want non-nil")
	}
	if ok.Kind() != TypeKindU32 {
		t.Errorf("ResultOk().Kind() = %v, want TypeKindU32", ok.Kind())
	}
	err := info.ResultErr()
	if err == nil {
		t.Fatal("ResultErr() is nil, want non-nil")
	}
	if err.Kind() != TypeKindString {
		t.Errorf("ResultErr().Kind() = %v, want TypeKindString", err.Kind())
	}
}

func TestValTypeInfoResultErrOnly(t *testing.T) {
	ct, h := buildTestTypes(t)
	info := NewValTypeInfo(h.resultErrOnly, ct)
	ok := info.ResultOk()
	if ok != nil {
		t.Errorf("ResultOk() = %v, want nil", ok)
	}
	err := info.ResultErr()
	if err == nil {
		t.Fatal("ResultErr() is nil, want non-nil")
	}
	if err.Kind() != TypeKindString {
		t.Errorf("ResultErr().Kind() = %v, want TypeKindString", err.Kind())
	}
}

func TestValTypeInfoResultOkOnly(t *testing.T) {
	ct, h := buildTestTypes(t)
	info := NewValTypeInfo(h.resultOkOnly, ct)
	ok := info.ResultOk()
	if ok == nil {
		t.Fatal("ResultOk() is nil, want non-nil")
	}
	if ok.Kind() != TypeKindU32 {
		t.Errorf("ResultOk().Kind() = %v, want TypeKindU32", ok.Kind())
	}
	err := info.ResultErr()
	if err != nil {
		t.Errorf("ResultErr() = %v, want nil", err)
	}
}

func TestValTypeInfoFlagsNames(t *testing.T) {
	ct, h := buildTestTypes(t)
	info := NewValTypeInfo(h.flags, ct)
	if info.Kind() != TypeKindFlags {
		t.Fatalf("Kind() = %v, want TypeKindFlags", info.Kind())
	}
	names := info.FlagsNames()
	if len(names) != 3 {
		t.Fatalf("len(FlagsNames()) = %d, want 3", len(names))
	}
	expected := []string{"read", "write", "execute"}
	for i, want := range expected {
		if names[i] != want {
			t.Errorf("FlagsNames()[%d] = %q, want %q", i, names[i], want)
		}
	}
}

// TestValTypeInfoNestedComposite verifies introspection through nested types:
// a record containing a list field.
func TestValTypeInfoNestedComposite(t *testing.T) {
	b := types.NewComponentTypesBuilder()

	listStr := b.InternList(types.String_)
	rec := b.InternRecord([]types.RecordField{
		{Name: "tags", Type: listStr},
		{Name: "count", Type: types.U32},
	})
	ct := b.Finish()

	info := NewValTypeInfo(rec, ct)
	fields := info.RecordFields()
	if len(fields) != 2 {
		t.Fatalf("len(RecordFields()) = %d, want 2", len(fields))
	}

	// First field is list<string>
	if fields[0].Name != "tags" {
		t.Errorf("field[0].Name = %q, want %q", fields[0].Name, "tags")
	}
	if fields[0].Type.Kind() != TypeKindList {
		t.Fatalf("field[0].Type.Kind() = %v, want TypeKindList", fields[0].Type.Kind())
	}
	elem := fields[0].Type.ListElement()
	if elem.Kind() != TypeKindString {
		t.Errorf("field[0] list element Kind() = %v, want TypeKindString", elem.Kind())
	}
}
