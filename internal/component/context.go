// internal/component/context.go

package component

import "context"

// contextKey is a type for context keys in the component package.
type contextKey int

const (
	// resourceTableContextKey is the context key for ResourceTable.
	resourceTableContextKey contextKey = iota
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
