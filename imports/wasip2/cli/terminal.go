// Package cli implements the wasi:cli interfaces for WASI Preview 2.
package cli

import (
	"os"

	"golang.org/x/term"
)

// TerminalInput is a marker resource for terminal input handles.
// Currently has no methods per WASI spec - future versions may add
// echo control, buffering settings, etc.
type TerminalInput struct{}

// TerminalOutput is a marker resource for terminal output handles.
// Currently has no methods per WASI spec - future versions may add
// terminal size queries, resize notifications, etc.
type TerminalOutput struct{}

// detectTerminal checks if the given reader/writer is connected to a TTY.
// Returns true only if it's an *os.File with a valid terminal fd.
func detectTerminal(stream interface{}) bool {
	// Try to get the underlying file descriptor
	var fd int

	switch s := stream.(type) {
	case *os.File:
		fd = int(s.Fd())
	case interface{ Fd() uintptr }:
		// Support wrappers that expose Fd()
		fd = int(s.Fd())
	default:
		return false
	}

	return term.IsTerminal(fd)
}
