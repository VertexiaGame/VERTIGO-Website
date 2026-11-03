package service

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/repository"
)

const (
	ClanNameMinLen        = 3
	ClanNameMaxLen        = 40
	ClanTagMinLen         = 2
	ClanTagMaxLen         = 5
	ClanDescriptionMaxLen = 500
	ClanJoinMessageMaxLen = 280
	ClanAnnounceTitleMax  = 120
	ClanAnnounceBodyMax   = 1000
	ClanWallPostMax       = 500
	ClanRoleNameMaxLen    = 24
	ClanRoleDefaultRank   = 50
	ClanCreateCost        = 150
)

var (
	ErrClanNotFound       = errors.New("clan not found")
	ErrClanPermission     = errors.New("you don't have permission to do that")
	ErrClanOwnerProtected = errors.New("the clan owner can't be changed")
	clanHexColorPattern   = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	clanTagPattern        = regexp.MustCompile(`^[A-Z0-9]{2,5}$`)
)

const ClanCreateAccountAge = 24 * time.Hour

type ClanService struct {
	clanRepo *repository.ClanRepository
	userRepo *repository.UserRepository
	cooldown *CooldownService
}

func NewClanService(clanRepo *repository.ClanRepository, userRepo *repository.UserRepository, cooldown *CooldownService) *ClanService {
	return &ClanService{clanRepo: clanRepo, userRepo: userRepo, cooldown: cooldown}
}

type ClanAccess struct {
	ViewerID       int
	IsAdmin        bool
	IsOwner        bool
	IsMember       bool
	Pending        bool
	CanViewContent bool
	CanJoin        bool
	CanRequest     bool
	Member         *models.ClanMember
	Role           *models.ClanRole
}

func (a *ClanAccess) Allows(bit uint64) bool {
	if a == nil {
		return false
	}
	if a.IsAdmin || a.IsOwner {
		return true
	}
	return a.Role != nil && a.Role.Permissions&bit != 0
}

func (a *ClanAccess) Rank() int {
	if a == nil {
		return 0
	}
	if a.IsAdmin || a.IsOwner {
		return models.ClanRankOwner
	}
	if a.Role != nil {
		return a.Role.Rank
	}
	return 0
}

func (a *ClanAccess) CanManageClan() bool {
	if a == nil {
		return false
	}
	if a.IsAdmin || a.IsOwner {
		return true
	}
	return a.Role != nil && a.Role.Permissions != 0
}

func (a *ClanAccess) CanModerateMembers() bool {
	return a.Allows(models.ClanPermissionManageMembers) ||
		a.Allows(models.ClanPermissionKickMembers) ||
		a.Allows(models.ClanPermissionManageRoles)
}

func (a *ClanAccess) CanPostWall() bool {
	return a != nil && a.CanViewContent && (a.IsMember || a.IsOwner)
}

type ClanInput struct {
	Name         string
	Description  string
	Category     string
	Color        string
	Tag          string
	TagColor     string
	ApprovalOnly bool
}

func (s *ClanService) GetPublicClans(viewerID int, search, category, sortBy string) ([]*models.Clan, error) {
	if s.clanRepo == nil {
		return []*models.Clan{}, nil
	}
	return s.clanRepo.GetPublicClans(viewerID, search, category, sortBy)
}

func (s *ClanService) GetJoinedClans(userID int) ([]*models.Clan, error) {
	if s.clanRepo == nil || userID <= 0 {
		return []*models.Clan{}, nil
	}
	return s.clanRepo.GetJoinedClans(userID)
}

func (s *ClanService) GetOwnedClans(userID int) ([]*models.Clan, error) {
	if s.clanRepo == nil || userID <= 0 {
		return []*models.Clan{}, nil
	}
	return s.clanRepo.GetOwnedClans(userID)
}

func (s *ClanService) GetClan(identifier string, viewerID int, isAdmin bool) (*models.Clan, *ClanAccess, error) {
	if s.clanRepo == nil {
		return nil, nil, errors.New("clan repository uninitialized")
	}
	clan, err := s.clanRepo.GetByIdentifier(identifier, viewerID)
	if err != nil {
		return nil, nil, err
	}
	if clan == nil {
		return nil, nil, ErrClanNotFound
	}
	member, err := s.clanRepo.GetMember(clan.ID, viewerID)
	if err != nil {
		return nil, nil, err
	}
	access := s.buildAccess(clan, viewerID, isAdmin, member)
	clan.IsOwner = access.IsOwner
	clan.Joined = access.IsMember
	return clan, access, nil
}

func (s *ClanService) GetClanPage(identifier string, viewerID int, isAdmin bool) (*models.Clan, *ClanAccess, error) {
	clan, access, err := s.GetClan(identifier, viewerID, isAdmin)
	if err != nil || clan == nil {
		return clan, access, err
	}

	if access.CanViewContent {
		members, merr := s.clanRepo.GetMembers(clan.ID)
		if merr != nil {
			return nil, access, merr
		}
		clan.Roster = members
		clan.Members = len(members)

		wall, werr := s.clanRepo.GetWallPosts(clan.ID, viewerID, 40)
		if werr != nil {
			return nil, access, werr
		}
		clan.Wall = wall

		announcements, aerr := s.clanRepo.GetAnnouncements(clan.ID, 10)
		if aerr != nil {
			return nil, access, aerr
		}
		clan.Announcements = announcements

		items, ierr := s.clanRepo.GetClanItems(clan.ID)
		if ierr != nil {
			return nil, access, ierr
		}
		clan.Items = items

		roles, rerr := s.clanRepo.GetRoles(clan.ID)
		if rerr != nil {
			return nil, access, rerr
		}
		clan.Roles = roles
	}

	if access.Allows(models.ClanPermissionManageMembers) {
		requests, rerr := s.clanRepo.GetJoinRequests(clan.ID)
		if rerr != nil {
			return nil, access, rerr
		}
		clan.Requests = requests
	}

	return clan, access, nil
}

func (s *ClanService) buildAccess(clan *models.Clan, viewerID int, isAdmin bool, member *models.ClanMember) *ClanAccess {
	access := &ClanAccess{
		ViewerID: viewerID,
		IsAdmin:  isAdmin,
		Member:   member,
		Pending:  clan.Pending,
	}
	if member != nil {
		access.IsMember = true
		access.Role = &models.ClanRole{
			ID:          member.RoleID,
			ClanID:      member.ClanID,
			Name:        member.RoleName,
			Color:       member.RoleColor,
			Permissions: member.RoleMask,
			Rank:        member.RoleRank,
			IsOwner:     member.IsOwnerRole,
		}
	}
	if viewerID > 0 && clan.OwnerID == viewerID {
		access.IsOwner = true
		access.IsMember = true
	}
	access.CanViewContent = true
	if clan.ApprovalOnly && !access.IsMember && !isAdmin {
		access.CanViewContent = false
	}
	access.CanJoin = viewerID > 0 && !access.IsMember && !clan.Pending
	access.CanRequest = access.CanJoin && clan.ApprovalOnly
	return access
}

func (s *ClanService) loadAccess(clanID int, actorID int, isAdmin bool) (*models.Clan, *ClanAccess, error) {
	if s.clanRepo == nil {
		return nil, nil, errors.New("clan repository uninitialized")
	}
	clan, err := s.clanRepo.GetByID(clanID, actorID)
	if err != nil {
		return nil, nil, err
	}
	if clan == nil {
		return nil, nil, ErrClanNotFound
	}
	member, err := s.clanRepo.GetMember(clanID, actorID)
	if err != nil {
		return nil, nil, err
	}
	return clan, s.buildAccess(clan, actorID, isAdmin, member), nil
}

func (s *ClanService) requireAccess(clanID, actorID int, isAdmin bool, bit uint64) (*models.Clan, *ClanAccess, error) {
	clan, access, err := s.loadAccess(clanID, actorID, isAdmin)
	if err != nil {
		return nil, nil, err
	}
	if !access.Allows(bit) {
		return nil, access, ErrClanPermission
	}
	return clan, access, nil
}

func normalizeClanName(name string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(name)), " ")
}

func normalizeClanText(text string) string {
	return strings.TrimSpace(text)
}

func normalizeClanColor(color, fallback string) (string, error) {
	color = strings.TrimSpace(color)
	if color == "" {
		color = fallback
	}
	if color == "" {
		return "", nil
	}
	if !strings.HasPrefix(color, "#") {
		color = "#" + color
	}
	if !clanHexColorPattern.MatchString(color) {
		return "", errors.New("invalid clan color")
	}
	return strings.ToUpper(color), nil
}

func normalizeClanTag(tag string) (string, error) {
	tag = strings.ToUpper(strings.TrimSpace(tag))
	if tag == "" {
		return "", errors.New("clan tag is required")
	}
	if !clanTagPattern.MatchString(tag) {
		return "", fmt.Errorf("clan tags must be %d to %d letters or numbers (A-Z, 0-9)", ClanTagMinLen, ClanTagMaxLen)
	}
	return tag, nil
}

func validateClanProfile(name, description string) (string, string, error) {
	name = normalizeClanName(name)
	description = normalizeClanText(description)
	if name == "" {
		return "", "", errors.New("clan name is required")
	}
	length := utf8.RuneCountInString(name)
	if length < ClanNameMinLen {
		return "", "", fmt.Errorf("clan name must be at least %d characters", ClanNameMinLen)
	}
	if length > ClanNameMaxLen {
		return "", "", fmt.Errorf("clan name can't be longer than %d characters", ClanNameMaxLen)
	}
	if utf8.RuneCountInString(description) > ClanDescriptionMaxLen {
		return "", "", fmt.Errorf("clan description can't be longer than %d characters", ClanDescriptionMaxLen)
	}
	return name, description, nil
}

func (s *ClanService) uniqueClanSlug(name string, excludeID int) (string, error) {
	base := models.ClanSlugify(name)
	if base == "" {
		base = "clan"
	}
	if len(base) > 60 {
		base = strings.Trim(base[:60], "-")
	}
	candidate := base
	for i := 2; i <= 99; i++ {
		exists, err := s.clanRepo.SlugExists(candidate, excludeID)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
		suffix := "-" + strconv.Itoa(i)
		trimmed := base
		if len(trimmed)+len(suffix) > 80 {
			trimmed = strings.Trim(trimmed[:80-len(suffix)], "-")
		}
		candidate = trimmed + suffix
	}
	return "", errors.New("couldn't generate a clan link, try a different name")
}

func (s *ClanService) CreateClan(owner *models.User, input ClanInput) (*models.Clan, error) {
	if s.clanRepo == nil {
		return nil, errors.New("clan repository uninitialized")
	}
	if owner == nil || owner.ID <= 0 {
		return nil, errors.New("you must be signed in")
	}
	if !owner.HasAdminAccess() && time.Since(owner.CreationDate) < ClanCreateAccountAge {
		return nil, errors.New("your account must be at least 1 day old to create a clan")
	}

	name, description, err := validateClanProfile(input.Name, input.Description)
	if err != nil {
		return nil, err
	}
	exists, err := s.clanRepo.NameExists(name, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("that clan name is already taken")
	}

	slug, err := s.uniqueClanSlug(name, 0)
	if err != nil {
		return nil, err
	}

	category, _ := models.NormalizeClanCategory(input.Category)
	color, err := normalizeClanColor(input.Color, models.ClanColorForName(name))
	if err != nil {
		return nil, err
	}
	tag, err := normalizeClanTag(input.Tag)
	if err != nil {
		return nil, err
	}
	tagExists, err := s.clanRepo.TagExists(tag, 0)
	if err != nil {
		return nil, err
	}
	if tagExists {
		return nil, errors.New("that clan tag is already taken")
	}
	tagColor, err := normalizeClanColor(input.TagColor, color)
	if err != nil {
		return nil, err
	}
	if tagColor == "" {
		tagColor = "#912ADF"
	}

	clan := &models.Clan{
		Name:         name,
		Slug:         slug,
		Description:  description,
		Category:     category,
		ApprovalOnly: input.ApprovalOnly,
		Color:        color,
		Tag:          tag,
		TagColor:     tagColor,
	}
	id, err := s.clanRepo.CreateClan(clan, owner.ID, ClanCreateCost)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return nil, errors.New("that clan name is already taken")
		}
		return nil, err
	}

	created, err := s.clanRepo.GetByID(id, owner.ID)
	if err != nil {
		return nil, err
	}
	if created == nil {
		created = clan
		created.ID = id
		created.OwnerID = owner.ID
		created.OwnerName = owner.Username
	}
	created.IsOwner = true
	created.Joined = true
	return created, nil
}

func (s *ClanService) UpdateClan(clanID, actorID int, isAdmin bool, input ClanInput) (*models.Clan, error) {
	clan, _, err := s.requireAccess(clanID, actorID, isAdmin, models.ClanPermissionEditClan)
	if err != nil {
		return nil, err
	}

	name, description, err := validateClanProfile(input.Name, input.Description)
	if err != nil {
		return nil, err
	}
	exists, err := s.clanRepo.NameExists(name, clan.ID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("that clan name is already taken")
	}

	slug := clan.Slug
	if name != clan.Name || slug == "" {
		slug, err = s.uniqueClanSlug(name, clan.ID)
		if err != nil {
			return nil, err
		}
	}

	category, _ := models.NormalizeClanCategory(input.Category)
	color, err := normalizeClanColor(input.Color, clan.Color)
	if err != nil {
		return nil, err
	}
	if color == "" {
		color = models.ClanColorForName(name)
	}
	tag, err := normalizeClanTag(input.Tag)
	if err != nil {
		return nil, err
	}
	tagExists, err := s.clanRepo.TagExists(tag, clan.ID)
	if err != nil {
		return nil, err
	}
	if tagExists {
		return nil, errors.New("that clan tag is already taken")
	}
	tagColor, err := normalizeClanColor(input.TagColor, clan.TagColor)
	if err != nil {
		return nil, err
	}
	if tagColor == "" {
		tagColor = clan.Color
	}

	if err := s.clanRepo.UpdateClanProfile(clan.ID, name, slug, description, category, color, tag, tagColor, input.ApprovalOnly); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return nil, errors.New("that clan name is already taken")
		}
		return nil, err
	}

	updated, err := s.clanRepo.GetByID(clan.ID, actorID)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		updated = clan
		updated.Name = name
		updated.Slug = slug
		updated.Description = description
		updated.Category = category
		updated.Color = color
		updated.Tag = tag
		updated.TagColor = tagColor
		updated.ApprovalOnly = input.ApprovalOnly
	}
	return updated, nil
}

func (s *ClanService) SetClanIcon(clanID, actorID int, isAdmin bool, icon string) error {
	_, _, err := s.requireAccess(clanID, actorID, isAdmin, models.ClanPermissionEditClan)
	if err != nil {
		return err
	}
	return s.clanRepo.SetClanIcon(clanID, strings.TrimSpace(icon))
}

func (s *ClanService) SetClanPendingIcon(clanID, actorID int, isAdmin bool, icon string) error {
	_, _, err := s.requireAccess(clanID, actorID, isAdmin, models.ClanPermissionEditClan)
	if err != nil {
		return err
	}
	return s.clanRepo.SetClanPendingIcon(clanID, strings.TrimSpace(icon))
}

func (s *ClanService) SetClanPendingBanner(clanID, actorID int, isAdmin bool, banner string) error {
	_, _, err := s.requireAccess(clanID, actorID, isAdmin, models.ClanPermissionEditClan)
	if err != nil {
		return err
	}
	return s.clanRepo.SetClanPendingBanner(clanID, strings.TrimSpace(banner))
}

func (s *ClanService) ApproveClanMedia(clanID int, kind string) (string, error) {
	if s.clanRepo == nil {
		return "", errors.New("clan service unavailable")
	}
	return s.clanRepo.ApproveClanMedia(clanID, kind)
}

func (s *ClanService) RejectClanMedia(clanID int, kind string) (string, error) {
	if s.clanRepo == nil {
		return "", errors.New("clan service unavailable")
	}
	return s.clanRepo.RejectClanMedia(clanID, kind)
}

func (s *ClanService) GetPendingMediaClans(page, limit int) ([]*models.Clan, int, error) {
	if s.clanRepo == nil {
		return []*models.Clan{}, 0, nil
	}
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 50 {
		limit = 12
	}
	return s.clanRepo.GetPendingMediaClans(limit, (page-1)*limit)
}

func (s *ClanService) WornClan(userID int) (*models.Clan, error) {
	if s.clanRepo == nil || userID <= 0 {
		return nil, nil
	}
	return s.clanRepo.GetWornClan(userID)
}

func (s *ClanService) ToggleWornTag(clanID, userID int) (bool, error) {
	clan, member, err := s.loadMembership(clanID, userID)
	if err != nil {
		return false, err
	}
	if member == nil && clan.OwnerID != userID {
		return false, errors.New("join this clan before wearing its tag")
	}
	if strings.TrimSpace(clan.Tag) == "" {
		return false, errors.New("this clan doesn't have a tag to wear")
	}

	current, err := s.clanRepo.GetPrimaryClan(userID)
	if err != nil {
		return false, err
	}
	if current == clanID {
		return false, s.clanRepo.SetPrimaryClan(userID, 0)
	}
	return true, s.clanRepo.SetPrimaryClan(userID, clanID)
}

func (s *ClanService) JoinClan(clanID, userID int, message string) (string, int, error) {
	if s.clanRepo == nil {
		return "", 0, errors.New("clan repository uninitialized")
	}
	if userID <= 0 {
		return "", 0, errors.New("you must be signed in")
	}
	clan, member, err := s.loadMembership(clanID, userID)
	if err != nil {
		return "", 0, err
	}
	if member != nil {
		return "", 0, errors.New("you're already a member of this clan")
	}
	count, err := s.clanRepo.CountMembers(clan.ID)
	if err != nil {
		return "", 0, err
	}

	if clan.ApprovalOnly {
		message = strings.TrimSpace(message)
		if utf8.RuneCountInString(message) > ClanJoinMessageMaxLen {
			return "", count, fmt.Errorf("join message can't be longer than %d characters", ClanJoinMessageMaxLen)
		}
		if err := s.clanRepo.CreateJoinRequest(clan.ID, userID, message); err != nil {
			return "", count, err
		}
		return "requested", count, nil
	}

	if count >= models.ClanMaxMembers {
		return "", count, errors.New("this clan is full")
	}
	role, err := s.clanRepo.GetDefaultRole(clan.ID)
	if err != nil {
		return "", count, err
	}
	if role == nil {
		return "", count, errors.New("this clan isn't accepting members right now")
	}
	if err := s.clanRepo.AddMember(clan.ID, userID, role.ID); err != nil {
		return "", count, err
	}
	return "joined", count + 1, nil
}

func (s *ClanService) MemberCount(clanID int) (int, error) {
	if s.clanRepo == nil {
		return 0, errors.New("clan repository uninitialized")
	}
	return s.clanRepo.CountMembers(clanID)
}

func (s *ClanService) loadMembership(clanID, userID int) (*models.Clan, *models.ClanMember, error) {
	if s.clanRepo == nil {
		return nil, nil, errors.New("clan repository uninitialized")
	}
	clan, err := s.clanRepo.GetByID(clanID, userID)
	if err != nil {
		return nil, nil, err
	}
	if clan == nil {
		return nil, nil, ErrClanNotFound
	}
	member, err := s.clanRepo.GetMember(clanID, userID)
	if err != nil {
		return nil, nil, err
	}
	return clan, member, nil
}

func (s *ClanService) LeaveClan(clanID, userID int) error {
	clan, member, err := s.loadMembership(clanID, userID)
	if err != nil {
		return err
	}
	if member == nil {
		return errors.New("you're not a member of this clan")
	}
	if clan.OwnerID == userID || member.IsOwnerRole {
		return errors.New("the owner can't leave the clan")
	}
	return s.clanRepo.RemoveMember(clan.ID, userID)
}

func (s *ClanService) CancelJoinRequest(clanID, userID int) error {
	if s.clanRepo == nil {
		return errors.New("clan repository uninitialized")
	}
	request, err := s.clanRepo.GetJoinRequest(clanID, userID)
	if err != nil {
		return err
	}
	if request == nil {
		return errors.New("you don't have a pending request for this clan")
	}
	return s.clanRepo.DeleteJoinRequest(clanID, userID)
}

func (s *ClanService) ApproveJoinRequest(clanID, actorID int, isAdmin bool, requestID int64, roleID int) (int, error) {
	_, access, err := s.requireAccess(clanID, actorID, isAdmin, models.ClanPermissionManageMembers)
	if err != nil {
		return 0, err
	}
	request, err := s.clanRepo.GetJoinRequestByID(clanID, requestID)
	if err != nil {
		return 0, err
	}
	if request == nil {
		return 0, errors.New("request not found")
	}

	if roleID <= 0 {
		role, rerr := s.clanRepo.GetDefaultRole(clanID)
		if rerr != nil {
			return 0, rerr
		}
		if role == nil {
			return 0, errors.New("this clan has no default role")
		}
		roleID = role.ID
	}
	role, err := s.clanRepo.GetRole(clanID, roleID)
	if err != nil {
		return 0, err
	}
	if role == nil {
		return 0, errors.New("role not found")
	}
	if !canAssignRole(access, role) {
		return 0, errors.New("you can't assign that role")
	}

	count, err := s.clanRepo.CountMembers(clanID)
	if err != nil {
		return 0, err
	}
	if count >= models.ClanMaxMembers {
		return 0, errors.New("this clan is full")
	}

	return s.clanRepo.ApproveJoinRequest(clanID, requestID, role.ID)
}

func (s *ClanService) DenyJoinRequest(clanID, actorID int, isAdmin bool, requestID int64) error {
	_, _, err := s.requireAccess(clanID, actorID, isAdmin, models.ClanPermissionManageMembers)
	if err != nil {
		return err
	}
	return s.clanRepo.DeleteJoinRequestByID(clanID, requestID)
}

func (s *ClanService) CreateRole(clanID, actorID int, isAdmin bool, name, color string, permissionKeys []string, rank int) (int, error) {
	_, access, err := s.requireAccess(clanID, actorID, isAdmin, models.ClanPermissionManageRoles)
	if err != nil {
		return 0, err
	}
	name, color, permissions, rank, err := validateRoleInput(access, name, color, permissionKeys, rank)
	if err != nil {
		return 0, err
	}

	roles, err := s.clanRepo.GetRoles(clanID)
	if err != nil {
		return 0, err
	}
	for _, existing := range roles {
		if strings.EqualFold(existing.Name, name) {
			return 0, errors.New("a role with that name already exists")
		}
	}

	return s.clanRepo.CreateRole(&models.ClanRole{
		ClanID:      clanID,
		Name:        name,
		Color:       color,
		Permissions: permissions,
		Rank:        rank,
	})
}

func (s *ClanService) UpdateRole(clanID, actorID int, isAdmin bool, roleID int, name, color string, permissionKeys []string, rank int) error {
	_, access, err := s.requireAccess(clanID, actorID, isAdmin, models.ClanPermissionManageRoles)
	if err != nil {
		return err
	}
	role, err := s.clanRepo.GetRole(clanID, roleID)
	if err != nil {
		return err
	}
	if role == nil {
		return errors.New("role not found")
	}
	if role.IsOwner {
		return errors.New("the owner role can't be edited")
	}
	if !canManageRole(access, role) {
		return errors.New("you can't edit that role")
	}

	name, color, permissions, rank, err := validateRoleInput(access, name, color, permissionKeys, rank)
	if err != nil {
		return err
	}

	roles, err := s.clanRepo.GetRoles(clanID)
	if err != nil {
		return err
	}
	for _, existing := range roles {
		if existing.ID != role.ID && strings.EqualFold(existing.Name, name) {
			return errors.New("a role with that name already exists")
		}
	}

	role.Name = name
	role.Color = color
	role.Permissions = permissions
	role.Rank = rank
	return s.clanRepo.UpdateRole(role)
}

func (s *ClanService) DeleteRole(clanID, actorID int, isAdmin bool, roleID int) error {
	_, access, err := s.requireAccess(clanID, actorID, isAdmin, models.ClanPermissionManageRoles)
	if err != nil {
		return err
	}
	role, err := s.clanRepo.GetRole(clanID, roleID)
	if err != nil {
		return err
	}
	if role == nil {
		return errors.New("role not found")
	}
	if role.IsOwner || role.IsDefault {
		return errors.New("this role can't be deleted")
	}
	if !canManageRole(access, role) {
		return errors.New("you can't delete that role")
	}
	return s.clanRepo.DeleteRole(clanID, roleID)
}

func (s *ClanService) SetMemberRole(clanID, actorID int, isAdmin bool, targetID, roleID int) error {
	_, access, err := s.requireAccess(clanID, actorID, isAdmin, models.ClanPermissionManageMembers)
	if err != nil {
		return err
	}
	target, err := s.clanRepo.GetMember(clanID, targetID)
	if err != nil {
		return err
	}
	if target == nil {
		return errors.New("member not found")
	}
	if target.IsOwnerRole {
		return ErrClanOwnerProtected
	}
	if !canActOnMember(access, target) {
		return errors.New("you can't change that member's role")
	}
	role, err := s.clanRepo.GetRole(clanID, roleID)
	if err != nil {
		return err
	}
	if role == nil {
		return errors.New("role not found")
	}
	if !canAssignRole(access, role) {
		return errors.New("you can't assign that role")
	}
	return s.clanRepo.SetMemberRole(clanID, targetID, role.ID)
}

func (s *ClanService) KickMember(clanID, actorID int, isAdmin bool, targetID int) error {
	_, access, err := s.requireAccess(clanID, actorID, isAdmin, models.ClanPermissionKickMembers)
	if err != nil {
		return err
	}
	if targetID == actorID {
		return errors.New("use leave instead of kicking yourself")
	}
	target, err := s.clanRepo.GetMember(clanID, targetID)
	if err != nil {
		return err
	}
	if target == nil {
		return errors.New("member not found")
	}
	if target.IsOwnerRole {
		return ErrClanOwnerProtected
	}
	if !canActOnMember(access, target) {
		return errors.New("you can't kick that member")
	}
	return s.clanRepo.RemoveMember(clanID, targetID)
}

func (s *ClanService) CreateWallPost(clanID, userID int, body string) (int64, error) {
	if s.clanRepo == nil {
		return 0, errors.New("clan repository uninitialized")
	}
	_, member, err := s.loadMembership(clanID, userID)
	if err != nil {
		return 0, err
	}
	if member == nil {
		return 0, errors.New("only members can post on the clan wall")
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return 0, errors.New("write something first")
	}
	if utf8.RuneCountInString(body) > ClanWallPostMax {
		return 0, fmt.Errorf("wall posts can't be longer than %d characters", ClanWallPostMax)
	}
	if s.cooldown != nil {
		if ok, remaining := s.cooldown.Allow(fmt.Sprintf("clan-wall:%d", userID), 5*time.Second); !ok {
			return 0, fmt.Errorf("slow down for %s", s.cooldown.FormatRemaining(remaining))
		}
	}
	return s.clanRepo.CreateWallPost(clanID, userID, body)
}

func (s *ClanService) DeleteWallPost(clanID, actorID int, isAdmin bool, postID int64) error {
	clan, access, err := s.loadAccess(clanID, actorID, isAdmin)
	if err != nil {
		return err
	}
	if !access.CanViewContent {
		return ErrClanPermission
	}
	authorID, err := s.clanRepo.GetWallPostAuthor(clan.ID, postID)
	if err != nil {
		return err
	}
	if authorID != actorID && !access.Allows(models.ClanPermissionManageWall) {
		return ErrClanPermission
	}
	return s.clanRepo.DeleteWallPost(clanID, postID)
}

func (s *ClanService) ToggleWallReaction(clanID, userID int, postID int64) (bool, int, error) {
	if s.clanRepo == nil {
		return false, 0, errors.New("clan repository uninitialized")
	}
	_, member, err := s.loadMembership(clanID, userID)
	if err != nil {
		return false, 0, err
	}
	if member == nil {
		return false, 0, errors.New("only members can react to clan posts")
	}
	if _, err := s.clanRepo.GetWallPostAuthor(clanID, postID); err != nil {
		return false, 0, err
	}
	reacted, err := s.clanRepo.ToggleWallReaction(postID, userID)
	if err != nil {
		return false, 0, err
	}
	count, err := s.clanRepo.CountWallReactions(postID)
	if err != nil {
		return false, 0, err
	}
	return reacted, count, nil
}

func (s *ClanService) CreateAnnouncement(clanID, actorID int, isAdmin bool, title, body string) (int64, error) {
	_, _, err := s.requireAccess(clanID, actorID, isAdmin, models.ClanPermissionPostAnnouncements)
	if err != nil {
		return 0, err
	}
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)
	if body == "" {
		return 0, errors.New("write something first")
	}
	if utf8.RuneCountInString(title) > ClanAnnounceTitleMax {
		return 0, fmt.Errorf("announcement title can't be longer than %d characters", ClanAnnounceTitleMax)
	}
	if utf8.RuneCountInString(body) > ClanAnnounceBodyMax {
		return 0, fmt.Errorf("announcements can't be longer than %d characters", ClanAnnounceBodyMax)
	}
	if s.cooldown != nil {
		if ok, remaining := s.cooldown.Allow(fmt.Sprintf("clan-announce:%d", actorID), 15*time.Second); !ok {
			return 0, fmt.Errorf("slow down for %s", s.cooldown.FormatRemaining(remaining))
		}
	}
	return s.clanRepo.CreateAnnouncement(clanID, actorID, title, body)
}

func (s *ClanService) DeleteAnnouncement(clanID, actorID int, isAdmin bool, announcementID int64) error {
	_, access, err := s.loadAccess(clanID, actorID, isAdmin)
	if err != nil {
		return err
	}
	authorID, err := s.clanRepo.GetAnnouncementAuthor(clanID, announcementID)
	if err != nil {
		return err
	}
	if authorID != actorID && !access.Allows(models.ClanPermissionPostAnnouncements) {
		return ErrClanPermission
	}
	return s.clanRepo.DeleteAnnouncement(clanID, announcementID)
}

func (s *ClanService) ClanMembers(clanID, viewerID int, isAdmin bool) ([]*models.ClanMember, error) {
	_, access, err := s.loadAccess(clanID, viewerID, isAdmin)
	if err != nil {
		return nil, err
	}
	if !access.CanViewContent {
		return nil, ErrClanPermission
	}
	return s.clanRepo.GetMembers(clanID)
}

func validateRoleInput(access *ClanAccess, name, color string, permissionKeys []string, rank int) (string, string, uint64, int, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", "", 0, 0, errors.New("role name is required")
	}
	if utf8.RuneCountInString(name) > ClanRoleNameMaxLen {
		return "", "", 0, 0, fmt.Errorf("role name can't be longer than %d characters", ClanRoleNameMaxLen)
	}
	if strings.EqualFold(name, "owner") {
		return "", "", 0, 0, errors.New("that role name is reserved")
	}

	permissions := models.ClanPermissionsFromKeys(permissionKeys)
	actorMask := uint64(0)
	if access.Role != nil {
		actorMask = access.Role.Permissions
	}
	if !access.IsAdmin && !access.IsOwner && permissions & ^actorMask != 0 {
		return "", "", 0, 0, errors.New("you can't grant permissions you don't have")
	}

	maxRank := models.ClanRankMax
	if !access.IsAdmin && !access.IsOwner {
		maxRank = access.Rank() - 1
	}
	if maxRank < 0 {
		maxRank = 0
	}
	if rank <= 0 {
		rank = min(ClanRoleDefaultRank, maxRank)
	}
	if rank > maxRank {
		return "", "", 0, 0, fmt.Errorf("role rank can't be higher than %d", maxRank)
	}

	color, err := normalizeClanColor(color, models.ClanColorForName(name))
	if err != nil {
		return "", "", 0, 0, err
	}
	if color == "" {
		color = "#912ADF"
	}
	return name, color, permissions, rank, nil
}

func canManageRole(access *ClanAccess, role *models.ClanRole) bool {
	if access == nil || role == nil {
		return false
	}
	if role.IsOwner {
		return false
	}
	if access.IsAdmin || access.IsOwner {
		return true
	}
	return access.Role != nil && access.Role.Rank > role.Rank
}

func canAssignRole(access *ClanAccess, role *models.ClanRole) bool {
	if access == nil || role == nil {
		return false
	}
	if role.IsOwner {
		return false
	}
	if access.IsAdmin || access.IsOwner {
		return true
	}
	return access.Role != nil && access.Role.Rank > role.Rank
}

func canActOnMember(access *ClanAccess, target *models.ClanMember) bool {
	if access == nil || target == nil {
		return false
	}
	if target.IsOwnerRole {
		return false
	}
	if access.IsAdmin || access.IsOwner {
		return true
	}
	return access.Role != nil && access.Role.Rank > target.RoleRank
}
