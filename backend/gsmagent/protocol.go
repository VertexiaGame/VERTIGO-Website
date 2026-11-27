// Package gsmagent holds everything shared between the platform and the
// gameserver host agent (cmd/gsm-agent): the JSON protocol, HMAC request
// signing and verification, nonce replay protection and the HTTP client the
// platform uses to drive the agent.
package gsmagent

import "time"

// Protocol paths. Every request, including health checks, must be signed.
const (
	PathServers = "/v1/servers"
	PathHealth  = "/v1/health"
)

// Agent-side server states reported in Server.Status.
const (
	StatusStarting   = "starting"
	StatusRunning    = "running"
	StatusRestarting = "restarting"
	StatusStopping   = "stopping"
	StatusFailed     = "failed"
)

// Input limits enforced by the agent.
const (
	MinMaxPlayers = 1
	MaxMaxPlayers = 1000
	// MaxRequestBody bounds signed request bodies read by the middleware.
	MaxRequestBody = 64 << 10
	// maxResponseBody bounds responses read by the client.
	maxResponseBody = 1 << 20
)

// CreateServerRequest is the body of POST /v1/servers. The platform chooses
// the 8-character server id; the agent chooses the UDP port.
type CreateServerRequest struct {
	ServerID   string `json:"server_id"`
	GameID     int    `json:"game_id"`
	Map        string `json:"map,omitempty"`
	MaxPlayers int    `json:"max_players"`
}

// Server is the agent's view of one gameserver process.
type Server struct {
	ID         string    `json:"id"`
	Port       int       `json:"port"`
	PID        int       `json:"pid"`
	Status     string    `json:"status"`
	GameID     int       `json:"game_id"`
	MaxPlayers int       `json:"max_players"`
	StartedAt  time.Time `json:"started_at"`
	Restarts   int       `json:"restarts"`
}

// Alive reports whether the agent still considers the process usable. A
// server that is being restarted on the same port counts as alive.
func (s Server) Alive() bool {
	switch s.Status {
	case StatusStarting, StatusRunning, StatusRestarting:
		return true
	}
	return false
}

// ListServersResponse is the body of GET /v1/servers.
type ListServersResponse struct {
	Servers []Server `json:"servers"`
}

// StopServerResponse is the body of DELETE /v1/servers/{id}.
type StopServerResponse struct {
	Stopped bool `json:"stopped"`
}

// Health is the body of GET /v1/health.
type Health struct {
	OK       bool   `json:"ok"`
	Servers  int    `json:"servers"`
	Capacity int    `json:"capacity"`
	Version  string `json:"version"`
	AgentID  string `json:"agent_id,omitempty"`
}

// ErrorResponse is the body of every non-2xx response.
type ErrorResponse struct {
	Error string `json:"error"`
}

// ErrorCapacity is the ErrorResponse.Error value of a 409 meaning the agent
// has no free port or reached its server limit.
const ErrorCapacity = "capacity"
