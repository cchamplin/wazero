// internal/component/binary/errors.go

package binary

import "errors"

// Decoder errors
var (
	// ErrInvalidMagic is returned when the magic number is not "\0asm".
	ErrInvalidMagic = errors.New("invalid component: bad magic number")

	// ErrInvalidVersion is returned when the version is not recognized.
	ErrInvalidVersion = errors.New("invalid component: bad version")

	// ErrInvalidLayer is returned when the layer byte indicates a core module.
	ErrInvalidLayer = errors.New("invalid component: not a component (core module?)")

	// ErrUnexpectedEOF is returned when the binary ends unexpectedly.
	ErrUnexpectedEOF = errors.New("invalid component: unexpected end of file")
)
