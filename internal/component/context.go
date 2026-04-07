// internal/component/context.go

package component

import (
	"context"
	"io"

	"github.com/tetratelabs/wazero/internal/component/runtime"
)

// TerminalMode controls how terminal detection behaves for WASI CLI interfaces.
type TerminalMode int

const (
	// TerminalModeNone always reports no terminal connection (safe default).
	TerminalModeNone TerminalMode = iota
	// TerminalModeAuto detects real TTY by checking if streams are *os.File
	// with valid terminal file descriptors.
	TerminalModeAuto
	// TerminalModeCustom uses explicit host-provided values.
	TerminalModeCustom
)

// contextKey is a type for context keys in the component package.
type contextKey int

const (
	// resourceTableContextKey is the context key for ResourceTable.
	resourceTableContextKey contextKey = iota
	// wasiConfigContextKey is the context key for WASIConfig.
	wasiConfigContextKey
)

// WithResourceTable returns a new context with the given resource table stored.
// The old runtime.ResourceTable type has been unified into runtime.Table.
func WithResourceTable(ctx context.Context, table *runtime.Table) context.Context {
	return context.WithValue(ctx, resourceTableContextKey, table)
}

// ResourceTableFromContext retrieves the resource table from the context.
// Returns nil if no table is set.
func ResourceTableFromContext(ctx context.Context) *runtime.Table {
	table, _ := ctx.Value(resourceTableContextKey).(*runtime.Table)
	return table
}

// WASIConfig is an interface for WASI configuration that can be stored in context.
// This interface is defined here to avoid import cycles between wasip2 and cli packages.
type WASIConfig interface {
	// Stdin returns the stdin reader.
	Stdin() io.Reader
	// Stdout returns the stdout writer.
	Stdout() io.Writer
	// Stderr returns the stderr writer.
	Stderr() io.Writer
	// Environ returns environment variables as "KEY=value" strings.
	Environ() []string
	// Args returns command-line arguments.
	Args() []string
	// TerminalMode returns the terminal detection mode.
	TerminalMode() TerminalMode
	// StdinIsTerminal returns true if stdin is a terminal (used with TerminalModeCustom).
	StdinIsTerminal() bool
	// StdoutIsTerminal returns true if stdout is a terminal (used with TerminalModeCustom).
	StdoutIsTerminal() bool
	// StderrIsTerminal returns true if stderr is a terminal (used with TerminalModeCustom).
	StderrIsTerminal() bool
}

// WithWASIConfig returns a new context with the given WASIConfig stored.
func WithWASIConfig(ctx context.Context, config WASIConfig) context.Context {
	return context.WithValue(ctx, wasiConfigContextKey, config)
}

// WASIConfigFromContext retrieves the WASIConfig from the context.
// Returns nil if no WASIConfig is set.
func WASIConfigFromContext(ctx context.Context) WASIConfig {
	config, _ := ctx.Value(wasiConfigContextKey).(WASIConfig)
	return config
}
