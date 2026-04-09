// internal/component/binary/alias_densify_test.go

package binary

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestDecoderOuterTypeAliasConsumesSlot asserts the decoder appends
// an Alias TypeDef for every outer type alias, preserving the
// component's flat type index space.
//
// Spec: Binary.md:118-122 (outer aliastarget), 263-268 (exportdecl
// prose on alias slot semantics), Explainer.md:326-338 ("the id of
// the alias is bound to the new index added by the alias").
// Wasmtime: crates/wizer/src/component/parse.rs:124-137
// (inc_types() on outer Type alias proves slot consumption).
// Wasm-tools corpus:
// debug-vendored/wasm-tools/tests/cli/component-model/resources.wast:779-796
// (outer count=0 alias-to-self pattern).
func TestDecoderOuterTypeAliasConsumesSlot(t *testing.T) {
	// Component layout:
	//   type section: typeidx 0 = (type (func))
	//   alias section: typeidx 1 = (alias outer 0 0 (type))
	// The outer alias has count=0 (same scope) and idx=0 (points at
	// the preceding func-type slot).
	typeSection := []byte{
		0x01, // 1 type definition
		0x40, // sync functype opcode
		0x00, // 0 params
		0x01, // named results
		0x00, // 0 results
	}
	aliasSection := []byte{
		0x01, // 1 alias
		0x03, // sort: type
		0x02, // aliastarget: outer
		0x00, // outer count 0 (same scope)
		0x00, // outer index 0
	}

	input := append(append(Magic[:], Version[:]...), LayerComponent[:]...)
	input = append(input, byte(SectionIDType))
	input = append(input, byte(len(typeSection)))
	input = append(input, typeSection...)
	input = append(input, byte(SectionIDAlias))
	input = append(input, byte(len(aliasSection)))
	input = append(input, aliasSection...)

	c, err := DecodeComponent(input)
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Equal(t, uint32(2), c.NextTypeIdx, "NextTypeIdx should be 2 after one type + one alias")
	require.Equal(t, 2, len(c.TypeDefs), "TypeDefs must be densified: one entry per NextTypeIdx bump")
	require.Equal(t, component.TypeDefKindFunc, c.TypeDefs[0].Kind)
	require.Equal(t, component.TypeDefKindAlias, c.TypeDefs[1].Kind)
	require.NotNil(t, c.TypeDefs[1].Alias)
	require.False(t, c.TypeDefs[1].Alias.IsExport)
	require.Equal(t, uint32(0), c.TypeDefs[1].Alias.OuterCount)
	require.Equal(t, uint32(0), c.TypeDefs[1].Alias.OuterIndex)
}

// TestDecoderExportTypeAliasConsumesSlot asserts the decoder appends
// an Alias TypeDef for an export type alias, preserving the
// component's flat type index space.
//
// Spec: Binary.md:119-126 (export aliastarget 0x00 i:<instanceidx>
// n:<name>), Explainer.md:326-338.
// Wasmtime: tests/all/component_model/bindgen.rs:424
// `(alias export $i "x" (type $x))`.
// Wizer parser: crates/wizer/src/component/parse.rs:118-122
// (ComponentAlias::InstanceExport with kind Type routes to inc_types).
func TestDecoderExportTypeAliasConsumesSlot(t *testing.T) {
	// Component layout:
	//   type section: typeidx 0 = (type (func))
	//     — placeholder slot so the alias's typeidx is 1, not 0.
	//   alias section: typeidx 1 = (alias export 0 "T" (type))
	// Semantic validity of the instance index is not checked by the
	// decoder (decodeAliasSection does not dereference InstanceIdx);
	// this test exercises the decoder's slot-densification behavior
	// in isolation from the linker / type checker.
	typeSection := []byte{
		0x01, // 1 type definition
		0x40, // sync functype
		0x00, // 0 params
		0x01, // named results
		0x00, // 0 results
	}
	aliasSection := []byte{
		0x01,      // 1 alias
		0x03,      // sort: type
		0x00,      // aliastarget: export
		0x00,      // instance index 0
		0x01, 'T', // export name "T"
	}

	input := append(append(Magic[:], Version[:]...), LayerComponent[:]...)
	input = append(input, byte(SectionIDType))
	input = append(input, byte(len(typeSection)))
	input = append(input, typeSection...)
	input = append(input, byte(SectionIDAlias))
	input = append(input, byte(len(aliasSection)))
	input = append(input, aliasSection...)

	c, err := DecodeComponent(input)
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Equal(t, uint32(2), c.NextTypeIdx, "NextTypeIdx should be 2 after type + export alias")
	require.Equal(t, 2, len(c.TypeDefs), "TypeDefs must be densified: one entry per NextTypeIdx bump")
	require.Equal(t, component.TypeDefKindAlias, c.TypeDefs[1].Kind)
	require.NotNil(t, c.TypeDefs[1].Alias)
	require.True(t, c.TypeDefs[1].Alias.IsExport)
	require.Equal(t, uint32(0), c.TypeDefs[1].Alias.InstanceIdx)
	require.Equal(t, "T", c.TypeDefs[1].Alias.ExportName)
}
