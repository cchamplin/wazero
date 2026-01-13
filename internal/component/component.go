// internal/component/component.go

package component

import "github.com/tetratelabs/wazero/internal/wasm"

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
// Either a primitive type opcode or a type index.
type ValTypeRef struct {
	// IsPrimitive is true if this is a primitive type (0x73-0x7f).
	IsPrimitive bool

	// Primitive is the primitive type opcode (if IsPrimitive).
	Primitive byte

	// TypeIdx is the type index (if !IsPrimitive).
	TypeIdx uint32
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
