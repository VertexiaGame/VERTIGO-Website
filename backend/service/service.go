package service

import (
	"database/sql"
	"errors"
	"log"
	"time"

	"vertexia-frontend/backend/config"
	"vertexia-frontend/backend/repository"
	filtersvc "vertexia-frontend/backend/service/filter"
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
	Clan       *ClanService
	Forum      *ForumService
	Asset      *AssetService
	Banner     *BannerService
	StudioAuth *StudioAuthService
	GSM        *gsm.Manager
	Filter     *filtersvc.Service
)

func Init(db *sql.DB) {
	userRepo := repository.NewUserRepository(db)
	feedRepo := repository.NewFeedRepository(db)
	gameRepo := repository.NewGameRepository(db)
	friendRepo := repository.NewFriendRepository(db)
	modHistRepo := repository.NewModHistoryRepository(db)
	avatarRepo := repository.NewAvatarRepository(db)
	shopRepo := repository.NewShopRepository(db)
	clanRepo := repository.NewClanRepository(db)
	forumRepo := repository.NewForumRepository(db)
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
	Clan = NewClanService(clanRepo, userRepo, Cooldown)
	Forum = NewForumService(forumRepo, Cooldown)
	Asset = NewAssetService(assetRepo)
	Banner = NewBannerService(bannerRepo, Cooldown)
	StudioAuth = NewStudioAuthService(userRepo)
	GSM = initGSMService()
	initFilterService(db)
}

func initFilterService(db *sql.DB) {
	cfg := config.Global
	if cfg == nil || !cfg.FilterEnabled || cfg.TypeSafeAPIKey == "" {
		log.Printf("Filter API disabled (set FILTER_ENABLED=true and TYPESAFE_API_KEY to enable)")
		return
	}
	var cache filtersvc.Cache
	if db != nil {
		cache = repository.NewJevCacheRepository(db)
	}
	filterService, err := filtersvc.Start(filtersvc.Config{
		APIKey:  cfg.TypeSafeAPIKey,
		BaseURL: cfg.TypeSafeBaseURL,
		Model:   cfg.TypeSafeModel,
		Timeout: cfg.FilterTimeout,
		Cache:   cache,
	})
	if err != nil {
		log.Printf("Filter API disabled: %v", err)
		return
	}
	Filter = filterService
	log.Printf("Filter API (Jev model %s) started on loopback only", cfg.TypeSafeModel)
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
	})
	log.Printf("GSM enabled: ports %d-%d public=%s", gsmCfg.PortMin, gsmCfg.PortMax, gsmCfg.PublicHost)
	manager.Start()
	return manager
}
