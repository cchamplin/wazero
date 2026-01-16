// internal/component/index_space.go
package component

import "fmt"

// CoreFuncIndexSpace tracks the mapping from component function indices
// to core instance exports.
type CoreFuncIndexSpace struct {
	aliases map[uint32]coreExportRef
}

type coreExportRef struct {
	InstanceIdx uint32
	ExportName  string
}

// NewCoreFuncIndexSpace creates a new function index space tracker.
func NewCoreFuncIndexSpace() *CoreFuncIndexSpace {
	return &CoreFuncIndexSpace{
		aliases: make(map[uint32]coreExportRef),
	}
}

// AddAlias maps a component function index to a core instance export.
func (s *CoreFuncIndexSpace) AddAlias(funcIdx, instanceIdx uint32, exportName string) {
	s.aliases[funcIdx] = coreExportRef{
		InstanceIdx: instanceIdx,
		ExportName:  exportName,
	}
}

// Resolve looks up a component function index and returns the core instance
// index and export name it references.
func (s *CoreFuncIndexSpace) Resolve(funcIdx uint32) (instanceIdx uint32, exportName string, err error) {
	ref, ok := s.aliases[funcIdx]
	if !ok {
		return 0, "", fmt.Errorf("core function index %d not found", funcIdx)
	}
	return ref.InstanceIdx, ref.ExportName, nil
}

// CoreMemoryIndexSpace tracks the mapping from component memory indices
// to core instance exports.
type CoreMemoryIndexSpace struct {
	aliases map[uint32]coreExportRef
}

// NewCoreMemoryIndexSpace creates a new memory index space tracker.
func NewCoreMemoryIndexSpace() *CoreMemoryIndexSpace {
	return &CoreMemoryIndexSpace{
		aliases: make(map[uint32]coreExportRef),
	}
}

// AddAlias maps a component memory index to a core instance export.
func (s *CoreMemoryIndexSpace) AddAlias(memIdx, instanceIdx uint32, exportName string) {
	s.aliases[memIdx] = coreExportRef{
		InstanceIdx: instanceIdx,
		ExportName:  exportName,
	}
}

// Resolve looks up a component memory index.
func (s *CoreMemoryIndexSpace) Resolve(memIdx uint32) (instanceIdx uint32, exportName string, err error) {
	ref, ok := s.aliases[memIdx]
	if !ok {
		return 0, "", fmt.Errorf("core memory index %d not found", memIdx)
	}
	return ref.InstanceIdx, ref.ExportName, nil
}
