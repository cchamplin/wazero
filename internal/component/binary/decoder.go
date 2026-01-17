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
		case SectionIDCoreType:
			if err := decodeCoreTypeSection(c, bytes.NewReader(sectionContent)); err != nil {
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
		case SectionIDInstance:
			if err := decodeInstanceSection(c, bytes.NewReader(sectionContent)); err != nil {
				return nil, fmt.Errorf("section %s: %w", sectionID, err)
			}
		case SectionIDComponent:
			// Nested component - recursively decode
			nestedComponent, err := DecodeComponent(sectionContent)
			if err != nil {
				return nil, fmt.Errorf("section %s: %w", sectionID, err)
			}
			c.Components = append(c.Components, nestedComponent)
		case SectionIDStart:
			if err := decodeStartSection(c, bytes.NewReader(sectionContent)); err != nil {
				return nil, fmt.Errorf("section %s: %w", sectionID, err)
			}
		case SectionIDValue:
			if err := decodeValueSection(c, bytes.NewReader(sectionContent)); err != nil {
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
				Record: convertRecordTypeDef(record),
			}

		case ValTypeOpcodeOption:
			// Decode option type (opcode already consumed, decode inner type directly)
			option, err := decodeOptionTypeDef(r)
			if err != nil {
				return fmt.Errorf("decode option type %d: %w", i, err)
			}

			c.Types[i] = component.TypeDef{
				Kind:   component.TypeDefKindDefined,
				Option: convertOptionTypeDef(option),
			}

		case ValTypeOpcodeList:
			// Decode list type (opcode already consumed, decode element type directly)
			list, err := decodeListTypeDef(r)
			if err != nil {
				return fmt.Errorf("decode list type %d: %w", i, err)
			}

			c.Types[i] = component.TypeDef{
				Kind: component.TypeDefKindDefined,
				List: convertListTypeDef(list),
			}

		case ValTypeOpcodeResult:
			// Decode result type (opcode already consumed, decode ok/error types directly)
			result, err := decodeResultTypeDef(r)
			if err != nil {
				return fmt.Errorf("decode result type %d: %w", i, err)
			}

			c.Types[i] = component.TypeDef{
				Kind:   component.TypeDefKindDefined,
				Result: convertResultTypeDef(result),
			}

		case ValTypeOpcodeVariant:
			// Decode variant type (opcode already consumed, decode cases directly)
			variant, err := decodeVariantTypeDef(r)
			if err != nil {
				return fmt.Errorf("decode variant type %d: %w", i, err)
			}

			c.Types[i] = component.TypeDef{
				Kind:    component.TypeDefKindDefined,
				Variant: convertVariantTypeDef(variant),
			}

		case ValTypeOpcodeTuple:
			// Decode tuple type (opcode already consumed, decode element types directly)
			tuple, err := decodeTupleTypeDef(r)
			if err != nil {
				return fmt.Errorf("decode tuple type %d: %w", i, err)
			}

			c.Types[i] = component.TypeDef{
				Kind:  component.TypeDefKindDefined,
				Tuple: convertTupleTypeDef(tuple),
			}

		case ValTypeOpcodeFlags:
			// Decode flags type (opcode already consumed, decode flag names directly)
			flags, err := decodeFlagsTypeDef(r)
			if err != nil {
				return fmt.Errorf("decode flags type %d: %w", i, err)
			}

			c.Types[i] = component.TypeDef{
				Kind:  component.TypeDefKindDefined,
				Flags: convertFlagsTypeDef(flags),
			}

		case ValTypeOpcodeEnum:
			// Decode enum type (opcode already consumed, decode case names directly)
			enum, err := decodeEnumTypeDef(r)
			if err != nil {
				return fmt.Errorf("decode enum type %d: %w", i, err)
			}

			c.Types[i] = component.TypeDef{
				Kind: component.TypeDefKindDefined,
				Enum: convertEnumTypeDef(enum),
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

		case TypeOpInstance:
			// Decode instance type (opcode already consumed)
			inst, err := decodeInstanceTypeDef(r)
			if err != nil {
				return fmt.Errorf("decode instance type %d: %w", i, err)
			}

			c.Types[i] = component.TypeDef{
				Kind:     component.TypeDefKindInstance,
				Instance: inst,
			}

		case TypeOpComponent:
			// Decode component type (opcode already consumed)
			comp, err := decodeComponentTypeDef(r)
			if err != nil {
				return fmt.Errorf("decode component type %d: %w", i, err)
			}

			c.Types[i] = component.TypeDef{
				Kind:      component.TypeDefKindComponent,
				Component: comp,
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

// convertVariantTypeDef converts a binary package VariantTypeDef to a component package VariantTypeDef.
func convertVariantTypeDef(v *VariantTypeDef) *component.VariantTypeDef {
	cases := make([]component.VariantCase, len(v.Cases))
	for i, c := range v.Cases {
		cases[i] = component.VariantCase{
			Name:    c.Name,
			ValType: c.Type, // Type in binary package maps to ValType in component package
		}
	}
	return &component.VariantTypeDef{Cases: cases}
}

// convertTupleTypeDef converts a binary package TupleTypeDef to a component package TupleTypeDef.
func convertTupleTypeDef(t *TupleTypeDef) *component.TupleTypeDef {
	types := make([]component.ValTypeRef, len(t.Types))
	copy(types, t.Types)
	return &component.TupleTypeDef{Types: types}
}

// convertFlagsTypeDef converts a binary package FlagsTypeDef to a component package FlagsTypeDef.
func convertFlagsTypeDef(f *FlagsTypeDef) *component.FlagsTypeDef {
	names := make([]string, len(f.Names))
	copy(names, f.Names)
	return &component.FlagsTypeDef{Names: names}
}

// convertEnumTypeDef converts a binary package EnumTypeDef to a component package EnumTypeDef.
func convertEnumTypeDef(e *EnumTypeDef) *component.EnumTypeDef {
	names := make([]string, len(e.Cases))
	copy(names, e.Cases)
	return &component.EnumTypeDef{Names: names}
}

// convertRecordTypeDef converts a binary package RecordTypeDef to a component package RecordTypeDef.
func convertRecordTypeDef(r *RecordTypeDef) *component.RecordTypeDef {
	fields := make([]component.RecordField, len(r.Fields))
	for i, f := range r.Fields {
		fields[i] = component.RecordField{
			Name:    f.Name,
			ValType: f.Type,
		}
	}
	return &component.RecordTypeDef{Fields: fields}
}

// convertListTypeDef converts a binary package ListTypeDef to a component package ListTypeDef.
func convertListTypeDef(l *ListTypeDef) *component.ListTypeDef {
	return &component.ListTypeDef{ElementType: l.Element}
}

// convertOptionTypeDef converts a binary package OptionTypeDef to a component package OptionTypeDef.
func convertOptionTypeDef(o *OptionTypeDef) *component.OptionTypeDef {
	return &component.OptionTypeDef{InnerType: o.Some}
}

// convertResultTypeDef converts a binary package ResultTypeDef to a component package ResultTypeDef.
func convertResultTypeDef(r *ResultTypeDef) *component.ResultTypeDef {
	return &component.ResultTypeDef{
		OkType:  r.Ok,
		ErrType: r.Error,
	}
}
