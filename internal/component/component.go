// internal/component/component.go

package component

import (
	"fmt"

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

	// Types contains component type definitions (section ID 7).
	// This includes function types, component types, instance types, etc.
	Types []TypeDef

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
	ComponentInstances []ComponentInstance

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
}

// TypeDef represents a component type definition.
// This is a discriminated union of different type kinds.
type TypeDef struct {
	Kind TypeDefKind

	// For FuncType
	Func *FuncType

	// For Defined types (record, variant, etc.)
	// Record holds the decoded record type definition.
	Record *RecordTypeDef

	// Option holds the decoded option type definition.
	Option *OptionTypeDef

	// List holds the decoded list type definition.
	List *ListTypeDef

	// Result holds the decoded result type definition.
	Result *ResultTypeDef

	// Resource holds the decoded resource type definition.
	Resource interface{}

	// Variant holds the decoded variant type definition.
	Variant *VariantTypeDef

	// Tuple holds the decoded tuple type definition.
	Tuple *TupleTypeDef

	// Flags holds the decoded flags type definition.
	Flags *FlagsTypeDef

	// Enum holds the decoded enum type definition.
	Enum *EnumTypeDef

	// Instance holds the decoded instance type definition (0x42).
	Instance *InstanceTypeDef

	// Component holds the decoded component type definition (0x41).
	Component *ComponentTypeDef

	// Stream holds the decoded stream type definition (0x66).
	Stream *StreamTypeDef

	// Future holds the decoded future type definition (0x65).
	Future *FutureTypeDef

	// FixedSizeList holds the decoded fixed-size list type definition (0x67).
	FixedSizeList *FixedSizeListTypeDef

	// Handle holds a handle type (own<T> or borrow<T>) definition.
	// The ValTypeRef will have IsOwn or IsBorrow set with TypeIdx pointing to the resource.
	Handle *ValTypeRef
}

// TypeDefKind identifies the kind of type definition.
type TypeDefKind uint8

const (
	TypeDefKindFunc TypeDefKind = iota
	TypeDefKindComponent
	TypeDefKindInstance
	TypeDefKindResource
	TypeDefKindDefined
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

// FuncType represents a component function type.
// Format: 0x40 paramlist resultlist
type FuncType struct {
	Params  []NamedValType // Named parameters
	Results []NamedValType // Named results (may be unnamed for single result)
}

// NamedValType is a (name, type) pair used in function parameters/results.
type NamedValType struct {
	Name         string
	ValType      ValTypeRef
	ResolvedType *TypeDef // Optional: resolved type definition when ValType is a type reference
}

// ValTypeRef is a reference to a value type.
// Either a primitive type opcode, a type index, or a handle type.
type ValTypeRef struct {
	// IsPrimitive is true if this is a primitive type (0x73-0x7f).
	IsPrimitive bool

	// Primitive is the primitive type opcode (if IsPrimitive).
	Primitive byte

	// TypeIdx is the type index (if !IsPrimitive and !IsOwn and !IsBorrow).
	// For own and borrow handles, this is the resource type index.
	TypeIdx uint32

	// IsOwn is true if this is an own<T> handle type.
	// When true, TypeIdx contains the resource type index.
	IsOwn bool

	// IsBorrow is true if this is a borrow<T> handle type.
	// When true, TypeIdx contains the resource type index.
	IsBorrow bool
}

// CanonicalDef represents a canonical function definition.
type CanonicalDef struct {
	Kind CanonKind

	// ComponentFuncIdx is the component function index assigned to this canonical.
	// For Lift: this is the index in the component function index space.
	// For Lower: this is the core function index that is created.
	// For Resource operations: not used.
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
)

// CanonicalOptions holds optional parameters for canonical operations.
type CanonicalOptions struct {
	StringEncoding StringEncoding
	MemoryIdx      *uint32 // nil if not specified
	ReallocIdx     *uint32 // nil if not specified
	PostReturnIdx  *uint32 // nil if not specified
}

// StringEncoding specifies how strings are encoded.
type StringEncoding uint8

const (
	StringEncodingUTF8 StringEncoding = iota
	StringEncodingUTF16
	StringEncodingLatin1UTF16
)

// Export represents a component export.
type Export struct {
	Name    string
	Kind    ExportKind
	Idx     uint32  // Index into the appropriate index space
	TypeIdx *uint32 // Optional type annotation
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

	// For value: value type reference
	ValType *ValTypeRef

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
	Name string // Argument name
	Sort Sort   // What kind of item
	Idx  uint32 // Index of the item
}

// ComponentInlineExport is an inline export for a component instance.
type ComponentInlineExport struct {
	Name string
	Sort Sort
	Idx  uint32
}

// ComponentInstance represents a component instance definition (section ID 5).
type ComponentInstance struct {
	Kind ComponentInstanceExprKind

	// For Instantiate
	ComponentIdx uint32
	Args         []ComponentInstantiateArg

	// For Inline
	InlineExports []ComponentInlineExport
}

// VariantTypeDef represents a variant (tagged union) type.
type VariantTypeDef struct {
	Cases []VariantCase
}

// VariantCase represents a single case in a variant type.
type VariantCase struct {
	Name    string
	ValType *ValTypeRef // nil for cases without payload
}

// TupleTypeDef represents a tuple type with fixed elements.
type TupleTypeDef struct {
	Types []ValTypeRef
}

// FlagsTypeDef represents a flags (bitfield) type.
type FlagsTypeDef struct {
	Names []string
}

// EnumTypeDef represents an enumeration type.
type EnumTypeDef struct {
	Names []string
}

// RecordTypeDef represents a record (struct) type definition.
type RecordTypeDef struct {
	Fields []RecordField
}

// RecordField represents a field in a record type.
type RecordField struct {
	Name    string
	ValType ValTypeRef
}

// ListTypeDef represents a list type definition.
type ListTypeDef struct {
	ElementType ValTypeRef
}

// OptionTypeDef represents an option type definition.
type OptionTypeDef struct {
	InnerType ValTypeRef
}

// ResultTypeDef represents a result type definition.
type ResultTypeDef struct {
	OkType  *ValTypeRef // nil for result<_, E>
	ErrType *ValTypeRef // nil for result<T, _>
}

// StreamTypeDef represents a stream type (0x66).
type StreamTypeDef struct {
	ElementType *ValTypeRef // nil if no element type
	EndType     *ValTypeRef // nil if no end type
}

// FutureTypeDef represents a future type (0x65).
type FutureTypeDef struct {
	PayloadType *ValTypeRef // nil if no payload
}

// FixedSizeListTypeDef represents a fixed-size list type (0x67).
type FixedSizeListTypeDef struct {
	ElementType ValTypeRef
	Size        uint32
}

// CoreTypeDef represents a core type definition.
type CoreTypeDef struct {
	Kind   CoreTypeDefKind
	Func   *CoreFuncTypeDef
	Module *CoreModuleTypeDef
}

// CoreTypeDefKind identifies the kind of core type definition.
type CoreTypeDefKind uint8

const (
	CoreTypeDefKindFunc   CoreTypeDefKind = 0x60
	CoreTypeDefKindModule CoreTypeDefKind = 0x50
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
	Type ValTypeRef
	Data []byte
}

// TypeBoundKind represents the kind of type bound.
type TypeBoundKind uint8

const (
	TypeBoundSub TypeBoundKind = 0x00
	TypeBoundEq  TypeBoundKind = 0x01
)
