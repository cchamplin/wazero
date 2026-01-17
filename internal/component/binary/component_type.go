// internal/component/binary/component_type.go

package binary

import (
	"bytes"
	"fmt"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/leb128"
)

// decodeComponentTypeDef decodes a component type definition.
// Format: 0x41 vec(componenttypedecl)
// componenttypedecl ::= 0x00 core:type      (core type)
//
//	| 0x01 type           (type)
//	| 0x02 alias          (alias)
//	| 0x03 import         (import)
//	| 0x04 export         (export)
func decodeComponentTypeDef(r *bytes.Reader) (*component.ComponentTypeDef, error) {
	declCount, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read declaration count: %w", err)
	}

	decls := make([]component.ComponentDecl, declCount)
	for i := uint32(0); i < declCount; i++ {
		decl, err := decodeComponentDecl(r)
		if err != nil {
			return nil, fmt.Errorf("decode declaration %d: %w", i, err)
		}
		decls[i] = decl
	}

	return &component.ComponentTypeDef{Declarations: decls}, nil
}

// decodeComponentDecl decodes a single component type declaration.
func decodeComponentDecl(r *bytes.Reader) (component.ComponentDecl, error) {
	var decl component.ComponentDecl

	kindByte, err := r.ReadByte()
	if err != nil {
		return decl, fmt.Errorf("read declaration kind: %w", err)
	}
	decl.Kind = component.ComponentDeclKind(kindByte)

	switch decl.Kind {
	case component.ComponentDeclKindCoreType:
		coreType, err := decodeCoreTypeDefForInstance(r)
		if err != nil {
			return decl, fmt.Errorf("decode core type: %w", err)
		}
		decl.CoreType = coreType

	case component.ComponentDeclKindType:
		typeDef, err := decodeNestedTypeDef(r)
		if err != nil {
			return decl, fmt.Errorf("decode type: %w", err)
		}
		decl.Type = typeDef

	case component.ComponentDeclKindAlias:
		alias, err := decodeAlias(r)
		if err != nil {
			return decl, fmt.Errorf("decode alias: %w", err)
		}
		decl.Alias = &alias

	case component.ComponentDeclKindImport:
		imp, err := decodeImport(r)
		if err != nil {
			return decl, fmt.Errorf("decode import: %w", err)
		}
		decl.Import = &imp

	case component.ComponentDeclKindExport:
		export, err := decodeInstanceExportDecl(r)
		if err != nil {
			return decl, fmt.Errorf("decode export: %w", err)
		}
		decl.Export = export

	default:
		return decl, fmt.Errorf("unknown component declaration kind: 0x%02x", kindByte)
	}

	return decl, nil
}
