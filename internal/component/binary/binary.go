// Package binary provides constants for WebAssembly Component Model binary format.
package binary

import (
	"bytes"
	"fmt"
)

// Magic is the 4-byte preamble for all WebAssembly binaries (modules and components).
// See https://github.com/WebAssembly/component-model/blob/main/design/mvp/Binary.md
var Magic = [4]byte{0x00, 0x61, 0x73, 0x6d} // "\0asm"

// Version is the component model version (pre-standard).
// This will change to [4]byte{0x01, 0x00, 0x01, 0x00} at 1.0.
var Version = [2]byte{0x0d, 0x00}

// LayerComponent identifies a binary as a component (vs core module).
var LayerComponent = [2]byte{0x01, 0x00}

// LayerModule identifies a binary as a core module (vs component).
var LayerModule = [2]byte{0x00, 0x00}

// IsComponent returns true if the binary appears to be a component
// (as opposed to a core wasm module).
//
// This checks:
//   - Magic number matches "\0asm"
//   - Version is component version (0x0d 0x00)
//   - Layer byte is component layer (0x01 0x00)
func IsComponent(binary []byte) bool {
	// Need at least: magic(4) + version(2) + layer(2) = 8 bytes
	if len(binary) < 8 {
		return false
	}

	// Check magic
	if !bytes.Equal(binary[0:4], Magic[:]) {
		return false
	}

	// Check version (component model pre-standard version)
	if !bytes.Equal(binary[4:6], Version[:]) {
		return false
	}

	// Check layer (component vs module)
	if !bytes.Equal(binary[6:8], LayerComponent[:]) {
		return false
	}

	return true
}

// SectionID identifies a component section.
// Component sections have different IDs than core wasm sections.
type SectionID byte

const (
	// SectionIDCoreCustom is for custom sections within components.
	SectionIDCoreCustom SectionID = 0

	// SectionIDCoreModule contains an embedded core wasm module.
	SectionIDCoreModule SectionID = 1

	// SectionIDCoreInstance instantiates a core module.
	SectionIDCoreInstance SectionID = 2

	// SectionIDCoreType defines core types (for use in aliases).
	SectionIDCoreType SectionID = 3

	// SectionIDComponent contains a nested component.
	SectionIDComponent SectionID = 4

	// SectionIDInstance instantiates a component.
	SectionIDInstance SectionID = 5

	// SectionIDAlias creates aliases to items in other scopes.
	SectionIDAlias SectionID = 6

	// SectionIDType defines component types (functions, resources, etc).
	SectionIDType SectionID = 7

	// SectionIDCanon defines canonical functions (lift/lower).
	SectionIDCanon SectionID = 8

	// SectionIDStart specifies the component start function.
	SectionIDStart SectionID = 9

	// SectionIDImport declares component imports.
	SectionIDImport SectionID = 10

	// SectionIDExport declares component exports.
	SectionIDExport SectionID = 11

	// SectionIDValue defines component values (gated feature).
	SectionIDValue SectionID = 12
)

// String returns a human-readable section name.
func (s SectionID) String() string {
	switch s {
	case SectionIDCoreCustom:
		return "core-custom"
	case SectionIDCoreModule:
		return "core-module"
	case SectionIDCoreInstance:
		return "core-instance"
	case SectionIDCoreType:
		return "core-type"
	case SectionIDComponent:
		return "component"
	case SectionIDInstance:
		return "instance"
	case SectionIDAlias:
		return "alias"
	case SectionIDType:
		return "type"
	case SectionIDCanon:
		return "canon"
	case SectionIDStart:
		return "start"
	case SectionIDImport:
		return "import"
	case SectionIDExport:
		return "export"
	case SectionIDValue:
		return "value"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}
