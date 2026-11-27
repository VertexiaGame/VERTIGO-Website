package service

import (
	"encoding/hex"
	"errors"
	"log"
	"strconv"

	"vertexia-frontend/backend/config"
	"vertexia-frontend/backend/kv"
	"vertexia-frontend/backend/service/gsm"
)

func initGSMService() *gsm.Manager {
	cfg := config.Global
	if cfg == nil || !cfg.GSMEnabled {
		return nil
	}
	gsmCfg := gsm.Config{
		BindHost:          cfg.GSMBindHost,
		PublicHost:        cfg.GSMPublicHost,
		PortMin:           cfg.GSMPortMin,
		PortMax:           cfg.GSMPortMax,
		MaxPlayersDefault: cfg.GSMMaxPlayersDefault,
		MinPlayersDefault: cfg.GSMMinPlayersDefault,
		IdleTimeout:       cfg.GSMIdleTimeout,
		ReapInterval:      cfg.GSMReapInterval,
		ShutdownGrace:     cfg.GSMShutdownGrace,
		ServerBin:         cfg.GSMServerBin,
		ServerWorkdir:     cfg.GSMServerWorkdir,
		ServerMap:         cfg.GSMServerMap,
		LogsDir:           cfg.GSMServerLogsDir,
		AgentURL:          cfg.GSMAgentURL,
		AgentSecret:       cfg.GSMAgentSecret,
		AgentTimeout:      cfg.GSMAgentTimeout,
		HeartbeatTimeout:  cfg.GSMHeartbeatTimeout,
		MaxServers:        cfg.GSMMaxServers,
		ReadyTimeout:      cfg.GSMReadyTimeout,
		NetcodeProtocolID: cfg.NetcodeProtocolID,
		APIKey:            cfg.GameserverAPIKey,
	}
	if len(cfg.NetcodePrivateKey) == 32 {
		gsmCfg.NetcodeKeyHex = hex.EncodeToString(cfg.NetcodePrivateKey)
	}

	var store gsm.Store
	if client := kv.Client(); client != nil {
		store = gsm.NewRedisStore(client)
	} else if cfg.IsProduction() || cfg.RedisRequired {
		// Never fall back to process memory here: several platform instances
		// would each believe they own every gameserver.
		log.Printf("GSM disabled: Redis is required but not connected")
		return nil
	} else {
		log.Printf("GSM: Redis not connected, using in-memory gameserver records (development only)")
		store = gsm.NewMemoryStore()
	}

	resolve := func(gameID int) (gsm.GameInfo, error) {
		if Game == nil {
			return gsm.GameInfo{}, errors.New("game service unavailable")
		}
		game, err := Game.GetGameByID(gameID)
		if err != nil || game == nil {
			return gsm.GameInfo{}, errors.New("game not found")
		}
		owner := ""
		if game.CreatorID.Valid {
			owner = game.CreatorName
		}
		return gsm.GameInfo{
			ID:         game.ID,
			MaxPlayers: game.MaxPlayers,
			Private:    game.Private,
			Locked:     game.Locked,
			Owner:      owner,
		}, nil
	}

	var manager *gsm.Manager
	if cfg.GSMRemote() {
		client, err := gsm.NewAgentClient(gsmCfg)
		if err != nil {
			log.Printf("GSM disabled: invalid agent configuration: %v", err)
			return nil
		}
		manager = gsm.NewRemoteManager(gsmCfg, store, client, resolve)
		log.Printf("GSM enabled (remote agent %s): public=%s max_servers=%d", client.BaseURL(), gsmCfg.PublicHost, gsmCfg.MaxServers)
	} else {
		var env []string
		if gsmCfg.NetcodeKeyHex != "" {
			env = append(env, "NETCODE_PRIVATE_KEY="+gsmCfg.NetcodeKeyHex)
		}
		if gsmCfg.NetcodeProtocolID != 0 {
			env = append(env, "NETCODE_PROTOCOL_ID="+strconv.FormatUint(gsmCfg.NetcodeProtocolID, 10))
		}
		launcher := gsm.NewProcessLauncherWithOptions(gsm.ProcessOptions{
			Bind:       gsmCfg.BindHost,
			Bin:        gsmCfg.ServerBin,
			Workdir:    gsmCfg.ServerWorkdir,
			MapPath:    gsmCfg.ServerMap,
			LogsDir:    gsmCfg.LogsDir,
			APIKey:     gsmCfg.APIKey,
			Env:        env,
			InheritEnv: true,
		})
		manager = gsm.NewManager(gsmCfg, store, launcher, resolve)
		log.Printf("GSM enabled (local processes): ports %d-%d public=%s", gsmCfg.PortMin, gsmCfg.PortMax, gsmCfg.PublicHost)
	}
	manager.Start()
	return manager
}
