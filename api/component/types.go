package component

import "github.com/tetratelabs/wazero/internal/component/types"

// TypeKind discriminates the kind of a ValTypeInfo. It mirrors the internal
// types.TypeKind and is used by ValTypeInfo.Kind to report what component
// model type a value type represents (e.g., Bool, String, List, Record, etc.).
//
// For scalar kinds (TypeKindBool through TypeKindString), the ValTypeInfo
// carries no further detail. For composite kinds, introspection methods
// on ValTypeInfo return the inner structure.
type TypeKind = types.TypeKind

// TypeKind constants re-exported for public use.
const (
	TypeKindBool         = types.TypeKindBool
	TypeKindS8           = types.TypeKindS8
	TypeKindU8           = types.TypeKindU8
	TypeKindS16          = types.TypeKindS16
	TypeKindU16          = types.TypeKindU16
	TypeKindS32          = types.TypeKindS32
	TypeKindU32          = types.TypeKindU32
	TypeKindS64          = types.TypeKindS64
	TypeKindU64          = types.TypeKindU64
	TypeKindF32          = types.TypeKindF32
	TypeKindF64          = types.TypeKindF64
	TypeKindChar         = types.TypeKindChar
	TypeKindString       = types.TypeKindString
	TypeKindList         = types.TypeKindList
	TypeKindFixedList    = types.TypeKindFixedList
	TypeKindRecord       = types.TypeKindRecord
	TypeKindTuple        = types.TypeKindTuple
	TypeKindVariant      = types.TypeKindVariant
	TypeKindEnum         = types.TypeKindEnum
	TypeKindOption       = types.TypeKindOption
	TypeKindResult       = types.TypeKindResult
	TypeKindFlags        = types.TypeKindFlags
	TypeKindOwn          = types.TypeKindOwn
	TypeKindBorrow       = types.TypeKindBorrow
	TypeKindStream       = types.TypeKindStream
	TypeKindFuture       = types.TypeKindFuture
	TypeKindErrorContext = types.TypeKindErrorContext
)

// Param represents a named function parameter or result.
type Param struct {
	Name string
	Type ValTypeInfo
}

// Field represents a named record field.
type Field struct {
	Name string
	Type ValTypeInfo
}

// Case represents a variant case with an optional payload type.
type Case struct {
	Name string
	Type *ValTypeInfo // nil if no payload
}

// FuncTypeInfo provides introspection into a component function's type.
// It wraps the internal TypeFunc and ComponentTypes to resolve parameter
// and result types without exposing internal structures.
//
// Obtain a FuncTypeInfo via NewFuncTypeInfo (typically called by the
// runtime when implementing ComponentFunc.Type).
type FuncTypeInfo struct {
	inner *types.TypeFunc
	types *types.ComponentTypes
}

// NewFuncTypeInfo creates a FuncTypeInfo wrapping the given internal types.
// Exported so that the runtime can construct FuncTypeInfo values when
// implementing ComponentFunc.Type().
func NewFuncTypeInfo(ft *types.TypeFunc, ct *types.ComponentTypes) FuncTypeInfo {
	return FuncTypeInfo{inner: ft, types: ct}
}

// NumParams returns the number of parameters this function type expects.
func (f FuncTypeInfo) NumParams() int {
	tup := &f.types.Tuples[f.inner.Params.Index]
	return len(tup.Types)
}

// NumResults returns the number of results this function type returns.
func (f FuncTypeInfo) NumResults() int {
	tup := &f.types.Tuples[f.inner.Results.Index]
	return len(tup.Types)
}

// Params returns the function's parameters as a slice of named Param values.
// The names come from the function type's ParamNames; the types come from
// the Params tuple.
func (f FuncTypeInfo) Params() []Param {
	tup := &f.types.Tuples[f.inner.Params.Index]
	out := make([]Param, len(tup.Types))
	for i, vt := range tup.Types {
		name := ""
		if i < len(f.inner.ParamNames) {
			name = f.inner.ParamNames[i]
		}
		out[i] = Param{
			Name: name,
			Type: ValTypeInfo{inner: vt, types: f.types},
		}
	}
	return out
}

// Results returns the function's results as a slice of Param values.
// Result names are empty strings because the component model TypeFunc
// does not store result names.
func (f FuncTypeInfo) Results() []Param {
	tup := &f.types.Tuples[f.inner.Results.Index]
	out := make([]Param, len(tup.Types))
	for i, vt := range tup.Types {
		out[i] = Param{
			Name: "",
			Type: ValTypeInfo{inner: vt, types: f.types},
		}
	}
	return out
}

// ValTypeInfo provides introspection into a component value type.
// It wraps an internal ValType and the ComponentTypes bag so that
// composite types can be resolved without exposing internal structures.
//
// For scalar kinds (Bool through String), only Kind() is meaningful.
// For composite kinds, use the corresponding method (ListElement,
// RecordFields, etc.) to inspect the inner structure.
type ValTypeInfo struct {
	inner types.ValType
	types *types.ComponentTypes
}

// NewValTypeInfo creates a ValTypeInfo wrapping the given internal types.
// Exported so that the runtime can construct ValTypeInfo values.
func NewValTypeInfo(vt types.ValType, ct *types.ComponentTypes) ValTypeInfo {
	return ValTypeInfo{inner: vt, types: ct}
}

// Kind returns the TypeKind discriminant for this value type.
func (v ValTypeInfo) Kind() TypeKind {
	return v.inner.Kind
}

// ListElement returns the element type of a list. Panics if Kind is not
// TypeKindList.
func (v ValTypeInfo) ListElement() ValTypeInfo {
	list := &v.types.Lists[v.inner.Index]
	return ValTypeInfo{inner: list.Element, types: v.types}
}

// FixedListElement returns the element type of a fixed-length list.
// Panics if Kind is not TypeKindFixedList.
func (v ValTypeInfo) FixedListElement() ValTypeInfo {
	fl := &v.types.FixedLists[v.inner.Index]
	return ValTypeInfo{inner: fl.Element, types: v.types}
}

// FixedListLength returns the compile-time length of a fixed-length list.
// Panics if Kind is not TypeKindFixedList.
func (v ValTypeInfo) FixedListLength() uint32 {
	fl := &v.types.FixedLists[v.inner.Index]
	return fl.Length
}

// RecordFields returns the fields of a record type. Panics if Kind is not
// TypeKindRecord.
func (v ValTypeInfo) RecordFields() []Field {
	rec := &v.types.Records[v.inner.Index]
	out := make([]Field, len(rec.Fields))
	for i, f := range rec.Fields {
		out[i] = Field{
			Name: f.Name,
			Type: ValTypeInfo{inner: f.Type, types: v.types},
		}
	}
	return out
}

// TupleTypes returns the element types of a tuple type. Panics if Kind is
// not TypeKindTuple.
func (v ValTypeInfo) TupleTypes() []ValTypeInfo {
	tup := &v.types.Tuples[v.inner.Index]
	out := make([]ValTypeInfo, len(tup.Types))
	for i, vt := range tup.Types {
		out[i] = ValTypeInfo{inner: vt, types: v.types}
	}
	return out
}

// VariantCases returns the cases of a variant type. Panics if Kind is not
// TypeKindVariant.
func (v ValTypeInfo) VariantCases() []Case {
	variant := &v.types.Variants[v.inner.Index]
	out := make([]Case, len(variant.Cases))
	for i, c := range variant.Cases {
		out[i] = Case{Name: c.Name}
		if c.HasPayload {
			info := ValTypeInfo{inner: c.Payload, types: v.types}
			out[i].Type = &info
		}
	}
	return out
}

// EnumCases returns the case names of an enum type. Panics if Kind is not
// TypeKindEnum.
func (v ValTypeInfo) EnumCases() []string {
	enum := &v.types.Enums[v.inner.Index]
	return enum.Names
}

// OptionSome returns the "some" element type of an option type. Panics if
// Kind is not TypeKindOption.
func (v ValTypeInfo) OptionSome() ValTypeInfo {
	opt := &v.types.Options[v.inner.Index]
	return ValTypeInfo{inner: opt.Element, types: v.types}
}

// ResultOk returns the "ok" type of a result type, or nil if the result
// has no ok payload. Panics if Kind is not TypeKindResult.
func (v ValTypeInfo) ResultOk() *ValTypeInfo {
	res := &v.types.Results[v.inner.Index]
	if !res.HasOK {
		return nil
	}
	info := ValTypeInfo{inner: res.OK, types: v.types}
	return &info
}

// ResultErr returns the "err" type of a result type, or nil if the result
// has no error payload. Panics if Kind is not TypeKindResult.
func (v ValTypeInfo) ResultErr() *ValTypeInfo {
	res := &v.types.Results[v.inner.Index]
	if !res.HasErr {
		return nil
	}
	info := ValTypeInfo{inner: res.Err, types: v.types}
	return &info
}

// FlagsNames returns the flag names of a flags type. Panics if Kind is not
// TypeKindFlags.
func (v ValTypeInfo) FlagsNames() []string {
	flags := &v.types.Flags[v.inner.Index]
	return flags.Names
}
