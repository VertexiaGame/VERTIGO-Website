package gsmagent

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// Authentication headers.
const (
	HeaderTimestamp = "X-GSM-Timestamp"
	HeaderNonce     = "X-GSM-Nonce"
	HeaderSignature = "X-GSM-Signature"
)

const (
	// MaxSkew is the largest accepted difference between the request
	// timestamp and the verifier's clock, in either direction.
	MaxSkew = 30 * time.Second
	// NonceTTL is how long a nonce is remembered. It must cover at least
	// twice the skew window so a replay can never fall outside both checks.
	NonceTTL = 2*MaxSkew + 30*time.Second
	// MinSecretLen is the minimum shared secret length accepted by the
	// client and the agent.
	MinSecretLen = 32
	nonceBytes   = 16
)

var (
	ErrMissingAuth  = errors.New("gsmagent: missing authentication headers")
	ErrBadTimestamp = errors.New("gsmagent: malformed timestamp")
	ErrClockSkew    = errors.New("gsmagent: timestamp outside allowed skew")
	ErrBadNonce     = errors.New("gsmagent: malformed nonce")
	ErrBadSignature = errors.New("gsmagent: invalid signature")
	ErrReplay       = errors.New("gsmagent: nonce already used")
	ErrNoSecret     = errors.New("gsmagent: secret is not configured")
)

// CanonicalString builds the string covered by the signature:
// METHOD \n path-with-raw-query \n timestamp \n nonce \n hex(sha256(body)).
func CanonicalString(method, requestURI, timestamp, nonce string, body []byte) string {
	sum := sha256.Sum256(body)
	return method + "\n" + requestURI + "\n" + timestamp + "\n" + nonce + "\n" + hex.EncodeToString(sum[:])
}

func computeSignature(secret []byte, canonical string) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(canonical))
	return mac.Sum(nil)
}

func newNonce() string {
	buf := make([]byte, nonceBytes)
	// crypto/rand.Read never returns an error on supported platforms.
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

// Sign stamps req with a fresh timestamp, nonce and signature over body. The
// path is taken from req.URL.RequestURI(), which is what the Go HTTP client
// writes on the wire.
func Sign(req *http.Request, body []byte, secret []byte, now time.Time) {
	ts := strconv.FormatInt(now.Unix(), 10)
	nonce := newNonce()
	sig := computeSignature(secret, CanonicalString(req.Method, req.URL.RequestURI(), ts, nonce, body))
	req.Header.Set(HeaderTimestamp, ts)
	req.Header.Set(HeaderNonce, nonce)
	req.Header.Set(HeaderSignature, hex.EncodeToString(sig))
}

func requestTarget(r *http.Request) string {
	if r.RequestURI != "" {
		return r.RequestURI
	}
	return r.URL.RequestURI()
}

func validNonce(n string) bool {
	if len(n) != 2*nonceBytes {
		return false
	}
	for i := 0; i < len(n); i++ {
		c := n[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// Verify checks the authentication headers of r against body. The nonce is
// only recorded after the signature is proven valid so unauthenticated
// callers cannot fill the nonce store.
func Verify(r *http.Request, body []byte, secret []byte, nonces NonceStore, now time.Time) error {
	if len(secret) == 0 {
		return ErrNoSecret
	}
	ts := r.Header.Get(HeaderTimestamp)
	nonce := r.Header.Get(HeaderNonce)
	sigHex := r.Header.Get(HeaderSignature)
	if ts == "" || nonce == "" || sigHex == "" {
		return ErrMissingAuth
	}
	unix, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || strconv.FormatInt(unix, 10) != ts {
		return ErrBadTimestamp
	}
	diff := now.Sub(time.Unix(unix, 0))
	if diff > MaxSkew || diff < -MaxSkew {
		return ErrClockSkew
	}
	if !validNonce(nonce) {
		return ErrBadNonce
	}
	got, err := hex.DecodeString(sigHex)
	if err != nil || len(got) != sha256.Size {
		return ErrBadSignature
	}
	want := computeSignature(secret, CanonicalString(r.Method, requestTarget(r), ts, nonce, body))
	if !hmac.Equal(got, want) {
		return ErrBadSignature
	}
	if nonces == nil {
		return errors.New("gsmagent: nonce store is not configured")
	}
	ctx := r.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	fresh, err := nonces.Remember(ctx, nonce, NonceTTL)
	if err != nil {
		return err
	}
	if !fresh {
		return ErrReplay
	}
	return nil
}

// Middleware authenticates every request with Verify before calling next.
// The body is buffered (bounded by MaxRequestBody) and restored for next.
func Middleware(secret []byte, nonces NonceStore, logger *slog.Logger, next http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, MaxRequestBody+1))
		_ = r.Body.Close()
		if err != nil {
			writeAuthError(w, http.StatusBadRequest, "unreadable body")
			return
		}
		if len(body) > MaxRequestBody {
			writeAuthError(w, http.StatusRequestEntityTooLarge, "body too large")
			return
		}
		if err := Verify(r, body, secret, nonces, time.Now()); err != nil {
			status := http.StatusUnauthorized
			if !isAuthError(err) {
				status = http.StatusServiceUnavailable
			}
			logger.Warn("gsm-agent request rejected", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr, "reason", err.Error())
			if status == http.StatusUnauthorized {
				writeAuthError(w, status, "unauthorized")
			} else {
				writeAuthError(w, status, "authentication unavailable")
			}
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		r.ContentLength = int64(len(body))
		next.ServeHTTP(w, r)
	})
}

func isAuthError(err error) bool {
	for _, e := range []error{ErrMissingAuth, ErrBadTimestamp, ErrClockSkew, ErrBadNonce, ErrBadSignature, ErrReplay, ErrNoSecret} {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}

func writeAuthError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: msg})
}
