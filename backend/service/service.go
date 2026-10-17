package service

import (
	"database/sql"
	"errors"
	"log"
	"time"

	"vertexia-frontend/backend/config"
	"vertexia-frontend/backend/repository"
	"vertexia-frontend/backend/service/gsm"
)

var (
	Auth       *AuthService
	User       *UserService
	Feed       *FeedService
	Game       *GameService
	Music      *MusicService
	Cooldown   *CooldownService
	Friend     *FriendService
	ModHistory *ModHistoryService
	Avatar     *AvatarService
	Shop       *ShopService
	Asset      *AssetService
	Banner     *BannerService
	StudioAuth *StudioAuthService
	GSM        *gsm.Manager
)

func Init(db *sql.DB) {
	userRepo := repository.NewUserRepository(db)
	feedRepo := repository.NewFeedRepository(db)
	gameRepo := repository.NewGameRepository(db)
	friendRepo := repository.NewFriendRepository(db)
	modHistRepo := repository.NewModHistoryRepository(db)
	avatarRepo := repository.NewAvatarRepository(db)
	shopRepo := repository.NewShopRepository(db)
	assetRepo := repository.NewAssetRepository(db)
	bannerRepo := repository.NewBannerRepository(db)
	Cooldown = NewCooldownService(5 * time.Second)
	Auth = NewAuthService(userRepo)
	User = NewUserService(userRepo, Cooldown)
	Feed = NewFeedService(feedRepo, userRepo, Cooldown)
	Game = NewGameService(gameRepo)
	Music = NewMusicService()
	Friend = NewFriendService(friendRepo, userRepo, Cooldown)
	ModHistory = NewModHistoryService(modHistRepo, userRepo)
	Avatar = NewAvatarService(avatarRepo, userRepo)
	Shop = NewShopService(shopRepo)
	Asset = NewAssetService(assetRepo)
	Banner = NewBannerService(bannerRepo, Cooldown)
	StudioAuth = NewStudioAuthService(userRepo)
	GSM = initGSMService()
}

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
	}
	store := gsm.NewStore(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	launcher := gsm.NewProcessLauncher(gsmCfg.BindHost, gsmCfg.ServerBin, gsmCfg.ServerWorkdir, gsmCfg.ServerMap, gsmCfg.LogsDir, cfg.GameserverAPIKey)
	manager := gsm.NewManager(gsmCfg, store, launcher, func(gameID int) (gsm.GameInfo, error) {
		if Game == nil {
			return gsm.GameInfo{}, errors.New("game service unavailable")
		}
		game, err := Game.GetGameByID(gameID)
		if err != nil || game == nil {
			return gsm.GameInfo{}, errors.New("game not found")
		}
		return gsm.GameInfo{
			ID:         game.ID,
			MaxPlayers: game.MaxPlayers,
			Private:    game.Private,
			Locked:     game.Locked,
		}, nil
	})
	log.Printf("GSM enabled: ports %d-%d public=%s", gsmCfg.PortMin, gsmCfg.PortMax, gsmCfg.PublicHost)
	manager.Start()
	return manager
}