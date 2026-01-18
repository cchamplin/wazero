// internal/component/binary/alias.go

package binary

import (
	"bytes"
	"fmt"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/leb128"
)

// decodeAlias parses a single alias definition.
// Format: sort aliastarget
// aliastarget ::= 0x00 instanceidx name       (export)
//
//	| 0x01 core:instanceidx name  (core export)
//	| 0x02 count idx              (outer)
func decodeAlias(r *bytes.Reader) (component.Alias, error) {
	var alias component.Alias

	// Read sort byte
	sortByte, err := r.ReadByte()
	if err != nil {
		return alias, fmt.Errorf("reading sort: %w", err)
	}

	// Handle core sort prefix
	if sortByte == 0x00 {
		coreSortByte, err := r.ReadByte()
		if err != nil {
			return alias, fmt.Errorf("reading core sort: %w", err)
		}
		alias.CoreSort = component.CoreSort(coreSortByte)
		alias.Sort = component.SortCoreSort
	} else {
		alias.Sort = component.Sort(sortByte)
	}

	// Read alias target
	targetByte, err := r.ReadByte()
	if err != nil {
		return alias, fmt.Errorf("reading alias target: %w", err)
	}

	switch targetByte {
	case 0x00: // export alias
		alias.Kind = component.AliasKindExport
		alias.InstanceIdx, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return alias, fmt.Errorf("reading instance index: %w", err)
		}
		alias.ExportName, err = decodeName(r)
		if err != nil {
			return alias, fmt.Errorf("reading export name: %w", err)
		}

	case 0x01: // core export alias
		alias.Kind = component.AliasKindCoreExport
		alias.InstanceIdx, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return alias, fmt.Errorf("reading core instance index: %w", err)
		}
		alias.ExportName, err = decodeName(r)
		if err != nil {
			return alias, fmt.Errorf("reading core export name: %w", err)
		}

	case 0x02: // outer alias
		alias.Kind = component.AliasKindOuter
		alias.OuterCount, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return alias, fmt.Errorf("reading outer count: %w", err)
		}
		alias.OuterIndex, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return alias, fmt.Errorf("reading outer index: %w", err)
		}

	default:
		return alias, fmt.Errorf("unknown alias target: 0x%02x", targetByte)
	}

	return alias, nil
}

// decodeAliasSection parses the alias section (section ID 6).
// Multiple alias sections may exist; aliases are accumulated.
// For core export aliases, the Idx field is assigned based on the current
// index in the appropriate core index space (func, memory, etc.).
func decodeAliasSection(c *component.Component, r *bytes.Reader) error {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("reading alias count: %w", err)
	}

	startIdx := uint32(len(c.Aliases))
	for i := uint32(0); i < count; i++ {
		alias, err := decodeAlias(r)
		if err != nil {
			return fmt.Errorf("decoding alias %d: %w", startIdx+i, err)
		}

		// Assign the target index based on the core sort.
		// Core export aliases add to the appropriate core index space.
		if alias.Kind == component.AliasKindCoreExport {
			switch alias.CoreSort {
			case component.CoreSortFunc:
				alias.Idx = c.NextCoreFuncIdx
				c.NextCoreFuncIdx++
			case component.CoreSortMemory:
				alias.Idx = c.NextCoreMemoryIdx
				c.NextCoreMemoryIdx++
			}
		}

		c.Aliases = append(c.Aliases, alias)
	}

	return nil
}
