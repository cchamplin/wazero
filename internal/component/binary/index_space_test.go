// internal/component/binary/index_space_test.go

package binary

import (
	"bytes"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestIndexSpaceTracking_TypeSection verifies that the NextTypeIdx counter
// is correctly incremented when decoding the type section.
func TestIndexSpaceTracking_TypeSection(t *testing.T) {
	tests := []struct {
		name            string
		typeSection     []byte
		expectedTypeIdx uint32
	}{
		{
			name: "single function type",
			typeSection: []byte{
				0x01,      // 1 type definition
				0x40,      // sync functype
				0x00,      // 0 params
				0x01,      // named results
				0x00,      // 0 results
			},
			expectedTypeIdx: 1,
		},
		{
			name: "two function types",
			typeSection: []byte{
				0x02, // 2 type definitions
				// Type 0: func() -> ()
				0x40, // sync functype
				0x00, // 0 params
				0x01, // named results
				0x00, // 0 results
				// Type 1: func(a: s32) -> s32
				0x40,      // sync functype
				0x01,      // 1 param
				0x01, 'a', // param name "a"
				0x7a,      // s32
				0x00,      // single result
				0x7a,      // s32
			},
			expectedTypeIdx: 2,
		},
		{
			name: "three types mixed",
			typeSection: []byte{
				0x03, // 3 type definitions
				// Type 0: resource without destructor
				0x3f, // resource type opcode
				0x7f, // rep type i32
				0x00, // no destructor
				// Type 1: func() -> ()
				0x40, // sync functype
				0x00, // 0 params
				0x01, // named results
				0x00, // 0 results
				// Type 2: record { x: s32 }
				0x72,      // record opcode
				0x01,      // 1 field
				0x01, 'x', // field name "x"
				0x7a,      // s32
			},
			expectedTypeIdx: 3,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			// Build component
			input := append(append(Magic[:], Version[:]...), LayerComponent[:]...)
			input = append(input, byte(SectionIDType))
			input = append(input, byte(len(tc.typeSection)))
			input = append(input, tc.typeSection...)

			c, err := DecodeComponent(input)
			require.NoError(t, err)
			require.NotNil(t, c)
			require.Equal(t, tc.expectedTypeIdx, c.NextTypeIdx, "NextTypeIdx mismatch")
			require.Equal(t, int(tc.expectedTypeIdx), len(c.Types), "Types slice length should match NextTypeIdx")
		})
	}
}

// TestIndexSpaceTracking_CoreTypeSection verifies that core type definitions
// are correctly parsed. Note: The current implementation does not increment
// NextCoreTypeIdx in the core type section decoder - core type index tracking
// is done via aliases with CoreSortType.
func TestIndexSpaceTracking_CoreTypeSection(t *testing.T) {
	// Core type section with one function type
	coreTypeSection := []byte{
		0x01, // 1 core type definition
		0x60, // core func type opcode
		0x00, // 0 params
		0x00, // 0 results
	}

	input := append(append(Magic[:], Version[:]...), LayerComponent[:]...)
	input = append(input, byte(SectionIDCoreType))
	input = append(input, byte(len(coreTypeSection)))
	input = append(input, coreTypeSection...)

	c, err := DecodeComponent(input)
	require.NoError(t, err)
	require.NotNil(t, c)
	// Verify the core types were parsed correctly
	require.Equal(t, 1, len(c.CoreTypes), "CoreTypes slice should have one entry")
	require.Equal(t, component.CoreTypeDefKindFunc, c.CoreTypes[0].Kind)
}

// TestIndexSpaceTracking_AliasSection verifies that alias operations correctly
// increment the appropriate index space counters.
func TestIndexSpaceTracking_AliasSection(t *testing.T) {
	t.Run("core export func alias increments NextCoreFuncIdx", func(t *testing.T) {
		// Alias section with core export func alias
		aliasSection := []byte{
			0x01,                // 1 alias
			0x00,                // sort: core
			0x00,                // core sort: func
			0x01,                // target: core export
			0x00,                // instance index
			0x03, 'f', 'o', 'o', // name: "foo"
		}

		c := &component.Component{}
		err := decodeAliasSection(c, bytes.NewReader(aliasSection))
		require.NoError(t, err)
		require.Equal(t, uint32(1), c.NextCoreFuncIdx, "NextCoreFuncIdx should be incremented")
		require.Equal(t, uint32(0), c.Aliases[0].Idx, "Alias should have Idx 0")
	})

	t.Run("core export memory alias increments NextCoreMemoryIdx", func(t *testing.T) {
		aliasSection := []byte{
			0x01,                // 1 alias
			0x00,                // sort: core
			0x02,                // core sort: memory
			0x01,                // target: core export
			0x00,                // instance index
			0x03, 'm', 'e', 'm', // name: "mem"
		}

		c := &component.Component{}
		err := decodeAliasSection(c, bytes.NewReader(aliasSection))
		require.NoError(t, err)
		require.Equal(t, uint32(1), c.NextCoreMemoryIdx, "NextCoreMemoryIdx should be incremented")
		require.Equal(t, uint32(0), c.Aliases[0].Idx, "Alias should have Idx 0")
	})

	t.Run("component export func alias increments NextFuncIdx", func(t *testing.T) {
		aliasSection := []byte{
			0x01,                     // 1 alias
			0x01,                     // sort: func
			0x00,                     // target: export
			0x00,                     // instance index
			0x04, 't', 'e', 's', 't', // name: "test"
		}

		c := &component.Component{}
		err := decodeAliasSection(c, bytes.NewReader(aliasSection))
		require.NoError(t, err)
		require.Equal(t, uint32(1), c.NextFuncIdx, "NextFuncIdx should be incremented")
		require.Equal(t, uint32(0), c.Aliases[0].Idx, "Alias should have Idx 0")
	})

	t.Run("outer type alias increments NextTypeIdx", func(t *testing.T) {
		aliasSection := []byte{
			0x01, // 1 alias
			0x03, // sort: type
			0x02, // target: outer
			0x01, // outer count
			0x05, // outer index
		}

		c := &component.Component{}
		err := decodeAliasSection(c, bytes.NewReader(aliasSection))
		require.NoError(t, err)
		require.Equal(t, uint32(1), c.NextTypeIdx, "NextTypeIdx should be incremented")
		require.Equal(t, uint32(0), c.Aliases[0].Idx, "Alias should have Idx 0")
	})

	t.Run("multiple aliases increment correct counters", func(t *testing.T) {
		aliasSection := []byte{
			0x03, // 3 aliases
			// Alias 1: core export func
			0x00,                // sort: core
			0x00,                // core sort: func
			0x01,                // target: core export
			0x00,                // instance index
			0x01, 'a',           // name: "a"
			// Alias 2: core export func (another one)
			0x00,                // sort: core
			0x00,                // core sort: func
			0x01,                // target: core export
			0x00,                // instance index
			0x01, 'b',           // name: "b"
			// Alias 3: outer type
			0x03, // sort: type
			0x02, // target: outer
			0x01, // outer count
			0x00, // outer index
		}

		c := &component.Component{}
		err := decodeAliasSection(c, bytes.NewReader(aliasSection))
		require.NoError(t, err)
		require.Equal(t, 3, len(c.Aliases))
		require.Equal(t, uint32(2), c.NextCoreFuncIdx, "NextCoreFuncIdx should be 2 after two core func aliases")
		require.Equal(t, uint32(1), c.NextTypeIdx, "NextTypeIdx should be 1 after one type alias")
		// Verify index assignments
		require.Equal(t, uint32(0), c.Aliases[0].Idx, "First core func alias should have Idx 0")
		require.Equal(t, uint32(1), c.Aliases[1].Idx, "Second core func alias should have Idx 1")
		require.Equal(t, uint32(0), c.Aliases[2].Idx, "Type alias should have Idx 0")
	})
}

// TestIndexSpaceTracking_CanonSection verifies that canonical function definitions
// correctly increment the appropriate index space counters.
func TestIndexSpaceTracking_CanonSection(t *testing.T) {
	t.Run("canon lift increments NextFuncIdx", func(t *testing.T) {
		canonSection := []byte{
			0x01, // 1 canonical definition
			0x00, // canon.lift
			0x00, // core sort
			0x00, // core:funcidx = 0
			0x00, // opts count = 0
			0x00, // typeidx = 0
		}

		c := &component.Component{
			FuncIdxToCanonical: make(map[uint32]uint32),
		}
		err := decodeCanonSection(c, bytes.NewReader(canonSection))
		require.NoError(t, err)
		require.Equal(t, uint32(1), c.NextFuncIdx, "NextFuncIdx should be 1 after canon lift")
		require.Equal(t, uint32(0), c.Canonicals[0].ComponentFuncIdx, "ComponentFuncIdx should be 0")
	})

	t.Run("canon lower increments NextCoreFuncIdx", func(t *testing.T) {
		// canon lower format: 0x01 0x00 funcidx opts
		canonSection := []byte{
			0x01, // 1 canonical definition
			0x01, // canon.lower
			0x00, // reserved byte (always 0x00)
			0x00, // funcidx = 0
			0x00, // opts count = 0
		}

		c := &component.Component{
			FuncIdxToCanonical: make(map[uint32]uint32),
		}
		err := decodeCanonSection(c, bytes.NewReader(canonSection))
		require.NoError(t, err)
		require.Equal(t, uint32(1), c.NextCoreFuncIdx, "NextCoreFuncIdx should be 1 after canon lower")
	})

	t.Run("multiple canon operations increment correctly", func(t *testing.T) {
		canonSection := []byte{
			0x02, // 2 canonical definitions
			// Canon 1: lift
			0x00, // canon.lift
			0x00, // core sort
			0x00, // core:funcidx = 0
			0x00, // opts count = 0
			0x00, // typeidx = 0
			// Canon 2: lift another
			0x00, // canon.lift
			0x00, // core sort
			0x01, // core:funcidx = 1
			0x00, // opts count = 0
			0x01, // typeidx = 1
		}

		c := &component.Component{
			FuncIdxToCanonical: make(map[uint32]uint32),
		}
		err := decodeCanonSection(c, bytes.NewReader(canonSection))
		require.NoError(t, err)
		require.Equal(t, uint32(2), c.NextFuncIdx, "NextFuncIdx should be 2 after two canon lifts")
		require.Equal(t, uint32(0), c.Canonicals[0].ComponentFuncIdx, "First canon lift should have ComponentFuncIdx 0")
		require.Equal(t, uint32(1), c.Canonicals[1].ComponentFuncIdx, "Second canon lift should have ComponentFuncIdx 1")
	})
}

// TestIndexSpaceTracking_ExportSection verifies that function exports
// correctly increment NextFuncIdx.
func TestIndexSpaceTracking_ExportSection(t *testing.T) {
	t.Run("function export increments NextFuncIdx", func(t *testing.T) {
		exportSection := []byte{
			0x01,                     // 1 export
			0x00,                     // simple name
			0x04, 't', 'e', 's', 't', // name "test"
			0x01,                     // sort = func
			0x00,                     // index = 0
			0x00,                     // no externdesc
		}

		c := &component.Component{}
		err := decodeExportSection(c, bytes.NewReader(exportSection))
		require.NoError(t, err)
		require.Equal(t, uint32(1), c.NextFuncIdx, "NextFuncIdx should be 1 after function export")
	})

	t.Run("type export does not increment NextFuncIdx", func(t *testing.T) {
		exportSection := []byte{
			0x01,                       // 1 export
			0x00,                       // simple name
			0x04, 'm', 'y', '-', 't',   // name "my-t"
			0x05,                       // sort = type
			0x00,                       // index = 0
			0x00,                       // no externdesc
		}

		c := &component.Component{}
		err := decodeExportSection(c, bytes.NewReader(exportSection))
		require.NoError(t, err)
		require.Equal(t, uint32(0), c.NextFuncIdx, "NextFuncIdx should still be 0 after type export")
	})
}

// TestIndexSpaceTracking_CoreModuleSection verifies that core module definitions
// correctly increment NextModuleIdx.
func TestIndexSpaceTracking_CoreModuleSection(t *testing.T) {
	// Minimal valid core module: magic + version = 8 bytes
	coreModule := []byte{
		0x00, 0x61, 0x73, 0x6d, // magic
		0x01, 0x00, 0x00, 0x00, // version
	}

	// Build component with one core module
	input := append(append(Magic[:], Version[:]...), LayerComponent[:]...)
	input = append(input, byte(SectionIDCoreModule))
	input = append(input, byte(len(coreModule)))
	input = append(input, coreModule...)

	c, err := DecodeComponent(input)
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Equal(t, uint32(1), c.NextModuleIdx, "NextModuleIdx should be 1 after one core module")
	require.Equal(t, 1, len(c.CoreModules), "CoreModules slice should have one entry")
}

// TestIndexSpaceTracking_ComponentStructureInitialization verifies that
// the component struct has properly initialized index counters at zero.
func TestIndexSpaceTracking_ComponentStructureInitialization(t *testing.T) {
	c := &component.Component{}

	require.Equal(t, uint32(0), c.NextTypeIdx, "NextTypeIdx should initialize to 0")
	require.Equal(t, uint32(0), c.NextFuncIdx, "NextFuncIdx should initialize to 0")
	require.Equal(t, uint32(0), c.NextValueIdx, "NextValueIdx should initialize to 0")
	require.Equal(t, uint32(0), c.NextComponentIdx, "NextComponentIdx should initialize to 0")
	require.Equal(t, uint32(0), c.NextComponentInstanceIdx, "NextComponentInstanceIdx should initialize to 0")
	require.Equal(t, uint32(0), c.NextCoreFuncIdx, "NextCoreFuncIdx should initialize to 0")
	require.Equal(t, uint32(0), c.NextCoreTypeIdx, "NextCoreTypeIdx should initialize to 0")
	require.Equal(t, uint32(0), c.NextCoreTableIdx, "NextCoreTableIdx should initialize to 0")
	require.Equal(t, uint32(0), c.NextCoreMemoryIdx, "NextCoreMemoryIdx should initialize to 0")
	require.Equal(t, uint32(0), c.NextCoreGlobalIdx, "NextCoreGlobalIdx should initialize to 0")
	require.Equal(t, uint32(0), c.NextModuleIdx, "NextModuleIdx should initialize to 0")
	require.Equal(t, uint32(0), c.NextModuleInstanceIdx, "NextModuleInstanceIdx should initialize to 0")
}

// TestIndexSpaceTracking_IncrementMechanics verifies basic counter increment behavior.
func TestIndexSpaceTracking_IncrementMechanics(t *testing.T) {
	c := &component.Component{}

	// Initial value should be 0
	require.Equal(t, uint32(0), c.NextTypeIdx)

	// After incrementing, should be 1
	c.NextTypeIdx++
	require.Equal(t, uint32(1), c.NextTypeIdx)

	// After incrementing again, should be 2
	c.NextTypeIdx++
	require.Equal(t, uint32(2), c.NextTypeIdx)
}
