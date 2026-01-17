// internal/component/binary/instance_type.go

package binary

import (
	"bytes"
	"fmt"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/leb128"
)

// decodeInstanceTypeDef decodes an instance type definition.
// Format: 0x42 vec(instancetypedecl)
// instancetypedecl ::= 0x00 core:type         (core type)
//
//	| 0x01 type              (type)
//	| 0x02 alias             (alias)
//	| 0x04 export            (export)
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
		// Core type declaration
		coreType, err := decodeCoreTypeDefForInstance(r)
		if err != nil {
			return decl, fmt.Errorf("decode core type: %w", err)
		}
		decl.CoreType = coreType

	case component.InstanceDeclKindType:
		// Nested type declaration - recursively decode
		typeDef, err := decodeNestedTypeDef(r)
		if err != nil {
			return decl, fmt.Errorf("decode type: %w", err)
		}
		decl.Type = typeDef

	case component.InstanceDeclKindAlias:
		// Alias declaration
		alias, err := decodeAlias(r)
		if err != nil {
			return decl, fmt.Errorf("decode alias: %w", err)
		}
		decl.Alias = &alias

	case component.InstanceDeclKindExport:
		// Export declaration
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

// decodeInstanceExportDecl decodes an export declaration within an instance type.
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

	// Read externdesc kind byte
	kindByte, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("read externdesc kind: %w", err)
	}

	// Handle externdesc based on kind
	switch kindByte {
	case 0x00:
		// Core module: expect 0x11 prefix then core type index
		prefix, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("read core module prefix: %w", err)
		}
		if prefix != 0x11 {
			return nil, fmt.Errorf("expected 0x11 for core module, got 0x%02x", prefix)
		}
		export.Kind = component.ExportKindFunc // Core module tracked as func for now
		idx, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read core type index: %w", err)
		}
		export.Idx = idx

	case 0x01:
		// Function: typeidx
		export.Kind = component.ExportKindFunc
		idx, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read func type index: %w", err)
		}
		export.Idx = idx

	case 0x02:
		// Value: valtype
		export.Kind = component.ExportKindValue
		_, err := decodeValType(r)
		if err != nil {
			return nil, fmt.Errorf("decode value type: %w", err)
		}
		// Value exports don't have an index in the same way

	case 0x03:
		// Type: typebound
		// Format: 0x00 typeidx (sub resource bound) or 0x01 typeidx (eq bound)
		// Note: For type exports in instance types, we may just have a type index
		// without a bound tag, depending on the version of the spec.
		export.Kind = component.ExportKindType
		// Read the type bound or index
		idx, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read type bound: %w", err)
		}
		export.Idx = idx

	case 0x04:
		// Component: typeidx
		export.Kind = component.ExportKindComponent
		idx, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read component type index: %w", err)
		}
		export.Idx = idx

	case 0x05:
		// Instance: typeidx
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

// decodeCoreTypeDefForInstance decodes a core type definition for use in instance/component types.
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

// decodeNestedTypeDef decodes a type definition nested within an instance/component type.
func decodeNestedTypeDef(r *bytes.Reader) (*component.TypeDef, error) {
	opcode, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("read type opcode: %w", err)
	}

	typeDef := &component.TypeDef{}

	switch opcode {
	case TypeOpFuncSync, TypeOpFuncAsync:
		if err := r.UnreadByte(); err != nil {
			return nil, err
		}
		ft, err := decodeFuncType(r)
		if err != nil {
			return nil, err
		}
		typeDef.Kind = component.TypeDefKindFunc
		typeDef.Func = ft

	case TypeOpInstance:
		inst, err := decodeInstanceTypeDef(r)
		if err != nil {
			return nil, err
		}
		typeDef.Kind = component.TypeDefKindInstance
		typeDef.Instance = inst

	case ValTypeOpcodeRecord:
		record, err := decodeRecordTypeDef(r)
		if err != nil {
			return nil, err
		}
		typeDef.Kind = component.TypeDefKindDefined
		typeDef.Record = convertRecordTypeDef(record)

	case ValTypeOpcodeVariant:
		variant, err := decodeVariantTypeDef(r)
		if err != nil {
			return nil, err
		}
		typeDef.Kind = component.TypeDefKindDefined
		typeDef.Variant = convertVariantTypeDef(variant)

	case ValTypeOpcodeList:
		list, err := decodeListTypeDef(r)
		if err != nil {
			return nil, err
		}
		typeDef.Kind = component.TypeDefKindDefined
		typeDef.List = convertListTypeDef(list)

	case ValTypeOpcodeTuple:
		tuple, err := decodeTupleTypeDef(r)
		if err != nil {
			return nil, err
		}
		typeDef.Kind = component.TypeDefKindDefined
		typeDef.Tuple = convertTupleTypeDef(tuple)

	case ValTypeOpcodeFlags:
		flags, err := decodeFlagsTypeDef(r)
		if err != nil {
			return nil, err
		}
		typeDef.Kind = component.TypeDefKindDefined
		typeDef.Flags = convertFlagsTypeDef(flags)

	case ValTypeOpcodeEnum:
		enum, err := decodeEnumTypeDef(r)
		if err != nil {
			return nil, err
		}
		typeDef.Kind = component.TypeDefKindDefined
		typeDef.Enum = convertEnumTypeDef(enum)

	case ValTypeOpcodeOption:
		option, err := decodeOptionTypeDef(r)
		if err != nil {
			return nil, err
		}
		typeDef.Kind = component.TypeDefKindDefined
		typeDef.Option = convertOptionTypeDef(option)

	case ValTypeOpcodeResult:
		result, err := decodeResultTypeDef(r)
		if err != nil {
			return nil, err
		}
		typeDef.Kind = component.TypeDefKindDefined
		typeDef.Result = convertResultTypeDef(result)

	case TypeOpResourceSync:
		resourceDef, err := decodeResourceTypeDef(r)
		if err != nil {
			return nil, err
		}
		typeDef.Kind = component.TypeDefKindResource
		typeDef.Resource = resourceDef

	default:
		return nil, fmt.Errorf("unsupported nested type opcode: 0x%02x", opcode)
	}

	return typeDef, nil
}
