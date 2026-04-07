// internal/component/context_test.go

package component

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestWithResourceTable(t *testing.T) {
	// Session 0 compile-fix: runtime.NewResourceTable was renamed to
	// runtime.NewTable in Task 10 (the legacy ResourceTable type was
	// unified into Table).
	table := runtime.NewTable()
	ctx := WithResourceTable(context.Background(), table)

	retrieved := ResourceTableFromContext(ctx)
	require.Same(t, table, retrieved)
}

func TestResourceTableFromContext_Nil(t *testing.T) {
	ctx := context.Background()
	retrieved := ResourceTableFromContext(ctx)
	require.Nil(t, retrieved)
}

func TestResourceTableFromContext_WrongType(t *testing.T) {
	// Put something else in the context with the same key type
	ctx := context.WithValue(context.Background(), resourceTableContextKey, "not a resource table")
	retrieved := ResourceTableFromContext(ctx)
	require.Nil(t, retrieved)
}
