// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package binary

import (
	"bytes"
	"fmt"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/leb128"
)

// decodeImportName decodes an import name with optional version suffix.
// Format: 0x00 len name       (plain name)
//
//	| 0x01 len name       (name with version suffix embedded)
func decodeImportName(r *bytes.Reader) (string, error) {
	prefix, err := r.ReadByte()
	if err != nil {
		return "", fmt.Errorf("reading import name prefix: %w", err)
	}

	switch prefix {
	case 0x00, 0x01:
		// Both cases: read length-prefixed name.
		// The version suffix is embedded in the name string itself.
		return decodeName(r)
	default:
		return "", fmt.Errorf("unknown import name prefix: 0x%02x", prefix)
	}
}

// decodeExternDesc decodes an import extern descriptor.
// Format: 0x00 0x11 core:typeidx  (core module)
//
//	| 0x01 typeidx            (func)
//	| 0x02 valuebound         (value)
//	| 0x03 typebound          (type)
//	| 0x04 typeidx            (component)
//	| 0x05 typeidx            (instance)
func decodeExternDesc(dc *decodeContext, r *bytes.Reader) (component.ImportExternDesc, error) {
	var desc component.ImportExternDesc

	kindByte, err := r.ReadByte()
	if err != nil {
		return desc, fmt.Errorf("reading externdesc kind: %w", err)
	}

	switch kindByte {
	case 0x00:
		// Core module: expect 0x11 prefix then core type index
		prefix, err := r.ReadByte()
		if err != nil {
			return desc, fmt.Errorf("reading core module prefix: %w", err)
		}
		if prefix != 0x11 {
			return desc, fmt.Errorf("expected 0x11 for core module, got 0x%02x", prefix)
		}
		desc.Kind = component.ImportExternDescCoreModule
		desc.CoreTypeIdx, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return desc, fmt.Errorf("reading core type index: %w", err)
		}

	case 0x01:
		desc.Kind = component.ImportExternDescFunc
		desc.TypeIdx, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return desc, fmt.Errorf("reading func type index: %w", err)
		}

	case 0x02:
		desc.Kind = component.ImportExternDescValue
		// Decode valuebound: valtype (the type of the value being imported).
		vt, err := decodeValType(r, dc.scope, dc.builder)
		if err != nil {
			return desc, fmt.Errorf("decode value type: %w", err)
		}
		desc.ValType = vt

	case 0x03:
		// Type import: externdesc 0x03 b:<typebound>.
		// Spec: Binary.md:236,239-240.
		//   typebound 0x00 i:<typeidx> = (eq i)
		//   typebound 0x01            = (sub resource)  [no payload]
		desc.Kind = component.ImportExternDescType
		boundTag, err := r.ReadByte()
		if err != nil {
			return desc, fmt.Errorf("read typebound tag: %w", err)
		}
		switch boundTag {
		case 0x00: // (eq i)
			idx, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return desc, fmt.Errorf("decode typebound eq typeidx: %w", err)
			}
			desc.TypeBoundKind = component.TypeBoundEq
			desc.TypeBoundIdx = &idx
		case 0x01: // (sub resource)
			desc.TypeBoundKind = component.TypeBoundSubResource
			desc.TypeBoundIdx = nil
		default:
			return desc, fmt.Errorf("unknown typebound tag: 0x%02x", boundTag)
		}

	case 0x04:
		desc.Kind = component.ImportExternDescComponent
		desc.TypeIdx, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return desc, fmt.Errorf("reading component type index: %w", err)
		}

	case 0x05:
		desc.Kind = component.ImportExternDescInstance
		desc.TypeIdx, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return desc, fmt.Errorf("reading instance type index: %w", err)
		}

	default:
		return desc, fmt.Errorf("unknown externdesc kind: 0x%02x", kindByte)
	}

	return desc, nil
}

// decodeImport decodes a single import.
func decodeImport(dc *decodeContext, r *bytes.Reader) (component.Import, error) {
	var imp component.Import

	name, err := decodeImportName(r)
	if err != nil {
		return imp, fmt.Errorf("decoding import name: %w", err)
	}
	imp.Name = name

	desc, err := decodeExternDesc(dc, r)
	if err != nil {
		return imp, fmt.Errorf("decoding externdesc: %w", err)
	}
	imp.ExternDesc = desc

	return imp, nil
}

// decodeImportSection parses the import section (section ID 10).
// Multiple import sections may exist; imports are accumulated.
//
// Each import introduces a new entry into its corresponding index
// space. Type imports additionally register scope entries so that
// later type-section entries (own<>, borrow<>, type-index references)
// can resolve against them.
//
// Spec: Binary.md §2.5 – each import "introduces a new index that
// aliases the imported definition" into the relevant index space.
func decodeImportSection(dc *decodeContext, r *bytes.Reader) error {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("reading import count: %w", err)
	}

	startIdx := uint32(len(dc.c.Imports))
	for i := uint32(0); i < count; i++ {
		imp, err := decodeImport(dc, r)
		if err != nil {
			return fmt.Errorf("decoding import %d: %w", startIdx+i, err)
		}
		dc.c.Imports = append(dc.c.Imports, imp)

		// Update index spaces and scope based on the import kind.
		switch imp.ExternDesc.Kind {
		case component.ImportExternDescType:
			switch imp.ExternDesc.TypeBoundKind {
			case component.TypeBoundSubResource:
				// (sub resource) introduces a fresh abstract resource
				// into the type index space and scope.
				rtIdx := dc.builder.InternAbstractResource()
				dc.scope.appendResource(rtIdx)
				dc.c.TypeDefs = append(dc.c.TypeDefs, component.TypeDef{
					Kind:     component.TypeDefKindResource,
					Resource: rtIdx,
				})
			case component.TypeBoundEq:
				// (eq i) aliases the existing type at index i.
				idx := *imp.ExternDesc.TypeBoundIdx
				if int(idx) >= len(dc.scope.entries) {
					return fmt.Errorf("decoding import %d: typebound eq index %d out of range", startIdx+i, idx)
				}
				src := dc.scope.entries[idx]
				dc.scope.entries = append(dc.scope.entries, src)
				dc.c.TypeDefs = append(dc.c.TypeDefs, component.TypeDef{
					Kind: component.TypeDefKindAlias,
				})
			}
			dc.c.NextTypeIdx++

		case component.ImportExternDescFunc:
			dc.c.NextFuncIdx++

		case component.ImportExternDescComponent:
			dc.c.NextComponentIdx++

		case component.ImportExternDescInstance:
			dc.c.NextComponentInstanceIdx++

		case component.ImportExternDescValue:
			dc.c.NextValueIdx++
		}
	}

	return nil
}
