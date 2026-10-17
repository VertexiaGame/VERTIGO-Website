package handlers

import (
	"errors"
	"log"
	"sort"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/service"
	"vertexia-frontend/backend/service/gsm"
)

type gameserverMemberBody struct {
	Ukey     string `json:"ukey"`
	Unikey   string `json:"unikey"`
	Username string `json:"username"`
	Member   string `json:"member"`
	Player   string `json:"player"`
	UID      int    `json:"uid"`
	UserID   int    `json:"user_id"`
}

type gameserverCountBody struct {
	PlayerCount    *int `json:"player_count"`
	Playing        *int `json:"playing"`
	Count          *int `json:"count"`
	CurrentPlayers *int `json:"current_players"`
}

type gameserverRosterBody struct {
	Ukeys   []string `json:"ukeys"`
	Unikeys []string `json:"unikeys"`
}

func cleanRosterKeys(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.Trim(strings.TrimSpace(v), "\"")
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
		if len(out) >= 128 {
			break
		}
	}
	return out
}

func gameserverRequestRoster(c fiber.Ctx) ([]string, bool) {
	var body gameserverRosterBody
	_ = c.Bind().JSON(&body)
	present := body.Ukeys != nil || body.Unikeys != nil
	raw := append(body.Ukeys, body.Unikeys...)
	for _, key := range []string{"ukeys", "unikey"} {
		if v := strings.TrimSpace(c.Query(key)); v != "" {
			present = true
			raw = append(raw, strings.Split(v, ",")...)
		}
		if v := strings.TrimSpace(c.FormValue(key)); v != "" {
			present = true
			raw = append(raw, strings.Split(v, ",")...)
		}
	}
	if !present {
		return nil, false
	}
	return cleanRosterKeys(raw), true
}

func resolveRosterUsernames(ukeys []string) []string {
	if len(ukeys) == 0 || service.User == nil {
		return nil
	}
	users, err := service.User.GetUsersByUnikeys(ukeys)
	if err != nil || len(users) == 0 {
		return nil
	}
	names := make([]string, 0, len(users))
	for _, u := range users {
		if u != nil && u.Username != "" {
			names = append(names, u.Username)
		}
	}
	sort.Strings(names)
	return names
}

func applyRosterReport(id string, ukeys []string) (gsm.GameServer, error) {
	server, err := service.GSM.SyncRoster(id, resolveRosterUsernames(ukeys))
	if err != nil {
		return gsm.GameServer{}, err
	}
	syncGamePlaying(server.GameID, "", -1)
	return server, nil
}

func gameserverRequestMember(c fiber.Ctx) string {
	var body gameserverMemberBody
	_ = c.Bind().JSON(&body)
	ukey := strings.Trim(strings.TrimSpace(body.Ukey), "\"")
	if ukey == "" {
		ukey = strings.Trim(strings.TrimSpace(body.Unikey), "\"")
	}
	username := strings.TrimSpace(body.Username)
	if username == "" {
		username = strings.TrimSpace(body.Member)
	}
	if username == "" {
		username = strings.TrimSpace(body.Player)
	}
	uid := body.UID
	if uid == 0 {
		uid = body.UserID
	}
	if ukey == "" {
		ukey = gameserverUkeyFromRequest(c)
	}
	if ukey == "" {
		for _, key := range []string{"unikey", "user_key"} {
			if v := strings.TrimSpace(c.Query(key)); v != "" {
				ukey = strings.Trim(strings.TrimSpace(v), "\"")
				break
			}
		}
	}
	if ukey == "" {
		ukey = strings.TrimSpace(c.FormValue("ukey"))
		if ukey == "" {
			ukey = strings.TrimSpace(c.FormValue("unikey"))
		}
	}
	if ukey != "" && service.User != nil {
		if u, _ := service.User.GetUserByUnikey(ukey); u != nil {
			return u.Username
		}
		return ""
	}
	if username == "" {
		username = strings.TrimSpace(c.Query("username"))
	}
	if username == "" {
		username = strings.TrimSpace(c.Query("member"))
	}
	if username == "" {
		username = strings.TrimSpace(c.Query("player"))
	}
	if username == "" {
		username = strings.TrimSpace(c.FormValue("username"))
	}
	if username == "" {
		username = strings.TrimSpace(c.FormValue("member"))
	}
	if username != "" && service.User != nil {
		if u, _ := service.User.GetUserByUsername(username); u != nil {
			return u.Username
		}
		return ""
	}
	if uid == 0 {
		if v := strings.TrimSpace(c.Query("uid")); v != "" {
			uid, _ = strconv.Atoi(v)
		}
	}
	if uid == 0 {
		if v := strings.TrimSpace(c.Query("user_id")); v != "" {
			uid, _ = strconv.Atoi(v)
		}
	}
	if uid == 0 {
		if v := strings.TrimSpace(c.FormValue("uid")); v != "" {
			uid, _ = strconv.Atoi(v)
		}
	}
	if uid == 0 {
		if v := strings.TrimSpace(c.FormValue("user_id")); v != "" {
			uid, _ = strconv.Atoi(v)
		}
	}
	if uid > 0 && service.User != nil {
		if u, _ := service.User.GetUserByID(uid); u != nil {
			return u.Username
		}
	}
	if ukey != "" || username != "" {
		return ""
	}
	return ""
}

func gameserverRequestCount(c fiber.Ctx) (int, bool) {
	var body gameserverCountBody
	_ = c.Bind().JSON(&body)
	if body.PlayerCount != nil {
		return *body.PlayerCount, true
	}
	if body.Playing != nil {
		return *body.Playing, true
	}
	if body.Count != nil {
		return *body.Count, true
	}
	if body.CurrentPlayers != nil {
		return *body.CurrentPlayers, true
	}
	for _, key := range []string{"player_count", "playing", "count", "current_players"} {
		if v := strings.TrimSpace(c.Query(key)); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				return n, true
			}
			return -1, true
		}
	}
	for _, key := range []string{"player_count", "playing", "count", "current_players"} {
		if v := strings.TrimSpace(c.FormValue(key)); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				return n, true
			}
			return -1, true
		}
	}
	return 0, false
}

func syncGamePlaying(gameID int, overrideID string, overrideCount int) {
	if service.GSM == nil || service.Game == nil || gameID <= 0 {
		return
	}
	servers, err := service.GSM.Servers(gameID)
	if err != nil {
		log.Printf("Gameserver sync playing game %d failed: %v", gameID, err)
		return
	}
	total := 0
	for _, s := range servers {
		if overrideID != "" && s.ID == overrideID && overrideCount >= 0 {
			total += overrideCount
			continue
		}
		if s.PlayerCount < 0 {
			continue
		}
		total += s.PlayerCount
	}
	if err := service.Game.SetPlaying(gameID, total); err != nil {
		log.Printf("Gameserver sync playing game %d to %d failed: %v", gameID, total, err)
	}
}

func gameserverRequireServer(c fiber.Ctx) (gsm.GameServer, error) {
	id := strings.TrimSpace(c.Params("id"))
	if !gsm.ValidID(id) {
		return gsm.GameServer{}, errors.New("invalid")
	}
	if service.GSM == nil {
		return gsm.GameServer{}, gsm.ErrUnavailable
	}
	return service.GSM.Get(id)
}

func GameserverGameGet(c fiber.Ctx) error {
	if !gameserverKeyValid(c) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized gameserver request"})
	}
	gameID, err := strconv.Atoi(c.Params("id"))
	if err != nil || gameID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid game ID"})
	}
	if service.Game == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Game service unavailable"})
	}
	game, err := service.Game.GetGameByID(gameID)
	if err != nil || game == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Game not found"})
	}
	maxPlayers := game.MaxPlayers
	if service.GSM != nil {
		maxPlayers = service.GSM.EffectiveMaxForGame(game.MaxPlayers)
	} else {
		maxPlayers = service.NormalizeMaxPlayers(game.MaxPlayers, 100)
	}
	return c.JSON(fiber.Map{
		"id":          game.ID,
		"name":        game.Name,
		"max_players": maxPlayers,
		"playing":     game.Playing,
		"visits":      game.Visits,
		"private":     game.Private,
		"locked":      game.Locked,
		"genre":       game.GenreText,
	})
}

func GameserverServerGet(c fiber.Ctx) error {
	if !gameserverKeyValid(c) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized gameserver request"})
	}
	server, err := gameserverRequireServer(c)
	if err != nil {
		if err.Error() == "invalid" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid gameserver ID"})
		}
		if errors.Is(err, gsm.ErrMissing) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Gameserver not found"})
		}
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Gameservers are currently unavailable"})
	}
	if service.Game != nil {
		if game, _ := service.Game.GetGameByID(server.GameID); game != nil {
			if service.GSM != nil {
				server.MaxPlayers = service.GSM.EffectiveMaxForGame(game.MaxPlayers)
			} else {
				server.MaxPlayers = service.NormalizeMaxPlayers(game.MaxPlayers, 100)
			}
		}
	}
	return c.JSON(fiber.Map{"server": toGSView(server)})
}

func GameserverServerJoinPost(c fiber.Ctx) error {
	if !gameserverKeyValid(c) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized gameserver request"})
	}
	id := strings.TrimSpace(c.Params("id"))
	if !gsm.ValidID(id) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid gameserver ID"})
	}
	if service.GSM == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Gameservers are currently unavailable"})
	}
	if service.User == nil || service.Game == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Service unavailable"})
	}
	member := gameserverRequestMember(c)
	if member == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing or unknown user: provide ukey, username or uid"})
	}
	server, isNew, err := service.GSM.ServerJoin(id, member)
	if err != nil {
		if errors.Is(err, gsm.ErrMissing) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Gameserver not found"})
		}
		if errors.Is(err, gsm.ErrFull) {
			cur, _ := service.GSM.Get(id)
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Gameserver is full", "server": toGSView(cur)})
		}
		if errors.Is(err, gsm.ErrGameMissing) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Game not found"})
		}
		if errors.Is(err, gsm.ErrGameClosed) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Game is not joinable"})
		}
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Could not join gameserver"})
	}
	if isNew && service.Game != nil {
		_ = service.Game.AddVisit(server.GameID)
	}
	if roster, ok := gameserverRequestRoster(c); ok {
		var err error
		server, err = applyRosterReport(id, roster)
		if err != nil {
			if errors.Is(err, gsm.ErrMissing) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Gameserver not found"})
			}
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Could not sync gameserver roster"})
		}
	} else if count, ok := gameserverRequestCount(c); ok {
		if count < 0 || count > server.MaxPlayers {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid player_count"})
		}
		syncGamePlaying(server.GameID, server.ID, count)
		server.PlayerCount = count
	} else {
		syncGamePlaying(server.GameID, "", -1)
	}
	return c.JSON(fiber.Map{"success": true, "is_new": isNew, "player_count": server.PlayerCount, "server": toGSView(server)})
}

func GameserverServerLeavePost(c fiber.Ctx) error {
	if !gameserverKeyValid(c) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized gameserver request"})
	}
	id := strings.TrimSpace(c.Params("id"))
	if !gsm.ValidID(id) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid gameserver ID"})
	}
	if service.GSM == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Gameservers are currently unavailable"})
	}
	if service.User == nil || service.Game == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Service unavailable"})
	}
	member := gameserverRequestMember(c)
	if member == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing or unknown user: provide ukey, username or uid"})
	}
	server, err := service.GSM.Leave(id, member)
	if err != nil {
		if errors.Is(err, gsm.ErrMissing) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Gameserver not found"})
		}
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Could not leave gameserver"})
	}
	if roster, ok := gameserverRequestRoster(c); ok {
		server, err = applyRosterReport(id, roster)
		if err != nil {
			if errors.Is(err, gsm.ErrMissing) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Gameserver not found"})
			}
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Could not sync gameserver roster"})
		}
	} else if count, ok := gameserverRequestCount(c); ok {
		if count < 0 || count > server.MaxPlayers {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid player_count"})
		}
		syncGamePlaying(server.GameID, server.ID, count)
		server.PlayerCount = count
	} else {
		syncGamePlaying(server.GameID, "", -1)
	}
	return c.JSON(fiber.Map{"success": true, "player_count": server.PlayerCount, "server": toGSView(server)})
}

func GameserverServerHeartbeatPost(c fiber.Ctx) error {
	if !gameserverKeyValid(c) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized gameserver request"})
	}
	id := strings.TrimSpace(c.Params("id"))
	if !gsm.ValidID(id) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid gameserver ID"})
	}
	if service.GSM == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Gameservers are currently unavailable"})
	}
	if roster, ok := gameserverRequestRoster(c); ok {
		server, err := applyRosterReport(id, roster)
		if err != nil {
			if errors.Is(err, gsm.ErrMissing) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Gameserver not found"})
			}
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Could not sync gameserver roster"})
		}
		return c.JSON(fiber.Map{"success": true, "player_count": server.PlayerCount, "server": toGSView(server)})
	}
	server, err := service.GSM.Heartbeat(id)
	if err != nil {
		if errors.Is(err, gsm.ErrMissing) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Gameserver not found"})
		}
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Could not heartbeat gameserver"})
	}
	if count, ok := gameserverRequestCount(c); ok {
		if count < 0 || count > server.MaxPlayers {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid player_count"})
		}
		syncGamePlaying(server.GameID, server.ID, count)
		server.PlayerCount = count
	} else {
		syncGamePlaying(server.GameID, "", -1)
	}
	return c.JSON(fiber.Map{"success": true, "player_count": server.PlayerCount, "server": toGSView(server)})
}
