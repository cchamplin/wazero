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
		case SectionIDType:
			if err := decodeTypeSection(c, bytes.NewReader(sectionContent)); err != nil {
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

		default:
			return fmt.Errorf("unsupported type opcode 0x%02x at index %d", opcode, i)
		}
	}

	return nil
}
