// internal/component/outer_alias_test.go
package component

import (
	"testing"
)

func TestResolveOuterAlias_Type(t *testing.T) {
	// Create parent with a type
	parent := &Instance{}
	parentType := &TypeDef{Kind: TypeDefKindFunc, Func: &FuncType{}}
	parent.AddTypeToSpace(parentType)

	// Create child
	child := &Instance{}
	parent.AddChild(child)

	// Outer alias: depth=1, index=0 (parent's type at index 0)
	alias := &Alias{
		Kind:       AliasKindOuter,
		Sort:       SortType,
		OuterCount: 1,
		OuterIndex: 0,
	}

	resolved, err := ResolveOuterAlias(child, alias)
	if err != nil {
		t.Fatalf("ResolveOuterAlias failed: %v", err)
	}

	resolvedType, ok := resolved.(*TypeDef)
	if !ok {
		t.Fatalf("expected *TypeDef, got %T", resolved)
	}
	if resolvedType != parentType {
		t.Error("resolved type should match parent's type")
	}
}

func TestResolveOuterAlias_TooDeep(t *testing.T) {
	// Create single instance (no parent)
	inst := &Instance{}

	// Try to resolve outer alias with depth > nesting
	alias := &Alias{
		Kind:       AliasKindOuter,
		Sort:       SortType,
		OuterCount: 2, // No grandparent exists
		OuterIndex: 0,
	}

	_, err := ResolveOuterAlias(inst, alias)
	if err == nil {
		t.Error("should fail when outer depth exceeds nesting")
	}
}

func TestResolveOuterAlias_Component(t *testing.T) {
	parent := &Instance{}
	nestedComp := &Component{}
	parent.AddComponentToSpace(nestedComp)

	child := &Instance{}
	parent.AddChild(child)

	alias := &Alias{
		Kind:       AliasKindOuter,
		Sort:       SortComponent,
		OuterCount: 1,
		OuterIndex: 0,
	}

	resolved, err := ResolveOuterAlias(child, alias)
	if err != nil {
		t.Fatalf("ResolveOuterAlias failed: %v", err)
	}

	resolvedComp, ok := resolved.(*Component)
	if !ok {
		t.Fatalf("expected *Component, got %T", resolved)
	}
	if resolvedComp != nestedComp {
		t.Error("resolved component should match parent's component")
	}
}

func TestResolveOuterAlias_NotOuterAlias(t *testing.T) {
	inst := &Instance{}

	// Try to resolve an export alias (not outer)
	alias := &Alias{
		Kind:        AliasKindExport,
		Sort:        SortFunc,
		InstanceIdx: 0,
		ExportName:  "test",
	}

	_, err := ResolveOuterAlias(inst, alias)
	if err == nil {
		t.Error("should fail when alias is not an outer alias")
	}
}

func TestResolveOuterAlias_MutableSort(t *testing.T) {
	parent := &Instance{}
	child := &Instance{}
	parent.AddChild(child)

	// Try to outer-alias a function (mutable, not allowed)
	alias := &Alias{
		Kind:       AliasKindOuter,
		Sort:       SortFunc,
		OuterCount: 1,
		OuterIndex: 0,
	}

	_, err := ResolveOuterAlias(child, alias)
	if err == nil {
		t.Error("should fail when outer-aliasing mutable items (functions)")
	}

	// Try to outer-alias an instance (mutable, not allowed)
	alias = &Alias{
		Kind:       AliasKindOuter,
		Sort:       SortInstance,
		OuterCount: 1,
		OuterIndex: 0,
	}

	_, err = ResolveOuterAlias(child, alias)
	if err == nil {
		t.Error("should fail when outer-aliasing mutable items (instances)")
	}
}

func TestResolveOuterAlias_IndexNotFound(t *testing.T) {
	parent := &Instance{}
	// Parent has no types in type space

	child := &Instance{}
	parent.AddChild(child)

	// Try to resolve type at index 0 (doesn't exist)
	alias := &Alias{
		Kind:       AliasKindOuter,
		Sort:       SortType,
		OuterCount: 1,
		OuterIndex: 0,
	}

	_, err := ResolveOuterAlias(child, alias)
	if err == nil {
		t.Error("should fail when index not found in target scope")
	}
}

func TestResolveOuterAlias_Grandparent(t *testing.T) {
	// Create grandparent -> parent -> child hierarchy
	grandparent := &Instance{}
	grandparentType := &TypeDef{Kind: TypeDefKindInstance, Instance: &InstanceTypeDef{}}
	grandparent.AddTypeToSpace(grandparentType)

	parent := &Instance{}
	grandparent.AddChild(parent)

	child := &Instance{}
	parent.AddChild(child)

	// Outer alias: depth=2, index=0 (grandparent's type at index 0)
	alias := &Alias{
		Kind:       AliasKindOuter,
		Sort:       SortType,
		OuterCount: 2,
		OuterIndex: 0,
	}

	resolved, err := ResolveOuterAlias(child, alias)
	if err != nil {
		t.Fatalf("ResolveOuterAlias failed: %v", err)
	}

	resolvedType, ok := resolved.(*TypeDef)
	if !ok {
		t.Fatalf("expected *TypeDef, got %T", resolved)
	}
	if resolvedType != grandparentType {
		t.Error("resolved type should match grandparent's type")
	}
}

func TestResolveOuterAlias_ValueSort(t *testing.T) {
	parent := &Instance{}
	parent.AddValue(ValS32(42))

	child := &Instance{}
	parent.AddChild(child)

	// Try to outer-alias a value (mutable, not allowed)
	alias := &Alias{
		Kind:       AliasKindOuter,
		Sort:       SortValue,
		OuterCount: 1,
		OuterIndex: 0,
	}

	_, err := ResolveOuterAlias(child, alias)
	if err == nil {
		t.Error("should fail when outer-aliasing mutable items (values)")
	}
}

func TestResolveOuterAlias_ZeroDepth(t *testing.T) {
	inst := &Instance{}
	instType := &TypeDef{Kind: TypeDefKindFunc}
	inst.AddTypeToSpace(instType)

	// Depth 0 should refer to the current instance's scope
	alias := &Alias{
		Kind:       AliasKindOuter,
		Sort:       SortType,
		OuterCount: 0,
		OuterIndex: 0,
	}

	resolved, err := ResolveOuterAlias(inst, alias)
	if err != nil {
		t.Fatalf("ResolveOuterAlias failed: %v", err)
	}

	resolvedType, ok := resolved.(*TypeDef)
	if !ok {
		t.Fatalf("expected *TypeDef, got %T", resolved)
	}
	if resolvedType != instType {
		t.Error("resolved type should match instance's own type")
	}
}

func TestResolveOuterAlias_ComponentNotFound(t *testing.T) {
	parent := &Instance{}
	// Parent has NO components in component space

	child := &Instance{}
	parent.AddChild(child)

	alias := &Alias{
		Kind:       AliasKindOuter,
		Sort:       SortComponent,
		OuterCount: 1,
		OuterIndex: 99, // Out of range
	}

	_, err := ResolveOuterAlias(child, alias)
	if err == nil {
		t.Error("should fail when component index is out of range")
	}
}
