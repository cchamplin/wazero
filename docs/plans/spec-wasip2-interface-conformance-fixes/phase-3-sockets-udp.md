# Phase 3: wasi:sockets/udp - UDP Socket Implementation

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement full UDP socket support including datagram send/receive streams.

**Architecture:** Create UdpSocket struct mirroring TcpSocket pattern, implement incoming/outgoing datagram streams, and wire to WASI interface using async start/finish pattern.

**Tech Stack:** Go net package, net.UDPConn, goroutines for async operations

**Gap Analysis Reference:** [wasip2-interface-gap-analysis.md](../wasip2-interface-gap-analysis.md) - Section 5.3 wasi:sockets/udp@0.2.0

**Prerequisite:** Phases 1-2 (poll/streams) should be complete for proper integration.

---

## Reference Materials

### Specification
- **WIT File:** `debug-vendored/WASI/proposals/sockets/wit/udp.wit`
- **Key Types:**
  - `resource udp-socket` - UDP socket handle
  - `resource incoming-datagram-stream` - Receive datagrams
  - `resource outgoing-datagram-stream` - Send datagrams
  - `record incoming-datagram` - {data: list<u8>, remote-address: ip-socket-address}
  - `record outgoing-datagram` - {data: list<u8>, remote-address: option<ip-socket-address>}

### Current Implementation
- **File:** `imports/wasip2/sockets/sockets.go`
- **Issues:**
  - udp-socket resource exists but has no methods
  - No datagram stream resources
  - udp-create-socket returns error

### Wasmtime Reference
- **File:** `debug-vendored/wasmtime/crates/wasi/src/host/udp.rs`
- Look for UdpSocket, IncomingDatagramStream, OutgoingDatagramStream

### TCP Reference (same codebase)
- **File:** `imports/wasip2/sockets/tcp.go`
- Use TcpSocket as pattern for state machine and async operations

---

## Task 3.1: Create UdpSocket Struct with State Management

**Files:**
- Create: `imports/wasip2/sockets/udp.go`
- Test: `imports/wasip2/sockets/udp_test.go`

**Step 1: Write failing test**

Create `imports/wasip2/sockets/udp_test.go`:

```go
package sockets

import (
	"testing"
)

func TestUdpSocket_NewSocket(t *testing.T) {
	// Create IPv4 UDP socket
	sock, err := NewUdpSocket(AddressFamilyIPv4)
	if err != nil {
		t.Fatalf("failed to create socket: %v", err)
	}
	if sock == nil {
		t.Fatal("socket is nil")
	}

	if sock.AddressFamily() != AddressFamilyIPv4 {
		t.Errorf("expected IPv4, got %v", sock.AddressFamily())
	}

	// Initial state should be Unbound
	if sock.State() != UdpStateUnbound {
		t.Errorf("expected Unbound state, got %v", sock.State())
	}
}

func TestUdpSocket_NewSocketIPv6(t *testing.T) {
	sock, err := NewUdpSocket(AddressFamilyIPv6)
	if err != nil {
		t.Fatalf("failed to create socket: %v", err)
	}

	if sock.AddressFamily() != AddressFamilyIPv6 {
		t.Errorf("expected IPv6, got %v", sock.AddressFamily())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/sockets/... -run "TestUdpSocket_New"`
Expected: FAIL - NewUdpSocket doesn't exist

**Step 3: Create udp.go with UdpSocket struct**

Create `imports/wasip2/sockets/udp.go`:

```go
package sockets

import (
	"net"
	"sync"
)

// UdpState represents the state of a UDP socket
type UdpState int

const (
	UdpStateUnbound UdpState = iota
	UdpStateBinding
	UdpStateBound
	UdpStateConnected // Has remote address set
)

// UdpSocket represents a wasi:sockets/udp socket
type UdpSocket struct {
	mu sync.Mutex

	family AddressFamily
	state  UdpState
	conn   *net.UDPConn

	// Pending bind address
	pendingLocalAddr *net.UDPAddr

	// Remote address for connected mode
	remoteAddr *net.UDPAddr

	// Socket options
	unicastHopLimit   uint8
	receiveBufferSize uint64
	sendBufferSize    uint64
}

// NewUdpSocket creates a new UDP socket
func NewUdpSocket(family AddressFamily) (*UdpSocket, error) {
	return &UdpSocket{
		family:            family,
		state:             UdpStateUnbound,
		unicastHopLimit:   64,
		receiveBufferSize: 64 * 1024,
		sendBufferSize:    64 * 1024,
	}, nil
}

// AddressFamily returns the socket's address family
func (s *UdpSocket) AddressFamily() AddressFamily {
	return s.family
}

// State returns the current socket state
func (s *UdpSocket) State() UdpState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/sockets/... -run "TestUdpSocket_New"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/sockets/udp.go imports/wasip2/sockets/udp_test.go
git commit -m "feat(wasip2): add UdpSocket struct with state management

Initial UDP socket implementation with state machine matching TCP pattern."
```

---

## Task 3.2: Implement start-bind/finish-bind for UDP

**Files:**
- Modify: `imports/wasip2/sockets/udp.go`
- Test: `imports/wasip2/sockets/udp_test.go`

**Step 1: Write failing test**

Add to `imports/wasip2/sockets/udp_test.go`:

```go
func TestUdpSocket_Bind(t *testing.T) {
	sock, _ := NewUdpSocket(AddressFamilyIPv4)

	// Start bind to localhost:0 (random port)
	addr := &net.UDPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: 0,
	}

	err := sock.StartBind(addr)
	if err != nil {
		t.Fatalf("StartBind failed: %v", err)
	}

	if sock.State() != UdpStateBinding {
		t.Errorf("expected Binding state, got %v", sock.State())
	}

	err = sock.FinishBind()
	if err != nil {
		t.Fatalf("FinishBind failed: %v", err)
	}

	if sock.State() != UdpStateBound {
		t.Errorf("expected Bound state, got %v", sock.State())
	}

	// Should have a local address now
	localAddr := sock.LocalAddress()
	if localAddr == nil {
		t.Error("expected local address after bind")
	}
	if localAddr.Port == 0 {
		t.Error("expected non-zero port after bind")
	}
}

func TestUdpSocket_Bind_AlreadyBound(t *testing.T) {
	sock, _ := NewUdpSocket(AddressFamilyIPv4)
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}

	sock.StartBind(addr)
	sock.FinishBind()

	// Try to bind again - should fail
	err := sock.StartBind(addr)
	if err == nil {
		t.Error("expected error when binding already-bound socket")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/sockets/... -run "TestUdpSocket_Bind"`
Expected: FAIL - StartBind, FinishBind don't exist

**Step 3: Implement bind methods**

Add to `imports/wasip2/sockets/udp.go`:

```go
import (
	"errors"
	"net"
	"sync"
)

var (
	ErrUdpAlreadyBound    = errors.New("socket already bound")
	ErrUdpNotBinding      = errors.New("no bind operation in progress")
	ErrUdpInvalidState    = errors.New("invalid socket state")
	ErrUdpAddressMismatch = errors.New("address family mismatch")
)

// StartBind begins an async bind operation
func (s *UdpSocket) StartBind(addr *net.UDPAddr) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != UdpStateUnbound {
		return ErrUdpAlreadyBound
	}

	// Validate address family
	if addr.IP.To4() != nil && s.family != AddressFamilyIPv4 {
		return ErrUdpAddressMismatch
	}
	if addr.IP.To4() == nil && s.family != AddressFamilyIPv6 {
		return ErrUdpAddressMismatch
	}

	s.pendingLocalAddr = addr
	s.state = UdpStateBinding
	return nil
}

// FinishBind completes the bind operation
func (s *UdpSocket) FinishBind() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != UdpStateBinding {
		return ErrUdpNotBinding
	}

	network := "udp4"
	if s.family == AddressFamilyIPv6 {
		network = "udp6"
	}

	conn, err := net.ListenUDP(network, s.pendingLocalAddr)
	if err != nil {
		s.state = UdpStateUnbound
		s.pendingLocalAddr = nil
		return err
	}

	s.conn = conn
	s.pendingLocalAddr = nil
	s.state = UdpStateBound
	return nil
}

// LocalAddress returns the bound local address
func (s *UdpSocket) LocalAddress() *net.UDPAddr {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn == nil {
		return nil
	}
	return s.conn.LocalAddr().(*net.UDPAddr)
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/sockets/... -run "TestUdpSocket_Bind"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/sockets/udp.go imports/wasip2/sockets/udp_test.go
git commit -m "feat(wasip2): implement UDP start-bind/finish-bind

Async bind pattern matching TCP socket implementation."
```

---

## Task 3.3: Implement stream Method for UDP Datagram Streams

**Files:**
- Modify: `imports/wasip2/sockets/udp.go`
- Test: `imports/wasip2/sockets/udp_test.go`

**Step 1: Write failing test**

Add to `imports/wasip2/sockets/udp_test.go`:

```go
func TestUdpSocket_Stream(t *testing.T) {
	sock, _ := NewUdpSocket(AddressFamilyIPv4)
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}

	sock.StartBind(addr)
	sock.FinishBind()

	// Get streams without remote address (unconnected mode)
	inStream, outStream, err := sock.Stream(nil)
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	if inStream == nil {
		t.Error("expected incoming stream")
	}
	if outStream == nil {
		t.Error("expected outgoing stream")
	}
}

func TestUdpSocket_Stream_WithRemote(t *testing.T) {
	sock, _ := NewUdpSocket(AddressFamilyIPv4)
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}

	sock.StartBind(localAddr)
	sock.FinishBind()

	// Get streams with remote address (connected mode)
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}
	inStream, outStream, err := sock.Stream(remoteAddr)
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	if inStream == nil || outStream == nil {
		t.Error("expected both streams")
	}

	// Socket should now be in connected state
	if sock.State() != UdpStateConnected {
		t.Errorf("expected Connected state, got %v", sock.State())
	}
}

func TestUdpSocket_Stream_NotBound(t *testing.T) {
	sock, _ := NewUdpSocket(AddressFamilyIPv4)

	// Should fail - not bound
	_, _, err := sock.Stream(nil)
	if err == nil {
		t.Error("expected error when getting streams on unbound socket")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/sockets/... -run "TestUdpSocket_Stream"`
Expected: FAIL - Stream method doesn't exist

**Step 3: Implement Stream method and datagram stream types**

Add to `imports/wasip2/sockets/udp.go`:

```go
// IncomingDatagramStream receives datagrams from a UDP socket
type IncomingDatagramStream struct {
	socket *UdpSocket
}

// OutgoingDatagramStream sends datagrams to a UDP socket
type OutgoingDatagramStream struct {
	socket *UdpSocket
}

// Stream creates datagram streams for this socket
// If remoteAddr is provided, the socket enters "connected" mode
func (s *UdpSocket) Stream(remoteAddr *net.UDPAddr) (*IncomingDatagramStream, *OutgoingDatagramStream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != UdpStateBound && s.state != UdpStateConnected {
		return nil, nil, ErrUdpInvalidState
	}

	// If remote address provided, enter connected mode
	if remoteAddr != nil {
		s.remoteAddr = remoteAddr
		s.state = UdpStateConnected
	} else {
		// Clear any previous remote address
		s.remoteAddr = nil
		if s.state == UdpStateConnected {
			s.state = UdpStateBound
		}
	}

	inStream := &IncomingDatagramStream{socket: s}
	outStream := &OutgoingDatagramStream{socket: s}

	return inStream, outStream, nil
}

// RemoteAddress returns the connected remote address, if any
func (s *UdpSocket) RemoteAddress() *net.UDPAddr {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.remoteAddr
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/sockets/... -run "TestUdpSocket_Stream"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/sockets/udp.go imports/wasip2/sockets/udp_test.go
git commit -m "feat(wasip2): implement UDP stream method for datagram streams

Creates incoming/outgoing datagram streams with optional connected mode."
```

---

## Task 3.4: Implement incoming-datagram-stream Receive

**Files:**
- Modify: `imports/wasip2/sockets/udp.go`
- Test: `imports/wasip2/sockets/udp_test.go`

**Step 1: Write failing test**

Add to `imports/wasip2/sockets/udp_test.go`:

```go
func TestIncomingDatagramStream_Receive(t *testing.T) {
	// Create two sockets for testing
	sock1, _ := NewUdpSocket(AddressFamilyIPv4)
	sock1.StartBind(&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	sock1.FinishBind()
	addr1 := sock1.LocalAddress()

	sock2, _ := NewUdpSocket(AddressFamilyIPv4)
	sock2.StartBind(&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	sock2.FinishBind()

	inStream, _, _ := sock1.Stream(nil)
	_, outStream, _ := sock2.Stream(nil)

	// Send a datagram from sock2 to sock1
	testData := []byte("hello udp")
	outDatagram := OutgoingDatagram{
		Data:          testData,
		RemoteAddress: addr1,
	}

	sent, err := outStream.Send([]OutgoingDatagram{outDatagram})
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if sent != 1 {
		t.Errorf("expected to send 1, sent %d", sent)
	}

	// Receive on sock1
	datagrams, err := inStream.Receive(10)
	if err != nil {
		t.Fatalf("Receive failed: %v", err)
	}
	if len(datagrams) != 1 {
		t.Fatalf("expected 1 datagram, got %d", len(datagrams))
	}

	if string(datagrams[0].Data) != "hello udp" {
		t.Errorf("expected 'hello udp', got '%s'", string(datagrams[0].Data))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/sockets/... -run "TestIncomingDatagramStream_Receive"`
Expected: FAIL - Receive, IncomingDatagram don't exist

**Step 3: Implement datagram types and Receive**

Add to `imports/wasip2/sockets/udp.go`:

```go
// IncomingDatagram represents a received UDP datagram
type IncomingDatagram struct {
	Data          []byte
	RemoteAddress *net.UDPAddr
}

// OutgoingDatagram represents a datagram to be sent
type OutgoingDatagram struct {
	Data          []byte
	RemoteAddress *net.UDPAddr // Required for unconnected, optional for connected
}

// Receive receives up to maxResults datagrams
func (s *IncomingDatagramStream) Receive(maxResults uint64) ([]IncomingDatagram, error) {
	if maxResults == 0 {
		return []IncomingDatagram{}, nil
	}

	s.socket.mu.Lock()
	conn := s.socket.conn
	remoteAddr := s.socket.remoteAddr
	s.socket.mu.Unlock()

	if conn == nil {
		return nil, ErrUdpInvalidState
	}

	// Set read deadline for non-blocking check
	// If no data available, return empty list
	conn.SetReadDeadline(time.Now().Add(time.Millisecond))
	defer conn.SetReadDeadline(time.Time{})

	var datagrams []IncomingDatagram
	buf := make([]byte, 65536) // Max UDP datagram size

	for uint64(len(datagrams)) < maxResults {
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			// Timeout means no more data available
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break
			}
			return datagrams, err
		}

		// In connected mode, filter to only datagrams from remote address
		if remoteAddr != nil {
			if !addr.IP.Equal(remoteAddr.IP) || addr.Port != remoteAddr.Port {
				continue
			}
		}

		datagram := IncomingDatagram{
			Data:          make([]byte, n),
			RemoteAddress: addr,
		}
		copy(datagram.Data, buf[:n])
		datagrams = append(datagrams, datagram)
	}

	return datagrams, nil
}

// Subscribe returns a pollable for receive readiness
func (s *IncomingDatagramStream) Subscribe() *wasip2io.Pollable {
	// For now, return a ready pollable
	// TODO: Implement proper async notification
	return wasip2io.NewReadyPollable()
}
```

Also add `"time"` to imports and import the io package:
```go
import (
	wasip2io "github.com/tetratelabs/wazero/imports/wasip2/io"
)
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/sockets/... -run "TestIncomingDatagramStream_Receive"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/sockets/udp.go imports/wasip2/sockets/udp_test.go
git commit -m "feat(wasip2): implement IncomingDatagramStream.Receive

Non-blocking receive that returns available datagrams up to max count."
```

---

## Task 3.5: Implement outgoing-datagram-stream Send

**Files:**
- Modify: `imports/wasip2/sockets/udp.go`
- Test: `imports/wasip2/sockets/udp_test.go`

**Step 1: Write failing test**

Add to `imports/wasip2/sockets/udp_test.go`:

```go
func TestOutgoingDatagramStream_CheckSend(t *testing.T) {
	sock, _ := NewUdpSocket(AddressFamilyIPv4)
	sock.StartBind(&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	sock.FinishBind()

	_, outStream, _ := sock.Stream(nil)

	// check-send should return non-zero when socket is ready
	capacity, err := outStream.CheckSend()
	if err != nil {
		t.Fatalf("CheckSend failed: %v", err)
	}
	if capacity == 0 {
		t.Error("expected non-zero send capacity")
	}
}

func TestOutgoingDatagramStream_Send_Connected(t *testing.T) {
	// Create receiver
	receiver, _ := NewUdpSocket(AddressFamilyIPv4)
	receiver.StartBind(&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	receiver.FinishBind()
	receiverAddr := receiver.LocalAddress()

	// Create sender in connected mode
	sender, _ := NewUdpSocket(AddressFamilyIPv4)
	sender.StartBind(&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	sender.FinishBind()

	// Connect to receiver
	_, outStream, _ := sender.Stream(receiverAddr)

	// Send without specifying address (uses connected address)
	datagram := OutgoingDatagram{
		Data:          []byte("connected mode"),
		RemoteAddress: nil, // Should use connected address
	}

	sent, err := outStream.Send([]OutgoingDatagram{datagram})
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if sent != 1 {
		t.Errorf("expected to send 1, sent %d", sent)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/sockets/... -run "TestOutgoingDatagramStream"`
Expected: FAIL - CheckSend, Send don't exist

**Step 3: Implement CheckSend and Send**

Add to `imports/wasip2/sockets/udp.go`:

```go
// CheckSend returns the number of datagrams that can be sent
func (s *OutgoingDatagramStream) CheckSend() (uint64, error) {
	s.socket.mu.Lock()
	conn := s.socket.conn
	s.socket.mu.Unlock()

	if conn == nil {
		return 0, ErrUdpInvalidState
	}

	// UDP can typically accept many datagrams
	// Return a reasonable batch size
	return 1024, nil
}

// Send sends datagrams, returning the number successfully sent
func (s *OutgoingDatagramStream) Send(datagrams []OutgoingDatagram) (uint64, error) {
	if len(datagrams) == 0 {
		return 0, nil
	}

	s.socket.mu.Lock()
	conn := s.socket.conn
	connectedAddr := s.socket.remoteAddr
	s.socket.mu.Unlock()

	if conn == nil {
		return 0, ErrUdpInvalidState
	}

	var sent uint64
	for _, dg := range datagrams {
		addr := dg.RemoteAddress

		// In connected mode, use connected address if not specified
		if addr == nil {
			if connectedAddr == nil {
				// Unconnected mode requires address
				return sent, errors.New("remote address required in unconnected mode")
			}
			addr = connectedAddr
		} else if connectedAddr != nil {
			// Connected mode: must match connected address
			if !addr.IP.Equal(connectedAddr.IP) || addr.Port != connectedAddr.Port {
				return sent, errors.New("address must match connected remote address")
			}
		}

		_, err := conn.WriteToUDP(dg.Data, addr)
		if err != nil {
			return sent, err
		}
		sent++
	}

	return sent, nil
}

// Subscribe returns a pollable for send readiness
func (s *OutgoingDatagramStream) Subscribe() *wasip2io.Pollable {
	// UDP send is typically always ready
	return wasip2io.NewReadyPollable()
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/sockets/... -run "TestOutgoingDatagramStream"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/sockets/udp.go imports/wasip2/sockets/udp_test.go
git commit -m "feat(wasip2): implement OutgoingDatagramStream.Send

Supports both connected and unconnected sending modes."
```

---

## Task 3.6: Implement UDP Socket Options

**Files:**
- Modify: `imports/wasip2/sockets/udp.go`
- Test: `imports/wasip2/sockets/udp_test.go`

**Step 1: Write failing test**

Add to `imports/wasip2/sockets/udp_test.go`:

```go
func TestUdpSocket_Options(t *testing.T) {
	sock, _ := NewUdpSocket(AddressFamilyIPv4)

	// Test unicast hop limit
	limit, err := sock.UnicastHopLimit()
	if err != nil {
		t.Fatalf("UnicastHopLimit failed: %v", err)
	}
	if limit == 0 {
		t.Error("expected non-zero default hop limit")
	}

	err = sock.SetUnicastHopLimit(128)
	if err != nil {
		t.Fatalf("SetUnicastHopLimit failed: %v", err)
	}

	limit, _ = sock.UnicastHopLimit()
	if limit != 128 {
		t.Errorf("expected hop limit 128, got %d", limit)
	}

	// Test buffer sizes
	size, _ := sock.ReceiveBufferSize()
	if size == 0 {
		t.Error("expected non-zero default receive buffer")
	}

	sock.SetReceiveBufferSize(32768)
	size, _ = sock.ReceiveBufferSize()
	if size != 32768 {
		t.Errorf("expected 32768, got %d", size)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/sockets/... -run "TestUdpSocket_Options"`
Expected: FAIL - Option methods don't exist

**Step 3: Implement socket options**

Add to `imports/wasip2/sockets/udp.go`:

```go
// UnicastHopLimit returns the TTL/hop limit for unicast packets
func (s *UdpSocket) UnicastHopLimit() (uint8, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.unicastHopLimit, nil
}

// SetUnicastHopLimit sets the TTL/hop limit
func (s *UdpSocket) SetUnicastHopLimit(value uint8) error {
	if value == 0 {
		return errors.New("hop limit must be at least 1")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unicastHopLimit = value

	// Apply to socket if bound
	if s.conn != nil {
		// TODO: Apply via syscall
	}
	return nil
}

// ReceiveBufferSize returns the receive buffer size
func (s *UdpSocket) ReceiveBufferSize() (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.receiveBufferSize, nil
}

// SetReceiveBufferSize sets the receive buffer size
func (s *UdpSocket) SetReceiveBufferSize(value uint64) error {
	if value == 0 {
		return errors.New("buffer size must be non-zero")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.receiveBufferSize = value

	if s.conn != nil {
		s.conn.SetReadBuffer(int(value))
	}
	return nil
}

// SendBufferSize returns the send buffer size
func (s *UdpSocket) SendBufferSize() (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sendBufferSize, nil
}

// SetSendBufferSize sets the send buffer size
func (s *UdpSocket) SetSendBufferSize(value uint64) error {
	if value == 0 {
		return errors.New("buffer size must be non-zero")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sendBufferSize = value

	if s.conn != nil {
		s.conn.SetWriteBuffer(int(value))
	}
	return nil
}

// Subscribe returns a pollable for I/O readiness
func (s *UdpSocket) Subscribe() *wasip2io.Pollable {
	return wasip2io.NewReadyPollable()
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/sockets/... -run "TestUdpSocket_Options"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/sockets/udp.go imports/wasip2/sockets/udp_test.go
git commit -m "feat(wasip2): implement UDP socket options

Adds hop limit, receive/send buffer size options."
```

---

## Task 3.7: Wire UDP Socket to WASI Interface

**Files:**
- Modify: `imports/wasip2/sockets/sockets.go`
- Create: `imports/wasip2/sockets/udp_wasi.go`

**Step 1: Create WASI bindings file**

Create `imports/wasip2/sockets/udp_wasi.go`:

```go
package sockets

import (
	"context"
	"net"

	"github.com/tetratelabs/wazero/internal/component"
)

// RegisterUdpSocket registers wasi:sockets/udp@0.2.0 interface
func RegisterUdpSocket(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:sockets/udp@0.2.0")

	// Define resources
	inst.Resource("udp-socket", func(rep uint32) {})
	inst.Resource("incoming-datagram-stream", func(rep uint32) {})
	inst.Resource("outgoing-datagram-stream", func(rep uint32) {})

	// udp-socket methods
	inst.FuncNoType("[method]udp-socket.start-bind", udpSocketStartBind)
	inst.FuncNoType("[method]udp-socket.finish-bind", udpSocketFinishBind)
	inst.FuncNoType("[method]udp-socket.stream", udpSocketStream)
	inst.FuncNoType("[method]udp-socket.local-address", udpSocketLocalAddress)
	inst.FuncNoType("[method]udp-socket.remote-address", udpSocketRemoteAddress)
	inst.FuncNoType("[method]udp-socket.address-family", udpSocketAddressFamily)
	inst.FuncNoType("[method]udp-socket.unicast-hop-limit", udpSocketUnicastHopLimit)
	inst.FuncNoType("[method]udp-socket.set-unicast-hop-limit", udpSocketSetUnicastHopLimit)
	inst.FuncNoType("[method]udp-socket.receive-buffer-size", udpSocketReceiveBufferSize)
	inst.FuncNoType("[method]udp-socket.set-receive-buffer-size", udpSocketSetReceiveBufferSize)
	inst.FuncNoType("[method]udp-socket.send-buffer-size", udpSocketSendBufferSize)
	inst.FuncNoType("[method]udp-socket.set-send-buffer-size", udpSocketSetSendBufferSize)
	inst.FuncNoType("[method]udp-socket.subscribe", udpSocketSubscribe)

	// incoming-datagram-stream methods
	inst.FuncNoType("[method]incoming-datagram-stream.receive", incomingDatagramReceive)
	inst.FuncNoType("[method]incoming-datagram-stream.subscribe", incomingDatagramSubscribe)

	// outgoing-datagram-stream methods
	inst.FuncNoType("outgoing-datagram-stream.check-send", outgoingDatagramCheckSend)
	inst.FuncNoType("[method]outgoing-datagram-stream.send", outgoingDatagramSend)
	inst.FuncNoType("[method]outgoing-datagram-stream.subscribe", outgoingDatagramSubscribe)

	return inst.Build()
}

// RegisterUdpCreateSocket registers wasi:sockets/udp-create-socket@0.2.0
func RegisterUdpCreateSocket(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:sockets/udp-create-socket@0.2.0")

	inst.FuncNoType("create-udp-socket", createUdpSocket)

	return inst.Build()
}

func createUdpSocket(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return errorCodeResult(ErrorCodeUnknown), nil
	}

	family := AddressFamily(args[0].U8())
	sock, err := NewUdpSocket(family)
	if err != nil {
		return errorCodeResult(ErrorCodeUnknown), nil
	}

	handle := table.New(sock, true)
	return []component.Val{component.ValResultOk(component.ValOwn(uint32(handle)))}, nil
}

func udpSocketStartBind(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return errorCodeResult(ErrorCodeUnknown), nil
	}

	handle := args[0].Borrow()
	// args[1] is network borrow (ignored for now)
	addrVal := args[2]

	resource := table.Get(int(handle))
	sock, ok := resource.(*UdpSocket)
	if !ok {
		return errorCodeResult(ErrorCodeInvalidState), nil
	}

	addr := parseIPSocketAddress(addrVal)
	if addr == nil {
		return errorCodeResult(ErrorCodeInvalidArgument), nil
	}

	udpAddr := &net.UDPAddr{
		IP:   addr.IP,
		Port: addr.Port,
	}

	if err := sock.StartBind(udpAddr); err != nil {
		return errorCodeResult(ErrorCodeAddressInUse), nil
	}

	return []component.Val{component.ValResultOk(component.ValUnit())}, nil
}

func udpSocketFinishBind(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return errorCodeResult(ErrorCodeUnknown), nil
	}

	handle := args[0].Borrow()
	resource := table.Get(int(handle))
	sock, ok := resource.(*UdpSocket)
	if !ok {
		return errorCodeResult(ErrorCodeInvalidState), nil
	}

	if err := sock.FinishBind(); err != nil {
		return errorCodeResult(ErrorCodeAddressInUse), nil
	}

	return []component.Val{component.ValResultOk(component.ValUnit())}, nil
}

func udpSocketStream(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return errorCodeResult(ErrorCodeUnknown), nil
	}

	handle := args[0].Borrow()
	remoteAddrOpt := args[1]

	resource := table.Get(int(handle))
	sock, ok := resource.(*UdpSocket)
	if !ok {
		return errorCodeResult(ErrorCodeInvalidState), nil
	}

	var remoteAddr *net.UDPAddr
	if someVal := remoteAddrOpt.Option(); someVal != nil {
		addr := parseIPSocketAddress(*someVal)
		if addr != nil {
			remoteAddr = &net.UDPAddr{IP: addr.IP, Port: addr.Port}
		}
	}

	inStream, outStream, err := sock.Stream(remoteAddr)
	if err != nil {
		return errorCodeResult(ErrorCodeInvalidState), nil
	}

	inHandle := table.New(inStream, true)
	outHandle := table.New(outStream, true)

	result := component.ValTuple([]component.Val{
		component.ValOwn(uint32(inHandle)),
		component.ValOwn(uint32(outHandle)),
	})

	return []component.Val{component.ValResultOk(result)}, nil
}

// ... Additional method implementations follow the same pattern
// Each method extracts socket from resource table and calls Go method
```

**Step 2: Update sockets.go to register UDP**

In `imports/wasip2/sockets/sockets.go`, add to the Instantiate function:

```go
func Instantiate(linker *component.Linker) error {
	// ... existing TCP registrations ...

	if err := RegisterUdpSocket(linker); err != nil {
		return err
	}
	if err := RegisterUdpCreateSocket(linker); err != nil {
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
git add imports/wasip2/sockets/udp_wasi.go imports/wasip2/sockets/sockets.go
git commit -m "feat(wasip2): wire UDP socket to WASI interface

Complete UDP support with all socket methods registered."
```

---

## Phase 3 Checkpoint

**Run calculator tests to verify no regressions:**

```bash
go test -v ./internal/component/wasip2test/... -run "TestCalculator"
```

**Expected:** All tests pass for `add` and `subtract` plugins.

**If tests fail:**
1. Check that socket registration doesn't conflict with existing interfaces
2. Verify resource table operations
3. Check import for wasip2io package
4. Debug and fix before proceeding to Phase 4

**Mark Phase 3 complete in README.md**
