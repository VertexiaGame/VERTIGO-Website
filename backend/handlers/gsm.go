package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/config"
	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/service"
	"vertexia-frontend/backend/service/gsm"
)

type gsView struct {
	ID          string     `json:"id"`
	GameID      int        `json:"game_id"`
	Host        string     `json:"host"`
	Port        int        `json:"port"`
	Status      string     `json:"status"`
	PlayerCount int        `json:"player_count"`
	MaxPlayers  int        `json:"max_players"`
	CreatedUnix int64      `json:"created_unix"`
	EmptyUnix   int64      `json:"empty_unix"`
	Players     []gsPlayer `json:"players,omitempty"`
}

type gsPlayer struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Headshot string `json:"headshot"`
}

func serverRoster(id string) ([]gsPlayer, bool) {
	if service.GSM == nil || service.User == nil {
		return nil, false
	}
	members, err := service.GSM.Store().ConfirmedMembers(id)
	if err != nil {
		return nil, false
	}
	if len(members) == 0 {
		return nil, true
	}
	users, err := service.User.GetUsersByUsernames(members)
	if err != nil {
		return nil, false
	}
	players := make([]gsPlayer, 0, len(users))
	for _, u := range users {
		if u == nil {
			continue
		}
		players = append(players, gsPlayer{
			ID:       u.ID,
			Username: u.Username,
			Headshot: "/api/v1/avatar/headshot/" + strconv.Itoa(u.ID),
		})
	}
	sort.Slice(players, func(i, j int) bool { return players[i].Username < players[j].Username })
	return players, true
}

func toGSView(s gsm.GameServer) gsView {
	v := gsView{
		ID:          s.ID,
		GameID:      s.GameID,
		Host:        s.Host,
		Port:        s.Port,
		Status:      s.Status,
		PlayerCount: s.PlayerCount,
		MaxPlayers:  s.MaxPlayers,
		CreatedUnix: s.CreatedAt.Unix(),
	}
	if !s.EmptySince.IsZero() {
		v.EmptyUnix = s.EmptySince.Unix()
	}
	return v
}

// clientLaunchURI is the vertexia-client:// deep link the website opens. It
// carries only short-lived, server-specific credentials (the netcode connect
// token) and a one-time code the client can exchange at /api/v1/auth/exchange
// for its own access token. It never carries the account Unikey.
func clientLaunchURI(gameID int, ticket joinTicket, code string) string {
	q := url.Values{}
	q.Set("game_id", strconv.Itoa(gameID))
	q.Set("server_id", ticket.ServerID)
	q.Set("host", ticket.Host)
	q.Set("ip", ticket.Host)
	q.Set("port", strconv.Itoa(ticket.Port))
	q.Set("connect_token", ticket.ConnectToken)
	q.Set("expires_at", strconv.FormatInt(ticket.ExpiresAt, 10))
	if code != "" {
		q.Set("code", code)
	}
	if config.Global != nil && config.Global.PublicBaseURL != "" {
		q.Set("api", config.Global.PublicBaseURL)
	}
	return "vertexia-client://play?" + q.Encode()
}

func launchCode(user *models.User) string {
	if service.StudioAuth == nil || user == nil {
		return ""
	}
	code, err := service.StudioAuth.GenerateCode(user.ID)
	if err != nil {
		return ""
	}
	return code
}

// GameJoinPost implements POST /api/v1/games/:id/join. The native client
// authenticates with a Bearer access token; the website uses its session.
func GameJoinPost(c fiber.Ctx) error {
	user, err := requestUser(c)
	if err != nil || user == nil {
		return unauthorizedJSON(c)
	}
	gameID, err := strconv.Atoi(c.Params("id"))
	if err != nil || gameID <= 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not_found"})
	}
	server, redirected, ticket, err := joinGameForUser(user, gameID, nil)
	if err != nil {
		log.Printf("GSM join game %d by %s failed: %v", gameID, user.Username, err)
		return writeJoinFailure(c, err)
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	body := fiber.Map{
		"server_id":     ticket.ServerID,
		"host":          ticket.Host,
		"port":          ticket.Port,
		"connect_token": ticket.ConnectToken,
		"expires_at":    ticket.ExpiresAt,
	}
	if !hasAccessToken(c) {
		// Website callers additionally get the launch deep link and the
		// legacy fields the game page script reads.
		body["success"] = true
		body["redirected"] = redirected
		body["server"] = toGSView(server)
		body["client_uri"] = clientLaunchURI(gameID, ticket, launchCode(user))
	}
	return c.JSON(body)
}

func GameJoinProgressPost(c fiber.Ctx) error {
	user, err := requestUser(c)
	if err != nil || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Please log in to play"})
	}
	gameID, err := strconv.Atoi(c.Params("id"))
	if err != nil || gameID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid game ID"})
	}
	if service.User == nil || service.Game == nil || service.GSM == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Gameservers are currently unavailable"})
	}
	c.Set("Content-Type", "application/x-ndjson")
	c.Set("Cache-Control", "no-store, no-transform")
	c.Set("X-Accel-Buffering", "no")
	return c.SendStreamWriter(func(w *bufio.Writer) {
		send := func(value fiber.Map) {
			data, e := json.Marshal(value)
			if e == nil {
				_, _ = fmt.Fprintf(w, "%s\n", data)
				_ = w.Flush()
			}
		}
		send(fiber.Map{"stage": "looking"})
		_, _, ticket, err := joinGameForUser(user, gameID, func(stage string) { send(fiber.Map{"stage": stage}) })
		if err != nil {
			msg := "Could not join game. Please try again."
			switch status, _ := joinFailure(err); status {
			case fiber.StatusNotFound:
				msg = "Game not found"
			case fiber.StatusConflict:
				msg = "All gameservers are full right now. Please try again shortly."
			default:
				msg = "Could not start a gameserver. Please try again shortly."
			}
			log.Printf("GSM join game %d by %s failed: %v", gameID, user.Username, err)
			send(fiber.Map{"error": msg})
			return
		}
		send(fiber.Map{"stage": "joining", "client_uri": clientLaunchURI(gameID, ticket, launchCode(user))})
	})
}

func GameServersGet(c fiber.Ctx) error {
	gameID, err := strconv.Atoi(c.Params("id"))
	if err != nil || gameID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid game ID"})
	}
	if service.Game == nil || service.GSM == nil {
		return c.JSON(fiber.Map{"servers": []gsView{}, "total": 0})
	}
	game, err := service.Game.GetGameByID(gameID)
	if err != nil || game == nil || !game.VisibleTo(activeUserID(c)) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Game not found"})
	}
	servers, err := service.GSM.Servers(gameID)
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Gameservers are currently unavailable"})
	}
	views := make([]gsView, 0, len(servers))
	for _, s := range servers {
		view := toGSView(s)
		if players, ok := serverRoster(s.ID); ok {
			view.Players = players
			view.PlayerCount = len(players)
		}
		views = append(views, view)
	}
	return c.JSON(fiber.Map{"servers": views, "total": len(views)})
}

func GameserverLeavePost(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Please log in"})
	}
	if service.GSM == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Gameservers are currently unavailable"})
	}
	id := strings.TrimSpace(c.Params("id"))
	if !gsm.ValidID(id) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid gameserver ID"})
	}
	server, err := service.GSM.Leave(id, username)
	if err != nil {
		if errors.Is(err, gsm.ErrMissing) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Gameserver not found"})
		}
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Could not leave gameserver"})
	}
	syncGamePlaying(server.GameID, "", -1)
	return c.JSON(fiber.Map{"success": true, "server": toGSView(server)})
}

func AdminGSServersAPI(c fiber.Ctx) error {
	if _, err := getAdminUser(c); err != nil {
		return adminAPIError(c, err)
	}
	if service.GSM == nil {
		return c.JSON(fiber.Map{"servers": []gsView{}, "total": 0, "players": 0, "enabled": false})
	}
	servers, err := service.GSM.ListAll()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Gameserver store unavailable"})
	}
	type gsAdminView struct {
		gsView
		Alive bool `json:"alive"`
	}
	alive, _ := service.GSM.Liveness()
	views := make([]gsAdminView, 0, len(servers))
	players := 0
	for _, s := range servers {
		views = append(views, gsAdminView{gsView: toGSView(s), Alive: alive[s.ID]})
		players += s.PlayerCount
	}
	claimed, _ := service.GSM.Store().ClaimedPorts()
	if claimed == nil {
		claimed = []int{}
	}
	portMin, portMax := 0, 0
	if config.Global != nil {
		portMin = config.Global.GSMPortMin
		portMax = config.Global.GSMPortMax
	}
	return c.JSON(fiber.Map{
		"servers": views,
		"total":   len(views),
		"players": players,
		"enabled": true,
		"ports": fiber.Map{
			"min":     portMin,
			"max":     portMax,
			"claimed": claimed,
		},
	})
}

func AdminGSHealthAPI(c fiber.Ctx) error {
	if _, err := getAdminUser(c); err != nil {
		return adminAPIError(c, err)
	}
	health := fiber.Map{"enabled": service.GSM != nil}
	if service.GSM == nil {
		return c.JSON(health)
	}
	cfg := service.GSM.Config()
	if service.GSM.Remote() {
		ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
		agent, err := service.GSM.AgentHealth(ctx)
		cancel()
		agentInfo := fiber.Map{"url": cfg.AgentURL, "ok": err == nil}
		if err != nil {
			agentInfo["error"] = err.Error()
		} else {
			agentInfo["health"] = agent
		}
		health["mode"] = "remote"
		health["agent"] = agentInfo
	} else {
		binInfo := fiber.Map{"path": cfg.ServerBin, "exists": false, "size": 0}
		if st, err := os.Stat(cfg.ServerBin); err == nil && !st.IsDir() {
			binInfo["exists"] = true
			binInfo["size"] = st.Size()
		} else if err != nil {
			binInfo["error"] = err.Error()
		}
		logsInfo := fiber.Map{"dir": cfg.LogsDir, "writable": false}
		if err := os.MkdirAll(cfg.LogsDir, 0755); err == nil {
			logsInfo["writable"] = true
		} else {
			logsInfo["error"] = err.Error()
		}
		health["mode"] = "local"
		health["binary"] = binInfo
		health["logs"] = logsInfo
		health["map"] = cfg.ServerMap
		health["workdir"] = cfg.ServerWorkdir
	}
	redisInfo := fiber.Map{"addr": "", "ok": false}
	if config.Global != nil {
		redisInfo["addr"] = config.Global.RedisAddr
	}
	if err := service.GSM.Store().Ping(); err == nil {
		redisInfo["ok"] = true
	} else {
		redisInfo["error"] = err.Error()
	}
	alive, err := service.GSM.Liveness()
	if err != nil {
		alive = map[string]bool{}
	}
	claimed, _ := service.GSM.Store().ClaimedPorts()
	if claimed == nil {
		claimed = []int{}
	}
	health["redis"] = redisInfo
	health["alive"] = alive
	health["ports"] = fiber.Map{"min": cfg.PortMin, "max": cfg.PortMax, "claimed": claimed}
	return c.JSON(health)
}

// requireGSOperator limits drain and kill to Head Managers and above.
func requireGSOperator(c fiber.Ctx) error {
	user, err := getAdminUser(c)
	if err != nil {
		return err
	}
	if user.Power < models.PowerHeadManager {
		return fiber.ErrForbidden
	}
	return nil
}

func AdminGSDrainPost(c fiber.Ctx) error {
	if err := requireGSOperator(c); err != nil {
		return adminAPIError(c, err)
	}
	if service.GSM == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Gameserver manager unavailable"})
	}
	id := strings.TrimSpace(c.Params("id"))
	if !gsm.ValidID(id) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid gameserver ID"})
	}
	server, err := service.GSM.Drain(id)
	if err != nil {
		if errors.Is(err, gsm.ErrMissing) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Gameserver not found"})
		}
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Could not drain gameserver"})
	}
	return c.JSON(fiber.Map{"success": true, "server": toGSView(server)})
}

func AdminGSKillPost(c fiber.Ctx) error {
	if err := requireGSOperator(c); err != nil {
		return adminAPIError(c, err)
	}
	if service.GSM == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Gameserver manager unavailable"})
	}
	id := strings.TrimSpace(c.Params("id"))
	if !gsm.ValidID(id) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid gameserver ID"})
	}
	if err := service.GSM.Kill(id); err != nil {
		if errors.Is(err, gsm.ErrMissing) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Gameserver not found"})
		}
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Could not kill gameserver"})
	}
	return c.JSON(fiber.Map{"success": true, "id": id})
}
