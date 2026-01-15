// imports/wasip2/filesystem/preopens.go

package filesystem

import (
	"context"
	"os"

	"github.com/tetratelabs/wazero/internal/component"
)

// PreopensConfig is an interface for accessing preopen configuration.
// This allows the filesystem package to access preopens without importing wasip2.
type PreopensConfig interface {
	Preopens() map[string]string
}

// instantiatePreopens registers wasi:filesystem/preopens@0.2.0
func instantiatePreopens(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:filesystem/preopens@0.2.0")

	inst.FuncNoType("get-directories", getDirectories)

	return inst.Build()
}

// getDirectories returns the list of preopened directories.
// Signature: func() -> list<tuple<own<descriptor>, string>>
func getDirectories(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		// No resource table, return empty list
		return []component.Val{component.ValList([]component.Val{})}, nil
	}

	// Try to get preopens config from context
	wasiConfig := component.WASIConfigFromContext(ctx)
	if wasiConfig == nil {
		// No config, return empty list
		return []component.Val{component.ValList([]component.Val{})}, nil
	}

	// Check if config implements PreopensConfig
	preopensConfig, ok := wasiConfig.(PreopensConfig)
	if !ok {
		// Config doesn't have preopens, return empty list
		return []component.Val{component.ValList([]component.Val{})}, nil
	}

	preopens := preopensConfig.Preopens()
	if len(preopens) == 0 {
		return []component.Val{component.ValList([]component.Val{})}, nil
	}

	// Build list of tuples
	tuples := make([]component.Val, 0, len(preopens))
	for guestPath, hostPath := range preopens {
		// Open the host directory
		file, err := os.Open(hostPath)
		if err != nil {
			// Skip directories that can't be opened
			continue
		}

		// Verify it's a directory
		info, err := file.Stat()
		if err != nil || !info.IsDir() {
			file.Close()
			continue
		}

		// Create descriptor with read+write+mutate-directory permissions
		desc := NewDescriptor(file, true, hostPath, DescriptorFlagRead|DescriptorFlagWrite|DescriptorFlagMutateDirectory)

		// Add to resource table
		handle := table.New(desc, true)

		// Create tuple: (own<descriptor>, string)
		handleVal := component.ValOwn(uint32(handle.Index()))
		pathVal := component.ValString(guestPath)
		tuple := component.ValTuple([]component.Val{handleVal, pathVal})
		tuples = append(tuples, tuple)
	}

	return []component.Val{component.ValList(tuples)}, nil
}
