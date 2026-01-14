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

	// Types contains component type definitions (section ID 7).
	// This includes function types, component types, instance types, etc.
	Types []TypeDef

	// Canonicals contains canonical function definitions (section ID 8).
	// These define lift/lower wrappers around core functions.
	Canonicals []CanonicalDef

	// Exports contains component exports (section ID 11).
	// These expose functions and instances to the outside world.
	Exports []Export

	// Aliases contains alias definitions (section ID 6).
	Aliases []Alias

	// Imports contains component imports (section ID 10).
	Imports []Import

	// CoreInstances contains core instance definitions (section ID 2).
	CoreInstances []CoreInstance
}

// TypeDef represents a component type definition.
// This is a discriminated union of different type kinds.
type TypeDef struct {
	Kind TypeDefKind

	// For FuncType
	Func *FuncType

	// For Defined types (record, variant, etc.)
	// These are stored as parsed structures from the binary package.
	// Record holds the decoded record type definition.
	Record interface{}

	// Option holds the decoded option type definition.
	Option interface{}

	// List holds the decoded list type definition.
	List interface{}

	// Result holds the decoded result type definition.
	Result interface{}

	// Resource holds the decoded resource type definition.
	Resource interface{}
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

// FuncType represents a component function type.
// Format: 0x40 paramlist resultlist
type FuncType struct {
	Params  []NamedValType // Named parameters
	Results []NamedValType // Named results (may be unnamed for single result)
}

// NamedValType is a (name, type) pair used in function parameters/results.
type NamedValType struct {
	Name    string
	ValType ValTypeRef
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
	Name string
	Kind ExportKind
	Idx  uint32 // Index into the appropriate index space
}

// ExportKind identifies what kind of item is being exported.
type ExportKind uint8

const (
	ExportKindFunc ExportKind = iota
	ExportKindValue
	ExportKindType
	ExportKindComponent
	ExportKindInstance
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

	// For value: value bound
	// For type: type bound
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
