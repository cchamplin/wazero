// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package binary

import (
	"bytes"
	"fmt"
	"io"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/leb128"
	wasmbinary "github.com/tetratelabs/wazero/internal/wasm/binary"
)

// decodeContext carries mutable decoder state across per-section calls.
// One decodeContext per top-level component: it owns the single
// *types.ComponentTypesBuilder and the root *typeScope, so that
// multiple type sections (plus alias pull-ins) all land in the same
// interned table and share the same scope-local index space.
type decodeContext struct {
	c       *component.Component
	builder *types.ComponentTypesBuilder
	scope   *typeScope
	// funcTypeIdx tracks, per type-section slot that was a function
	// declaration, the builder-assigned FuncTypeIdx. Used by callers
	// that need to look up a signature by its type-section index.
	funcTypeIdx map[uint32]types.FuncTypeIdx
	// resourceDefs tracks, per type-section slot that was a resource
	// declaration, the decoded ResourceTypeDef (destructor / callback
	// metadata plus builder resource-table index).
	resourceDefs map[uint32]*ResourceTypeDef
}

func newDecodeContext() *decodeContext {
	return &decodeContext{
		c: &component.Component{
			FuncIdxToCanonical: make(map[uint32]uint32),
		},
		builder:      types.NewComponentTypesBuilder(),
		scope:        newTypeScope(nil),
		funcTypeIdx:  make(map[uint32]types.FuncTypeIdx),
		resourceDefs: make(map[uint32]*ResourceTypeDef),
	}
}

// DecodeComponent parses a WebAssembly component from binary format.
func DecodeComponent(binary []byte) (*component.Component, error) {
	dc := newDecodeContext()
	if err := decodeComponentInto(dc, binary); err != nil {
		return nil, err
	}
	dc.c.Types = dc.builder.Finish()
	return dc.c, nil
}

// decodeComponentInto walks the binary stream and dispatches each
// section to its decoder. The *decodeContext outlives individual
// sections so the type builder and scope accumulate state across them.
func decodeComponentInto(dc *decodeContext, binary []byte) error {
	r := bytes.NewReader(binary)

	// Read and validate magic number
	magic := make([]byte, 4)
	if _, err := io.ReadFull(r, magic); err != nil {
		return ErrInvalidMagic
	}
	if !bytes.Equal(magic, Magic[:]) {
		return ErrInvalidMagic
	}

	// Read and validate version
	version := make([]byte, 2)
	if _, err := io.ReadFull(r, version); err != nil {
		return ErrUnexpectedEOF
	}
	if !bytes.Equal(version, Version[:]) {
		return ErrInvalidVersion
	}

	// Read and validate layer
	layer := make([]byte, 2)
	if _, err := io.ReadFull(r, layer); err != nil {
		return ErrUnexpectedEOF
	}
	if !bytes.Equal(layer, LayerComponent[:]) {
		return ErrInvalidLayer
	}

	c := dc.c

	// Parse sections
	for r.Len() > 0 {
		sectionIDByte, err := r.ReadByte()
		if err != nil {
			return ErrUnexpectedEOF
		}
		sectionID := SectionID(sectionIDByte)

		sectionSize, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return fmt.Errorf("section %s: %w", sectionID, ErrUnexpectedEOF)
		}

		sectionContent := make([]byte, sectionSize)
		if _, err := io.ReadFull(r, sectionContent); err != nil {
			return fmt.Errorf("section %s: %w", sectionID, ErrUnexpectedEOF)
		}

		switch sectionID {
		case SectionIDCoreModule:
			if err := decodeCoreModuleSection(c, sectionContent); err != nil {
				return fmt.Errorf("section %s: %w", sectionID, err)
			}
		case SectionIDCoreInstance:
			if err := decodeCoreInstanceSection(c, bytes.NewReader(sectionContent)); err != nil {
				return fmt.Errorf("section %s: %w", sectionID, err)
			}
		case SectionIDCoreType:
			if err := decodeCoreTypeSection(c, bytes.NewReader(sectionContent)); err != nil {
				return fmt.Errorf("section %s: %w", sectionID, err)
			}
		case SectionIDType:
			if err := decodeTypeSection(dc, bytes.NewReader(sectionContent)); err != nil {
				return fmt.Errorf("section %s: %w", sectionID, err)
			}
		case SectionIDCanon:
			if err := decodeCanonSection(c, bytes.NewReader(sectionContent)); err != nil {
				return fmt.Errorf("section %s: %w", sectionID, err)
			}
		case SectionIDExport:
			if err := decodeExportSection(c, bytes.NewReader(sectionContent)); err != nil {
				return fmt.Errorf("section %s: %w", sectionID, err)
			}
		case SectionIDAlias:
			if err := decodeAliasSection(c, bytes.NewReader(sectionContent)); err != nil {
				return fmt.Errorf("section %s: %w", sectionID, err)
			}
		case SectionIDImport:
			if err := decodeImportSection(dc, bytes.NewReader(sectionContent)); err != nil {
				return fmt.Errorf("section %s: %w", sectionID, err)
			}
		case SectionIDInstance:
			if err := decodeInstanceSection(c, bytes.NewReader(sectionContent)); err != nil {
				return fmt.Errorf("section %s: %w", sectionID, err)
			}
		case SectionIDComponent:
			// Nested component: recursively decode. Nested components
			// own their own *ComponentTypes — they are a self-contained
			// top-level component from the decoder's perspective.
			nestedComponent, err := DecodeComponent(sectionContent)
			if err != nil {
				return fmt.Errorf("section %s: %w", sectionID, err)
			}
			c.Components = append(c.Components, nestedComponent)
			c.NextComponentIdx++
		case SectionIDStart:
			if err := decodeStartSection(c, bytes.NewReader(sectionContent)); err != nil {
				return fmt.Errorf("section %s: %w", sectionID, err)
			}
		case SectionIDValue:
			if err := decodeValueSection(dc, bytes.NewReader(sectionContent)); err != nil {
				return fmt.Errorf("section %s: %w", sectionID, err)
			}
		default:
			// Skip unknown sections for now
		}
	}

	return nil
}

// decodeCoreModuleSection parses an embedded core wasm module.
func decodeCoreModuleSection(c *component.Component, content []byte) error {
	// Use DecodeModuleForComponent to allow empty import module names
	m, err := wasmbinary.DecodeModuleForComponent(
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
	c.NextModuleIdx++
	return nil
}

// decodeTypeSection parses the type section (section ID 7).
//
// Multiple type sections may exist in a component; all of them feed
// into the same *decodeContext scope and builder, so scope-local
// indices from a later section continue numbering from where the
// previous section left off.
//
// For each entry the decoder dispatches on the leading opcode:
//
//   - 0x40 / 0x43                 → function type; interned via b.InternFunc,
//     stored in dc.funcTypeIdx and appended to the scope as scopeEntryOther
//   - 0x3f / 0x3e                 → resource declaration; interned via
//     b.InternAbstractResource, stored in dc.resourceDefs and appended to
//     the scope as a scopeEntryResource
//   - 0x41 / 0x42                 → component / instance type declaration;
//     decoded with the per-kind helpers and appended to the scope as
//     scopeEntryOther (Session 2 work upgrades these to first-class slots)
//   - primitive / composite       → value type; decoded by decodeDefinedType
//     or decodeValType and appended to the scope as a scopeEntryValType
func decodeTypeSection(dc *decodeContext, r *bytes.Reader) error {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("read type count: %w", err)
	}

	// Invariant baseline: TypeDefs may already hold entries from earlier
	// type sections in the same component. Each iteration of the loop
	// below appends exactly one entry, so len(TypeDefs) must grow by
	// `count` by the time this section is done. Alias-section handling
	// is out of scope for Task A3 — outer / export type aliases bump
	// NextTypeIdx without populating TypeDefs, which is why the
	// invariant below checks the delta across this single call rather
	// than equality with NextTypeIdx.
	beforeTypeDefs := len(dc.c.TypeDefs)

	for i := uint32(0); i < count; i++ {
		slot := dc.c.NextTypeIdx
		opcode, err := r.ReadByte()
		if err != nil {
			return fmt.Errorf("read type %d opcode: %w", slot, err)
		}

		switch opcode {
		case TypeOpFuncSync, TypeOpFuncAsync:
			if err := r.UnreadByte(); err != nil {
				return err
			}
			ftIdx, err := decodeFuncType(r, dc.scope, dc.builder)
			if err != nil {
				return fmt.Errorf("decode functype %d: %w", slot, err)
			}
			dc.funcTypeIdx[slot] = ftIdx
			dc.scope.appendOther()
			dc.c.TypeDefs = append(dc.c.TypeDefs, component.TypeDef{
				Kind: component.TypeDefKindFunc,
				Func: ftIdx,
			})

		case TypeOpResourceSync:
			resourceDef, err := decodeResourceDecl(r, dc.scope, dc.builder, false)
			if err != nil {
				return fmt.Errorf("decode resource type %d: %w", slot, err)
			}
			dc.resourceDefs[slot] = resourceDef
			dc.scope.appendResource(resourceDef.ResourceTableIdx)
			dc.c.TypeDefs = append(dc.c.TypeDefs, component.TypeDef{
				Kind:                 component.TypeDefKindResource,
				Resource:             resourceDef.ResourceTableIdx,
				ResourceDtor:         resourceDef.Destructor,
				ResourceDtorAsync:    resourceDef.AsyncDestructor,
				ResourceDtorCallback: resourceDef.Callback,
			})

		case TypeOpResourceAsync:
			resourceDef, err := decodeResourceDecl(r, dc.scope, dc.builder, true)
			if err != nil {
				return fmt.Errorf("decode async resource type %d: %w", slot, err)
			}
			dc.resourceDefs[slot] = resourceDef
			dc.scope.appendResource(resourceDef.ResourceTableIdx)
			dc.c.TypeDefs = append(dc.c.TypeDefs, component.TypeDef{
				Kind:                 component.TypeDefKindResource,
				Resource:             resourceDef.ResourceTableIdx,
				ResourceDtor:         resourceDef.Destructor,
				ResourceDtorAsync:    resourceDef.AsyncDestructor,
				ResourceDtorCallback: resourceDef.Callback,
			})

		case TypeOpInstance:
			// Session 2 work: nested instance types still parse as
			// opaque declarations so the scope index space stays in
			// sync with the binary format. The decoded declaration
			// list is now surfaced via Component.TypeDefs so later
			// passes can walk it structurally.
			itd, err := decodeInstanceTypeDef(r)
			if err != nil {
				return fmt.Errorf("decode instance type %d: %w", slot, err)
			}
			dc.scope.appendOther()
			dc.c.TypeDefs = append(dc.c.TypeDefs, component.TypeDef{
				Kind:     component.TypeDefKindInstance,
				Instance: itd,
			})

		case TypeOpComponent:
			// See TypeOpInstance note above.
			ctd, err := decodeComponentTypeDef(r)
			if err != nil {
				return fmt.Errorf("decode component type %d: %w", slot, err)
			}
			dc.scope.appendOther()
			dc.c.TypeDefs = append(dc.c.TypeDefs, component.TypeDef{
				Kind:      component.TypeDefKindComponent,
				Component: ctd,
			})

		case ValTypeOpcodeRecord:
			vt, err := decodeRecord(r, dc.scope, dc.builder)
			if err != nil {
				return fmt.Errorf("decode record type %d: %w", slot, err)
			}
			dc.scope.appendValType(vt)
			dc.c.TypeDefs = append(dc.c.TypeDefs, component.TypeDef{
				Kind:    component.TypeDefKindDefined,
				ValType: vt,
			})

		case ValTypeOpcodeVariant:
			vt, err := decodeVariant(r, dc.scope, dc.builder)
			if err != nil {
				return fmt.Errorf("decode variant type %d: %w", slot, err)
			}
			dc.scope.appendValType(vt)
			dc.c.TypeDefs = append(dc.c.TypeDefs, component.TypeDef{
				Kind:    component.TypeDefKindDefined,
				ValType: vt,
			})

		case ValTypeOpcodeList:
			vt, err := decodeList(r, dc.scope, dc.builder)
			if err != nil {
				return fmt.Errorf("decode list type %d: %w", slot, err)
			}
			dc.scope.appendValType(vt)
			dc.c.TypeDefs = append(dc.c.TypeDefs, component.TypeDef{
				Kind:    component.TypeDefKindDefined,
				ValType: vt,
			})

		case ValTypeOpcodeFixedSizeList:
			vt, err := decodeFixedLengthList(r, dc.scope, dc.builder)
			if err != nil {
				return fmt.Errorf("decode fixed-size list type %d: %w", slot, err)
			}
			dc.scope.appendValType(vt)
			dc.c.TypeDefs = append(dc.c.TypeDefs, component.TypeDef{
				Kind:    component.TypeDefKindDefined,
				ValType: vt,
			})

		case ValTypeOpcodeTuple:
			vt, err := decodeTuple(r, dc.scope, dc.builder)
			if err != nil {
				return fmt.Errorf("decode tuple type %d: %w", slot, err)
			}
			dc.scope.appendValType(vt)
			dc.c.TypeDefs = append(dc.c.TypeDefs, component.TypeDef{
				Kind:    component.TypeDefKindDefined,
				ValType: vt,
			})

		case ValTypeOpcodeFlags:
			vt, err := decodeFlags(r, dc.builder)
			if err != nil {
				return fmt.Errorf("decode flags type %d: %w", slot, err)
			}
			dc.scope.appendValType(vt)
			dc.c.TypeDefs = append(dc.c.TypeDefs, component.TypeDef{
				Kind:    component.TypeDefKindDefined,
				ValType: vt,
			})

		case ValTypeOpcodeEnum:
			vt, err := decodeEnum(r, dc.builder)
			if err != nil {
				return fmt.Errorf("decode enum type %d: %w", slot, err)
			}
			dc.scope.appendValType(vt)
			dc.c.TypeDefs = append(dc.c.TypeDefs, component.TypeDef{
				Kind:    component.TypeDefKindDefined,
				ValType: vt,
			})

		case ValTypeOpcodeOption:
			vt, err := decodeOption(r, dc.scope, dc.builder)
			if err != nil {
				return fmt.Errorf("decode option type %d: %w", slot, err)
			}
			dc.scope.appendValType(vt)
			dc.c.TypeDefs = append(dc.c.TypeDefs, component.TypeDef{
				Kind:    component.TypeDefKindDefined,
				ValType: vt,
			})

		case ValTypeOpcodeResult:
			vt, err := decodeResult(r, dc.scope, dc.builder)
			if err != nil {
				return fmt.Errorf("decode result type %d: %w", slot, err)
			}
			dc.scope.appendValType(vt)
			dc.c.TypeDefs = append(dc.c.TypeDefs, component.TypeDef{
				Kind:    component.TypeDefKindDefined,
				ValType: vt,
			})

		case ValTypeOpcodeStream:
			vt, err := decodeStream(r, dc.scope, dc.builder)
			if err != nil {
				return fmt.Errorf("decode stream type %d: %w", slot, err)
			}
			dc.scope.appendValType(vt)
			dc.c.TypeDefs = append(dc.c.TypeDefs, component.TypeDef{
				Kind:    component.TypeDefKindDefined,
				ValType: vt,
			})

		case ValTypeOpcodeFuture:
			vt, err := decodeFuture(r, dc.scope, dc.builder)
			if err != nil {
				return fmt.Errorf("decode future type %d: %w", slot, err)
			}
			dc.scope.appendValType(vt)
			dc.c.TypeDefs = append(dc.c.TypeDefs, component.TypeDef{
				Kind:    component.TypeDefKindDefined,
				ValType: vt,
			})

		default:
			// Primitive value types can appear as bare type-section
			// entries (a defvaltype equal to a primitive). Re-read as a
			// ValType via decodeValType against the current scope.
			if IsPrimValType(opcode) {
				if err := r.UnreadByte(); err != nil {
					return err
				}
				vt, err := decodeValType(r, dc.scope, dc.builder)
				if err != nil {
					return fmt.Errorf("decode primitive type %d: %w", slot, err)
				}
				dc.scope.appendValType(vt)
				dc.c.TypeDefs = append(dc.c.TypeDefs, component.TypeDef{
					Kind:    component.TypeDefKindDefined,
					ValType: vt,
				})
				break
			}
			return fmt.Errorf("unsupported type opcode 0x%02x at index %d", opcode, slot)
		}

		dc.c.NextTypeIdx++
		// Invariant: each slot above appends exactly one TypeDef
		// entry. A missing append in a new opcode case would leave
		// TypeDefs short of (beforeTypeDefs + i + 1); flag that here
		// so a misbehaving branch surfaces as an internal error
		// rather than a silently misaligned TypeDefs slice.
		if len(dc.c.TypeDefs) != beforeTypeDefs+int(i)+1 {
			return fmt.Errorf("decoder invariant: TypeDefs length %d != expected %d after slot %d",
				len(dc.c.TypeDefs), beforeTypeDefs+int(i)+1, slot)
		}
	}

	return nil
}

// decodeCanonSection parses the canonical section (section ID 8).
// Multiple canon sections may exist; entries are accumulated.
// Each canon lift operation consumes one entry from the component function index space.
// Canon lower and resource operations consume entries from the core function index space.
func decodeCanonSection(c *component.Component, r *bytes.Reader) error {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("read canon count: %w", err)
	}

	startIdx := uint32(len(c.Canonicals))
	for i := uint32(0); i < count; i++ {
		def, err := decodeCanonical(r)
		if err != nil {
			return fmt.Errorf("decode canonical %d: %w", startIdx+i, err)
		}

		switch def.Kind {
		case component.CanonKindLift:
			// Assign the component function index for lift operations.
			// Each canon lift consumes the next available component function index.
			def.ComponentFuncIdx = c.NextFuncIdx
			c.FuncIdxToCanonical[c.NextFuncIdx] = startIdx + i
			c.NextFuncIdx++

		case component.CanonKindLower:
			// Canon lower produces a core function.
			// Store the assigned core function index, then increment.
			def.ComponentFuncIdx = c.NextCoreFuncIdx
			c.NextCoreFuncIdx++

		case component.CanonKindResourceNew, component.CanonKindResourceDrop, component.CanonKindResourceRep:
			// Resource operations (new, drop, rep) produce core functions.
			// Store the assigned core function index, then increment.
			def.ComponentFuncIdx = c.NextCoreFuncIdx
			c.NextCoreFuncIdx++
		}

		c.Canonicals = append(c.Canonicals, def)
	}

	return nil
}

// decodeExportSection parses the export section (section ID 11).
// Multiple export sections may exist; exports are accumulated.
// According to the Component Model spec, exports introduce a new index that
// aliases the exported definition. Function exports increment the function index space.
func decodeExportSection(c *component.Component, r *bytes.Reader) error {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("read export count: %w", err)
	}

	startIdx := uint32(len(c.Exports))
	for i := uint32(0); i < count; i++ {
		exp, err := decodeExport(r)
		if err != nil {
			return fmt.Errorf("decode export %d: %w", startIdx+i, err)
		}
		c.Exports = append(c.Exports, exp)

		// According to the Component Model spec, exports introduce a new index
		// that aliases the exported definition. For function exports, this
		// increments the component function index space.
		if exp.Kind == component.ExportKindFunc {
			c.NextFuncIdx++
		}
	}

	return nil
}
