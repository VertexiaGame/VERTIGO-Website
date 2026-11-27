package handlers

import (
	"context"
	"encoding/base64"
	"errors"
	"log"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/config"
	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/netcode"
	"vertexia-frontend/backend/service"
	"vertexia-frontend/backend/service/gsm"
)

// joinRetryAfterSeconds is sent with 503 responses from the join endpoint.
const joinRetryAfterSeconds = 5

// joinTicket is everything a client needs to connect to a gameserver over UDP.
type joinTicket struct {
	ServerID     string `json:"server_id"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	ConnectToken string `json:"connect_token"`
	ExpiresAt    int64  `json:"expires_at"`
}

// joinFailure maps a gsm error to the HTTP status and body of the join
// contract: 404 game missing, 409 {error:"full"} without capacity, 503
// {error:"unavailable"} when no server could be made ready.
func joinFailure(err error) (int, fiber.Map) {
	switch {
	case errors.Is(err, gsm.ErrGameMissing), errors.Is(err, gsm.ErrGameClosed):
		return fiber.StatusNotFound, fiber.Map{"error": "not_found"}
	case errors.Is(err, gsm.ErrNoCapacity), errors.Is(err, gsm.ErrFull):
		return fiber.StatusConflict, fiber.Map{"error": "full"}
	default:
		return fiber.StatusServiceUnavailable, fiber.Map{"error": "unavailable"}
	}
}

func writeJoinFailure(c fiber.Ctx, err error) error {
	status, body := joinFailure(err)
	if status == fiber.StatusServiceUnavailable {
		c.Set(fiber.HeaderRetryAfter, strconv.Itoa(joinRetryAfterSeconds))
	}
	return c.Status(status).JSON(body)
}

var (
	gsAddrMu      sync.Mutex
	gsAddrCache   []netip.Addr
	gsAddrExpires time.Time
)

// gameserverAddrs resolves the public UDP endpoint for port. Netcode connect
// tokens carry IP addresses, so GSM_PUBLIC_HOST is resolved here (cached for
// a minute); GSM_PUBLIC_IP (comma separated) overrides the lookup.
func gameserverAddrs(cfg *config.Config, port int) ([]netip.AddrPort, error) {
	if port <= 0 || port > 65535 {
		return nil, errors.New("invalid gameserver port")
	}
	ips, err := gameserverIPs(cfg)
	if err != nil {
		return nil, err
	}
	out := make([]netip.AddrPort, 0, len(ips))
	for _, ip := range ips {
		out = append(out, netip.AddrPortFrom(ip, uint16(port)))
	}
	return out, nil
}

func gameserverIPs(cfg *config.Config) ([]netip.Addr, error) {
	if raw := strings.TrimSpace(cfg.GSMPublicIP); raw != "" {
		var ips []netip.Addr
		for _, part := range strings.Split(raw, ",") {
			ip, err := netip.ParseAddr(strings.TrimSpace(part))
			if err != nil {
				return nil, errors.New("GSM_PUBLIC_IP is not a valid IP list")
			}
			ips = append(ips, ip.Unmap())
		}
		return ips, nil
	}
	host := strings.TrimSpace(cfg.GSMPublicHost)
	if host == "" {
		return nil, errors.New("GSM_PUBLIC_HOST is not configured")
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		return []netip.Addr{ip.Unmap()}, nil
	}
	gsAddrMu.Lock()
	defer gsAddrMu.Unlock()
	if len(gsAddrCache) > 0 && time.Now().Before(gsAddrExpires) {
		return gsAddrCache, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	found, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(found) == 0 {
		if len(gsAddrCache) > 0 {
			return gsAddrCache, nil
		}
		return nil, errors.New("could not resolve GSM_PUBLIC_HOST")
	}
	ips := make([]netip.Addr, 0, len(found))
	// IPv4 first: most home networks still lack IPv6 and netcode clients try
	// the addresses in order.
	for _, ip := range found {
		if ip.Unmap().Is4() {
			ips = append(ips, ip.Unmap())
		}
	}
	for _, ip := range found {
		if !ip.Unmap().Is4() {
			ips = append(ips, ip)
		}
	}
	if len(ips) > 32 {
		ips = ips[:32]
	}
	gsAddrCache, gsAddrExpires = ips, time.Now().Add(time.Minute)
	return ips, nil
}

// mintJoinTicket builds the netcode connect token for user on server.
func mintJoinTicket(cfg *config.Config, user *models.User, server gsm.GameServer) (joinTicket, error) {
	addrs, err := gameserverAddrs(cfg, server.Port)
	if err != nil {
		return joinTicket{}, err
	}
	key := cfg.NetcodePrivateKey
	if len(key) == 0 {
		if cfg.IsProduction() {
			return joinTicket{}, errors.New("NETCODE_PRIVATE_KEY is not configured")
		}
		// Development only: RaveEngineServer defaults to an all-zero key when
		// NETCODE_PRIVATE_KEY is unset, so local play keeps working.
		key = make([]byte, 32)
	}
	userData := netcode.UserData(uint64(user.ID), server.ID)
	token, expiresAt, err := netcode.NewConnectToken(key, cfg.NetcodeProtocolID, uint64(user.ID), cfg.NetcodeTokenTTL, netcode.DefaultTimeoutSeconds, addrs, userData)
	if err != nil {
		return joinTicket{}, err
	}
	return joinTicket{
		ServerID:     server.ID,
		Host:         cfg.GSMPublicHost,
		Port:         server.Port,
		ConnectToken: encodeConnectToken(token),
		ExpiresAt:    expiresAt.Unix(),
	}, nil
}

// joinGameForUser allocates a seat for user in game gameID and mints a
// connect token. It returns the server too so callers can report extras.
func joinGameForUser(user *models.User, gameID int, progress func(string)) (gsm.GameServer, bool, joinTicket, error) {
	cfg := config.Global
	if cfg == nil || service.Game == nil || service.GSM == nil {
		return gsm.GameServer{}, false, joinTicket{}, gsm.ErrUnavailable
	}
	game, err := service.Game.GetGameByID(gameID)
	if err != nil || game == nil || !game.VisibleTo(user.ID) {
		return gsm.GameServer{}, false, joinTicket{}, gsm.ErrGameMissing
	}
	server, redirected, err := service.GSM.JoinWithProgress(gameID, user.Username, progress)
	if err != nil {
		return gsm.GameServer{}, false, joinTicket{}, err
	}
	ticket, err := mintJoinTicket(cfg, user, server)
	if err != nil {
		// The seat reservation would otherwise linger until the reaper runs.
		_, _ = service.GSM.Leave(server.ID, user.Username)
		log.Printf("GSM join game %d by %s: connect token: %v", gameID, user.Username, err)
		return gsm.GameServer{}, false, joinTicket{}, gsm.ErrUnavailable
	}
	syncGamePlaying(gameID, "", -1)
	return server, redirected, ticket, nil
}

// encodeConnectToken is the wire form of the contract: standard base64 of the
// 2048-byte serialized token.
func encodeConnectToken(token []byte) string {
	return base64.StdEncoding.EncodeToString(token)
}
