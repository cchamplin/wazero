# Phase 3: WASI Interface Implementation

Complete the stub implementations in the WASI P2 interfaces.

---

## Task 3.1: Implement Filesystem Stream Methods

**Status:** PENDING

**Files:**
- Modify: `imports/wasip2/filesystem/filesystem.go:230-250`
- Test: `imports/wasip2/filesystem/filesystem_test.go`

**Step 1: Write the failing test**

```go
func TestDescriptorReadViaStream(t *testing.T) {
    // Create a temp file with content
    tmpFile, err := os.CreateTemp("", "wasi-test-*")
    if err != nil {
        t.Fatal(err)
    }
    defer os.Remove(tmpFile.Name())

    content := []byte("Hello, WASI!")
    tmpFile.Write(content)
    tmpFile.Close()

    // Open the file as a descriptor
    // Call read-via-stream
    // Read from the returned stream
    // Verify content matches
}

func TestDescriptorWriteViaStream(t *testing.T) {
    // Create a temp file
    // Get write-via-stream handle
    // Write to stream
    // Close stream
    // Verify file content
}
```

**Step 2:** Run: `go test ./imports/wasip2/filesystem -run "TestDescriptor.*ViaStream" -v`

**Step 3: Implementation**

```go
func descriptorReadViaStream(ctx context.Context, args []component.Val) ([]component.Val, error) {
    table := component.ResourceTableFromContext(ctx)
    if table == nil {
        return nil, fmt.Errorf("no resource table")
    }

    handle := component.Handle(args[0].Borrow())
    entry, err := table.Get(handle)
    if err != nil {
        return []component.Val{component.ValResultError(&errBadDescriptor)}, nil
    }

    desc, ok := entry.Rep.(*Descriptor)
    if !ok {
        return []component.Val{component.ValResultError(&errBadDescriptor)}, nil
    }

    // Create an input stream from the file
    file, err := os.Open(desc.path)
    if err != nil {
        errVal := mapFSError(err)
        return []component.Val{component.ValResultError(&errVal)}, nil
    }

    inputStream := io.NewInputStream(file)
    streamHandle := table.New(inputStream, true)

    return []component.Val{component.ValResultOk(&component.ValOwn(uint32(streamHandle)))}, nil
}
```

---

## Task 3.2: Implement TCP Socket Core Operations

**Status:** PENDING

**Files:**
- Modify: `imports/wasip2/sockets/tcp.go`
- Test: `imports/wasip2/sockets/tcp_test.go`

**Step 1: Write the failing test**

```go
func TestTCPSocket_BindAndListen(t *testing.T) {
    // Create a TCP socket
    // Bind to localhost:0 (any available port)
    // Start listening
    // Verify local address has assigned port
}

func TestTCPSocket_Connect(t *testing.T) {
    // Start a listening socket on localhost
    // Create client socket
    // Connect to listener
    // Verify connection established
}
```

**Step 2:** Run: `go test ./imports/wasip2/sockets -run "TestTCPSocket_" -v`

**Step 3:** Implementation details for TCP socket operations - bind, listen, connect, accept

---

## Task 3.3: Implement HTTP Incoming Request Handlers

**Status:** PENDING

**Files:**
- Modify: `imports/wasip2/http/http.go:725-745`
- Test: `imports/wasip2/http/http_test.go`

**Step 1: Write the failing test**

```go
func TestIncomingRequest_Method(t *testing.T) {
    // Create an incoming request with POST method
    req := &IncomingRequest{
        method: "POST",
        pathWithQuery: "/api/test?foo=bar",
        scheme: "https",
        authority: "example.com",
    }

    // Register in resource table
    // Call method accessor
    // Verify returns correct method variant
}
```

---

## Task 3.4: Implement Poll Multiplexing

**Status:** PENDING

**Files:**
- Modify: `imports/wasip2/io/poll.go:127-162`
- Test: `imports/wasip2/io/poll_test.go`

**Step 1: Write the failing test**

```go
func TestPoll_MultiplePollables(t *testing.T) {
    // Create multiple pollables with different ready times
    // Poll all of them
    // Verify the first one ready returns immediately
    // Verify others are properly tracked
}
```

**Step 2-6:** Implement using Go's select/channels, verify, and commit
