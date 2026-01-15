// imports/wasip2/sockets/types.go

// Package sockets implements the wasi:sockets interfaces for WASI Preview 2.
// It provides TCP and UDP socket capabilities.
package sockets

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
