// internal/component/binary/import_test.go

package binary

import (
	"bytes"
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestDecodeImportName_Plain(t *testing.T) {
	// 0x00 prefix = plain name without version suffix
	data := []byte{
		0x00,                           // plain name
		0x08,                           // length
		't', 'e', 's', 't', '-', 'a', 'p', 'i',
	}

	name, err := decodeImportName(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, "test-api", name)
}

func TestDecodeImportName_WithVersion(t *testing.T) {
	// 0x01 prefix = name with version suffix
	data := []byte{
		0x01,                           // with version suffix
		0x0a,                           // length = 10
		't', 'e', 's', 't', '@', '1', '.', '2', '.', '3',
	}

	name, err := decodeImportName(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, "test@1.2.3", name)
}
