// internal/component/binary/import.go

package binary

import (
	"bytes"
	"fmt"
)

// decodeImportName decodes an import name with optional version suffix.
// Format: 0x00 len name       (plain name)
//
//	| 0x01 len name       (name with version suffix embedded)
func decodeImportName(r *bytes.Reader) (string, error) {
	prefix, err := r.ReadByte()
	if err != nil {
		return "", fmt.Errorf("reading import name prefix: %w", err)
	}

	switch prefix {
	case 0x00, 0x01:
		// Both cases: read length-prefixed name
		// The version suffix is embedded in the name string itself
		return decodeName(r)
	default:
		return "", fmt.Errorf("unknown import name prefix: 0x%02x", prefix)
	}
}
