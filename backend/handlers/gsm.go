package handlers

import (
	"errors"
	"log"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/config"
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

func gsmJoinError(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, gsm.ErrGameMissing), errors.Is(err, gsm.ErrGameClosed):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Game not found"})
	case errors.Is(err, gsm.ErrNoCapacity):
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "All gameservers are full right now. Please try again shortly."})
	case errors.Is(err, gsm.ErrLaunch):
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Could not launch a gameserver. Please try again shortly."})
	case errors.Is(err, gsm.ErrUnavailable):
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Gameservers are currently unavailable"})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not join game"})
	}
}

func GameJoinPost(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Please log in to play"})
	}
	gameID, err := strconv.Atoi(c.Params("id"))
	if err != nil || gameID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid game ID"})
	}
	if service.User == nil || service.Game == nil || service.GSM == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Gameservers are currently unavailable"})
	}
	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Please log in to play"})
	}
	server, redirected, err := service.GSM.Join(gameID, username)
	if err != nil {
		log.Printf("GSM join game %d by %s failed: %v", gameID, username, err)
		return gsmJoinError(c, err)
	}
	syncGamePlaying(gameID, "", -1)
	return c.JSON(fiber.Map{
		"success":     true,
		"redirected":  redirected,
		"server":      toGSView(server),
		"ukey":        user.Unikey,
		"client_args": "--ip " + server.Host + " --port " + strconv.Itoa(server.Port) + " --ukey " + user.Unikey,
		"client_uri":  "vertexia-client://play?ip=" + url.QueryEscape(server.Host) + "&port=" + strconv.Itoa(server.Port) + "&ukey=" + url.QueryEscape(user.Unikey),
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
	if err != nil || game == nil || game.Private || game.Locked {
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
	health["binary"] = binInfo
	health["logs"] = logsInfo
	health["redis"] = redisInfo
	health["alive"] = alive
	health["ports"] = fiber.Map{"min": cfg.PortMin, "max": cfg.PortMax, "claimed": claimed}
	health["map"] = cfg.ServerMap
	health["workdir"] = cfg.ServerWorkdir
	return c.JSON(health)
}

func AdminGSDrainPost(c fiber.Ctx) error {
	if _, err := getAdminUser(c); err != nil {
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
	if _, err := getAdminUser(c); err != nil {
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
