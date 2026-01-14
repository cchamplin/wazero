// internal/component/linker_test.go

package component

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestNewLinker(t *testing.T) {
	l := NewLinker()
	require.NotNil(t, l)
	require.NotNil(t, l.definitions)
}

func TestLinker_DefineFunc(t *testing.T) {
	l := NewLinker()

	funcType := &FuncType{
		Params:  []NamedValType{{Name: "x", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7f}}},
		Results: []NamedValType{{Name: "", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7f}}},
	}

	err := l.DefineFunc("test:api", "add", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return []Val{ValS32(42)}, nil
	})
	require.NoError(t, err)

	// Check it was added
	def, ok := l.definitions["test:api/add"]
	require.True(t, ok)
	require.NotNil(t, def)
}

func TestLinker_DefineFunc_Duplicate(t *testing.T) {
	l := NewLinker()
	funcType := &FuncType{}

	err := l.DefineFunc("test", "fn", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	// Duplicate should error
	err = l.DefineFunc("test", "fn", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return nil, nil
	})
	require.Error(t, err)
}
