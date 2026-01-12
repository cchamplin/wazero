// internal/component/val_test.go

package component

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
		require.Equal(t, "hello", v.String())
	})
}
