// internal/component/binary/exports.go

package binary

import (
	"bytes"
	"fmt"
	"io"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/leb128"
)

// Export name discriminators
const (
	ExportNameSimple    byte = 0x00 // No version suffix
	ExportNameVersioned byte = 0x01 // Has version suffix
)

// Sort indicators (for sortidx)
const (
	SortCore      byte = 0x00 // Core definition
	SortFunc      byte = 0x01 // Component function
	SortValue     byte = 0x02 // Value (gated)
	SortType      byte = 0x03 // Type
	SortComponent byte = 0x04 // Component
	SortInstance  byte = 0x05 // Instance
)

// decodeExport reads a single export definition.
func decodeExport(r *bytes.Reader) (component.Export, error) {
	exp := component.Export{}

	// Read export name
	name, err := decodeExportName(r)
	if err != nil {
		return exp, fmt.Errorf("read export name: %w", err)
	}
	exp.Name = name

	// Read sortidx (sort + index)
	sort, err := r.ReadByte()
	if err != nil {
		return exp, fmt.Errorf("read sort: %w", err)
	}

	// Handle core sort prefix
	if sort == SortCore {
		coreSortByte, err := r.ReadByte()
		if err != nil {
			return exp, fmt.Errorf("read core sort: %w", err)
		}
		// Map core sort to export kind
		exp.IsCoreSort = true
		exp.CoreSort = component.CoreSort(coreSortByte)
		switch component.CoreSort(coreSortByte) {
		case component.CoreSortFunc:
			exp.Kind = component.ExportKindFunc
		case component.CoreSortTable:
			exp.Kind = component.ExportKindTable
		case component.CoreSortMemory:
			exp.Kind = component.ExportKindMemory
		case component.CoreSortGlobal:
			exp.Kind = component.ExportKindGlobal
		case component.CoreSortTag:
			exp.Kind = component.ExportKindTag
		case component.CoreSortType:
			exp.Kind = component.ExportKindType
		case component.CoreSortModule:
			exp.Kind = component.ExportKindModule
		case component.CoreSortInstance:
			exp.Kind = component.ExportKindCoreInstance
		default:
			return exp, fmt.Errorf("unknown core sort: 0x%02x", coreSortByte)
		}
	} else {
		// Map sort to ExportKind
		switch sort {
		case SortFunc:
			exp.Kind = component.ExportKindFunc
		case SortValue:
			exp.Kind = component.ExportKindValue
		case SortType:
			exp.Kind = component.ExportKindType
		case SortComponent:
			exp.Kind = component.ExportKindComponent
		case SortInstance:
			exp.Kind = component.ExportKindInstance
		default:
			return exp, fmt.Errorf("unknown sort: 0x%02x", sort)
		}
	}

	idx, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return exp, fmt.Errorf("read index: %w", err)
	}
	exp.Idx = idx

	// Read the mandatory externdesc flag (0x00 = no type, 0x01 = has type)
	hasExternDesc, err := r.ReadByte()
	if err != nil {
		return exp, fmt.Errorf("read extern desc flag: %w", err)
	}
	if hasExternDesc == 0x01 {
		// Read extern desc kind
		externKind, err := r.ReadByte()
		if err != nil {
			return exp, fmt.Errorf("read extern desc kind: %w", err)
		}
		_ = externKind

		// Read type index
		typeIdx, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return exp, fmt.Errorf("read extern desc type index: %w", err)
		}
		exp.TypeIdx = &typeIdx
	}

	return exp, nil
}

// decodeExportName reads an export name with optional version suffix.
func decodeExportName(r *bytes.Reader) (string, error) {
	discriminator, err := r.ReadByte()
	if err != nil {
		return "", err
	}

	switch discriminator {
	case ExportNameSimple:
		// Just read the name
		return decodeName(r)

	case ExportNameVersioned:
		// Read name, then skip version suffix
		name, err := decodeName(r)
		if err != nil {
			return "", err
		}
		// Skip version suffix for now
		suffixLen, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return "", fmt.Errorf("read version suffix length: %w", err)
		}
		if _, err := io.CopyN(io.Discard, r, int64(suffixLen)); err != nil {
			return "", err
		}
		return name, nil

	default:
		return "", fmt.Errorf("unknown export name discriminator: 0x%02x", discriminator)
	}
}
