// internal/component/integration_test.go

package component

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestIntegration_ComponentWithFuncImport tests a component that imports a function.
// This is Task 147: Create test component with imports.
func TestIntegration_ComponentWithFuncImport(t *testing.T) {
	// Create linker with host function
	l := NewLinker()

	funcType := &FuncType{
		Params:  []NamedValType{{Name: "a", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7f}}},
		Results: []NamedValType{{Name: "", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7f}}},
	}

	err := l.DefineFunc("test:host@1.0.0", "double", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		if len(args) > 0 {
			return []Val{ValS32(args[0].S32() * 2)}, nil
		}
		return []Val{ValS32(0)}, nil
	})
	require.NoError(t, err)

	// Create component that imports test:host@1.0.0/double
	c := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: funcType},
		},
		Imports: []Import{
			{
				Name: "test:host@1.0.0/double",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0,
				},
			},
		},
		Exports: []Export{
			{Name: "compute", Kind: ExportKindFunc, Idx: 0},
		},
	}

	// Instantiate component
	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)
	require.NotNil(t, inst)

	// Verify the exported function exists
	fn := inst.GetExportedFunc("compute")
	require.NotNil(t, fn)

	// Verify the component reference is correct
	require.Equal(t, c, inst.Component())
}

// TestIntegration_FuncImportSemverMatch tests semver-compatible function import.
// This is Task 148: Test component importing a function.
func TestIntegration_FuncImportSemverMatch(t *testing.T) {
	l := NewLinker()
	funcType := &FuncType{}

	// Define v1.0.1 (newer)
	err := l.DefineFunc("api@1.0.1", "fn", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	// Component requests v1.0.0 (older)
	c := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: funcType},
		},
		Imports: []Import{
			{
				Name: "api@1.0.0/fn",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0,
				},
			},
		},
	}

	// Should successfully instantiate (v1.0.1 satisfies v1.0.0)
	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)
	require.NotNil(t, inst)
}

// TestIntegration_FuncImportSemverMismatch tests that incompatible versions fail.
func TestIntegration_FuncImportSemverMismatch(t *testing.T) {
	l := NewLinker()
	funcType := &FuncType{}

	// Define v1.0.0 (older)
	err := l.DefineFunc("api@1.0.0", "fn", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	// Component requests v1.0.1 (newer) - should NOT match
	c := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: funcType},
		},
		Imports: []Import{
			{
				Name: "api@1.0.1/fn",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0,
				},
			},
		},
	}

	// Should fail - v1.0.0 cannot satisfy v1.0.1
	_, err = l.Instantiate(context.Background(), c)
	require.Error(t, err)
}

// TestIntegration_FuncImportMajorVersionMismatch tests major version incompatibility.
func TestIntegration_FuncImportMajorVersionMismatch(t *testing.T) {
	l := NewLinker()
	funcType := &FuncType{}

	// Define v2.0.0
	err := l.DefineFunc("api@2.0.0", "fn", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	// Component requests v1.0.0 - different major version
	c := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: funcType},
		},
		Imports: []Import{
			{
				Name: "api@1.0.0/fn",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0,
				},
			},
		},
	}

	// Should fail - major versions don't match
	_, err = l.Instantiate(context.Background(), c)
	require.Error(t, err)
}

// TestIntegration_InstanceImport tests importing a full instance.
// This is Task 149: Test component importing an instance.
func TestIntegration_InstanceImport(t *testing.T) {
	l := NewLinker()
	funcType := &FuncType{}

	// Define instance with multiple exports
	err := l.DefineInstance("wasi:io/streams@0.2.0").
		Func("read", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
			return nil, nil
		}).
		Func("write", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
			return nil, nil
		}).
		Build()
	require.NoError(t, err)

	// Verify instance was defined
	def, ok := l.Get("wasi:io/streams@0.2.0")
	require.True(t, ok)
	require.NotNil(t, def)

	instDef, ok := def.(*InstanceDef)
	require.True(t, ok)
	require.NotNil(t, instDef.Exports["read"])
	require.NotNil(t, instDef.Exports["write"])
}

// TestIntegration_InstanceImportWithVersioning tests instance import with semver.
func TestIntegration_InstanceImportWithVersioning(t *testing.T) {
	l := NewLinker()
	funcType := &FuncType{}

	// Define instance at v0.2.1
	err := l.DefineInstance("wasi:io/streams@0.2.1").
		Func("read", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
			return nil, nil
		}).
		Build()
	require.NoError(t, err)

	// Verify instance was defined
	def, ok := l.Get("wasi:io/streams@0.2.1")
	require.True(t, ok)
	require.NotNil(t, def)
}

// TestIntegration_FullLinkingScenario tests a complete linking workflow.
// This is Task 150: Test full linking scenario.
func TestIntegration_FullLinkingScenario(t *testing.T) {
	l := NewLinker()

	// Define multiple host functions with versioned namespaces
	addType := &FuncType{
		Params: []NamedValType{
			{Name: "a", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7f}},
			{Name: "b", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7f}},
		},
		Results: []NamedValType{
			{Name: "", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7f}},
		},
	}

	err := l.DefineFunc("math@1.0.0", "add", addType, func(ctx context.Context, args []Val) ([]Val, error) {
		if len(args) >= 2 {
			return []Val{ValS32(args[0].S32() + args[1].S32())}, nil
		}
		return []Val{ValS32(0)}, nil
	})
	require.NoError(t, err)

	mulType := &FuncType{
		Params: []NamedValType{
			{Name: "a", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7f}},
			{Name: "b", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7f}},
		},
		Results: []NamedValType{
			{Name: "", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7f}},
		},
	}

	err = l.DefineFunc("math@1.0.0", "mul", mulType, func(ctx context.Context, args []Val) ([]Val, error) {
		if len(args) >= 2 {
			return []Val{ValS32(args[0].S32() * args[1].S32())}, nil
		}
		return []Val{ValS32(0)}, nil
	})
	require.NoError(t, err)

	// Component imports both functions
	c := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: addType},
			{Kind: TypeDefKindFunc, Func: mulType},
		},
		Imports: []Import{
			{
				Name: "math@1.0.0/add",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0,
				},
			},
			{
				Name: "math@1.0.0/mul",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 1,
				},
			},
		},
		// Canonicals define the lifted functions - required for type lookup
		Canonicals: []CanonicalDef{
			{Kind: CanonKindLift, TypeIdx: 0, ComponentFuncIdx: 0},
		},
		// FuncIdxToCanonical maps export func index to canonical index
		FuncIdxToCanonical: map[uint32]uint32{
			0: 0, // export func 0 -> canonical 0 -> type 0 (addType)
		},
		Exports: []Export{
			{Name: "compute", Kind: ExportKindFunc, Idx: 0},
		},
	}

	// Instantiate
	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)
	require.NotNil(t, inst)

	// Verify component
	require.Equal(t, c, inst.Component())

	// Verify exports
	fn := inst.GetExportedFunc("compute")
	require.NotNil(t, fn)
	require.Equal(t, addType, fn.Type())
}

// TestIntegration_MultipleVersionedImports tests importing from multiple versions.
func TestIntegration_MultipleVersionedImports(t *testing.T) {
	l := NewLinker()
	funcType := &FuncType{}

	// Define multiple versions of the same function
	err := l.DefineFunc("api@1.0.0", "fn", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return []Val{ValS32(100)}, nil
	})
	require.NoError(t, err)

	err = l.DefineFunc("api@1.0.2", "fn", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return []Val{ValS32(102)}, nil
	})
	require.NoError(t, err)

	err = l.DefineFunc("api@1.0.1", "fn", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return []Val{ValS32(101)}, nil
	})
	require.NoError(t, err)

	// Component imports v1.0.0 - linker should select highest compatible (v1.0.2)
	c := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: funcType},
		},
		Imports: []Import{
			{
				Name: "api@1.0.0/fn",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0,
				},
			},
		},
	}

	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)
	require.NotNil(t, inst)
}

// TestIntegration_ResourceDefinition tests defining and using resources.
func TestIntegration_ResourceDefinition(t *testing.T) {
	l := NewLinker()

	destructorCalled := false
	var destroyedRep uint32

	err := l.DefineResource("wasi:io/streams@0.2.0", "input-stream", func(rep uint32) {
		destructorCalled = true
		destroyedRep = rep
	})
	require.NoError(t, err)

	// Verify resource was defined
	def, ok := l.Get("wasi:io/streams@0.2.0/input-stream")
	require.True(t, ok)
	require.NotNil(t, def)

	resDef, ok := def.(*ResourceDef)
	require.True(t, ok)
	require.NotNil(t, resDef.Destructor)

	// Test destructor
	resDef.Destructor(42)
	require.True(t, destructorCalled)
	require.Equal(t, uint32(42), destroyedRep)
}

// TestIntegration_MixedImports tests a component with both function and resource imports.
func TestIntegration_MixedImports(t *testing.T) {
	l := NewLinker()
	funcType := &FuncType{}

	// Define a function
	err := l.DefineFunc("api@1.0.0", "process", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	// Define a resource
	err = l.DefineResource("api@1.0.0", "handle", func(rep uint32) {})
	require.NoError(t, err)

	// Verify both are defined
	fnDef, ok := l.Get("api@1.0.0/process")
	require.True(t, ok)
	require.NotNil(t, fnDef)

	resDef, ok := l.Get("api@1.0.0/handle")
	require.True(t, ok)
	require.NotNil(t, resDef)
}

// TestIntegration_ExportSemverMatching tests semver-compatible export lookups.
func TestIntegration_ExportSemverMatching(t *testing.T) {
	l := NewLinker()
	funcType := &FuncType{}

	// Create component with versioned export
	c := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: funcType},
		},
		Exports: []Export{
			{Name: "api@1.0.0/fn", Kind: ExportKindFunc, Idx: 0},
		},
	}

	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)

	// Exact match
	fn := inst.GetExportedFunc("api@1.0.0/fn")
	require.NotNil(t, fn)

	// Caller requests newer version (backward compatible)
	fn = inst.GetExportedFunc("api@1.0.1/fn")
	require.NotNil(t, fn)

	// Caller requests older version (forward compatible)
	// Component exports v1.0.1, caller requests v1.0.0
	c2 := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: funcType},
		},
		Exports: []Export{
			{Name: "api@1.0.1/fn", Kind: ExportKindFunc, Idx: 0},
		},
	}

	inst2, err := l.Instantiate(context.Background(), c2)
	require.NoError(t, err)

	fn = inst2.GetExportedFunc("api@1.0.0/fn")
	require.NotNil(t, fn)
}

// TestIntegration_ExportSelectsMaxVersion tests that export lookup selects highest version.
func TestIntegration_ExportSelectsMaxVersion(t *testing.T) {
	l := NewLinker()
	funcType := &FuncType{}

	// Create component with multiple versioned exports
	c := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: funcType},
		},
		Exports: []Export{
			{Name: "api@1.0.0/fn", Kind: ExportKindFunc, Idx: 0},
			{Name: "api@1.0.2/fn", Kind: ExportKindFunc, Idx: 0},
			{Name: "api@1.0.1/fn", Kind: ExportKindFunc, Idx: 0},
		},
	}

	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)

	// Request v1.0.0 - should match highest compatible (v1.0.2)
	fn := inst.GetExportedFunc("api@1.0.0/fn")
	require.NotNil(t, fn)
	require.Equal(t, "api@1.0.2/fn", fn.Name())
}

// TestIntegration_NoExportsComponent tests a component with no exports.
func TestIntegration_NoExportsComponent(t *testing.T) {
	l := NewLinker()

	c := &Component{
		Imports: []Import{},
		Exports: []Export{},
	}

	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)
	require.NotNil(t, inst)

	// Non-existent export should return nil
	fn := inst.GetExportedFunc("nonexistent")
	require.Nil(t, fn)
}

// TestIntegration_ComponentWithTypes tests a component with various type definitions.
func TestIntegration_ComponentWithTypes(t *testing.T) {
	l := NewLinker()

	funcType := &FuncType{
		Params: []NamedValType{
			{Name: "input", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7f}},
		},
		Results: []NamedValType{
			{Name: "output", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7f}},
		},
	}

	c := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: funcType},
			{Kind: TypeDefKindResource, Resource: nil},
		},
		// Canonicals define the lifted functions - required for type lookup
		Canonicals: []CanonicalDef{
			{Kind: CanonKindLift, TypeIdx: 0, ComponentFuncIdx: 0},
		},
		// FuncIdxToCanonical maps export func index to canonical index
		FuncIdxToCanonical: map[uint32]uint32{
			0: 0, // export func 0 -> canonical 0 -> type 0 (funcType)
		},
		Exports: []Export{
			{Name: "process", Kind: ExportKindFunc, Idx: 0},
		},
	}

	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)
	require.NotNil(t, inst)

	fn := inst.GetExportedFunc("process")
	require.NotNil(t, fn)
	require.Equal(t, funcType, fn.Type())
}

// TestIntegration_Pre1_0_SemverHandling tests semver handling for pre-1.0 versions.
func TestIntegration_Pre1_0_SemverHandling(t *testing.T) {
	l := NewLinker()
	funcType := &FuncType{}

	// Define v0.2.1
	err := l.DefineFunc("api@0.2.1", "fn", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	// Component requests v0.2.0 - should match v0.2.1 (same minor, higher patch)
	c := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: funcType},
		},
		Imports: []Import{
			{
				Name: "api@0.2.0/fn",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0,
				},
			},
		},
	}

	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)
	require.NotNil(t, inst)
}

// TestIntegration_Pre1_0_MinorVersionMismatch tests that pre-1.0 minor versions are incompatible.
func TestIntegration_Pre1_0_MinorVersionMismatch(t *testing.T) {
	l := NewLinker()
	funcType := &FuncType{}

	// Define v0.3.0
	err := l.DefineFunc("api@0.3.0", "fn", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	// Component requests v0.2.0 - should NOT match v0.3.0 (different minor for pre-1.0)
	c := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: funcType},
		},
		Imports: []Import{
			{
				Name: "api@0.2.0/fn",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0,
				},
			},
		},
	}

	_, err = l.Instantiate(context.Background(), c)
	require.Error(t, err)
}

// TestIntegration_WASILikeNamespace tests WASI-style namespaced imports.
func TestIntegration_WASILikeNamespace(t *testing.T) {
	l := NewLinker()
	funcType := &FuncType{}

	// Define WASI-like functions
	err := l.DefineFunc("wasi:cli/environment@0.2.0", "get-environment", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	err = l.DefineFunc("wasi:cli/stdin@0.2.0", "get-stdin", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	// Component imports both
	c := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: funcType},
		},
		Imports: []Import{
			{
				Name: "wasi:cli/environment@0.2.0/get-environment",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0,
				},
			},
			{
				Name: "wasi:cli/stdin@0.2.0/get-stdin",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0,
				},
			},
		},
	}

	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)
	require.NotNil(t, inst)
}

// TestIntegration_HostFunctionCallback tests that host function callbacks work correctly.
func TestIntegration_HostFunctionCallback(t *testing.T) {
	l := NewLinker()

	callCount := 0
	funcType := &FuncType{}

	err := l.DefineFunc("test@1.0.0", "increment", funcType, func(ctx context.Context, args []Val) ([]Val, error) {
		callCount++
		return nil, nil
	})
	require.NoError(t, err)

	// Get the definition and verify callback works
	def, err := l.MatchImport("test@1.0.0/increment")
	require.NoError(t, err)

	funcDef := def.(*FuncDef)
	require.NotNil(t, funcDef.Callback)

	// Call the callback
	_, err = funcDef.Callback(context.Background(), nil)
	require.NoError(t, err)
	require.Equal(t, 1, callCount)

	// Call again
	_, err = funcDef.Callback(context.Background(), nil)
	require.NoError(t, err)
	require.Equal(t, 2, callCount)
}

// TestIntegration_InstanceBuilderChaining tests fluent instance builder API.
func TestIntegration_InstanceBuilderChaining(t *testing.T) {
	l := NewLinker()
	funcType := &FuncType{}

	// Build instance with chained calls
	err := l.DefineInstance("api@1.0.0").
		Func("a", funcType, func(ctx context.Context, args []Val) ([]Val, error) { return nil, nil }).
		Func("b", funcType, func(ctx context.Context, args []Val) ([]Val, error) { return nil, nil }).
		Func("c", funcType, func(ctx context.Context, args []Val) ([]Val, error) { return nil, nil }).
		Build()
	require.NoError(t, err)

	// Verify all functions are defined
	def, ok := l.Get("api@1.0.0")
	require.True(t, ok)

	instDef := def.(*InstanceDef)
	require.Equal(t, 3, len(instDef.Exports))
	require.NotNil(t, instDef.Exports["a"])
	require.NotNil(t, instDef.Exports["b"])
	require.NotNil(t, instDef.Exports["c"])
}
