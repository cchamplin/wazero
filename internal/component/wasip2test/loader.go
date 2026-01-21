package wasip2test

import (
	"os"
	"path/filepath"
	"strings"
)

// WasiTestLoader helps load pre-compiled WASI test components
type WasiTestLoader struct {
	baseDir string
}

// NewWasiTestLoader creates a loader for test components in the given directory
func NewWasiTestLoader(baseDir string) *WasiTestLoader {
	return &WasiTestLoader{baseDir: baseDir}
}

// List returns all .wasm files in the base directory
func (l *WasiTestLoader) List() ([]string, error) {
	var components []string
	err := filepath.Walk(l.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".wasm") {
			components = append(components, path)
		}
		return nil
	})
	return components, err
}

// Load reads a component by name from the base directory
func (l *WasiTestLoader) Load(name string) ([]byte, error) {
	path := filepath.Join(l.baseDir, name)
	if !strings.HasSuffix(path, ".wasm") {
		path += ".wasm"
	}
	return os.ReadFile(path)
}
