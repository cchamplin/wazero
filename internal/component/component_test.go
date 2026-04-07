// internal/component/component_test.go

package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
	"github.com/tetratelabs/wazero/internal/wasm"
)

func TestComponentStructure(t *testing.T) {
	c := &Component{}

	// Verify all slice fields are nil by default
	require.Nil(t, c.CoreModules)
	require.Nil(t, c.Types)
	require.Nil(t, c.Canonicals)
	require.Nil(t, c.Exports)
}

func TestComponentWithCoreModule(t *testing.T) {
	m := &wasm.Module{}
	c := &Component{
		CoreModules: []*wasm.Module{m},
	}

	require.Equal(t, 1, len(c.CoreModules))
	require.Same(t, m, c.CoreModules[0])
}

func TestSortString(t *testing.T) {
	tests := []struct {
		sort     Sort
		expected string
	}{
		{SortFunc, "func"},
		{SortValue, "value"},
		{SortType, "type"},
		{SortComponent, "component"},
		{SortInstance, "instance"},
		{SortCoreSort, "core"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.expected, tt.sort.String())
	}
}

func TestCoreSortString(t *testing.T) {
	tests := []struct {
		sort     CoreSort
		expected string
	}{
		{CoreSortFunc, "func"},
		{CoreSortTable, "table"},
		{CoreSortMemory, "memory"},
		{CoreSortGlobal, "global"},
		{CoreSortType, "type"},
		{CoreSortModule, "module"},
		{CoreSortInstance, "instance"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.expected, tt.sort.String())
	}
}

func TestAliasKindString(t *testing.T) {
	tests := []struct {
		kind     AliasKind
		expected string
	}{
		{AliasKindExport, "export"},
		{AliasKindCoreExport, "core-export"},
		{AliasKindOuter, "outer"},
		{AliasKind(255), "unknown(255)"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.expected, tt.kind.String())
	}
}

func TestAlias_ExportAlias(t *testing.T) {
	a := Alias{
		Kind:        AliasKindExport,
		Sort:        SortFunc,
		InstanceIdx: 1,
		ExportName:  "my-func",
	}
	require.Equal(t, AliasKindExport, a.Kind)
	require.Equal(t, SortFunc, a.Sort)
	require.Equal(t, uint32(1), a.InstanceIdx)
	require.Equal(t, "my-func", a.ExportName)
}

func TestAlias_CoreExportAlias(t *testing.T) {
	a := Alias{
		Kind:        AliasKindCoreExport,
		CoreSort:    CoreSortMemory,
		InstanceIdx: 0,
		ExportName:  "memory",
	}
	require.Equal(t, AliasKindCoreExport, a.Kind)
	require.Equal(t, CoreSortMemory, a.CoreSort)
	require.Equal(t, uint32(0), a.InstanceIdx)
	require.Equal(t, "memory", a.ExportName)
}

func TestAlias_OuterAlias(t *testing.T) {
	a := Alias{
		Kind:       AliasKindOuter,
		Sort:       SortType,
		OuterCount: 1,
		OuterIndex: 5,
	}
	require.Equal(t, AliasKindOuter, a.Kind)
	require.Equal(t, SortType, a.Sort)
	require.Equal(t, uint32(1), a.OuterCount)
	require.Equal(t, uint32(5), a.OuterIndex)
}

func TestComponent_Aliases(t *testing.T) {
	c := &Component{
		Aliases: []Alias{
			{Kind: AliasKindExport, Sort: SortFunc, InstanceIdx: 0, ExportName: "test"},
		},
	}
	require.Equal(t, 1, len(c.Aliases))
	require.Equal(t, AliasKindExport, c.Aliases[0].Kind)
}

func TestImportExternDescKind(t *testing.T) {
	tests := []struct {
		kind     ImportExternDescKind
		expected string
	}{
		{ImportExternDescCoreModule, "core-module"},
		{ImportExternDescFunc, "func"},
		{ImportExternDescValue, "value"},
		{ImportExternDescType, "type"},
		{ImportExternDescComponent, "component"},
		{ImportExternDescInstance, "instance"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.expected, tt.kind.String())
	}
}

func TestImport_FuncImport(t *testing.T) {
	imp := Import{
		Name: "wasi:cli/environment@0.2.0",
		ExternDesc: ImportExternDesc{
			Kind:    ImportExternDescFunc,
			TypeIdx: 5,
		},
	}
	require.Equal(t, "wasi:cli/environment@0.2.0", imp.Name)
	require.Equal(t, ImportExternDescFunc, imp.ExternDesc.Kind)
	require.Equal(t, uint32(5), imp.ExternDesc.TypeIdx)
}

func TestComponent_Imports(t *testing.T) {
	c := &Component{
		Imports: []Import{
			{
				Name: "wasi:io/streams@0.2.0",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescInstance,
					TypeIdx: 3,
				},
			},
		},
	}
	require.Equal(t, 1, len(c.Imports))
	require.Equal(t, "wasi:io/streams@0.2.0", c.Imports[0].Name)
}

func TestCoreInstanceExprKind(t *testing.T) {
	tests := []struct {
		kind     CoreInstanceExprKind
		expected string
	}{
		{CoreInstanceExprInstantiate, "instantiate"},
		{CoreInstanceExprInline, "inline"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.expected, tt.kind.String())
	}
}

func TestCoreInstance_Instantiate(t *testing.T) {
	ci := CoreInstance{
		Kind:      CoreInstanceExprInstantiate,
		ModuleIdx: 0,
		Args: []CoreInstantiateArg{
			{Name: "memory", InstanceIdx: 1},
		},
	}
	require.Equal(t, CoreInstanceExprInstantiate, ci.Kind)
	require.Equal(t, uint32(0), ci.ModuleIdx)
	require.Equal(t, 1, len(ci.Args))
}

func TestComponent_CoreInstances(t *testing.T) {
	c := &Component{
		CoreInstances: []CoreInstance{
			{Kind: CoreInstanceExprInstantiate, ModuleIdx: 0},
		},
	}
	require.Equal(t, 1, len(c.CoreInstances))
}

func TestComponentInstanceExprKind(t *testing.T) {
	tests := []struct {
		kind     ComponentInstanceExprKind
		expected string
	}{
		{ComponentInstanceExprInstantiate, "instantiate"},
		{ComponentInstanceExprInline, "inline"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.expected, tt.kind.String())
	}
}

func TestComponentInstance_Instantiate(t *testing.T) {
	ci := ParsedComponentInstance{
		Kind:         ComponentInstanceExprInstantiate,
		ComponentIdx: 0,
		Args: []ComponentInstantiateArg{
			{Name: "streams", Sort: SortInstance, Idx: 1},
		},
	}
	require.Equal(t, ComponentInstanceExprInstantiate, ci.Kind)
	require.Equal(t, uint32(0), ci.ComponentIdx)
	require.Equal(t, 1, len(ci.Args))
}

func TestComponent_ComponentInstances(t *testing.T) {
	c := &Component{
		ComponentInstances: []ParsedComponentInstance{
			{Kind: ComponentInstanceExprInstantiate, ComponentIdx: 0},
		},
	}
	require.Equal(t, 1, len(c.ComponentInstances))
}

func TestComponent_NestedComponents(t *testing.T) {
	c := &Component{
		Components: []*Component{
			{}, // Empty nested component
		},
	}
	require.Equal(t, 1, len(c.Components))
}

func TestNewTypeDefs(t *testing.T) {
	// Verify type definitions exist and can be instantiated
	_ = VariantTypeDef{Cases: []VariantCase{{Name: "a"}}}
	_ = TupleTypeDef{Types: []ValTypeRef{}}
	_ = FlagsTypeDef{Names: []string{"read", "write"}}
	_ = EnumTypeDef{Names: []string{"red", "green", "blue"}}
}

func TestInstanceTypeDef(t *testing.T) {
	// Verify instance type definition can be created
	decl := InstanceDecl{
		Kind: InstanceDeclKindExport,
		Export: &InstanceExport{
			Name: "test",
			Kind: ExportKindFunc,
			Idx:  0,
		},
	}
	instType := InstanceTypeDef{
		Declarations: []InstanceDecl{decl},
	}
	require.Equal(t, 1, len(instType.Declarations))
}

func TestComponent_StartDef(t *testing.T) {
	start := &StartDef{
		FuncIdx:     5,
		ArgValueIdx: []uint32{0, 1},
		ResultCount: 2,
	}

	require.Equal(t, uint32(5), start.FuncIdx)
	require.Equal(t, 2, len(start.ArgValueIdx))
	require.Equal(t, uint32(2), start.ResultCount)
}

func TestComponent_StartField(t *testing.T) {
	// Test that Component struct has Start field
	start := &StartDef{
		FuncIdx:     3,
		ArgValueIdx: []uint32{0},
		ResultCount: 1,
	}
	c := &Component{
		Start: start,
	}

	require.NotNil(t, c.Start)
	require.Equal(t, uint32(3), c.Start.FuncIdx)
	require.Equal(t, 1, len(c.Start.ArgValueIdx))
	require.Equal(t, uint32(1), c.Start.ResultCount)
}
