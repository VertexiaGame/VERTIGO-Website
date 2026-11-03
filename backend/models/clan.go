package models

import (
	"fmt"
	"html/template"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	ClanPermissionCreateGames uint64 = 1 << iota
	ClanPermissionManageGames
	ClanPermissionCreateItems
	ClanPermissionManageItems
	ClanPermissionManageMembers
	ClanPermissionKickMembers
	ClanPermissionManageRoles
	ClanPermissionEditClan
	ClanPermissionPostAnnouncements
	ClanPermissionManageWall
)

const (
	ClanRankOwner  = 100
	ClanRankMax    = 99
	ClanMaxMembers = 500
)

const ClanPermissionAll = ClanPermissionCreateGames |
	ClanPermissionManageGames |
	ClanPermissionCreateItems |
	ClanPermissionManageItems |
	ClanPermissionManageMembers |
	ClanPermissionKickMembers |
	ClanPermissionManageRoles |
	ClanPermissionEditClan |
	ClanPermissionPostAnnouncements |
	ClanPermissionManageWall

type ClanPermission struct {
	Bit         uint64
	Key         string
	Label       string
	Description string
	Icon        string
}

var ClanPermissions = []ClanPermission{
	{ClanPermissionCreateGames, "create_games", "Create games", "Publish games made for the clan", "fa-solid fa-gamepad"},
	{ClanPermissionManageGames, "manage_games", "Manage games", "Edit and remove the clan's games", "fa-solid fa-pen-to-square"},
	{ClanPermissionCreateItems, "create_items", "Create items", "Submit shop items under the clan's name", "fa-solid fa-shirt"},
	{ClanPermissionManageItems, "manage_items", "Manage items", "Edit and retire the clan's shop items", "fa-solid fa-boxes-stacked"},
	{ClanPermissionManageMembers, "manage_members", "Manage members", "Approve join requests and assign roles", "fa-solid fa-users-gear"},
	{ClanPermissionKickMembers, "kick_members", "Kick members", "Remove members from the clan", "fa-solid fa-user-slash"},
	{ClanPermissionManageRoles, "manage_roles", "Manage roles", "Create, edit and delete roles below your own", "fa-solid fa-shield-halved"},
	{ClanPermissionEditClan, "edit_clan", "Edit clan", "Change the clan's name, icon and details", "fa-solid fa-pen"},
	{ClanPermissionPostAnnouncements, "post_announcements", "Post announcements", "Write updates on the clan home page", "fa-solid fa-bullhorn"},
	{ClanPermissionManageWall, "manage_wall", "Manage wall", "Remove any post from the clan wall", "fa-solid fa-comment-slash"},
}

func ClanPermissionByKey(key string) (ClanPermission, bool) {
	for _, permission := range ClanPermissions {
		if permission.Key == key {
			return permission, true
		}
	}
	return ClanPermission{}, false
}

func ClanPermissionsFromKeys(keys []string) uint64 {
	var mask uint64
	for _, key := range keys {
		if permission, ok := ClanPermissionByKey(strings.TrimSpace(key)); ok {
			mask |= permission.Bit
		}
	}
	return mask
}

func ClanPermissionKeys(mask uint64) []string {
	var keys []string
	for _, permission := range ClanPermissions {
		if mask&permission.Bit != 0 {
			keys = append(keys, permission.Key)
		}
	}
	return keys
}

type ClanPermissionState struct {
	ClanPermission
	Checked bool
}

type Clan struct {
	ID            int
	Name          string
	Slug          string
	Description   string
	Category      string
	ApprovalOnly  bool
	Icon          string
	Banner        string
	IconPending   string
	BannerPending string
	Color         string
	Tag           string
	TagColor      string
	OwnerID       int
	OwnerName     string
	CreatedAt     time.Time
	Members       int
	WorldCount    int
	ItemCount     int
	Joined        bool
	Pending       bool
	IsOwner       bool
	Announcements []*ClanAnnouncement
	Wall          []*ClanWallPost
	Roster        []*ClanMember
	Roles         []*ClanRole
	Requests      []*ClanJoinRequest
	Items         []*ShopItem
	Worlds        []*Game
}

func (c *Clan) Letter() string {
	name := strings.TrimSpace(c.Name)
	if name == "" {
		return "C"
	}
	r, _ := utf8.DecodeRuneInString(name)
	if unicode.IsLetter(r) {
		return strings.ToUpper(string(r))
	}
	return "C"
}

func (c *Clan) HasIcon() bool {
	return strings.TrimSpace(c.Icon) != ""
}

func (c *Clan) HasTag() bool {
	return c != nil && strings.TrimSpace(c.Tag) != ""
}

func (c *Clan) TagLabel() string {
	if !c.HasTag() {
		return ""
	}
	return "[" + strings.ToUpper(strings.TrimSpace(c.Tag)) + "]"
}

func (c *Clan) TagColorValue() string {
	if c == nil {
		return "#912ADF"
	}
	color := strings.TrimSpace(c.TagColor)
	if color == "" {
		color = strings.TrimSpace(c.Color)
	}
	if color == "" {
		return "#912ADF"
	}
	if !strings.HasPrefix(color, "#") {
		color = "#" + color
	}
	return color
}

func (c *Clan) TagStyle() template.CSS {
	return template.CSS("--cln-tag-color: " + c.TagColorValue() + ";")
}

type ClanTag struct {
	Label string       `json:"label"`
	Slug  string       `json:"slug"`
	Name  string       `json:"name"`
	Style template.CSS `json:"style"`
}

func (c *Clan) TagInfo() *ClanTag {
	if !c.HasTag() {
		return nil
	}
	return &ClanTag{
		Label: c.TagLabel(),
		Slug:  c.SlugPath(),
		Name:  strings.TrimSpace(c.Name),
		Style: c.TagStyle(),
	}
}

func (c *Clan) IconURL() string {
	return strings.TrimSpace(c.Icon)
}

func (c *Clan) HasBanner() bool {
	return c != nil && strings.TrimSpace(c.Banner) != ""
}

func (c *Clan) BannerURL() string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.Banner)
}

func (c *Clan) HasPendingIcon() bool {
	return c != nil && strings.TrimSpace(c.IconPending) != ""
}

func (c *Clan) HasPendingBanner() bool {
	return c != nil && strings.TrimSpace(c.BannerPending) != ""
}

func (c *Clan) HasPendingMedia() bool {
	return c.HasPendingIcon() || c.HasPendingBanner()
}

func (c *Clan) PendingIconURL() string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.IconPending)
}

func (c *Clan) PendingBannerURL() string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.BannerPending)
}

func (c *Clan) PrivacyLabel() string {
	if c.ApprovalOnly {
		return "Approval only"
	}
	return "Public"
}

func (c *Clan) CategoryLabel() string {
	_, label := NormalizeClanCategory(c.Category)
	return label
}

func (c *Clan) CatsAttr() string {
	return NormalizeClanCategoryKey(c.Category)
}

func (c *Clan) CreatedLabel() string {
	if c.CreatedAt.IsZero() {
		return "Recently"
	}
	return c.CreatedAt.Format("1/2/2006")
}

func (c *Clan) Days() int {
	if c.CreatedAt.IsZero() {
		return 0
	}
	days := int(time.Since(c.CreatedAt).Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}

func (c *Clan) DescriptionText() string {
	desc := strings.TrimSpace(c.Description)
	if desc == "" {
		return "This clan hasn't written a description yet."
	}
	return desc
}

func (c *Clan) SlugPath() string {
	if c.Slug != "" {
		return c.Slug
	}
	return ClanSlugify(c.Name)
}

type ClanRole struct {
	ID          int
	ClanID      int
	Name        string
	Color       string
	Permissions uint64
	Rank        int
	IsOwner     bool
	IsDefault   bool
	MemberCount int
	CreatedAt   time.Time
}

func (r *ClanRole) HasPermission(bit uint64) bool {
	return r != nil && r.Permissions&bit != 0
}

func (r *ClanRole) IsReserved() bool {
	return r != nil && (r.IsOwner || r.IsDefault)
}

func (r *ClanRole) BadgeStyle() template.CSS {
	color := strings.TrimSpace(r.Color)
	if color == "" {
		color = "#912ADF"
	}
	return template.CSS("--cln-role-color: " + color + ";")
}

func (r *ClanRole) PermissionStates() []ClanPermissionState {
	states := make([]ClanPermissionState, 0, len(ClanPermissions))
	for _, permission := range ClanPermissions {
		states = append(states, ClanPermissionState{
			ClanPermission: permission,
			Checked:        r != nil && r.Permissions&permission.Bit != 0,
		})
	}
	return states
}

func (r *ClanRole) PermissionCount() int {
	if r == nil {
		return 0
	}
	count := 0
	for _, permission := range ClanPermissions {
		if r.Permissions&permission.Bit != 0 {
			count++
		}
	}
	return count
}

func (r *ClanRole) PermissionKeyString() string {
	if r == nil {
		return ""
	}
	return strings.Join(ClanPermissionKeys(r.Permissions), ",")
}

type ClanMember struct {
	MembershipID int64
	ClanID       int
	UserID       int
	Username     string
	DisplayName  string
	RoleID       int
	RoleName     string
	RoleColor    string
	RoleRank     int
	RoleMask     uint64
	IsOwnerRole  bool
	JoinedAt     time.Time
	LastOnline   time.Time
}

func (m *ClanMember) Name() string {
	if strings.TrimSpace(m.DisplayName) != "" {
		return m.DisplayName
	}
	return m.Username
}

func (m *ClanMember) IsOnline() bool {
	return !m.LastOnline.IsZero() && time.Since(m.LastOnline) < 5*time.Minute
}

func (m *ClanMember) Status() string {
	if m.IsOnline() {
		return "Online"
	}
	return "Offline"
}

func (m *ClanMember) JoinedLabel() string {
	if m.JoinedAt.IsZero() {
		return ""
	}
	return m.JoinedAt.Format("1/2/2006")
}

func (m *ClanMember) BadgeStyle() template.CSS {
	color := strings.TrimSpace(m.RoleColor)
	if color == "" {
		color = "#912ADF"
	}
	return template.CSS("--cln-role-color: " + color + ";")
}

func (m *ClanMember) HasPermission(bit uint64) bool {
	return m != nil && (m.IsOwnerRole || m.RoleMask&bit != 0)
}

type ClanJoinRequest struct {
	ID          int64
	ClanID      int
	UserID      int
	Username    string
	DisplayName string
	Message     string
	CreatedAt   time.Time
}

func (r *ClanJoinRequest) Name() string {
	if strings.TrimSpace(r.DisplayName) != "" {
		return r.DisplayName
	}
	return r.Username
}

func (r *ClanJoinRequest) CreatedLabel() string {
	return RelativeClanTime(r.CreatedAt)
}

type ClanAnnouncement struct {
	ID              int64
	ClanID          int
	UserID          int
	AuthorName      string
	AuthorDisplay   string
	AuthorRoleName  string
	AuthorRoleColor string
	Title           string
	Body            string
	CreatedAt       time.Time
}

func (a *ClanAnnouncement) Author() string {
	if strings.TrimSpace(a.AuthorDisplay) != "" {
		return a.AuthorDisplay
	}
	return a.AuthorName
}

func (a *ClanAnnouncement) TimeAgo() string {
	return RelativeClanTime(a.CreatedAt)
}

func (a *ClanAnnouncement) FullDate() string {
	if a.CreatedAt.IsZero() {
		return ""
	}
	return a.CreatedAt.Format("January 02, 2006 at 03:04 PM")
}

func (a *ClanAnnouncement) BadgeStyle() template.CSS {
	color := strings.TrimSpace(a.AuthorRoleColor)
	if color == "" {
		color = "#912ADF"
	}
	return template.CSS("--cln-role-color: " + color + ";")
}

type ClanWallPost struct {
	ID            int64
	ClanID        int
	UserID        int
	AuthorName    string
	AuthorDisplay string
	RoleName      string
	RoleColor     string
	Body          string
	CreatedAt     time.Time
	Reactions     int
	HasReacted    bool
}

func (p *ClanWallPost) Author() string {
	if strings.TrimSpace(p.AuthorDisplay) != "" {
		return p.AuthorDisplay
	}
	return p.AuthorName
}

func (p *ClanWallPost) TimeAgo() string {
	return RelativeClanTime(p.CreatedAt)
}

func (p *ClanWallPost) FullDate() string {
	if p.CreatedAt.IsZero() {
		return ""
	}
	return p.CreatedAt.Format("January 02, 2006 at 03:04 PM")
}

func (p *ClanWallPost) BadgeStyle() template.CSS {
	color := strings.TrimSpace(p.RoleColor)
	if color == "" {
		color = "#912ADF"
	}
	return template.CSS("--cln-role-color: " + color + ";")
}

func RelativeClanTime(t time.Time) string {
	diff := time.Since(t)
	if diff < 0 {
		diff = 0
	}
	switch {
	case diff < time.Minute:
		return "Just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins <= 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours <= 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case diff < 30*24*time.Hour:
		days := int(diff.Hours() / 24)
		if days <= 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		return t.Format("Jan 02, 2006")
	}
}

type ClanCategory struct {
	Key   string
	Label string
}

var ClanCategories = []ClanCategory{
	{"building", "Building"},
	{"social", "Social"},
	{"grinding", "Grinding"},
	{"roleplay", "Roleplay"},
	{"games", "Games"},
	{"trading", "Trading"},
	{"creative", "Creative"},
	{"music", "Music"},
	{"ugc", "UGC"},
}

var clanCategoryAliases = map[string]string{
	"build":            "building",
	"builds":           "building",
	"grind":            "grinding",
	"grind-heavy":      "grinding",
	"rp":               "roleplay",
	"pvp":              "games",
	"player-vs-player": "games",
	"trade":            "trading",
	"trades":           "trading",
	"economy":          "trading",
	"art":              "creative",
	"arts":             "creative",
	"design":           "creative",
	"audio":            "music",
	"explore":          "ugc",
	"exploring":        "ugc",
	"exploration":      "ugc",
	"adventure":        "ugc",
	"user-generated":   "ugc",
}

func NormalizeClanCategoryKey(raw string) string {
	key, _ := NormalizeClanCategory(raw)
	return key
}

func NormalizeClanCategory(raw string) (string, string) {
	key := strings.ToLower(strings.TrimSpace(raw))
	if alias, ok := clanCategoryAliases[key]; ok {
		key = alias
	}
	for _, category := range ClanCategories {
		if category.Key == key {
			return category.Key, category.Label
		}
	}
	return "social", "Social"
}

var clanColorPalette = []string{
	"#7423CB",
	"#912ADF",
	"#28A745",
	"#DC3545",
	"#E67E22",
	"#17A2B8",
	"#3D3D3D",
}

func ClanColorForName(name string) string {
	seed := 0
	for _, r := range name {
		seed += int(r)
	}
	if seed < 0 {
		seed = -seed
	}
	return clanColorPalette[seed%len(clanColorPalette)]
}

func ClanSlugify(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var out strings.Builder
	lastDash := false
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out.WriteRune(r)
			lastDash = false
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			out.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && out.Len() > 0 {
				out.WriteByte('-')
				lastDash = true
			}
		}
		if out.Len() >= 64 {
			break
		}
	}
	slug := strings.Trim(out.String(), "-")
	if slug == "" {
		return "clan"
	}
	return slug
}
