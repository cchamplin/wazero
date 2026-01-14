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

func TestLinker_DefineInstance(t *testing.T) {
	l := NewLinker()

	funcType := &FuncType{}

	err := l.DefineInstance("wasi:io/streams@0.2.0").
		Func("read", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
			return nil, nil
		}).
		Func("write", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
			return nil, nil
		}).
		Build()
	require.NoError(t, err)

	// Check it was added
	def, ok := l.definitions["wasi:io/streams@0.2.0"]
	require.True(t, ok)
	instDef, ok := def.(*InstanceDef)
	require.True(t, ok)
	require.Equal(t, 2, len(instDef.Exports))
}

func TestLinker_DefineResource(t *testing.T) {
	l := NewLinker()

	destroyed := false
	err := l.DefineResource("wasi:io/streams@0.2.0", "input-stream", func(rep uint32) {
		destroyed = true
	})
	require.NoError(t, err)

	// Check it was added
	def, ok := l.definitions["wasi:io/streams@0.2.0/input-stream"]
	require.True(t, ok)
	resDef, ok := def.(*ResourceDef)
	require.True(t, ok)

	// Call destructor to verify it works
	resDef.Destructor(0)
	require.True(t, destroyed)
}

func TestLinker_DefineResource_Duplicate(t *testing.T) {
	l := NewLinker()

	err := l.DefineResource("test", "res", func(rep uint32) {})
	require.NoError(t, err)

	// Duplicate should error
	err = l.DefineResource("test", "res", func(rep uint32) {})
	require.Error(t, err)
}

func TestLinker_Get_Direct(t *testing.T) {
	l := NewLinker()
	funcType := &FuncType{}

	l.DefineFunc("test:api", "fn", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return nil, nil
	})

	// Direct lookup
	def, ok := l.Get("test:api/fn")
	require.True(t, ok)
	require.NotNil(t, def)
}

func TestLinker_Get_NotFound(t *testing.T) {
	l := NewLinker()

	def, ok := l.Get("nonexistent")
	require.False(t, ok)
	require.Nil(t, def)
}

func TestLinker_Get_Instance(t *testing.T) {
	l := NewLinker()
	funcType := &FuncType{}

	l.DefineInstance("wasi:io/streams@0.2.0").
		Func("read", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
			return nil, nil
		}).
		Build()

	// Get the instance
	def, ok := l.Get("wasi:io/streams@0.2.0")
	require.True(t, ok)
	instDef, ok := def.(*InstanceDef)
	require.True(t, ok)
	require.NotNil(t, instDef.Exports["read"])
}
