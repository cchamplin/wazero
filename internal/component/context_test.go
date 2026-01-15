// internal/component/context_test.go

package component

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestWithResourceTable(t *testing.T) {
	table := NewResourceTable()
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
