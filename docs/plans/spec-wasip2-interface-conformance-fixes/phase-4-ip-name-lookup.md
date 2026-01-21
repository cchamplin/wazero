# Phase 4: wasi:sockets/ip-name-lookup - DNS Resolution

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement DNS hostname resolution via the resolve-addresses function and resolve-address-stream resource.

**Architecture:** Use Go's net.LookupIP for DNS resolution, wrap results in an async stream pattern using channels for non-blocking iteration.

**Tech Stack:** Go net package, net.LookupIP, channels for async streaming

**Gap Analysis Reference:** [wasip2-interface-gap-analysis.md](../wasip2-interface-gap-analysis.md) - Section 5.4 wasi:sockets/ip-name-lookup@0.2.0

**Prerequisite:** Phases 1-3 should be complete. DNS is needed by Phase 5 (HTTP).

---

## Reference Materials

### Specification
- **WIT File:** `debug-vendored/WASI/proposals/sockets/wit/ip-name-lookup.wit`
- **Key Functions:**
  - `resolve-addresses: func(network: borrow<network>, name: string) -> result<resolve-address-stream, error-code>`
  - `resource resolve-address-stream` with `resolve-next-address` method

### Current Implementation
- **File:** `imports/wasip2/sockets/sockets.go`
- **Issues:**
  - resolve-addresses not implemented
  - resolve-address-stream resource missing

### Wasmtime Reference
- **File:** `debug-vendored/wasmtime/crates/wasi/src/host/network.rs`
- Look for `resolve_addresses` implementation

---

## Task 4.1: Create ResolveAddressStream Resource

**Files:**
- Create: `imports/wasip2/sockets/dns.go`
- Test: `imports/wasip2/sockets/dns_test.go`

**Step 1: Write failing test**

Create `imports/wasip2/sockets/dns_test.go`:

```go
package sockets

import (
	"net"
	"testing"
)

func TestResolveAddressStream_ResolveNext(t *testing.T) {
	// Create stream with known addresses
	addrs := []net.IP{
		net.ParseIP("127.0.0.1"),
		net.ParseIP("192.168.1.1"),
	}

	stream := NewResolveAddressStream(addrs)

	// Should return first address
	addr1, err := stream.ResolveNextAddress()
	if err != nil {
		t.Fatalf("first resolve failed: %v", err)
	}
	if !addr1.Equal(net.ParseIP("127.0.0.1")) {
		t.Errorf("expected 127.0.0.1, got %v", addr1)
	}

	// Should return second address
	addr2, err := stream.ResolveNextAddress()
	if err != nil {
		t.Fatalf("second resolve failed: %v", err)
	}
	if !addr2.Equal(net.ParseIP("192.168.1.1")) {
		t.Errorf("expected 192.168.1.1, got %v", addr2)
	}

	// Should return nil when exhausted
	addr3, err := stream.ResolveNextAddress()
	if err != nil {
		t.Fatalf("third resolve failed: %v", err)
	}
	if addr3 != nil {
		t.Errorf("expected nil, got %v", addr3)
	}
}

func TestResolveAddressStream_Subscribe(t *testing.T) {
	addrs := []net.IP{net.ParseIP("127.0.0.1")}
	stream := NewResolveAddressStream(addrs)

	pollable := stream.Subscribe()
	if pollable == nil {
		t.Error("expected pollable")
	}

	// Pollable should be ready when addresses available
	if !pollable.IsReady() {
		t.Error("pollable should be ready when addresses available")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/sockets/... -run "TestResolveAddressStream"`
Expected: FAIL - ResolveAddressStream doesn't exist

**Step 3: Create dns.go with ResolveAddressStream**

Create `imports/wasip2/sockets/dns.go`:

```go
package sockets

import (
	"net"
	"sync"

	wasip2io "github.com/tetratelabs/wazero/imports/wasip2/io"
)

// ResolveAddressStream provides async iteration over resolved addresses
type ResolveAddressStream struct {
	mu        sync.Mutex
	addresses []net.IP
	index     int
	pollable  *wasip2io.Pollable
}

// NewResolveAddressStream creates a stream from resolved addresses
func NewResolveAddressStream(addresses []net.IP) *ResolveAddressStream {
	stream := &ResolveAddressStream{
		addresses: addresses,
		index:     0,
	}

	// Create pollable - ready if we have addresses
	if len(addresses) > 0 {
		stream.pollable = wasip2io.NewReadyPollable()
	} else {
		stream.pollable = wasip2io.NewPollable()
	}

	return stream
}

// ResolveNextAddress returns the next resolved address, or nil if exhausted
func (s *ResolveAddressStream) ResolveNextAddress() (net.IP, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.index >= len(s.addresses) {
		return nil, nil
	}

	addr := s.addresses[s.index]
	s.index++

	return addr, nil
}

// Subscribe returns a pollable for address availability
func (s *ResolveAddressStream) Subscribe() *wasip2io.Pollable {
	return s.pollable
}

// HasMore returns true if more addresses are available
func (s *ResolveAddressStream) HasMore() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.index < len(s.addresses)
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/sockets/... -run "TestResolveAddressStream"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/sockets/dns.go imports/wasip2/sockets/dns_test.go
git commit -m "feat(wasip2): add ResolveAddressStream for DNS results

Provides async iteration over DNS resolution results."
```

---

## Task 4.2: Implement resolve-addresses Function

**Files:**
- Modify: `imports/wasip2/sockets/dns.go`
- Test: `imports/wasip2/sockets/dns_test.go`

**Step 1: Write failing test**

Add to `imports/wasip2/sockets/dns_test.go`:

```go
func TestResolveAddresses_Localhost(t *testing.T) {
	stream, err := ResolveAddresses("localhost")
	if err != nil {
		t.Fatalf("ResolveAddresses failed: %v", err)
	}

	// Should resolve to at least one address
	addr, err := stream.ResolveNextAddress()
	if err != nil {
		t.Fatalf("ResolveNextAddress failed: %v", err)
	}
	if addr == nil {
		t.Error("expected at least one address for localhost")
	}

	// Check it's a loopback address
	if !addr.IsLoopback() {
		t.Errorf("expected loopback address, got %v", addr)
	}
}

func TestResolveAddresses_InvalidHost(t *testing.T) {
	_, err := ResolveAddresses("this.host.definitely.does.not.exist.invalid")
	if err == nil {
		t.Error("expected error for invalid hostname")
	}
}

func TestResolveAddresses_IPAddress(t *testing.T) {
	// Resolving an IP address should return that address
	stream, err := ResolveAddresses("127.0.0.1")
	if err != nil {
		t.Fatalf("ResolveAddresses failed: %v", err)
	}

	addr, _ := stream.ResolveNextAddress()
	if addr == nil {
		t.Fatal("expected address")
	}
	if !addr.Equal(net.ParseIP("127.0.0.1")) {
		t.Errorf("expected 127.0.0.1, got %v", addr)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/sockets/... -run "TestResolveAddresses"`
Expected: FAIL - ResolveAddresses doesn't exist

**Step 3: Implement ResolveAddresses**

Add to `imports/wasip2/sockets/dns.go`:

```go
import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	wasip2io "github.com/tetratelabs/wazero/imports/wasip2/io"
)

var (
	ErrNameResolutionFailed = errors.New("name resolution failed")
)

// ResolveAddresses performs DNS resolution for a hostname
// Returns a stream that can be iterated for results
func ResolveAddresses(name string) (*ResolveAddressStream, error) {
	// First check if it's already an IP address
	if ip := net.ParseIP(name); ip != nil {
		return NewResolveAddressStream([]net.IP{ip}), nil
	}

	// Perform DNS lookup with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var resolver net.Resolver
	addrs, err := resolver.LookupIPAddr(ctx, name)
	if err != nil {
		return nil, ErrNameResolutionFailed
	}

	if len(addrs) == 0 {
		return nil, ErrNameResolutionFailed
	}

	// Extract IPs from IPAddr
	ips := make([]net.IP, len(addrs))
	for i, addr := range addrs {
		ips[i] = addr.IP
	}

	return NewResolveAddressStream(ips), nil
}

// ResolveAddressesWithFamily resolves filtering by address family
func ResolveAddressesWithFamily(name string, family AddressFamily) (*ResolveAddressStream, error) {
	// First check if it's already an IP address
	if ip := net.ParseIP(name); ip != nil {
		// Check family matches
		isIPv4 := ip.To4() != nil
		if (family == AddressFamilyIPv4 && !isIPv4) ||
			(family == AddressFamilyIPv6 && isIPv4) {
			return nil, ErrNameResolutionFailed
		}
		return NewResolveAddressStream([]net.IP{ip}), nil
	}

	// Determine network string for lookup
	network := "ip"
	switch family {
	case AddressFamilyIPv4:
		network = "ip4"
	case AddressFamilyIPv6:
		network = "ip6"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var resolver net.Resolver
	addrs, err := resolver.LookupIP(ctx, network, name)
	if err != nil {
		return nil, ErrNameResolutionFailed
	}

	if len(addrs) == 0 {
		return nil, ErrNameResolutionFailed
	}

	return NewResolveAddressStream(addrs), nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/sockets/... -run "TestResolveAddresses"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/sockets/dns.go imports/wasip2/sockets/dns_test.go
git commit -m "feat(wasip2): implement ResolveAddresses DNS resolution

Uses Go's net.Resolver for DNS lookups with timeout support."
```

---

## Task 4.3: Wire DNS to WASI Interface

**Files:**
- Create: `imports/wasip2/sockets/dns_wasi.go`
- Modify: `imports/wasip2/sockets/sockets.go`

**Step 1: Create WASI bindings file**

Create `imports/wasip2/sockets/dns_wasi.go`:

```go
package sockets

import (
	"context"
	"net"

	"github.com/tetratelabs/wazero/internal/component"
)

// RegisterIPNameLookup registers wasi:sockets/ip-name-lookup@0.2.0
func RegisterIPNameLookup(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:sockets/ip-name-lookup@0.2.0")

	// Define resource
	inst.Resource("resolve-address-stream", func(rep uint32) {})

	// Functions
	inst.FuncNoType("resolve-addresses", wasiResolveAddresses)
	inst.FuncNoType("[method]resolve-address-stream.resolve-next-address", wasiResolveNextAddress)
	inst.FuncNoType("[method]resolve-address-stream.subscribe", wasiResolveAddressStreamSubscribe)

	return inst.Build()
}

// wasiResolveAddresses implements resolve-addresses: func(network: borrow<network>, name: string) -> result<resolve-address-stream, error-code>
func wasiResolveAddresses(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return errorCodeResult(ErrorCodeUnknown), nil
	}

	// args[0] is network borrow (ignored for now, we use system DNS)
	name := args[1].String()

	stream, err := ResolveAddresses(name)
	if err != nil {
		return errorCodeResult(ErrorCodeNameUnresolvable), nil
	}

	handle := table.New(stream, true)
	return []component.Val{component.ValResultOk(component.ValOwn(uint32(handle)))}, nil
}

// wasiResolveNextAddress implements [method]resolve-address-stream.resolve-next-address
func wasiResolveNextAddress(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return errorCodeResult(ErrorCodeUnknown), nil
	}

	handle := args[0].Borrow()
	resource := table.Get(int(handle))
	stream, ok := resource.(*ResolveAddressStream)
	if !ok {
		return errorCodeResult(ErrorCodeUnknown), nil
	}

	addr, err := stream.ResolveNextAddress()
	if err != nil {
		return errorCodeResult(ErrorCodeUnknown), nil
	}

	if addr == nil {
		// No more addresses - return None
		return []component.Val{component.ValResultOk(component.ValOption(nil))}, nil
	}

	// Convert to ip-address variant
	ipVal := ipToComponentVal(addr)
	return []component.Val{component.ValResultOk(component.ValOption(&ipVal))}, nil
}

// wasiResolveAddressStreamSubscribe implements [method]resolve-address-stream.subscribe
func wasiResolveAddressStreamSubscribe(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []component.Val{component.ValOwn(0)}, nil
	}

	handle := args[0].Borrow()
	resource := table.Get(int(handle))
	stream, ok := resource.(*ResolveAddressStream)
	if !ok {
		return []component.Val{component.ValOwn(0)}, nil
	}

	pollable := stream.Subscribe()
	pollHandle := table.New(pollable, true)
	return []component.Val{component.ValOwn(uint32(pollHandle))}, nil
}

// ipToComponentVal converts a net.IP to the WASI ip-address variant
func ipToComponentVal(ip net.IP) component.Val {
	if ip4 := ip.To4(); ip4 != nil {
		// IPv4 - variant discriminant 0
		// tuple<u8, u8, u8, u8>
		tuple := component.ValTuple([]component.Val{
			component.ValU8(ip4[0]),
			component.ValU8(ip4[1]),
			component.ValU8(ip4[2]),
			component.ValU8(ip4[3]),
		})
		return component.ValVariant(0, tuple)
	}

	// IPv6 - variant discriminant 1
	// tuple<u16, u16, u16, u16, u16, u16, u16, u16>
	ip6 := ip.To16()
	tuple := component.ValTuple([]component.Val{
		component.ValU16(uint16(ip6[0])<<8 | uint16(ip6[1])),
		component.ValU16(uint16(ip6[2])<<8 | uint16(ip6[3])),
		component.ValU16(uint16(ip6[4])<<8 | uint16(ip6[5])),
		component.ValU16(uint16(ip6[6])<<8 | uint16(ip6[7])),
		component.ValU16(uint16(ip6[8])<<8 | uint16(ip6[9])),
		component.ValU16(uint16(ip6[10])<<8 | uint16(ip6[11])),
		component.ValU16(uint16(ip6[12])<<8 | uint16(ip6[13])),
		component.ValU16(uint16(ip6[14])<<8 | uint16(ip6[15])),
	})
	return component.ValVariant(1, tuple)
}
```

**Step 2: Update sockets.go to register DNS**

In `imports/wasip2/sockets/sockets.go`, add to the Instantiate function:

```go
func Instantiate(linker *component.Linker) error {
	// ... existing registrations ...

	if err := RegisterIPNameLookup(linker); err != nil {
		return err
	}

	return nil
}
```

**Step 3: Run tests**

Run: `go test -v ./imports/wasip2/sockets/...`
Expected: PASS

**Step 4: Commit**

```bash
git add imports/wasip2/sockets/dns_wasi.go imports/wasip2/sockets/sockets.go
git commit -m "feat(wasip2): wire DNS resolution to WASI ip-name-lookup interface

Complete implementation of wasi:sockets/ip-name-lookup@0.2.0."
```

---

## Phase 4 Checkpoint

**Run calculator tests to verify no regressions:**

```bash
go test -v ./internal/component/wasip2test/... -run "TestCalculator"
```

**Expected:** All tests pass for `add` and `subtract` plugins.

**If tests fail:**
1. Check that DNS registration doesn't conflict with existing interfaces
2. Verify error code definitions exist
3. Check component.Val helper functions exist
4. Debug and fix before proceeding to Phase 5

**Mark Phase 4 complete in README.md**
