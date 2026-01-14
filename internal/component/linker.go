// internal/component/linker.go

package component

import (
	"context"
	"fmt"
)

// HostFunc is a host function that can be called from a component.
type HostFunc func(ctx context.Context, args []Val) ([]Val, error)

// Definition is an item that can satisfy a component import.
type Definition interface {
	definition()
}

// FuncDef is a function definition.
type FuncDef struct {
	Type     *FuncType
	Callback HostFunc
}

func (*FuncDef) definition() {}

// InstanceDef is an instance definition with multiple exports.
type InstanceDef struct {
	Exports map[string]Definition
}

func (*InstanceDef) definition() {}

// ResourceDef is a resource type definition.
type ResourceDef struct {
	Destructor func(rep uint32)
}

func (*ResourceDef) definition() {}

// Linker resolves component imports and instantiates components.
type Linker struct {
	definitions map[string]Definition
}

// NewLinker creates a new component linker.
func NewLinker() *Linker {
	return &Linker{
		definitions: make(map[string]Definition),
	}
}

// DefineFunc adds a host function definition.
func (l *Linker) DefineFunc(namespace, name string, typ *FuncType, fn HostFunc) error {
	key := namespace + "/" + name
	if _, exists := l.definitions[key]; exists {
		return fmt.Errorf("definition already exists: %s", key)
	}
	l.definitions[key] = &FuncDef{Type: typ, Callback: fn}
	return nil
}
