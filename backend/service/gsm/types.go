package gsm

import "time"

const (
	StatusStarting = "starting"
	StatusActive   = "active"
	StatusDraining = "draining"
)

type GameServer struct {
	ID          string    `json:"id"`
	GameID      int       `json:"game_id"`
	Host        string    `json:"host"`
	Port        int       `json:"port"`
	PID         int       `json:"pid"`
	Status      string    `json:"status"`
	PlayerCount int       `json:"player_count"`
	MaxPlayers  int       `json:"max_players"`
	CreatedAt   time.Time `json:"created_at"`
	LastBeat    time.Time `json:"last_beat"`
	EmptySince  time.Time `json:"empty_since"`
	DrainingAt  time.Time `json:"draining_at"`
	// LastHeartbeat is only advanced by the gameserver's own reports
	// (Heartbeat, ServerJoin, SyncRoster). Zero until the first report.
	LastHeartbeat time.Time `json:"last_heartbeat"`
}

type GameInfo struct {
	ID         int
	MaxPlayers int
	Private    bool
	Locked     bool
	Owner      string
}

type GameResolver func(gameID int) (GameInfo, error)

type Config struct {
	BindHost          string
	PublicHost        string
	PortMin           int
	PortMax           int
	MaxPlayersDefault int
	MinPlayersDefault int
	IdleTimeout       time.Duration
	ReapInterval      time.Duration
	ShutdownGrace     time.Duration
	ServerBin         string
	ServerWorkdir     string
	ServerMap         string
	LogsDir           string

	// AgentURL switches the manager to remote mode: gameservers run on a
	// dedicated host managed by gsm-agent instead of as local children.
	AgentURL     string
	AgentSecret  string
	AgentTimeout time.Duration
	// HeartbeatTimeout evicts an active server whose last heartbeat (or
	// creation time when it never reported) is older than this. 0 disables.
	HeartbeatTimeout time.Duration
	// MaxServers caps the number of gameserver records. 0 means unlimited.
	MaxServers int
	// ReadyTimeout bounds how long a launch may take to become ready.
	ReadyTimeout time.Duration
	// NetcodeKeyHex and NetcodeProtocolID are passed to local children.
	NetcodeKeyHex     string
	NetcodeProtocolID uint64
	// APIKey is the GAMESERVER_API_KEY given to local children.
	APIKey string
}

func DefaultConfig() Config {
	return Config{
		BindHost:          "127.0.0.1",
		PublicHost:        "127.0.0.1",
		PortMin:           5100,
		PortMax:           5250,
		MaxPlayersDefault: 100,
		MinPlayersDefault: 2,
		IdleTimeout:       5 * time.Minute,
		ReapInterval:      15 * time.Second,
		ShutdownGrace:     10 * time.Second,
		ServerMap:         "assets/maps/default.vrtx",
		LogsDir:           "./logs/gsm",
		AgentTimeout:      10 * time.Second,
		HeartbeatTimeout:  90 * time.Second,
		ReadyTimeout:      30 * time.Second,
	}
}

func (s GameServer) IsJoinable(now time.Time) bool {
	if s.Status == StatusActive {
		return s.PlayerCount < s.MaxPlayers
	}
	if s.Status == StatusStarting {
		return now.Sub(s.CreatedAt) < time.Minute && s.PlayerCount < s.MaxPlayers
	}
	return false
}
