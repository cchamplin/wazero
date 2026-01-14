// internal/component/binary/core_instance.go

package binary

import (
	"bytes"
	"fmt"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/leb128"
)

// decodeCoreInstance decodes a single core instance definition.
func decodeCoreInstance(r *bytes.Reader) (component.CoreInstance, error) {
	var ci component.CoreInstance

	kindByte, err := r.ReadByte()
	if err != nil {
		return ci, fmt.Errorf("reading core instance kind: %w", err)
	}
	ci.Kind = component.CoreInstanceExprKind(kindByte)

	switch ci.Kind {
	case component.CoreInstanceExprInstantiate:
		ci.ModuleIdx, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return ci, fmt.Errorf("reading module index: %w", err)
		}

		argCount, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return ci, fmt.Errorf("reading arg count: %w", err)
		}

		ci.Args = make([]component.CoreInstantiateArg, argCount)
		for i := uint32(0); i < argCount; i++ {
			name, err := decodeName(r)
			if err != nil {
				return ci, fmt.Errorf("reading arg %d name: %w", i, err)
			}

			// Read sort byte (must be 0x12 for instance)
			sortByte, err := r.ReadByte()
			if err != nil {
				return ci, fmt.Errorf("reading arg %d sort: %w", i, err)
			}
			if sortByte != 0x12 {
				return ci, fmt.Errorf("expected instance sort 0x12, got 0x%02x", sortByte)
			}

			instanceIdx, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return ci, fmt.Errorf("reading arg %d instance index: %w", i, err)
			}

			ci.Args[i] = component.CoreInstantiateArg{
				Name:        name,
				InstanceIdx: instanceIdx,
			}
		}

	case component.CoreInstanceExprInline:
		exportCount, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return ci, fmt.Errorf("reading inline export count: %w", err)
		}

		ci.InlineExports = make([]component.CoreInlineExport, exportCount)
		for i := uint32(0); i < exportCount; i++ {
			name, err := decodeName(r)
			if err != nil {
				return ci, fmt.Errorf("reading export %d name: %w", i, err)
			}

			sortByte, err := r.ReadByte()
			if err != nil {
				return ci, fmt.Errorf("reading export %d sort: %w", i, err)
			}

			idx, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return ci, fmt.Errorf("reading export %d index: %w", i, err)
			}

			ci.InlineExports[i] = component.CoreInlineExport{
				Name: name,
				Sort: component.CoreSort(sortByte),
				Idx:  idx,
			}
		}

	default:
		return ci, fmt.Errorf("unknown core instance kind: 0x%02x", kindByte)
	}

	return ci, nil
}

// decodeCoreInstanceSection parses the core instance section (section ID 2).
func decodeCoreInstanceSection(c *component.Component, r *bytes.Reader) error {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("reading core instance count: %w", err)
	}

	c.CoreInstances = make([]component.CoreInstance, count)
	for i := uint32(0); i < count; i++ {
		ci, err := decodeCoreInstance(r)
		if err != nil {
			return fmt.Errorf("decoding core instance %d: %w", i, err)
		}
		c.CoreInstances[i] = ci
	}

	return nil
}
