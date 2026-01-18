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

func TestLinker_MatchImport_OldImportNewItem(t *testing.T) {
	// Wasmtime test: old_import_importing_new_item
	// Component requires v1.0.0, linker provides v1.0.1
	l := NewLinker()
	funcType := &FuncType{}

	// Define v1.0.1
	err := l.DefineFunc("test:api@1.0.1", "fn", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	// Request v1.0.0 - should match v1.0.1
	def, err := l.MatchImport("test:api@1.0.0/fn")
	require.NoError(t, err)
	require.NotNil(t, def)
}

func TestLinker_MatchImport_NewImportOldItem(t *testing.T) {
	// Wasmtime test: new_import_importing_old_item
	// Component requires v1.0.1, linker provides v1.0.0 - should NOT match
	l := NewLinker()
	funcType := &FuncType{}

	// Define v1.0.0
	err := l.DefineFunc("test:api@1.0.0", "fn", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	// Request v1.0.1 - should NOT match v1.0.0
	_, err = l.MatchImport("test:api@1.0.1/fn")
	require.Error(t, err)
}

func TestLinker_MatchImport_SelectsMax(t *testing.T) {
	// Wasmtime test: missing_import_selects_max
	l := NewLinker()
	funcType := &FuncType{}

	// Define multiple versions
	l.DefineFunc("test:api@1.0.0", "fn", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return []Val{ValS32(100)}, nil
	})
	l.DefineFunc("test:api@1.0.2", "fn", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return []Val{ValS32(102)}, nil
	})
	l.DefineFunc("test:api@1.0.1", "fn", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return []Val{ValS32(101)}, nil
	})

	// Request v1.0.0 - should select highest compatible (v1.0.2)
	def, err := l.MatchImport("test:api@1.0.0/fn")
	require.NoError(t, err)
	funcDef := def.(*FuncDef)

	// Call to verify we got v1.0.2
	results, err := funcDef.Callback(context.Background(), nil)
	require.NoError(t, err)
	require.Equal(t, int32(102), results[0].S32())
}

func TestLinker_MatchImport_DirectMatch(t *testing.T) {
	// Direct match (no version) should still work
	l := NewLinker()
	funcType := &FuncType{}

	l.DefineFunc("test", "fn", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return nil, nil
	})

	def, err := l.MatchImport("test/fn")
	require.NoError(t, err)
	require.NotNil(t, def)
}

func TestLinker_Instantiate_Basic(t *testing.T) {
	l := NewLinker()

	// Create a minimal component
	c := &Component{
		Exports: []Export{
			{Name: "test", Kind: ExportKindFunc, Idx: 0},
		},
	}

	// Instantiate without imports
	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)
	require.NotNil(t, inst)
	require.Equal(t, c, inst.Component())
}

func TestLinker_Instantiate_WithImports(t *testing.T) {
	l := NewLinker()
	funcType := &FuncType{}

	// Define the import
	err := l.DefineFunc("test:api@1.0.0", "fn", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return []Val{ValS32(42)}, nil
	})
	require.NoError(t, err)

	// Create component with import
	c := &Component{
		Imports: []Import{
			{
				Name: "test:api@1.0.0/fn",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0,
				},
			},
		},
	}

	// Instantiate - should resolve import
	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)
	require.NotNil(t, inst)
}

func TestLinker_Instantiate_MissingImport(t *testing.T) {
	l := NewLinker()

	// Create component with unresolved import
	c := &Component{
		Imports: []Import{
			{
				Name: "missing:api@1.0.0/fn",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0,
				},
			},
		},
	}

	// Instantiate - should fail
	_, err := l.Instantiate(context.Background(), c)
	require.Error(t, err)
}

func TestInstance_GetExportedFunc(t *testing.T) {
	l := NewLinker()
	funcType := &FuncType{}

	// Create component with exported function
	c := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: funcType},
		},
		Exports: []Export{
			{Name: "add", Kind: ExportKindFunc, Idx: 0},
		},
	}

	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)

	// Get exported function
	fn := inst.GetExportedFunc("add")
	require.NotNil(t, fn)
}

func TestInstance_GetExportedFunc_NotFound(t *testing.T) {
	l := NewLinker()

	c := &Component{
		Exports: []Export{
			{Name: "add", Kind: ExportKindFunc, Idx: 0},
		},
	}

	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)

	// Non-existent export returns nil
	fn := inst.GetExportedFunc("missing")
	require.Nil(t, fn)
}

func TestInstance_GetExportedFunc_ExportOldGetNew(t *testing.T) {
	// Wasmtime test: export_old_get_new
	// Component exports v1.0.0, caller requests v1.0.1
	l := NewLinker()
	funcType := &FuncType{}

	c := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: funcType},
		},
		Exports: []Export{
			{Name: "test:api@1.0.0/fn", Kind: ExportKindFunc, Idx: 0},
		},
	}

	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)

	// Request v1.0.1 - should match v1.0.0 (backward compatible)
	fn := inst.GetExportedFunc("test:api@1.0.1/fn")
	require.NotNil(t, fn)
}

func TestInstance_GetExportedFunc_ExportNewGetOld(t *testing.T) {
	// Wasmtime test: export_new_get_old
	// Component exports v1.0.1, caller requests v1.0.0
	l := NewLinker()
	funcType := &FuncType{}

	c := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: funcType},
		},
		Exports: []Export{
			{Name: "test:api@1.0.1/fn", Kind: ExportKindFunc, Idx: 0},
		},
	}

	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)

	// Request v1.0.0 - should match v1.0.1 (forward compatible)
	fn := inst.GetExportedFunc("test:api@1.0.0/fn")
	require.NotNil(t, fn)
}

func TestInstance_GetExportedFunc_SelectsMax(t *testing.T) {
	// Wasmtime test: export_missing_get_max
	l := NewLinker()
	funcType := &FuncType{}

	c := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: funcType},
		},
		Exports: []Export{
			{Name: "test:api@1.0.0/fn", Kind: ExportKindFunc, Idx: 0},
			{Name: "test:api@1.0.2/fn", Kind: ExportKindFunc, Idx: 0},
			{Name: "test:api@1.0.1/fn", Kind: ExportKindFunc, Idx: 0},
		},
	}

	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)

	// Request v1.0.0 - should match highest compatible (v1.0.2)
	fn := inst.GetExportedFunc("test:api@1.0.0/fn")
	require.NotNil(t, fn)
	// Verify we got the right one by checking the name
	require.Equal(t, "test:api@1.0.2/fn", fn.name)
}

func TestLinker_RelaxedSemverMatching_FuncImport(t *testing.T) {
	// Test relaxed semver matching for function imports
	l := NewLinker()
	funcType := &FuncType{}

	// Define v0.2.0
	err := l.DefineFunc("test:api@0.2.0", "fn", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return []Val{ValS32(200)}, nil
	})
	require.NoError(t, err)

	// Strict mode: request v0.2.3 - should NOT match v0.2.0 (strict requires patch >= required)
	_, err = l.MatchImport("test:api@0.2.3/fn")
	require.Error(t, err, "strict mode should not match 0.2.0 for 0.2.3 requirement")

	// Enable relaxed matching
	l.SetRelaxedSemverMatching(true)
	require.True(t, l.RelaxedSemverMatching())

	// Relaxed mode: request v0.2.3 - should match v0.2.0
	def, err := l.MatchImport("test:api@0.2.3/fn")
	require.NoError(t, err, "relaxed mode should match 0.2.0 for 0.2.3 requirement")
	require.NotNil(t, def)

	// Verify we got the right function
	funcDef := def.(*FuncDef)
	results, err := funcDef.Callback(context.Background(), nil)
	require.NoError(t, err)
	require.Equal(t, int32(200), results[0].S32())
}

func TestLinker_RelaxedSemverMatching_InstanceImport(t *testing.T) {
	// Test relaxed semver matching for instance imports
	l := NewLinker()
	funcType := &FuncType{}

	// Define instance at v0.2.0
	err := l.DefineInstance("wasi:cli/environment@0.2.0").
		Func("get-environment", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
			return nil, nil
		}).
		Build()
	require.NoError(t, err)

	// Strict mode: request v0.2.3 - should NOT match v0.2.0
	_, err = l.MatchImport("wasi:cli/environment@0.2.3")
	require.Error(t, err, "strict mode should not match 0.2.0 for 0.2.3 requirement")

	// Enable relaxed matching
	l.SetRelaxedSemverMatching(true)

	// Relaxed mode: request v0.2.3 - should match v0.2.0
	def, err := l.MatchImport("wasi:cli/environment@0.2.3")
	require.NoError(t, err, "relaxed mode should match 0.2.0 for 0.2.3 requirement")
	require.NotNil(t, def)

	// Verify we got the instance
	instDef, ok := def.(*InstanceDef)
	require.True(t, ok)
	require.NotNil(t, instDef.Exports["get-environment"])
}

func TestLinker_RelaxedSemverMatching_DifferentMinor(t *testing.T) {
	// Test that relaxed matching still requires same minor version
	l := NewLinker()
	funcType := &FuncType{}

	// Define v0.2.0
	err := l.DefineFunc("test:api@0.2.0", "fn", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	// Enable relaxed matching
	l.SetRelaxedSemverMatching(true)

	// Request v0.3.0 - should NOT match v0.2.0 (different minor)
	_, err = l.MatchImport("test:api@0.3.0/fn")
	require.Error(t, err, "relaxed mode should not match 0.2.0 for 0.3.0 requirement")
}

func TestLinker_RelaxedSemverMatching_Post1_0(t *testing.T) {
	// Test that relaxed matching doesn't affect post-1.0 versions
	l := NewLinker()
	funcType := &FuncType{}

	// Define v1.0.0
	err := l.DefineFunc("test:api@1.0.0", "fn", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	// Enable relaxed matching
	l.SetRelaxedSemverMatching(true)

	// Request v1.0.1 - should NOT match v1.0.0 (relaxed doesn't affect 1.x+)
	_, err = l.MatchImport("test:api@1.0.1/fn")
	require.Error(t, err, "relaxed mode should not affect post-1.0 versions")
}
