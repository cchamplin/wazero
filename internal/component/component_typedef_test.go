package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component/types"
)

// TestTypeDefFuncStoresIndex asserts TypeDef.Func is a types.FuncTypeIdx
// (not a *types.TypeFunc pointer), per Session 1 Decision 5 option A.
//
// Spec: definitions.py:88-101 (FuncType shape — function types are
// structural and interned in the canonical bag).
// Wasmtime parallel: crates/environ/src/component/types.rs (canonical bag
// uses indices for cross-type references to avoid dangling pointers).
func TestTypeDefFuncStoresIndex(t *testing.T) {
	td := TypeDef{
		Kind: TypeDefKindFunc,
		Func: types.FuncTypeIdx(5),
	}
	if td.Func != types.FuncTypeIdx(5) {
		t.Fatalf("TypeDef.Func = %v, want 5", td.Func)
	}
}

// TestTypeDefResourceDtorFields asserts TypeDef carries resource
// destructor metadata so bindResourceTypes can wire Dtor without a
// second pass over decoder state. Design lines 1192-1210.
//
// Spec: definitions.py:351-361 (ResourceType {dtor, dtor_async, dtor_callback}).
func TestTypeDefResourceDtorFields(t *testing.T) {
	dtorIdx := uint32(7)
	callbackIdx := uint32(9)
	td := TypeDef{
		Kind:                 TypeDefKindResource,
		Resource:             types.ResourceTableIdx(2),
		ResourceDtor:         &dtorIdx,
		ResourceDtorAsync:    true,
		ResourceDtorCallback: &callbackIdx,
	}
	if td.ResourceDtor == nil || *td.ResourceDtor != 7 {
		t.Fatalf("ResourceDtor = %v, want 7", td.ResourceDtor)
	}
	if !td.ResourceDtorAsync {
		t.Fatalf("ResourceDtorAsync = false, want true")
	}
	if td.ResourceDtorCallback == nil || *td.ResourceDtorCallback != 9 {
		t.Fatalf("ResourceDtorCallback = %v, want 9", td.ResourceDtorCallback)
	}
}

// TestComponentTypeDefsField asserts Component.TypeDefs exists as an
// accessible []TypeDef slice — the single source of truth for type-section
// slot → canonical-bag index resolution.
//
// No counterpart (justified): this is a wazero engineering convenience to
// carry per-slot type kind through the decoder → linker boundary. The
// spec's type section is a linear slot sequence; wazero models it as a
// slice alongside the canonical *types.ComponentTypes bag.
func TestComponentTypeDefsField(t *testing.T) {
	c := &Component{
		TypeDefs: []TypeDef{
			{Kind: TypeDefKindFunc, Func: types.FuncTypeIdx(0)},
			{Kind: TypeDefKindResource, Resource: types.ResourceTableIdx(0)},
		},
	}
	if len(c.TypeDefs) != 2 {
		t.Fatalf("len(c.TypeDefs) = %d, want 2", len(c.TypeDefs))
	}
	if c.TypeDefs[0].Kind != TypeDefKindFunc {
		t.Fatalf("TypeDefs[0].Kind = %v, want TypeDefKindFunc", c.TypeDefs[0].Kind)
	}
	if c.TypeDefs[1].Kind != TypeDefKindResource {
		t.Fatalf("TypeDefs[1].Kind = %v, want TypeDefKindResource", c.TypeDefs[1].Kind)
	}
}

// TestComponentResolveTypeDefWalksAlias asserts the helper walks
// transitive alias chains to reach a concrete TypeDef, mirroring
// wasmparser::Validator.component_any_type_at(typeidx) which
// transparently follows alias chains at use sites.
//
// Spec: Binary.md:263-265 ("In the (eq i) case, the new type index
// is effectively an alias to type i").
// Wasmtime: crates/environ/src/component/translate.rs:796-801
// (translator calls validator.types(0).component_any_type_at(type_index)
// at every canon.lift use site, which walks alias chains for free).
func TestComponentResolveTypeDefWalksAlias(t *testing.T) {
	c := &Component{
		TypeDefs: []TypeDef{
			// typeidx 0 — concrete func
			{Kind: TypeDefKindFunc, Func: types.FuncTypeIdx(0)},
			// typeidx 1 — outer alias pointing at typeidx 0
			{
				Kind: TypeDefKindAlias,
				Alias: &AliasTarget{
					IsExport:   false,
					OuterCount: 0,
					OuterIndex: 0,
				},
			},
			// typeidx 2 — outer alias pointing at typeidx 1 (alias to alias)
			{
				Kind: TypeDefKindAlias,
				Alias: &AliasTarget{
					IsExport:   false,
					OuterCount: 0,
					OuterIndex: 1,
				},
			},
		},
	}

	td0, idx0, err := c.ResolveTypeDef(0)
	if err != nil {
		t.Fatalf("ResolveTypeDef(0) error: %v", err)
	}
	if idx0 != 0 {
		t.Fatalf("ResolveTypeDef(0) idx = %d, want 0", idx0)
	}
	if td0.Kind != TypeDefKindFunc {
		t.Fatalf("ResolveTypeDef(0).Kind = %v, want TypeDefKindFunc", td0.Kind)
	}

	td1, idx1, err := c.ResolveTypeDef(1)
	if err != nil {
		t.Fatalf("ResolveTypeDef(1) error: %v", err)
	}
	if idx1 != 0 {
		t.Fatalf("ResolveTypeDef(1) idx = %d, want 0", idx1)
	}
	if td1.Kind != TypeDefKindFunc {
		t.Fatalf("ResolveTypeDef(1).Kind = %v, want TypeDefKindFunc", td1.Kind)
	}

	td2, idx2, err := c.ResolveTypeDef(2)
	if err != nil {
		t.Fatalf("ResolveTypeDef(2) error: %v", err)
	}
	if idx2 != 0 {
		t.Fatalf("ResolveTypeDef(2) idx = %d, want 0", idx2)
	}
	if td2.Kind != TypeDefKindFunc {
		t.Fatalf("ResolveTypeDef(2).Kind = %v, want TypeDefKindFunc", td2.Kind)
	}

	// All three must resolve to the same underlying *TypeDef (the
	// concrete func slot at index 0).
	if td0 != td1 || td1 != td2 {
		t.Fatalf("ResolveTypeDef did not return identical pointers for chained aliases: td0=%p td1=%p td2=%p", td0, td1, td2)
	}
}
