package gsmagent

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	// ErrCapacity: the agent has no free port or reached its server limit.
	ErrCapacity = errors.New("gsmagent: agent has no capacity")
	// ErrNotReady: the gameserver exited or did not become ready in time.
	ErrNotReady = errors.New("gsmagent: gameserver did not become ready")
	// ErrNotFound: the agent does not know the server id.
	ErrNotFound = errors.New("gsmagent: server not found")
	// ErrExists: the server id is already in use on the agent. CreateServer
	// returns the existing record alongside this error.
	ErrExists = errors.New("gsmagent: server id already exists")
	// ErrUnauthorized: the agent rejected the request signature.
	ErrUnauthorized = errors.New("gsmagent: request rejected by agent authentication")
)

// APIError is returned for unexpected agent responses.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("gsmagent: agent returned %d: %s", e.Status, e.Message)
}

// Client talks to a gsm-agent over HTTPS (or loopback HTTP behind a tunnel).
type Client struct {
	base    *url.URL
	secret  []byte
	timeout time.Duration
	http    *http.Client
	now     func() time.Time
}

// NewClient builds a client. timeout bounds every call except CreateServer,
// which waits for gameserver readiness and is bounded by the caller's context.
func NewClient(baseURL string, secret []byte, timeout time.Duration) (*Client, error) {
	u, err := url.Parse(strings.TrimSuffix(strings.TrimSpace(baseURL), "/"))
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, errors.New("gsmagent: agent URL must be an absolute http(s) URL")
	}
	if u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return nil, errors.New("gsmagent: agent URL must not contain credentials, a query or a fragment")
	}
	if len(secret) < MinSecretLen {
		return nil, fmt.Errorf("gsmagent: secret must be at least %d bytes", MinSecretLen)
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	transport.ForceAttemptHTTP2 = true
	transport.MaxIdleConnsPerHost = 8
	return &Client{
		base:    u,
		secret:  append([]byte(nil), secret...),
		timeout: timeout,
		http: &http.Client{
			Transport: transport,
			// Never follow redirects: the signature covers the exact path.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		now: time.Now,
	}, nil
}

// Timeout returns the per-call timeout used for quick calls.
func (c *Client) Timeout() time.Duration { return c.timeout }

// BaseURL returns the configured agent URL.
func (c *Client) BaseURL() string { return c.base.String() }

func (c *Client) do(ctx context.Context, method, path string, in any, out any, quick bool) (int, []byte, error) {
	if quick {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}
	var body []byte
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return 0, nil, err
		}
		body = raw
	}
	u := *c.base
	u.Path = strings.TrimSuffix(c.base.Path, "/") + path
	u.RawPath = ""
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	Sign(req, body, c.secret, c.now())
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if out != nil {
			if err := json.Unmarshal(raw, out); err != nil {
				return resp.StatusCode, raw, fmt.Errorf("gsmagent: bad response body: %w", err)
			}
		}
		return resp.StatusCode, raw, nil
	}
	return resp.StatusCode, raw, statusError(resp.StatusCode, raw)
}

func statusError(status int, raw []byte) error {
	var e ErrorResponse
	_ = json.Unmarshal(raw, &e)
	msg := strings.TrimSpace(e.Error)
	if msg == "" {
		msg = http.StatusText(status)
	}
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrUnauthorized
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusServiceUnavailable:
		return fmt.Errorf("%w: %s", ErrNotReady, msg)
	case http.StatusConflict:
		if msg == ErrorCapacity {
			return ErrCapacity
		}
		return ErrExists
	}
	return &APIError{Status: status, Message: msg}
}

// CreateServer launches a gameserver and returns once it is ready. When the
// id already exists the existing record is returned together with ErrExists.
func (c *Client) CreateServer(ctx context.Context, req CreateServerRequest) (Server, error) {
	var out Server
	_, raw, err := c.do(ctx, http.MethodPost, PathServers, req, &out, false)
	if errors.Is(err, ErrExists) {
		var existing Server
		if jerr := json.Unmarshal(raw, &existing); jerr == nil && existing.ID != "" {
			return existing, ErrExists
		}
	}
	if err != nil {
		return Server{}, err
	}
	return out, nil
}

// GetServer returns one server record.
func (c *Client) GetServer(ctx context.Context, id string) (Server, error) {
	var out Server
	if _, _, err := c.do(ctx, http.MethodGet, PathServers+"/"+url.PathEscape(id), nil, &out, true); err != nil {
		return Server{}, err
	}
	return out, nil
}

// ListServers returns every server the agent tracks, including failed ones.
func (c *Client) ListServers(ctx context.Context) ([]Server, error) {
	var out ListServersResponse
	if _, _, err := c.do(ctx, http.MethodGet, PathServers, nil, &out, true); err != nil {
		return nil, err
	}
	if out.Servers == nil {
		out.Servers = []Server{}
	}
	return out.Servers, nil
}

// StopServer stops a server (SIGTERM, then SIGKILL after the agent's grace).
// The call may take up to the agent's shutdown grace, so the caller's context
// bounds it in addition to the client timeout plus that grace.
func (c *Client) StopServer(ctx context.Context, id string) error {
	var out StopServerResponse
	_, _, err := c.do(ctx, http.MethodDelete, PathServers+"/"+url.PathEscape(id), nil, &out, false)
	return err
}

// Health returns the agent's health summary.
func (c *Client) Health(ctx context.Context) (Health, error) {
	var out Health
	if _, _, err := c.do(ctx, http.MethodGet, PathHealth, nil, &out, true); err != nil {
		return Health{}, err
	}
	return out, nil
}
