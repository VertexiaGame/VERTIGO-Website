package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"vertexia-frontend/backend/models"
)

type ClanRepository struct {
	db *sql.DB
}

func NewClanRepository(db *sql.DB) *ClanRepository {
	return &ClanRepository{db: db}
}

const clanViewerSelect = `SELECT c.id, c.name, c.slug, COALESCE(c.description, ''), c.category, c.approval_only, c.icon, c.banner, c.icon_pending, c.banner_pending, c.color,
	COALESCE(c.tag, ''), c.tag_color, c.owner_id,
	COALESCE(NULLIF(u.username, ''), 'VERTEXIA'),
	c.created_at,
	(SELECT COUNT(*) FROM clan_members cm WHERE cm.clan_id = c.id) AS member_count,
	(SELECT COUNT(*) FROM shopitem si WHERE si.creator_type = 'clan' AND si.cid = c.id AND si.deleted = 0 AND si.approval_state = 'approved') AS item_count,
	EXISTS(SELECT 1 FROM clan_members vm WHERE vm.clan_id = c.id AND vm.user_id = ?) AS joined,
	EXISTS(SELECT 1 FROM clan_join_requests vr WHERE vr.clan_id = c.id AND vr.user_id = ?) AS pending
	FROM clans c
	LEFT JOIN users u ON c.owner_id = u.id`

func scanClan(scanner interface{ Scan(dest ...any) error }) (*models.Clan, error) {
	clan := &models.Clan{}
	err := scanner.Scan(
		&clan.ID,
		&clan.Name,
		&clan.Slug,
		&clan.Description,
		&clan.Category,
		&clan.ApprovalOnly,
		&clan.Icon,
		&clan.Banner,
		&clan.IconPending,
		&clan.BannerPending,
		&clan.Color,
		&clan.Tag,
		&clan.TagColor,
		&clan.OwnerID,
		&clan.OwnerName,
		&clan.CreatedAt,
		&clan.Members,
		&clan.ItemCount,
		&clan.Joined,
		&clan.Pending,
	)
	if err != nil {
		return nil, err
	}
	clan.IsOwner = false
	return clan, nil
}

func (r *ClanRepository) getClan(query string, args ...any) (*models.Clan, error) {
	if r.db == nil {
		return nil, errors.New("database connection is offline")
	}
	clan, err := scanClan(r.db.QueryRow(query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return clan, err
}

func (r *ClanRepository) GetByID(id int, viewerID int) (*models.Clan, error) {
	if id <= 0 {
		return nil, nil
	}
	return r.getClan(clanViewerSelect+" WHERE c.id = ? LIMIT 1", viewerID, viewerID, id)
}

func (r *ClanRepository) GetBySlug(slug string, viewerID int) (*models.Clan, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, nil
	}
	return r.getClan(clanViewerSelect+" WHERE c.slug = ? LIMIT 1", viewerID, viewerID, slug)
}

func (r *ClanRepository) GetByName(name string, viewerID int) (*models.Clan, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	return r.getClan(clanViewerSelect+" WHERE c.name = ? LIMIT 1", viewerID, viewerID, name)
}

func (r *ClanRepository) GetByIdentifier(identifier string, viewerID int) (*models.Clan, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, nil
	}
	id, err := strconv.Atoi(identifier)
	if err == nil && id > 0 {
		if clan, err := r.GetByID(id, viewerID); err != nil || clan != nil {
			return clan, err
		}
	}
	if clan, err := r.GetBySlug(identifier, viewerID); err != nil || clan != nil {
		return clan, err
	}
	return r.GetByName(identifier, viewerID)
}

func (r *ClanRepository) GetPublicClans(viewerID int, search, category, sortBy string) ([]*models.Clan, error) {
	if r.db == nil {
		return []*models.Clan{}, nil
	}
	where := []string{"c.approval_only = 0"}
	args := []any{viewerID, viewerID}
	search = strings.TrimSpace(search)
	if search != "" {
		where = append(where, "(c.name LIKE ? OR c.description LIKE ?)")
		pattern := "%" + search + "%"
		args = append(args, pattern, pattern)
	}
	rawCategory := strings.ToLower(strings.TrimSpace(category))
	if rawCategory != "" && rawCategory != "all" {
		key, _ := models.NormalizeClanCategory(rawCategory)
		where = append(where, "c.category = ?")
		args = append(args, key)
	}
	order := "member_count DESC, c.created_at DESC"
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "newest":
		order = "c.created_at DESC"
	case "oldest":
		order = "c.created_at ASC"
	case "name", "name-asc":
		order = "c.name ASC"
	case "members-asc":
		order = "member_count ASC, c.created_at DESC"
	}
	query := clanViewerSelect + " WHERE " + strings.Join(where, " AND ") + " ORDER BY " + order + " LIMIT 200"
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var clans []*models.Clan
	for rows.Next() {
		clan, err := scanClan(rows)
		if err != nil {
			return nil, err
		}
		clans = append(clans, clan)
	}
	return clans, rows.Err()
}

func (r *ClanRepository) GetJoinedClans(userID int) ([]*models.Clan, error) {
	if r.db == nil || userID <= 0 {
		return []*models.Clan{}, nil
	}
	query := clanViewerSelect + ` JOIN clan_members jm ON jm.clan_id = c.id AND jm.user_id = ?
		WHERE 1 = 1
		ORDER BY jm.joined_at DESC, c.name ASC`
	rows, err := r.db.Query(query, userID, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var clans []*models.Clan
	for rows.Next() {
		clan, err := scanClan(rows)
		if err != nil {
			return nil, err
		}
		clan.Joined = true
		clans = append(clans, clan)
	}
	return clans, rows.Err()
}

func (r *ClanRepository) GetOwnedClans(ownerID int) ([]*models.Clan, error) {
	if r.db == nil || ownerID <= 0 {
		return []*models.Clan{}, nil
	}
	rows, err := r.db.Query(clanViewerSelect+" WHERE c.owner_id = ? ORDER BY c.created_at DESC, c.name ASC", ownerID, ownerID, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var clans []*models.Clan
	for rows.Next() {
		clan, err := scanClan(rows)
		if err != nil {
			return nil, err
		}
		clan.IsOwner = true
		clan.Joined = true
		clans = append(clans, clan)
	}
	return clans, rows.Err()
}

func (r *ClanRepository) CountOwnedClans(ownerID int) (int, error) {
	if r.db == nil || ownerID <= 0 {
		return 0, nil
	}
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM clans WHERE owner_id = ?", ownerID).Scan(&count)
	return count, err
}

func (r *ClanRepository) CreateClan(clan *models.Clan, ownerID int, cost int) (int, error) {
	if r.db == nil {
		return 0, errors.New("database connection is offline")
	}
	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	var bucks int
	if err := tx.QueryRow("SELECT bucks FROM users WHERE id = ? FOR UPDATE", ownerID).Scan(&bucks); err != nil {
		return 0, errors.New("user not found")
	}
	if bucks < cost {
		return 0, fmt.Errorf("you need %d Vertices to create a clan", cost)
	}

	res, err := tx.Exec(
		"INSERT INTO clans (name, slug, description, category, approval_only, icon, banner, color, tag, tag_color, owner_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		clan.Name, clan.Slug, clan.Description, clan.Category, clan.ApprovalOnly, clan.Icon, clan.Banner, clan.Color, nullableClanTag(clan.Tag), clan.TagColor, ownerID,
	)
	if err != nil {
		return 0, err
	}
	lastID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	clanID := int(lastID)

	ownerRole, err := tx.Exec(
		"INSERT INTO clan_roles (clan_id, name, color, permissions, role_rank, is_owner, is_default) VALUES (?, 'Owner', '#7423CB', ?, ?, 1, 0)",
		clanID, models.ClanPermissionAll, models.ClanRankOwner,
	)
	if err != nil {
		return 0, err
	}
	ownerRoleID, err := ownerRole.LastInsertId()
	if err != nil {
		return 0, err
	}

	if _, err := tx.Exec(
		"INSERT INTO clan_roles (clan_id, name, color, permissions, role_rank, is_owner, is_default) VALUES (?, 'Member', '#718096', 0, 0, 0, 1)",
		clanID,
	); err != nil {
		return 0, err
	}

	if _, err := tx.Exec(
		"INSERT INTO clan_members (clan_id, user_id, role_id) VALUES (?, ?, ?)",
		clanID, ownerID, ownerRoleID,
	); err != nil {
		return 0, err
	}
	if _, err := tx.Exec("UPDATE users SET primary_clan = ? WHERE id = ? AND primary_clan = 0", clanID, ownerID); err != nil {
		return 0, err
	}
	if cost > 0 {
		if _, err := tx.Exec("UPDATE users SET bucks = bucks - ? WHERE id = ?", cost, ownerID); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	committed = true
	clan.ID = clanID
	return clanID, nil
}

func (r *ClanRepository) UpdateClanProfile(id int, name, slug, description, category, color, tag, tagColor string, approvalOnly bool) error {
	if r.db == nil {
		return errors.New("database connection is offline")
	}
	_, err := r.db.Exec(
		"UPDATE clans SET name = ?, slug = ?, description = ?, category = ?, color = ?, tag = ?, tag_color = ?, approval_only = ? WHERE id = ?",
		name, slug, description, category, color, nullableClanTag(tag), tagColor, approvalOnly, id,
	)
	return err
}

func nullableClanTag(tag string) any {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return nil
	}
	return tag
}

func (r *ClanRepository) SetClanIcon(id int, icon string) error {
	if r.db == nil {
		return errors.New("database connection is offline")
	}
	_, err := r.db.Exec("UPDATE clans SET icon = ? WHERE id = ?", icon, id)
	return err
}

func (r *ClanRepository) SetClanPendingIcon(id int, icon string) error {
	if r.db == nil {
		return errors.New("database connection is offline")
	}
	_, err := r.db.Exec("UPDATE clans SET icon_pending = ? WHERE id = ?", icon, id)
	return err
}

func (r *ClanRepository) SetClanPendingBanner(id int, banner string) error {
	if r.db == nil {
		return errors.New("database connection is offline")
	}
	_, err := r.db.Exec("UPDATE clans SET banner_pending = ? WHERE id = ?", banner, id)
	return err
}

func clanMediaColumns(kind string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "icon":
		return "icon", "icon_pending"
	case "banner":
		return "banner", "banner_pending"
	default:
		return "", ""
	}
}

func (r *ClanRepository) ApproveClanMedia(id int, kind string) (string, error) {
	if r.db == nil {
		return "", errors.New("database connection is offline")
	}
	live, pending := clanMediaColumns(kind)
	if live == "" {
		return "", errors.New("unknown media type")
	}
	var current, queued string
	row := r.db.QueryRow("SELECT COALESCE("+live+", ''), COALESCE("+pending+", '') FROM clans WHERE id = ?", id)
	if err := row.Scan(&current, &queued); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("clan not found")
		}
		return "", err
	}
	if strings.TrimSpace(queued) == "" {
		return "", errors.New("there is nothing awaiting review for this clan")
	}
	if _, err := r.db.Exec("UPDATE clans SET "+live+" = "+pending+", "+pending+" = '' WHERE id = ?", id); err != nil {
		return "", err
	}
	return current, nil
}

func (r *ClanRepository) RejectClanMedia(id int, kind string) (string, error) {
	if r.db == nil {
		return "", errors.New("database connection is offline")
	}
	_, pending := clanMediaColumns(kind)
	if pending == "" {
		return "", errors.New("unknown media type")
	}
	var queued string
	row := r.db.QueryRow("SELECT COALESCE("+pending+", '') FROM clans WHERE id = ?", id)
	if err := row.Scan(&queued); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("clan not found")
		}
		return "", err
	}
	if strings.TrimSpace(queued) == "" {
		return "", errors.New("there is nothing awaiting review for this clan")
	}
	if _, err := r.db.Exec("UPDATE clans SET "+pending+" = '' WHERE id = ?", id); err != nil {
		return "", err
	}
	return queued, nil
}

func (r *ClanRepository) GetPendingMediaClans(limit, offset int) ([]*models.Clan, int, error) {
	if r.db == nil {
		return []*models.Clan{}, 0, nil
	}
	where := " WHERE (COALESCE(c.icon_pending, '') <> '' OR COALESCE(c.banner_pending, '') <> '')"
	rows, err := r.db.Query(clanViewerSelect+where+" ORDER BY c.updated_at DESC, c.id DESC LIMIT ? OFFSET ?", 0, 0, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	clans := []*models.Clan{}
	for rows.Next() {
		clan, scanErr := scanClan(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		clans = append(clans, clan)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var total int
	if err := r.db.QueryRow("SELECT COUNT(*) FROM clans WHERE (COALESCE(icon_pending, '') <> '' OR COALESCE(banner_pending, '') <> '')").Scan(&total); err != nil {
		return nil, 0, err
	}
	return clans, total, nil
}

func (r *ClanRepository) NameExists(name string, excludeID int) (bool, error) {
	if r.db == nil {
		return false, errors.New("database connection is offline")
	}
	var exists bool
	err := r.db.QueryRow("SELECT EXISTS(SELECT 1 FROM clans WHERE name = ? AND id != ?)", name, excludeID).Scan(&exists)
	return exists, err
}

func (r *ClanRepository) SlugExists(slug string, excludeID int) (bool, error) {
	if r.db == nil {
		return false, errors.New("database connection is offline")
	}
	var exists bool
	err := r.db.QueryRow("SELECT EXISTS(SELECT 1 FROM clans WHERE slug = ? AND id != ?)", slug, excludeID).Scan(&exists)
	return exists, err
}

func (r *ClanRepository) TagExists(tag string, excludeID int) (bool, error) {
	if r.db == nil {
		return false, errors.New("database connection is offline")
	}
	var exists bool
	err := r.db.QueryRow("SELECT EXISTS(SELECT 1 FROM clans WHERE tag = ? AND id != ?)", strings.ToUpper(strings.TrimSpace(tag)), excludeID).Scan(&exists)
	return exists, err
}

func (r *ClanRepository) GetWornClan(userID int) (*models.Clan, error) {
	if userID <= 0 {
		return nil, nil
	}
	return r.getClan(clanViewerSelect+" JOIN users wu ON wu.primary_clan = c.id WHERE wu.id = ? LIMIT 1", userID, userID, userID)
}

func (r *ClanRepository) GetPrimaryClan(userID int) (int, error) {
	if r.db == nil || userID <= 0 {
		return 0, nil
	}
	var clanID int
	err := r.db.QueryRow("SELECT primary_clan FROM users WHERE id = ?", userID).Scan(&clanID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return clanID, err
}

func (r *ClanRepository) SetPrimaryClan(userID, clanID int) error {
	if r.db == nil {
		return errors.New("database connection is offline")
	}
	_, err := r.db.Exec("UPDATE users SET primary_clan = ? WHERE id = ?", clanID, userID)
	return err
}

const clanMemberSelect = `SELECT m.id, m.clan_id, m.user_id, u.username, u.displayname, m.role_id, r.name, r.color,
	r.role_rank, r.permissions, r.is_owner, m.joined_at, u.last_online
	FROM clan_members m
	JOIN users u ON u.id = m.user_id
	JOIN clan_roles r ON r.id = m.role_id`

func scanClanMember(scanner interface{ Scan(dest ...any) error }) (*models.ClanMember, error) {
	member := &models.ClanMember{}
	var lastOnline sql.NullTime
	err := scanner.Scan(
		&member.MembershipID,
		&member.ClanID,
		&member.UserID,
		&member.Username,
		&member.DisplayName,
		&member.RoleID,
		&member.RoleName,
		&member.RoleColor,
		&member.RoleRank,
		&member.RoleMask,
		&member.IsOwnerRole,
		&member.JoinedAt,
		&lastOnline,
	)
	if err != nil {
		return nil, err
	}
	if lastOnline.Valid {
		member.LastOnline = lastOnline.Time
	}
	return member, nil
}

func (r *ClanRepository) GetMembers(clanID int) ([]*models.ClanMember, error) {
	if r.db == nil {
		return []*models.ClanMember{}, nil
	}
	rows, err := r.db.Query(clanMemberSelect+" WHERE m.clan_id = ? ORDER BY r.role_rank DESC, r.id ASC, m.joined_at ASC", clanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var members []*models.ClanMember
	for rows.Next() {
		member, err := scanClanMember(rows)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func (r *ClanRepository) GetMember(clanID, userID int) (*models.ClanMember, error) {
	if r.db == nil || clanID <= 0 || userID <= 0 {
		return nil, nil
	}
	member, err := scanClanMember(r.db.QueryRow(clanMemberSelect+" WHERE m.clan_id = ? AND m.user_id = ? LIMIT 1", clanID, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return member, err
}

func (r *ClanRepository) CountMembers(clanID int) (int, error) {
	if r.db == nil {
		return 0, nil
	}
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM clan_members WHERE clan_id = ?", clanID).Scan(&count)
	return count, err
}

func (r *ClanRepository) AddMember(clanID, userID, roleID int) error {
	if r.db == nil {
		return errors.New("database connection is offline")
	}
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if _, err := tx.Exec(
		"INSERT INTO clan_members (clan_id, user_id, role_id) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE role_id = VALUES(role_id)",
		clanID, userID, roleID,
	); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM clan_join_requests WHERE clan_id = ? AND user_id = ?", clanID, userID); err != nil {
		return err
	}
	if _, err := tx.Exec("UPDATE users SET primary_clan = ? WHERE id = ? AND primary_clan = 0", clanID, userID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func (r *ClanRepository) RemoveMember(clanID, userID int) error {
	if r.db == nil {
		return errors.New("database connection is offline")
	}
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if _, err := tx.Exec("DELETE FROM clan_members WHERE clan_id = ? AND user_id = ?", clanID, userID); err != nil {
		return err
	}
	if _, err := tx.Exec("UPDATE users SET primary_clan = 0 WHERE id = ? AND primary_clan = ?", userID, clanID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func (r *ClanRepository) SetMemberRole(clanID, userID, roleID int) error {
	if r.db == nil {
		return errors.New("database connection is offline")
	}
	_, err := r.db.Exec("UPDATE clan_members SET role_id = ? WHERE clan_id = ? AND user_id = ?", roleID, clanID, userID)
	return err
}

func (r *ClanRepository) GetRoles(clanID int) ([]*models.ClanRole, error) {
	if r.db == nil {
		return []*models.ClanRole{}, nil
	}
	rows, err := r.db.Query(`
		SELECT r.id, r.clan_id, r.name, r.color, r.permissions, r.role_rank, r.is_owner, r.is_default,
		       (SELECT COUNT(*) FROM clan_members m WHERE m.role_id = r.id), r.created_at
		FROM clan_roles r
		WHERE r.clan_id = ?
		ORDER BY r.role_rank DESC, r.id ASC`, clanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var roles []*models.ClanRole
	for rows.Next() {
		role := &models.ClanRole{}
		if err := rows.Scan(&role.ID, &role.ClanID, &role.Name, &role.Color, &role.Permissions, &role.Rank, &role.IsOwner, &role.IsDefault, &role.MemberCount, &role.CreatedAt); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (r *ClanRepository) GetRole(clanID, roleID int) (*models.ClanRole, error) {
	if r.db == nil || clanID <= 0 || roleID <= 0 {
		return nil, nil
	}
	role := &models.ClanRole{}
	err := r.db.QueryRow(`
		SELECT r.id, r.clan_id, r.name, r.color, r.permissions, r.role_rank, r.is_owner, r.is_default,
		       (SELECT COUNT(*) FROM clan_members m WHERE m.role_id = r.id), r.created_at
		FROM clan_roles r
		WHERE r.clan_id = ? AND r.id = ? LIMIT 1`, clanID, roleID).Scan(
		&role.ID, &role.ClanID, &role.Name, &role.Color, &role.Permissions, &role.Rank,
		&role.IsOwner, &role.IsDefault, &role.MemberCount, &role.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return role, nil
}

func (r *ClanRepository) GetDefaultRole(clanID int) (*models.ClanRole, error) {
	if r.db == nil || clanID <= 0 {
		return nil, nil
	}
	role := &models.ClanRole{}
	err := r.db.QueryRow(`
		SELECT r.id, r.clan_id, r.name, r.color, r.permissions, r.role_rank, r.is_owner, r.is_default,
		       (SELECT COUNT(*) FROM clan_members m WHERE m.role_id = r.id), r.created_at
		FROM clan_roles r
		WHERE r.clan_id = ? AND r.is_default = 1 LIMIT 1`, clanID).Scan(
		&role.ID, &role.ClanID, &role.Name, &role.Color, &role.Permissions, &role.Rank,
		&role.IsOwner, &role.IsDefault, &role.MemberCount, &role.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return role, nil
}

func (r *ClanRepository) CreateRole(role *models.ClanRole) (int, error) {
	if r.db == nil {
		return 0, errors.New("database connection is offline")
	}
	res, err := r.db.Exec(
		"INSERT INTO clan_roles (clan_id, name, color, permissions, role_rank) VALUES (?, ?, ?, ?, ?)",
		role.ClanID, role.Name, role.Color, role.Permissions, role.Rank,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func (r *ClanRepository) UpdateRole(role *models.ClanRole) error {
	if r.db == nil {
		return errors.New("database connection is offline")
	}
	_, err := r.db.Exec(
		"UPDATE clan_roles SET name = ?, color = ?, permissions = ?, role_rank = ? WHERE id = ? AND clan_id = ? AND is_owner = 0",
		role.Name, role.Color, role.Permissions, role.Rank, role.ID, role.ClanID,
	)
	return err
}

func (r *ClanRepository) DeleteRole(clanID, roleID int) error {
	if r.db == nil {
		return errors.New("database connection is offline")
	}
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	var defaultRoleID int
	err = tx.QueryRow("SELECT id FROM clan_roles WHERE clan_id = ? AND is_default = 1 AND id != ? LIMIT 1", clanID, roleID).Scan(&defaultRoleID)
	if err != nil {
		return errors.New("this role cannot be deleted")
	}
	if _, err := tx.Exec("UPDATE clan_members SET role_id = ? WHERE clan_id = ? AND role_id = ?", defaultRoleID, clanID, roleID); err != nil {
		return err
	}
	res, err := tx.Exec("DELETE FROM clan_roles WHERE id = ? AND clan_id = ? AND is_owner = 0 AND is_default = 0", roleID, clanID)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return errors.New("this role cannot be deleted")
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

const clanRequestSelect = `SELECT r.id, r.clan_id, r.user_id, u.username, u.displayname, r.message, r.created_at
	FROM clan_join_requests r
	JOIN users u ON u.id = r.user_id`

func scanClanRequest(scanner interface{ Scan(dest ...any) error }) (*models.ClanJoinRequest, error) {
	request := &models.ClanJoinRequest{}
	err := scanner.Scan(
		&request.ID, &request.ClanID, &request.UserID, &request.Username,
		&request.DisplayName, &request.Message, &request.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return request, nil
}

func (r *ClanRepository) CreateJoinRequest(clanID, userID int, message string) error {
	if r.db == nil {
		return errors.New("database connection is offline")
	}
	_, err := r.db.Exec(
		`INSERT INTO clan_join_requests (clan_id, user_id, message) VALUES (?, ?, ?)
		 ON DUPLICATE KEY UPDATE message = VALUES(message), created_at = CURRENT_TIMESTAMP`,
		clanID, userID, message,
	)
	return err
}

func (r *ClanRepository) GetJoinRequest(clanID, userID int) (*models.ClanJoinRequest, error) {
	if r.db == nil || clanID <= 0 || userID <= 0 {
		return nil, nil
	}
	request, err := scanClanRequest(r.db.QueryRow(clanRequestSelect+" WHERE r.clan_id = ? AND r.user_id = ? LIMIT 1", clanID, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return request, err
}

func (r *ClanRepository) GetJoinRequestByID(clanID int, requestID int64) (*models.ClanJoinRequest, error) {
	if r.db == nil || clanID <= 0 || requestID <= 0 {
		return nil, nil
	}
	request, err := scanClanRequest(r.db.QueryRow(clanRequestSelect+" WHERE r.clan_id = ? AND r.id = ? LIMIT 1", clanID, requestID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return request, err
}

func (r *ClanRepository) GetJoinRequests(clanID int) ([]*models.ClanJoinRequest, error) {
	if r.db == nil {
		return []*models.ClanJoinRequest{}, nil
	}
	rows, err := r.db.Query(clanRequestSelect+" WHERE r.clan_id = ? ORDER BY r.created_at ASC, r.id ASC", clanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var requests []*models.ClanJoinRequest
	for rows.Next() {
		request, err := scanClanRequest(rows)
		if err != nil {
			return nil, err
		}
		requests = append(requests, request)
	}
	return requests, rows.Err()
}

func (r *ClanRepository) CountJoinRequests(clanID int) (int, error) {
	if r.db == nil {
		return 0, nil
	}
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM clan_join_requests WHERE clan_id = ?", clanID).Scan(&count)
	return count, err
}

func (r *ClanRepository) DeleteJoinRequest(clanID, userID int) error {
	if r.db == nil {
		return errors.New("database connection is offline")
	}
	_, err := r.db.Exec("DELETE FROM clan_join_requests WHERE clan_id = ? AND user_id = ?", clanID, userID)
	return err
}

func (r *ClanRepository) DeleteJoinRequestByID(clanID int, requestID int64) error {
	if r.db == nil {
		return errors.New("database connection is offline")
	}
	_, err := r.db.Exec("DELETE FROM clan_join_requests WHERE clan_id = ? AND id = ?", clanID, requestID)
	return err
}

func (r *ClanRepository) ApproveJoinRequest(clanID int, requestID int64, roleID int) (int, error) {
	if r.db == nil {
		return 0, errors.New("database connection is offline")
	}
	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	var userID int
	if err := tx.QueryRow("SELECT user_id FROM clan_join_requests WHERE id = ? AND clan_id = ? FOR UPDATE", requestID, clanID).Scan(&userID); err != nil {
		return 0, errors.New("request not found")
	}
	if _, err := tx.Exec(
		"INSERT INTO clan_members (clan_id, user_id, role_id) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE role_id = VALUES(role_id)",
		clanID, userID, roleID,
	); err != nil {
		return 0, err
	}
	if _, err := tx.Exec("DELETE FROM clan_join_requests WHERE id = ? AND clan_id = ?", requestID, clanID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec("UPDATE users SET primary_clan = ? WHERE id = ? AND primary_clan = 0", clanID, userID); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	committed = true
	return userID, nil
}

const clanWallSelect = `SELECT p.id, p.clan_id, p.user_id, u.username, u.displayname,
	COALESCE(r.name, ''), COALESCE(r.color, ''), p.body, p.created_at,
	(SELECT COUNT(*) FROM clan_wall_reactions wr WHERE wr.post_id = p.id),
	EXISTS(SELECT 1 FROM clan_wall_reactions wr WHERE wr.post_id = p.id AND wr.user_id = ?)
	FROM clan_wall_posts p
	JOIN users u ON u.id = p.user_id
	LEFT JOIN clan_members m ON m.clan_id = p.clan_id AND m.user_id = p.user_id
	LEFT JOIN clan_roles r ON r.id = m.role_id`

func scanClanWallPost(scanner interface{ Scan(dest ...any) error }) (*models.ClanWallPost, error) {
	post := &models.ClanWallPost{}
	err := scanner.Scan(
		&post.ID, &post.ClanID, &post.UserID, &post.AuthorName, &post.AuthorDisplay,
		&post.RoleName, &post.RoleColor, &post.Body, &post.CreatedAt, &post.Reactions, &post.HasReacted,
	)
	if err != nil {
		return nil, err
	}
	return post, nil
}

func (r *ClanRepository) GetWallPosts(clanID, viewerID, limit int) ([]*models.ClanWallPost, error) {
	if r.db == nil {
		return []*models.ClanWallPost{}, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 40
	}
	rows, err := r.db.Query(clanWallSelect+" WHERE p.clan_id = ? ORDER BY p.created_at DESC, p.id DESC LIMIT ?", viewerID, clanID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var posts []*models.ClanWallPost
	for rows.Next() {
		post, err := scanClanWallPost(rows)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	return posts, rows.Err()
}

func (r *ClanRepository) CreateWallPost(clanID, userID int, body string) (int64, error) {
	if r.db == nil {
		return 0, errors.New("database connection is offline")
	}
	res, err := r.db.Exec("INSERT INTO clan_wall_posts (clan_id, user_id, body) VALUES (?, ?, ?)", clanID, userID, body)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *ClanRepository) GetWallPostAuthor(clanID int, postID int64) (int, error) {
	if r.db == nil {
		return 0, errors.New("database connection is offline")
	}
	var userID int
	err := r.db.QueryRow("SELECT user_id FROM clan_wall_posts WHERE id = ? AND clan_id = ?", postID, clanID).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, errors.New("post not found")
	}
	return userID, err
}

func (r *ClanRepository) DeleteWallPost(clanID int, postID int64) error {
	if r.db == nil {
		return errors.New("database connection is offline")
	}
	if _, err := r.db.Exec("DELETE FROM clan_wall_reactions WHERE post_id = ?", postID); err != nil {
		return err
	}
	_, err := r.db.Exec("DELETE FROM clan_wall_posts WHERE id = ? AND clan_id = ?", postID, clanID)
	return err
}

func (r *ClanRepository) ToggleWallReaction(postID int64, userID int) (bool, error) {
	if r.db == nil {
		return false, errors.New("database connection is offline")
	}
	res, err := r.db.Exec("DELETE FROM clan_wall_reactions WHERE post_id = ? AND user_id = ?", postID, userID)
	if err != nil {
		return false, err
	}
	if affected, _ := res.RowsAffected(); affected > 0 {
		return false, nil
	}
	if _, err := r.db.Exec("INSERT IGNORE INTO clan_wall_reactions (post_id, user_id) VALUES (?, ?)", postID, userID); err != nil {
		return false, err
	}
	return true, nil
}

func (r *ClanRepository) CountWallReactions(postID int64) (int, error) {
	if r.db == nil {
		return 0, nil
	}
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM clan_wall_reactions WHERE post_id = ?", postID).Scan(&count)
	return count, err
}

const clanAnnouncementSelect = `SELECT a.id, a.clan_id, a.user_id, u.username, u.displayname,
	COALESCE(r.name, ''), COALESCE(r.color, ''), a.title, a.body, a.created_at
	FROM clan_announcements a
	JOIN users u ON u.id = a.user_id
	LEFT JOIN clan_members m ON m.clan_id = a.clan_id AND m.user_id = a.user_id
	LEFT JOIN clan_roles r ON r.id = m.role_id`

func scanClanAnnouncement(scanner interface{ Scan(dest ...any) error }) (*models.ClanAnnouncement, error) {
	announcement := &models.ClanAnnouncement{}
	err := scanner.Scan(
		&announcement.ID, &announcement.ClanID, &announcement.UserID,
		&announcement.AuthorName, &announcement.AuthorDisplay,
		&announcement.AuthorRoleName, &announcement.AuthorRoleColor,
		&announcement.Title, &announcement.Body, &announcement.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return announcement, nil
}

func (r *ClanRepository) GetAnnouncements(clanID, limit int) ([]*models.ClanAnnouncement, error) {
	if r.db == nil {
		return []*models.ClanAnnouncement{}, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	rows, err := r.db.Query(clanAnnouncementSelect+" WHERE a.clan_id = ? ORDER BY a.created_at DESC, a.id DESC LIMIT ?", clanID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var announcements []*models.ClanAnnouncement
	for rows.Next() {
		announcement, err := scanClanAnnouncement(rows)
		if err != nil {
			return nil, err
		}
		announcements = append(announcements, announcement)
	}
	return announcements, rows.Err()
}

func (r *ClanRepository) CreateAnnouncement(clanID, userID int, title, body string) (int64, error) {
	if r.db == nil {
		return 0, errors.New("database connection is offline")
	}
	res, err := r.db.Exec(
		"INSERT INTO clan_announcements (clan_id, user_id, title, body) VALUES (?, ?, ?, ?)",
		clanID, userID, title, body,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *ClanRepository) GetAnnouncementAuthor(clanID int, announcementID int64) (int, error) {
	if r.db == nil {
		return 0, errors.New("database connection is offline")
	}
	var userID int
	err := r.db.QueryRow("SELECT user_id FROM clan_announcements WHERE id = ? AND clan_id = ?", announcementID, clanID).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, errors.New("announcement not found")
	}
	return userID, err
}

func (r *ClanRepository) DeleteAnnouncement(clanID int, announcementID int64) error {
	if r.db == nil {
		return errors.New("database connection is offline")
	}
	_, err := r.db.Exec("DELETE FROM clan_announcements WHERE id = ? AND clan_id = ?", announcementID, clanID)
	return err
}

func (r *ClanRepository) GetClanItems(clanID int) ([]*models.ShopItem, error) {
	if r.db == nil {
		return []*models.ShopItem{}, nil
	}
	rows, err := r.db.Query(`
		SELECT s.id, s.cid, s.creator_type, COALESCE(NULLIF(c.name, ''), 'Clan') AS creator_name,
		       s.name, COALESCE(s.description, '') AS description, s.type, s.bucks, s.bits, s.special,
		       s.stock, s.deleted, s.ugc, s.approval_state, s.onsale, s.originaltitle, s.originaldescription,
		       s.createdat, s.updatedat, s.original_stock, s.rap, s.offsale, s.transform
		FROM shopitem s
		LEFT JOIN clans c ON s.cid = c.id AND s.creator_type = 'clan'
		WHERE s.cid = ? AND s.creator_type = 'clan' AND s.deleted = 0 AND s.approval_state = 'approved'
		ORDER BY s.createdat DESC, s.id DESC
		LIMIT 24`, clanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	shopRepo := &ShopRepository{}
	var items []*models.ShopItem
	for rows.Next() {
		item, err := shopRepo.scanShopItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
