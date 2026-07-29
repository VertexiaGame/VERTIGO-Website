package handlers

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"vertexia-frontend/backend/config"
	"vertexia-frontend/backend/database"
	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/service"
)

var startTime = time.Now()

type AdminUserView struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	DisplayName  string `json:"display_name"`
	Mail         string `json:"mail"`
	Unikey       string `json:"unikey"`
	Power        int    `json:"power"`
	RoleName     string `json:"role_name"`
	RoleDisplay  string `json:"role_display"`
	Bits         string `json:"bits"`
	Bucks        string `json:"bucks"`
	LastOnline   string `json:"last_online"`
	CreationDate string `json:"creation_date"`
	Views        int    `json:"views"`
	Pronouns     string `json:"pronouns"`
	Description  string `json:"description"`
}

type AvatarItems struct {
	HeadColor  string `json:"head_color"`
	TorsoColor string `json:"torso_color"`
	LArmColor  string `json:"larm_color"`
	RArmColor  string `json:"rarm_color"`
	LLegColor  string `json:"lleg_color"`
	RLegColor  string `json:"rleg_color"`
	Hat1       int    `json:"hat1"`
	Hat2       int    `json:"hat2"`
	Hat3       int    `json:"hat3"`
	Hat4       int    `json:"hat4"`
	Hat5       int    `json:"hat5"`
	Tool       int    `json:"tool"`
	Shirt      int    `json:"shirt"`
	TShirt     int    `json:"tshirt"`
	Pants      int    `json:"pants"`
	Face       int    `json:"face"`
}

func getAdminUser(c fiber.Ctx) (*models.User, error) {
	username := GetActiveUser(c)
	if username == "" {
		return nil, fiber.ErrUnauthorized
	}
	if service.User == nil {
		return nil, fiber.ErrInternalServerError
	}
	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		return nil, fiber.ErrUnauthorized
	}
	if !user.HasAdminAccess() {
		return nil, fiber.ErrForbidden
	}
	return user, nil
}

func getSystemStats() (fiber.Map, runtime.MemStats) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	cpuPct := 0.0
	if percentages, err := cpu.Percent(0, false); err == nil && len(percentages) > 0 {
		cpuPct = percentages[0]
	}

	sysMemTotal, sysMemUsed, sysMemPct := uint64(0), uint64(0), 0.0
	if v, err := mem.VirtualMemory(); err == nil {
		sysMemTotal = v.Total
		sysMemUsed = v.Used
		sysMemPct = v.UsedPercent
	}

	return fiber.Map{
		"uptime":        formatDuration(time.Since(startTime)),
		"goroutines":    runtime.NumGoroutine(),
		"cpu_usage":     fmt.Sprintf("%.1f%%", cpuPct),
		"mem_alloc":     formatBytes(m.Alloc),
		"mem_sys":       formatBytes(m.Sys),
		"sys_mem_total": formatBytes(sysMemTotal),
		"sys_mem_used":  formatBytes(sysMemUsed),
		"sys_mem_pct":   fmt.Sprintf("%.1f%%", sysMemPct),
		"gc_cycles":     m.NumGC,
		"cpuPct":        cpuPct,
		"sysMemTotal":   sysMemTotal,
		"sysMemUsed":    sysMemUsed,
		"sysMemPct":     sysMemPct,
	}, m
}

func getAvatarItems(userID int) *AvatarItems {
	if database.DB == nil {
		return nil
	}
	var a AvatarItems
	query := `SELECT head_color, larm_color, rarm_color, torso_color, lleg_color, rleg_color,
	                 hat1, hat2, hat3, hat4, hat5, tool, shirt, tshirt, pants, face
              FROM avatar WHERE id = ?`
	err := database.DB.QueryRow(query, userID).Scan(
		&a.HeadColor, &a.LArmColor, &a.RArmColor, &a.TorsoColor, &a.LLegColor, &a.RLegColor,
		&a.Hat1, &a.Hat2, &a.Hat3, &a.Hat4, &a.Hat5, &a.Tool, &a.Shirt, &a.TShirt, &a.Pants, &a.Face,
	)
	if err != nil {
		return nil
	}
	return &a
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	days := d / (24 * time.Hour)
	d -= days * 24 * time.Hour
	hours := d / time.Hour
	d -= hours * time.Hour
	minutes := d / time.Minute
	d -= minutes * time.Minute
	seconds := d / time.Second

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

func getCompilationDate() string {
	info, ok := debug.ReadBuildInfo()
	if ok {
		for _, setting := range info.Settings {
			if setting.Key == "-time" {
				if t, err := time.Parse(time.RFC3339, setting.Value); err == nil {
					return t.Format("Jan 02, 2006 15:04")
				}
			}
		}
	}
	return startTime.Format("Jan 02, 2006 15:04")
}

func formatUserForAdmin(u *models.User, adminPower int) AdminUserView {
	dispName := u.DisplayName
	if dispName == "" {
		dispName = u.Username
	}

	view := AdminUserView{
		ID:           u.ID,
		Username:     u.Username,
		DisplayName:  dispName,
		Power:        u.Power,
		RoleName:     u.RoleName(),
		RoleDisplay:  u.RoleDisplayName(),
		CreationDate: u.CreationDate.Format("Jan 02, 2006"),
		Views:        u.Views,
		Pronouns:     u.Pronouns,
		Description:  u.Description,
	}

	if adminPower >= models.PowerModerator {
		view.Mail = u.Mail
		view.Bits = fmt.Sprintf("%d", u.Bits)
		view.Bucks = fmt.Sprintf("%d", u.Bucks)
		view.LastOnline = u.LastOnline.Format("Jan 02, 2006 15:04")
	} else {
		view.Mail = "[Protected]"
		view.Bits = "---"
		view.Bucks = "---"
		view.LastOnline = "---"
	}

	if adminPower >= models.PowerVertexiaTeam {
		view.Unikey = u.Unikey
	} else {
		view.Unikey = "[Protected]"
	}

	return view
}

func AdminIndex(c fiber.Ctx) error {
	user, err := getAdminUser(c)
	if err != nil {
		if err == fiber.ErrUnauthorized {
			if c.Get("HX-Request") == "true" {
				c.Set("HX-Redirect", "/login")
				return c.SendStatus(fiber.StatusUnauthorized)
			}
			return c.Redirect().To("/login")
		}
		if err == fiber.ErrInternalServerError {
			return c.Status(fiber.StatusInternalServerError).SendString("Database offline")
		}
		return c.Status(fiber.StatusForbidden).SendString("Access Denied: You do not have permission to access the admin panel.")
	}

	stats, m := getSystemStats()

	dbConnected := false
	dbMaxOpen, dbOpenConns, dbInUse, dbIdle := 0, 0, 0, 0
	dbWaitCount := int64(0)
	dbWaitDuration := "0s"

	if database.DB != nil {
		dbConnected = true
		dbStats := database.DB.Stats()
		dbMaxOpen = dbStats.MaxOpenConnections
		dbOpenConns = dbStats.OpenConnections
		dbInUse = dbStats.InUse
		dbIdle = dbStats.Idle
		dbWaitCount = dbStats.WaitCount
		dbWaitDuration = dbStats.WaitDuration.String()
	}

	userCount := 0
	if service.User != nil {
		userCount, _ = service.User.GetUserCount()
	}

	gameCount := 0
	if service.Game != nil {
		games, err := service.Game.GetPopularGames(1000)
		if err == nil {
			gameCount = len(games)
		}
	}

	liveTimerActive := false
	if config.Global != nil {
		liveTimerActive = config.Global.IsLiveTimerActive()
	}

	initialUsers, totalUsers, _ := service.User.SearchAdminUsers("", -1, 15, 0)
	userViews := make([]AdminUserView, len(initialUsers))
	for i, u := range initialUsers {
		userViews[i] = formatUserForAdmin(u, user.Power)
	}

	return Render(c, "pages/admin", fiber.Map{
		"Title":           "Admin - VERTEXIA",
		"AdminUser":       user,
		"RoleName":        user.RoleName(),
		"RoleDisplayName": user.RoleDisplayName(),
		"StartTimeMs":     startTime.UnixMilli(),
		"Uptime":          stats["uptime"],
		"StartTime":       startTime.Format("Jan 02, 2006 15:04"),
		"CompilationDate": getCompilationDate(),
		"GoVersion":       runtime.Version(),
		"OSArch":          fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		"NumGoroutines":   stats["goroutines"],
		"NumCPU":          runtime.NumCPU(),
		"CPUUsage":        stats["cpu_usage"],
		"MemAlloc":        stats["mem_alloc"],
		"MemSys":          stats["mem_sys"],
		"SysMemTotal":     stats["sys_mem_total"],
		"SysMemUsed":      stats["sys_mem_used"],
		"SysMemPct":       stats["sys_mem_pct"],
		"NumGC":           m.NumGC,
		"DBConnected":     dbConnected,
		"DBMaxOpen":       dbMaxOpen,
		"DBOpenConns":     dbOpenConns,
		"DBInUse":         dbInUse,
		"DBIdle":          dbIdle,
		"DBWaitCount":     dbWaitCount,
		"DBWaitDuration":  dbWaitDuration,
		"UserCount":       userCount,
		"GameCount":       gameCount,
		"LiveTimerActive": liveTimerActive,
		"Users":           userViews,
		"TotalUsers":      totalUsers,
	}, "layouts/main")
}

func AdminUserViewPage(c fiber.Ctx) error {
	adminUser, err := getAdminUser(c)
	if err != nil {
		if err == fiber.ErrUnauthorized {
			if c.Get("HX-Request") == "true" {
				c.Set("HX-Redirect", "/login")
				return c.SendStatus(fiber.StatusUnauthorized)
			}
			return c.Redirect().To("/login")
		}
		if err == fiber.ErrInternalServerError {
			return c.Status(fiber.StatusInternalServerError).SendString("Database offline")
		}
		return c.Status(fiber.StatusForbidden).SendString("Access Denied: You do not have permission to access the admin panel.")
	}

	targetID, err := strconv.Atoi(c.Params("id"))
	if err != nil || targetID <= 0 {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid user ID")
	}

	targetUser, err := service.User.GetUserByID(targetID)
	if err != nil || targetUser == nil {
		return c.Status(fiber.StatusNotFound).SendString("User not found")
	}

	var modHistory []*models.ModHistory
	if service.ModHistory != nil {
		modHistory, _ = service.ModHistory.GetByUserID(targetID)
	}

	userView := formatUserForAdmin(targetUser, adminUser.Power)
	avatarData := getAvatarItems(targetID)

	return Render(c, "pages/admin_user", fiber.Map{
		"Title":      fmt.Sprintf("Admin - User #%d - VERTEXIA", targetUser.ID),
		"AdminUser":  adminUser,
		"TargetUser": userView,
		"ModHistory": modHistory,
		"Avatar":     avatarData,
	}, "layouts/main")
}

func AdminStatusAPI(c fiber.Ctx) error {
	_, err := getAdminUser(c)
	if err != nil {
		if err == fiber.ErrForbidden {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	stats, _ := getSystemStats()
	delete(stats, "cpuPct")
	delete(stats, "sysMemTotal")
	delete(stats, "sysMemUsed")
	delete(stats, "sysMemPct")

	return c.JSON(stats)
}

func AdminUsersAPI(c fiber.Ctx) error {
	adminUser, err := getAdminUser(c)
	if err != nil {
		if err == fiber.ErrForbidden {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	search := c.Query("q")
	roleFilter, _ := strconv.Atoi(c.Query("role", "-1"))
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "15"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 15
	}
	offset := (page - 1) * limit

	users, total, err := service.User.SearchAdminUsers(search, roleFilter, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	views := make([]AdminUserView, len(users))
	for i, u := range users {
		views[i] = formatUserForAdmin(u, adminUser.Power)
	}

	totalPages := 1
	if total > 0 {
		totalPages = (total + limit - 1) / limit
	}

	return c.JSON(fiber.Map{
		"users":       views,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": totalPages,
	})
}

func AdminUserDetailAPI(c fiber.Ctx) error {
	adminUser, err := getAdminUser(c)
	if err != nil {
		if err == fiber.ErrForbidden {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	targetID, err := strconv.Atoi(c.Params("id"))
	if err != nil || targetID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	targetUser, err := service.User.GetUserByID(targetID)
	if err != nil || targetUser == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	return c.JSON(formatUserForAdmin(targetUser, adminUser.Power))
}