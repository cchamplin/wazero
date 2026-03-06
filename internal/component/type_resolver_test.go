// internal/component/type_resolver_test.go
package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component/types"
)

func TestTypeResolverPrimitive(t *testing.T) {
	c := &Component{}
	resolver := NewTypeResolver(c)

	tests := []struct {
		name     string
		ref      ValTypeRef
		expected types.ValType
	}{
		{"s32", ValTypeRef{IsPrimitive: true, Primitive: 0x7a}, types.S32{}},
		{"u32", ValTypeRef{IsPrimitive: true, Primitive: 0x79}, types.U32{}},
		{"s64", ValTypeRef{IsPrimitive: true, Primitive: 0x78}, types.S64{}},
		{"u64", ValTypeRef{IsPrimitive: true, Primitive: 0x77}, types.U64{}},
		{"f32", ValTypeRef{IsPrimitive: true, Primitive: 0x76}, types.F32{}},
		{"f64", ValTypeRef{IsPrimitive: true, Primitive: 0x75}, types.F64{}},
		{"bool", ValTypeRef{IsPrimitive: true, Primitive: 0x7f}, types.Bool{}},
		{"char", ValTypeRef{IsPrimitive: true, Primitive: 0x74}, types.Char{}},
		{"string", ValTypeRef{IsPrimitive: true, Primitive: 0x73}, types.String{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolver.ResolveValType(tt.ref)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %T, got %T", tt.expected, result)
			}
		})
	}
}

func TestTypeResolverRecord(t *testing.T) {
	c := &Component{
		Types: []TypeDef{
			{
				Kind: TypeDefKindDefined,
				Record: &RecordTypeDef{
					Fields: []RecordField{
						{Name: "x", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}},
						{Name: "y", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}},
					},
				},
			},
		},
	}
	resolver := NewTypeResolver(c)

	ref := ValTypeRef{IsPrimitive: false, TypeIdx: 0}
	result, err := resolver.ResolveValType(ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	record, ok := result.(types.Record)
	if !ok {
		t.Fatalf("expected Record, got %T", result)
	}

	if len(record.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(record.Fields))
	}
}

func TestTypeResolverOwn(t *testing.T) {
	c := &Component{}
	resolver := NewTypeResolver(c)

	ref := ValTypeRef{IsOwn: true, TypeIdx: 5}
	result, err := resolver.ResolveValType(ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	own, ok := result.(types.Own)
	if !ok {
		t.Fatalf("expected Own, got %T", result)
	}

	if own.ResourceIdx != 5 {
		t.Errorf("expected ResourceIdx 5, got %d", own.ResourceIdx)
	}
}

func TestTypeResolverBorrow(t *testing.T) {
	c := &Component{}
	resolver := NewTypeResolver(c)

	ref := ValTypeRef{IsBorrow: true, TypeIdx: 3}
	result, err := resolver.ResolveValType(ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	borrow, ok := result.(types.Borrow)
	if !ok {
		t.Fatalf("expected Borrow, got %T", result)
	}

	if borrow.ResourceIdx != 3 {
		t.Errorf("expected ResourceIdx 3, got %d", borrow.ResourceIdx)
	}
}

func TestTypeResolverVariant(t *testing.T) {
	c := &Component{
		Types: []TypeDef{
			{
				Kind: TypeDefKindDefined,
				Variant: &VariantTypeDef{
					Cases: []VariantCase{
						{Name: "none", ValType: nil},
						{Name: "some", ValType: &ValTypeRef{IsPrimitive: true, Primitive: 0x7a}},
					},
				},
			},
		},
	}
	resolver := NewTypeResolver(c)

	ref := ValTypeRef{IsPrimitive: false, TypeIdx: 0}
	result, err := resolver.ResolveValType(ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	variant, ok := result.(types.Variant)
	if !ok {
		t.Fatalf("expected Variant, got %T", result)
	}

	if len(variant.Cases) != 2 {
		t.Errorf("expected 2 cases, got %d", len(variant.Cases))
	}
}

func TestTypeResolverList(t *testing.T) {
	c := &Component{
		Types: []TypeDef{
			{
				Kind: TypeDefKindDefined,
				List: &ListTypeDef{
					ElementType: ValTypeRef{IsPrimitive: true, Primitive: 0x79}, // u32
				},
			},
		},
	}
	resolver := NewTypeResolver(c)

	ref := ValTypeRef{IsPrimitive: false, TypeIdx: 0}
	result, err := resolver.ResolveValType(ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	list, ok := result.(types.List)
	if !ok {
		t.Fatalf("expected List, got %T", result)
	}

	if _, ok := list.Element.(types.U32); !ok {
		t.Errorf("expected element type U32, got %T", list.Element)
	}
}

func TestTypeResolverOption(t *testing.T) {
	c := &Component{
		Types: []TypeDef{
			{
				Kind: TypeDefKindDefined,
				Option: &OptionTypeDef{
					InnerType: ValTypeRef{IsPrimitive: true, Primitive: 0x73}, // string
				},
			},
		},
	}
	resolver := NewTypeResolver(c)

	ref := ValTypeRef{IsPrimitive: false, TypeIdx: 0}
	result, err := resolver.ResolveValType(ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	option, ok := result.(types.Option)
	if !ok {
		t.Fatalf("expected Option, got %T", result)
	}

	if _, ok := option.Some.(types.String); !ok {
		t.Errorf("expected inner type String, got %T", option.Some)
	}
}

func TestTypeResolverResult(t *testing.T) {
	okType := ValTypeRef{IsPrimitive: true, Primitive: 0x7a}  // s32
	errType := ValTypeRef{IsPrimitive: true, Primitive: 0x73} // string

	c := &Component{
		Types: []TypeDef{
			{
				Kind: TypeDefKindDefined,
				Result: &ResultTypeDef{
					OkType:  &okType,
					ErrType: &errType,
				},
			},
		},
	}
	resolver := NewTypeResolver(c)

	ref := ValTypeRef{IsPrimitive: false, TypeIdx: 0}
	result, err := resolver.ResolveValType(ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultType, ok := result.(types.Result)
	if !ok {
		t.Fatalf("expected Result, got %T", result)
	}

	if _, ok := resultType.Ok.(types.S32); !ok {
		t.Errorf("expected ok type S32, got %T", resultType.Ok)
	}

	if _, ok := resultType.Error.(types.String); !ok {
		t.Errorf("expected error type String, got %T", resultType.Error)
	}
}

func TestTypeResolverTuple(t *testing.T) {
	c := &Component{
		Types: []TypeDef{
			{
				Kind: TypeDefKindDefined,
				Tuple: &TupleTypeDef{
					Types: []ValTypeRef{
						{IsPrimitive: true, Primitive: 0x7a}, // s32
						{IsPrimitive: true, Primitive: 0x73}, // string
						{IsPrimitive: true, Primitive: 0x7f}, // bool
					},
				},
			},
		},
	}
	resolver := NewTypeResolver(c)

	ref := ValTypeRef{IsPrimitive: false, TypeIdx: 0}
	result, err := resolver.ResolveValType(ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tuple, ok := result.(types.Tuple)
	if !ok {
		t.Fatalf("expected Tuple, got %T", result)
	}

	if len(tuple.Types) != 3 {
		t.Errorf("expected 3 types, got %d", len(tuple.Types))
	}

	if _, ok := tuple.Types[0].(types.S32); !ok {
		t.Errorf("expected first type S32, got %T", tuple.Types[0])
	}
}

func TestTypeResolverFlags(t *testing.T) {
	c := &Component{
		Types: []TypeDef{
			{
				Kind: TypeDefKindDefined,
				Flags: &FlagsTypeDef{
					Names: []string{"read", "write", "execute"},
				},
			},
		},
	}
	resolver := NewTypeResolver(c)

	ref := ValTypeRef{IsPrimitive: false, TypeIdx: 0}
	result, err := resolver.ResolveValType(ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	flags, ok := result.(types.Flags)
	if !ok {
		t.Fatalf("expected Flags, got %T", result)
	}

	if len(flags.Names) != 3 {
		t.Errorf("expected 3 flags, got %d", len(flags.Names))
	}

	if flags.Names[0] != "read" {
		t.Errorf("expected first flag 'read', got %q", flags.Names[0])
	}
}

func TestTypeResolverEnum(t *testing.T) {
	c := &Component{
		Types: []TypeDef{
			{
				Kind: TypeDefKindDefined,
				Enum: &EnumTypeDef{
					Names: []string{"red", "green", "blue"},
				},
			},
		},
	}
	resolver := NewTypeResolver(c)

	ref := ValTypeRef{IsPrimitive: false, TypeIdx: 0}
	result, err := resolver.ResolveValType(ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	enum, ok := result.(types.Enum)
	if !ok {
		t.Fatalf("expected Enum, got %T", result)
	}

	if len(enum.Cases) != 3 {
		t.Errorf("expected 3 cases, got %d", len(enum.Cases))
	}

	if enum.Cases[0] != "red" {
		t.Errorf("expected first case 'red', got %q", enum.Cases[0])
	}
}

func TestTypeResolverNestedTypes(t *testing.T) {
	// Test a record containing a list of tuples
	c := &Component{
		Types: []TypeDef{
			// Type 0: tuple (s32, string)
			{
				Kind: TypeDefKindDefined,
				Tuple: &TupleTypeDef{
					Types: []ValTypeRef{
						{IsPrimitive: true, Primitive: 0x7a}, // s32
						{IsPrimitive: true, Primitive: 0x73}, // string
					},
				},
			},
			// Type 1: list<tuple (s32, string)>
			{
				Kind: TypeDefKindDefined,
				List: &ListTypeDef{
					ElementType: ValTypeRef{IsPrimitive: false, TypeIdx: 0},
				},
			},
			// Type 2: record { items: list<tuple (s32, string)> }
			{
				Kind: TypeDefKindDefined,
				Record: &RecordTypeDef{
					Fields: []RecordField{
						{Name: "items", ValType: ValTypeRef{IsPrimitive: false, TypeIdx: 1}},
					},
				},
			},
		},
	}
	resolver := NewTypeResolver(c)

	ref := ValTypeRef{IsPrimitive: false, TypeIdx: 2}
	result, err := resolver.ResolveValType(ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	record, ok := result.(types.Record)
	if !ok {
		t.Fatalf("expected Record, got %T", result)
	}

	if len(record.Fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(record.Fields))
	}

	list, ok := record.Fields[0].Type.(types.List)
	if !ok {
		t.Fatalf("expected field type List, got %T", record.Fields[0].Type)
	}

	tuple, ok := list.Element.(types.Tuple)
	if !ok {
		t.Fatalf("expected list element Tuple, got %T", list.Element)
	}

	if len(tuple.Types) != 2 {
		t.Errorf("expected 2 tuple elements, got %d", len(tuple.Types))
	}
}

func TestTypeResolverCaching(t *testing.T) {
	c := &Component{
		Types: []TypeDef{
			{
				Kind: TypeDefKindDefined,
				Record: &RecordTypeDef{
					Fields: []RecordField{
						{Name: "x", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}},
					},
				},
			},
		},
	}
	resolver := NewTypeResolver(c)

	ref := ValTypeRef{IsPrimitive: false, TypeIdx: 0}

	// Resolve same type twice
	result1, err := resolver.ResolveValType(ref)
	if err != nil {
		t.Fatalf("first resolve error: %v", err)
	}

	result2, err := resolver.ResolveValType(ref)
	if err != nil {
		t.Fatalf("second resolve error: %v", err)
	}

	// Results should be equal (cached)
	record1, ok1 := result1.(types.Record)
	record2, ok2 := result2.(types.Record)

	if !ok1 || !ok2 {
		t.Fatal("expected both results to be Record")
	}

	if len(record1.Fields) != len(record2.Fields) {
		t.Error("cached results differ")
	}
}

func TestTypeResolverErrors(t *testing.T) {
	t.Run("type index out of bounds", func(t *testing.T) {
		c := &Component{
			Types: []TypeDef{},
		}
		resolver := NewTypeResolver(c)

		ref := ValTypeRef{IsPrimitive: false, TypeIdx: 5}
		_, err := resolver.ResolveValType(ref)
		if err == nil {
			t.Fatal("expected error for out of bounds type index")
		}
	})

	t.Run("unknown primitive opcode", func(t *testing.T) {
		c := &Component{}
		resolver := NewTypeResolver(c)

		ref := ValTypeRef{IsPrimitive: true, Primitive: 0x50}
		_, err := resolver.ResolveValType(ref)
		if err == nil {
			t.Fatal("expected error for unknown primitive opcode")
		}
	})

	t.Run("function type as value type", func(t *testing.T) {
		c := &Component{
			Types: []TypeDef{
				{
					Kind: TypeDefKindFunc,
					Func: &FuncType{},
				},
			},
		}
		resolver := NewTypeResolver(c)

		ref := ValTypeRef{IsPrimitive: false, TypeIdx: 0}
		_, err := resolver.ResolveValType(ref)
		if err == nil {
			t.Fatal("expected error for function type as value type")
		}
	})

	t.Run("resource type without handle", func(t *testing.T) {
		c := &Component{
			Types: []TypeDef{
				{
					Kind: TypeDefKindResource,
				},
			},
		}
		resolver := NewTypeResolver(c)

		ref := ValTypeRef{IsPrimitive: false, TypeIdx: 0}
		_, err := resolver.ResolveValType(ref)
		if err == nil {
			t.Fatal("expected error for resource type without handle")
		}
	})
}

func TestTypeResolver_TypeIdxToStoredIdx(t *testing.T) {
	// Type index 5 maps to stored index 2 via TypeIdxToStoredIdx.
	// This simulates aliases consuming indices between type section entries.
	c := &Component{
		Types: []TypeDef{
			// stored index 0
			{
				Kind: TypeDefKindDefined,
				Record: &RecordTypeDef{
					Fields: []RecordField{
						{Name: "x", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}},
					},
				},
			},
			// stored index 1
			{
				Kind: TypeDefKindDefined,
				Record: &RecordTypeDef{
					Fields: []RecordField{
						{Name: "only", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x79}},
					},
				},
			},
			// stored index 2: the record we want to reach via type index 5
			{
				Kind: TypeDefKindDefined,
				Record: &RecordTypeDef{
					Fields: []RecordField{
						{Name: "a", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}},
						{Name: "b", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x73}},
					},
				},
			},
		},
		TypeIdxToStoredIdx: map[uint32]uint32{
			0: 0,
			1: 1,
			5: 2, // type index 5 -> stored index 2
		},
	}
	resolver := NewTypeResolver(c)

	ref := ValTypeRef{IsPrimitive: false, TypeIdx: 5}
	result, err := resolver.ResolveValType(ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	record, ok := result.(types.Record)
	if !ok {
		t.Fatalf("expected Record, got %T", result)
	}

	if len(record.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(record.Fields))
	}

	if record.Fields[0].Name != "a" {
		t.Errorf("expected first field name 'a', got %q", record.Fields[0].Name)
	}
	if record.Fields[1].Name != "b" {
		t.Errorf("expected second field name 'b', got %q", record.Fields[1].Name)
	}
}

func TestTypeResolver_TypeIdxToStoredIdx_OutOfRange(t *testing.T) {
	// Type index 10 has no mapping and is out of range, should fail with an error.
	c := &Component{
		Types: []TypeDef{
			{
				Kind: TypeDefKindDefined,
				Record: &RecordTypeDef{
					Fields: []RecordField{
						{Name: "x", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}},
					},
				},
			},
		},
		TypeIdxToStoredIdx: map[uint32]uint32{
			0: 0,
		},
	}
	resolver := NewTypeResolver(c)

	ref := ValTypeRef{IsPrimitive: false, TypeIdx: 10}
	_, err := resolver.ResolveValType(ref)
	if err == nil {
		t.Fatal("expected error for unmapped type index 10, got nil")
	}
}

func TestTypeResolver_WithInstance_TypeSpace(t *testing.T) {
	// Type index 44 is NOT in TypeIdxToStoredIdx but the instance's typeSpace
	// has it populated (via buildTypeSpace). Uses NewTypeResolverWithInstance.
	c := &Component{
		Types:              []TypeDef{},
		TypeIdxToStoredIdx: map[uint32]uint32{},
	}

	inst := &Instance{}
	// Fill typeSpace slots 0-43 with nil, put actual type at slot 44.
	for i := 0; i < 44; i++ {
		inst.typeSpace = append(inst.typeSpace, nil)
	}
	inst.typeSpace = append(inst.typeSpace, &TypeDef{
		Kind: TypeDefKindDefined,
		Record: &RecordTypeDef{
			Fields: []RecordField{
				{Name: "from_instance", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x79}},
			},
		},
	})

	resolver := NewTypeResolverWithInstance(c, inst)

	ref := ValTypeRef{IsPrimitive: false, TypeIdx: 44}
	result, err := resolver.ResolveValType(ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	record, ok := result.(types.Record)
	if !ok {
		t.Fatalf("expected Record, got %T", result)
	}

	if len(record.Fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(record.Fields))
	}
	if record.Fields[0].Name != "from_instance" {
		t.Errorf("expected field name 'from_instance', got %q", record.Fields[0].Name)
	}
}

func TestTypeResolver_DirectFallback_NoMapping(t *testing.T) {
	// When TypeIdxToStoredIdx is nil (backward compat), direct indexing works.
	c := &Component{
		Types: []TypeDef{
			{
				Kind: TypeDefKindDefined,
				Record: &RecordTypeDef{
					Fields: []RecordField{
						{Name: "direct", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}},
					},
				},
			},
		},
		// TypeIdxToStoredIdx intentionally left nil
	}
	resolver := NewTypeResolver(c)

	ref := ValTypeRef{IsPrimitive: false, TypeIdx: 0}
	result, err := resolver.ResolveValType(ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	record, ok := result.(types.Record)
	if !ok {
		t.Fatalf("expected Record, got %T", result)
	}

	if len(record.Fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(record.Fields))
	}
	if record.Fields[0].Name != "direct" {
		t.Errorf("expected field name 'direct', got %q", record.Fields[0].Name)
	}
}

func TestTypeResolverAllPrimitives(t *testing.T) {
	c := &Component{}
	resolver := NewTypeResolver(c)

	// Test all primitive types including s8, u8, s16, u16
	tests := []struct {
		name     string
		ref      ValTypeRef
		expected types.ValType
	}{
		{"bool", ValTypeRef{IsPrimitive: true, Primitive: 0x7f}, types.Bool{}},
		{"s8", ValTypeRef{IsPrimitive: true, Primitive: 0x7e}, types.S8{}},
		{"u8", ValTypeRef{IsPrimitive: true, Primitive: 0x7d}, types.U8{}},
		{"s16", ValTypeRef{IsPrimitive: true, Primitive: 0x7c}, types.S16{}},
		{"u16", ValTypeRef{IsPrimitive: true, Primitive: 0x7b}, types.U16{}},
		{"s32", ValTypeRef{IsPrimitive: true, Primitive: 0x7a}, types.S32{}},
		{"u32", ValTypeRef{IsPrimitive: true, Primitive: 0x79}, types.U32{}},
		{"s64", ValTypeRef{IsPrimitive: true, Primitive: 0x78}, types.S64{}},
		{"u64", ValTypeRef{IsPrimitive: true, Primitive: 0x77}, types.U64{}},
		{"f32", ValTypeRef{IsPrimitive: true, Primitive: 0x76}, types.F32{}},
		{"f64", ValTypeRef{IsPrimitive: true, Primitive: 0x75}, types.F64{}},
		{"char", ValTypeRef{IsPrimitive: true, Primitive: 0x74}, types.Char{}},
		{"string", ValTypeRef{IsPrimitive: true, Primitive: 0x73}, types.String{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolver.ResolveValType(tt.ref)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %T, got %T", tt.expected, result)
			}
		})
	}
}
