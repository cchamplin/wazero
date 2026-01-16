// internal/component/type_resolver.go
package component

import (
	"fmt"

	"github.com/tetratelabs/wazero/internal/component/types"
)

// TypeResolver resolves ValTypeRef to concrete types.ValType.
type TypeResolver struct {
	component *Component
	cache     map[uint32]types.ValType
}

// NewTypeResolver creates a new TypeResolver for the given component.
func NewTypeResolver(c *Component) *TypeResolver {
	return &TypeResolver{
		component: c,
		cache:     make(map[uint32]types.ValType),
	}
}

// ResolveValType converts a ValTypeRef to a concrete types.ValType.
func (r *TypeResolver) ResolveValType(ref ValTypeRef) (types.ValType, error) {
	if ref.IsPrimitive {
		return r.resolvePrimitive(ref.Primitive)
	}

	if ref.IsOwn {
		return types.Own{ResourceIdx: ref.TypeIdx}, nil
	}

	if ref.IsBorrow {
		return types.Borrow{ResourceIdx: ref.TypeIdx}, nil
	}

	return r.resolveTypeIdx(ref.TypeIdx)
}

func (r *TypeResolver) resolvePrimitive(opcode byte) (types.ValType, error) {
	switch opcode {
	case 0x7f:
		return types.Bool{}, nil
	case 0x7e:
		return types.S8{}, nil
	case 0x7d:
		return types.U8{}, nil
	case 0x7c:
		return types.S16{}, nil
	case 0x7b:
		return types.U16{}, nil
	case 0x7a:
		return types.S32{}, nil
	case 0x79:
		return types.U32{}, nil
	case 0x78:
		return types.S64{}, nil
	case 0x77:
		return types.U64{}, nil
	case 0x76:
		return types.F32{}, nil
	case 0x75:
		return types.F64{}, nil
	case 0x74:
		return types.Char{}, nil
	case 0x73:
		return types.String{}, nil
	default:
		return nil, fmt.Errorf("unknown primitive opcode: 0x%02x", opcode)
	}
}

func (r *TypeResolver) resolveTypeIdx(idx uint32) (types.ValType, error) {
	if cached, ok := r.cache[idx]; ok {
		return cached, nil
	}

	if int(idx) >= len(r.component.Types) {
		return nil, fmt.Errorf("type index out of bounds: %d (have %d types)", idx, len(r.component.Types))
	}

	typeDef := &r.component.Types[idx]

	var result types.ValType
	var err error

	switch typeDef.Kind {
	case TypeDefKindDefined:
		result, err = r.resolveDefinedType(typeDef)
	case TypeDefKindFunc:
		return nil, fmt.Errorf("cannot use function type as value type")
	case TypeDefKindResource:
		return nil, fmt.Errorf("cannot use resource type directly (use own<T> or borrow<T>)")
	default:
		return nil, fmt.Errorf("unsupported type def kind: %d", typeDef.Kind)
	}

	if err != nil {
		return nil, err
	}

	r.cache[idx] = result
	return result, nil
}

func (r *TypeResolver) resolveDefinedType(typeDef *TypeDef) (types.ValType, error) {
	if typeDef.Record != nil {
		return r.resolveRecord(typeDef.Record)
	}
	if typeDef.Variant != nil {
		return r.resolveVariant(typeDef.Variant)
	}
	if typeDef.List != nil {
		return r.resolveList(typeDef.List)
	}
	if typeDef.Option != nil {
		return r.resolveOption(typeDef.Option)
	}
	if typeDef.Result != nil {
		return r.resolveResult(typeDef.Result)
	}
	if typeDef.Tuple != nil {
		return r.resolveTuple(typeDef.Tuple)
	}
	if typeDef.Flags != nil {
		return r.resolveFlags(typeDef.Flags)
	}
	if typeDef.Enum != nil {
		return r.resolveEnum(typeDef.Enum)
	}
	return nil, fmt.Errorf("unhandled defined type")
}

func (r *TypeResolver) resolveRecord(def *RecordTypeDef) (types.ValType, error) {
	fields := make([]types.Field, len(def.Fields))
	for i, f := range def.Fields {
		fieldType, err := r.ResolveValType(f.ValType)
		if err != nil {
			return nil, fmt.Errorf("resolve field %q: %w", f.Name, err)
		}
		fields[i] = types.Field{Name: f.Name, Type: fieldType}
	}
	return types.Record{Fields: fields}, nil
}

func (r *TypeResolver) resolveVariant(def *VariantTypeDef) (types.ValType, error) {
	cases := make([]types.Case, len(def.Cases))
	for i, c := range def.Cases {
		var caseType types.ValType
		if c.ValType != nil {
			var err error
			caseType, err = r.ResolveValType(*c.ValType)
			if err != nil {
				return nil, fmt.Errorf("resolve case %q: %w", c.Name, err)
			}
		}
		cases[i] = types.Case{Name: c.Name, Type: caseType}
	}
	return types.Variant{Cases: cases}, nil
}

func (r *TypeResolver) resolveList(def *ListTypeDef) (types.ValType, error) {
	elemType, err := r.ResolveValType(def.ElementType)
	if err != nil {
		return nil, fmt.Errorf("resolve list element: %w", err)
	}
	return types.List{Element: elemType}, nil
}

func (r *TypeResolver) resolveOption(def *OptionTypeDef) (types.ValType, error) {
	innerType, err := r.ResolveValType(def.InnerType)
	if err != nil {
		return nil, fmt.Errorf("resolve option inner: %w", err)
	}
	return types.Option{Some: innerType}, nil
}

func (r *TypeResolver) resolveResult(def *ResultTypeDef) (types.ValType, error) {
	var okType, errType types.ValType
	var err error

	if def.OkType != nil {
		okType, err = r.ResolveValType(*def.OkType)
		if err != nil {
			return nil, fmt.Errorf("resolve result ok: %w", err)
		}
	}

	if def.ErrType != nil {
		errType, err = r.ResolveValType(*def.ErrType)
		if err != nil {
			return nil, fmt.Errorf("resolve result err: %w", err)
		}
	}

	return types.Result{Ok: okType, Error: errType}, nil
}

func (r *TypeResolver) resolveTuple(def *TupleTypeDef) (types.ValType, error) {
	elemTypes := make([]types.ValType, len(def.Types))
	for i, t := range def.Types {
		elemType, err := r.ResolveValType(t)
		if err != nil {
			return nil, fmt.Errorf("resolve tuple element %d: %w", i, err)
		}
		elemTypes[i] = elemType
	}
	return types.Tuple{Types: elemTypes}, nil
}

func (r *TypeResolver) resolveFlags(def *FlagsTypeDef) (types.ValType, error) {
	return types.Flags{Names: def.Names}, nil
}

func (r *TypeResolver) resolveEnum(def *EnumTypeDef) (types.ValType, error) {
	return types.Enum{Cases: def.Names}, nil
}
