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
}

type GameInfo struct {
	ID         int
	MaxPlayers int
	Private    bool
	Locked     bool
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
