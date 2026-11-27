package netcode

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
)

const vectorPath = "../../tests/fixtures/connect_token_vector.json"

type tokenVector struct {
	Description           string   `json:"description"`
	NetcodeVersion        string   `json:"netcode_version"`
	Crate                 string   `json:"crate"`
	Endianness            string   `json:"endianness"`
	PrivateKeyHex         string   `json:"private_key_hex"`
	ProtocolID            uint64   `json:"protocol_id"`
	ClientID              uint64   `json:"client_id"`
	CreateTimestamp       uint64   `json:"create_timestamp"`
	ExpireTimestamp       uint64   `json:"expire_timestamp"`
	TimeoutSeconds        int32    `json:"timeout_seconds"`
	NonceHex              string   `json:"nonce_hex"`
	ServerAddresses       []string `json:"server_addresses"`
	ClientToServerKeyHex  string   `json:"client_to_server_key_hex"`
	ServerToClientKeyHex  string   `json:"server_to_client_key_hex"`
	UserDataUserID        uint64   `json:"user_data_user_id"`
	UserDataServerID      string   `json:"user_data_server_id"`
	UserDataHex           string   `json:"user_data_hex"`
	UserDataLayout        string   `json:"user_data_layout"`
	ExpectedTokenHex      string   `json:"expected_token_hex"`
	ExpectedTokenBase64   string   `json:"expected_token_base64"`
	ExpectedTokenByteSize int      `json:"expected_token_bytes"`
}

func fixedBytes(t *testing.T, name, h string, n int) []byte {
	t.Helper()
	b, err := hex.DecodeString(h)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if len(b) != n {
		t.Fatalf("%s: got %d bytes, want %d", name, len(b), n)
	}
	return b
}

func (v tokenVector) params(t *testing.T) ConnectTokenParams {
	t.Helper()
	p := ConnectTokenParams{
		ProtocolID:      v.ProtocolID,
		ClientID:        v.ClientID,
		CreateTimestamp: v.CreateTimestamp,
		ExpireTimestamp: v.ExpireTimestamp,
		TimeoutSeconds:  v.TimeoutSeconds,
	}
	copy(p.PrivateKey[:], fixedBytes(t, "private_key_hex", v.PrivateKeyHex, KeyBytes))
	copy(p.Nonce[:], fixedBytes(t, "nonce_hex", v.NonceHex, NonceBytes))
	copy(p.ClientToServerKey[:], fixedBytes(t, "client_to_server_key_hex", v.ClientToServerKeyHex, KeyBytes))
	copy(p.ServerToClientKey[:], fixedBytes(t, "server_to_client_key_hex", v.ServerToClientKeyHex, KeyBytes))
	copy(p.UserData[:], fixedBytes(t, "user_data_hex", v.UserDataHex, UserDataBytes))
	for _, s := range v.ServerAddresses {
		ap, err := netip.ParseAddrPort(s)
		if err != nil {
			t.Fatalf("server address %q: %v", s, err)
		}
		p.ServerAddresses = append(p.ServerAddresses, ap)
	}
	return p
}

// decodedToken is the result of fully parsing and decrypting a token. It is
// a Go port of ConnectToken::read_from + ConnectTokenPrivate::decrypt from
// lightyear_netcode 0.28.0 (src/token.rs:172-225, 472-520), test-only.
type decodedToken struct {
	ProtocolID      uint64
	CreateTimestamp uint64
	ExpireTimestamp uint64
	Nonce           [24]byte
	PublicTimeout   int32
	PublicAddresses []netip.AddrPort
	PublicC2SKey    [32]byte
	PublicS2CKey    [32]byte

	ClientID         uint64
	PrivateTimeout   int32
	PrivateAddresses []netip.AddrPort
	PrivateC2SKey    [32]byte
	PrivateS2CKey    [32]byte
	UserData         [UserDataBytes]byte
}

type byteReader struct {
	b   []byte
	off int
	err error
}

func (r *byteReader) next(n int) []byte {
	if r.err != nil {
		return make([]byte, n)
	}
	if r.off+n > len(r.b) {
		r.err = fmt.Errorf("short read at offset %d (need %d of %d)", r.off, n, len(r.b))
		return make([]byte, n)
	}
	s := r.b[r.off : r.off+n]
	r.off += n
	return s
}
func (r *byteReader) u8() uint8   { return r.next(1)[0] }
func (r *byteReader) u16() uint16 { return binary.BigEndian.Uint16(r.next(2)) }
func (r *byteReader) u32() uint32 { return binary.BigEndian.Uint32(r.next(4)) }
func (r *byteReader) u64() uint64 { return binary.BigEndian.Uint64(r.next(8)) }

func (r *byteReader) addresses() []netip.AddrPort {
	n := r.u32()
	if r.err == nil && (n < 1 || n > MaxServerAddresses) {
		r.err = fmt.Errorf("address list length out of range: %d", n)
		return nil
	}
	var out []netip.AddrPort
	for i := uint32(0); i < n && r.err == nil; i++ {
		var a netip.Addr
		switch typ := r.u8(); typ {
		case addressTypeIPv4:
			a = netip.AddrFrom4([4]byte(r.next(4)))
		case addressTypeIPv6:
			a = netip.AddrFrom16([16]byte(r.next(16)))
		default:
			r.err = fmt.Errorf("invalid address type %d", typ)
			return nil
		}
		out = append(out, netip.AddrPortFrom(a, r.u16()))
	}
	return out
}

func decryptConnectToken(token []byte, privateKey [32]byte) (*decodedToken, error) {
	if len(token) != ConnectTokenBytes {
		return nil, fmt.Errorf("token is %d bytes, want %d", len(token), ConnectTokenBytes)
	}
	r := &byteReader{b: token}
	if v := r.next(len(versionInfo)); !bytes.Equal(v, versionInfo[:]) {
		return nil, fmt.Errorf("bad version info %q", v)
	}
	d := &decodedToken{}
	d.ProtocolID = r.u64()
	d.CreateTimestamp = r.u64()
	d.ExpireTimestamp = r.u64()
	if d.CreateTimestamp > d.ExpireTimestamp {
		return nil, errors.New("create timestamp is greater than expire timestamp")
	}
	copy(d.Nonce[:], r.next(NonceBytes))
	sealed := r.next(privateTokenBytes)
	d.PublicTimeout = int32(r.u32())
	d.PublicAddresses = r.addresses()
	copy(d.PublicC2SKey[:], r.next(KeyBytes))
	copy(d.PublicS2CKey[:], r.next(KeyBytes))
	if r.err != nil {
		return nil, r.err
	}
	if !allZero(token[r.off:]) {
		return nil, errors.New("public token padding is not zero")
	}

	aead, err := chacha20poly1305.NewX(privateKey[:])
	if err != nil {
		return nil, err
	}
	plain, err := aead.Open(nil, d.Nonce[:], sealed, associatedData(d.ProtocolID, d.ExpireTimestamp))
	if err != nil {
		return nil, fmt.Errorf("decrypt private token: %w", err)
	}
	if len(plain) != privateTokenBytes-macBytes {
		return nil, fmt.Errorf("private plaintext is %d bytes", len(plain))
	}
	pr := &byteReader{b: plain}
	d.ClientID = pr.u64()
	d.PrivateTimeout = int32(pr.u32())
	d.PrivateAddresses = pr.addresses()
	copy(d.PrivateC2SKey[:], pr.next(KeyBytes))
	copy(d.PrivateS2CKey[:], pr.next(KeyBytes))
	copy(d.UserData[:], pr.next(UserDataBytes))
	if pr.err != nil {
		return nil, pr.err
	}
	if !allZero(plain[pr.off:]) {
		return nil, errors.New("private token padding is not zero")
	}
	return d, nil
}

func allZero(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}

func checkDecoded(t *testing.T, d *decodedToken, p ConnectTokenParams) {
	t.Helper()
	check := func(name string, got, want any) {
		t.Helper()
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%s: got %v, want %v", name, got, want)
		}
	}
	check("protocol id", d.ProtocolID, p.ProtocolID)
	check("create timestamp", d.CreateTimestamp, p.CreateTimestamp)
	check("expire timestamp", d.ExpireTimestamp, p.ExpireTimestamp)
	check("nonce", d.Nonce, p.Nonce)
	check("public timeout", d.PublicTimeout, p.TimeoutSeconds)
	check("public addresses", d.PublicAddresses, p.ServerAddresses)
	check("public c2s key", d.PublicC2SKey, p.ClientToServerKey)
	check("public s2c key", d.PublicS2CKey, p.ServerToClientKey)
	check("client id", d.ClientID, p.ClientID)
	check("private timeout", d.PrivateTimeout, p.TimeoutSeconds)
	check("private addresses", d.PrivateAddresses, p.ServerAddresses)
	check("private c2s key", d.PrivateC2SKey, p.ClientToServerKey)
	check("private s2c key", d.PrivateS2CKey, p.ServerToClientKey)
	check("user data", d.UserData, p.UserData)
}

func seq(start byte, n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = start + byte(i)
	}
	return b
}

// TestWriteVector regenerates the fixture. Run with
// NETCODE_WRITE_VECTOR=1 go test ./backend/netcode -run TestWriteVector
func TestWriteVector(t *testing.T) {
	if os.Getenv("NETCODE_WRITE_VECTOR") == "" {
		t.Skip("set NETCODE_WRITE_VECTOR=1 to regenerate the fixture")
	}
	ud := UserData(42, "AbCd1234")
	v := tokenVector{
		Description: "netcode.io connect token minted by backend/netcode (Go) for lightyear_netcode (Rust). " +
			"Inputs are fixed; expected_token_* must be reproduced byte-for-byte by EncodeConnectToken, " +
			"and the Rust side must accept it via ConnectToken::try_from_bytes and decrypt the private part " +
			"with ConnectTokenPrivate::decrypt using private_key_hex. Server address list is identical in the public and private parts.",
		NetcodeVersion:        "NETCODE 1.02",
		Crate:                 "lightyear_netcode 0.28.0",
		Endianness:            "all netcode integer fields (protocol id, timestamps, timeout, address count, ports, client id, AEAD associated data) are BIG-endian, following lightyear_serde 0.28.0 WriteInteger; upstream netcode.io 1.02 uses little-endian, so this token is NOT upstream-compatible",
		PrivateKeyHex:         hex.EncodeToString(seq(0x01, 32)),
		ProtocolID:            0x1122334455,
		ClientID:              123456789,
		CreateTimestamp:       1700000000,
		ExpireTimestamp:       1700000030,
		TimeoutSeconds:        DefaultTimeoutSeconds,
		NonceHex:              hex.EncodeToString(seq(0xa0, 24)),
		ServerAddresses:       []string{"203.0.113.7:7001"},
		ClientToServerKeyHex:  hex.EncodeToString(seq(0x40, 32)),
		ServerToClientKeyHex:  hex.EncodeToString(seq(0x60, 32)),
		UserDataUserID:        42,
		UserDataServerID:      "AbCd1234",
		UserDataHex:           hex.EncodeToString(ud[:]),
		UserDataLayout:        "256 bytes: bytes 0..8 = user id u64 little-endian; byte 8 = length N of the server id (N <= 64); bytes 9..9+N = server id ASCII; remaining bytes zero",
		ExpectedTokenByteSize: ConnectTokenBytes,
	}
	tok, err := EncodeConnectToken(v.params(t))
	if err != nil {
		t.Fatal(err)
	}
	v.ExpectedTokenHex = hex.EncodeToString(tok)
	v.ExpectedTokenBase64 = base64.StdEncoding.EncodeToString(tok)
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(vectorPath, append(out, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func loadVector(t *testing.T) tokenVector {
	t.Helper()
	raw, err := os.ReadFile(vectorPath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var v tokenVector
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return v
}

func TestConnectTokenVector(t *testing.T) {
	v := loadVector(t)
	if v.NetcodeVersion != "NETCODE 1.02" || !strings.HasPrefix(v.Crate, "lightyear_netcode 0.28") {
		t.Fatalf("unexpected fixture header: %q %q", v.NetcodeVersion, v.Crate)
	}
	p := v.params(t)

	want, err := hex.DecodeString(v.ExpectedTokenHex)
	if err != nil {
		t.Fatal(err)
	}
	want64, err := base64.StdEncoding.DecodeString(v.ExpectedTokenBase64)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, want64) {
		t.Fatal("expected_token_hex and expected_token_base64 disagree")
	}
	if len(want) != ConnectTokenBytes || v.ExpectedTokenByteSize != ConnectTokenBytes {
		t.Fatalf("expected token is %d bytes", len(want))
	}

	got, err := EncodeConnectToken(p)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("token differs from fixture at byte %d: got %02x want %02x", i, got[i], want[i])
			}
		}
	}

	// Spot check the fixed layout against the crate source.
	if !bytes.Equal(got[:13], []byte("NETCODE 1.02\x00")) {
		t.Errorf("version info: %q", got[:13])
	}
	if binary.BigEndian.Uint64(got[13:21]) != v.ProtocolID {
		t.Error("protocol id is not big-endian at offset 13")
	}
	if !bytes.Equal(got[37:61], p.Nonce[:]) {
		t.Error("nonce is not at offset 37")
	}
	if int32(binary.BigEndian.Uint32(got[1085:1089])) != v.TimeoutSeconds {
		t.Error("public timeout is not at offset 1085")
	}

	d, err := decryptConnectToken(got, p.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	checkDecoded(t, d, p)

	uid, sid := ParseUserData(d.UserData)
	if uid != v.UserDataUserID || sid != v.UserDataServerID {
		t.Errorf("user data: got (%d, %q), want (%d, %q)", uid, sid, v.UserDataUserID, v.UserDataServerID)
	}
	if UserData(v.UserDataUserID, v.UserDataServerID) != p.UserData {
		t.Error("user_data_hex does not match UserData(user_data_user_id, user_data_server_id)")
	}

	// Tampering with the AAD inputs or the key must break decryption.
	var wrongKey [32]byte
	if _, err := decryptConnectToken(got, wrongKey); err == nil {
		t.Error("decrypt with wrong key succeeded")
	}
	tampered := bytes.Clone(got)
	tampered[29+7] ^= 1 // expire timestamp is part of the associated data
	if _, err := decryptConnectToken(tampered, p.PrivateKey); err == nil {
		t.Error("decrypt with modified expire timestamp succeeded")
	}
	tampered = bytes.Clone(got)
	tampered[13+7] ^= 1 // protocol id is part of the associated data
	if _, err := decryptConnectToken(tampered, p.PrivateKey); err == nil {
		t.Error("decrypt with modified protocol id succeeded")
	}
}

func TestEncodeMixedAddresses(t *testing.T) {
	v := loadVector(t)
	p := v.params(t)
	p.ServerAddresses = nil
	for i := 0; i < MaxServerAddresses; i++ {
		if i%2 == 0 {
			p.ServerAddresses = append(p.ServerAddresses, netip.AddrPortFrom(netip.AddrFrom4([4]byte{10, 0, 0, byte(i)}), uint16(7000+i)))
		} else {
			p.ServerAddresses = append(p.ServerAddresses, netip.MustParseAddrPort(fmt.Sprintf("[2001:db8::%x]:%d", i, 7000+i)))
		}
	}
	tok, err := EncodeConnectToken(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(tok) != ConnectTokenBytes {
		t.Fatalf("token is %d bytes", len(tok))
	}
	d, err := decryptConnectToken(tok, p.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	checkDecoded(t, d, p)
}

func TestNewConnectToken(t *testing.T) {
	key := seq(0x11, KeyBytes)
	servers := []netip.AddrPort{netip.MustParseAddrPort("127.0.0.1:5000")}
	ud := UserData(7, "srv-1")
	before := time.Now().Unix()
	tok, exp, err := NewConnectToken(key, 99, 1234, DefaultTokenTTL, DefaultTimeoutSeconds, servers, ud)
	if err != nil {
		t.Fatal(err)
	}
	after := time.Now().Unix()
	if len(tok) != ConnectTokenBytes {
		t.Fatalf("token is %d bytes", len(tok))
	}
	d, err := decryptConnectToken(tok, [32]byte(key))
	if err != nil {
		t.Fatal(err)
	}
	if d.ClientID != 1234 || d.ProtocolID != 99 || d.PrivateTimeout != DefaultTimeoutSeconds || d.UserData != ud {
		t.Errorf("unexpected decoded token: %+v", d)
	}
	if int64(d.CreateTimestamp) < before || int64(d.CreateTimestamp) > after {
		t.Errorf("create timestamp %d not in [%d, %d]", d.CreateTimestamp, before, after)
	}
	if d.ExpireTimestamp != d.CreateTimestamp+30 || exp.Unix() != int64(d.ExpireTimestamp) {
		t.Errorf("expire timestamp %d, expiresAt %v, create %d", d.ExpireTimestamp, exp, d.CreateTimestamp)
	}
	if d.PublicC2SKey != d.PrivateC2SKey || d.PublicS2CKey != d.PrivateS2CKey || d.PrivateC2SKey == d.PrivateS2CKey {
		t.Error("session keys mismatch or not random")
	}

	tok2, _, err := NewConnectToken(key, 99, 1234, DefaultTokenTTL, DefaultTimeoutSeconds, servers, ud)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(tok[37:61], tok2[37:61]) {
		t.Error("nonce was reused across tokens")
	}
}

func TestUserDataRoundTrip(t *testing.T) {
	cases := []struct {
		uid uint64
		sid string
	}{
		{0, ""},
		{42, "AbCd1234"},
		{^uint64(0), strings.Repeat("x", MaxServerIDBytes)},
	}
	for _, c := range cases {
		b := UserData(c.uid, c.sid)
		uid, sid := ParseUserData(b)
		if uid != c.uid || sid != c.sid {
			t.Errorf("round trip (%d, %q) -> (%d, %q)", c.uid, c.sid, uid, sid)
		}
		if int(b[8]) != len(c.sid) || !allZero(b[9+len(c.sid):]) {
			t.Errorf("bad layout for %q", c.sid)
		}
	}

	b := UserData(42, "AbCd1234")
	want := append([]byte{42, 0, 0, 0, 0, 0, 0, 0, 8}, "AbCd1234"...)
	if !bytes.Equal(b[:len(want)], want) {
		t.Errorf("layout: % x", b[:len(want)])
	}

	// Too long server ids are truncated to 64 bytes.
	_, sid := ParseUserData(UserData(1, strings.Repeat("y", 100)))
	if sid != strings.Repeat("y", MaxServerIDBytes) {
		t.Errorf("truncation: got %d bytes", len(sid))
	}
	// A corrupt length byte is clamped.
	b[8] = 200
	if _, sid := ParseUserData(b); len(sid) != MaxServerIDBytes {
		t.Errorf("clamp: got %d bytes", len(sid))
	}
}

func TestEncodeErrors(t *testing.T) {
	v := loadVector(t)
	base := v.params(t)

	p := base
	p.ServerAddresses = nil
	if _, err := EncodeConnectToken(p); !errors.Is(err, ErrNoServers) {
		t.Errorf("no servers: %v", err)
	}

	p = base
	p.ServerAddresses = make([]netip.AddrPort, MaxServerAddresses+1)
	for i := range p.ServerAddresses {
		p.ServerAddresses[i] = netip.MustParseAddrPort("10.0.0.1:1")
	}
	if _, err := EncodeConnectToken(p); !errors.Is(err, ErrTooManyServers) {
		t.Errorf("too many servers: %v", err)
	}

	p = base
	p.ServerAddresses = []netip.AddrPort{{}}
	if _, err := EncodeConnectToken(p); !errors.Is(err, ErrInvalidAddress) {
		t.Errorf("invalid address: %v", err)
	}

	p = base
	p.ServerAddresses = []netip.AddrPort{netip.MustParseAddrPort("[fe80::1%eth0]:1")}
	if _, err := EncodeConnectToken(p); !errors.Is(err, ErrInvalidAddress) {
		t.Errorf("zoned address: %v", err)
	}

	p = base
	p.CreateTimestamp = p.ExpireTimestamp + 1
	if _, err := EncodeConnectToken(p); !errors.Is(err, ErrInvalidTimestamps) {
		t.Errorf("timestamps: %v", err)
	}

	servers := []netip.AddrPort{netip.MustParseAddrPort("127.0.0.1:5000")}
	for _, n := range []int{0, 16, 31, 33, 64} {
		if _, _, err := NewConnectToken(make([]byte, n), 1, 1, time.Minute, 15, servers, [UserDataBytes]byte{}); !errors.Is(err, ErrInvalidKeyLength) {
			t.Errorf("key length %d: %v", n, err)
		}
	}
	if _, _, err := NewConnectToken(make([]byte, KeyBytes), 1, 1, 500*time.Millisecond, 15, servers, [UserDataBytes]byte{}); !errors.Is(err, ErrInvalidTTL) {
		t.Errorf("short ttl: %v", err)
	}
	if _, _, err := NewConnectToken(make([]byte, KeyBytes), 1, 1, time.Minute, 15, nil, [UserDataBytes]byte{}); !errors.Is(err, ErrNoServers) {
		t.Errorf("new without servers: %v", err)
	}
}
