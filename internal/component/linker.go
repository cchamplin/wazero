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

// InstanceBuilder builds an instance definition with multiple exports.
type InstanceBuilder struct {
	linker    *Linker
	namespace string
	exports   map[string]Definition
}

// DefineInstance starts building an instance definition.
func (l *Linker) DefineInstance(namespace string) *InstanceBuilder {
	return &InstanceBuilder{
		linker:    l,
		namespace: namespace,
		exports:   make(map[string]Definition),
	}
}

// Func adds a function export to the instance.
func (b *InstanceBuilder) Func(name string, typ *FuncType, fn HostFunc) *InstanceBuilder {
	b.exports[name] = &FuncDef{Type: typ, Callback: fn}
	return b
}

// Build finalizes the instance definition and registers it with the linker.
func (b *InstanceBuilder) Build() error {
	if _, exists := b.linker.definitions[b.namespace]; exists {
		return fmt.Errorf("definition already exists: %s", b.namespace)
	}
	b.linker.definitions[b.namespace] = &InstanceDef{Exports: b.exports}
	return nil
}
