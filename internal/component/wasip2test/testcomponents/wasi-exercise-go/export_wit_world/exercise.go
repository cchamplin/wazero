// Package export_wit_world implements the WASI exercise world's exported functions.
// The generated wit_exports.go (in package main) dispatches to these functions.
package export_wit_world

import (
	"wit_component/wasi_filesystem_preopens"
	"wit_component/wasi_filesystem_types"
)

// TestFsSetSize implements the "test-fs-set-size" export.
// WIT: export test-fs-set-size: func() -> string;
func TestFsSetSize() string {
	dirs := wasi_filesystem_preopens.GetDirectories()
	if len(dirs) == 0 {
		return "no preopened directories"
	}
	dir := dirs[0].F0

	openResult := dir.OpenAt(
		wasi_filesystem_types.PathFlagsSymlinkFollow,
		"test-set-size.txt",
		wasi_filesystem_types.OpenFlagsCreate|wasi_filesystem_types.OpenFlagsTruncate,
		wasi_filesystem_types.DescriptorFlagsRead|wasi_filesystem_types.DescriptorFlagsWrite,
	)
	if openResult.IsErr() {
		return "open_at failed"
	}
	file := openResult.Ok()

	buf := make([]byte, 20)
	for i := range buf {
		buf[i] = 'B'
	}
	writeResult := file.Write(buf, 0)
	if writeResult.IsErr() {
		return "write failed"
	}
	if writeResult.Ok() != 20 {
		return "write returned wrong count"
	}

	if setSizeResult := file.SetSize(5); setSizeResult.IsErr() {
		return "set_size failed"
	}

	statResult := file.Stat()
	if statResult.IsErr() {
		return "stat failed"
	}
	stat := statResult.Ok()
	if stat.Size != 5 {
		return "expected size 5"
	}
	return "ok"
}

// TestFsMetadataHash implements the "test-fs-metadata-hash" export.
// WIT: export test-fs-metadata-hash: func() -> string;
func TestFsMetadataHash() string {
	dirs := wasi_filesystem_preopens.GetDirectories()
	if len(dirs) == 0 {
		return "no preopened directories"
	}
	dir := dirs[0].F0

	openResult := dir.OpenAt(
		wasi_filesystem_types.PathFlagsSymlinkFollow,
		"test-hash.txt",
		wasi_filesystem_types.OpenFlagsCreate|wasi_filesystem_types.OpenFlagsTruncate,
		wasi_filesystem_types.DescriptorFlagsRead|wasi_filesystem_types.DescriptorFlagsWrite,
	)
	if openResult.IsErr() {
		return "open_at failed"
	}
	file := openResult.Ok()

	hash1Result := file.MetadataHash()
	hash2Result := file.MetadataHash()
	if hash1Result.IsErr() || hash2Result.IsErr() {
		return "metadata_hash failed"
	}
	hash1 := hash1Result.Ok()
	hash2 := hash2Result.Ok()
	if hash1.Lower != hash2.Lower || hash1.Upper != hash2.Upper {
		return "hashes differ"
	}
	if hash1.Lower == 0 && hash1.Upper == 0 {
		return "hash is all zeros"
	}
	return "ok"
}

// TestFsIsSameObject implements the "test-fs-is-same-object" export.
// WIT: export test-fs-is-same-object: func() -> string;
func TestFsIsSameObject() string {
	dirs := wasi_filesystem_preopens.GetDirectories()
	if len(dirs) == 0 {
		return "no preopened directories"
	}
	dir := dirs[0].F0

	createResult := dir.OpenAt(
		wasi_filesystem_types.PathFlagsSymlinkFollow,
		"test-same.txt",
		wasi_filesystem_types.OpenFlagsCreate|wasi_filesystem_types.OpenFlagsTruncate,
		wasi_filesystem_types.DescriptorFlagsRead|wasi_filesystem_types.DescriptorFlagsWrite,
	)
	if createResult.IsErr() {
		return "create failed"
	}

	open1 := dir.OpenAt(
		wasi_filesystem_types.PathFlagsSymlinkFollow,
		"test-same.txt",
		0,
		wasi_filesystem_types.DescriptorFlagsRead,
	)
	if open1.IsErr() {
		return "open 1 failed"
	}
	open2 := dir.OpenAt(
		wasi_filesystem_types.PathFlagsSymlinkFollow,
		"test-same.txt",
		0,
		wasi_filesystem_types.DescriptorFlagsRead,
	)
	if open2.IsErr() {
		return "open 2 failed"
	}

	if !open1.Ok().IsSameObject(open2.Ok()) {
		return "same file not detected as same object"
	}
	return "ok"
}
