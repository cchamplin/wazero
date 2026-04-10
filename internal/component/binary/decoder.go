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
//
// Per-slot function and resource metadata is recorded on
// `dc.c.TypeDefs` (Session 1 Decision 5: TypeDefs is the single source
// of truth for type-section metadata). Inside decodeTypeSection the
// local `slot` variable equals the global `dc.c.NextTypeIdx`, so the
// just-appended entry is `dc.c.TypeDefs[len(dc.c.TypeDefs)-1]`. After
// the alias-densification fix (see binary/alias.go), type aliases
// also append a TypeDefKindAlias entry alongside the NextTypeIdx++
// bump, so `len(dc.c.TypeDefs) == dc.c.NextTypeIdx` is a whole-
// component invariant. Callers that need to resolve an alias chain
// to the underlying concrete TypeDef must use
// Component.ResolveTypeDef rather than bare indexing.
type decodeContext struct {
	c       *component.Component
	builder *types.ComponentTypesBuilder
	scope   *typeScope
}

func newDecodeContext() *decodeContext {
	return &decodeContext{
		c: &component.Component{
			FuncIdxToCanonical: make(map[uint32]uint32),
		},
		builder: types.NewComponentTypesBuilder(),
		scope:   newTypeScope(nil),
	}
}

// DecodeComponent parses a WebAssembly component from binary format.
func DecodeComponent(binary []byte) (*component.Component, error) {
	dc := newDecodeContext()
	if err := decodeComponentInto(dc, binary); err != nil {
		return nil, err
	}
	dc.c.Types = dc.builder.Finish()
	// Invariant: densified TypeDefs — every NextTypeIdx++ during
	// decode (type-section entries AND alias sections) appends
	// exactly one TypeDef entry.
	// Spec: Binary.md:110-122 (flat type index space).
	if uint32(len(dc.c.TypeDefs)) != dc.c.NextTypeIdx {
		return nil, fmt.Errorf("decoder invariant: len(TypeDefs)=%d != NextTypeIdx=%d (alias-densification bug)",
			len(dc.c.TypeDefs), dc.c.NextTypeIdx)
	}
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
			if err := decodeCanonSection(dc, bytes.NewReader(sectionContent)); err != nil {
				return fmt.Errorf("section %s: %w", sectionID, err)
			}
		case SectionIDExport:
			if err := decodeExportSection(dc, bytes.NewReader(sectionContent)); err != nil {
				return fmt.Errorf("section %s: %w", sectionID, err)
			}
		case SectionIDAlias:
			if err := decodeAliasSection(dc, bytes.NewReader(sectionContent)); err != nil {
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
//   - 0x40 / 0x43                 → function type; interned via b.InternFunc
//     and appended to the scope as scopeEntryOther. The builder-assigned
//     FuncTypeIdx is recorded on dc.c.TypeDefs[slot].Func.
//   - 0x3f / 0x3e                 → resource declaration; interned via
//     b.InternAbstractResource and appended to the scope as a
//     scopeEntryResource. The destructor / callback metadata is recorded
//     on dc.c.TypeDefs[slot].Resource* fields.
//   - 0x41 / 0x42                 → component / instance type declaration;
//     decoded with the per-kind helpers and appended to the scope as
//     scopeEntryOther (TODO: upgrade these to first-class scope slots)
//   - primitive / composite       → value type; decoded by decodeDefinedType
//     or decodeValType and appended to the scope as a scopeEntryValType
func decodeTypeSection(dc *decodeContext, r *bytes.Reader) error {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("read type count: %w", err)
	}

	// After the alias-densification fix, TypeDefs is densely aligned
	// with NextTypeIdx across the full component decode. Within this
	// function, each iteration appends exactly one TypeDef; the
	// baseline delta check confirms that invariant locally. A whole-
	// component invariant (`len(TypeDefs) == NextTypeIdx`) is
	// additionally enforced at the end of DecodeComponent.
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
			dc.scope.appendResource(resourceDef.ResourceTableIdx)
			dc.c.TypeDefs = append(dc.c.TypeDefs, component.TypeDef{
				Kind:                 component.TypeDefKindResource,
				Resource:             resourceDef.ResourceTableIdx,
				ResourceDtor:         resourceDef.Destructor,
				ResourceDtorAsync:    resourceDef.AsyncDestructor,
				ResourceDtorCallback: resourceDef.Callback,
			})

		case TypeOpInstance:
			// TODO: nested instance types still parse as opaque
			// declarations so the scope index space stays in sync
			// with the binary format. The decoded declaration list
			// is surfaced via Component.TypeDefs so later passes
			// can walk it structurally.
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

		case ValTypeOpcodeOwn:
			// own<R> as a standalone type entry in the type section.
			// The resource index follows as a LEB128 uint32.
			resIdx, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return fmt.Errorf("decode own type %d: read resource index: %w", slot, err)
			}
			if int(resIdx) >= len(dc.scope.entries) {
				return fmt.Errorf("decode own type %d: type index %d out of range", slot, resIdx)
			}
			entry := dc.scope.entries[resIdx]
			if entry.kind != scopeEntryResource && entry.kind != scopeEntryAlias {
				return fmt.Errorf("decode own type %d: type index %d is not a resource type", slot, resIdx)
			}
			vt := dc.builder.InternOwnHandle(entry.resource)
			dc.scope.appendValType(vt)
			dc.c.TypeDefs = append(dc.c.TypeDefs, component.TypeDef{
				Kind:    component.TypeDefKindDefined,
				ValType: vt,
			})

		case ValTypeOpcodeBorrow:
			// borrow<R> as a standalone type entry in the type section.
			// The resource index follows as a LEB128 uint32.
			resIdx, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return fmt.Errorf("decode borrow type %d: read resource index: %w", slot, err)
			}
			if int(resIdx) >= len(dc.scope.entries) {
				return fmt.Errorf("decode borrow type %d: type index %d out of range", slot, resIdx)
			}
			entry := dc.scope.entries[resIdx]
			if entry.kind != scopeEntryResource && entry.kind != scopeEntryAlias {
				return fmt.Errorf("decode borrow type %d: type index %d is not a resource type", slot, resIdx)
			}
			vt := dc.builder.InternBorrowHandle(entry.resource)
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
func decodeCanonSection(dc *decodeContext, r *bytes.Reader) error {
	c := dc.c
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
			// Validate that the type index is within bounds and refers
			// to a resource type. Spec: "resource.new", "resource.drop",
			// and "resource.rep" each require a typeidx that resolves to
			// a resource declaration.
			typeIdx := def.TypeIdx
			if int(typeIdx) >= len(dc.scope.entries) {
				return fmt.Errorf("decode canonical %d: type index %d out of bounds", startIdx+i, typeIdx)
			}
			entry := dc.scope.entries[typeIdx]
			if entry.kind != scopeEntryResource && entry.kind != scopeEntryAlias {
				return fmt.Errorf("decode canonical %d: type index %d is not a resource type", startIdx+i, typeIdx)
			}

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
//
// According to the Component Model spec, exports introduce a new index
// that aliases the exported definition. Each export kind increments
// its corresponding index space, and type exports additionally
// register scope entries so that later own<>/borrow<> references
// resolve correctly.
func decodeExportSection(dc *decodeContext, r *bytes.Reader) error {
	c := dc.c
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
		// that aliases the exported definition.
		switch exp.Kind {
		case component.ExportKindFunc:
			c.NextFuncIdx++
		case component.ExportKindType:
			// Type exports alias the exported type into a new type index slot.
			srcIdx := exp.Idx
			if int(srcIdx) < len(dc.scope.entries) {
				src := dc.scope.entries[srcIdx]
				dc.scope.entries = append(dc.scope.entries, src)
			} else {
				// Source index not yet in scope — append an alias placeholder
				// to keep the scope aligned with NextTypeIdx.
				dc.scope.appendAlias(dc.builder.InternAbstractResource())
			}
			c.TypeDefs = append(c.TypeDefs, component.TypeDef{
				Kind: component.TypeDefKindAlias,
			})
			c.NextTypeIdx++
		case component.ExportKindInstance:
			c.NextComponentInstanceIdx++
		case component.ExportKindComponent:
			c.NextComponentIdx++
		}
	}

	return nil
}
