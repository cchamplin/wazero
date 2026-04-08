// imports/wasip2/filesystem/preopens.go

package filesystem

import (
	"context"
	"os"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/types"
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

	return inst.SkipValidation().Build()
}

// getDirectories returns the list of preopened directories.
// Signature: func() -> list<tuple<own<descriptor>, string>>
func getDirectories(ctx context.Context, args []types.Val) ([]types.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		// No resource table, return empty list
		return []types.Val{types.ValList([]types.Val{})}, nil
	}

	// Try to get preopens config from context
	wasiConfig := component.WASIConfigFromContext(ctx)
	if wasiConfig == nil {
		// No config, return empty list
		return []types.Val{types.ValList([]types.Val{})}, nil
	}

	// Check if config implements PreopensConfig
	preopensConfig, ok := wasiConfig.(PreopensConfig)
	if !ok {
		// Config doesn't have preopens, return empty list
		return []types.Val{types.ValList([]types.Val{})}, nil
	}

	preopens := preopensConfig.Preopens()
	if len(preopens) == 0 {
		return []types.Val{types.ValList([]types.Val{})}, nil
	}

	// Build list of tuples
	tuples := make([]types.Val, 0, len(preopens))
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
		handle, hErr := table.NewResourceHandle(desc, true, descriptorResourceType)
		if hErr != nil {
			file.Close()
			continue
		}

		// Create tuple: (own<descriptor>, string)
		handleVal := types.ValOwn(uint32(handle.Index()))
		pathVal := types.ValString(guestPath)
		tuple := types.ValTuple([]types.Val{handleVal, pathVal})
		tuples = append(tuples, tuple)
	}

	return []types.Val{types.ValList(tuples)}, nil
}
