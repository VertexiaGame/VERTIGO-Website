package service

import (
	"database/sql"
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
	Comment    *CommentService
	Asset      *AssetService
	Banner     *BannerService
	StudioAuth *StudioAuthService
	Tokens     *TokenService
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
	commentRepo := repository.NewCommentRepository(db)
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
	Comment = NewCommentService(commentRepo, Cooldown)
	Asset = NewAssetService(assetRepo)
	Banner = NewBannerService(bannerRepo, Cooldown)
	StudioAuth = NewStudioAuthService(userRepo)
	if cfg := config.Global; cfg != nil {
		Tokens = NewTokenService(cfg.ClientTokenTTL, cfg.StudioTokenTTL)
	} else {
		Tokens = NewTokenService(0, 0)
	}
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
