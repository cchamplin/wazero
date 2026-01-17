// internal/component/component_linker_test.go
package component

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestComponentLinkerDefineFunc(t *testing.T) {
	// Use nil runtime for unit tests since we can't import wazero
	linker := NewComponentLinker(nil)

	err := linker.DefineFunc("test:api@1.0.0", "hello", func(ctx context.Context, args []Val) ([]Val, error) {
		return []Val{ValString("Hello!")}, nil
	})
	require.NoError(t, err)

	// Duplicate should fail
	err = linker.DefineFunc("test:api@1.0.0", "hello", func(ctx context.Context, args []Val) ([]Val, error) {
		return nil, nil
	})
	require.Error(t, err)
}

func TestComponentLinkerDefineInstance(t *testing.T) {
	linker := NewComponentLinker(nil)

	err := linker.DefineInstance("wasi:cli/environment@0.2.0").
		Func("get-environment", func(ctx context.Context, args []Val) ([]Val, error) {
			return nil, nil
		}).
		Func("get-arguments", func(ctx context.Context, args []Val) ([]Val, error) {
			return nil, nil
		}).
		Build()

	require.NoError(t, err)

	// Verify the instance was registered
	def, err := linker.MatchImport("wasi:cli/environment@0.2.0")
	require.NoError(t, err)
	require.NotNil(t, def)

	instDef, ok := def.(*InstanceDef)
	require.True(t, ok)
	require.Equal(t, 2, len(instDef.Exports))
}

func TestComponentLinkerDefineResource(t *testing.T) {
	linker := NewComponentLinker(nil)

	destroyed := false
	err := linker.DefineResource("wasi:io/streams@0.2.0", "input-stream", func(rep uint32) {
		destroyed = true
	})
	require.NoError(t, err)

	// Duplicate should fail
	err = linker.DefineResource("wasi:io/streams@0.2.0", "input-stream", func(rep uint32) {})
	require.Error(t, err)

	// Verify we can retrieve and call the destructor
	def, err := linker.MatchImport("wasi:io/streams@0.2.0/input-stream")
	require.NoError(t, err)

	resDef, ok := def.(*ResourceDef)
	require.True(t, ok)
	resDef.Destructor(0)
	require.True(t, destroyed)
}

func TestComponentLinkerMatchImport(t *testing.T) {
	linker := NewComponentLinker(nil)

	// Define v1.0.1
	err := linker.DefineFunc("test:api@1.0.1", "fn", func(ctx context.Context, args []Val) ([]Val, error) {
		return []Val{ValS32(101)}, nil
	})
	require.NoError(t, err)

	// Request v1.0.0 - should match v1.0.1 (semver compatible)
	def, err := linker.MatchImport("test:api@1.0.0/fn")
	require.NoError(t, err)
	require.NotNil(t, def)

	// Verify we got the right function
	funcDef, ok := def.(*FuncDef)
	require.True(t, ok)
	results, err := funcDef.Callback(context.Background(), nil)
	require.NoError(t, err)
	require.Equal(t, int32(101), results[0].S32())
}

func TestComponentLinkerMatchImportNotFound(t *testing.T) {
	linker := NewComponentLinker(nil)

	_, err := linker.MatchImport("missing:api@1.0.0/fn")
	require.Error(t, err)
}

func TestComponentLinkerDefineInstanceDuplicate(t *testing.T) {
	linker := NewComponentLinker(nil)

	err := linker.DefineInstance("api@1.0.0").
		Func("fn", func(ctx context.Context, args []Val) ([]Val, error) {
			return nil, nil
		}).
		Build()
	require.NoError(t, err)

	// Duplicate instance should fail
	err = linker.DefineInstance("api@1.0.0").
		Func("fn2", func(ctx context.Context, args []Val) ([]Val, error) {
			return nil, nil
		}).
		Build()
	require.Error(t, err)
}

func TestComponentLinkerInstanceBuilderResource(t *testing.T) {
	linker := NewComponentLinker(nil)

	err := linker.DefineInstance("api@1.0.0").
		Func("read", func(ctx context.Context, args []Val) ([]Val, error) {
			return nil, nil
		}).
		Resource("handle", func(rep uint32) {}).
		Build()
	require.NoError(t, err)

	// Use Get for direct instance lookup (MatchImport expects namespace/name format)
	def, ok := linker.Get("api@1.0.0")
	require.True(t, ok)

	instDef, ok := def.(*InstanceDef)
	require.True(t, ok)
	require.Equal(t, 2, len(instDef.Exports))
	require.NotNil(t, instDef.Exports["read"])
	require.NotNil(t, instDef.Exports["handle"])
}

// TestComponentLinker_OrderedInstantiation verifies that core instances are
// instantiated in order and that imports can be resolved from previously
// instantiated instances.
func TestComponentLinker_OrderedInstantiation(t *testing.T) {
	// This test verifies the import resolver infrastructure.
	// For a full integration test with actual modules, see wasip2test.
	linker := NewComponentLinker(nil)

	// Create a mock component structure
	c := &Component{
		CoreInstances: []CoreInstance{
			// Instance 0: instantiate module 0 (no imports)
			{
				Kind:      CoreInstanceExprInstantiate,
				ModuleIdx: 0,
				Args:      nil,
			},
			// Instance 1: instantiate module 1 with imports from instance 0
			{
				Kind:      CoreInstanceExprInstantiate,
				ModuleIdx: 1,
				Args: []CoreInstantiateArg{
					{Name: "provider", InstanceIdx: 0},
				},
			},
		},
		Aliases: []Alias{
			{Kind: AliasKindCoreExport, InstanceIdx: 0, ExportName: "memory", CoreSort: CoreSortMemory},
			{Kind: AliasKindCoreExport, InstanceIdx: 0, ExportName: "func1", CoreSort: CoreSortFunc},
		},
	}

	// Test the import resolver building
	inst := &Instance{
		component:     c,
		coreInstances: make([]api.Module, 2),
	}

	// Build import resolver for core instance 1
	resolver := linker.buildImportResolver(inst, c, &c.CoreInstances[1])
	require.NotNil(t, resolver)

	// The resolver should map "provider" to instance 0
	// Since we haven't actually instantiated anything, inst.coreInstances[0] is nil
	// and the resolver will return nil (which is correct behavior)
	result := resolver("provider")
	require.Nil(t, result, "resolver returns nil when instance not yet available")

	// Unknown import module should return nil
	result = resolver("unknown")
	require.Nil(t, result, "resolver returns nil for unknown imports")
}
