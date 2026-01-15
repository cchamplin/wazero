// imports/wasip2/sockets/types.go

// Package sockets implements the wasi:sockets interfaces for WASI Preview 2.
// It provides TCP and UDP socket capabilities.
package sockets

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"

	"github.com/tetratelabs/wazero/internal/component"
)

// IpAddressFamily represents the IP address family (IPv4 or IPv6).
// Matches wasi:sockets/network ip-address-family enum.
type IpAddressFamily uint8

const (
	IpAddressFamilyIpv4 IpAddressFamily = iota
	IpAddressFamilyIpv6
)

// String returns the WASI name for this IP address family.
func (f IpAddressFamily) String() string {
	switch f {
	case IpAddressFamilyIpv4:
		return "ipv4"
	case IpAddressFamilyIpv6:
		return "ipv6"
	default:
		return "ipv4"
	}
}

// IpAddress represents an IP address (either IPv4 or IPv6).
// Matches wasi:sockets/network ip-address variant.
type IpAddress struct {
	family IpAddressFamily
	ipv4   [4]byte
	ipv6   [16]byte
}

// NewIpv4Address creates an IPv4 address.
func NewIpv4Address(addr [4]byte) IpAddress {
	return IpAddress{
		family: IpAddressFamilyIpv4,
		ipv4:   addr,
	}
}

// NewIpv6Address creates an IPv6 address.
func NewIpv6Address(addr [16]byte) IpAddress {
	return IpAddress{
		family: IpAddressFamilyIpv6,
		ipv6:   addr,
	}
}

// Family returns the IP address family.
func (a IpAddress) Family() IpAddressFamily {
	return a.family
}

// Ipv4 returns the IPv4 address bytes.
func (a IpAddress) Ipv4() [4]byte {
	return a.ipv4
}

// Ipv6 returns the IPv6 address bytes.
func (a IpAddress) Ipv6() [16]byte {
	return a.ipv6
}

// IpSocketAddress represents an IP socket address (address + port).
// Matches wasi:sockets/network ip-socket-address variant.
type IpSocketAddress struct {
	family  IpAddressFamily
	port    uint16
	address IpAddress
}

// NewIpv4SocketAddress creates an IPv4 socket address.
func NewIpv4SocketAddress(addr [4]byte, port uint16) IpSocketAddress {
	return IpSocketAddress{
		family:  IpAddressFamilyIpv4,
		port:    port,
		address: NewIpv4Address(addr),
	}
}

// NewIpv6SocketAddress creates an IPv6 socket address.
func NewIpv6SocketAddress(addr [16]byte, port uint16) IpSocketAddress {
	return IpSocketAddress{
		family:  IpAddressFamilyIpv6,
		port:    port,
		address: NewIpv6Address(addr),
	}
}

// Family returns the address family.
func (s IpSocketAddress) Family() IpAddressFamily {
	return s.family
}

// Port returns the port number.
func (s IpSocketAddress) Port() uint16 {
	return s.port
}

// Address returns the IP address.
func (s IpSocketAddress) Address() IpAddress {
	return s.address
}

// ErrorCode represents socket error codes.
// Matches wasi:sockets/network error-code enum.
type ErrorCode string

const (
	ErrorCodeUnknown                  ErrorCode = "unknown"
	ErrorCodeAccessDenied             ErrorCode = "access-denied"
	ErrorCodeNotSupported             ErrorCode = "not-supported"
	ErrorCodeInvalidArgument          ErrorCode = "invalid-argument"
	ErrorCodeOutOfMemory              ErrorCode = "out-of-memory"
	ErrorCodeTimeout                  ErrorCode = "timeout"
	ErrorCodeConcurrencyConflict      ErrorCode = "concurrency-conflict"
	ErrorCodeNotInProgress            ErrorCode = "not-in-progress"
	ErrorCodeWouldBlock               ErrorCode = "would-block"
	ErrorCodeInvalidState             ErrorCode = "invalid-state"
	ErrorCodeNewSocketLimit           ErrorCode = "new-socket-limit"
	ErrorCodeAddressNotBindable       ErrorCode = "address-not-bindable"
	ErrorCodeAddressInUse             ErrorCode = "address-in-use"
	ErrorCodeRemoteUnreachable        ErrorCode = "remote-unreachable"
	ErrorCodeConnectionRefused        ErrorCode = "connection-refused"
	ErrorCodeConnectionReset          ErrorCode = "connection-reset"
	ErrorCodeConnectionAborted        ErrorCode = "connection-aborted"
	ErrorCodeDatagramTooLarge         ErrorCode = "datagram-too-large"
	ErrorCodeNameUnresolvable         ErrorCode = "name-unresolvable"
	ErrorCodeTemporaryResolverFailure ErrorCode = "temporary-resolver-failure"
	ErrorCodePermanentResolverFailure ErrorCode = "permanent-resolver-failure"
)

// tcpState represents the state of a TCP socket.
type tcpState uint8

const (
	tcpStateUnbound tcpState = iota
	tcpStateBinding
	tcpStateBound
	tcpStateListening
	tcpStateConnecting
	tcpStateConnected
	tcpStateClosed
)

// TcpSocket represents a TCP socket.
// Matches wasi:sockets/tcp tcp-socket resource.
type TcpSocket struct {
	family            IpAddressFamily
	state             tcpState
	localAddr         *IpSocketAddress
	remoteAddr        *IpSocketAddress
	listenBacklog     uint64
	keepAliveEnabled  bool
	keepAliveIdleTime uint64
	keepAliveInterval uint64
	keepAliveCount    uint32
	hopLimit          uint8
	receiveBufferSize uint64
	sendBufferSize    uint64

	// Go net types for actual socket operations
	listener   *net.TCPListener // Set when listening
	conn       *net.TCPConn     // Set when connected (client or accepted)
	pendingErr error            // Error from async operation
}

// NewTcpSocket creates a new TCP socket with the given address family.
func NewTcpSocket(family IpAddressFamily) *TcpSocket {
	return &TcpSocket{
		family:            family,
		state:             tcpStateUnbound,
		listenBacklog:     128,
		keepAliveEnabled:  false,
		keepAliveIdleTime: 7200, // 2 hours in seconds
		keepAliveInterval: 75,   // 75 seconds
		keepAliveCount:    9,
		hopLimit:          64,
		receiveBufferSize: 65536,
		sendBufferSize:    65536,
	}
}

// Family returns the address family.
func (s *TcpSocket) Family() IpAddressFamily {
	return s.family
}

// State returns the current socket state.
func (s *TcpSocket) State() tcpState {
	return s.state
}

// IsListening returns true if the socket is in listening state.
func (s *TcpSocket) IsListening() bool {
	return s.state == tcpStateListening
}

// LocalAddress returns the local address if bound.
func (s *TcpSocket) LocalAddress() *IpSocketAddress {
	return s.localAddr
}

// RemoteAddress returns the remote address if connected.
func (s *TcpSocket) RemoteAddress() *IpSocketAddress {
	return s.remoteAddr
}

// ShutdownType represents the type of shutdown operation.
// Matches wasi:sockets/tcp shutdown-type enum.
type ShutdownType uint8

const (
	ShutdownTypeReceive ShutdownType = iota
	ShutdownTypeSend
	ShutdownTypeBoth
)

// String returns the WASI name for this shutdown type.
func (st ShutdownType) String() string {
	switch st {
	case ShutdownTypeReceive:
		return "receive"
	case ShutdownTypeSend:
		return "send"
	case ShutdownTypeBoth:
		return "both"
	default:
		return "both"
	}
}

// udpState represents the state of a UDP socket.
type udpState uint8

const (
	udpStateUnbound udpState = iota
	udpStateBinding
	udpStateBound
	udpStateClosed
)

// UdpSocket represents a UDP socket.
// Matches wasi:sockets/udp udp-socket resource.
type UdpSocket struct {
	family            IpAddressFamily
	state             udpState
	localAddr         *IpSocketAddress
	remoteAddr        *IpSocketAddress
	unicastHopLimit   uint8
	receiveBufferSize uint64
	sendBufferSize    uint64

	// Go net types for actual socket operations
	conn       *net.UDPConn // Set when bound
	pendingErr error        // Error from async operation
}

// NewUdpSocket creates a new UDP socket with the given address family.
func NewUdpSocket(family IpAddressFamily) *UdpSocket {
	return &UdpSocket{
		family:            family,
		state:             udpStateUnbound,
		unicastHopLimit:   64,
		receiveBufferSize: 65536,
		sendBufferSize:    65536,
	}
}

// Family returns the address family.
func (s *UdpSocket) Family() IpAddressFamily {
	return s.family
}

// State returns the current socket state.
func (s *UdpSocket) State() udpState {
	return s.state
}

// LocalAddress returns the local address if bound.
func (s *UdpSocket) LocalAddress() *IpSocketAddress {
	return s.localAddr
}

// RemoteAddress returns the remote address if connected.
func (s *UdpSocket) RemoteAddress() *IpSocketAddress {
	return s.remoteAddr
}

// IncomingDatagram represents an incoming UDP datagram.
// Matches wasi:sockets/udp incoming-datagram record.
type IncomingDatagram struct {
	Data       []byte
	RemoteAddr IpSocketAddress
}

// OutgoingDatagram represents an outgoing UDP datagram.
// Matches wasi:sockets/udp outgoing-datagram record.
type OutgoingDatagram struct {
	Data       []byte
	RemoteAddr *IpSocketAddress // Optional
}

// IncomingDatagramStream represents a stream of incoming datagrams.
// Matches wasi:sockets/udp incoming-datagram-stream resource.
type IncomingDatagramStream struct {
	socket *UdpSocket
}

// NewIncomingDatagramStream creates a new incoming datagram stream.
func NewIncomingDatagramStream(socket *UdpSocket) *IncomingDatagramStream {
	return &IncomingDatagramStream{socket: socket}
}

// OutgoingDatagramStream represents a stream of outgoing datagrams.
// Matches wasi:sockets/udp outgoing-datagram-stream resource.
type OutgoingDatagramStream struct {
	socket *UdpSocket
}

// NewOutgoingDatagramStream creates a new outgoing datagram stream.
func NewOutgoingDatagramStream(socket *UdpSocket) *OutgoingDatagramStream {
	return &OutgoingDatagramStream{socket: socket}
}

// Network represents a network capability.
// Matches wasi:sockets/network network resource.
type Network struct {
	// For now, just a placeholder
}

// NewNetwork creates a new network capability.
func NewNetwork() *Network {
	return &Network{}
}

// ResolveAddressStream represents a stream of resolved addresses.
// Matches wasi:sockets/ip-name-lookup resolve-address-stream resource.
type ResolveAddressStream struct {
	addresses []IpAddress
	index     int
}

// NewResolveAddressStream creates a new resolve address stream.
func NewResolveAddressStream(addresses []IpAddress) *ResolveAddressStream {
	return &ResolveAddressStream{
		addresses: addresses,
		index:     0,
	}
}

// ResolveNextAddress returns the next address from the stream.
func (s *ResolveAddressStream) ResolveNextAddress() (*IpAddress, bool) {
	if s.index >= len(s.addresses) {
		return nil, false
	}
	addr := &s.addresses[s.index]
	s.index++
	return addr, true
}

// ===========================
// Address Conversion Helpers
// ===========================

// ipSocketAddressFromVal converts a component.Val ip-socket-address variant to IpSocketAddress.
func ipSocketAddressFromVal(val component.Val) (*IpSocketAddress, error) {
	caseName, payload := val.Variant()
	if payload == nil {
		return nil, errors.New("invalid ip-socket-address: nil payload")
	}

	record := payload.Record()
	portVal, hasPort := record["port"]
	if !hasPort {
		return nil, errors.New("invalid ip-socket-address: missing port")
	}
	port := portVal.U16()

	addrVal, hasAddr := record["address"]
	if !hasAddr {
		return nil, errors.New("invalid ip-socket-address: missing address")
	}

	switch caseName {
	case "ipv4":
		addrTuple := addrVal.Tuple()
		if len(addrTuple) != 4 {
			return nil, errors.New("invalid ipv4 address: expected 4 bytes")
		}
		addr := [4]byte{
			addrTuple[0].U8(),
			addrTuple[1].U8(),
			addrTuple[2].U8(),
			addrTuple[3].U8(),
		}
		result := NewIpv4SocketAddress(addr, port)
		return &result, nil

	case "ipv6":
		addrTuple := addrVal.Tuple()
		if len(addrTuple) != 8 {
			return nil, errors.New("invalid ipv6 address: expected 8 u16 values")
		}
		var addr [16]byte
		for i := 0; i < 8; i++ {
			u16Val := addrTuple[i].U16()
			addr[i*2] = byte(u16Val >> 8)
			addr[i*2+1] = byte(u16Val)
		}
		result := NewIpv6SocketAddress(addr, port)
		return &result, nil

	default:
		return nil, fmt.Errorf("unknown ip-socket-address variant: %s", caseName)
	}
}

// ipSocketAddressToVal converts an IpSocketAddress to a component.Val ip-socket-address variant.
func ipSocketAddressToVal(addr *IpSocketAddress) component.Val {
	if addr == nil {
		// Return default IPv4 0.0.0.0:0
		addrRecord := component.ValRecord(map[string]component.Val{
			"port":    component.ValU16(0),
			"address": component.ValTuple([]component.Val{component.ValU8(0), component.ValU8(0), component.ValU8(0), component.ValU8(0)}),
		})
		return component.ValVariant("ipv4", &addrRecord)
	}

	switch addr.Family() {
	case IpAddressFamilyIpv4:
		ipv4 := addr.Address().Ipv4()
		addrRecord := component.ValRecord(map[string]component.Val{
			"port": component.ValU16(addr.Port()),
			"address": component.ValTuple([]component.Val{
				component.ValU8(ipv4[0]),
				component.ValU8(ipv4[1]),
				component.ValU8(ipv4[2]),
				component.ValU8(ipv4[3]),
			}),
		})
		return component.ValVariant("ipv4", &addrRecord)

	case IpAddressFamilyIpv6:
		ipv6 := addr.Address().Ipv6()
		tupleVals := make([]component.Val, 8)
		for i := 0; i < 8; i++ {
			u16Val := uint16(ipv6[i*2])<<8 | uint16(ipv6[i*2+1])
			tupleVals[i] = component.ValU16(u16Val)
		}
		addrRecord := component.ValRecord(map[string]component.Val{
			"port":    component.ValU16(addr.Port()),
			"address": component.ValTuple(tupleVals),
		})
		return component.ValVariant("ipv6", &addrRecord)

	default:
		// Fallback to IPv4 0.0.0.0:0
		addrRecord := component.ValRecord(map[string]component.Val{
			"port":    component.ValU16(0),
			"address": component.ValTuple([]component.Val{component.ValU8(0), component.ValU8(0), component.ValU8(0), component.ValU8(0)}),
		})
		return component.ValVariant("ipv4", &addrRecord)
	}
}

// ipSocketAddressToTCPAddr converts IpSocketAddress to net.TCPAddr.
func ipSocketAddressToTCPAddr(addr *IpSocketAddress) *net.TCPAddr {
	if addr == nil {
		return nil
	}
	switch addr.Family() {
	case IpAddressFamilyIpv4:
		ipv4 := addr.Address().Ipv4()
		return &net.TCPAddr{
			IP:   net.IPv4(ipv4[0], ipv4[1], ipv4[2], ipv4[3]),
			Port: int(addr.Port()),
		}
	case IpAddressFamilyIpv6:
		ipv6 := addr.Address().Ipv6()
		return &net.TCPAddr{
			IP:   net.IP(ipv6[:]),
			Port: int(addr.Port()),
		}
	default:
		return nil
	}
}

// tcpAddrToIpSocketAddress converts net.TCPAddr to IpSocketAddress.
func tcpAddrToIpSocketAddress(addr *net.TCPAddr) *IpSocketAddress {
	if addr == nil {
		return nil
	}
	if ipv4 := addr.IP.To4(); ipv4 != nil {
		result := NewIpv4SocketAddress([4]byte{ipv4[0], ipv4[1], ipv4[2], ipv4[3]}, uint16(addr.Port))
		return &result
	}
	// IPv6
	var ipv6 [16]byte
	copy(ipv6[:], addr.IP.To16())
	result := NewIpv6SocketAddress(ipv6, uint16(addr.Port))
	return &result
}

// ipSocketAddressToUDPAddr converts IpSocketAddress to net.UDPAddr.
func ipSocketAddressToUDPAddr(addr *IpSocketAddress) *net.UDPAddr {
	if addr == nil {
		return nil
	}
	switch addr.Family() {
	case IpAddressFamilyIpv4:
		ipv4 := addr.Address().Ipv4()
		return &net.UDPAddr{
			IP:   net.IPv4(ipv4[0], ipv4[1], ipv4[2], ipv4[3]),
			Port: int(addr.Port()),
		}
	case IpAddressFamilyIpv6:
		ipv6 := addr.Address().Ipv6()
		return &net.UDPAddr{
			IP:   net.IP(ipv6[:]),
			Port: int(addr.Port()),
		}
	default:
		return nil
	}
}

// udpAddrToIpSocketAddress converts net.UDPAddr to IpSocketAddress.
func udpAddrToIpSocketAddress(addr *net.UDPAddr) *IpSocketAddress {
	if addr == nil {
		return nil
	}
	if ipv4 := addr.IP.To4(); ipv4 != nil {
		result := NewIpv4SocketAddress([4]byte{ipv4[0], ipv4[1], ipv4[2], ipv4[3]}, uint16(addr.Port))
		return &result
	}
	// IPv6
	var ipv6 [16]byte
	copy(ipv6[:], addr.IP.To16())
	result := NewIpv6SocketAddress(ipv6, uint16(addr.Port))
	return &result
}

// ===========================
// Error Code Mapping
// ===========================

// mapNetError maps a Go net error to an ErrorCode.
func mapNetError(err error) ErrorCode {
	if err == nil {
		return ErrorCodeUnknown
	}

	errStr := err.Error()

	// Check for common error patterns
	if strings.Contains(errStr, "permission denied") ||
		strings.Contains(errStr, "operation not permitted") {
		return ErrorCodeAccessDenied
	}
	if strings.Contains(errStr, "address already in use") ||
		strings.Contains(errStr, "bind: address already in use") {
		return ErrorCodeAddressInUse
	}
	if strings.Contains(errStr, "connection refused") {
		return ErrorCodeConnectionRefused
	}
	if strings.Contains(errStr, "connection reset") {
		return ErrorCodeConnectionReset
	}
	if strings.Contains(errStr, "connection aborted") {
		return ErrorCodeConnectionAborted
	}
	if strings.Contains(errStr, "network is unreachable") ||
		strings.Contains(errStr, "host is unreachable") ||
		strings.Contains(errStr, "no route to host") {
		return ErrorCodeRemoteUnreachable
	}
	if strings.Contains(errStr, "operation timed out") ||
		strings.Contains(errStr, "i/o timeout") {
		return ErrorCodeTimeout
	}
	if strings.Contains(errStr, "would block") ||
		strings.Contains(errStr, "resource temporarily unavailable") {
		return ErrorCodeWouldBlock
	}
	if strings.Contains(errStr, "too many open files") {
		return ErrorCodeNewSocketLimit
	}

	// Check for syscall errors
	var sysErr syscall.Errno
	if errors.As(err, &sysErr) {
		switch sysErr {
		case syscall.EADDRINUSE:
			return ErrorCodeAddressInUse
		case syscall.ECONNREFUSED:
			return ErrorCodeConnectionRefused
		case syscall.ECONNRESET:
			return ErrorCodeConnectionReset
		case syscall.ECONNABORTED:
			return ErrorCodeConnectionAborted
		case syscall.ETIMEDOUT:
			return ErrorCodeTimeout
		case syscall.ENETUNREACH, syscall.EHOSTUNREACH:
			return ErrorCodeRemoteUnreachable
		case syscall.EACCES, syscall.EPERM:
			return ErrorCodeAccessDenied
		case syscall.EWOULDBLOCK:
			return ErrorCodeWouldBlock
		case syscall.EMFILE, syscall.ENFILE:
			return ErrorCodeNewSocketLimit
		case syscall.EINVAL:
			return ErrorCodeInvalidArgument
		}
	}

	return ErrorCodeUnknown
}

// errorCodeToVal converts an ErrorCode to a component.Val error result.
func errorCodeToVal(code ErrorCode) component.Val {
	errVal := component.ValEnum(string(code))
	return component.ValResultError(&errVal)
}

// ===========================
// Resource Table Helpers
// ===========================

// getTcpSocket retrieves a TcpSocket from the ResourceTable using a borrow handle.
func getTcpSocket(ctx context.Context, handle uint32) (*TcpSocket, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return nil, errors.New("no resource table in context")
	}
	entry, err := table.Get(component.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	sock, ok := entry.Rep.(*TcpSocket)
	if !ok {
		return nil, fmt.Errorf("handle %d is not a TcpSocket", handle)
	}
	return sock, nil
}

// getUdpSocket retrieves a UdpSocket from the ResourceTable using a borrow handle.
func getUdpSocket(ctx context.Context, handle uint32) (*UdpSocket, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return nil, errors.New("no resource table in context")
	}
	entry, err := table.Get(component.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	sock, ok := entry.Rep.(*UdpSocket)
	if !ok {
		return nil, fmt.Errorf("handle %d is not a UdpSocket", handle)
	}
	return sock, nil
}

// getIncomingDatagramStream retrieves an IncomingDatagramStream from the ResourceTable.
func getIncomingDatagramStream(ctx context.Context, handle uint32) (*IncomingDatagramStream, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return nil, errors.New("no resource table in context")
	}
	entry, err := table.Get(component.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	stream, ok := entry.Rep.(*IncomingDatagramStream)
	if !ok {
		return nil, fmt.Errorf("handle %d is not an IncomingDatagramStream", handle)
	}
	return stream, nil
}

// getOutgoingDatagramStream retrieves an OutgoingDatagramStream from the ResourceTable.
func getOutgoingDatagramStream(ctx context.Context, handle uint32) (*OutgoingDatagramStream, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return nil, errors.New("no resource table in context")
	}
	entry, err := table.Get(component.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	stream, ok := entry.Rep.(*OutgoingDatagramStream)
	if !ok {
		return nil, fmt.Errorf("handle %d is not an OutgoingDatagramStream", handle)
	}
	return stream, nil
}

// parseAddressFamily parses an address family enum value.
func parseAddressFamily(val component.Val) IpAddressFamily {
	family := val.Enum()
	if family == "ipv6" {
		return IpAddressFamilyIpv6
	}
	return IpAddressFamilyIpv4
}

// ===========================
// TcpSocket Close Method
// ===========================

// Close closes the TCP socket and releases resources.
func (s *TcpSocket) Close() error {
	var err error
	if s.listener != nil {
		err = s.listener.Close()
		s.listener = nil
	}
	if s.conn != nil {
		err = s.conn.Close()
		s.conn = nil
	}
	s.state = tcpStateClosed
	return err
}

// ===========================
// UdpSocket Close Method
// ===========================

// Close closes the UDP socket and releases resources.
func (s *UdpSocket) Close() error {
	var err error
	if s.conn != nil {
		err = s.conn.Close()
		s.conn = nil
	}
	s.state = udpStateClosed
	return err
}
