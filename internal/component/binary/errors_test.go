// internal/component/binary/errors_test.go

package binary

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestErrors(t *testing.T) {
	require.EqualError(t, ErrInvalidMagic, "invalid component: bad magic number")
	require.EqualError(t, ErrInvalidVersion, "invalid component: bad version")
	require.EqualError(t, ErrInvalidLayer, "invalid component: not a component (core module?)")
	require.EqualError(t, ErrUnexpectedEOF, "invalid component: unexpected end of file")
}
