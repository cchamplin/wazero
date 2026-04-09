package component

import (
	"testing"
)

// wasmtime reference: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/types.rs:1129-1141
// (ComponentItem::from_export resolves Export::Type(idx) via Self::from(engine, idx, ty))
// and debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/instance.rs:260-273
// (get_resource matches Export::Type(TypeDef::Resource(id)) to produce a ResourceType).

// TestResolveExportTypeAlias_DirectType verifies that resolveExportTypeAlias
// correctly traces through parent.instanceSpace[alias.InstanceIdx] to the
// source component's exports and returns the resolved TypeDef.
func TestResolveExportTypeAlias_DirectType(t *testing.T) {
	// No spec counterpart (justified): type alias resolution is a wazero
	// linker-layer concern not covered by canonical-abi definitions.py;
	// equivalent logic in wasmtime lives in
	// runtime/component/types.rs:1129-1141 (ComponentItem::from_export)
	// and runtime/component/instance.rs:260-273 (get_resource).

	// Build source component with two type defs and one type export.
	srcComp := &Component{
		TypeDefs: []TypeDef{
			{Kind: TypeDefKindFunc},
			{Kind: TypeDefKindResource, Resource: 0},
		},
		Exports: []Export{
			{Name: "my-resource", Kind: ExportKindType, Idx: 1},
		},
	}
	srcInst := NewInstance(srcComp, 0, nil)

	// Build parent with the source instance in instanceSpace.
	parentComp := &Component{}
	parent := NewInstance(parentComp, 1, nil)
	parent.instanceSpace = []*Instance{srcInst}

	alias := &Alias{
		Kind:        AliasKindExport,
		Sort:        SortType,
		InstanceIdx: 0,
		ExportName:  "my-resource",
	}

	l := NewComponentLinker(nil)
	got := l.resolveExportTypeAlias(parent, parentComp, alias)
	if got == nil {
		t.Fatal("expected non-nil TypeDef, got nil")
	}
	if got.Kind != TypeDefKindResource {
		t.Fatalf("expected TypeDefKindResource, got %v", got.Kind)
	}
}

// TestResolveExportTypeAlias_NotFound verifies that resolveExportTypeAlias
// returns nil when no export matches alias.ExportName.
func TestResolveExportTypeAlias_NotFound(t *testing.T) {
	// No spec counterpart (justified): type alias resolution is a wazero
	// linker-layer concern not covered by canonical-abi definitions.py;
	// equivalent logic in wasmtime lives in
	// runtime/component/types.rs:1129-1141 (ComponentItem::from_export)
	// and runtime/component/instance.rs:260-273 (get_resource).

	srcComp := &Component{
		TypeDefs: []TypeDef{
			{Kind: TypeDefKindFunc},
		},
		Exports: []Export{
			{Name: "other-export", Kind: ExportKindType, Idx: 0},
		},
	}
	srcInst := NewInstance(srcComp, 0, nil)

	parentComp := &Component{}
	parent := NewInstance(parentComp, 1, nil)
	parent.instanceSpace = []*Instance{srcInst}

	alias := &Alias{
		Kind:        AliasKindExport,
		Sort:        SortType,
		InstanceIdx: 0,
		ExportName:  "does-not-exist",
	}

	l := NewComponentLinker(nil)
	got := l.resolveExportTypeAlias(parent, parentComp, alias)
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

// TestResolveExportTypeAlias_NilParent verifies that resolveExportTypeAlias
// returns nil without panicking when parent.instanceSpace is nil or empty.
func TestResolveExportTypeAlias_NilParent(t *testing.T) {
	// No spec counterpart (justified): type alias resolution is a wazero
	// linker-layer concern not covered by canonical-abi definitions.py;
	// equivalent logic in wasmtime lives in
	// runtime/component/types.rs:1129-1141 (ComponentItem::from_export)
	// and runtime/component/instance.rs:260-273 (get_resource).

	parentComp := &Component{}
	parent := NewInstance(parentComp, 0, nil)
	// instanceSpace is nil (not populated).

	alias := &Alias{
		Kind:        AliasKindExport,
		Sort:        SortType,
		InstanceIdx: 0,
		ExportName:  "some-type",
	}

	l := NewComponentLinker(nil)
	got := l.resolveExportTypeAlias(parent, parentComp, alias)
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

// TestResolveExportTypeAlias_WrongExportKind verifies that resolveExportTypeAlias
// returns nil when the matching export has a non-type kind (e.g., ExportKindFunc).
func TestResolveExportTypeAlias_WrongExportKind(t *testing.T) {
	// No spec counterpart (justified): type alias resolution is a wazero
	// linker-layer concern not covered by canonical-abi definitions.py;
	// equivalent logic in wasmtime lives in
	// runtime/component/types.rs:1129-1141 (ComponentItem::from_export)
	// and runtime/component/instance.rs:260-273 (get_resource).

	srcComp := &Component{
		TypeDefs: []TypeDef{
			{Kind: TypeDefKindResource, Resource: 0},
		},
		Exports: []Export{
			{Name: "my-func", Kind: ExportKindFunc, Idx: 0},
		},
	}
	srcInst := NewInstance(srcComp, 0, nil)

	parentComp := &Component{}
	parent := NewInstance(parentComp, 1, nil)
	parent.instanceSpace = []*Instance{srcInst}

	alias := &Alias{
		Kind:        AliasKindExport,
		Sort:        SortType,
		InstanceIdx: 0,
		ExportName:  "my-func",
	}

	l := NewComponentLinker(nil)
	got := l.resolveExportTypeAlias(parent, parentComp, alias)
	if got != nil {
		t.Fatalf("expected nil for wrong export kind, got %+v", got)
	}
}
