package handlers

import (
	"errors"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/service"
)

const clanIconUploadDir = "./static/uploads/clans"

var errClanSession = errors.New("your session expired, please log in again")

type clanShelf struct {
	Name  string
	Cat   string
	Clans []*models.Clan
}

func clanViewer(c fiber.Ctx) (*models.User, error) {
	if service.User == nil {
		return nil, errClanSession
	}
	username := GetActiveUser(c)
	if username == "" {
		return nil, errClanSession
	}
	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		return nil, errClanSession
	}
	return user, nil
}

func clanErrorStatus(err error) int {
	switch {
	case err == nil:
		return fiber.StatusOK
	case errors.Is(err, errClanSession):
		return fiber.StatusUnauthorized
	case errors.Is(err, service.ErrClanNotFound):
		return fiber.StatusNotFound
	case errors.Is(err, service.ErrClanPermission), errors.Is(err, service.ErrClanOwnerProtected):
		return fiber.StatusForbidden
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "permission"), strings.Contains(msg, "only members"), strings.Contains(msg, "can't"):
		return fiber.StatusForbidden
	case strings.Contains(msg, "not found"):
		return fiber.StatusNotFound
	default:
		return fiber.StatusBadRequest
	}
}

func clanActionError(c fiber.Ctx, err error, redirect string) error {
	status := clanErrorStatus(err)
	if wantsAssetJSON(c) {
		return c.Status(status).JSON(fiber.Map{"error": err.Error()})
	}
	if status == fiber.StatusUnauthorized {
		return c.Redirect().To("/login")
	}
	sep := "?"
	if strings.Contains(redirect, "?") {
		sep = "&"
	}
	return c.Redirect().To(redirect + sep + "error=" + url.QueryEscape(err.Error()))
}

func clanActionSucceed(c fiber.Ctx, redirect string, payload fiber.Map) error {
	if wantsAssetJSON(c) {
		if payload == nil {
			payload = fiber.Map{}
		}
		payload["success"] = true
		return c.JSON(payload)
	}
	return c.Redirect().To(redirect)
}

func clanCheckCSRF(c fiber.Ctx) error {
	if !csrfValid(c) {
		return errors.New("your session expired, refresh the page and try again")
	}
	return nil
}

func clanNormalizeSort(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "members-asc":
		return "members-asc"
	case "name-asc", "name":
		return "name-asc"
	case "newest":
		return "newest"
	case "oldest":
		return "oldest"
	default:
		return "members-desc"
	}
}

func clanCategory(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" || raw == "all" {
		return "all"
	}
	return models.NormalizeClanCategoryKey(raw)
}

func sortClansByMembers(clans []*models.Clan) []*models.Clan {
	sorted := make([]*models.Clan, len(clans))
	copy(sorted, clans)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Members != sorted[j].Members {
			return sorted[i].Members > sorted[j].Members
		}
		return sorted[i].CreatedAt.After(sorted[j].CreatedAt)
	})
	return sorted
}

func sortClansByNewest(clans []*models.Clan) []*models.Clan {
	sorted := make([]*models.Clan, len(clans))
	copy(sorted, clans)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].CreatedAt.After(sorted[j].CreatedAt)
	})
	return sorted
}

func clanShelves(clans []*models.Clan) []clanShelf {
	shelves := make([]clanShelf, 0, len(models.ClanCategories))
	for _, entry := range models.ClanCategories {
		var members []*models.Clan
		for _, clan := range clans {
			if clan.Category == entry.Key {
				members = append(members, clan)
			}
		}
		if len(members) == 0 {
			continue
		}
		members = sortClansByMembers(members)
		if len(members) > 8 {
			members = members[:8]
		}
		shelves = append(shelves, clanShelf{Name: entry.Label, Cat: entry.Key, Clans: members})
	}
	return shelves
}

func clanPageData(clan *models.Clan, access *service.ClanAccess) fiber.Map {
	return fiber.Map{
		"Clan":                clan,
		"Access":              access,
		"CanViewContent":      access.CanViewContent,
		"CanManageClan":       access.CanManageClan(),
		"CanEditClan":         access.Allows(models.ClanPermissionEditClan),
		"CanManageRoles":      access.Allows(models.ClanPermissionManageRoles),
		"CanManageMembers":    access.Allows(models.ClanPermissionManageMembers),
		"CanKickMembers":      access.Allows(models.ClanPermissionKickMembers),
		"CanPostAnnouncement": access.Allows(models.ClanPermissionPostAnnouncements),
		"CanManageWall":       access.Allows(models.ClanPermissionManageWall),
		"CanPostWall":         access.CanPostWall(),
		"CanJoin":             access.CanJoin,
		"CanRequest":          access.CanRequest,
		"IsOwner":             access.IsOwner,
		"IsMember":            access.IsMember,
		"IsPending":           access.Pending,
		"ViewerRole":          access.Role,
		"ViewerMember":        access.Member,
	}
}

func ClansGet(c fiber.Ctx) error {
	if service.Clan == nil {
		return serveErrorPage(c, fiber.StatusInternalServerError)
	}

	viewerID := activeUserID(c)
	search := strings.TrimSpace(c.Query("q"))
	category := clanCategory(c.Query("cat", c.Query("category", "all")))
	sortBy := clanNormalizeSort(c.Query("sort"))

	clans, err := service.Clan.GetPublicClans(viewerID, search, category, sortBy)
	if err != nil {
		clans = []*models.Clan{}
	}
	if clans == nil {
		clans = []*models.Clan{}
	}

	var joined []*models.Clan
	if viewerID > 0 {
		joined, _ = service.Clan.GetJoinedClans(viewerID)
	}
	if joined == nil {
		joined = []*models.Clan{}
	}

	biggest := sortClansByMembers(clans)
	if len(biggest) > 8 {
		biggest = biggest[:8]
	}
	featured := biggest
	if len(featured) > 5 {
		featured = featured[:5]
	}
	newest := sortClansByNewest(clans)
	if len(newest) > 8 {
		newest = newest[:8]
	}

	return Render(c, "pages/clans", fiber.Map{
		"Title":       "Clans - VERTEXIA",
		"Query":       search,
		"ActiveCat":   category,
		"Sort":        sortBy,
		"Clans":       clans,
		"Featured":    featured,
		"JoinedClans": joined,
		"Biggest":     biggest,
		"Newest":      newest,
		"CatShelves":  clanShelves(clans),
	}, "layouts/main")
}

func ClanDetailGet(c fiber.Ctx) error {
	if service.Clan == nil {
		return serveErrorPage(c, fiber.StatusInternalServerError)
	}

	identifier := c.Params("name")
	if decoded, err := url.PathUnescape(identifier); err == nil {
		identifier = decoded
	}

	viewerID := 0
	isAdmin := false
	if viewer, err := clanViewer(c); err == nil && viewer != nil {
		viewerID = viewer.ID
		isAdmin = viewer.HasAdminAccess()
	}

	clan, access, err := service.Clan.GetClanPage(identifier, viewerID, isAdmin)
	if errors.Is(err, service.ErrClanNotFound) || (err == nil && clan == nil) {
		return serveErrorPage(c, fiber.StatusNotFound)
	}
	if err != nil {
		return serveErrorPage(c, fiber.StatusInternalServerError)
	}

	wearingTag := false
	if viewerID > 0 {
		if worn, werr := service.Clan.WornClan(viewerID); werr == nil && worn != nil {
			wearingTag = worn.ID == clan.ID
		}
	}

	data := clanPageData(clan, access)
	data["Title"] = clan.Name + " - VERTEXIA"
	data["IsWearingTag"] = wearingTag
	data["Error"] = strings.TrimSpace(c.Query("error"))
	data["Notice"] = strings.TrimSpace(c.Query("notice"))
	return Render(c, "pages/clan", data, "layouts/main")
}

func MyClansPage(c fiber.Ctx) error {
	user, err := clanViewer(c)
	if err != nil {
		return c.Redirect().To("/login")
	}
	if service.Clan == nil {
		return serveErrorPage(c, fiber.StatusInternalServerError)
	}

	owned, _ := service.Clan.GetOwnedClans(user.ID)
	if owned == nil {
		owned = []*models.Clan{}
	}
	joined, _ := service.Clan.GetJoinedClans(user.ID)
	if joined == nil {
		joined = []*models.Clan{}
	}

	ownedIDs := make(map[int]bool, len(owned))
	for _, clan := range owned {
		ownedIDs[clan.ID] = true
	}
	crews := make([]*models.Clan, 0, len(joined))
	for _, clan := range joined {
		if !ownedIDs[clan.ID] {
			crews = append(crews, clan)
		}
	}

	return Render(c, "pages/myclans", fiber.Map{
		"Title":      "My Clans - VERTEXIA",
		"Owned":      owned,
		"Joined":     crews,
		"WornClanID": user.PrimaryClan,
	}, "layouts/main")
}

func ClanEditPage(c fiber.Ctx) error {
	user, err := clanViewer(c)
	if err != nil {
		return c.Redirect().To("/login")
	}
	if service.Clan == nil {
		return serveErrorPage(c, fiber.StatusInternalServerError)
	}

	clanID, err := strconv.Atoi(c.Params("id"))
	if err != nil || clanID <= 0 {
		return serveErrorPage(c, fiber.StatusNotFound)
	}

	clan, access, err := service.Clan.GetClanPage(strconv.Itoa(clanID), user.ID, user.HasAdminAccess())
	if errors.Is(err, service.ErrClanNotFound) || (err == nil && clan == nil) {
		return serveErrorPage(c, fiber.StatusNotFound)
	}
	if err != nil {
		return serveErrorPage(c, fiber.StatusInternalServerError)
	}
	if !access.CanManageClan() {
		return serveErrorPage(c, fiber.StatusForbidden)
	}

	data := clanPageData(clan, access)
	data["Title"] = "Edit " + clan.Name + " - VERTEXIA"
	data["Permissions"] = models.ClanPermissions
	data["Error"] = strings.TrimSpace(c.Query("error"))
	data["Notice"] = strings.TrimSpace(c.Query("notice"))
	data["MaxMemberSlots"] = models.ClanMaxMembers
	return Render(c, "pages/clan_edit", data, "layouts/main")
}

func clanValidateUpload(c fiber.Ctx, field, kind string) error {
	header := openGameUpload(c, field)
	if header == nil {
		return nil
	}
	return checkGameUploadImage(header, kind)
}

func clanSavePendingImage(c fiber.Ctx, field, kind string, clanID int) (string, error) {
	header := openGameUpload(c, field)
	if header == nil {
		return "", nil
	}
	if err := checkGameUploadImage(header, kind); err != nil {
		return "", err
	}
	if err := os.MkdirAll(clanIconUploadDir, 0755); err != nil {
		return "", errors.New("could not save upload")
	}
	name := strconv.Itoa(clanID) + "_" + field + "_" + strconv.FormatInt(time.Now().UnixNano(), 36) + ".png"
	if err := saveGameUploadImage(header, kind, filepath.Join(clanIconUploadDir, name)); err != nil {
		return "", err
	}
	return "/static/uploads/clans/" + name + "?v=" + strconv.FormatInt(time.Now().Unix(), 10), nil
}

func clanMediaFilePath(url string) string {
	u := strings.TrimSpace(url)
	if u == "" {
		return ""
	}
	if i := strings.IndexByte(u, '?'); i >= 0 {
		u = u[:i]
	}
	name := path.Base(u)
	if name == "" || name == "." || name == "/" {
		return ""
	}
	return filepath.Join(clanIconUploadDir, name)
}

func clanApprovalOnly(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "approval", "approval-only", "approval_only", "request", "private":
		return true
	default:
		return false
	}
}

func ClanCreatePost(c fiber.Ctx) error {
	jsonResponse := wantsAssetJSON(c)
	user, err := clanViewer(c)
	if err != nil {
		if jsonResponse {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Redirect().To("/login")
	}

	fail := func(msg string, status int) error {
		if jsonResponse {
			return c.Status(status).JSON(fiber.Map{"error": msg})
		}
		return c.Redirect().To("/create?type=clan&error=" + url.QueryEscape(msg))
	}

	if service.Clan == nil {
		return fail("Clan service unavailable", fiber.StatusInternalServerError)
	}
	if err := clanCheckCSRF(c); err != nil {
		return fail(err.Error(), fiber.StatusForbidden)
	}

	if err := clanValidateUpload(c, "icon", "Clan icon"); err != nil {
		return fail(err.Error(), fiber.StatusBadRequest)
	}
	if err := clanValidateUpload(c, "banner", "Clan banner"); err != nil {
		return fail(err.Error(), fiber.StatusBadRequest)
	}

	clan, err := service.Clan.CreateClan(user, service.ClanInput{
		Name:         c.FormValue("name"),
		Description:  c.FormValue("description"),
		Category:     c.FormValue("category"),
		Tag:          c.FormValue("tag"),
		TagColor:     c.FormValue("tag_color"),
		ApprovalOnly: clanApprovalOnly(c.FormValue("privacy")),
	})
	if err != nil {
		return fail(err.Error(), clanErrorStatus(err))
	}

	if iconURL, ierr := clanSavePendingImage(c, "icon", "Clan icon", clan.ID); ierr == nil && iconURL != "" {
		_ = service.Clan.SetClanPendingIcon(clan.ID, user.ID, user.HasAdminAccess(), iconURL)
	}
	if bannerURL, berr := clanSavePendingImage(c, "banner", "Clan banner", clan.ID); berr == nil && bannerURL != "" {
		_ = service.Clan.SetClanPendingBanner(clan.ID, user.ID, user.HasAdminAccess(), bannerURL)
	}

	clanURL := "/clans/" + url.PathEscape(clan.SlugPath())
	if jsonResponse {
		return c.JSON(fiber.Map{
			"success":       true,
			"id":            clan.ID,
			"slug":          clan.SlugPath(),
			"url":           clanURL,
			"approval_only": clan.ApprovalOnly,
		})
	}
	return c.Redirect().To(clanURL)
}

func ClanUpdatePost(c fiber.Ctx) error {
	jsonResponse := wantsAssetJSON(c)
	user, err := clanViewer(c)
	if err != nil {
		if jsonResponse {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Redirect().To("/login")
	}

	clanID, err := strconv.Atoi(c.Params("id"))
	if err != nil || clanID <= 0 {
		if jsonResponse {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid clan id"})
		}
		return c.Redirect().To("/create/clans")
	}
	editURL := "/create/clans/" + strconv.Itoa(clanID) + "/edit"

	fail := func(msg string, status int) error {
		if jsonResponse {
			return c.Status(status).JSON(fiber.Map{"error": msg})
		}
		return c.Redirect().To(editURL + "?error=" + url.QueryEscape(msg))
	}

	if service.Clan == nil {
		return fail("Clan service unavailable", fiber.StatusInternalServerError)
	}
	if err := clanCheckCSRF(c); err != nil {
		return fail(err.Error(), fiber.StatusForbidden)
	}

	clan, access, err := service.Clan.GetClanPage(strconv.Itoa(clanID), user.ID, user.HasAdminAccess())
	if err != nil || clan == nil {
		return fail("Clan not found", fiber.StatusNotFound)
	}
	if !access.Allows(models.ClanPermissionEditClan) {
		return fail("You don't have permission to edit this clan", fiber.StatusForbidden)
	}

	updated, err := service.Clan.UpdateClan(clanID, user.ID, user.HasAdminAccess(), service.ClanInput{
		Name:         c.FormValue("name"),
		Description:  c.FormValue("description"),
		Category:     c.FormValue("category"),
		Tag:          c.FormValue("tag"),
		TagColor:     c.FormValue("tag_color"),
		ApprovalOnly: clanApprovalOnly(c.FormValue("privacy")),
	})
	if err != nil {
		return fail(err.Error(), clanErrorStatus(err))
	}

	if c.FormValue("clear_icon") == "1" {
		_ = service.Clan.SetClanIcon(clanID, user.ID, user.HasAdminAccess(), "")
		if p := clanMediaFilePath(clan.Icon); p != "" {
			os.Remove(p)
		}
	} else if iconURL, ierr := clanSavePendingImage(c, "icon", "Clan icon", clanID); ierr != nil {
		return fail(ierr.Error(), fiber.StatusBadRequest)
	} else if iconURL != "" {
		if err := service.Clan.SetClanPendingIcon(clanID, user.ID, user.HasAdminAccess(), iconURL); err != nil {
			return fail(err.Error(), clanErrorStatus(err))
		}
	}

	clanURL := "/clans/" + url.PathEscape(updated.SlugPath())
	if jsonResponse {
		return c.JSON(fiber.Map{"success": true, "id": updated.ID, "slug": updated.SlugPath(), "url": clanURL})
	}
	return c.Redirect().To(editURL + "?notice=" + url.QueryEscape("Clan updated"))
}

func ClanJoinPost(c fiber.Ctx) error {
	user, err := clanViewer(c)
	if err != nil {
		return clanActionError(c, err, "/clans")
	}
	clanID, _ := strconv.Atoi(c.Params("id"))
	if clanID <= 0 {
		return clanActionError(c, service.ErrClanNotFound, "/clans")
	}
	if err := clanCheckCSRF(c); err != nil {
		return clanActionError(c, err, "/clans/"+strconv.Itoa(clanID))
	}
	if service.Clan == nil {
		return clanActionError(c, errors.New("clan service unavailable"), "/clans")
	}

	status, members, err := service.Clan.JoinClan(clanID, user.ID, c.FormValue("message"))
	if err != nil {
		return clanActionError(c, err, "/clans/"+strconv.Itoa(clanID))
	}
	return clanActionSucceed(c, "/clans/"+strconv.Itoa(clanID), fiber.Map{
		"status":  status,
		"members": members,
	})
}

func ClanLeavePost(c fiber.Ctx) error {
	user, err := clanViewer(c)
	if err != nil {
		return clanActionError(c, err, "/clans")
	}
	clanID, _ := strconv.Atoi(c.Params("id"))
	if clanID <= 0 {
		return clanActionError(c, service.ErrClanNotFound, "/clans")
	}
	if err := clanCheckCSRF(c); err != nil {
		return clanActionError(c, err, "/clans/"+strconv.Itoa(clanID))
	}
	if service.Clan == nil {
		return clanActionError(c, errors.New("clan service unavailable"), "/clans")
	}

	if err := service.Clan.LeaveClan(clanID, user.ID); err != nil {
		return clanActionError(c, err, "/clans/"+strconv.Itoa(clanID))
	}
	members, _ := service.Clan.MemberCount(clanID)
	return clanActionSucceed(c, "/clans/"+strconv.Itoa(clanID), fiber.Map{
		"status":  "left",
		"members": members,
	})
}

func ClanTagTogglePost(c fiber.Ctx) error {
	user, err := clanViewer(c)
	if err != nil {
		return clanActionError(c, err, "/clans")
	}
	clanID, _ := strconv.Atoi(c.Params("id"))
	if clanID <= 0 {
		return clanActionError(c, service.ErrClanNotFound, "/clans")
	}
	redirect := "/clans/" + strconv.Itoa(clanID)
	if err := clanCheckCSRF(c); err != nil {
		return clanActionError(c, err, redirect)
	}
	if service.Clan == nil {
		return clanActionError(c, errors.New("clan service unavailable"), "/clans")
	}

	wearing, err := service.Clan.ToggleWornTag(clanID, user.ID)
	if err != nil {
		return clanActionError(c, err, redirect)
	}
	return clanActionSucceed(c, redirect, fiber.Map{"wearing": wearing})
}

func ClanCancelRequestPost(c fiber.Ctx) error {
	user, err := clanViewer(c)
	if err != nil {
		return clanActionError(c, err, "/clans")
	}
	clanID, _ := strconv.Atoi(c.Params("id"))
	if clanID <= 0 {
		return clanActionError(c, service.ErrClanNotFound, "/clans")
	}
	if err := clanCheckCSRF(c); err != nil {
		return clanActionError(c, err, "/clans/"+strconv.Itoa(clanID))
	}
	if service.Clan == nil {
		return clanActionError(c, errors.New("clan service unavailable"), "/clans")
	}

	if err := service.Clan.CancelJoinRequest(clanID, user.ID); err != nil {
		return clanActionError(c, err, "/clans/"+strconv.Itoa(clanID))
	}
	return clanActionSucceed(c, "/clans/"+strconv.Itoa(clanID), fiber.Map{"status": "cancelled"})
}

func clanEditRedirect(c fiber.Ctx) string {
	return "/create/clans/" + c.Params("id") + "/edit"
}

func ClanRequestApprovePost(c fiber.Ctx) error {
	user, err := clanViewer(c)
	if err != nil {
		return clanActionError(c, err, "/clans")
	}
	clanID, _ := strconv.Atoi(c.Params("id"))
	requestID, _ := strconv.ParseInt(c.Params("requestId"), 10, 64)
	if clanID <= 0 || requestID <= 0 {
		return clanActionError(c, service.ErrClanNotFound, "/clans")
	}
	if err := clanCheckCSRF(c); err != nil {
		return clanActionError(c, err, clanEditRedirect(c))
	}
	if service.Clan == nil {
		return clanActionError(c, errors.New("clan service unavailable"), "/clans")
	}

	roleID, _ := strconv.Atoi(c.FormValue("role_id"))
	userID, err := service.Clan.ApproveJoinRequest(clanID, user.ID, user.HasAdminAccess(), requestID, roleID)
	if err != nil {
		return clanActionError(c, err, clanEditRedirect(c))
	}
	members, _ := service.Clan.MemberCount(clanID)
	return clanActionSucceed(c, clanEditRedirect(c), fiber.Map{"user_id": userID, "members": members})
}

func ClanRequestDenyPost(c fiber.Ctx) error {
	user, err := clanViewer(c)
	if err != nil {
		return clanActionError(c, err, "/clans")
	}
	clanID, _ := strconv.Atoi(c.Params("id"))
	requestID, _ := strconv.ParseInt(c.Params("requestId"), 10, 64)
	if clanID <= 0 || requestID <= 0 {
		return clanActionError(c, service.ErrClanNotFound, "/clans")
	}
	if err := clanCheckCSRF(c); err != nil {
		return clanActionError(c, err, clanEditRedirect(c))
	}
	if service.Clan == nil {
		return clanActionError(c, errors.New("clan service unavailable"), "/clans")
	}

	if err := service.Clan.DenyJoinRequest(clanID, user.ID, user.HasAdminAccess(), requestID); err != nil {
		return clanActionError(c, err, clanEditRedirect(c))
	}
	return clanActionSucceed(c, clanEditRedirect(c), fiber.Map{"request_id": requestID})
}

func clanPermissionKeys(c fiber.Ctx) []string {
	var values []string
	if form, err := c.MultipartForm(); err == nil && form != nil {
		values = append(values, form.Value["permissions"]...)
	}
	if args := c.Request().PostArgs(); args != nil {
		for _, value := range args.PeekMulti("permissions") {
			values = append(values, string(value))
		}
	}
	if len(values) == 0 {
		values = append(values, c.FormValue("permissions"))
	}

	seen := make(map[string]bool)
	var keys []string
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			key := strings.TrimSpace(part)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			keys = append(keys, key)
		}
	}
	return keys
}

func ClanRoleCreatePost(c fiber.Ctx) error {
	user, err := clanViewer(c)
	if err != nil {
		return clanActionError(c, err, "/clans")
	}
	clanID, _ := strconv.Atoi(c.Params("id"))
	if clanID <= 0 {
		return clanActionError(c, service.ErrClanNotFound, "/clans")
	}
	if err := clanCheckCSRF(c); err != nil {
		return clanActionError(c, err, clanEditRedirect(c))
	}
	if service.Clan == nil {
		return clanActionError(c, errors.New("clan service unavailable"), "/clans")
	}

	rank, _ := strconv.Atoi(c.FormValue("rank"))
	roleID, err := service.Clan.CreateRole(clanID, user.ID, user.HasAdminAccess(), c.FormValue("name"), c.FormValue("color"), clanPermissionKeys(c), rank)
	if err != nil {
		return clanActionError(c, err, clanEditRedirect(c))
	}
	return clanActionSucceed(c, clanEditRedirect(c), fiber.Map{"id": roleID})
}

func ClanRoleUpdatePost(c fiber.Ctx) error {
	user, err := clanViewer(c)
	if err != nil {
		return clanActionError(c, err, "/clans")
	}
	clanID, _ := strconv.Atoi(c.Params("id"))
	roleID, _ := strconv.Atoi(c.Params("roleId"))
	if clanID <= 0 || roleID <= 0 {
		return clanActionError(c, service.ErrClanNotFound, "/clans")
	}
	if err := clanCheckCSRF(c); err != nil {
		return clanActionError(c, err, clanEditRedirect(c))
	}
	if service.Clan == nil {
		return clanActionError(c, errors.New("clan service unavailable"), "/clans")
	}

	rank, _ := strconv.Atoi(c.FormValue("rank"))
	if err := service.Clan.UpdateRole(clanID, user.ID, user.HasAdminAccess(), roleID, c.FormValue("name"), c.FormValue("color"), clanPermissionKeys(c), rank); err != nil {
		return clanActionError(c, err, clanEditRedirect(c))
	}
	return clanActionSucceed(c, clanEditRedirect(c), fiber.Map{"id": roleID})
}

func ClanRoleDeletePost(c fiber.Ctx) error {
	user, err := clanViewer(c)
	if err != nil {
		return clanActionError(c, err, "/clans")
	}
	clanID, _ := strconv.Atoi(c.Params("id"))
	roleID, _ := strconv.Atoi(c.Params("roleId"))
	if clanID <= 0 || roleID <= 0 {
		return clanActionError(c, service.ErrClanNotFound, "/clans")
	}
	if err := clanCheckCSRF(c); err != nil {
		return clanActionError(c, err, clanEditRedirect(c))
	}
	if service.Clan == nil {
		return clanActionError(c, errors.New("clan service unavailable"), "/clans")
	}

	if err := service.Clan.DeleteRole(clanID, user.ID, user.HasAdminAccess(), roleID); err != nil {
		return clanActionError(c, err, clanEditRedirect(c))
	}
	return clanActionSucceed(c, clanEditRedirect(c), fiber.Map{"id": roleID})
}

func ClanMemberRolePost(c fiber.Ctx) error {
	user, err := clanViewer(c)
	if err != nil {
		return clanActionError(c, err, "/clans")
	}
	clanID, _ := strconv.Atoi(c.Params("id"))
	targetID, _ := strconv.Atoi(c.Params("userId"))
	roleID, _ := strconv.Atoi(c.FormValue("role_id"))
	if clanID <= 0 || targetID <= 0 {
		return clanActionError(c, service.ErrClanNotFound, "/clans")
	}
	if err := clanCheckCSRF(c); err != nil {
		return clanActionError(c, err, clanEditRedirect(c))
	}
	if service.Clan == nil {
		return clanActionError(c, errors.New("clan service unavailable"), "/clans")
	}

	if err := service.Clan.SetMemberRole(clanID, user.ID, user.HasAdminAccess(), targetID, roleID); err != nil {
		return clanActionError(c, err, clanEditRedirect(c))
	}
	return clanActionSucceed(c, clanEditRedirect(c), fiber.Map{"user_id": targetID, "role_id": roleID})
}

func ClanMemberKickPost(c fiber.Ctx) error {
	user, err := clanViewer(c)
	if err != nil {
		return clanActionError(c, err, "/clans")
	}
	clanID, _ := strconv.Atoi(c.Params("id"))
	targetID, _ := strconv.Atoi(c.Params("userId"))
	if clanID <= 0 || targetID <= 0 {
		return clanActionError(c, service.ErrClanNotFound, "/clans")
	}
	if err := clanCheckCSRF(c); err != nil {
		return clanActionError(c, err, clanEditRedirect(c))
	}
	if service.Clan == nil {
		return clanActionError(c, errors.New("clan service unavailable"), "/clans")
	}

	if err := service.Clan.KickMember(clanID, user.ID, user.HasAdminAccess(), targetID); err != nil {
		return clanActionError(c, err, clanEditRedirect(c))
	}
	members, _ := service.Clan.MemberCount(clanID)
	return clanActionSucceed(c, clanEditRedirect(c), fiber.Map{"user_id": targetID, "members": members})
}

func ClanWallPost(c fiber.Ctx) error {
	user, err := clanViewer(c)
	if err != nil {
		return clanActionError(c, err, "/clans")
	}
	clanID, _ := strconv.Atoi(c.Params("id"))
	if clanID <= 0 {
		return clanActionError(c, service.ErrClanNotFound, "/clans")
	}
	if err := clanCheckCSRF(c); err != nil {
		return clanActionError(c, err, "/clans/"+strconv.Itoa(clanID))
	}
	if service.Clan == nil {
		return clanActionError(c, errors.New("clan service unavailable"), "/clans")
	}

	postID, err := service.Clan.CreateWallPost(clanID, user.ID, c.FormValue("body"))
	if err != nil {
		return clanActionError(c, err, "/clans/"+strconv.Itoa(clanID))
	}
	return clanActionSucceed(c, "/clans/"+strconv.Itoa(clanID), fiber.Map{"id": postID})
}

func ClanWallDeletePost(c fiber.Ctx) error {
	user, err := clanViewer(c)
	if err != nil {
		return clanActionError(c, err, "/clans")
	}
	clanID, _ := strconv.Atoi(c.Params("id"))
	postID, _ := strconv.ParseInt(c.Params("postId"), 10, 64)
	if clanID <= 0 || postID <= 0 {
		return clanActionError(c, service.ErrClanNotFound, "/clans")
	}
	if err := clanCheckCSRF(c); err != nil {
		return clanActionError(c, err, "/clans/"+strconv.Itoa(clanID))
	}
	if service.Clan == nil {
		return clanActionError(c, errors.New("clan service unavailable"), "/clans")
	}

	if err := service.Clan.DeleteWallPost(clanID, user.ID, user.HasAdminAccess(), postID); err != nil {
		return clanActionError(c, err, "/clans/"+strconv.Itoa(clanID))
	}
	return clanActionSucceed(c, "/clans/"+strconv.Itoa(clanID), fiber.Map{"id": postID})
}

func ClanWallReactPost(c fiber.Ctx) error {
	user, err := clanViewer(c)
	if err != nil {
		return clanActionError(c, err, "/clans")
	}
	clanID, _ := strconv.Atoi(c.Params("id"))
	postID, _ := strconv.ParseInt(c.Params("postId"), 10, 64)
	if clanID <= 0 || postID <= 0 {
		return clanActionError(c, service.ErrClanNotFound, "/clans")
	}
	if err := clanCheckCSRF(c); err != nil {
		return clanActionError(c, err, "/clans/"+strconv.Itoa(clanID))
	}
	if service.Clan == nil {
		return clanActionError(c, errors.New("clan service unavailable"), "/clans")
	}

	reacted, count, err := service.Clan.ToggleWallReaction(clanID, user.ID, postID)
	if err != nil {
		return clanActionError(c, err, "/clans/"+strconv.Itoa(clanID))
	}
	return clanActionSucceed(c, "/clans/"+strconv.Itoa(clanID), fiber.Map{"reacted": reacted, "count": count})
}

func ClanAnnouncementCreatePost(c fiber.Ctx) error {
	user, err := clanViewer(c)
	if err != nil {
		return clanActionError(c, err, "/clans")
	}
	clanID, _ := strconv.Atoi(c.Params("id"))
	if clanID <= 0 {
		return clanActionError(c, service.ErrClanNotFound, "/clans")
	}
	if err := clanCheckCSRF(c); err != nil {
		return clanActionError(c, err, "/clans/"+strconv.Itoa(clanID))
	}
	if service.Clan == nil {
		return clanActionError(c, errors.New("clan service unavailable"), "/clans")
	}

	announcementID, err := service.Clan.CreateAnnouncement(clanID, user.ID, user.HasAdminAccess(), c.FormValue("title"), c.FormValue("body"))
	if err != nil {
		return clanActionError(c, err, "/clans/"+strconv.Itoa(clanID))
	}
	return clanActionSucceed(c, "/clans/"+strconv.Itoa(clanID), fiber.Map{"id": announcementID})
}

func ClanAnnouncementDeletePost(c fiber.Ctx) error {
	user, err := clanViewer(c)
	if err != nil {
		return clanActionError(c, err, "/clans")
	}
	clanID, _ := strconv.Atoi(c.Params("id"))
	announcementID, _ := strconv.ParseInt(c.Params("announcementId"), 10, 64)
	if clanID <= 0 || announcementID <= 0 {
		return clanActionError(c, service.ErrClanNotFound, "/clans")
	}
	if err := clanCheckCSRF(c); err != nil {
		return clanActionError(c, err, "/clans/"+strconv.Itoa(clanID))
	}
	if service.Clan == nil {
		return clanActionError(c, errors.New("clan service unavailable"), "/clans")
	}

	if err := service.Clan.DeleteAnnouncement(clanID, user.ID, user.HasAdminAccess(), announcementID); err != nil {
		return clanActionError(c, err, "/clans/"+strconv.Itoa(clanID))
	}
	return clanActionSucceed(c, "/clans/"+strconv.Itoa(clanID), fiber.Map{"id": announcementID})
}

func AdminClanMediaQueueAPI(c fiber.Ctx) error {
	if _, err := getAdminUser(c); err != nil {
		return adminAPIError(c, err)
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "12"))

	if service.Clan == nil {
		return c.JSON(fiber.Map{
			"clans":       []*models.Clan{},
			"total":       0,
			"page":        page,
			"limit":       limit,
			"total_pages": 1,
		})
	}

	clans, total, err := service.Clan.GetPendingMediaClans(page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	items := make([]fiber.Map, 0, len(clans))
	for _, clan := range clans {
		items = append(items, fiber.Map{
			"id":             clan.ID,
			"name":           clan.Name,
			"slug":           clan.SlugPath(),
			"owner_id":       clan.OwnerID,
			"owner_name":     clan.OwnerName,
			"icon_pending":   clan.PendingIconURL(),
			"banner_pending": clan.PendingBannerURL(),
		})
	}

	totalPages := 1
	if total > 0 {
		totalPages = (total + limit - 1) / limit
	}

	return c.JSON(fiber.Map{
		"clans":       items,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": totalPages,
	})
}

func AdminClanMediaReviewPost(c fiber.Ctx) error {
	if _, err := getAdminUser(c); err != nil {
		return adminAuthError(c, err)
	}

	if !csrfValid(c) {
		return c.Status(fiber.StatusForbidden).SendString("Forbidden")
	}

	clanID, err := strconv.Atoi(c.Params("id"))
	if err != nil || clanID <= 0 {
		return adminModerationError(c, "Invalid clan ID")
	}

	kind := strings.ToLower(strings.TrimSpace(c.Params("kind")))
	if kind != "icon" && kind != "banner" {
		return adminModerationError(c, "Unknown media type")
	}

	action := c.Params("action")
	if action != "approve" && action != "reject" {
		return adminModerationError(c, "Unknown review action")
	}

	if service.Clan == nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Clan service unavailable")
	}

	var removed string
	if action == "approve" {
		removed, err = service.Clan.ApproveClanMedia(clanID, kind)
	} else {
		removed, err = service.Clan.RejectClanMedia(clanID, kind)
	}
	if err != nil {
		return adminModerationError(c, err.Error())
	}
	if p := clanMediaFilePath(removed); p != "" {
		_ = os.Remove(p)
	}

	if c.Get("Accept") == "application/json" || c.Get("X-Requested-With") == "XMLHttpRequest" || strings.HasPrefix(c.Path(), "/api/") {
		return c.JSON(fiber.Map{
			"success": true,
			"id":      clanID,
			"kind":    kind,
			"action":  action,
		})
	}

	return c.Redirect().To("/admin")
}
