package binary

import (
	"bytes"
	"fmt"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/leb128"
)

func decodeStartSection(c *component.Component, r *bytes.Reader) error {
	funcIdx, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("read start func index: %w", err)
	}

	argCount, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("read start arg count: %w", err)
	}

	args := make([]uint32, argCount)
	for i := uint32(0); i < argCount; i++ {
		argIdx, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return fmt.Errorf("read start arg %d: %w", i, err)
		}
		args[i] = argIdx
	}

	resultCount, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("read start result count: %w", err)
	}

	c.Start = &component.StartDef{
		FuncIdx:     funcIdx,
		ArgValueIdx: args,
		ResultCount: resultCount,
	}

	return nil
}
