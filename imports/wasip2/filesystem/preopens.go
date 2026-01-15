// imports/wasip2/filesystem/preopens.go

package filesystem

import (
	"context"

	"github.com/tetratelabs/wazero/internal/component"
)

// instantiatePreopens registers wasi:filesystem/preopens@0.2.0
func instantiatePreopens(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:filesystem/preopens@0.2.0")

	inst.FuncNoType("get-directories", getDirectories)

	return inst.Build()
}

// getDirectories returns the list of preopened directories.
// Signature: func() -> list<tuple<own<descriptor>, string>>
func getDirectories(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return empty list as placeholder
	// Full implementation will get preopened directories from config
	return []component.Val{component.ValList([]component.Val{})}, nil
}
