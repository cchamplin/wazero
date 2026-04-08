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
