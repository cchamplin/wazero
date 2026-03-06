# Component Model Test Expansion Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Close test coverage gaps that allowed three bugs to ship: type alias resolution during nested component instantiation, TypeResolver not using TypeIdxToStoredIdx, and record retptr lifting when flat count exceeds MAX_FLAT_RESULTS.

**Architecture:** Three layers of new tests: (1) unit tests targeting specific code paths in type_resolver.go, nested_component.go, and instance.go; (2) WAT-based integration tests using `testutil.BuildComponentFromWAT` that exercise binary-level patterns without external toolchains; (3) Go plugin integration tests for end-to-end patterns with WASI context.

**Tech Stack:** Go testing, `wasm-tools` (v1.245.1 for WAT compilation), `tinygo` (v0.40.1 for Go WASM plugins), `wit-bindgen-go` for WIT bindings.

**Previous bugs for reference:**
- `ba58a6fc` — "func N not found in parent": buildComponentFuncs ordering, instanceSpace alignment, shim export wiring
- Current session — "type N not found in parent": TypeIdxToStoredIdx mapping, buildTypeSpace, export alias resolution, retptr record lifting

---

## Task 1: Unit tests for TypeResolver with TypeIdxToStoredIdx

**Files:**
- Modify: `internal/component/type_resolver_test.go`

These tests verify that `resolveTypeIdx` correctly uses `TypeIdxToStoredIdx` instead of indexing into `Types` directly. This is the exact code path that was broken.

**Step 1: Write failing tests**

Add these test functions to `type_resolver_test.go`:

```go
func TestTypeResolver_TypeIdxToStoredIdx(t *testing.T) {
	// Type index 5 maps to stored index 2 in the Types array.
	// This simulates the common case where type aliases consume
	// indices between type section entries.
	c := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: &FuncType{}},          // stored 0
			{Kind: TypeDefKindDefined, Record: &RecordTypeDef{   // stored 1
				Fields: []RecordField{{Name: "x", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x79}}},
			}},
			{Kind: TypeDefKindDefined, Record: &RecordTypeDef{   // stored 2
				Fields: []RecordField{
					{Name: "a", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x79}},
					{Name: "b", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7f}},
				},
			}},
		},
		TypeIdxToStoredIdx: map[uint32]uint32{
			0: 0, // type idx 0 -> stored 0
			1: 1, // type idx 1 -> stored 1 (no gap)
			5: 2, // type idx 5 -> stored 2 (indices 2-4 consumed by aliases)
		},
	}

	resolver := NewTypeResolver(c)

	// Type index 5 should resolve to stored index 2 (the record with a, b)
	resolved, err := resolver.ResolveValType(ValTypeRef{TypeIdx: 5})
	if err != nil {
		t.Fatalf("ResolveValType(typeIdx=5): %v", err)
	}
	rec, ok := resolved.(types.Record)
	if !ok {
		t.Fatalf("expected Record, got %T", resolved)
	}
	if len(rec.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(rec.Fields))
	}
	if rec.Fields[0].Name != "a" || rec.Fields[1].Name != "b" {
		t.Errorf("field names = %q, %q; want a, b", rec.Fields[0].Name, rec.Fields[1].Name)
	}
}

func TestTypeResolver_TypeIdxToStoredIdx_OutOfRange(t *testing.T) {
	// Type index 10 has no mapping and is beyond Types length.
	// Should fail gracefully.
	c := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindDefined, Record: &RecordTypeDef{}},
		},
		TypeIdxToStoredIdx: map[uint32]uint32{0: 0},
	}

	resolver := NewTypeResolver(c)
	_, err := resolver.ResolveValType(ValTypeRef{TypeIdx: 10})
	if err == nil {
		t.Fatal("expected error for unmapped type index 10")
	}
}

func TestTypeResolver_WithInstance_TypeSpace(t *testing.T) {
	// Type index 44 is NOT in TypeIdxToStoredIdx (it's from an alias).
	// But the instance's typeSpace has it populated via buildTypeSpace.
	c := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindDefined, Record: &RecordTypeDef{
				Fields: []RecordField{
					{Name: "value", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x79}},
					{Name: "ok", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7f}},
				},
			}},
		},
		TypeIdxToStoredIdx: map[uint32]uint32{0: 0},
	}

	inst := &Instance{}
	// Simulate buildTypeSpace having resolved alias type 44 to the record
	for inst.AddTypeToSpace(nil) < 44 {
		// fill slots 0-43 with nil
	}
	// Overwrite slot 44 with the actual type
	inst.typeSpace[44] = &c.Types[0]

	resolver := NewTypeResolverWithInstance(c, inst)
	resolved, err := resolver.ResolveValType(ValTypeRef{TypeIdx: 44})
	if err != nil {
		t.Fatalf("ResolveValType(typeIdx=44): %v", err)
	}
	rec, ok := resolved.(types.Record)
	if !ok {
		t.Fatalf("expected Record, got %T", resolved)
	}
	if len(rec.Fields) != 2 || rec.Fields[0].Name != "value" {
		t.Errorf("unexpected record fields: %+v", rec.Fields)
	}
}

func TestTypeResolver_DirectFallback_NoMapping(t *testing.T) {
	// When TypeIdxToStoredIdx is nil (backward compat), direct indexing works.
	c := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindDefined, Record: &RecordTypeDef{
				Fields: []RecordField{{Name: "x", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x79}}},
			}},
		},
	}

	resolver := NewTypeResolver(c)
	resolved, err := resolver.ResolveValType(ValTypeRef{TypeIdx: 0})
	if err != nil {
		t.Fatalf("ResolveValType(typeIdx=0): %v", err)
	}
	if _, ok := resolved.(types.Record); !ok {
		t.Fatalf("expected Record, got %T", resolved)
	}
}
```

**Step 2: Run tests to verify they pass**

Run: `go test ./internal/component/ -run 'TestTypeResolver_TypeIdxToStoredIdx|TestTypeResolver_WithInstance|TestTypeResolver_DirectFallback' -v`
Expected: PASS (these test the fixed code paths)

**Step 3: Commit**

```bash
git add internal/component/type_resolver_test.go
git commit -m "test(component): add TypeResolver tests for TypeIdxToStoredIdx and instance typeSpace"
```

---

## Task 2: Unit tests for buildTypeSpace and resolveTypeAlias

**Files:**
- Modify: `internal/component/nested_component_test.go`

These tests verify that `buildTypeSpace` correctly populates the instance's typeSpace from TypeIdxToStoredIdx and type aliases, and that `resolveTypeAlias` traces export aliases through instance type declarations.

**Step 1: Write tests**

Add to `nested_component_test.go`:

```go
func TestBuildTypeSpace_FromTypeIdxToStoredIdx(t *testing.T) {
	// Component with 3 type section entries but sparse type index space
	// (aliases consumed indices 1, 3, 4)
	c := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: &FuncType{}},
			{Kind: TypeDefKindDefined, Record: &RecordTypeDef{
				Fields: []RecordField{{Name: "x", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x79}}},
			}},
			{Kind: TypeDefKindDefined, Record: &RecordTypeDef{
				Fields: []RecordField{{Name: "y", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x79}}},
			}},
		},
		TypeIdxToStoredIdx: map[uint32]uint32{
			0: 0,
			2: 1,
			5: 2,
		},
		NextTypeIdx: 6,
	}

	inst := &Instance{}
	l := &ComponentLinker{}
	l.buildTypeSpace(inst, c)

	// Verify type 0 was populated
	if td := inst.GetTypeFromSpace(0); td == nil {
		t.Error("type 0 should be populated")
	} else if td.Kind != TypeDefKindFunc {
		t.Errorf("type 0 kind = %v, want Func", td.Kind)
	}

	// Verify type 2 maps to stored index 1 (record with field "x")
	if td := inst.GetTypeFromSpace(2); td == nil {
		t.Error("type 2 should be populated")
	} else if td.Record == nil || td.Record.Fields[0].Name != "x" {
		t.Error("type 2 should be the record with field x")
	}

	// Verify type 5 maps to stored index 2 (record with field "y")
	if td := inst.GetTypeFromSpace(5); td == nil {
		t.Error("type 5 should be populated")
	} else if td.Record == nil || td.Record.Fields[0].Name != "y" {
		t.Error("type 5 should be the record with field y")
	}

	// Indices 1, 3, 4 should be nil (consumed by aliases, not populated here)
	if td := inst.GetTypeFromSpace(1); td != nil {
		t.Error("type 1 should be nil (alias slot)")
	}
}

func TestBuildTypeSpace_ExportAliases(t *testing.T) {
	// Component with an export alias that references a type from an imported instance.
	// The import's instance type defines a record type exported as "my-record".
	recordType := &TypeDef{
		Kind: TypeDefKindDefined,
		Record: &RecordTypeDef{
			Fields: []RecordField{
				{Name: "val", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x79}},
			},
		},
	}
	c := &Component{
		Types: []TypeDef{
			// Type 0: instance type for the import
			{Kind: TypeDefKindInstance, Instance: &InstanceTypeDef{
				Declarations: []InstanceDecl{
					{Kind: InstanceDeclKindType, Type: recordType},
					{Kind: InstanceDeclKindExport, Export: &InstanceExport{
						Name: "my-record",
						Kind: ExportKindType,
						Idx:  0, // local type index
					}},
				},
			}},
		},
		TypeIdxToStoredIdx: map[uint32]uint32{0: 0},
		Imports: []Import{
			{
				Name: "my-instance",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescInstance,
					TypeIdx: 0,
				},
			},
		},
		Aliases: []Alias{
			{
				Kind:        AliasKindExport,
				Sort:        SortType,
				Idx:         1, // type index 1 = alias
				InstanceIdx: 0,
				ExportName:  "my-record",
			},
		},
		NextTypeIdx: 2,
	}

	inst := &Instance{}
	l := &ComponentLinker{}
	l.buildTypeSpace(inst, c)

	// Type 1 should be resolved from the export alias
	td := inst.GetTypeFromSpace(1)
	if td == nil {
		t.Fatal("type 1 (export alias) should be resolved")
	}
	if td.Record == nil || len(td.Record.Fields) != 1 || td.Record.Fields[0].Name != "val" {
		t.Errorf("type 1 should be the record with field 'val', got %+v", td)
	}
}

func TestResolveFromParentScope_TypeWithStoredIdxMapping(t *testing.T) {
	// Verify resolveFromParentScope uses TypeIdxToStoredIdx when looking up types
	recordType := TypeDef{
		Kind: TypeDefKindDefined,
		Record: &RecordTypeDef{
			Fields: []RecordField{
				{Name: "data", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x79}},
			},
		},
	}
	parentComponent := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: &FuncType{}}, // stored 0
			recordType,                                  // stored 1
		},
		TypeIdxToStoredIdx: map[uint32]uint32{
			0:  0,
			10: 1, // type index 10 -> stored 1 (gap from aliases)
		},
	}
	parent := &Instance{}

	l := &ComponentLinker{}
	arg := ComponentInstantiateArg{
		Name: "my-type",
		Sort: SortType,
		Idx:  10,
	}

	def, err := l.resolveFromParentScope(parent, parentComponent, arg)
	if err != nil {
		t.Fatalf("resolveFromParentScope: %v", err)
	}
	typeDef, ok := def.(*TypeDefDef)
	if !ok {
		t.Fatalf("expected TypeDefDef, got %T", def)
	}
	if typeDef.TypeDef.Record == nil || typeDef.TypeDef.Record.Fields[0].Name != "data" {
		t.Errorf("expected record with field 'data', got %+v", typeDef.TypeDef)
	}
}

func TestResolveFromParentScope_TypeFromExportAlias(t *testing.T) {
	// Type index 5 comes from an export alias, not a type section.
	// resolveFromParentScope should find it via resolveTypeAlias.
	recordType := &TypeDef{
		Kind: TypeDefKindDefined,
		Record: &RecordTypeDef{
			Fields: []RecordField{
				{Name: "status", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7f}},
			},
		},
	}
	parentComponent := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindInstance, Instance: &InstanceTypeDef{
				Declarations: []InstanceDecl{
					{Kind: InstanceDeclKindType, Type: recordType},
					{Kind: InstanceDeclKindExport, Export: &InstanceExport{
						Name: "status-record",
						Kind: ExportKindType,
						Idx:  0,
					}},
				},
			}},
		},
		TypeIdxToStoredIdx: map[uint32]uint32{0: 0},
		Imports: []Import{
			{
				Name: "types-iface",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescInstance,
					TypeIdx: 0,
				},
			},
		},
		Aliases: []Alias{
			{
				Kind:        AliasKindExport,
				Sort:        SortType,
				Idx:         5,
				InstanceIdx: 0,
				ExportName:  "status-record",
			},
		},
	}
	parent := &Instance{}

	l := &ComponentLinker{}
	arg := ComponentInstantiateArg{
		Name: "import-type-status",
		Sort: SortType,
		Idx:  5,
	}

	def, err := l.resolveFromParentScope(parent, parentComponent, arg)
	if err != nil {
		t.Fatalf("resolveFromParentScope: %v", err)
	}
	typeDef, ok := def.(*TypeDefDef)
	if !ok {
		t.Fatalf("expected TypeDefDef, got %T", def)
	}
	if typeDef.TypeDef.Record == nil {
		t.Fatal("expected record type from export alias")
	}
	if typeDef.TypeDef.Record.Fields[0].Name != "status" {
		t.Errorf("field name = %q, want 'status'", typeDef.TypeDef.Record.Fields[0].Name)
	}
}
```

**Step 2: Run tests**

Run: `go test ./internal/component/ -run 'TestBuildTypeSpace|TestResolveFromParentScope_Type' -v`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/component/nested_component_test.go
git commit -m "test(component): add buildTypeSpace and resolveFromParentScope type alias tests"
```

---

## Task 3: Unit tests for record retptr lifting and liftFieldFromMemory

**Files:**
- Modify: `internal/component/instance_test.go`

These tests verify that `liftResolvedType` correctly handles the retptr case for records (flat count > MAX_FLAT_RESULTS=1) and that `liftFieldFromMemory` reads all primitive types correctly from linear memory.

**Step 1: Find the mock memory pattern used in existing tests**

Check: `grep -n 'mockMemory\|api.Memory\|ReadUint32Le' internal/component/instance_test.go | head -20`

The existing tests should have a mock memory. If not, we need a minimal one. Use the same pattern as the existing `liftStringFromRetptr` tests.

**Step 2: Write tests**

Add to `instance_test.go`. The tests need a mock api.Memory. Check what's already available in the test file. If there's a `mockMemory` or similar, use it. Otherwise, add a minimal one:

```go
// mockMemoryForLift implements api.Memory for testing record retptr lifting.
// Only the read methods needed for liftFieldFromMemory are implemented.
type mockMemoryForLift struct {
	api.Memory // embed to satisfy interface; panics on unimplemented methods
	data       []byte
}

func newMockMemory(size int) *mockMemoryForLift {
	return &mockMemoryForLift{data: make([]byte, size)}
}

func (m *mockMemoryForLift) ReadByteAt(offset uint32) (byte, bool) {
	if int(offset) >= len(m.data) {
		return 0, false
	}
	return m.data[offset], true
}

func (m *mockMemoryForLift) ReadUint16Le(offset uint32) (uint16, bool) {
	if int(offset)+2 > len(m.data) {
		return 0, false
	}
	return binary.LittleEndian.Uint16(m.data[offset:]), true
}

func (m *mockMemoryForLift) ReadUint32Le(offset uint32) (uint32, bool) {
	if int(offset)+4 > len(m.data) {
		return 0, false
	}
	return binary.LittleEndian.Uint32(m.data[offset:]), true
}

func (m *mockMemoryForLift) ReadUint64Le(offset uint32) (uint64, bool) {
	if int(offset)+8 > len(m.data) {
		return 0, false
	}
	return binary.LittleEndian.Uint64(m.data[offset:]), true
}

func (m *mockMemoryForLift) WriteUint32Le(offset, val uint32) bool {
	if int(offset)+4 > len(m.data) {
		return false
	}
	binary.LittleEndian.PutUint32(m.data[offset:], val)
	return true
}

func (m *mockMemoryForLift) WriteByte(offset uint32, val byte) bool {
	if int(offset) >= len(m.data) {
		return false
	}
	m.data[offset] = val
	return true
}
```

Then the actual tests:

```go
func TestLiftResolvedType_RecordRetptr(t *testing.T) {
	// Record {value: u32, ok: bool} has flat count 2 > MAX_FLAT_RESULTS=1.
	// Core function returns a single retptr to the record in linear memory.
	mem := newMockMemory(1024)

	// Write record at offset 100: value=42 (u32, 4 bytes), ok=1 (bool, 1 byte)
	mem.WriteUint32Le(100, 42)
	mem.WriteByte(104, 1)

	f := &ExportedFunc{
		memory: mem,
	}

	recordType := types.Record{
		Fields: []types.Field{
			{Name: "value", Type: types.U32{}},
			{Name: "ok", Type: types.Bool{}},
		},
	}

	callCtx := NewCallContext()
	result, err := f.liftResolvedType(recordType, []uint64{100}, nil, callCtx)
	if err != nil {
		t.Fatalf("liftResolvedType: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	rec := result[0].Record()
	if rec == nil {
		t.Fatalf("expected record, got kind=%v", result[0].Kind())
	}
	if got := rec["value"].U32(); got != 42 {
		t.Errorf("value = %d, want 42", got)
	}
	if got := rec["ok"].Bool(); !got {
		t.Errorf("ok = %v, want true", got)
	}
}

func TestLiftResolvedType_RecordFlat(t *testing.T) {
	// When flat results are available (e.g. single-field record), use flat path.
	f := &ExportedFunc{}

	recordType := types.Record{
		Fields: []types.Field{
			{Name: "x", Type: types.S32{}},
		},
	}

	callCtx := NewCallContext()
	result, err := f.liftResolvedType(recordType, []uint64{99}, nil, callCtx)
	if err != nil {
		t.Fatalf("liftResolvedType: %v", err)
	}
	rec := result[0].Record()
	if got := rec["x"].S32(); got != 99 {
		t.Errorf("x = %d, want 99", got)
	}
}

func TestLiftResolvedType_LargeRecordRetptr(t *testing.T) {
	// Record with 4 u32 fields. Flat count = 4 >> MAX_FLAT_RESULTS.
	mem := newMockMemory(1024)
	mem.WriteUint32Le(200, 10)
	mem.WriteUint32Le(204, 20)
	mem.WriteUint32Le(208, 30)
	mem.WriteUint32Le(212, 40)

	f := &ExportedFunc{memory: mem}

	recordType := types.Record{
		Fields: []types.Field{
			{Name: "a", Type: types.U32{}},
			{Name: "b", Type: types.U32{}},
			{Name: "c", Type: types.U32{}},
			{Name: "d", Type: types.U32{}},
		},
	}

	callCtx := NewCallContext()
	result, err := f.liftResolvedType(recordType, []uint64{200}, nil, callCtx)
	if err != nil {
		t.Fatalf("liftResolvedType: %v", err)
	}
	rec := result[0].Record()
	if rec["a"].U32() != 10 || rec["b"].U32() != 20 || rec["c"].U32() != 30 || rec["d"].U32() != 40 {
		t.Errorf("record = %v, want {10, 20, 30, 40}", rec)
	}
}

func TestLiftFieldFromMemory_AllPrimitiveTypes(t *testing.T) {
	mem := newMockMemory(1024)
	f := &ExportedFunc{memory: mem}

	tests := []struct {
		name     string
		valType  types.ValType
		setup    func(offset uint32)
		check    func(val Val) bool
		size     uint32
	}{
		{"bool_true", types.Bool{}, func(o uint32) { mem.WriteByte(o, 1) },
			func(v Val) bool { return v.Bool() == true }, 1},
		{"bool_false", types.Bool{}, func(o uint32) { mem.WriteByte(o, 0) },
			func(v Val) bool { return v.Bool() == false }, 1},
		{"u8", types.U8{}, func(o uint32) { mem.WriteByte(o, 255) },
			func(v Val) bool { return v.U8() == 255 }, 1},
		{"s8", types.S8{}, func(o uint32) { mem.WriteByte(o, 0x80) },
			func(v Val) bool { return v.S8() == -128 }, 1},
		{"u32", types.U32{}, func(o uint32) { mem.WriteUint32Le(o, 12345) },
			func(v Val) bool { return v.U32() == 12345 }, 4},
		{"s32", types.S32{}, func(o uint32) { mem.WriteUint32Le(o, uint32(int32(-42))) },
			func(v Val) bool { return v.S32() == -42 }, 4},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			offset := uint32(500)
			tc.setup(offset)
			val, size := f.liftFieldFromMemory(offset, tc.valType)
			if size != tc.size {
				t.Errorf("size = %d, want %d", size, tc.size)
			}
			if !tc.check(val) {
				t.Errorf("value check failed for %s: got %v", tc.name, val)
			}
		})
	}
}
```

**Step 3: Run tests**

Run: `go test ./internal/component/ -run 'TestLiftResolvedType_Record|TestLiftFieldFromMemory' -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/component/instance_test.go
git commit -m "test(component): add record retptr lifting and liftFieldFromMemory tests"
```

---

## Task 4: Unit tests for instance space alignment and shim export wiring

**Files:**
- Modify: `internal/component/nested_component_test.go`

These tests cover the three sub-bugs from the `ba58a6fc` fix: imported instances occupying instanceSpace slots, buildComponentFuncs ordering, and wireNestedComponentExports.

**Step 1: Write tests**

```go
func TestResolveFromParentScope_InstanceSpaceAlignment(t *testing.T) {
	// When a component has N instance imports, they occupy slots 0..N-1
	// in the instance space. Nested component instances start at slot N.
	// Verify that instance args reference the correct slots.
	importedInst := &Instance{
		exports: map[string]*ExportedFunc{
			"helper": {name: "helper"},
		},
	}

	parent := &Instance{}
	// Simulate 3 imported instances occupying slots 0, 1, 2
	parent.AddInstanceToSpace(nil) // import 0
	parent.AddInstanceToSpace(nil) // import 1
	parent.AddInstanceToSpace(nil) // import 2
	// Slot 3 is the first nested component instance
	parent.AddInstanceToSpace(importedInst)

	parentComponent := &Component{}
	l := &ComponentLinker{}

	// Asking for instance at index 3 should find importedInst
	arg := ComponentInstantiateArg{
		Name: "my-instance",
		Sort: SortInstance,
		Idx:  3,
	}
	def, err := l.resolveFromParentScope(parent, parentComponent, arg)
	if err != nil {
		t.Fatalf("resolveFromParentScope(instance 3): %v", err)
	}
	if def == nil {
		t.Fatal("expected non-nil definition")
	}

	// Index 0 (imported, nil) should fail
	arg.Idx = 0
	_, err = l.resolveFromParentScope(parent, parentComponent, arg)
	if err == nil {
		t.Error("expected error for nil instance at index 0")
	}
}

func TestResolveFromParentScope_ComponentFuncsOrdering(t *testing.T) {
	// buildComponentFuncs must run before nested component instantiation.
	// This test verifies that function args are resolvable.
	parent := &Instance{
		componentFuncs: map[uint32]ComponentFunc{
			0: {Impl: func(ctx context.Context, args []Val) ([]Val, error) {
				return []Val{ValS32(1)}, nil
			}},
			5: {Impl: func(ctx context.Context, args []Val) ([]Val, error) {
				return []Val{ValS32(5)}, nil
			}},
		},
	}

	parentComponent := &Component{}
	l := &ComponentLinker{}

	// Function at index 5 should be resolvable
	arg := ComponentInstantiateArg{Name: "fn5", Sort: SortFunc, Idx: 5}
	def, err := l.resolveFromParentScope(parent, parentComponent, arg)
	if err != nil {
		t.Fatalf("resolveFromParentScope(func 5): %v", err)
	}
	if def == nil {
		t.Fatal("expected non-nil definition for func 5")
	}

	// Function at index 99 should fail
	arg.Idx = 99
	_, err = l.resolveFromParentScope(parent, parentComponent, arg)
	if err == nil {
		t.Error("expected error for missing func 99")
	}
}
```

**Step 2: Run tests**

Run: `go test ./internal/component/ -run 'TestResolveFromParentScope_InstanceSpace|TestResolveFromParentScope_ComponentFuncs' -v`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/component/nested_component_test.go
git commit -m "test(component): add instance space alignment and componentFuncs ordering tests"
```

---

## Task 5: Integration test — Go plugin with variant/enum shared types

**Files:**
- Create: `internal/component/wasip2test/go-variant-plugin/` (wit, main.go, build.sh)
- Create: `internal/component/wasip2test/variant_types_test.go`

This tests type aliases for variant and enum types across import/export interfaces — a pattern not currently tested.

**Step 1: Create the WIT definition**

Create `internal/component/wasip2test/go-variant-plugin/wit/variant.wit`:

```wit
package test:variant;

interface types {
    enum severity {
        info,
        warning,
        error,
    }

    variant log-entry {
        message(string),
        code(u32),
        level(severity),
    }

    record log-result {
        count: u32,
        last-severity: severity,
    }
}

interface host-logger {
    use types.{severity};
    get-default-severity: func() -> severity;
}

interface processor {
    use types.{log-entry, log-result, severity};
    process-entry: func(entry: log-entry) -> log-result;
}

world variant-plugin {
    import host-logger;
    export processor;
}
```

**Step 2: Create the Go implementation**

Follow the go-repro-plugin pattern:
1. Run `wit-bindgen-go` to generate bindings
2. Implement the export (process-entry) calling the import (get-default-severity)
3. Build with `GOARCH=wasm GOOS=wasip1 go build` + wasm-tools component embed/new

**Step 3: Create the test**

Create `internal/component/wasip2test/variant_types_test.go`:

```go
func TestVariantPlugin_EnumTypeResolution(t *testing.T) {
    // Tests that enum and variant types defined in a shared types interface
    // are correctly resolved during nested component instantiation and
    // can be lifted back from core module linear memory.
    // ... (follows repro_test.go pattern)
}
```

**Step 4: Build and verify**

Run: `cd internal/component/wasip2test/go-variant-plugin && bash build.sh`
Run: `go test ./internal/component/wasip2test/ -run TestVariantPlugin -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/wasip2test/go-variant-plugin/ internal/component/wasip2test/variant_types_test.go
git commit -m "test(component): add variant/enum shared type integration test"
```

---

## Task 6: Integration test — Go plugin with large record (retptr path)

**Files:**
- Create: `internal/component/wasip2test/go-large-record-plugin/` (wit, main.go, build.sh)
- Create: `internal/component/wasip2test/large_record_test.go`

This tests the retptr lifting path for records with flat count > MAX_FLAT_RESULTS=1 in a real end-to-end scenario.

**Step 1: Create the WIT definition**

Create `internal/component/wasip2test/go-large-record-plugin/wit/large-record.wit`:

```wit
package test:large-record;

interface types {
    record coordinates {
        x: s32,
        y: s32,
        z: s32,
    }

    record full-status {
        coords: coordinates,
        health: u32,
        alive: bool,
        score: u64,
    }
}

interface host-data {
    use types.{coordinates};
    get-position: func() -> coordinates;
}

interface status-handler {
    use types.{full-status, coordinates};
    get-status: func() -> full-status;
    get-position: func() -> coordinates;
}

world large-record-plugin {
    import host-data;
    export status-handler;
}
```

**Step 2: Create Go implementation**

The guest calls `host-data.get-position` and wraps it into `full-status`.
The `get-position` export returns 3 fields (flat count 3), forcing retptr.
The `get-status` export returns a nested record (flat count 6+), definitely retptr.

**Step 3: Create the test**

```go
func TestLargeRecordPlugin_RetptrLifting(t *testing.T) {
    // Tests that records with flat count > MAX_FLAT_RESULTS=1 are
    // correctly lifted via retptr from linear memory.
    // Verifies both a 3-field record and a nested record with 6+ fields.
}
```

**Step 4: Build and verify**

Run: `cd internal/component/wasip2test/go-large-record-plugin && bash build.sh`
Run: `go test ./internal/component/wasip2test/ -run TestLargeRecordPlugin -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/wasip2test/go-large-record-plugin/ internal/component/wasip2test/large_record_test.go
git commit -m "test(component): add large record retptr lifting integration test"
```

---

## Task 7: Integration test — Go plugin with option/result wrapping shared records

**Files:**
- Create: `internal/component/wasip2test/go-nested-types-plugin/` (wit, main.go, build.sh)
- Create: `internal/component/wasip2test/nested_types_test.go`

This tests complex type alias chains: shared record used inside option and result types across import/export interfaces.

**Step 1: Create the WIT definition**

```wit
package test:nested-types;

interface types {
    record item {
        id: u32,
        name: string,
    }
}

interface store {
    use types.{item};
    get-item: func(id: u32) -> option<item>;
}

interface handler {
    use types.{item};
    lookup: func(id: u32) -> option<item>;
    create: func(name: string) -> result<item, string>;
}

world nested-types-plugin {
    import store;
    export handler;
}
```

**Step 2: Implement, build, test**

Follow the same pattern as Tasks 5 and 6. The guest's `lookup` calls `store.get-item` and returns the result. The guest's `create` builds a new item.

This exercises:
- Type aliases for records used inside option<T> and result<T, E>
- String types in records (triggers memory allocation)
- The full TypeResolver chain for nested type references

**Step 3: Commit**

```bash
git add internal/component/wasip2test/go-nested-types-plugin/ internal/component/wasip2test/nested_types_test.go
git commit -m "test(component): add option/result with shared record types integration test"
```

---

## Task 8: WAT-based integration tests for nested component patterns

**Files:**
- Create: `internal/component/conformance/nested_instantiation_test.go`

These tests use `testutil.BuildComponentFromWAT` to create inline components exercising specific binary patterns without external toolchains. They test the actual `Instantiate` path (not just API-level structures).

**Step 1: Write WAT-based tests**

```go
package conformance

import (
    "context"
    "testing"

    "github.com/tetratelabs/wazero"
    "github.com/tetratelabs/wazero/internal/component"
    "github.com/tetratelabs/wazero/internal/component/testutil"
)

func TestNestedInstantiation_SimpleExport(t *testing.T) {
    // Component with core module that exports a function,
    // canon lift wrapping it, and a world-level export.
    // Verifies the basic instantiation-to-call path works.
    wat := `
    (component
        (core module $m
            (func (export "get-val") (result i32)
                i32.const 42
            )
        )
        (core instance $i (instantiate $m))
        (alias core export $i "get-val" (core func $f))
        (type $ft (func (result u32)))
        (func (export "get-val") (type $ft)
            (canon lift (core func $f)))
    )`

    wasmBytes := testutil.MustBuildComponentFromWAT(wat)
    ctx := context.Background()
    rt := wazero.NewRuntime(ctx)
    defer rt.Close(ctx)

    compiled, err := rt.CompileComponent(ctx, wasmBytes)
    if err != nil {
        t.Fatalf("CompileComponent: %v", err)
    }

    linker := component.NewComponentLinker(rt)
    inst, err := linker.Instantiate(ctx, compiled.(*component.CompiledComponent))
    if err != nil {
        t.Fatalf("Instantiate: %v", err)
    }

    fn := inst.ExportedFunction("get-val")
    if fn == nil {
        t.Fatal("get-val not found")
    }

    results, err := fn.Call(ctx)
    if err != nil {
        t.Fatalf("Call: %v", err)
    }
    if results[0].U32() != 42 {
        t.Errorf("get-val() = %d, want 42", results[0].U32())
    }
}

func TestNestedInstantiation_RecordReturn(t *testing.T) {
    // Component with a core function that returns a record via retptr.
    // The record has 2 fields (flat count 2 > MAX_FLAT_RESULTS=1).
    // This exercises the retptr lifting path through full instantiation.
    wat := `
    (component
        (core module $m
            (memory (export "memory") 1)
            (func $realloc (export "cabi_realloc")
                (param i32 i32 i32 i32) (result i32)
                ;; bump allocator
                i32.const 1024
            )
            (func (export "get-pair") (param i32) (result)
                ;; Write {x: 10, y: 20} to retptr (param 0)
                local.get 0
                i32.const 10
                i32.store
                local.get 0
                i32.const 4
                i32.add
                i32.const 20
                i32.store
            )
        )
        (core instance $i (instantiate $m))
        (alias core export $i "get-pair" (core func $f))
        (alias core export $i "memory" (core memory $mem))
        (alias core export $i "cabi_realloc" (core func $realloc))
        (type $pair (record (field "x" s32) (field "y" s32)))
        (type $ft (func (result $pair)))
        (func (export "get-pair") (type $ft)
            (canon lift (core func $f)
                (memory $mem)
                (realloc $realloc)))
    )`

    wasmBytes := testutil.MustBuildComponentFromWAT(wat)
    ctx := context.Background()
    rt := wazero.NewRuntime(ctx)
    defer rt.Close(ctx)

    compiled, err := rt.CompileComponent(ctx, wasmBytes)
    if err != nil {
        t.Fatalf("CompileComponent: %v", err)
    }

    linker := component.NewComponentLinker(rt)
    inst, err := linker.Instantiate(ctx, compiled.(*component.CompiledComponent))
    if err != nil {
        t.Fatalf("Instantiate: %v", err)
    }

    fn := inst.ExportedFunction("get-pair")
    if fn == nil {
        t.Fatal("get-pair not found")
    }

    results, err := fn.Call(ctx)
    if err != nil {
        t.Fatalf("Call: %v", err)
    }
    rec := results[0].Record()
    if rec["x"].S32() != 10 || rec["y"].S32() != 20 {
        t.Errorf("get-pair() = {x: %d, y: %d}, want {x: 10, y: 20}",
            rec["x"].S32(), rec["y"].S32())
    }
}
```

NOTE: The WAT syntax for component model records may need adjustment based on what `wasm-tools parse` accepts. Test the WAT compiles before finalizing. The implementer should check `wasm-tools parse` output and adjust syntax as needed.

**Step 2: Run tests**

Run: `go test ./internal/component/conformance/ -run 'TestNestedInstantiation' -v`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/component/conformance/nested_instantiation_test.go
git commit -m "test(component): add WAT-based nested instantiation integration tests"
```

---

## Task 9: Run full test suite and verify no regressions

**Step 1: Run all component tests**

Run: `go test ./internal/component/... -count=1 -timeout 300s`
Expected: All packages PASS

**Step 2: Run full project tests**

Run: `go test ./... -count=1 -timeout 600s`
Expected: All packages PASS, exit code 0

**Step 3: Final commit if any cleanup needed**

---

## Execution Notes

**Task dependency graph:**
- Tasks 1-4 are independent (unit tests, can be parallelized)
- Tasks 5-7 depend on toolchains (tinygo, wit-bindgen-go, wasm-tools) and are independent of each other
- Task 8 depends only on wasm-tools
- Task 9 depends on all others

**Key files to reference when implementing:**
- `internal/component/wasip2test/repro_test.go` — pattern for Go plugin integration tests
- `internal/component/wasip2test/go-repro-plugin/build.sh` — build script pattern
- `internal/component/testutil/builder.go` — BuildComponentFromWAT API
- `internal/component/nested_component_test.go` — mock pattern for unit tests
- `internal/component/type_resolver_test.go` — existing TypeResolver test patterns
- `internal/component/instance_test.go` — existing instance test patterns

**What to watch for:**
- WAT syntax for component model types (records, variants) is specific to wasm-tools version
- Go plugin builds require the WASI adapter at `testdata/wasi_snapshot_preview1.reactor.wasm`
- The `encoding/binary` import is needed for mock memory in instance_test.go
- Type index space math: TypeIdxToStoredIdx maps sparse indices to compact array positions
