package routes

import (
	"strings"

	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
	"vertexia-frontend/backend/config"
	"vertexia-frontend/backend/handlers"
)

func Setup(app *fiber.App) {
	app.Use(func(c fiber.Ctx) error {
		if config.Global != nil && config.Global.IsLiveTimerActive() {
			path := c.Path()
			if path != "/livetimer" &&
				path != "/api/v1/livetimer" &&
				!strings.HasPrefix(path, "/static") &&
				!strings.HasPrefix(path, "/assets") &&
				path != "/favicon.ico" {
				if c.Get("HX-Request") == "true" {
					c.Set("HX-Redirect", "/livetimer")
					return c.SendStatus(fiber.StatusFound)
				}
				return c.Redirect().To("/livetimer")
			}
		}
		return c.Next()
	})

	app.Get("/livetimer", handlers.LiveTimerGet)

	app.Use("/ws/feed", func(c fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			username := handlers.GetActiveUser(c)
			if username == "" {
				return c.SendStatus(fiber.StatusUnauthorized)
			}
			c.Locals("username", username)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/ws/feed", websocket.New(handlers.FeedWS))

	app.Get("/", handlers.Home)
	app.Get("/login", handlers.LoginGet)
	app.Post("/login", handlers.LoginPost)
	app.Get("/register", handlers.RegisterGet)
	app.Post("/register", handlers.RegisterPost)
	app.Get("/logout", handlers.Logout)
	app.Get("/altcha", handlers.AltchaGet)
	app.Get("/auth/validate", handlers.ValidateUkey)

	app.Get("/search", handlers.SearchRedirect)
	app.Get("/api/v1/search", handlers.SearchAPI)

	app.Get("/profile", handlers.ProfileRedirect)
	app.Get("/user/:id", handlers.ProfileGet)
	app.Get("/u/:id", handlers.ProfileGet)

	app.Get("/friends", handlers.FriendsGet)
	app.Get("/friends/:id", handlers.FriendsGet)
	app.Post("/friends/send/:id", handlers.FriendSendPost)
	app.Post("/friends/accept/:id", handlers.FriendAcceptPost)
	app.Post("/friends/decline/:id", handlers.FriendDeclinePost)
	app.Post("/friends/cancel/:id", handlers.FriendCancelPost)
	app.Post("/friends/remove/:id", handlers.FriendRemovePost)

	app.Get("/settings", handlers.SettingsGet)
	app.Post("/settings/displayname", handlers.SettingsDisplayNamePost)
	app.Post("/settings/username", handlers.SettingsUsernamePost)
	app.Post("/settings/password", handlers.SettingsPasswordPost)
	app.Post("/settings/bio", handlers.SettingsBioPost)
	app.Post("/settings/pronouns", handlers.SettingsPronounsPost)
	app.Post("/settings/socials", handlers.SettingsSocialsPost)
	app.Post("/settings/music", handlers.SettingsMusicPost)
	app.Post("/settings/music/remove", handlers.SettingsMusicRemovePost)

	app.Get("/shop", handlers.ShopGet)

	app.Get("/avatar", handlers.AvatarEditorPage)
	app.Get("/avatar/:id", handlers.AvatarGet)
	app.Get("/avatar/headshot/:id", handlers.AvatarHeadshotGet)
	app.Get("/avatar/headshots/:id", handlers.AvatarHeadshotGet)
	app.Get("/avatar/shop/:type/:id", handlers.ShopRenderGet)
	app.Get("/avatar/outfit/:id", handlers.AvatarOutfitGet)
	app.Get("/avatar/mesh/:id", handlers.AssetRenderGet)
	app.Post("/avatar/color", handlers.AvatarUpdateColorsPost)
	app.Post("/avatar/wear", handlers.AvatarWearItemPost)
	app.Post("/avatar/unwear", handlers.AvatarUnwearItemPost)
	app.Post("/avatar/outfit/save", handlers.AvatarSaveOutfitPost)
	app.Post("/avatar/outfit/wear", handlers.AvatarWearOutfitPost)
	app.Post("/avatar/outfit/delete", handlers.AvatarDeleteOutfitPost)
	app.Post("/avatar/rerender", handlers.AvatarReRenderPost)

	app.Get("/create", handlers.CreateGet)
	app.Get("/create/assets", handlers.AssetsPage)
	app.Post("/create/assets", handlers.AssetUploadPost)
	app.Get("/assets", handlers.AssetsPage)
	app.Post("/assets", handlers.AssetUploadPost)
	app.Get("/assets/:id/file", handlers.AssetFileGet)
	app.Get("/assets/:id/render", handlers.AssetRenderGet)
	app.Get("/assets/:id", handlers.AssetFileGet)
	app.Get("/asset/:id/file", handlers.AssetFileGet)
	app.Get("/asset/:id", handlers.AssetFileGet)

	app.Post("/feed", handlers.PostFeed)

	app.Get("/admin", handlers.AdminIndex)
	app.Get("/admin/users/:id", handlers.AdminUserViewPage)
	app.Post("/admin/users/:id/scrub/:action", handlers.AdminScrubPost)
	app.Post("/admin/users/:id/modhist/:mid/retract", handlers.AdminModhistRetractPost)
	app.Post("/admin/users/:id/resetavatar", handlers.AdminResetAvatarPost)
	app.Post("/admin/users/:id/outfits/:oid/delete", handlers.AdminOutfitDeletePost)
	app.Post("/admin/assets/:id/:action", handlers.AdminAssetReviewPost)

	api := app.Group("/api/v1")

	api.Get("/altcha", handlers.AltchaGet)
	api.Get("/auth/validate", handlers.ValidateUkey)
	api.Get("/search", handlers.SearchAPI)

	api.Get("/feed", handlers.GetFeedPaginated)
	api.Post("/feed", handlers.PostFeed)
	api.Get("/feed/comments", handlers.GetFeedCommentsHandler)

	api.Get("/user/:id", handlers.ProfileGet)
	api.Get("/u/:id", handlers.ProfileGet)

	api.Get("/friends", handlers.FriendsGet)
	api.Get("/friends/:id", handlers.FriendsGet)
	api.Post("/friends/send/:id", handlers.FriendSendPost)
	api.Post("/friends/accept/:id", handlers.FriendAcceptPost)
	api.Post("/friends/decline/:id", handlers.FriendDeclinePost)
	api.Post("/friends/cancel/:id", handlers.FriendCancelPost)
	api.Post("/friends/remove/:id", handlers.FriendRemovePost)

	api.Get("/settings", handlers.SettingsGet)
	api.Post("/settings/displayname", handlers.SettingsDisplayNamePost)
	api.Post("/settings/username", handlers.SettingsUsernamePost)
	api.Post("/settings/password", handlers.SettingsPasswordPost)
	api.Post("/settings/bio", handlers.SettingsBioPost)
	api.Post("/settings/pronouns", handlers.SettingsPronounsPost)
	api.Post("/settings/socials", handlers.SettingsSocialsPost)
	api.Post("/settings/music", handlers.SettingsMusicPost)
	api.Post("/settings/music/remove", handlers.SettingsMusicRemovePost)

	api.Get("/shop/items", handlers.ShopItemsAPI)
	api.Get("/shop/items/:id", handlers.ShopItemDetailAPI)

	api.Get("/avatar/data/:id", handlers.AvatarDataGet)
	api.Get("/avatar/inventory", handlers.AvatarInventoryAPI)
	api.Get("/avatar/wearing", handlers.AvatarWearingAPI)
	api.Get("/avatar/outfits", handlers.AvatarOutfitsAPI)
	api.Get("/avatar/outfit/:id", handlers.AvatarOutfitGet)
	api.Get("/avatar/:id", handlers.AvatarGet)
	api.Get("/avatar/headshots/:id", handlers.AvatarHeadshotGet)
	api.Get("/avatar/headshot/:id", handlers.AvatarHeadshotGet)
	api.Get("/avatar/shop/:type/:id", handlers.ShopRenderGet)
	api.Get("/avatar/mesh/:id", handlers.AssetRenderGet)
	api.Post("/avatar/color", handlers.AvatarUpdateColorsPost)
	api.Post("/avatar/wear", handlers.AvatarWearItemPost)
	api.Post("/avatar/unwear", handlers.AvatarUnwearItemPost)
	api.Post("/avatar/outfit/save", handlers.AvatarSaveOutfitPost)
	api.Post("/avatar/outfit/wear", handlers.AvatarWearOutfitPost)
	api.Post("/avatar/outfit/delete", handlers.AvatarDeleteOutfitPost)
	api.Post("/avatar/rerender", handlers.AvatarReRenderPost)

	api.Get("/music/search", handlers.MusicSearchGet)
	api.Get("/music/track/:id", handlers.MusicTrackGet)
	api.Post("/music/update", handlers.SettingsMusicPost)

	api.Post("/create/preview", handlers.CreatePreviewPost)
	api.Get("/assets", handlers.AssetsPage)
	api.Post("/assets", handlers.AssetUploadPost)
	api.Get("/assets/:id/file", handlers.AssetFileGet)
	api.Get("/assets/:id/render", handlers.AssetRenderGet)
	api.Get("/assets/:id", handlers.AssetFileGet)
	api.Get("/asset/:id/file", handlers.AssetFileGet)
	api.Get("/asset/:id", handlers.AssetFileGet)

	api.Get("/livetimer", handlers.LiveTimerAPIGet)

	api.Get("/admin/status", handlers.AdminStatusAPI)
	api.Get("/admin/users", handlers.AdminUsersAPI)
	api.Get("/admin/users/:id", handlers.AdminUserDetailAPI)
	api.Get("/admin/logs", handlers.AdminLogsAPI)
	api.Get("/admin/assets", handlers.AdminAssetsAPI)
	api.Post("/admin/assets/:id/:action", handlers.AdminAssetReviewPost)
	api.Post("/admin/users/:id/scrub/:action", handlers.AdminScrubPost)
	api.Post("/admin/users/:id/modhist/:mid/retract", handlers.AdminModhistRetractPost)
	api.Post("/admin/users/:id/resetavatar", handlers.AdminResetAvatarPost)
	api.Post("/admin/users/:id/outfits/:oid/delete", handlers.AdminOutfitDeletePost)

	app.Get("/static/renders/avatars/full/:id", handlers.AvatarGet)
	app.Get("/static/renders/avatars/headshots/:id", handlers.AvatarHeadshotGet)
	app.Get("/static/renders/outfits/:id", handlers.AvatarOutfitGet)
	app.Get("/static/renders/meshes/:id", handlers.AssetRenderGet)
	app.Get("/static/renders/shop/:type/:id", handlers.ShopRenderGet)

	app.Get("/assets*", static.New("./assets", static.Config{
		NotFoundHandler: func(c fiber.Ctx) error {
			return c.Next()
		},
	}))

	app.Get("/static*", static.New("./static", static.Config{
		NotFoundHandler: func(c fiber.Ctx) error {
			return c.Next()
		},
	}))

	app.Get("/*", static.New("./public", static.Config{
		NotFoundHandler: func(c fiber.Ctx) error {
			return c.Next()
		},
	}))

	app.Use(handlers.NotFoundHandler)
}