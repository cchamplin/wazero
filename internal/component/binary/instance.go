// internal/component/binary/instance.go

package binary

import (
	"bytes"
	"fmt"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/leb128"
)

// decodeComponentInstance decodes a single component instance definition.
func decodeComponentInstance(r *bytes.Reader) (component.ParsedComponentInstance, error) {
	var ci component.ParsedComponentInstance

	kindByte, err := r.ReadByte()
	if err != nil {
		return ci, fmt.Errorf("reading instance kind: %w", err)
	}
	ci.Kind = component.ComponentInstanceExprKind(kindByte)

	switch ci.Kind {
	case component.ComponentInstanceExprInstantiate:
		ci.ComponentIdx, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return ci, fmt.Errorf("reading component index: %w", err)
		}

		argCount, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return ci, fmt.Errorf("reading arg count: %w", err)
		}

		ci.Args = make([]component.ComponentInstantiateArg, argCount)
		for i := uint32(0); i < argCount; i++ {
			name, err := decodeName(r)
			if err != nil {
				return ci, fmt.Errorf("reading arg %d name: %w", i, err)
			}

			sortByte, err := r.ReadByte()
			if err != nil {
				return ci, fmt.Errorf("reading arg %d sort: %w", i, err)
			}

			idx, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return ci, fmt.Errorf("reading arg %d index: %w", i, err)
			}

			ci.Args[i] = component.ComponentInstantiateArg{
				Name: name,
				Sort: component.Sort(sortByte),
				Idx:  idx,
			}
		}

	case component.ComponentInstanceExprInline:
		exportCount, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return ci, fmt.Errorf("reading inline export count: %w", err)
		}

		ci.InlineExports = make([]component.ComponentInlineExport, exportCount)
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

			ci.InlineExports[i] = component.ComponentInlineExport{
				Name: name,
				Sort: component.Sort(sortByte),
				Idx:  idx,
			}
		}

	default:
		return ci, fmt.Errorf("unknown instance kind: 0x%02x", kindByte)
	}

	return ci, nil
}

// decodeInstanceSection parses the instance section (section ID 5).
func decodeInstanceSection(c *component.Component, r *bytes.Reader) error {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("reading instance count: %w", err)
	}

	// Append to existing ComponentInstances instead of replacing
	for i := uint32(0); i < count; i++ {
		ci, err := decodeComponentInstance(r)
		if err != nil {
			return fmt.Errorf("decoding instance %d: %w", i, err)
		}
		c.ComponentInstances = append(c.ComponentInstances, ci)
		c.NextComponentInstanceIdx++
	}

	return nil
}
