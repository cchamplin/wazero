// internal/component/types/val_test.go

package types

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestValConstructorsAndAccessors(t *testing.T) {
	t.Run("Bool", func(t *testing.T) {
		v := ValBool(true)
		require.Equal(t, ValKindBool, v.Kind())
		require.True(t, v.Bool())

		v = ValBool(false)
		require.False(t, v.Bool())
	})

	t.Run("S32", func(t *testing.T) {
		v := ValS32(-42)
		require.Equal(t, ValKindS32, v.Kind())
		require.Equal(t, int32(-42), v.S32())
	})

	t.Run("U32", func(t *testing.T) {
		v := ValU32(42)
		require.Equal(t, ValKindU32, v.Kind())
		require.Equal(t, uint32(42), v.U32())
	})

	t.Run("S64", func(t *testing.T) {
		v := ValS64(-123456789)
		require.Equal(t, ValKindS64, v.Kind())
		require.Equal(t, int64(-123456789), v.S64())
	})

	t.Run("U64", func(t *testing.T) {
		v := ValU64(123456789)
		require.Equal(t, ValKindU64, v.Kind())
		require.Equal(t, uint64(123456789), v.U64())
	})

	t.Run("F32", func(t *testing.T) {
		v := ValF32(3.14)
		require.Equal(t, ValKindF32, v.Kind())
		require.Equal(t, float32(3.14), v.F32())
	})

	t.Run("F64", func(t *testing.T) {
		v := ValF64(3.14159265359)
		require.Equal(t, ValKindF64, v.Kind())
		require.Equal(t, float64(3.14159265359), v.F64())
	})

	t.Run("Char", func(t *testing.T) {
		v := ValChar('A')
		require.Equal(t, ValKindChar, v.Kind())
		require.Equal(t, rune('A'), v.Char())
	})

	t.Run("String", func(t *testing.T) {
		v := ValString("hello")
		require.Equal(t, ValKindString, v.Kind())
		require.Equal(t, "hello", v.StringVal())
	})
}

func TestValRecord(t *testing.T) {
	// Create a record value { a: 42, b: "hello" }
	fields := map[string]Val{
		"a": ValS32(42),
		"b": ValString("hello"),
	}
	v := ValRecord(fields)

	require.Equal(t, ValKindRecord, v.Kind())

	got := v.Record()
	require.Equal(t, int32(42), got["a"].S32())
	require.Equal(t, "hello", got["b"].StringVal())
}

func TestValRecordField(t *testing.T) {
	fields := map[string]Val{
		"x": ValF64(3.14),
	}
	v := ValRecord(fields)

	// Access single field
	x, ok := v.RecordField("x")
	require.True(t, ok)
	require.Equal(t, 3.14, x.F64())

	_, ok = v.RecordField("missing")
	require.False(t, ok)
}

func TestValVariant(t *testing.T) {
	// Create variant { some: 42 }
	payload := ValS32(42)
	v := ValVariant("some", &payload)

	require.Equal(t, ValKindVariant, v.Kind())

	caseName, casePayload := v.Variant()
	require.Equal(t, "some", caseName)
	require.NotNil(t, casePayload)
	require.Equal(t, int32(42), casePayload.S32())
}

func TestValVariantNoPayload(t *testing.T) {
	// Create variant { none }
	v := ValVariant("none", nil)

	caseName, casePayload := v.Variant()
	require.Equal(t, "none", caseName)
	require.Nil(t, casePayload)
}

func TestValOption(t *testing.T) {
	// Some(42)
	payload := ValS32(42)
	v := ValOption(&payload)

	require.Equal(t, ValKindOption, v.Kind())

	opt := v.Option()
	require.NotNil(t, opt)
	require.Equal(t, int32(42), opt.S32())
}

func TestValOptionNone(t *testing.T) {
	// None
	v := ValOption(nil)

	opt := v.Option()
	require.Nil(t, opt)
}

func TestValList(t *testing.T) {
	elements := []Val{ValS32(1), ValS32(2), ValS32(3)}
	v := ValList(elements)

	require.Equal(t, ValKindList, v.Kind())
	require.Equal(t, elements, v.List())
}

func TestValTuple(t *testing.T) {
	elements := []Val{ValS32(1), ValString("hello")}
	v := ValTuple(elements)

	require.Equal(t, ValKindTuple, v.Kind())
	require.Equal(t, elements, v.Tuple())
}

func TestValResult(t *testing.T) {
	// Ok(42)
	okVal := ValS32(42)
	v := ValResultOk(&okVal)

	require.Equal(t, ValKindResult, v.Kind())
	isOk, ok, err := v.Result()
	require.True(t, isOk)
	require.NotNil(t, ok)
	require.Equal(t, int32(42), ok.S32())
	require.Nil(t, err)
}

func TestValResultError(t *testing.T) {
	// Error("oops")
	errVal := ValString("oops")
	v := ValResultError(&errVal)

	isOk, ok, err := v.Result()
	require.False(t, isOk)
	require.Nil(t, ok)
	require.NotNil(t, err)
	require.Equal(t, "oops", err.StringVal())
}

func TestValFlags(t *testing.T) {
	flags := map[string]bool{"read": true, "write": false, "execute": true}
	v := ValFlags(flags)

	require.Equal(t, ValKindFlags, v.Kind())
	got := v.Flags()
	require.True(t, got["read"])
	require.False(t, got["write"])
	require.True(t, got["execute"])
}

func TestValEnum(t *testing.T) {
	v := ValEnum("green")

	require.Equal(t, ValKindEnum, v.Kind())
	require.Equal(t, "green", v.Enum())
}

func TestValOwn(t *testing.T) {
	v := ValOwn(42)
	require.Equal(t, ValKindOwn, v.Kind())
	require.Equal(t, uint32(42), v.Own())
}

func TestValBorrow(t *testing.T) {
	v := ValBorrow(99)
	require.Equal(t, ValKindBorrow, v.Kind())
	require.Equal(t, uint32(99), v.Borrow())
}

func TestValOwnWrongKind(t *testing.T) {
	v := ValS32(5)
	err := require.CapturePanic(func() { v.Own() })
	require.Error(t, err)
}

func TestValBorrowWrongKind(t *testing.T) {
	v := ValS32(5)
	err := require.CapturePanic(func() { v.Borrow() })
	require.Error(t, err)
}
