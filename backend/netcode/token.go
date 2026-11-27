// Package netcode mints netcode.io connect tokens that the Rust crate
// lightyear_netcode 0.28.0 (lightyear's fork of netcode.io 1.02) accepts.
//
// The layout follows the crate source, not the upstream netcode STANDARD.md.
// The important difference: lightyear_netcode serializes every integer with
// lightyear_serde's WriteInteger/ReadInteger traits, which are BIG-endian
// (lightyear_serde 0.28.0 src/writer.rs:259-290, src/reader.rs:226-265).
// Upstream netcode.io 1.02 is little-endian. This applies to the protocol id,
// timestamps, timeout, address count, ports, client id and the AEAD
// associated data. Byte-for-byte compatibility with upstream netcode.io
// servers is therefore NOT a goal; compatibility with lightyear is.
//
// Source references (lightyear_netcode 0.28.0, git 28e823d9df39):
//   - src/lib.rs:114  CONNECTION_TIMEOUT_SEC = 15
//   - src/lib.rs:118  PRIVATE_KEY_BYTES = 32
//   - src/lib.rs:120  USER_DATA_BYTES = 256
//   - src/lib.rs:122  CONNECT_TOKEN_BYTES = 2048
//   - src/lib.rs:126  NETCODE_VERSION = b"NETCODE 1.02\0" (13 bytes)
//   - src/lib.rs:112  MAC_BYTES = 16
//   - src/token.rs:20 MAX_SERVERS_PER_CONNECT = 32
//   - src/token.rs:21 TOKEN_EXPIRE_SEC = 30
//   - src/token.rs:83-132  AddressList write/read (u32 count, type 1=v4, 2=v6)
//   - src/token.rs:145-155 AEAD associated data
//   - src/token.rs:157-183 ConnectTokenPrivate encrypt/decrypt
//   - src/token.rs:186-225 ConnectTokenPrivate field order, SIZE = 1024
//   - src/token.rs:454-520 ConnectToken field order, SIZE = 2048
//   - src/crypto.rs:115-162 XChaCha20-Poly1305 with detached tag in the last 16 bytes
//
// Public token layout (2048 bytes, all integers big-endian):
//
//	offset  size  field
//	0       13    version info "NETCODE 1.02\x00"
//	13      8     protocol id u64
//	21      8     create timestamp u64 (unix seconds)
//	29      8     expire timestamp u64 (unix seconds)
//	37      24    XChaCha20-Poly1305 nonce
//	61      1024  encrypted private token (1008 ciphertext + 16 byte tag)
//	1085    4     timeout seconds i32
//	1089    4     server address count u32 (1..32)
//	1093    var   addresses: type u8 (1=IPv4: 4 octets, 2=IPv6: 16 octets) + port u16
//	...     32    client to server key
//	...     32    server to client key
//	...     rest  zero padding up to 2048
//
// Private token plaintext (zero padded to 1008 bytes before encryption):
//
//	client id u64, timeout seconds i32, address list (as above),
//	client to server key [32], server to client key [32], user data [256]
//
// Associated data: version info (13) || protocol id u64 || expire timestamp u64.
package netcode

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
)

const (
	// ConnectTokenBytes is the size of a serialized public connect token.
	ConnectTokenBytes = 2048
	// UserDataBytes is the size of the opaque user data in the private token.
	UserDataBytes = 256
	// KeyBytes is the size of the private key and the session keys.
	KeyBytes = 32
	// NonceBytes is the XChaCha20-Poly1305 nonce size used for the private token.
	NonceBytes = chacha20poly1305.NonceSizeX
	// MaxServerAddresses is the maximum number of server addresses per token
	// (MAX_SERVERS_PER_CONNECT, src/token.rs:20). The reader rejects 0 and > 32.
	MaxServerAddresses = 32
	// MaxServerIDBytes is the maximum length of the server id stored in UserData.
	MaxServerIDBytes = 64

	// DefaultTimeoutSeconds is lightyear's default connection timeout,
	// CONNECTION_TIMEOUT_SEC = 15 in lightyear_netcode 0.28.0 src/lib.rs:114,
	// used by ConnectTokenBuilder::new (src/token.rs:346).
	DefaultTimeoutSeconds int32 = 15
	// DefaultTokenTTL is lightyear's default token lifetime,
	// TOKEN_EXPIRE_SEC = 30 in lightyear_netcode 0.28.0 src/token.rs:21.
	DefaultTokenTTL = 30 * time.Second

	privateTokenBytes = 1024 // ConnectTokenPrivate::SIZE, includes the 16 byte tag
	macBytes          = 16
	addressTypeIPv4   = 1
	addressTypeIPv6   = 2
)

// versionInfo is NETCODE_VERSION: "NETCODE 1.02" followed by a NUL byte.
var versionInfo = [13]byte{'N', 'E', 'T', 'C', 'O', 'D', 'E', ' ', '1', '.', '0', '2', 0}

// Byte offsets inside the public token (see the layout table in the package doc).
const (
	offVersion    = 0
	offProtocolID = offVersion + len(versionInfo) // 13
	offCreate     = offProtocolID + 8             // 21
	offExpire     = offCreate + 8                 // 29
	offNonce      = offExpire + 8                 // 37
	offPrivate    = offNonce + NonceBytes         // 61
	offTimeout    = offPrivate + privateTokenBytes
	offAddresses  = offTimeout + 4 // 1089
)

var (
	ErrNoServers          = errors.New("netcode: at least one server address is required")
	ErrTooManyServers     = fmt.Errorf("netcode: at most %d server addresses are allowed", MaxServerAddresses)
	ErrInvalidAddress     = errors.New("netcode: invalid server address")
	ErrInvalidKeyLength   = fmt.Errorf("netcode: private key must be %d bytes", KeyBytes)
	ErrInvalidTimestamps  = errors.New("netcode: create timestamp is greater than expire timestamp")
	ErrInvalidTTL         = errors.New("netcode: ttl must be at least one second")
	ErrPrivateTooLarge    = errors.New("netcode: private token data exceeds its buffer")
	ErrPublicTokenTooLong = errors.New("netcode: public token data exceeds its buffer")
)

// ConnectTokenParams holds every input of a connect token. EncodeConnectToken
// is a pure function of these values.
type ConnectTokenParams struct {
	ProtocolID        uint64
	PrivateKey        [32]byte
	ClientID          uint64
	CreateTimestamp   uint64 // unix seconds
	ExpireTimestamp   uint64 // unix seconds
	TimeoutSeconds    int32
	Nonce             [24]byte
	ServerAddresses   []netip.AddrPort // 1..32
	ClientToServerKey [32]byte
	ServerToClientKey [32]byte
	UserData          [UserDataBytes]byte
}

// EncodeConnectToken serializes and encrypts a connect token. The output is
// deterministic and always exactly ConnectTokenBytes long.
//
// The same server address list is written to both the private and the public
// part, which matches ConnectTokenBuilder::generate when no internal
// addresses are set (src/token.rs:393-397).
func EncodeConnectToken(p ConnectTokenParams) ([]byte, error) {
	if p.CreateTimestamp > p.ExpireTimestamp {
		// ConnectToken::read_from rejects this (src/token.rs:485).
		return nil, ErrInvalidTimestamps
	}
	addrs, err := encodeAddressList(p.ServerAddresses)
	if err != nil {
		return nil, err
	}

	// Private part: plaintext occupies the first 1008 bytes (zero padded),
	// the Poly1305 tag the last 16 (src/token.rs:165-168, src/crypto.rs:121-135).
	plain := make([]byte, 0, privateTokenBytes-macBytes)
	plain = binary.BigEndian.AppendUint64(plain, p.ClientID)
	plain = binary.BigEndian.AppendUint32(plain, uint32(p.TimeoutSeconds))
	plain = append(plain, addrs...)
	plain = append(plain, p.ClientToServerKey[:]...)
	plain = append(plain, p.ServerToClientKey[:]...)
	plain = append(plain, p.UserData[:]...)
	if len(plain) > privateTokenBytes-macBytes {
		return nil, ErrPrivateTooLarge
	}
	plain = plain[:privateTokenBytes-macBytes]

	aead, err := chacha20poly1305.NewX(p.PrivateKey[:])
	if err != nil {
		return nil, fmt.Errorf("netcode: init xchacha20poly1305: %w", err)
	}

	out := make([]byte, ConnectTokenBytes)
	copy(out[offVersion:], versionInfo[:])
	binary.BigEndian.PutUint64(out[offProtocolID:], p.ProtocolID)
	binary.BigEndian.PutUint64(out[offCreate:], p.CreateTimestamp)
	binary.BigEndian.PutUint64(out[offExpire:], p.ExpireTimestamp)
	copy(out[offNonce:], p.Nonce[:])

	sealed := aead.Seal(out[offPrivate:offPrivate], p.Nonce[:], plain, associatedData(p.ProtocolID, p.ExpireTimestamp))
	if len(sealed) != privateTokenBytes {
		return nil, ErrPrivateTooLarge
	}

	pub := out[offTimeout:offTimeout]
	pub = binary.BigEndian.AppendUint32(pub, uint32(p.TimeoutSeconds))
	pub = append(pub, addrs...)
	pub = append(pub, p.ClientToServerKey[:]...)
	pub = append(pub, p.ServerToClientKey[:]...)
	if offTimeout+len(pub) > ConnectTokenBytes {
		return nil, ErrPublicTokenTooLong
	}
	// The remainder of out is already zero (padding).
	return out, nil
}

// NewConnectToken mints a token with a random nonce and random session keys.
// The create timestamp is now and the expire timestamp is now+ttl, both in
// whole unix seconds (ttl is truncated to seconds and must be >= 1s).
func NewConnectToken(privateKey []byte, protocolID, clientID uint64, ttl time.Duration, timeoutSeconds int32, servers []netip.AddrPort, userData [UserDataBytes]byte) (token []byte, expiresAt time.Time, err error) {
	if len(privateKey) != KeyBytes {
		return nil, time.Time{}, ErrInvalidKeyLength
	}
	ttlSeconds := int64(ttl / time.Second)
	if ttlSeconds < 1 {
		return nil, time.Time{}, ErrInvalidTTL
	}
	now := time.Now().Unix()
	if now < 0 {
		return nil, time.Time{}, errors.New("netcode: system clock is before the unix epoch")
	}

	p := ConnectTokenParams{
		ProtocolID:      protocolID,
		ClientID:        clientID,
		CreateTimestamp: uint64(now),
		ExpireTimestamp: uint64(now) + uint64(ttlSeconds),
		TimeoutSeconds:  timeoutSeconds,
		ServerAddresses: servers,
		UserData:        userData,
	}
	copy(p.PrivateKey[:], privateKey)
	for _, b := range [][]byte{p.Nonce[:], p.ClientToServerKey[:], p.ServerToClientKey[:]} {
		if _, err := rand.Read(b); err != nil {
			return nil, time.Time{}, fmt.Errorf("netcode: read random bytes: %w", err)
		}
	}

	token, err = EncodeConnectToken(p)
	if err != nil {
		return nil, time.Time{}, err
	}
	return token, time.Unix(int64(p.ExpireTimestamp), 0), nil
}

// UserData builds the 256 byte user data blob shared with the Rust game server:
//
//	bytes 0..8      user id, u64 little-endian
//	byte  8         length N of the server id (N <= 64)
//	bytes 9..9+N    server id (ASCII)
//	rest            zero
//
// Server ids longer than MaxServerIDBytes are truncated to 64 bytes.
// Note: this blob is opaque to netcode, so its little-endian user id is
// independent of the big-endian integers lightyear uses for token fields.
func UserData(userID uint64, serverID string) [UserDataBytes]byte {
	var b [UserDataBytes]byte
	binary.LittleEndian.PutUint64(b[0:8], userID)
	if len(serverID) > MaxServerIDBytes {
		serverID = serverID[:MaxServerIDBytes]
	}
	b[8] = byte(len(serverID))
	copy(b[9:], serverID)
	return b
}

// ParseUserData is the inverse of UserData. A length byte above 64 is clamped.
func ParseUserData(b [UserDataBytes]byte) (userID uint64, serverID string) {
	userID = binary.LittleEndian.Uint64(b[0:8])
	n := int(b[8])
	if n > MaxServerIDBytes {
		n = MaxServerIDBytes
	}
	return userID, string(b[9 : 9+n])
}

// associatedData mirrors ConnectTokenPrivate::aead (src/token.rs:145-155):
// version info (13 bytes) || protocol id u64 BE || expire timestamp u64 BE.
func associatedData(protocolID, expireTimestamp uint64) []byte {
	ad := make([]byte, 0, len(versionInfo)+16)
	ad = append(ad, versionInfo[:]...)
	ad = binary.BigEndian.AppendUint64(ad, protocolID)
	ad = binary.BigEndian.AppendUint64(ad, expireTimestamp)
	return ad
}

// encodeAddressList mirrors AddressList::write_to (src/token.rs:83-100):
// u32 BE count, then per address a type byte (1 = IPv4 + 4 octets,
// 2 = IPv6 + 16 octets) followed by a u16 BE port.
func encodeAddressList(addrs []netip.AddrPort) ([]byte, error) {
	if len(addrs) == 0 {
		return nil, ErrNoServers
	}
	if len(addrs) > MaxServerAddresses {
		return nil, ErrTooManyServers
	}
	out := binary.BigEndian.AppendUint32(nil, uint32(len(addrs)))
	for _, ap := range addrs {
		a := ap.Addr()
		switch {
		case !ap.IsValid():
			return nil, fmt.Errorf("%w: %v", ErrInvalidAddress, ap)
		case a.Zone() != "":
			// The wire format has no room for an IPv6 zone.
			return nil, fmt.Errorf("%w: zoned address %v", ErrInvalidAddress, ap)
		case a.Is4():
			o := a.As4()
			out = append(out, addressTypeIPv4)
			out = append(out, o[:]...)
		default:
			// Includes IPv4-mapped IPv6, which Rust also treats as SocketAddr::V6.
			o := a.As16()
			out = append(out, addressTypeIPv6)
			out = append(out, o[:]...)
		}
		out = binary.BigEndian.AppendUint16(out, ap.Port())
	}
	return out, nil
}
