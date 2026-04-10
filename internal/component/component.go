// internal/component/component.go

package component

import (
	"fmt"

	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/wasm"
)

// Component represents a parsed WebAssembly component.
// Unlike core wasm modules, components can contain nested modules
// and components, and use rich interface types.
type Component struct {
	// CoreModules contains embedded core wasm modules (section ID 1).
	// These are the raw modules that will be instantiated.
	CoreModules []*wasm.Module

	// CoreModuleData contains the raw bytes of each core module.
	// Used for instantiation via wazero's CompileModule API.
	CoreModuleData [][]byte

	// CoreTypes contains core type definitions (section ID 0).
	CoreTypes []CoreTypeDef

	// Types is the canonical bag of component-level type definitions
	// (section ID 7 plus aliased types). One ComponentTypes instance is
	// shared across the top-level component and all of its nested
	// components; individual TypeDef entries reference into it.
	Types *types.ComponentTypes

	// TypeDefs is one entry per slot in the component's type index space,
	// in declaration order. Densely aligned with Component.NextTypeIdx —
	// every slot that bumps NextTypeIdx has exactly one corresponding
	// entry here, including outer and export type aliases.
	//
	// Aliases are stored with Kind == TypeDefKindAlias and a populated
	// Alias *AliasTarget field carrying the unresolved target metadata.
	// Callers that need to resolve a typeidx to a concrete TypeDef must
	// call c.ResolveTypeDef(idx), which walks the alias chain — mirror
	// of wasmparser::Validator.component_any_type_at(typeidx).
	//
	// Spec: Binary.md:110-122 (type index space + alias grammar),
	// 263-268 (alias slot prose).
	// Wasmtime parallel: crates/environ/src/component/translate.rs:796-801
	// (validator.types(0).component_any_type_at(typeidx) at canon lift
	// sites).
	TypeDefs []TypeDef

	// Canonicals contains canonical function definitions (section ID 8).
	// These define lift/lower wrappers around core functions.
	Canonicals []CanonicalDef

	// FuncIdxToCanonical maps component function index to the index in Canonicals.
	// This is needed because canon lift operations can create component functions
	// at non-contiguous indices in the component function index space.
	FuncIdxToCanonical map[uint32]uint32

	// Exports contains component exports (section ID 11).
	// These expose functions and instances to the outside world.
	Exports []Export

	// Aliases contains alias definitions (section ID 6).
	Aliases []Alias

	// Imports contains component imports (section ID 10).
	Imports []Import

	// CoreInstances contains core instance definitions (section ID 2).
	CoreInstances []CoreInstance

	// ComponentInstances contains component instance definitions (section ID 5).
	ComponentInstances []ParsedComponentInstance

	// Components contains nested component definitions (section ID 4).
	Components []*Component

	// Start defines the component start function (section ID 9).
	Start *StartDef

	// Values contains component value definitions (section ID 12).
	Values []ValueDef

	// Component-level index spaces (5 total)

	// NextFuncIdx tracks the next component function index during parsing.
	// This is used internally by the decoder.
	NextFuncIdx uint32

	// NextValueIdx tracks the next component value index during parsing.
	// This is incremented by value definitions and import value operations.
	NextValueIdx uint32

	// NextTypeIdx tracks the next component type index during parsing.
	// This is incremented by type definitions and alias outer type operations.
	NextTypeIdx uint32

	// NextComponentInstanceIdx tracks the next component instance index during parsing.
	// This is incremented by component instance definitions and alias export instance operations.
	NextComponentInstanceIdx uint32

	// NextComponentIdx tracks the next nested component index during parsing.
	// This is incremented by component definitions and alias outer component operations.
	NextComponentIdx uint32

	// Core WebAssembly 1.0 index spaces (5 total)

	// NextCoreFuncIdx tracks the next core function index during parsing.
	// This is incremented by alias core export (func), canon lower, and
	// canon resource.drop/new/rep operations.
	NextCoreFuncIdx uint32

	// NextCoreTableIdx tracks the next core table index during parsing.
	// This is incremented by alias core export (table) operations.
	NextCoreTableIdx uint32

	// NextCoreMemoryIdx tracks the next core memory index during parsing.
	// This is incremented by alias core export (memory) operations.
	NextCoreMemoryIdx uint32

	// NextCoreGlobalIdx tracks the next core global index during parsing.
	// This is incremented by alias core export (global) operations.
	NextCoreGlobalIdx uint32

	// NextCoreTypeIdx tracks the next core type index during parsing.
	// This is incremented by core type definitions.
	NextCoreTypeIdx uint32

	// Core Extended index spaces (2 total)

	// NextModuleInstanceIdx tracks the next core module instance index during parsing.
	// This is incremented by core instance definitions.
	NextModuleInstanceIdx uint32

	// NextModuleIdx tracks the next core module index during parsing.
	// This is incremented by core module definitions and alias outer module operations.
	NextModuleIdx uint32

	// CompiledCoreModules holds pre-compiled core modules for this component.
	// For the root component these live in CompiledComponent.compiledModules;
	// for nested components they are populated by CompileComponent's recursive
	// compilation pass so that instantiateNestedComponent can instantiate
	// core modules without a full CompiledComponent wrapper.
	CompiledCoreModules []CompiledModuleCloser
}

// TypeDef is one entry per slot in the component's type index space,
// populated by the decoder. Every Kind-specific field is populated only
// when Kind matches; other fields are zero.
//
// An Alias slot (Kind == TypeDefKindAlias) is produced for every outer
// or export type alias in the binary; its Alias field carries the
// unresolved target. Callers that need the ultimate concrete TypeDef
// behind an alias chain must resolve via Component.ResolveTypeDef —
// direct indexing returns the alias entry itself.
//
// Session 1 Decision 5 option A: TypeDef.Func is a types.FuncTypeIdx
// (not a *types.TypeFunc) so it remains stable across canonical bag
// growth. Callers that need the *types.TypeFunc do:
//
//	&c.Types.Funcs[td.Func]
//
// after the bag is finalized, or use the FuncType helper below.
type TypeDef struct {
	Kind TypeDefKind

	// Func is the function-type index into Component.Types.Funcs when
	// Kind == TypeDefKindFunc.
	Func types.FuncTypeIdx

	// Resource is the resource-table index when Kind == TypeDefKindResource.
	// Refers into Component.Types.ResourceTables.
	Resource types.ResourceTableIdx

	// ValType is the value-type reference when Kind == TypeDefKindDefined.
	// Refers into Component.Types via the ValType.Index field.
	ValType types.ValType

	// Instance is the instance-type declaration when Kind == TypeDefKindInstance.
	Instance *InstanceTypeDef

	// Component is the component-type declaration when Kind == TypeDefKindComponent.
	Component *ComponentTypeDef

	// Alias carries the unresolved target of a type alias when
	// Kind == TypeDefKindAlias. Populated by binary/alias.go at decode
	// time; resolved at use time via Component.ResolveTypeDef.
	//
	// Spec: Binary.md:118-126 aliastarget grammar.
	Alias *AliasTarget

	// ResourceDtor, ResourceDtorAsync, ResourceDtorCallback carry the
	// destructor metadata the decoder extracts for TypeDefKindResource
	// slots. bindResourceTypes reads these at Instantiate time to
	// populate runtime.ResourceType fields. Spec: definitions.py:351-361.
	ResourceDtor         *uint32
	ResourceDtorAsync    bool
	ResourceDtorCallback *uint32
}

// AliasTarget records the unresolved target of a type alias. Exactly
// one of the two target-kinds is populated per instance:
//
//   - Outer*: this alias is an outer alias (Binary.md:121 aliastarget
//     0x02 ct:<u32> idx:<u32>). OuterCount is the de Bruijn count
//     (0 = same scope, 1 = enclosing, ...). OuterIndex is the type
//     index within that scope.
//
//   - Instance* / ExportName: this alias is an export alias
//     (Binary.md:119 aliastarget 0x00 i:<instanceidx> n:<name>).
//     InstanceIdx is the instance index; ExportName is the name of
//     the type export on that instance.
//
// Spec: Binary.md:118-122 aliastarget grammar; Explainer.md:326-338
// (alias binds new index in the current component's type index space).
type AliasTarget struct {
	// IsExport selects between the two variants: true = export-alias
	// (InstanceIdx + ExportName), false = outer-alias (OuterCount +
	// OuterIndex).
	IsExport bool

	// Outer alias fields (when IsExport == false).
	OuterCount uint32
	OuterIndex uint32

	// Export alias fields (when IsExport == true).
	InstanceIdx uint32
	ExportName  string
}

// FuncType resolves TypeDef.Func to its canonical *types.TypeFunc in the
// component's interned bag. Kind must be TypeDefKindFunc; callers are
// responsible for ensuring the canonical bag has been finalized before
// taking this pointer (Session 1 Decision 5 option A).
func (td *TypeDef) FuncType(c *Component) *types.TypeFunc {
	return &c.Types.Funcs[td.Func]
}

// ResolveTypeDef walks the alias chain starting at typeIdx in the
// component's type index space and returns the first non-alias
// TypeDef, plus its absolute index within c.TypeDefs. This mirrors
// wasmparser::Validator.component_any_type_at, which transparently
// follows alias chains at use sites.
//
// Outer aliases with OuterCount > 0 reference an enclosing component's
// scope; resolving these requires walking up a parent-component chain.
// In Session 1's local-only model, cross-scope resolution is deferred
// to the wiring layer (nested_component.go::resolveExportTypeAlias).
// ResolveTypeDef only walks outer aliases with OuterCount == 0 (same
// scope); any other alias kind returns an error naming the deferred
// scope.
//
// Export aliases (IsExport == true) are also deferred: resolving an
// export alias requires access to the instance type's exports, which
// is a wiring-layer concern. Callers that encounter an export alias
// at this level must use the explicit wiring path.
//
// Spec: Binary.md:118-126 aliastarget grammar, 263-268 alias prose.
func (c *Component) ResolveTypeDef(typeIdx uint32) (*TypeDef, uint32, error) {
	visited := make(map[uint32]bool)
	for {
		if int(typeIdx) >= len(c.TypeDefs) {
			return nil, 0, fmt.Errorf("type index %d out of range (len=%d)", typeIdx, len(c.TypeDefs))
		}
		if visited[typeIdx] {
			return nil, 0, fmt.Errorf("type alias cycle at index %d", typeIdx)
		}
		visited[typeIdx] = true
		td := &c.TypeDefs[typeIdx]
		if td.Kind != TypeDefKindAlias {
			return td, typeIdx, nil
		}
		if td.Alias == nil {
			return nil, 0, fmt.Errorf("alias at index %d has nil AliasTarget", typeIdx)
		}
		if td.Alias.IsExport {
			return nil, 0, fmt.Errorf("ResolveTypeDef cannot follow export alias at index %d (export alias resolution is deferred to the wiring layer; see nested_component.go::resolveExportTypeAlias)", typeIdx)
		}
		if td.Alias.OuterCount > 0 {
			return nil, 0, fmt.Errorf("ResolveTypeDef cannot follow cross-scope outer alias at index %d (OuterCount=%d; cross-scope resolution is deferred to the wiring layer)", typeIdx, td.Alias.OuterCount)
		}
		typeIdx = td.Alias.OuterIndex
	}
}

// TypeDefKind identifies the kind of type definition.
type TypeDefKind uint8

const (
	TypeDefKindFunc TypeDefKind = iota
	TypeDefKindComponent
	TypeDefKindInstance
	TypeDefKindResource
	TypeDefKindDefined
	// TypeDefKindAlias is a type-section slot produced by an outer or
	// export type alias (binary/alias.go). The alias's target metadata
	// lives on TypeDef.Alias; resolving to the ultimate concrete TypeDef
	// requires walking the alias chain via Component.ResolveTypeDef.
	//
	// Spec: Binary.md:118-122 (alias grammar) and Explainer.md:326-338
	// ("the `id` of the alias is bound to the new index added by the
	// alias") — aliases consume slots in the component's type index space.
	TypeDefKindAlias
)

// InstanceDeclKind identifies the kind of instance declaration.
type InstanceDeclKind uint8

const (
	InstanceDeclKindCoreType InstanceDeclKind = 0x00
	InstanceDeclKindType     InstanceDeclKind = 0x01
	InstanceDeclKindAlias    InstanceDeclKind = 0x02
	InstanceDeclKindExport   InstanceDeclKind = 0x04
)

// ComponentDeclKind identifies the kind of component declaration.
type ComponentDeclKind uint8

const (
	ComponentDeclKindCoreType ComponentDeclKind = 0x00
	ComponentDeclKindType     ComponentDeclKind = 0x01
	ComponentDeclKindAlias    ComponentDeclKind = 0x02
	ComponentDeclKindImport   ComponentDeclKind = 0x03
	ComponentDeclKindExport   ComponentDeclKind = 0x04
)

// ComponentDecl represents a declaration within a component type.
type ComponentDecl struct {
	Kind     ComponentDeclKind
	CoreType *CoreTypeDef
	Type     *TypeDef
	Alias    *Alias
	Import   *Import
	Export   *InstanceExport
}

// ComponentTypeDef represents a component type (0x41).
type ComponentTypeDef struct {
	Declarations []ComponentDecl
}

// InstanceDecl represents a declaration within an instance type.
type InstanceDecl struct {
	Kind     InstanceDeclKind
	CoreType *CoreTypeDef
	Type     *TypeDef
	Alias    *Alias
	Export   *InstanceExport
}

// InstanceExport represents an export declaration in an instance type.
type InstanceExport struct {
	Name    string
	Kind    ExportKind
	Idx     uint32
	TypeIdx *uint32 // Optional type annotation
}

// InstanceTypeDef represents an instance type (0x42).
type InstanceTypeDef struct {
	Declarations []InstanceDecl
}

// CanonicalDef represents a canonical function definition.
type CanonicalDef struct {
	Kind CanonKind

	// ComponentFuncIdx is the index assigned to this canonical in its target space.
	// For Lift: this is the index in the component function index space.
	// For Lower: this is the core function index that is created (NextCoreFuncIdx).
	// For Resource operations: this is the core function index (NextCoreFuncIdx).
	ComponentFuncIdx uint32

	// For Lift: core function index, options, and component function type
	CoreFuncIdx uint32
	TypeIdx     uint32

	// For Lower: component function index and options
	FuncIdx uint32

	// Options for both Lift and Lower
	Options CanonicalOptions
}

// CanonKind identifies the kind of canonical definition.
type CanonKind uint8

const (
	CanonKindLift CanonKind = iota
	CanonKindLower
	CanonKindResourceNew
	CanonKindResourceDrop
	CanonKindResourceRep
	CanonKindAsync // Async/threading builtins (all produce core func)
)

// CanonicalOptions holds optional parameters for canonical operations.
// Per Binary.md:346-354, canonopt codes are 0x00-0x07.
type CanonicalOptions struct {
	StringEncoding types.StringEncoding
	MemoryIdx      *uint32 // nil if not specified
	ReallocIdx     *uint32 // nil if not specified
	PostReturnIdx  *uint32 // nil if not specified
	Async          bool    // true if async option specified (gated)
	CallbackIdx    *uint32 // callback function index (gated)
}


// Export represents a component export.
type Export struct {
	Name       string
	Kind       ExportKind
	IsCoreSort bool     // True when the export uses a core sort (sort prefix 0x00)
	CoreSort   CoreSort // For core-sort exports, the specific core sort
	Idx        uint32   // Index into the appropriate index space
	TypeIdx    *uint32  // Optional type annotation
}

// ExportKind identifies what kind of item is being exported.
type ExportKind uint8

const (
	ExportKindFunc ExportKind = iota
	ExportKindValue
	ExportKindType
	ExportKindComponent
	ExportKindInstance
	ExportKindTable
	ExportKindMemory
	ExportKindGlobal
	ExportKindTag
	ExportKindModule
	ExportKindCoreInstance
)

// Sort identifies the kind of component-level item.
type Sort uint8

const (
	SortCoreSort  Sort = 0x00 // Followed by CoreSort
	SortFunc      Sort = 0x01
	SortValue     Sort = 0x02
	SortType      Sort = 0x03
	SortComponent Sort = 0x04
	SortInstance  Sort = 0x05
)

func (s Sort) String() string {
	switch s {
	case SortCoreSort:
		return "core"
	case SortFunc:
		return "func"
	case SortValue:
		return "value"
	case SortType:
		return "type"
	case SortComponent:
		return "component"
	case SortInstance:
		return "instance"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

// CoreSort identifies the kind of core wasm item.
type CoreSort uint8

const (
	CoreSortFunc     CoreSort = 0x00
	CoreSortTable    CoreSort = 0x01
	CoreSortMemory   CoreSort = 0x02
	CoreSortGlobal   CoreSort = 0x03
	CoreSortTag      CoreSort = 0x04
	CoreSortType     CoreSort = 0x10
	CoreSortModule   CoreSort = 0x11
	CoreSortInstance CoreSort = 0x12
)

func (s CoreSort) String() string {
	switch s {
	case CoreSortFunc:
		return "func"
	case CoreSortTable:
		return "table"
	case CoreSortMemory:
		return "memory"
	case CoreSortGlobal:
		return "global"
	case CoreSortTag:
		return "tag"
	case CoreSortType:
		return "type"
	case CoreSortModule:
		return "module"
	case CoreSortInstance:
		return "instance"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

// AliasKind identifies the kind of alias.
type AliasKind uint8

const (
	AliasKindExport     AliasKind = 0x00 // export alias from component instance
	AliasKindCoreExport AliasKind = 0x01 // core export alias from core instance
	AliasKindOuter      AliasKind = 0x02 // outer alias from enclosing scope
)

func (k AliasKind) String() string {
	switch k {
	case AliasKindExport:
		return "export"
	case AliasKindCoreExport:
		return "core-export"
	case AliasKindOuter:
		return "outer"
	default:
		return fmt.Sprintf("unknown(%d)", k)
	}
}

// Alias represents an alias definition in the component.
// Aliases create references to items in other scopes.
type Alias struct {
	Kind AliasKind
	Sort Sort // What kind of item is being aliased

	// Idx is the target index this alias produces in the appropriate index space.
	// For core export aliases with CoreSortFunc, this is the core function index.
	// For core export aliases with CoreSortMemory, this is the core memory index.
	// This is assigned during binary decoding based on the order of operations.
	Idx uint32

	// For export aliases (Kind == AliasKindExport or AliasKindCoreExport)
	InstanceIdx uint32 // Instance to alias from
	ExportName  string // Name of the export

	// For outer aliases (Kind == AliasKindOuter)
	OuterCount uint32 // Number of enclosing scopes to traverse
	OuterIndex uint32 // Index within that scope

	// For core export aliases, the core sort
	CoreSort CoreSort
}

// ImportExternDescKind identifies the kind of imported item.
type ImportExternDescKind uint8

const (
	ImportExternDescCoreModule ImportExternDescKind = 0x00
	ImportExternDescFunc       ImportExternDescKind = 0x01
	ImportExternDescValue      ImportExternDescKind = 0x02
	ImportExternDescType       ImportExternDescKind = 0x03
	ImportExternDescComponent  ImportExternDescKind = 0x04
	ImportExternDescInstance   ImportExternDescKind = 0x05
)

func (k ImportExternDescKind) String() string {
	switch k {
	case ImportExternDescCoreModule:
		return "core-module"
	case ImportExternDescFunc:
		return "func"
	case ImportExternDescValue:
		return "value"
	case ImportExternDescType:
		return "type"
	case ImportExternDescComponent:
		return "component"
	case ImportExternDescInstance:
		return "instance"
	default:
		return fmt.Sprintf("unknown(%d)", k)
	}
}

// ImportExternDesc describes the type of an import.
type ImportExternDesc struct {
	Kind ImportExternDescKind

	// For func, component, instance: type index
	TypeIdx uint32

	// For core module: core type index (after 0x11 prefix)
	CoreTypeIdx uint32

	// For value: value type reference into Component.Types.
	ValType types.ValType

	// ValueBoundKind indicates the kind of value bound (for value imports)
	ValueBoundKind byte

	// TypeBoundIdx is the type index for type bounds
	TypeBoundIdx *uint32

	// TypeBoundKind indicates the kind of type bound (sub or eq)
	TypeBoundKind TypeBoundKind
}

// Import represents a component import.
type Import struct {
	Name       string           // Import name (kebab-name with optional version)
	ExternDesc ImportExternDesc // What is being imported
}

// CoreInstanceExprKind identifies how a core instance is created.
type CoreInstanceExprKind uint8

const (
	CoreInstanceExprInstantiate CoreInstanceExprKind = 0x00 // Instantiate a module
	CoreInstanceExprInline      CoreInstanceExprKind = 0x01 // Inline exports
)

func (k CoreInstanceExprKind) String() string {
	switch k {
	case CoreInstanceExprInstantiate:
		return "instantiate"
	case CoreInstanceExprInline:
		return "inline"
	default:
		return fmt.Sprintf("unknown(%d)", k)
	}
}

// CoreInstantiateArg is an argument for core module instantiation.
type CoreInstantiateArg struct {
	Name        string // Import name
	InstanceIdx uint32 // Instance to import from (prefixed with 0x12)
}

// CoreInlineExport is an inline export for a core instance.
type CoreInlineExport struct {
	Name string
	Sort CoreSort
	Idx  uint32
}

// CoreInstance represents a core instance definition (section ID 2).
type CoreInstance struct {
	Kind CoreInstanceExprKind

	// For Instantiate
	ModuleIdx uint32
	Args      []CoreInstantiateArg

	// For Inline
	InlineExports []CoreInlineExport
}

// ComponentInstanceExprKind identifies how a component instance is created.
type ComponentInstanceExprKind uint8

const (
	ComponentInstanceExprInstantiate ComponentInstanceExprKind = 0x00 // Instantiate a component
	ComponentInstanceExprInline      ComponentInstanceExprKind = 0x01 // Inline exports
)

func (k ComponentInstanceExprKind) String() string {
	switch k {
	case ComponentInstanceExprInstantiate:
		return "instantiate"
	case ComponentInstanceExprInline:
		return "inline"
	default:
		return fmt.Sprintf("unknown(%d)", k)
	}
}

// ComponentInstantiateArg is an argument for component instantiation.
// Format: n:<name> si:<sortidx>
// sortidx ::= sort:<sort> idx:<u32>
type ComponentInstantiateArg struct {
	Name     string   // Argument name
	Sort     Sort     // What kind of item
	CoreSort CoreSort // For SortCoreSort: the nested core sort
	Idx      uint32   // Index of the item
}

// ComponentInlineExport is an inline export for a component instance.
type ComponentInlineExport struct {
	Name     string
	Sort     Sort
	CoreSort CoreSort // For SortCoreSort: the nested core sort
	Idx      uint32
}

// ParsedComponentInstance represents a component instance definition (section ID 5).
type ParsedComponentInstance struct {
	Kind ComponentInstanceExprKind

	// For Instantiate
	ComponentIdx uint32
	Args         []ComponentInstantiateArg

	// For Inline
	InlineExports []ComponentInlineExport
}

// CoreTypeDef represents a core type definition.
type CoreTypeDef struct {
	Kind     CoreTypeDefKind
	Func     *CoreFuncTypeDef
	Module   *CoreModuleTypeDef
	RecGroup *CoreRecGroupTypeDef
}

// CoreTypeDefKind identifies the kind of core type definition.
type CoreTypeDefKind uint8

const (
	CoreTypeDefKindFunc     CoreTypeDefKind = 0x60
	CoreTypeDefKindModule   CoreTypeDefKind = 0x50
	CoreTypeDefKindRecGroup CoreTypeDefKind = 0x00 // GC proposal rec group (0x00 0x50 prefix)
)

// CoreRecGroupTypeDef represents a GC proposal recursive group type.
// This is used when a non-final sub type is encoded as a top-level core type
// with the 0x00 0x50 prefix to disambiguate from module type.
type CoreRecGroupTypeDef struct {
	// Types contains the sub types in this rec group.
	// For now, we store raw bytes since full GC type parsing isn't required.
	Types []CoreSubType
}

// CoreSubType represents a sub type within a rec group.
type CoreSubType struct {
	// IsFinal indicates if this is a final type (no further subtypes allowed).
	IsFinal bool
	// SuperTypeIndices contains indices of super types.
	SuperTypeIndices []uint32
	// CompositeType is the underlying composite type (func, struct, array).
	CompositeType CoreCompositeType
}

// CoreCompositeType represents a composite type in the GC proposal.
type CoreCompositeType struct {
	Kind CoreCompositeTypeKind
	Func *CoreFuncTypeDef
	// Struct and Array fields could be added for full GC support
}

// CoreCompositeTypeKind identifies the kind of composite type.
type CoreCompositeTypeKind uint8

const (
	CoreCompositeTypeKindFunc   CoreCompositeTypeKind = 0x60
	CoreCompositeTypeKindStruct CoreCompositeTypeKind = 0x5f
	CoreCompositeTypeKindArray  CoreCompositeTypeKind = 0x5e
)

// CoreFuncTypeDef describes a core function type.
type CoreFuncTypeDef struct {
	Params  []byte // Value types
	Results []byte // Value types
}

// CoreModuleTypeDef describes a module type.
type CoreModuleTypeDef struct {
	Imports []CoreImportType
	Exports []CoreExportType
}

// CoreImportType describes a core import in a module type.
type CoreImportType struct {
	Module string
	Name   string
	Kind   byte
}

// CoreExportType describes a core export in a module type.
type CoreExportType struct {
	Name string
	Kind byte
}

// StartDef defines the component start function.
type StartDef struct {
	FuncIdx     uint32   // Function to call
	ArgValueIdx []uint32 // Value indices to pass as arguments
	ResultCount uint32   // Expected number of results
}

// ValueDef represents a component value.
type ValueDef struct {
	Type types.ValType
	Data []byte
}

// TypeBoundKind is the discriminator from Binary.md typebound grammar
// (Binary.md:239-240).
type TypeBoundKind uint8

const (
	// TypeBoundEq is typebound tag 0x00: (eq i), which reads a trailing
	// typeidx. Spec: Binary.md:239.
	TypeBoundEq TypeBoundKind = 0x00
	// TypeBoundSubResource is typebound tag 0x01: (sub resource), with
	// no trailing payload. Spec: Binary.md:240. The new type index
	// refers to a fresh abstract resource type unequal to every
	// existing type.
	TypeBoundSubResource TypeBoundKind = 0x01
)
