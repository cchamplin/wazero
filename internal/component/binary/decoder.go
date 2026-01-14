// internal/component/binary/decoder.go

package binary

import (
	"bytes"
	"fmt"
	"io"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/leb128"
	wasmbinary "github.com/tetratelabs/wazero/internal/wasm/binary"
)

// DecodeComponent parses a WebAssembly component from binary format.
func DecodeComponent(binary []byte) (*component.Component, error) {
	r := bytes.NewReader(binary)

	// Read and validate magic number
	magic := make([]byte, 4)
	if _, err := io.ReadFull(r, magic); err != nil {
		return nil, ErrInvalidMagic
	}
	if !bytes.Equal(magic, Magic[:]) {
		return nil, ErrInvalidMagic
	}

	// Read and validate version
	version := make([]byte, 2)
	if _, err := io.ReadFull(r, version); err != nil {
		return nil, ErrUnexpectedEOF
	}
	if !bytes.Equal(version, Version[:]) {
		return nil, ErrInvalidVersion
	}

	// Read and validate layer
	layer := make([]byte, 2)
	if _, err := io.ReadFull(r, layer); err != nil {
		return nil, ErrUnexpectedEOF
	}
	if !bytes.Equal(layer, LayerComponent[:]) {
		return nil, ErrInvalidLayer
	}

	c := &component.Component{}

	// Parse sections
	for r.Len() > 0 {
		// Read section ID
		sectionIDByte, err := r.ReadByte()
		if err != nil {
			return nil, ErrUnexpectedEOF
		}
		sectionID := SectionID(sectionIDByte)

		// Read section size (LEB128)
		sectionSize, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("section %s: %w", sectionID, ErrUnexpectedEOF)
		}

		// Read section content
		sectionContent := make([]byte, sectionSize)
		if _, err := io.ReadFull(r, sectionContent); err != nil {
			return nil, fmt.Errorf("section %s: %w", sectionID, ErrUnexpectedEOF)
		}

		switch sectionID {
		case SectionIDCoreModule:
			if err := decodeCoreModuleSection(c, sectionContent); err != nil {
				return nil, fmt.Errorf("section %s: %w", sectionID, err)
			}
		case SectionIDCoreInstance:
			if err := decodeCoreInstanceSection(c, bytes.NewReader(sectionContent)); err != nil {
				return nil, fmt.Errorf("section %s: %w", sectionID, err)
			}
		case SectionIDType:
			if err := decodeTypeSection(c, bytes.NewReader(sectionContent)); err != nil {
				return nil, fmt.Errorf("section %s: %w", sectionID, err)
			}
		case SectionIDCanon:
			if err := decodeCanonSection(c, bytes.NewReader(sectionContent)); err != nil {
				return nil, fmt.Errorf("section %s: %w", sectionID, err)
			}
		case SectionIDExport:
			if err := decodeExportSection(c, bytes.NewReader(sectionContent)); err != nil {
				return nil, fmt.Errorf("section %s: %w", sectionID, err)
			}
		case SectionIDAlias:
			if err := decodeAliasSection(c, bytes.NewReader(sectionContent)); err != nil {
				return nil, fmt.Errorf("section %s: %w", sectionID, err)
			}
		case SectionIDImport:
			if err := decodeImportSection(c, bytes.NewReader(sectionContent)); err != nil {
				return nil, fmt.Errorf("section %s: %w", sectionID, err)
			}
		default:
			// Skip unknown sections for now
		}
	}

	return c, nil
}

// decodeCoreModuleSection parses an embedded core wasm module.
func decodeCoreModuleSection(c *component.Component, content []byte) error {
	m, err := wasmbinary.DecodeModule(
		content,
		api.CoreFeaturesV2,
		65536,
		false,
		false,
		false,
	)
	if err != nil {
		return fmt.Errorf("decode core module: %w", err)
	}
	c.CoreModules = append(c.CoreModules, m)
	// Store raw bytes for instantiation via wazero's public API
	c.CoreModuleData = append(c.CoreModuleData, content)
	return nil
}

// decodeTypeSection parses the type section (section ID 7).
func decodeTypeSection(c *component.Component, r *bytes.Reader) error {
	// Type section is: vec(typedef)
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("read type count: %w", err)
	}

	c.Types = make([]component.TypeDef, count)
	for i := uint32(0); i < count; i++ {
		// Peek at the opcode to determine type kind
		opcode, err := r.ReadByte()
		if err != nil {
			return fmt.Errorf("read type %d opcode: %w", i, err)
		}

		switch opcode {
		case TypeOpFuncSync, TypeOpFuncAsync:
			// Unread the opcode so decodeFuncType can read it
			if err := r.UnreadByte(); err != nil {
				return err
			}

			ft, err := decodeFuncType(r)
			if err != nil {
				return fmt.Errorf("decode functype %d: %w", i, err)
			}

			c.Types[i] = component.TypeDef{
				Kind: component.TypeDefKindFunc,
				Func: ft,
			}

		case ValTypeOpcodeRecord:
			// Decode record type (opcode already consumed, decode fields directly)
			record, err := decodeRecordTypeDef(r)
			if err != nil {
				return fmt.Errorf("decode record type %d: %w", i, err)
			}

			c.Types[i] = component.TypeDef{
				Kind:   component.TypeDefKindDefined,
				Record: record,
			}

		case ValTypeOpcodeOption:
			// Decode option type (opcode already consumed, decode inner type directly)
			option, err := decodeOptionTypeDef(r)
			if err != nil {
				return fmt.Errorf("decode option type %d: %w", i, err)
			}

			c.Types[i] = component.TypeDef{
				Kind:   component.TypeDefKindDefined,
				Option: option,
			}

		case ValTypeOpcodeList:
			// Decode list type (opcode already consumed, decode element type directly)
			list, err := decodeListTypeDef(r)
			if err != nil {
				return fmt.Errorf("decode list type %d: %w", i, err)
			}

			c.Types[i] = component.TypeDef{
				Kind: component.TypeDefKindDefined,
				List: list,
			}

		case ValTypeOpcodeResult:
			// Decode result type (opcode already consumed, decode ok/error types directly)
			result, err := decodeResultTypeDef(r)
			if err != nil {
				return fmt.Errorf("decode result type %d: %w", i, err)
			}

			c.Types[i] = component.TypeDef{
				Kind:   component.TypeDefKindDefined,
				Result: result,
			}

		case TypeOpResourceSync:
			// Decode resource type (opcode already consumed)
			resourceDef, err := decodeResourceTypeDef(r)
			if err != nil {
				return fmt.Errorf("decode resource type %d: %w", i, err)
			}

			c.Types[i] = component.TypeDef{
				Kind:     component.TypeDefKindResource,
				Resource: resourceDef,
			}

		default:
			return fmt.Errorf("unsupported type opcode 0x%02x at index %d", opcode, i)
		}
	}

	return nil
}

// decodeCanonSection parses the canonical section (section ID 8).
func decodeCanonSection(c *component.Component, r *bytes.Reader) error {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("read canon count: %w", err)
	}

	c.Canonicals = make([]component.CanonicalDef, count)
	for i := uint32(0); i < count; i++ {
		def, err := decodeCanonical(r)
		if err != nil {
			return fmt.Errorf("decode canonical %d: %w", i, err)
		}
		c.Canonicals[i] = def
	}

	return nil
}

// decodeExportSection parses the export section (section ID 11).
func decodeExportSection(c *component.Component, r *bytes.Reader) error {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("read export count: %w", err)
	}

	c.Exports = make([]component.Export, count)
	for i := uint32(0); i < count; i++ {
		exp, err := decodeExport(r)
		if err != nil {
			return fmt.Errorf("decode export %d: %w", i, err)
		}
		c.Exports[i] = exp
	}

	return nil
}
