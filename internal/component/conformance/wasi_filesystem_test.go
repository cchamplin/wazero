// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: WASI filesystem conformance tests verify that
// wazero's filesystem host module types and Descriptor operations
// behave correctly.
package conformance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tetratelabs/wazero/imports/wasip2/filesystem"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestWASIFilesystem exercises the wasi:filesystem host module API surface.
//
// No counterpart (justified): WASI P2 host module conformance invariant.
func TestWASIFilesystem(t *testing.T) {

	// ------------------------------------------------------------------
	// Case 1: Instantiate registers all wasi:filesystem interfaces.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. filesystem.Instantiate must register the types and
	// preopens interfaces.
	t.Run("InstantiateRegistersInterfaces", func(t *testing.T) {
		linker := component.NewLinker()
		err := filesystem.Instantiate(linker)
		require.NoError(t, err)

		for _, iface := range []string{
			"wasi:filesystem/types@0.2.0",
			"wasi:filesystem/preopens@0.2.0",
		} {
			def, lookupErr := linker.MatchImport(iface)
			require.NoError(t, lookupErr, "interface %s should be registered", iface)
			_, ok := def.(*component.InstanceDef)
			require.True(t, ok, "expected InstanceDef for %s", iface)
		}
	})

	// ------------------------------------------------------------------
	// Case 2: Descriptor wraps an os.File for a regular file.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. A Descriptor created for a regular file must correctly
	// report its type and flags.
	t.Run("DescriptorRegularFile", func(t *testing.T) {
		// Create a temp file
		tmpDir := t.TempDir()
		tmpFile := filepath.Join(tmpDir, "test.txt")
		err := os.WriteFile(tmpFile, []byte("hello"), 0644)
		require.NoError(t, err)

		f, err := os.Open(tmpFile)
		require.NoError(t, err)
		defer f.Close()

		desc := filesystem.NewDescriptor(f, false, tmpFile, filesystem.DescriptorFlagRead)
		require.False(t, desc.IsDir(), "should not be a directory")
		require.Equal(t, tmpFile, desc.Path())
		require.True(t, desc.Flags().HasRead(), "should have read flag")
		require.False(t, desc.Flags().HasWrite(), "should not have write flag")
		require.True(t, desc.File() != nil, "underlying file should not be nil")
	})

	// ------------------------------------------------------------------
	// Case 3: Descriptor wraps an os.File for a directory.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. A Descriptor created for a directory must report
	// IsDir() == true.
	t.Run("DescriptorDirectory", func(t *testing.T) {
		tmpDir := t.TempDir()
		f, err := os.Open(tmpDir)
		require.NoError(t, err)
		defer f.Close()

		desc := filesystem.NewDescriptor(f, true, tmpDir,
			filesystem.DescriptorFlagRead|filesystem.DescriptorFlagMutateDirectory)
		require.True(t, desc.IsDir(), "should be a directory")
	})

	// ------------------------------------------------------------------
	// Case 4: DescriptorType.String returns correct WASI names.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. descriptor-type enum string conversion must match
	// the wasi:filesystem/types WIT definitions.
	t.Run("DescriptorTypeStrings", func(t *testing.T) {
		require.Equal(t, "regular-file", filesystem.DescriptorTypeRegularFile.String())
		require.Equal(t, "directory", filesystem.DescriptorTypeDirectory.String())
		require.Equal(t, "symbolic-link", filesystem.DescriptorTypeSymbolicLink.String())
		require.Equal(t, "unknown", filesystem.DescriptorTypeUnknown.String())
	})

	// ------------------------------------------------------------------
	// Case 5: DirectoryEntryStream iterates and exhausts correctly.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. directory-entry-stream must yield each entry once and
	// then signal exhaustion.
	t.Run("DirectoryEntryStream", func(t *testing.T) {
		entries := []filesystem.DirectoryEntry{
			{Type: filesystem.DescriptorTypeRegularFile, Name: "a.txt"},
			{Type: filesystem.DescriptorTypeDirectory, Name: "subdir"},
		}
		stream := filesystem.NewDirectoryEntryStream(entries)

		// Read first entry
		e1, ok1 := stream.ReadEntry()
		require.True(t, ok1, "should return first entry")
		require.Equal(t, "a.txt", e1.Name)

		// Read second entry
		e2, ok2 := stream.ReadEntry()
		require.True(t, ok2, "should return second entry")
		require.Equal(t, "subdir", e2.Name)

		// Stream exhausted
		_, ok3 := stream.ReadEntry()
		require.False(t, ok3, "should signal exhaustion")
	})

	// ------------------------------------------------------------------
	// Case 6: ErrorCode constants match WASI names.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. error-code enum values must match the WASI spec
	// string identifiers.
	t.Run("ErrorCodeValues", func(t *testing.T) {
		require.Equal(t, filesystem.ErrorCode("access"), filesystem.ErrorCodeAccess)
		require.Equal(t, filesystem.ErrorCode("no-entry"), filesystem.ErrorCodeNoEntry)
		require.Equal(t, filesystem.ErrorCode("is-directory"), filesystem.ErrorCodeIsDirectory)
		require.Equal(t, filesystem.ErrorCode("not-directory"), filesystem.ErrorCodeNotDirectory)
	})
}
