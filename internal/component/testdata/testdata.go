// internal/component/testdata/testdata.go

package testdata

import (
	_ "embed"
)

// EmptyComponent is a minimal valid component with no content.
// Binary: magic(4) + version(2) + layer(2) = 8 bytes
//
//go:embed empty.wasm
var EmptyComponent []byte
