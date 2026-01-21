// internal/component/import_name_test.go

package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestParseImportName_Interface(t *testing.T) {
	result, err := ParseImportName("wasi:io/streams@0.2.0")
	require.NoError(t, err)
	require.Equal(t, ImportNameKindInterface, result.Kind)
	require.Equal(t, "wasi:io", result.Namespace)
	require.Equal(t, "streams", result.Name)
	require.NotNil(t, result.Version)
	require.Equal(t, uint32(0), result.Version.Major)
	require.Equal(t, uint32(2), result.Version.Minor)
	require.Equal(t, uint32(0), result.Version.Patch)
}

func TestParseImportName_LockedDep(t *testing.T) {
	result, err := ParseImportName("locked-dep=my-pkg:1.2.3")
	require.NoError(t, err)
	require.Equal(t, ImportNameKindLockedDep, result.Kind)
	require.Equal(t, "my-pkg", result.DepName)
	require.NotNil(t, result.Version)
	require.Equal(t, uint32(1), result.Version.Major)
	require.Equal(t, uint32(2), result.Version.Minor)
	require.Equal(t, uint32(3), result.Version.Patch)
}

func TestParseImportName_UnlockedDep(t *testing.T) {
	result, err := ParseImportName("unlocked-dep=my-pkg:>=1.0.0")
	require.NoError(t, err)
	require.Equal(t, ImportNameKindUnlockedDep, result.Kind)
	require.Equal(t, "my-pkg", result.DepName)
	// Version range will be implemented in Task 2, for now just check kind and dep name
	require.Equal(t, ">=1.0.0", result.VersionRangeStr)
}

func TestParseImportName_Plain(t *testing.T) {
	result, err := ParseImportName("simple-name")
	require.NoError(t, err)
	require.Equal(t, ImportNameKindPlain, result.Kind)
	require.Equal(t, "simple-name", result.Name)
}

func TestParseImportName_URL(t *testing.T) {
	result, err := ParseImportName("url=https://example.com")
	require.NoError(t, err)
	require.Equal(t, ImportNameKindURL, result.Kind)
	require.Equal(t, "https://example.com", result.URL)
}

func TestParseImportName_Hash(t *testing.T) {
	result, err := ParseImportName("integrity=sha256:abc123")
	require.NoError(t, err)
	require.Equal(t, ImportNameKindHash, result.Kind)
	require.Equal(t, "sha256:abc123", result.Hash)
}
