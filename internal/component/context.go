// internal/component/context.go

package component

import (
	"context"
	"io"
)

// contextKey is a type for context keys in the component package.
type contextKey int

const (
	// resourceTableContextKey is the context key for ResourceTable.
	resourceTableContextKey contextKey = iota
	// wasiConfigContextKey is the context key for WASIConfig.
	wasiConfigContextKey
)

// WithResourceTable returns a new context with the given ResourceTable stored.
func WithResourceTable(ctx context.Context, table *ResourceTable) context.Context {
	return context.WithValue(ctx, resourceTableContextKey, table)
}

// ResourceTableFromContext retrieves the ResourceTable from the context.
// Returns nil if no ResourceTable is set.
func ResourceTableFromContext(ctx context.Context) *ResourceTable {
	table, _ := ctx.Value(resourceTableContextKey).(*ResourceTable)
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
