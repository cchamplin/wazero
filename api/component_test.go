// api/component_test.go
package api_test

import (
	"testing"

	"github.com/tetratelabs/wazero/api"
)

func TestComponentInterfaceExists(t *testing.T) {
	// Verify that the interface types exist and are usable
	var _ api.CompiledComponent
	var _ api.Component
	var _ api.ComponentFunc
	var _ api.ComponentLinker
	var _ api.ComponentImport
	var _ api.ComponentExport
}
