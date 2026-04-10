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
//
// Type aliases (both export and outer) additionally register scope
// entries so that later own<>/borrow<> references resolve correctly.
func decodeAliasSection(dc *decodeContext, r *bytes.Reader) error {
	c := dc.c
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

		// Assign the target index based on the alias kind and sort.
		// Each alias type increments the appropriate index space counter.
		switch alias.Kind {
		case component.AliasKindCoreExport:
			// Core export aliases add to the appropriate core index space.
			switch alias.CoreSort {
			case component.CoreSortFunc:
				alias.Idx = c.NextCoreFuncIdx
				c.NextCoreFuncIdx++
			case component.CoreSortTable:
				alias.Idx = c.NextCoreTableIdx
				c.NextCoreTableIdx++
			case component.CoreSortMemory:
				alias.Idx = c.NextCoreMemoryIdx
				c.NextCoreMemoryIdx++
			case component.CoreSortGlobal:
				alias.Idx = c.NextCoreGlobalIdx
				c.NextCoreGlobalIdx++
			case component.CoreSortTag:
				// Tag aliases: no dedicated counter yet, use a placeholder.
				alias.Idx = 0
			case component.CoreSortType:
				alias.Idx = c.NextCoreTypeIdx
				c.NextCoreTypeIdx++
			case component.CoreSortModule:
				alias.Idx = c.NextModuleIdx
				c.NextModuleIdx++
			case component.CoreSortInstance:
				alias.Idx = c.NextModuleInstanceIdx
				c.NextModuleInstanceIdx++
			default:
				return fmt.Errorf("unknown core sort in core export alias: 0x%02x", alias.CoreSort)
			}

		case component.AliasKindExport:
			// Component export aliases add to the appropriate component index space.
			switch alias.Sort {
			case component.SortCoreSort:
				// Core sort within a component export alias: dispatch on the nested core sort.
				switch alias.CoreSort {
				case component.CoreSortFunc:
					alias.Idx = c.NextCoreFuncIdx
					c.NextCoreFuncIdx++
				case component.CoreSortTable:
					alias.Idx = c.NextCoreTableIdx
					c.NextCoreTableIdx++
				case component.CoreSortMemory:
					alias.Idx = c.NextCoreMemoryIdx
					c.NextCoreMemoryIdx++
				case component.CoreSortGlobal:
					alias.Idx = c.NextCoreGlobalIdx
					c.NextCoreGlobalIdx++
				case component.CoreSortTag:
					// Tag aliases: no dedicated counter yet, use a placeholder.
					alias.Idx = 0
				case component.CoreSortType:
					alias.Idx = c.NextCoreTypeIdx
					c.NextCoreTypeIdx++
				case component.CoreSortModule:
					alias.Idx = c.NextModuleIdx
					c.NextModuleIdx++
				case component.CoreSortInstance:
					alias.Idx = c.NextModuleInstanceIdx
					c.NextModuleInstanceIdx++
				default:
					return fmt.Errorf("unknown core sort in export alias: 0x%02x", alias.CoreSort)
				}
			case component.SortFunc:
				alias.Idx = c.NextFuncIdx
				c.NextFuncIdx++
			case component.SortValue:
				alias.Idx = c.NextValueIdx
				c.NextValueIdx++
			case component.SortType:
				alias.Idx = c.NextTypeIdx
				c.NextTypeIdx++
				// Densify Component.TypeDefs: an export type alias
				// consumes a slot in the component's type index space
				// per Binary.md:119, 263-268. Callers resolve via
				// c.ResolveTypeDef(typeidx).
				c.TypeDefs = append(c.TypeDefs, component.TypeDef{
					Kind: component.TypeDefKindAlias,
					Alias: &component.AliasTarget{
						IsExport:    true,
						InstanceIdx: alias.InstanceIdx,
						ExportName:  alias.ExportName,
					},
				})
				// Register a scope entry so that own<>/borrow<> references
				// to this alias slot resolve correctly. Since we don't know
				// the aliased type's kind at decode time (it requires
				// cross-instance resolution), we use scopeEntryAlias with a
				// placeholder abstract resource. If the target turns out not
				// to be a resource, full validation catches it at link time.
				dc.scope.appendAlias(dc.builder.InternAbstractResource())
			case component.SortComponent:
				alias.Idx = c.NextComponentIdx
				c.NextComponentIdx++
			case component.SortInstance:
				alias.Idx = c.NextComponentInstanceIdx
				c.NextComponentInstanceIdx++
			default:
				return fmt.Errorf("unknown sort in export alias: 0x%02x", alias.Sort)
			}

		case component.AliasKindOuter:
			// Outer aliases reference items from enclosing scopes.
			switch alias.Sort {
			case component.SortType:
				alias.Idx = c.NextTypeIdx
				c.NextTypeIdx++
				// Densify Component.TypeDefs: an outer type alias
				// consumes a slot in the component's type index space
				// per Binary.md:118, 263-268.
				c.TypeDefs = append(c.TypeDefs, component.TypeDef{
					Kind: component.TypeDefKindAlias,
					Alias: &component.AliasTarget{
						IsExport:   false,
						OuterCount: alias.OuterCount,
						OuterIndex: alias.OuterIndex,
					},
				})
				// Register a scope entry for outer type aliases.
				// Use scopeEntryAlias with a placeholder since we can't
				// resolve the outer scope's type kind at decode time.
				dc.scope.appendAlias(dc.builder.InternAbstractResource())
			case component.SortComponent:
				alias.Idx = c.NextComponentIdx
				c.NextComponentIdx++
			case component.SortCoreSort:
				switch alias.CoreSort {
				case component.CoreSortModule:
					alias.Idx = c.NextModuleIdx
					c.NextModuleIdx++
				case component.CoreSortType:
					alias.Idx = c.NextCoreTypeIdx
					c.NextCoreTypeIdx++
				default:
					return fmt.Errorf("outer alias with core sort 0x%02x not supported (only module and type allowed)", alias.CoreSort)
				}
			default:
				return fmt.Errorf("unknown or unsupported sort in outer alias: 0x%02x", alias.Sort)
			}
		}

		c.Aliases = append(c.Aliases, alias)
	}

	return nil
}
