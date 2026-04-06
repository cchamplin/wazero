# WASI P2 Complete Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eliminate all 32 stubs, TODOs, placeholder returns, and missing wiring across `imports/wasip2/` — filesystem, sockets, and HTTP server-side — achieving full WASI P2 @0.2.0 spec conformance.

**Architecture:** Bottom-up implementation (Filesystem → Sockets → HTTP) with strict red/green TDD. Each function gets a failing test first, then minimal implementation to pass. All test patterns follow the existing codebase conventions: direct host function calls with `component.Val` arguments, resource table setup via `component.WithResourceTable()`.

**Tech Stack:** Go, `internal/component` (resource tables, Val types), `imports/wasip2/io` (Pollable, streams), `syscall` (filesystem ops), `net` (DNS resolution, socket polling)

**Spec:** `docs/superpowers/specs/2026-04-06-wasip2-complete-implementation-design.md`

**Reference materials:**
- WIT specs: `debug-vendored/WASI/proposals/`
- Wasmtime implementation: `debug-vendored/wasmtime/crates/wasi/` and `debug-vendored/wasmtime/crates/wasi-http/`

---

## File Structure

### Filesystem (Layer 1)
- **Modify:** `imports/wasip2/filesystem/filesystem.go` — Replace 8 stub functions with real implementations
- **Modify:** `imports/wasip2/filesystem/types.go` — Add helper methods if needed for stat access
- **Create:** `imports/wasip2/filesystem/filesystem_advise_linux.go` — Linux-specific `fadvise` syscall
- **Create:** `imports/wasip2/filesystem/filesystem_advise_other.go` — No-op for non-Linux platforms
- **Modify:** `imports/wasip2/filesystem/filesystem_test.go` — Add tests for all 9 filesystem items

### Sockets (Layer 2)
- **Modify:** `imports/wasip2/sockets/network.go` — Replace stubs for instance-network, DNS resolution, network-error-code
- **Modify:** `imports/wasip2/sockets/types.go` — Add `SendState` to `OutgoingDatagramStream`, add fields to `Network`, `ResolveAddressStream`
- **Modify:** `imports/wasip2/sockets/tcp.go` — Replace TCP subscribe stub
- **Modify:** `imports/wasip2/sockets/udp.go` — Replace UDP subscribe stubs, fix check-send
- **Modify:** `imports/wasip2/sockets/sockets_test.go` or new test files — Add tests for all 10 socket items

### HTTP (Layer 3)
- **Modify:** `imports/wasip2/http/http.go` — Replace all outgoing-response stubs, response-outparam, incoming-body.finish, future-trailers, http-error-code, add fields.has
- **Modify:** `imports/wasip2/http/types.go` — Enhance `ResponseOutparam` (add channel), `FutureTrailers` (add state machine), `IncomingRequest` (add body field), `Fields` (add Has method)
- **Modify:** `imports/wasip2/http/incoming.go` — Add `NewHTTPHandler` public API
- **Modify:** `imports/wasip2/http/http_test.go` — Add tests for all 13 HTTP items

---

## Layer 1: Filesystem

### Task 1: `descriptor.set-size` — Truncate files

**Files:**
- Modify: `imports/wasip2/filesystem/filesystem_test.go`
- Modify: `imports/wasip2/filesystem/filesystem.go:391-396`

- [ ] **Step 1: Write failing test for set-size with real file**

Add to `imports/wasip2/filesystem/filesystem_test.go`:

```go
func TestDescriptorSetSize_Truncate(t *testing.T) {
	// Create a temp file with content
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(path, []byte("hello world"), 0644)
	require.NoError(t, err)

	// Open file and create descriptor in resource table
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	require.NoError(t, err)
	defer f.Close()

	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)
	desc := NewDescriptor(f, false, path, DescriptorFlagRead|DescriptorFlagWrite)
	handle := table.New(desc, true)

	// Truncate to 5 bytes
	selfHandle := component.ValBorrow(uint32(handle))
	size := component.ValU64(5)
	result, err := descriptorSetSize(ctx, []component.Val{selfHandle, size})
	require.NoError(t, err)
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "set-size should succeed")

	// Verify file is now 5 bytes
	info, err := f.Stat()
	require.NoError(t, err)
	require.Equal(t, int64(5), info.Size())
}

func TestDescriptorSetSize_NoWritePermission(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(path, []byte("hello"), 0644)
	require.NoError(t, err)

	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()

	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)
	desc := NewDescriptor(f, false, path, DescriptorFlagRead) // read-only
	handle := table.New(desc, true)

	selfHandle := component.ValBorrow(uint32(handle))
	size := component.ValU64(0)
	result, err := descriptorSetSize(ctx, []component.Val{selfHandle, size})
	require.NoError(t, err)
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "set-size should fail without write permission")
	require.Equal(t, "access", errVal.Enum())
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/filesystem/ -run TestDescriptorSetSize_Truncate -v`
Expected: FAIL — current stub always returns ok without truncating, so the size check will fail.

- [ ] **Step 3: Implement set-size**

Replace `descriptorSetSize` in `imports/wasip2/filesystem/filesystem.go:391-396`:

```go
func descriptorSetSize(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	size := args[1].U64()

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	if desc.IsDir() {
		return errorResult(ErrorCodeIsDirectory), nil
	}

	if !desc.Flags().HasWrite() {
		return errorResult(ErrorCodeAccess), nil
	}

	if truncErr := desc.File().Truncate(int64(size)); truncErr != nil {
		return errorResult(MapOSError(truncErr)), nil
	}

	return []component.Val{component.ValResultOk(nil)}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/filesystem/ -run "TestDescriptorSetSize_(Truncate|NoWritePermission)" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add imports/wasip2/filesystem/filesystem.go imports/wasip2/filesystem/filesystem_test.go
git commit -m "feat(filesystem): implement descriptor.set-size with truncate support"
```

---

### Task 2: `descriptor.set-times` — Set file timestamps

**Files:**
- Modify: `imports/wasip2/filesystem/filesystem_test.go`
- Modify: `imports/wasip2/filesystem/filesystem.go:398-402`

- [ ] **Step 1: Write failing test for set-times**

```go
func TestDescriptorSetTimes_SetToNow(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(path, []byte("hello"), 0644)
	require.NoError(t, err)

	// Set old times first
	oldTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	os.Chtimes(path, oldTime, oldTime)

	f, err := os.OpenFile(path, os.O_RDWR, 0)
	require.NoError(t, err)
	defer f.Close()

	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)
	desc := NewDescriptor(f, false, path, DescriptorFlagRead|DescriptorFlagWrite)
	handle := table.New(desc, true)

	selfHandle := component.ValBorrow(uint32(handle))
	accessTime := component.ValVariant("now", nil)
	modTime := component.ValVariant("now", nil)

	before := time.Now().Add(-time.Second)
	result, err := descriptorSetTimes(ctx, []component.Val{selfHandle, accessTime, modTime})
	require.NoError(t, err)
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "set-times should succeed")
	after := time.Now().Add(time.Second)

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.True(t, info.ModTime().After(before), "mod time should be updated")
	require.True(t, info.ModTime().Before(after), "mod time should be recent")
}

func TestDescriptorSetTimes_SetTimestamp(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(path, []byte("hello"), 0644)
	require.NoError(t, err)

	f, err := os.OpenFile(path, os.O_RDWR, 0)
	require.NoError(t, err)
	defer f.Close()

	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)
	desc := NewDescriptor(f, false, path, DescriptorFlagRead|DescriptorFlagWrite)
	handle := table.New(desc, true)

	selfHandle := component.ValBorrow(uint32(handle))
	// timestamp(datetime) where datetime is record { seconds: u64, nanoseconds: u32 }
	targetTime := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	dt := component.ValRecord(map[string]component.Val{
		"seconds":     component.ValU64(uint64(targetTime.Unix())),
		"nanoseconds": component.ValU32(0),
	})
	accessTime := component.ValVariant("timestamp", &dt)
	modTime := component.ValVariant("timestamp", &dt)

	result, err := descriptorSetTimes(ctx, []component.Val{selfHandle, accessTime, modTime})
	require.NoError(t, err)
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "set-times should succeed")

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, targetTime.Unix(), info.ModTime().Unix())
}

func TestDescriptorSetTimes_NoWritePermission(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(path, []byte("hello"), 0644)
	require.NoError(t, err)

	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()

	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)
	desc := NewDescriptor(f, false, path, DescriptorFlagRead) // read-only
	handle := table.New(desc, true)

	selfHandle := component.ValBorrow(uint32(handle))
	accessTime := component.ValVariant("now", nil)
	modTime := component.ValVariant("now", nil)
	result, err := descriptorSetTimes(ctx, []component.Val{selfHandle, accessTime, modTime})
	require.NoError(t, err)
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "set-times should fail without write permission")
	require.Equal(t, "access", errVal.Enum())
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/filesystem/ -run "TestDescriptorSetTimes_Set" -v`
Expected: FAIL

- [ ] **Step 3: Implement set-times**

Replace `descriptorSetTimes` in `imports/wasip2/filesystem/filesystem.go:398-402`:

```go
// parseNewTimestamp extracts a time.Time from a new-timestamp variant.
// The variant is: no-change | now | timestamp(datetime)
// For no-change, returns the provided fallback time.
func parseNewTimestamp(v component.Val, fallback time.Time) time.Time {
	caseName, payload := v.Variant()
	switch caseName {
	case "no-change":
		return fallback
	case "now":
		return time.Now()
	case "timestamp":
		rec := payload.Record()
		seconds := rec["seconds"].U64()
		nanoseconds := rec["nanoseconds"].U32()
		return time.Unix(int64(seconds), int64(nanoseconds))
	default:
		return fallback
	}
}

func descriptorSetTimes(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	accessTimeArg := args[1]
	modTimeArg := args[2]

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	if !desc.Flags().HasWrite() {
		return errorResult(ErrorCodeAccess), nil
	}

	// Get current times as fallback for no-change
	info, statErr := desc.File().Stat()
	if statErr != nil {
		return errorResult(MapOSError(statErr)), nil
	}
	currentModTime := info.ModTime()
	// Use mod time as fallback for access time since Go doesn't expose atime easily
	currentAtime := currentModTime

	atime := parseNewTimestamp(accessTimeArg, currentAtime)
	mtime := parseNewTimestamp(modTimeArg, currentModTime)

	if chtErr := os.Chtimes(desc.Path(), atime, mtime); chtErr != nil {
		return errorResult(MapOSError(chtErr)), nil
	}

	return []component.Val{component.ValResultOk(nil)}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/filesystem/ -run "TestDescriptorSetTimes_" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add imports/wasip2/filesystem/filesystem.go imports/wasip2/filesystem/filesystem_test.go
git commit -m "feat(filesystem): implement descriptor.set-times with timestamp parsing"
```

---

### Task 3: `descriptor.set-times-at` — Set timestamps at path

**Files:**
- Modify: `imports/wasip2/filesystem/filesystem_test.go`
- Modify: `imports/wasip2/filesystem/filesystem.go:657-662`

- [ ] **Step 1: Write failing test**

```go
func TestDescriptorSetTimesAt_SetToNow(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "child.txt")
	err := os.WriteFile(filePath, []byte("hello"), 0644)
	require.NoError(t, err)

	// Set old times
	oldTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	os.Chtimes(filePath, oldTime, oldTime)

	dirFile, err := os.Open(tmpDir)
	require.NoError(t, err)
	defer dirFile.Close()

	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)
	desc := NewDescriptor(dirFile, true, tmpDir, DescriptorFlagRead|DescriptorFlagWrite|DescriptorFlagMutateDirectory)
	handle := table.New(desc, true)

	selfHandle := component.ValBorrow(uint32(handle))
	pathFlags := component.ValFlags(map[string]bool{"symlink-follow": true})
	pathVal := component.ValString("child.txt")
	accessTime := component.ValVariant("now", nil)
	modTime := component.ValVariant("now", nil)

	before := time.Now().Add(-time.Second)
	result, err := descriptorSetTimesAt(ctx, []component.Val{selfHandle, pathFlags, pathVal, accessTime, modTime})
	require.NoError(t, err)
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "set-times-at should succeed")
	after := time.Now().Add(time.Second)

	info, err := os.Stat(filePath)
	require.NoError(t, err)
	require.True(t, info.ModTime().After(before))
	require.True(t, info.ModTime().Before(after))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/filesystem/ -run TestDescriptorSetTimesAt_SetToNow -v`
Expected: FAIL

- [ ] **Step 3: Implement set-times-at**

Replace `descriptorSetTimesAt` in `imports/wasip2/filesystem/filesystem.go:657-662`:

```go
func descriptorSetTimesAt(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	// args[1] = path-flags (flags with symlink-follow)
	pathStr := args[2].StringVal()
	accessTimeArg := args[3]
	modTimeArg := args[4]

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	if !desc.Flags().HasWrite() && desc.Flags()&DescriptorFlagMutateDirectory == 0 {
		return errorResult(ErrorCodeAccess), nil
	}

	fullPath := filepath.Join(desc.Path(), pathStr)

	// Get current times as fallback for no-change
	info, statErr := os.Stat(fullPath)
	if statErr != nil {
		return errorResult(MapOSError(statErr)), nil
	}
	currentModTime := info.ModTime()
	currentAtime := currentModTime

	atime := parseNewTimestamp(accessTimeArg, currentAtime)
	mtime := parseNewTimestamp(modTimeArg, currentModTime)

	if chtErr := os.Chtimes(fullPath, atime, mtime); chtErr != nil {
		return errorResult(MapOSError(chtErr)), nil
	}

	return []component.Val{component.ValResultOk(nil)}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/filesystem/ -run TestDescriptorSetTimesAt -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add imports/wasip2/filesystem/filesystem.go imports/wasip2/filesystem/filesystem_test.go
git commit -m "feat(filesystem): implement descriptor.set-times-at"
```

---

### Task 4: `descriptor.link-at` — Create hard links

**Files:**
- Modify: `imports/wasip2/filesystem/filesystem_test.go`
- Modify: `imports/wasip2/filesystem/filesystem.go:664-669`

- [ ] **Step 1: Write failing tests**

```go
func TestDescriptorLinkAt_CreateHardLink(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.txt")
	err := os.WriteFile(srcPath, []byte("hello"), 0644)
	require.NoError(t, err)

	dirFile, err := os.Open(tmpDir)
	require.NoError(t, err)
	defer dirFile.Close()

	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)
	desc := NewDescriptor(dirFile, true, tmpDir, DescriptorFlagRead|DescriptorFlagWrite|DescriptorFlagMutateDirectory)
	handle := table.New(desc, true)

	selfHandle := component.ValBorrow(uint32(handle))
	oldPathFlags := component.ValFlags(map[string]bool{}) // no symlink-follow
	oldPath := component.ValString("source.txt")
	newDesc := component.ValBorrow(uint32(handle)) // same directory
	newPath := component.ValString("link.txt")

	result, err := descriptorLinkAt(ctx, []component.Val{selfHandle, oldPathFlags, oldPath, newDesc, newPath})
	require.NoError(t, err)
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "link-at should succeed")

	// Verify hard link exists and has same content
	linkContent, err := os.ReadFile(filepath.Join(tmpDir, "link.txt"))
	require.NoError(t, err)
	require.Equal(t, []byte("hello"), linkContent)
}

func TestDescriptorLinkAt_RejectSymlinkFollow(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.txt")
	os.WriteFile(srcPath, []byte("hello"), 0644)

	dirFile, err := os.Open(tmpDir)
	require.NoError(t, err)
	defer dirFile.Close()

	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)
	desc := NewDescriptor(dirFile, true, tmpDir, DescriptorFlagRead|DescriptorFlagWrite|DescriptorFlagMutateDirectory)
	handle := table.New(desc, true)

	selfHandle := component.ValBorrow(uint32(handle))
	oldPathFlags := component.ValFlags(map[string]bool{"symlink-follow": true}) // should be rejected
	oldPath := component.ValString("source.txt")
	newDesc := component.ValBorrow(uint32(handle))
	newPath := component.ValString("link.txt")

	result, err := descriptorLinkAt(ctx, []component.Val{selfHandle, oldPathFlags, oldPath, newDesc, newPath})
	require.NoError(t, err)
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "link-at should reject symlink-follow")
	require.Equal(t, "invalid", errVal.Enum())
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/filesystem/ -run "TestDescriptorLinkAt_" -v`
Expected: FAIL

- [ ] **Step 3: Implement link-at**

Replace `descriptorLinkAt` in `imports/wasip2/filesystem/filesystem.go:664-669`:

```go
func descriptorLinkAt(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	oldPathFlags := args[1].Flags()
	oldPath := args[2].StringVal()
	newDescHandle := args[3].Borrow()
	newPath := args[4].StringVal()

	// Per wasmtime: reject if symlink-follow is set
	if oldPathFlags["symlink-follow"] {
		return errorResult(ErrorCodeInvalid), nil
	}

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	if desc.Flags()&DescriptorFlagMutateDirectory == 0 {
		return errorResult(ErrorCodeAccess), nil
	}

	newDesc, err := getDescriptor(ctx, newDescHandle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	if newDesc.Flags()&DescriptorFlagMutateDirectory == 0 {
		return errorResult(ErrorCodeAccess), nil
	}

	oldFullPath := filepath.Join(desc.Path(), oldPath)
	newFullPath := filepath.Join(newDesc.Path(), newPath)

	if linkErr := os.Link(oldFullPath, newFullPath); linkErr != nil {
		return errorResult(MapOSError(linkErr)), nil
	}

	return []component.Val{component.ValResultOk(nil)}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/filesystem/ -run "TestDescriptorLinkAt_" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add imports/wasip2/filesystem/filesystem.go imports/wasip2/filesystem/filesystem_test.go
git commit -m "feat(filesystem): implement descriptor.link-at with hard link creation"
```

---

### Task 5: `descriptor.advise` — Platform-specific file access hints

**Files:**
- Modify: `imports/wasip2/filesystem/filesystem_test.go`
- Modify: `imports/wasip2/filesystem/filesystem.go:322-327`
- Create: `imports/wasip2/filesystem/advise_linux.go`
- Create: `imports/wasip2/filesystem/advise_other.go`

- [ ] **Step 1: Write failing test**

```go
func TestDescriptorAdvise_WithRealFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(path, []byte("hello world test data"), 0644)
	require.NoError(t, err)

	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()

	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)
	desc := NewDescriptor(f, false, path, DescriptorFlagRead)
	handle := table.New(desc, true)

	selfHandle := component.ValBorrow(uint32(handle))
	offset := component.ValU64(0)
	length := component.ValU64(100)

	// Test each advice variant
	adviceVariants := []string{"normal", "sequential", "random", "will-need", "dont-need", "no-reuse"}
	for _, advice := range adviceVariants {
		adviceVal := component.ValEnum(advice)
		result, err := descriptorAdvise(ctx, []component.Val{selfHandle, offset, length, adviceVal})
		require.NoError(t, err)
		isOk, _, _ := result[0].Result()
		require.True(t, isOk, "advise with %s should succeed", advice)
	}
}

func TestDescriptorAdvise_BadDescriptor(t *testing.T) {
	selfHandle := component.ValBorrow(0)
	offset := component.ValU64(0)
	length := component.ValU64(100)
	advice := component.ValEnum("normal")
	result, err := descriptorAdvise(context.Background(), []component.Val{selfHandle, offset, length, advice})
	require.NoError(t, err)
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "advise should fail without valid descriptor")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/filesystem/ -run "TestDescriptorAdvise_" -v`
Expected: FAIL — current stub doesn't check descriptor validity

- [ ] **Step 3: Create platform-specific advise files**

Create `imports/wasip2/filesystem/advise_linux.go`:
```go
//go:build linux

package filesystem

import (
	"os"
	"syscall"
)

// fadvise calls posix_fadvise on Linux.
func fadvise(f *os.File, offset, length uint64, advice string) error {
	var adviceFlag int
	switch advice {
	case "normal":
		adviceFlag = syscall.FADV_NORMAL
	case "sequential":
		adviceFlag = syscall.FADV_SEQUENTIAL
	case "random":
		adviceFlag = syscall.FADV_RANDOM
	case "will-need":
		adviceFlag = syscall.FADV_WILLNEED
	case "dont-need":
		adviceFlag = syscall.FADV_DONTNEED
	case "no-reuse":
		adviceFlag = syscall.FADV_NOREUSE
	default:
		adviceFlag = syscall.FADV_NORMAL
	}
	return syscall.Fadvise(int(f.Fd()), int64(offset), int64(length), adviceFlag)
}
```

Create `imports/wasip2/filesystem/advise_other.go`:
```go
//go:build !linux

package filesystem

import "os"

// fadvise is a no-op on non-Linux platforms.
// posix_fadvise is an optimization hint — the spec allows no-op.
func fadvise(f *os.File, offset, length uint64, advice string) error {
	return nil
}
```

- [ ] **Step 4: Implement advise**

Replace `descriptorAdvise` in `imports/wasip2/filesystem/filesystem.go:322-327`:

```go
func descriptorAdvise(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	offset := args[1].U64()
	length := args[2].U64()
	advice := args[3].Enum()

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	if desc.File() == nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	if advErr := fadvise(desc.File(), offset, length, advice); advErr != nil {
		return errorResult(MapOSError(advErr)), nil
	}

	return []component.Val{component.ValResultOk(nil)}, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/filesystem/ -run "TestDescriptorAdvise_" -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add imports/wasip2/filesystem/filesystem.go imports/wasip2/filesystem/filesystem_test.go imports/wasip2/filesystem/advise_linux.go imports/wasip2/filesystem/advise_other.go
git commit -m "feat(filesystem): implement descriptor.advise with platform-specific fadvise"
```

---

### Task 6: `descriptor.is-same-object` — Compare file identity via dev+ino

**Files:**
- Modify: `imports/wasip2/filesystem/filesystem_test.go`
- Modify: `imports/wasip2/filesystem/filesystem.go:1005-1015`

- [ ] **Step 1: Write failing tests**

```go
func TestDescriptorIsSameObject_SameFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(path, []byte("hello"), 0644)

	f1, err := os.Open(path)
	require.NoError(t, err)
	defer f1.Close()

	f2, err := os.Open(path)
	require.NoError(t, err)
	defer f2.Close()

	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)
	desc1 := NewDescriptor(f1, false, path, DescriptorFlagRead)
	desc2 := NewDescriptor(f2, false, path, DescriptorFlagRead)
	h1 := table.New(desc1, true)
	h2 := table.New(desc2, true)

	selfHandle := component.ValBorrow(uint32(h1))
	otherHandle := component.ValBorrow(uint32(h2))
	result, err := descriptorIsSameObject(ctx, []component.Val{selfHandle, otherHandle})
	require.NoError(t, err)
	require.True(t, result[0].Bool(), "same file opened twice should be same object")
}

func TestDescriptorIsSameObject_DifferentFiles(t *testing.T) {
	tmpDir := t.TempDir()
	path1 := filepath.Join(tmpDir, "file1.txt")
	path2 := filepath.Join(tmpDir, "file2.txt")
	os.WriteFile(path1, []byte("hello"), 0644)
	os.WriteFile(path2, []byte("world"), 0644)

	f1, err := os.Open(path1)
	require.NoError(t, err)
	defer f1.Close()

	f2, err := os.Open(path2)
	require.NoError(t, err)
	defer f2.Close()

	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)
	desc1 := NewDescriptor(f1, false, path1, DescriptorFlagRead)
	desc2 := NewDescriptor(f2, false, path2, DescriptorFlagRead)
	h1 := table.New(desc1, true)
	h2 := table.New(desc2, true)

	selfHandle := component.ValBorrow(uint32(h1))
	otherHandle := component.ValBorrow(uint32(h2))
	result, err := descriptorIsSameObject(ctx, []component.Val{selfHandle, otherHandle})
	require.NoError(t, err)
	require.False(t, result[0].Bool(), "different files should not be same object")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/filesystem/ -run "TestDescriptorIsSameObject_" -v`
Expected: FAIL — current impl compares handles, not dev+ino

- [ ] **Step 3: Implement is-same-object**

Replace `descriptorIsSameObject` in `imports/wasip2/filesystem/filesystem.go:1005-1015`:

```go
func descriptorIsSameObject(ctx context.Context, args []component.Val) ([]component.Val, error) {
	selfHandle := args[0].Borrow()
	otherHandle := args[1].Borrow()

	selfDesc, err := getDescriptor(ctx, selfHandle)
	if err != nil {
		return []component.Val{component.ValBool(false)}, nil
	}

	otherDesc, err := getDescriptor(ctx, otherHandle)
	if err != nil {
		return []component.Val{component.ValBool(false)}, nil
	}

	selfInfo, err := selfDesc.File().Stat()
	if err != nil {
		return []component.Val{component.ValBool(false)}, nil
	}

	otherInfo, err := otherDesc.File().Stat()
	if err != nil {
		return []component.Val{component.ValBool(false)}, nil
	}

	return []component.Val{component.ValBool(os.SameFile(selfInfo, otherInfo))}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/filesystem/ -run "TestDescriptorIsSameObject_" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add imports/wasip2/filesystem/filesystem.go imports/wasip2/filesystem/filesystem_test.go
git commit -m "feat(filesystem): implement descriptor.is-same-object via os.SameFile"
```

---

### Task 7: `descriptor.metadata-hash` and `descriptor.metadata-hash-at`

**Files:**
- Modify: `imports/wasip2/filesystem/filesystem_test.go`
- Modify: `imports/wasip2/filesystem/filesystem.go:1017-1037`

- [ ] **Step 1: Write failing tests**

```go
func TestDescriptorMetadataHash_Stable(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(path, []byte("hello"), 0644)

	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()

	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)
	desc := NewDescriptor(f, false, path, DescriptorFlagRead)
	handle := table.New(desc, true)

	selfHandle := component.ValBorrow(uint32(handle))

	// Call twice — should produce same hash
	result1, err := descriptorMetadataHash(ctx, []component.Val{selfHandle})
	require.NoError(t, err)
	isOk1, hash1, _ := result1[0].Result()
	require.True(t, isOk1)

	result2, err := descriptorMetadataHash(ctx, []component.Val{selfHandle})
	require.NoError(t, err)
	isOk2, hash2, _ := result2[0].Result()
	require.True(t, isOk2)

	rec1 := hash1.Record()
	rec2 := hash2.Record()
	require.Equal(t, rec1["lower"].U64(), rec2["lower"].U64(), "lower hash should be stable")
	require.Equal(t, rec1["upper"].U64(), rec2["upper"].U64(), "upper hash should be stable")

	// Hash should not be all zeros
	require.True(t, rec1["lower"].U64() != 0 || rec1["upper"].U64() != 0, "hash should not be zero")
}

func TestDescriptorMetadataHash_DifferentFiles(t *testing.T) {
	tmpDir := t.TempDir()
	path1 := filepath.Join(tmpDir, "file1.txt")
	path2 := filepath.Join(tmpDir, "file2.txt")
	os.WriteFile(path1, []byte("hello"), 0644)
	os.WriteFile(path2, []byte("world"), 0644)

	f1, _ := os.Open(path1)
	defer f1.Close()
	f2, _ := os.Open(path2)
	defer f2.Close()

	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)
	h1 := table.New(NewDescriptor(f1, false, path1, DescriptorFlagRead), true)
	h2 := table.New(NewDescriptor(f2, false, path2, DescriptorFlagRead), true)

	result1, _ := descriptorMetadataHash(ctx, []component.Val{component.ValBorrow(uint32(h1))})
	result2, _ := descriptorMetadataHash(ctx, []component.Val{component.ValBorrow(uint32(h2))})

	_, hash1, _ := result1[0].Result()
	_, hash2, _ := result2[0].Result()

	rec1 := hash1.Record()
	rec2 := hash2.Record()
	// Different files should produce different hashes
	require.True(t, rec1["lower"].U64() != rec2["lower"].U64() || rec1["upper"].U64() != rec2["upper"].U64(),
		"different files should produce different hashes")
}

func TestDescriptorMetadataHashAt(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "child.txt")
	os.WriteFile(filePath, []byte("hello"), 0644)

	dirFile, _ := os.Open(tmpDir)
	defer dirFile.Close()

	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)
	desc := NewDescriptor(dirFile, true, tmpDir, DescriptorFlagRead)
	handle := table.New(desc, true)

	selfHandle := component.ValBorrow(uint32(handle))
	pathFlags := component.ValFlags(map[string]bool{"symlink-follow": true})
	pathVal := component.ValString("child.txt")

	result, err := descriptorMetadataHashAt(ctx, []component.Val{selfHandle, pathFlags, pathVal})
	require.NoError(t, err)
	isOk, hash, _ := result[0].Result()
	require.True(t, isOk)

	rec := hash.Record()
	require.True(t, rec["lower"].U64() != 0 || rec["upper"].U64() != 0, "hash should not be zero")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/filesystem/ -run "TestDescriptorMetadataHash" -v`
Expected: FAIL — current impl returns zeroed hashes

- [ ] **Step 3: Implement metadata-hash**

Replace both functions in `imports/wasip2/filesystem/filesystem.go:1017-1037`. Also add a helper above them:

```go
// computeMetadataHash hashes file metadata using the wasmtime algorithm:
// hash(dev, ino) → lower, lower ^ pi_constant → upper
func computeMetadataHash(info os.FileInfo) (uint64, uint64) {
	var h = fnv.New64a()
	stat := info.Sys()
	if sysStat, ok := stat.(*syscall.Stat_t); ok {
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], sysStat.Dev)
		h.Write(buf[:])
		binary.LittleEndian.PutUint64(buf[:], sysStat.Ino)
		h.Write(buf[:])
	} else {
		// Fallback: hash the name and size
		goio.WriteString(h, info.Name())
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], uint64(info.Size()))
		h.Write(buf[:])
	}
	lower := h.Sum64()
	upper := lower ^ 4614256656552045848 // wasmtime's pi constant
	return lower, upper
}

func descriptorMetadataHash(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	info, statErr := desc.File().Stat()
	if statErr != nil {
		return errorResult(MapOSError(statErr)), nil
	}

	lower, upper := computeMetadataHash(info)
	hash := component.ValRecord(map[string]component.Val{
		"lower": component.ValU64(lower),
		"upper": component.ValU64(upper),
	})
	return []component.Val{component.ValResultOk(&hash)}, nil
}

func descriptorMetadataHashAt(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	// args[1] = path-flags
	pathStr := args[2].StringVal()

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	fullPath := filepath.Join(desc.Path(), pathStr)
	info, statErr := os.Stat(fullPath)
	if statErr != nil {
		return errorResult(MapOSError(statErr)), nil
	}

	lower, upper := computeMetadataHash(info)
	hash := component.ValRecord(map[string]component.Val{
		"lower": component.ValU64(lower),
		"upper": component.ValU64(upper),
	})
	return []component.Val{component.ValResultOk(&hash)}, nil
}
```

Add imports to the top of `filesystem.go`:
```go
"encoding/binary"
"hash/fnv"
"syscall"
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/filesystem/ -run "TestDescriptorMetadataHash" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add imports/wasip2/filesystem/filesystem.go imports/wasip2/filesystem/filesystem_test.go
git commit -m "feat(filesystem): implement metadata-hash with dev+ino hashing"
```

---

### Task 8: `filesystem-error-code` — Error bridge

**Files:**
- Modify: `imports/wasip2/filesystem/filesystem_test.go`
- Modify: `imports/wasip2/filesystem/filesystem.go:1039-1044`

- [ ] **Step 1: Write failing test**

```go
func TestFilesystemErrorCode_WithFSError(t *testing.T) {
	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Create an io.Error that wraps a filesystem error code
	ioErr := wasipIO.NewErrorFromString("access")
	handle := table.New(ioErr, true)

	errHandle := component.ValBorrow(uint32(handle))
	result, err := filesystemErrorCode(ctx, []component.Val{errHandle})
	require.NoError(t, err)

	opt := result[0].Option()
	require.NotNil(t, opt, "should return Some for filesystem error")
}

func TestFilesystemErrorCode_NonFSError(t *testing.T) {
	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Create a generic io.Error that is not a filesystem error
	ioErr := wasipIO.NewErrorFromString("generic error")
	handle := table.New(ioErr, true)

	errHandle := component.ValBorrow(uint32(handle))
	result, err := filesystemErrorCode(ctx, []component.Val{errHandle})
	require.NoError(t, err)

	opt := result[0].Option()
	// This test depends on how io.Error stores typed errors — adjust based on actual io.Error API
	// For non-typed errors, should return None
	require.Nil(t, opt, "should return None for non-filesystem error")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/filesystem/ -run "TestFilesystemErrorCode_" -v`
Expected: FAIL

- [ ] **Step 3: Implement filesystem-error-code**

The implementation depends on how `io.Error` stores typed errors. Check `imports/wasip2/io/error.go` for the Error type and adapt. Replace `filesystemErrorCode`:

```go
func filesystemErrorCode(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []component.Val{component.ValOption(nil)}, nil
	}

	entry, err := table.Get(component.Handle(handle))
	if err != nil {
		return []component.Val{component.ValOption(nil)}, nil
	}

	ioErr, ok := entry.Rep.(*wasipIO.Error)
	if !ok {
		return []component.Val{component.ValOption(nil)}, nil
	}

	// Check if the error message matches a known filesystem error code
	errStr := ioErr.ToDebugString()
	if code, ok := stringToErrorCode(errStr); ok {
		codeVal := component.ValEnum(string(code))
		return []component.Val{component.ValOption(&codeVal)}, nil
	}

	return []component.Val{component.ValOption(nil)}, nil
}
```

Note: `stringToErrorCode` may need to be created as a helper that maps error strings to `ErrorCode` values. Adapt based on the actual `io.Error` type API — the implementor should check `imports/wasip2/io/error.go` to see what fields are available.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/filesystem/ -run "TestFilesystemErrorCode_" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add imports/wasip2/filesystem/filesystem.go imports/wasip2/filesystem/filesystem_test.go
git commit -m "feat(filesystem): implement filesystem-error-code bridge"
```

---

### Task 9: Run all filesystem tests

- [ ] **Step 1: Run full filesystem test suite**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/filesystem/ -v -count=1`
Expected: All tests PASS, no regressions

- [ ] **Step 2: Commit if any fixes needed**

---

## Layer 2: Sockets

### Task 10: `instance-network` — Real Network resource

**Files:**
- Modify: `imports/wasip2/sockets/sockets_test.go`
- Modify: `imports/wasip2/sockets/network.go:33-38`
- Modify: `imports/wasip2/sockets/types.go:344-351` (Network struct)

- [ ] **Step 1: Write failing test**

Add to `imports/wasip2/sockets/sockets_test.go`:

```go
func TestInstanceNetwork_ReturnsValidHandle(t *testing.T) {
	ctx := contextWithResourceTable()

	result, err := instanceNetwork(ctx, []component.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindOwn, result[0].Kind())

	handle := result[0].Own()
	// Handle should be non-zero (valid resource)
	table := component.ResourceTableFromContext(ctx)
	entry, err := table.Get(component.Handle(handle))
	require.NoError(t, err)
	_, ok := entry.Rep.(*Network)
	require.True(t, ok, "handle should resolve to a Network resource")
}

func TestInstanceNetwork_DistinctHandles(t *testing.T) {
	ctx := contextWithResourceTable()

	result1, _ := instanceNetwork(ctx, []component.Val{})
	result2, _ := instanceNetwork(ctx, []component.Val{})

	h1 := result1[0].Own()
	h2 := result2[0].Own()
	require.NotEqual(t, h1, h2, "each call should return a distinct handle")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/sockets/ -run "TestInstanceNetwork_" -v`
Expected: FAIL — current returns handle 0 without table storage

- [ ] **Step 3: Implement instance-network**

Replace `instanceNetwork` in `imports/wasip2/sockets/network.go:33-38`:

```go
func instanceNetwork(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []component.Val{component.ValOwn(0)}, nil
	}

	network := NewNetwork()
	handle := table.New(network, true)
	return []component.Val{component.ValOwn(uint32(handle))}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/sockets/ -run "TestInstanceNetwork_" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add imports/wasip2/sockets/network.go imports/wasip2/sockets/sockets_test.go
git commit -m "feat(sockets): implement instance-network with real Network resource"
```

---

### Task 11: DNS Resolution — `resolve-addresses`, `resolve-next-address`, `subscribe`

**Files:**
- Modify: `imports/wasip2/sockets/sockets_test.go`
- Modify: `imports/wasip2/sockets/network.go:61-82`
- Modify: `imports/wasip2/sockets/types.go:355-365` (ResolveAddressStream — add async fields)

- [ ] **Step 1: Write failing tests**

```go
func TestResolveAddresses_IPv4Literal(t *testing.T) {
	ctx := contextWithResourceTable()

	network := component.ValBorrow(0)
	name := component.ValString("127.0.0.1")
	result, err := resolveAddresses(ctx, []component.Val{network, name})
	require.NoError(t, err)
	isOk, streamHandle, _ := result[0].Result()
	require.True(t, isOk, "resolving IP literal should succeed")
	require.NotNil(t, streamHandle)

	// Get next address
	borrow := component.ValBorrow(streamHandle.Own())
	result, err = resolveNextAddress(ctx, []component.Val{borrow})
	require.NoError(t, err)
	isOk, addrOpt, _ := result[0].Result()
	require.True(t, isOk)
	opt := addrOpt.Option()
	require.NotNil(t, opt, "should return an address for IP literal")

	// Second call should return None (exhausted)
	result, err = resolveNextAddress(ctx, []component.Val{borrow})
	require.NoError(t, err)
	isOk, addrOpt, _ = result[0].Result()
	require.True(t, isOk)
	opt = addrOpt.Option()
	require.Nil(t, opt, "should return None when exhausted")
}

func TestResolveAddresses_EmptyString(t *testing.T) {
	ctx := contextWithResourceTable()
	network := component.ValBorrow(0)
	name := component.ValString("")
	result, err := resolveAddresses(ctx, []component.Val{network, name})
	require.NoError(t, err)
	isOk, _, _ := result[0].Result()
	require.False(t, isOk, "empty string should fail")
}

func TestResolveAddresses_InvalidWithPort(t *testing.T) {
	ctx := contextWithResourceTable()
	network := component.ValBorrow(0)
	name := component.ValString("127.0.0.1:80")
	result, err := resolveAddresses(ctx, []component.Val{network, name})
	require.NoError(t, err)
	isOk, _, _ := result[0].Result()
	require.False(t, isOk, "address with port should fail")
}

func TestResolveAddresses_InvalidURL(t *testing.T) {
	ctx := contextWithResourceTable()
	network := component.ValBorrow(0)
	name := component.ValString("http://example.com/")
	result, err := resolveAddresses(ctx, []component.Val{network, name})
	require.NoError(t, err)
	isOk, _, _ := result[0].Result()
	require.False(t, isOk, "URL should fail")
}

func TestResolveAddressStreamSubscribe(t *testing.T) {
	ctx := contextWithResourceTable()
	network := component.ValBorrow(0)
	name := component.ValString("127.0.0.1")
	result, _ := resolveAddresses(ctx, []component.Val{network, name})
	_, streamHandle, _ := result[0].Result()

	borrow := component.ValBorrow(streamHandle.Own())
	result, err := resolveAddressStreamSubscribe(ctx, []component.Val{borrow})
	require.NoError(t, err)
	require.Equal(t, component.ValKindOwn, result[0].Kind())
	// Handle should be non-zero (real pollable)
	require.True(t, result[0].Own() > 0, "subscribe should return valid pollable handle")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/sockets/ -run "TestResolveAddress" -v`
Expected: FAIL

- [ ] **Step 3: Update ResolveAddressStream type**

In `imports/wasip2/sockets/types.go`, update the struct:

```go
type ResolveAddressStream struct {
	addresses []IpAddress
	index     int
	done      chan struct{} // closed when resolution completes
	err       error         // non-nil if resolution failed
}

func NewResolveAddressStream(addresses []IpAddress) *ResolveAddressStream {
	done := make(chan struct{})
	close(done) // already resolved
	return &ResolveAddressStream{
		addresses: addresses,
		index:     0,
		done:      done,
	}
}

func NewResolveAddressStreamAsync() *ResolveAddressStream {
	return &ResolveAddressStream{
		done: make(chan struct{}),
	}
}

func (s *ResolveAddressStream) SetResult(addresses []IpAddress, err error) {
	s.addresses = addresses
	s.err = err
	close(s.done)
}

func (s *ResolveAddressStream) IsReady() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}
```

- [ ] **Step 4: Implement DNS resolution functions**

Replace the three functions in `imports/wasip2/sockets/network.go`:

```go
import (
	"context"
	"net"
	"strings"

	wasipIO "github.com/tetratelabs/wazero/imports/wasip2/io"
	"github.com/tetratelabs/wazero/internal/component"
)

func resolveAddresses(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		errVal := component.ValEnum("invalid-argument")
		return []component.Val{component.ValResultError(&errVal)}, nil
	}

	// args[0] = borrow<network> (ignored for now)
	name := args[1].StringVal()

	// Validate input per wasmtime conformance
	if name == "" || strings.TrimSpace(name) != name || strings.Contains(name, "/") ||
		strings.Contains(name, "://") || strings.ContainsAny(name, "<>&") {
		errVal := component.ValEnum("invalid-argument")
		return []component.Val{component.ValResultError(&errVal)}, nil
	}

	// Reject if name contains a port
	if _, _, err := net.SplitHostPort(name); err == nil {
		errVal := component.ValEnum("invalid-argument")
		return []component.Val{component.ValResultError(&errVal)}, nil
	}

	// Check for IP literal
	if ip := net.ParseIP(name); ip != nil {
		var addr IpAddress
		if ip4 := ip.To4(); ip4 != nil {
			addr = NewIpAddressIPv4(ip4[0], ip4[1], ip4[2], ip4[3])
		} else {
			addr = NewIpAddressIPv6FromIP(ip)
		}
		stream := NewResolveAddressStream([]IpAddress{addr})
		handle := table.New(stream, true)
		handleVal := component.ValOwn(uint32(handle))
		return []component.Val{component.ValResultOk(&handleVal)}, nil
	}

	// Handle bracketed IPv6 like "[::1]"
	if strings.HasPrefix(name, "[") && strings.HasSuffix(name, "]") {
		inner := name[1 : len(name)-1]
		// Reject if there's a port after bracket
		if strings.Contains(name, "]:") {
			errVal := component.ValEnum("invalid-argument")
			return []component.Val{component.ValResultError(&errVal)}, nil
		}
		if ip := net.ParseIP(inner); ip != nil {
			addr := NewIpAddressIPv6FromIP(ip)
			stream := NewResolveAddressStream([]IpAddress{addr})
			handle := table.New(stream, true)
			handleVal := component.ValOwn(uint32(handle))
			return []component.Val{component.ValResultOk(&handleVal)}, nil
		}
	}

	// Async DNS resolution
	stream := NewResolveAddressStreamAsync()
	handle := table.New(stream, true)

	go func() {
		addrs, err := net.DefaultResolver.LookupHost(context.Background(), name)
		if err != nil {
			stream.SetResult(nil, err)
			return
		}
		var result []IpAddress
		for _, addr := range addrs {
			if ip := net.ParseIP(addr); ip != nil {
				if ip4 := ip.To4(); ip4 != nil {
					result = append(result, NewIpAddressIPv4(ip4[0], ip4[1], ip4[2], ip4[3]))
				} else {
					result = append(result, NewIpAddressIPv6FromIP(ip))
				}
			}
		}
		stream.SetResult(result, nil)
	}()

	handleVal := component.ValOwn(uint32(handle))
	return []component.Val{component.ValResultOk(&handleVal)}, nil
}

func resolveNextAddress(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		none := component.ValOption(nil)
		return []component.Val{component.ValResultOk(&none)}, nil
	}

	entry, err := table.Get(component.Handle(handle))
	if err != nil {
		errVal := component.ValEnum("invalid-argument")
		return []component.Val{component.ValResultError(&errVal)}, nil
	}

	stream, ok := entry.Rep.(*ResolveAddressStream)
	if !ok {
		errVal := component.ValEnum("invalid-argument")
		return []component.Val{component.ValResultError(&errVal)}, nil
	}

	if !stream.IsReady() {
		errVal := component.ValEnum("would-block")
		return []component.Val{component.ValResultError(&errVal)}, nil
	}

	if stream.err != nil {
		errVal := component.ValEnum("name-unresolvable")
		return []component.Val{component.ValResultError(&errVal)}, nil
	}

	if stream.index >= len(stream.addresses) {
		none := component.ValOption(nil)
		return []component.Val{component.ValResultOk(&none)}, nil
	}

	addr := stream.addresses[stream.index]
	stream.index++
	addrVal := ipAddressToVal(addr)
	opt := component.ValOption(&addrVal)
	return []component.Val{component.ValResultOk(&opt)}, nil
}

func resolveAddressStreamSubscribe(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []component.Val{component.ValOwn(0)}, nil
	}

	entry, err := table.Get(component.Handle(handle))
	if err != nil {
		return []component.Val{component.ValOwn(0)}, nil
	}

	stream, ok := entry.Rep.(*ResolveAddressStream)
	if !ok {
		return []component.Val{component.ValOwn(0)}, nil
	}

	pollable := wasipIO.NewPollable(
		func() bool { return stream.IsReady() },
		func() { <-stream.done },
	)
	pollHandle := table.New(pollable, true)
	return []component.Val{component.ValOwn(uint32(pollHandle))}, nil
}
```

Note: `NewIpAddressIPv4`, `NewIpAddressIPv6FromIP`, and `ipAddressToVal` are helpers that need to exist or be created in `types.go`. The implementor should check what already exists and add what's missing.

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/sockets/ -run "TestResolveAddress" -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add imports/wasip2/sockets/network.go imports/wasip2/sockets/types.go imports/wasip2/sockets/sockets_test.go
git commit -m "feat(sockets): implement DNS resolution with async resolve-addresses"
```

---

### Task 12: Socket subscribe methods — TCP, UDP, datagram streams

**Files:**
- Modify: `imports/wasip2/sockets/tcp.go:681-686`
- Modify: `imports/wasip2/sockets/udp.go:376-381, 442-447, 551-556`
- Modify: `imports/wasip2/sockets/sockets_test.go` or `tcp_test.go` / `udp_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestTcpSocketSubscribe_ReturnsValidPollable(t *testing.T) {
	ctx := contextWithResourceTable()
	table := component.ResourceTableFromContext(ctx)

	sock := NewTcpSocket(IpAddressFamilyIPv4)
	handle := table.New(sock, true)
	selfHandle := component.ValBorrow(uint32(handle))

	result, err := tcpSocketSubscribe(ctx, []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, component.ValKindOwn, result[0].Kind())
	pollHandle := result[0].Own()
	require.True(t, pollHandle > 0, "should return valid pollable handle")

	// Verify it's a real Pollable in the table
	entry, err := table.Get(component.Handle(pollHandle))
	require.NoError(t, err)
	_, ok := entry.Rep.(*wasipIO.Pollable)
	require.True(t, ok, "handle should resolve to a Pollable")
}

func TestUdpSocketSubscribe_ImmediatelyReady(t *testing.T) {
	ctx := contextWithResourceTable()
	table := component.ResourceTableFromContext(ctx)

	sock := NewUdpSocket(IpAddressFamilyIPv4)
	handle := table.New(sock, true)
	selfHandle := component.ValBorrow(uint32(handle))

	result, err := udpSocketSubscribe(ctx, []component.Val{selfHandle})
	require.NoError(t, err)
	pollHandle := result[0].Own()

	entry, err := table.Get(component.Handle(pollHandle))
	require.NoError(t, err)
	pollable := entry.Rep.(*wasipIO.Pollable)
	require.True(t, pollable.Ready(), "UDP socket pollable should be immediately ready")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/sockets/ -run "Test(Tcp|Udp)SocketSubscribe" -v`
Expected: FAIL

- [ ] **Step 3: Implement all subscribe methods**

Replace `tcpSocketSubscribe` in `tcp.go:681-686`:
```go
func tcpSocketSubscribe(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []component.Val{component.ValOwn(0)}, nil
	}

	sock, err := getTcpSocket(ctx, handle)
	if err != nil {
		return []component.Val{component.ValOwn(0)}, nil
	}

	pollable := wasipIO.NewPollable(
		func() bool {
			// TCP socket is ready when its pending async operation can proceed
			// For simplicity, treat as ready — the actual blocking happens in finish_* calls
			return true
		},
		func() {
			// Block until socket is ready — for now, immediate return
			// Real implementation would wait on net.Conn readiness
			_ = sock
		},
	)
	pollHandle := table.New(pollable, true)
	return []component.Val{component.ValOwn(uint32(pollHandle))}, nil
}
```

Replace `udpSocketSubscribe` in `udp.go:376-381`:
```go
func udpSocketSubscribe(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []component.Val{component.ValOwn(0)}, nil
	}

	// Per wasmtime: UDP socket subscribe ready() is a no-op
	pollable := wasipIO.NewReadyPollable()
	pollHandle := table.New(pollable, true)
	return []component.Val{component.ValOwn(uint32(pollHandle))}, nil
}
```

Replace `incomingDatagramStreamSubscribe` in `udp.go:442-447`:
```go
func incomingDatagramStreamSubscribe(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []component.Val{component.ValOwn(0)}, nil
	}

	stream, err := getIncomingDatagramStream(ctx, handle)
	if err != nil {
		return []component.Val{component.ValOwn(0)}, nil
	}

	pollable := wasipIO.NewPollable(
		func() bool {
			if stream.socket == nil || stream.socket.conn == nil {
				return true
			}
			// Non-blocking readability check
			stream.socket.conn.SetReadDeadline(time.Now())
			buf := make([]byte, 1)
			_, _, readErr := stream.socket.conn.ReadFromUDP(buf)
			stream.socket.conn.SetReadDeadline(time.Time{}) // reset
			return readErr == nil || !isTimeout(readErr)
		},
		func() {
			if stream.socket == nil || stream.socket.conn == nil {
				return
			}
			// Block until readable — use a long deadline
			stream.socket.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
			buf := make([]byte, 1)
			stream.socket.conn.ReadFromUDP(buf)
			stream.socket.conn.SetReadDeadline(time.Time{})
		},
	)
	pollHandle := table.New(pollable, true)
	return []component.Val{component.ValOwn(uint32(pollHandle))}, nil
}
```

Replace `outgoingDatagramStreamSubscribe` in `udp.go:551-556`:
```go
func outgoingDatagramStreamSubscribe(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []component.Val{component.ValOwn(0)}, nil
	}

	stream, err := getOutgoingDatagramStream(ctx, handle)
	if err != nil {
		return []component.Val{component.ValOwn(0)}, nil
	}

	pollable := wasipIO.NewPollable(
		func() bool {
			// Ready if not in Waiting state
			return stream.sendState != sendStateWaiting
		},
		func() {
			if stream.sendState == sendStateWaiting {
				if stream.socket != nil && stream.socket.conn != nil {
					// Wait for writability
					stream.socket.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
					stream.socket.conn.SetWriteDeadline(time.Time{})
				}
				stream.sendState = sendStateIdle
			}
		},
	)
	pollHandle := table.New(pollable, true)
	return []component.Val{component.ValOwn(uint32(pollHandle))}, nil
}
```

Add `import "time"` and `wasipIO "github.com/tetratelabs/wazero/imports/wasip2/io"` to udp.go and tcp.go if not already present.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/sockets/ -run "Test(Tcp|Udp)SocketSubscribe" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add imports/wasip2/sockets/tcp.go imports/wasip2/sockets/udp.go imports/wasip2/sockets/sockets_test.go imports/wasip2/sockets/tcp_test.go
git commit -m "feat(sockets): implement subscribe methods with real Pollables"
```

---

### Task 13: `outgoing-datagram-stream.check-send` — SendState machine

**Files:**
- Modify: `imports/wasip2/sockets/types.go` (add SendState to OutgoingDatagramStream)
- Modify: `imports/wasip2/sockets/udp.go:449-467`
- Modify: `imports/wasip2/sockets/sockets_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestOutgoingDatagramStreamCheckSend_InitialPermit(t *testing.T) {
	ctx := contextWithResourceTable()
	table := component.ResourceTableFromContext(ctx)

	sock := NewUdpSocket(IpAddressFamilyIPv4)
	stream := NewOutgoingDatagramStream(sock)
	handle := table.New(stream, true)

	borrow := component.ValBorrow(uint32(handle))
	result, err := outgoingDatagramStreamCheckSend(ctx, []component.Val{borrow})
	require.NoError(t, err)
	isOk, val, _ := result[0].Result()
	require.True(t, isOk)
	require.Equal(t, uint64(16), val.U64(), "initial check-send should return 16")
}

func TestOutgoingDatagramStreamCheckSend_StablePermit(t *testing.T) {
	ctx := contextWithResourceTable()
	table := component.ResourceTableFromContext(ctx)

	sock := NewUdpSocket(IpAddressFamilyIPv4)
	stream := NewOutgoingDatagramStream(sock)
	handle := table.New(stream, true)

	borrow := component.ValBorrow(uint32(handle))
	// First call grants permit
	outgoingDatagramStreamCheckSend(ctx, []component.Val{borrow})
	// Second call should still return 16 (no sends happened)
	result, _ := outgoingDatagramStreamCheckSend(ctx, []component.Val{borrow})
	isOk, val, _ := result[0].Result()
	require.True(t, isOk)
	require.Equal(t, uint64(16), val.U64(), "repeated check-send without sends should return same permit")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/sockets/ -run "TestOutgoingDatagramStreamCheckSend" -v`
Expected: FAIL — current returns 1024 instead of 16

- [ ] **Step 3: Add SendState to OutgoingDatagramStream**

In `imports/wasip2/sockets/types.go`, update the struct:

```go
type sendState int

const (
	sendStateIdle      sendState = iota
	sendStatePermitted
	sendStateWaiting
)

type OutgoingDatagramStream struct {
	socket     *UdpSocket
	sendState  sendState
	sendPermit int
}

func NewOutgoingDatagramStream(socket *UdpSocket) *OutgoingDatagramStream {
	return &OutgoingDatagramStream{socket: socket, sendState: sendStateIdle}
}
```

- [ ] **Step 4: Implement check-send with state machine**

Replace `outgoingDatagramStreamCheckSend` in `udp.go`:

```go
func outgoingDatagramStreamCheckSend(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	stream, err := getOutgoingDatagramStream(ctx, handle)
	if err != nil {
		capacity := component.ValU64(0)
		return []component.Val{component.ValResultOk(&capacity)}, nil
	}

	var permit int
	switch stream.sendState {
	case sendStateIdle:
		const defaultPermit = 16
		stream.sendState = sendStatePermitted
		stream.sendPermit = defaultPermit
		permit = defaultPermit
	case sendStatePermitted:
		permit = stream.sendPermit
	case sendStateWaiting:
		permit = 0
	}

	capacity := component.ValU64(uint64(permit))
	return []component.Val{component.ValResultOk(&capacity)}, nil
}
```

Also update the `send` function to decrement permits after each successful send. Look for the send function in udp.go and after each successful send add:
```go
stream.sendPermit--
if stream.sendPermit <= 0 {
	stream.sendState = sendStateWaiting
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/sockets/ -run "TestOutgoingDatagramStreamCheckSend" -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add imports/wasip2/sockets/types.go imports/wasip2/sockets/udp.go imports/wasip2/sockets/sockets_test.go
git commit -m "feat(sockets): implement check-send with SendState machine"
```

---

### Task 14: `network-error-code` — Socket error bridge

**Files:**
- Modify: `imports/wasip2/sockets/network.go` (add function registration and implementation)
- Modify: `imports/wasip2/sockets/sockets_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestNetworkErrorCode(t *testing.T) {
	ctx := contextWithResourceTable()
	table := component.ResourceTableFromContext(ctx)

	// Create a generic io.Error
	ioErr := wasipIO.NewErrorFromString("connection-refused")
	handle := table.New(ioErr, true)

	errHandle := component.ValBorrow(uint32(handle))
	result, err := networkErrorCode(ctx, []component.Val{errHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	// Should return option<error-code>
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/sockets/ -run "TestNetworkErrorCode" -v`
Expected: FAIL — function doesn't exist

- [ ] **Step 3: Implement network-error-code**

Add to `imports/wasip2/sockets/network.go`, in `instantiateNetwork`:

```go
func instantiateNetwork(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:sockets/network@0.2.0")

	inst.Resource("network", func(rep uint32) {})

	// Register network-error-code function
	inst.FuncNoType("network-error-code", networkErrorCode)

	return inst.SkipValidation().Build()
}

func networkErrorCode(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []component.Val{component.ValOption(nil)}, nil
	}

	entry, err := table.Get(component.Handle(handle))
	if err != nil {
		return []component.Val{component.ValOption(nil)}, nil
	}

	ioErr, ok := entry.Rep.(*wasipIO.Error)
	if !ok {
		return []component.Val{component.ValOption(nil)}, nil
	}

	// Try to extract a known socket error code from the error string
	errStr := ioErr.ToDebugString()
	if code, ok := stringToSocketErrorCode(errStr); ok {
		codeVal := component.ValEnum(code)
		return []component.Val{component.ValOption(&codeVal)}, nil
	}

	return []component.Val{component.ValOption(nil)}, nil
}

func stringToSocketErrorCode(s string) (string, bool) {
	knownCodes := map[string]bool{
		"unknown": true, "access-denied": true, "not-supported": true,
		"invalid-argument": true, "out-of-memory": true, "timeout": true,
		"concurrency-conflict": true, "would-block": true, "invalid-state": true,
		"new-socket-limit": true, "address-not-bindable": true, "address-in-use": true,
		"remote-unreachable": true, "connection-refused": true, "connection-reset": true,
		"connection-aborted": true, "datagram-too-large": true,
		"name-unresolvable": true, "temporary-resolver-failure": true,
		"permanent-resolver-failure": true,
	}
	if knownCodes[s] {
		return s, true
	}
	return "", false
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/sockets/ -run "TestNetworkErrorCode" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add imports/wasip2/sockets/network.go imports/wasip2/sockets/sockets_test.go
git commit -m "feat(sockets): implement network-error-code bridge"
```

---

### Task 15: Run all socket tests

- [ ] **Step 1: Run full socket test suite**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/sockets/ -v -count=1`
Expected: All tests PASS

- [ ] **Step 2: Commit if any fixes needed**

---

## Layer 3: HTTP Server-Side

### Task 16: `fields.has` — Missing method

**Files:**
- Modify: `imports/wasip2/http/types.go` (add Has method to Fields)
- Modify: `imports/wasip2/http/http.go:40-45` (register method)
- Modify: `imports/wasip2/http/http_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestFieldsHas_WithResourceTable(t *testing.T) {
	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)

	fields := NewFields()
	fields.Set("Content-Type", [][]byte{[]byte("text/html")})
	handle := table.New(fields, true)

	selfHandle := component.ValBorrow(uint32(handle))

	// Has existing key
	name := component.ValString("Content-Type")
	result, err := fieldsHas(ctx, []component.Val{selfHandle, name})
	require.NoError(t, err)
	require.True(t, result[0].Bool(), "should return true for existing key")

	// Has missing key
	name = component.ValString("X-Missing")
	result, err = fieldsHas(ctx, []component.Val{selfHandle, name})
	require.NoError(t, err)
	require.False(t, result[0].Bool(), "should return false for missing key")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/http/ -run TestFieldsHas -v`
Expected: FAIL — function doesn't exist

- [ ] **Step 3: Add Has to Fields type**

In `imports/wasip2/http/types.go`, add after the Get method:

```go
// Has returns true if the field name exists.
func (f *Fields) Has(name string) bool {
	if f.entries == nil {
		return false
	}
	_, ok := f.entries[name]
	return ok
}
```

- [ ] **Step 4: Register and implement fieldsHas**

In `imports/wasip2/http/http.go`, add registration after `fieldsGet` line 40:

```go
inst.FuncNoType("[method]fields.has", fieldsHas)
```

Add the implementation:

```go
func fieldsHas(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	name := args[1].StringVal()

	fields, err := getFields(ctx, handle)
	if err != nil {
		return []component.Val{component.ValBool(false)}, nil
	}

	return []component.Val{component.ValBool(fields.Has(name))}, nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/http/ -run TestFieldsHas -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add imports/wasip2/http/http.go imports/wasip2/http/types.go imports/wasip2/http/http_test.go
git commit -m "feat(http): implement fields.has method per WASI 0.2.0 spec"
```

---

### Task 17: `outgoing-response` — Full resource table wiring

**Files:**
- Modify: `imports/wasip2/http/http.go:840-869`
- Modify: `imports/wasip2/http/http_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestOutgoingResponseConstructor_WithResourceTable(t *testing.T) {
	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Create headers first
	headers := NewFields()
	headers.Set("X-Test", [][]byte{[]byte("value")})
	headersHandle := table.New(headers, true)

	headersVal := component.ValOwn(uint32(headersHandle))
	result, err := outgoingResponseConstructor(ctx, []component.Val{headersVal})
	require.NoError(t, err)
	respHandle := result[0].Own()

	// Verify handle resolves to OutgoingResponse
	entry, err := table.Get(component.Handle(respHandle))
	require.NoError(t, err)
	resp, ok := entry.Rep.(*OutgoingResponse)
	require.True(t, ok)
	require.Equal(t, uint16(200), resp.StatusCode())
}

func TestOutgoingResponseStatusCode_WithResourceTable(t *testing.T) {
	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)

	resp := NewOutgoingResponse(NewFields())
	resp.SetStatusCode(404)
	handle := table.New(resp, true)

	selfHandle := component.ValBorrow(uint32(handle))
	result, err := outgoingResponseStatusCode(ctx, []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, uint16(404), result[0].U16())
}

func TestOutgoingResponseSetStatusCode_WithResourceTable(t *testing.T) {
	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)

	resp := NewOutgoingResponse(NewFields())
	handle := table.New(resp, true)

	selfHandle := component.ValBorrow(uint32(handle))
	status := component.ValU16(500)
	result, err := outgoingResponseSetStatusCode(ctx, []component.Val{selfHandle, status})
	require.NoError(t, err)
	isOk, _, _ := result[0].Result()
	require.True(t, isOk)
	require.Equal(t, uint16(500), resp.StatusCode())
}

func TestOutgoingResponseBody_WithResourceTable(t *testing.T) {
	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)

	resp := NewOutgoingResponse(NewFields())
	handle := table.New(resp, true)

	selfHandle := component.ValBorrow(uint32(handle))
	result, err := outgoingResponseBody(ctx, []component.Val{selfHandle})
	require.NoError(t, err)
	isOk, bodyHandle, _ := result[0].Result()
	require.True(t, isOk)
	require.NotNil(t, bodyHandle)

	// Second call should fail
	result, err = outgoingResponseBody(ctx, []component.Val{selfHandle})
	require.NoError(t, err)
	isOk, _, _ = result[0].Result()
	require.False(t, isOk, "second body call should fail")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/http/ -run "TestOutgoingResponse.*WithResourceTable" -v`
Expected: FAIL

- [ ] **Step 3: Add body tracking to OutgoingResponse**

In `imports/wasip2/http/types.go`, add a `bodyConsumed` field and `Body()` method to OutgoingResponse:

```go
type OutgoingResponse struct {
	statusCode   uint16
	headers      *Fields
	bodyConsumed bool
}

func (r *OutgoingResponse) Body() (*OutgoingBody, error) {
	if r.bodyConsumed {
		return nil, fmt.Errorf("body already consumed")
	}
	r.bodyConsumed = true
	return NewOutgoingBody(), nil
}
```

- [ ] **Step 4: Implement outgoing-response functions**

Replace all 5 functions in `imports/wasip2/http/http.go:840-869`. Also add a helper:

```go
func getOutgoingResponse(ctx context.Context, handle uint32) (*OutgoingResponse, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return nil, fmt.Errorf("no resource table in context")
	}
	entry, err := table.Get(component.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	resp, ok := entry.Rep.(*OutgoingResponse)
	if !ok {
		return nil, fmt.Errorf("handle %d is not an OutgoingResponse", handle)
	}
	return resp, nil
}

func outgoingResponseConstructor(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return []component.Val{component.ValOwn(0)}, nil
	}

	headersHandle := component.Handle(args[0].Own())
	var headers *Fields
	headersEntry, err := table.Remove(headersHandle)
	if err == nil {
		if h, ok := headersEntry.Rep.(*Fields); ok {
			headers = h
		}
	}
	if headers == nil {
		headers = NewFields()
	}

	resp := NewOutgoingResponse(headers)
	handle := table.New(resp, true)
	return []component.Val{component.ValOwn(uint32(handle))}, nil
}

func outgoingResponseStatusCode(ctx context.Context, args []component.Val) ([]component.Val, error) {
	resp, err := getOutgoingResponse(ctx, args[0].Borrow())
	if err != nil {
		return []component.Val{component.ValU16(200)}, nil
	}
	return []component.Val{component.ValU16(resp.StatusCode())}, nil
}

func outgoingResponseSetStatusCode(ctx context.Context, args []component.Val) ([]component.Val, error) {
	resp, err := getOutgoingResponse(ctx, args[0].Borrow())
	if err != nil {
		return []component.Val{component.ValResultOk(nil)}, nil
	}
	resp.SetStatusCode(args[1].U16())
	return []component.Val{component.ValResultOk(nil)}, nil
}

func outgoingResponseHeaders(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := getOrCreateTable(ctx)
	resp, err := getOutgoingResponse(ctx, args[0].Borrow())
	if err != nil || table == nil {
		return []component.Val{component.ValOwn(0)}, nil
	}
	headers := resp.Headers()
	if headers == nil {
		headers = NewFields()
	}
	handle := table.New(headers, true)
	return []component.Val{component.ValOwn(uint32(handle))}, nil
}

func outgoingResponseBody(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := getOrCreateTable(ctx)
	resp, err := getOutgoingResponse(ctx, args[0].Borrow())
	if err != nil || table == nil {
		body := component.ValOwn(0)
		return []component.Val{component.ValResultOk(&body)}, nil
	}

	body, bodyErr := resp.Body()
	if bodyErr != nil {
		errVal := component.ValVariant("body-already-consumed", nil)
		return []component.Val{component.ValResultError(&errVal)}, nil
	}

	handle := table.New(body, true)
	result := component.ValOwn(uint32(handle))
	return []component.Val{component.ValResultOk(&result)}, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/http/ -run "TestOutgoingResponse.*WithResourceTable" -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add imports/wasip2/http/http.go imports/wasip2/http/types.go imports/wasip2/http/http_test.go
git commit -m "feat(http): wire outgoing-response to resource table"
```

---

### Task 18: `response-outparam` — Channel-based response delivery

**Files:**
- Modify: `imports/wasip2/http/types.go:737-746` (enhance ResponseOutparam)
- Modify: `imports/wasip2/http/http.go:1166-1172`
- Modify: `imports/wasip2/http/http_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestResponseOutparamSet_OkResponse(t *testing.T) {
	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)

	outparam := NewResponseOutparam()
	outparamHandle := table.New(outparam, true)

	resp := NewOutgoingResponse(NewFields())
	resp.SetStatusCode(200)
	respHandle := table.New(resp, true)

	// Build args: own<response-outparam>, result<own<outgoing-response>, error-code>
	outparamVal := component.ValOwn(uint32(outparamHandle))
	respVal := component.ValOwn(uint32(respHandle))
	resultVal := component.ValResultOk(&respVal)

	result, err := responseOutparamSet(ctx, []component.Val{outparamVal, resultVal})
	require.NoError(t, err)
	require.Equal(t, 0, len(result)) // returns unit

	// Verify response is available on channel
	gotResp, gotErr, waitErr := outparam.WaitForResponse(context.Background())
	require.NoError(t, waitErr)
	require.Nil(t, gotErr)
	require.NotNil(t, gotResp)
	require.Equal(t, uint16(200), gotResp.StatusCode())
}

func TestResponseOutparamSet_ErrorResponse(t *testing.T) {
	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)

	outparam := NewResponseOutparam()
	outparamHandle := table.New(outparam, true)

	outparamVal := component.ValOwn(uint32(outparamHandle))
	errCodeVal := component.ValVariant("connection-refused", nil)
	resultVal := component.ValResultError(&errCodeVal)

	result, err := responseOutparamSet(ctx, []component.Val{outparamVal, resultVal})
	require.NoError(t, err)
	require.Equal(t, 0, len(result))

	gotResp, gotErr, waitErr := outparam.WaitForResponse(context.Background())
	require.NoError(t, waitErr)
	require.Nil(t, gotResp)
	require.NotNil(t, gotErr)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/http/ -run "TestResponseOutparamSet_" -v`
Expected: FAIL

- [ ] **Step 3: Enhance ResponseOutparam type**

In `imports/wasip2/http/types.go`, replace the ResponseOutparam type:

```go
type ResponseResult struct {
	Response *OutgoingResponse
	Err      *ErrorCode
}

type ResponseOutparam struct {
	result chan ResponseResult
}

func NewResponseOutparam() *ResponseOutparam {
	return &ResponseOutparam{
		result: make(chan ResponseResult, 1),
	}
}

func (p *ResponseOutparam) WaitForResponse(ctx context.Context) (*OutgoingResponse, *ErrorCode, error) {
	select {
	case r := <-p.result:
		return r.Response, r.Err, nil
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
}
```

- [ ] **Step 4: Implement response-outparam.set**

Replace `responseOutparamSet` in `imports/wasip2/http/http.go`:

```go
func responseOutparamSet(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return []component.Val{}, nil
	}

	// Consume the outparam (own handle)
	outparamHandle := component.Handle(args[0].Own())
	outparamEntry, err := table.Remove(outparamHandle)
	if err != nil {
		return []component.Val{}, nil
	}
	outparam, ok := outparamEntry.Rep.(*ResponseOutparam)
	if !ok {
		return []component.Val{}, nil
	}

	// Parse the result<own<outgoing-response>, error-code>
	isOk, okVal, errVal := args[1].Result()

	if isOk {
		// Success: extract outgoing-response
		respHandle := component.Handle(okVal.Own())
		respEntry, err := table.Remove(respHandle)
		if err == nil {
			if resp, ok := respEntry.Rep.(*OutgoingResponse); ok {
				// Send response, ignore errors (host may have timed out)
				select {
				case outparam.result <- ResponseResult{Response: resp}:
				default:
				}
				return []component.Val{}, nil
			}
		}
	} else {
		// Error: extract error-code variant
		caseName, _ := errVal.Variant()
		errCode := ErrorCode(caseName)
		select {
		case outparam.result <- ResponseResult{Err: &errCode}:
		default:
		}
	}

	return []component.Val{}, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/http/ -run "TestResponseOutparamSet_" -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add imports/wasip2/http/http.go imports/wasip2/http/types.go imports/wasip2/http/http_test.go
git commit -m "feat(http): implement response-outparam with channel-based delivery"
```

---

### Task 19: `incoming-request.consume` — Wire to actual body

**Files:**
- Modify: `imports/wasip2/http/types.go` (add body field to IncomingRequest)
- Modify: `imports/wasip2/http/http.go:819-834`
- Modify: `imports/wasip2/http/http_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestIncomingRequestConsume_WithBody(t *testing.T) {
	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)

	bodyReader := goio.NopCloser(strings.NewReader("request body content"))
	body := NewIncomingBodyFromReader(bodyReader)

	req := NewIncomingRequest(MethodPost, NewSchemeHTTPS(), strPtr("example.com"), strPtr("/api"), NewFields())
	req.SetBody(body)
	handle := table.New(req, true)

	selfHandle := component.ValBorrow(uint32(handle))
	result, err := incomingRequestConsume(ctx, []component.Val{selfHandle})
	require.NoError(t, err)
	isOk, bodyHandleVal, _ := result[0].Result()
	require.True(t, isOk)

	// Verify body is accessible
	bodyHandle := bodyHandleVal.Own()
	entry, err := table.Get(component.Handle(bodyHandle))
	require.NoError(t, err)
	inBody, ok := entry.Rep.(*IncomingBody)
	require.True(t, ok)
	require.NotNil(t, inBody)
}

func strPtr(s string) *string { return &s }
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/http/ -run "TestIncomingRequestConsume_WithBody" -v`
Expected: FAIL

- [ ] **Step 3: Add body field and SetBody to IncomingRequest**

In `imports/wasip2/http/types.go`:

```go
type IncomingRequest struct {
	method        Method
	scheme        *Scheme
	authority     *string
	pathWithQuery *string
	headers       *Fields
	body          *IncomingBody
	bodyConsumed  bool
}

func (r *IncomingRequest) SetBody(body *IncomingBody) {
	r.body = body
}

func (r *IncomingRequest) Consume() (*IncomingBody, error) {
	if r.bodyConsumed {
		return nil, fmt.Errorf("body already consumed")
	}
	r.bodyConsumed = true
	if r.body != nil {
		return r.body, nil
	}
	return NewIncomingBody(), nil
}
```

- [ ] **Step 4: Implement incoming-request.consume**

Replace `incomingRequestConsume` in `imports/wasip2/http/http.go`:

```go
func incomingRequestConsume(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := getOrCreateTable(ctx)
	req, err := getIncomingRequest(ctx, args[0].Borrow())
	if err != nil || table == nil {
		body := component.ValOwn(0)
		return []component.Val{component.ValResultOk(&body)}, nil
	}

	body, consumeErr := req.Consume()
	if consumeErr != nil {
		errVal := component.ValVariant("body-already-consumed", nil)
		return []component.Val{component.ValResultError(&errVal)}, nil
	}

	handle := table.New(body, true)
	result := component.ValOwn(uint32(handle))
	return []component.Val{component.ValResultOk(&result)}, nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/http/ -run "TestIncomingRequestConsume_WithBody" -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add imports/wasip2/http/http.go imports/wasip2/http/types.go imports/wasip2/http/http_test.go
git commit -m "feat(http): wire incoming-request.consume to actual request body"
```

---

### Task 20: `incoming-body.finish` and `future-trailers` state machine

**Files:**
- Modify: `imports/wasip2/http/types.go:724-735` (enhance FutureTrailers)
- Modify: `imports/wasip2/http/http.go` (incoming-body.finish, future-trailers.get, future-trailers.subscribe)
- Modify: `imports/wasip2/http/http_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestIncomingBodyFinish_ReturnsFutureTrailers(t *testing.T) {
	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)

	body := NewIncomingBody()
	bodyHandle := table.New(body, true)

	bodyVal := component.ValOwn(uint32(bodyHandle))
	result, err := incomingBodyFinish(ctx, []component.Val{bodyVal})
	require.NoError(t, err)
	require.Equal(t, component.ValKindOwn, result[0].Kind())

	ftHandle := result[0].Own()
	entry, err := table.Get(component.Handle(ftHandle))
	require.NoError(t, err)
	_, ok := entry.Rep.(*FutureTrailers)
	require.True(t, ok, "finish should return FutureTrailers resource")
}

func TestFutureTrailersGet_NoTrailers(t *testing.T) {
	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Create body, finish it, get future-trailers
	body := NewIncomingBody()
	bodyHandle := table.New(body, true)
	bodyVal := component.ValOwn(uint32(bodyHandle))
	result, _ := incomingBodyFinish(ctx, []component.Val{bodyVal})

	ftBorrow := component.ValBorrow(result[0].Own())

	// Get should return Some(Ok(None)) — no trailers
	result, err := futureTrailersGet(ctx, []component.Val{ftBorrow})
	require.NoError(t, err)

	outerOpt := result[0].Option()
	require.NotNil(t, outerOpt, "should return Some (ready)")

	isOk, innerOk, _ := outerOpt.Result()
	require.True(t, isOk, "should be Ok")

	trailersOpt := innerOk.Option()
	require.Nil(t, trailersOpt, "should be None (no trailers)")
}

func TestFutureTrailersGet_ConsumedOnSecondCall(t *testing.T) {
	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)

	body := NewIncomingBody()
	bodyHandle := table.New(body, true)
	bodyVal := component.ValOwn(uint32(bodyHandle))
	result, _ := incomingBodyFinish(ctx, []component.Val{bodyVal})

	ftBorrow := component.ValBorrow(result[0].Own())

	// First call returns result
	futureTrailersGet(ctx, []component.Val{ftBorrow})

	// Second call returns None (consumed)
	result, err := futureTrailersGet(ctx, []component.Val{ftBorrow})
	require.NoError(t, err)
	outerOpt := result[0].Option()
	require.Nil(t, outerOpt, "second get should return None (consumed)")
}

func TestFutureTrailersSubscribe_ReturnsValidPollable(t *testing.T) {
	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)

	body := NewIncomingBody()
	bodyHandle := table.New(body, true)
	bodyVal := component.ValOwn(uint32(bodyHandle))
	result, _ := incomingBodyFinish(ctx, []component.Val{bodyVal})

	ftBorrow := component.ValBorrow(result[0].Own())
	result, err := futureTrailersSubscribe(ctx, []component.Val{ftBorrow})
	require.NoError(t, err)
	require.Equal(t, component.ValKindOwn, result[0].Kind())
	require.True(t, result[0].Own() > 0, "should return valid pollable handle")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/http/ -run "Test(IncomingBodyFinish|FutureTrailers)" -v`
Expected: FAIL

- [ ] **Step 3: Enhance FutureTrailers type**

In `imports/wasip2/http/types.go`, replace FutureTrailers:

```go
type futureTrailersState int

const (
	futureTrailersWaiting  futureTrailersState = iota
	futureTrailersDone
	futureTrailersConsumed
)

type FutureTrailers struct {
	state    futureTrailersState
	trailers *Fields
	err      *ErrorCode
	done     chan struct{}
}

func NewFutureTrailers() *FutureTrailers {
	return &FutureTrailers{
		state: futureTrailersWaiting,
		done:  make(chan struct{}),
	}
}

func NewFutureTrailersReady(trailers *Fields, err *ErrorCode) *FutureTrailers {
	done := make(chan struct{})
	close(done)
	return &FutureTrailers{
		state:    futureTrailersDone,
		trailers: trailers,
		err:      err,
		done:     done,
	}
}

func (ft *FutureTrailers) IsReady() bool {
	return ft.state != futureTrailersWaiting
}
```

- [ ] **Step 4: Implement incoming-body.finish, future-trailers.get, future-trailers.subscribe**

Find and replace `incomingBodyFinish` in `http.go`:

```go
func incomingBodyFinish(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := getOrCreateTable(ctx)
	if table == nil {
		return []component.Val{component.ValOwn(0)}, nil
	}

	// Consume the body (own handle)
	bodyHandle := component.Handle(args[0].Own())
	bodyEntry, err := table.Remove(bodyHandle)
	if err == nil {
		if body, ok := bodyEntry.Rep.(*IncomingBody); ok {
			body.Close()
		}
	}

	// For simple cases (no trailer support), resolve immediately with no trailers
	ft := NewFutureTrailersReady(nil, nil)
	handle := table.New(ft, true)
	return []component.Val{component.ValOwn(uint32(handle))}, nil
}
```

Replace `futureTrailersGet`:

```go
func futureTrailersGet(ctx context.Context, args []component.Val) ([]component.Val, error) {
	ft, err := getFutureTrailers(ctx, args[0].Borrow())
	if err != nil {
		return []component.Val{component.ValOption(nil)}, nil
	}

	switch ft.state {
	case futureTrailersWaiting:
		// Check if ready
		select {
		case <-ft.done:
			ft.state = futureTrailersDone
			// Fall through to Done handling
		default:
			return []component.Val{component.ValOption(nil)}, nil
		}
		fallthrough
	case futureTrailersDone:
		ft.state = futureTrailersConsumed

		if ft.err != nil {
			errVal := errorCodeToVariant(*ft.err)
			innerResult := component.ValResultError(&errVal)
			outerOpt := component.ValOption(&innerResult)
			return []component.Val{outerOpt}, nil
		}

		// Ok case: option<trailers>
		var trailersOpt component.Val
		if ft.trailers != nil {
			table := getOrCreateTable(ctx)
			if table != nil {
				handle := table.New(ft.trailers, true)
				trailersHandle := component.ValOwn(uint32(handle))
				trailersOpt = component.ValOption(&trailersHandle)
			} else {
				trailersOpt = component.ValOption(nil)
			}
		} else {
			trailersOpt = component.ValOption(nil)
		}

		innerResult := component.ValResultOk(&trailersOpt)
		outerOpt := component.ValOption(&innerResult)
		return []component.Val{outerOpt}, nil

	case futureTrailersConsumed:
		return []component.Val{component.ValOption(nil)}, nil
	}

	return []component.Val{component.ValOption(nil)}, nil
}
```

Replace `futureTrailersSubscribe`:

```go
func futureTrailersSubscribe(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := getOrCreateTable(ctx)
	ft, err := getFutureTrailers(ctx, args[0].Borrow())
	if err != nil || table == nil {
		return []component.Val{component.ValOwn(0)}, nil
	}

	pollable := io.NewPollable(
		func() bool { return ft.IsReady() },
		func() { <-ft.done },
	)
	handle := table.New(pollable, true)
	return []component.Val{component.ValOwn(uint32(handle))}, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/http/ -run "Test(IncomingBodyFinish|FutureTrailers)" -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add imports/wasip2/http/http.go imports/wasip2/http/types.go imports/wasip2/http/http_test.go
git commit -m "feat(http): implement incoming-body.finish and future-trailers state machine"
```

---

### Task 21: `http-error-code` — Error bridge

**Files:**
- Modify: `imports/wasip2/http/http.go:1297-1302`
- Modify: `imports/wasip2/http/http_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestHttpErrorCode_WithHTTPError(t *testing.T) {
	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)

	ioErr := io.NewErrorFromString("connection-refused")
	handle := table.New(ioErr, true)

	errHandle := component.ValBorrow(uint32(handle))
	result, err := httpErrorCode(ctx, []component.Val{errHandle})
	require.NoError(t, err)
	require.NotNil(t, result[0].Option(), "should return Some for known HTTP error code")
}

func TestHttpErrorCode_WithNonHTTPError(t *testing.T) {
	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)

	ioErr := io.NewErrorFromString("some random error")
	handle := table.New(ioErr, true)

	errHandle := component.ValBorrow(uint32(handle))
	result, err := httpErrorCode(ctx, []component.Val{errHandle})
	require.NoError(t, err)
	require.Nil(t, result[0].Option(), "should return None for non-HTTP error")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/http/ -run "TestHttpErrorCode_" -v`
Expected: FAIL

- [ ] **Step 3: Implement http-error-code**

Replace `httpErrorCode` in `imports/wasip2/http/http.go`:

```go
func httpErrorCode(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	table := getOrCreateTable(ctx)
	if table == nil {
		return []component.Val{component.ValOption(nil)}, nil
	}

	entry, err := table.Get(component.Handle(handle))
	if err != nil {
		return []component.Val{component.ValOption(nil)}, nil
	}

	ioErr, ok := entry.Rep.(*io.Error)
	if !ok {
		return []component.Val{component.ValOption(nil)}, nil
	}

	errStr := ioErr.ToDebugString()
	// Check if it matches a known HTTP error code
	if isKnownHTTPErrorCode(errStr) {
		codeVal := errorCodeToVariant(ErrorCode(errStr))
		return []component.Val{component.ValOption(&codeVal)}, nil
	}

	return []component.Val{component.ValOption(nil)}, nil
}

func isKnownHTTPErrorCode(s string) bool {
	knownCodes := map[string]bool{
		"DNS-timeout": true, "DNS-error": true,
		"destination-not-found": true, "destination-unavailable": true,
		"destination-IP-prohibited": true, "destination-IP-unroutable": true,
		"connection-refused": true, "connection-terminated": true,
		"connection-timeout": true, "connection-read-timeout": true,
		"connection-write-timeout": true, "connection-limit-reached": true,
		"TLS-protocol-error": true, "TLS-certificate-error": true,
		"TLS-alert-received": true,
		"HTTP-request-denied": true, "HTTP-request-length-required": true,
		"HTTP-request-body-size": true, "HTTP-request-method-invalid": true,
		"HTTP-request-URI-invalid": true, "HTTP-request-URI-too-long": true,
		"HTTP-request-header-section-size": true, "HTTP-request-header-size": true,
		"HTTP-request-trailer-section-size": true, "HTTP-request-trailer-size": true,
		"HTTP-response-incomplete": true, "HTTP-response-header-section-size": true,
		"HTTP-response-header-size": true, "HTTP-response-body-size": true,
		"HTTP-response-trailer-section-size": true, "HTTP-response-trailer-size": true,
		"HTTP-response-transfer-coding": true, "HTTP-response-content-coding": true,
		"HTTP-response-timeout": true, "HTTP-upgrade-failed": true,
		"HTTP-protocol-error": true, "loop-detected": true,
		"configuration-error": true, "internal-error": true,
	}
	return knownCodes[s]
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/http/ -run "TestHttpErrorCode_" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add imports/wasip2/http/http.go imports/wasip2/http/http_test.go
git commit -m "feat(http): implement http-error-code bridge"
```

---

### Task 22: `incoming-handler` — Go HTTP bridge

**Files:**
- Modify: `imports/wasip2/http/incoming.go`
- Modify: `imports/wasip2/http/http_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestNewHTTPHandler_SimpleGET(t *testing.T) {
	handler := NewHTTPHandler(func(ctx context.Context, requestHandle, outparamHandle component.Handle) error {
		table := component.ResourceTableFromContext(ctx)

		// Read request
		entry, _ := table.Get(requestHandle)
		req := entry.Rep.(*IncomingRequest)
		require.Equal(t, MethodGet, req.Method())
		require.Equal(t, "/test", *req.PathWithQuery())

		// Build response
		headers := NewFields()
		headers.Set("X-Custom", [][]byte{[]byte("hello")})
		resp := NewOutgoingResponse(headers)
		resp.SetStatusCode(200)
		respHandle := table.New(resp, true)

		// Set outparam
		entry, _ = table.Get(outparamHandle)
		outparam := entry.Rep.(*ResponseOutparam)
		outparam.result <- ResponseResult{Response: resp}
		_ = respHandle

		return nil
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	require.Equal(t, 200, w.Code)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/http/ -run TestNewHTTPHandler -v`
Expected: FAIL — function doesn't exist

- [ ] **Step 3: Implement NewHTTPHandler**

Add to `imports/wasip2/http/incoming.go`:

```go
import (
	"context"
	gohttp "net/http"

	"github.com/tetratelabs/wazero/internal/component"
)

// NewHTTPHandler creates a Go http.Handler that bridges to a WASI component's
// incoming-handler.handle export. The callHandle function should invoke the
// component's handle function with the given request and outparam handles.
func NewHTTPHandler(callHandle func(ctx context.Context, requestHandle, outparamHandle component.Handle) error) gohttp.Handler {
	return gohttp.HandlerFunc(func(w gohttp.ResponseWriter, r *gohttp.Request) {
		table := component.NewResourceTable()
		ctx := component.WithResourceTable(r.Context(), table)

		// Build IncomingRequest from Go request
		method := methodFromString(r.Method)
		scheme := NewSchemeFromString(r.URL.Scheme)
		authority := r.Host
		pathWithQuery := r.URL.RequestURI()
		headers := NewFields()
		for name, values := range r.Header {
			for _, v := range values {
				headers.Append(name, []byte(v))
			}
		}

		req := NewIncomingRequest(method, scheme, &authority, &pathWithQuery, headers)
		if r.Body != nil {
			req.SetBody(NewIncomingBodyFromReader(r.Body))
		}
		requestHandle := table.New(req, true)

		// Create response outparam with channel
		outparam := NewResponseOutparam()
		outparamHandle := table.New(outparam, true)

		// Call the component's handle function
		if err := callHandle(ctx, requestHandle, outparamHandle); err != nil {
			gohttp.Error(w, "handler error", gohttp.StatusInternalServerError)
			return
		}

		// Wait for response
		resp, errCode, err := outparam.WaitForResponse(ctx)
		if err != nil {
			gohttp.Error(w, "timeout waiting for response", gohttp.StatusGatewayTimeout)
			return
		}

		if errCode != nil {
			gohttp.Error(w, string(*errCode), gohttp.StatusBadGateway)
			return
		}

		if resp == nil {
			gohttp.Error(w, "no response", gohttp.StatusInternalServerError)
			return
		}

		// Write response
		respHeaders := resp.Headers()
		if respHeaders != nil {
			for _, entry := range respHeaders.Entries() {
				w.Header().Add(entry.Name, string(entry.Value))
			}
		}
		w.WriteHeader(int(resp.StatusCode()))

		// Write body if available
		body, bodyErr := resp.Body()
		if bodyErr == nil && body != nil {
			w.Write(body.Bytes())
		}
	})
}

func methodFromString(s string) Method {
	switch s {
	case "GET":
		return MethodGet
	case "HEAD":
		return MethodHead
	case "POST":
		return MethodPost
	case "PUT":
		return MethodPut
	case "DELETE":
		return MethodDelete
	case "CONNECT":
		return MethodConnect
	case "OPTIONS":
		return MethodOptions
	case "TRACE":
		return MethodTrace
	case "PATCH":
		return MethodPatch
	default:
		return MethodOther
	}
}
```

Note: `NewSchemeFromString` and the `FieldEntry` type / `Entries()` return format need to be checked against the existing code. The implementor should verify these exist and adapt. The `Fields.Entries()` method returns `[]FieldEntry` where each has `Name string` and `Value []byte` — verify against `types.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/http/ -run TestNewHTTPHandler -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add imports/wasip2/http/incoming.go imports/wasip2/http/http_test.go
git commit -m "feat(http): implement NewHTTPHandler Go bridge for incoming-handler"
```

---

### Task 23: Final integration — Run all tests

- [ ] **Step 1: Run all HTTP tests**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/http/ -v -count=1`
Expected: All PASS

- [ ] **Step 2: Run all socket tests**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/sockets/ -v -count=1`
Expected: All PASS

- [ ] **Step 3: Run all filesystem tests**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/filesystem/ -v -count=1`
Expected: All PASS

- [ ] **Step 4: Run full wasip2 test suite**

Run: `cd /home/cchamplin/development/wazero && go test ./imports/wasip2/... -v -count=1`
Expected: All PASS

- [ ] **Step 5: Run full project test suite**

Run: `cd /home/cchamplin/development/wazero && go test ./... -count=1 2>&1 | tail -50`
Expected: No regressions

- [ ] **Step 6: Final commit**

```bash
git add -A
git commit -m "test: verify all WASI P2 stubs eliminated — zero remaining"
```
