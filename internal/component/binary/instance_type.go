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

// decodeInstanceTypeDef decodes an instance type definition.
//
// In Session 0 the nested declarations are parsed for bytes-correctness
// only: their shapes are not threaded into any structural type table, so
// nested type slots carry empty payloads. Session 2 will revisit this
// with a scope-aware nested decoder.
//
// Format: 0x42 vec(instancetypedecl)
// instancetypedecl ::= 0x00 core:type   (core type)
//
//	| 0x01 type                 (type)
//	| 0x02 alias                (alias)
//	| 0x04 export               (export)
func decodeInstanceTypeDef(r *bytes.Reader) (*component.InstanceTypeDef, error) {
	declCount, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read declaration count: %w", err)
	}

	decls := make([]component.InstanceDecl, declCount)
	for i := uint32(0); i < declCount; i++ {
		decl, err := decodeInstanceDecl(r)
		if err != nil {
			return nil, fmt.Errorf("decode declaration %d: %w", i, err)
		}
		decls[i] = decl
	}

	return &component.InstanceTypeDef{Declarations: decls}, nil
}

// decodeInstanceDecl decodes a single instance type declaration.
func decodeInstanceDecl(r *bytes.Reader) (component.InstanceDecl, error) {
	var decl component.InstanceDecl

	kindByte, err := r.ReadByte()
	if err != nil {
		return decl, fmt.Errorf("read declaration kind: %w", err)
	}
	decl.Kind = component.InstanceDeclKind(kindByte)

	switch decl.Kind {
	case component.InstanceDeclKindCoreType:
		coreType, err := decodeCoreTypeDefForInstance(r)
		if err != nil {
			return decl, fmt.Errorf("decode core type: %w", err)
		}
		decl.CoreType = coreType

	case component.InstanceDeclKindType:
		typeDef, err := decodeNestedTypeDef(r)
		if err != nil {
			return decl, fmt.Errorf("decode type: %w", err)
		}
		decl.Type = typeDef

	case component.InstanceDeclKindAlias:
		alias, err := decodeAlias(r)
		if err != nil {
			return decl, fmt.Errorf("decode alias: %w", err)
		}
		decl.Alias = &alias

	case component.InstanceDeclKindExport:
		export, err := decodeInstanceExportDecl(r)
		if err != nil {
			return decl, fmt.Errorf("decode export: %w", err)
		}
		decl.Export = export

	default:
		return decl, fmt.Errorf("unknown instance declaration kind: 0x%02x", kindByte)
	}

	return decl, nil
}

// decodeInstanceExportDecl decodes an export declaration within an
// instance type.
//
// Format: exportname externdesc
// externdesc format: kind followed by type-specific data
func decodeInstanceExportDecl(r *bytes.Reader) (*component.InstanceExport, error) {
	name, err := decodeExportName(r)
	if err != nil {
		return nil, fmt.Errorf("decode export name: %w", err)
	}

	export := &component.InstanceExport{
		Name: name,
	}

	kindByte, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("read externdesc kind: %w", err)
	}

	switch kindByte {
	case 0x00:
		// Core module: expect 0x11 prefix then core type index.
		prefix, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("read core module prefix: %w", err)
		}
		if prefix != 0x11 {
			return nil, fmt.Errorf("expected 0x11 for core module, got 0x%02x", prefix)
		}
		export.Kind = component.ExportKindFunc // Core module tracked as func for now.
		idx, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read core type index: %w", err)
		}
		export.Idx = idx

	case 0x01:
		// Function: typeidx.
		export.Kind = component.ExportKindFunc
		idx, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read func type index: %w", err)
		}
		export.Idx = idx

	case 0x02:
		// Value: valtype.
		export.Kind = component.ExportKindValue
		if err := skipValType(r); err != nil {
			return nil, fmt.Errorf("skip value type: %w", err)
		}

	case 0x03:
		// Type: typebound.
		// Format: 0x00 typeidx (eq bound) or 0x01 (sub-resource, fresh).
		export.Kind = component.ExportKindType
		boundKind, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("read type bound kind: %w", err)
		}
		switch boundKind {
		case 0x00:
			idx, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return nil, fmt.Errorf("read eq type bound index: %w", err)
			}
			export.Idx = idx
		case 0x01:
			// Sub-resource: fresh resource type, no index follows.
		default:
			return nil, fmt.Errorf("unknown type bound kind: 0x%02x", boundKind)
		}

	case 0x04:
		export.Kind = component.ExportKindComponent
		idx, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read component type index: %w", err)
		}
		export.Idx = idx

	case 0x05:
		export.Kind = component.ExportKindInstance
		idx, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read instance type index: %w", err)
		}
		export.Idx = idx

	default:
		return nil, fmt.Errorf("unknown externdesc kind: 0x%02x", kindByte)
	}

	return export, nil
}

// decodeCoreTypeDefForInstance decodes a core type definition for use in
// instance/component types.
func decodeCoreTypeDefForInstance(r *bytes.Reader) (*component.CoreTypeDef, error) {
	opcode, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("read core type opcode: %w", err)
	}

	switch opcode {
	case 0x60: // func type
		funcType, err := decodeCoreFunc(r)
		if err != nil {
			return nil, fmt.Errorf("decode core func: %w", err)
		}
		return &component.CoreTypeDef{
			Kind: component.CoreTypeDefKindFunc,
			Func: funcType,
		}, nil
	case 0x50: // module type
		moduleType, err := decodeCoreModuleType(r)
		if err != nil {
			return nil, fmt.Errorf("decode core module: %w", err)
		}
		return &component.CoreTypeDef{
			Kind:   component.CoreTypeDefKindModule,
			Module: moduleType,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported core type opcode: 0x%02x", opcode)
	}
}

// decodeNestedTypeDef decodes a type definition nested within an
// instance/component type.
//
// Session 0 scope: the nested decoder parses enough bytes to keep the
// binary stream in lockstep with the spec. Composite value-type payloads
// are consumed via skipDefinedType so the outer stream advances
// correctly, but no interning happens and the returned TypeDef carries
// only the Kind discriminator. Session 2 will wire a nested
// *typeScope / *ComponentTypesBuilder through this path so the nested
// types actually land in the parent component's table.
func decodeNestedTypeDef(r *bytes.Reader) (*component.TypeDef, error) {
	opcode, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("read type opcode: %w", err)
	}

	typeDef := &component.TypeDef{}

	// Primitive value types as a bare defvaltype.
	if IsPrimValType(opcode) {
		typeDef.Kind = component.TypeDefKindDefined
		return typeDef, nil
	}

	switch opcode {
	case TypeOpFuncSync, TypeOpFuncAsync:
		// Skip the rest of the function type: params and results.
		if err := skipFuncTypeBody(r); err != nil {
			return nil, fmt.Errorf("skip nested func type: %w", err)
		}
		typeDef.Kind = component.TypeDefKindFunc

	case TypeOpInstance:
		inst, err := decodeInstanceTypeDef(r)
		if err != nil {
			return nil, err
		}
		typeDef.Kind = component.TypeDefKindInstance
		typeDef.Instance = inst

	case TypeOpResourceSync:
		if err := skipResourceTypeBody(r, false); err != nil {
			return nil, fmt.Errorf("skip nested resource type: %w", err)
		}
		typeDef.Kind = component.TypeDefKindResource

	case TypeOpResourceAsync:
		if err := skipResourceTypeBody(r, true); err != nil {
			return nil, fmt.Errorf("skip nested async resource type: %w", err)
		}
		typeDef.Kind = component.TypeDefKindResource

	case TypeOpComponent:
		comp, err := decodeComponentTypeDef(r)
		if err != nil {
			return nil, err
		}
		typeDef.Kind = component.TypeDefKindComponent
		typeDef.Component = comp

	case ValTypeOpcodeOwn, ValTypeOpcodeBorrow:
		// own<T> or borrow<T> where T is a resource type index.
		if _, _, err := leb128.DecodeUint32(r); err != nil {
			return nil, fmt.Errorf("read handle type index: %w", err)
		}
		typeDef.Kind = component.TypeDefKindDefined

	case ValTypeOpcodeRecord, ValTypeOpcodeVariant, ValTypeOpcodeList,
		ValTypeOpcodeTuple, ValTypeOpcodeFlags, ValTypeOpcodeEnum,
		ValTypeOpcodeOption, ValTypeOpcodeResult,
		ValTypeOpcodeStream, ValTypeOpcodeFuture, ValTypeOpcodeFixedSizeList:
		if err := skipDefinedTypeBody(r, opcode); err != nil {
			return nil, fmt.Errorf("skip nested defined type: %w", err)
		}
		typeDef.Kind = component.TypeDefKindDefined

	default:
		return nil, fmt.Errorf("unsupported nested type opcode: 0x%02x", opcode)
	}

	return typeDef, nil
}

// skipValType advances the reader past a single valtype without
// interning anything. Used by nested declarations where only byte-level
// correctness matters in Session 0.
func skipValType(r *bytes.Reader) error {
	opcode, err := r.ReadByte()
	if err != nil {
		return err
	}
	if IsPrimValType(opcode) {
		return nil
	}
	if opcode == ValTypeOpcodeOwn || opcode == ValTypeOpcodeBorrow {
		_, _, err := leb128.DecodeUint32(r)
		return err
	}
	// Type index: unread and decode the full LEB128.
	if err := r.UnreadByte(); err != nil {
		return err
	}
	_, _, err = leb128.DecodeUint32(r)
	return err
}

// skipDefinedTypeBody consumes the payload of a defvaltype whose leading
// opcode has already been read. It mirrors the per-kind decode helpers
// above but discards the results.
func skipDefinedTypeBody(r *bytes.Reader, opcode byte) error {
	switch opcode {
	case ValTypeOpcodeRecord:
		count, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return err
		}
		for i := uint32(0); i < count; i++ {
			if _, err := decodeName(r); err != nil {
				return err
			}
			if err := skipValType(r); err != nil {
				return err
			}
		}
	case ValTypeOpcodeVariant:
		count, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return err
		}
		for i := uint32(0); i < count; i++ {
			if _, err := decodeName(r); err != nil {
				return err
			}
			typeFlag, err := r.ReadByte()
			if err != nil {
				return err
			}
			if typeFlag == 0x01 {
				if err := skipValType(r); err != nil {
					return err
				}
			}
			refinesFlag, err := r.ReadByte()
			if err != nil {
				return err
			}
			if refinesFlag == 0x01 {
				if _, _, err := leb128.DecodeUint32(r); err != nil {
					return err
				}
			}
		}
	case ValTypeOpcodeList:
		return skipValType(r)
	case ValTypeOpcodeFixedSizeList:
		if err := skipValType(r); err != nil {
			return err
		}
		_, _, err := leb128.DecodeUint32(r)
		return err
	case ValTypeOpcodeTuple:
		count, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return err
		}
		for i := uint32(0); i < count; i++ {
			if err := skipValType(r); err != nil {
				return err
			}
		}
	case ValTypeOpcodeFlags, ValTypeOpcodeEnum:
		count, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return err
		}
		for i := uint32(0); i < count; i++ {
			if _, err := decodeName(r); err != nil {
				return err
			}
		}
	case ValTypeOpcodeOption:
		return skipValType(r)
	case ValTypeOpcodeResult:
		okFlag, err := r.ReadByte()
		if err != nil {
			return err
		}
		if okFlag == 0x01 {
			if err := skipValType(r); err != nil {
				return err
			}
		}
		errFlag, err := r.ReadByte()
		if err != nil {
			return err
		}
		if errFlag == 0x01 {
			return skipValType(r)
		}
	case ValTypeOpcodeStream, ValTypeOpcodeFuture:
		hasElem, err := r.ReadByte()
		if err != nil {
			return err
		}
		if hasElem == 0x01 {
			return skipValType(r)
		}
	default:
		return fmt.Errorf("skipDefinedTypeBody: unknown opcode 0x%02x", opcode)
	}
	return nil
}

// skipFuncTypeBody consumes params and results of a component function
// type. The leading 0x40 / 0x43 opcode has already been read.
func skipFuncTypeBody(r *bytes.Reader) error {
	paramCount, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return err
	}
	for i := uint32(0); i < paramCount; i++ {
		if _, err := decodeName(r); err != nil {
			return err
		}
		if err := skipValType(r); err != nil {
			return err
		}
	}
	resultTag, err := r.ReadByte()
	if err != nil {
		return err
	}
	switch resultTag {
	case ResultSingle:
		return skipValType(r)
	case ResultNamed:
		count, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return err
		}
		for i := uint32(0); i < count; i++ {
			if _, err := decodeName(r); err != nil {
				return err
			}
			if err := skipValType(r); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("invalid result tag: 0x%02x", resultTag)
	}
	return nil
}

// skipResourceTypeBody consumes the body of a resource declaration.
func skipResourceTypeBody(r *bytes.Reader, isAsync bool) error {
	rep, err := r.ReadByte()
	if err != nil {
		return err
	}
	if rep != 0x7f {
		return fmt.Errorf("unsupported resource rep type: 0x%02x", rep)
	}
	if isAsync {
		if _, _, err := leb128.DecodeUint32(r); err != nil {
			return err
		}
		cbFlag, err := r.ReadByte()
		if err != nil {
			return err
		}
		if cbFlag == 0x01 {
			if _, _, err := leb128.DecodeUint32(r); err != nil {
				return err
			}
		}
		return nil
	}
	dtorFlag, err := r.ReadByte()
	if err != nil {
		return err
	}
	if dtorFlag == 0x01 {
		if _, _, err := leb128.DecodeUint32(r); err != nil {
			return err
		}
	}
	return nil
}
