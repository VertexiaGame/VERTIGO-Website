package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"vertexia-frontend/backend/models"
)

type ShopRepository struct {
	db *sql.DB
}

func NewShopRepository(db *sql.DB) *ShopRepository {
	return &ShopRepository{db: db}
}

func (r *ShopRepository) scanShopItem(scanner interface{ Scan(dest ...any) error }) (*models.ShopItem, error) {
	var item models.ShopItem
	var transform sql.NullString
	err := scanner.Scan(
		&item.ID,
		&item.CID,
		&item.CreatorType,
		&item.CreatorName,
		&item.Name,
		&item.Description,
		&item.Type,
		&item.Bucks,
		&item.Bits,
		&item.Special,
		&item.Stock,
		&item.Deleted,
		&item.UGC,
		&item.ApprovalState,
		&item.OnSale,
		&item.OriginalTitle,
		&item.OriginalDescription,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.OriginalStock,
		&item.RAP,
		&item.OffSale,
		&transform,
	)
	if err != nil {
		return nil, err
	}
	if transform.Valid {
		item.Transform = transform.String
	}

	normType, typeName := models.NormalizeShopCategory(item.Type)
	if normType != "all" {
		item.Type = normType
		item.TypeName = typeName
	} else {
		item.TypeName = "Item"
	}

	if item.CreatorName == "" {
		item.CreatorName = "VERTEXIA"
	}

	return &item, nil
}

func shopTypeCondition(cat string) string {
	switch cat {
	case "hat":
		return "LOWER(s.type) IN ('hat', 'hats')"
	case "faces":
		return "LOWER(s.type) IN ('face', 'faces')"
	case "shirts":
		return "LOWER(s.type) IN ('shirt', 'shirts')"
	case "tshirts":
		return "LOWER(s.type) IN ('tshirt', 'tshirts')"
	case "pants":
		return "LOWER(s.type) IN ('pants', 'pant')"
	case "gear":
		return "LOWER(s.type) IN ('gear', 'tool', 'tools')"
	default:
		return ""
	}
}

func (r *ShopRepository) buildWhereClause(filter *models.ShopFilter) (string, []any) {
	whereClauses := []string{"s.deleted = 0", "s.onsale != 'false'", "s.offsale != 'true'"}
	var args []any

	if filter.Search != "" {
		pattern := "%" + filter.Search + "%"
		whereClauses = append(whereClauses, "(s.name LIKE ? OR s.description LIKE ?)")
		args = append(args, pattern, pattern)
	}

	cat, _ := models.NormalizeShopCategory(filter.Category)
	if cond := shopTypeCondition(cat); cond != "" {
		whereClauses = append(whereClauses, cond)
	}

	rawCurr := strings.ToLower(filter.Currency)
	if rawCurr != "" && rawCurr != "all" {
		tokens := strings.Split(rawCurr, ",")
		var currConditions []string
		for _, tok := range tokens {
			tok = strings.TrimSpace(tok)
			switch tok {
			case "free":
				currConditions = append(currConditions, "(s.bucks = 0 AND s.bits = 0)")
			case "bucks", "vertices":
				currConditions = append(currConditions, "s.bucks > 0")
			case "bits", "tickets":
				currConditions = append(currConditions, "s.bits > 0")
			}
		}
		if len(currConditions) > 0 {
			whereClauses = append(whereClauses, "("+strings.Join(currConditions, " OR ")+")")
		}
	}

	if filter.MinPrice > 0 {
		whereClauses = append(whereClauses, "(s.bucks >= ? OR s.bits >= ?)")
		args = append(args, filter.MinPrice, filter.MinPrice)
	}

	if filter.MaxPrice > 0 {
		whereClauses = append(whereClauses, "((s.bucks <= ? AND s.bucks > 0) OR (s.bits <= ? AND s.bits > 0) OR (s.bucks = 0 AND s.bits = 0))")
		args = append(args, filter.MaxPrice, filter.MaxPrice)
	}

	if filter.Creator != "" {
		whereClauses = append(whereClauses, "(u.username LIKE ? OR s.cid = ?)")
		creatorID, _ := strconv.Atoi(filter.Creator)
		args = append(args, "%"+filter.Creator+"%", creatorID)
	}

	return "WHERE " + strings.Join(whereClauses, " AND "), args
}

func (r *ShopRepository) buildOrderBy(sort string) string {
	switch strings.ToLower(sort) {
	case "oldest":
		return "ORDER BY s.createdat ASC, s.id ASC"
	case "price_asc":
		return "ORDER BY (s.bucks + s.bits) ASC, s.id ASC"
	case "price_desc":
		return "ORDER BY (s.bucks + s.bits) DESC, s.id DESC"
	case "popular", "bestselling":
		return "ORDER BY s.rap DESC, s.id DESC"
	default:
		return "ORDER BY s.createdat DESC, s.id DESC"
	}
}

func (r *ShopRepository) GetItems(filter *models.ShopFilter) ([]*models.ShopItem, int, error) {
	if r.db == nil {
		return []*models.ShopItem{}, 0, nil
	}

	whereSQL, args := r.buildWhereClause(filter)
	orderBySQL := r.buildOrderBy(filter.Sort)

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM shopitem s LEFT JOIN users u ON s.cid = u.id AND s.creator_type = 'user' %s`, whereSQL)

	var total int
	err := r.db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		log.Printf("ShopRepository query error: %v", err)
		return r.fallbackGetItemsFromShopTable(filter)
	}

	offset := (filter.Page - 1) * filter.Limit
	if offset < 0 {
		offset = 0
	}

	selectQuery := fmt.Sprintf(`
		SELECT s.id, s.cid, s.creator_type, COALESCE(u.username, 'VERTEXIA') AS creator_name,
		       s.name, COALESCE(s.description, '') AS description, s.type, s.bucks, s.bits, s.special,
		       s.stock, s.deleted, s.ugc, s.approval_state, s.onsale, s.originaltitle, s.originaldescription,
		       s.createdat, s.updatedat, s.original_stock, s.rap, s.offsale, s.transform
		FROM shopitem s
		LEFT JOIN users u ON s.cid = u.id AND s.creator_type = 'user'
		%s %s LIMIT ? OFFSET ?`, whereSQL, orderBySQL)

	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.Limit, offset)

	rows, err := r.db.Query(selectQuery, queryArgs...)
	if err != nil {
		log.Printf("ShopRepository select query error: %v", err)
		return []*models.ShopItem{}, 0, err
	}
	defer rows.Close()

	var items []*models.ShopItem
	for rows.Next() {
		item, err := r.scanShopItem(rows)
		if err != nil {
			log.Printf("ShopRepository scan error: %v", err)
			return []*models.ShopItem{}, 0, err
		}
		items = append(items, item)
	}

	return items, total, nil
}

func (r *ShopRepository) fallbackGetItemsFromShopTable(filter *models.ShopFilter) ([]*models.ShopItem, int, error) {
	whereClauses := []string{"1=1"}
	var args []any

	if filter.Search != "" {
		pattern := "%" + filter.Search + "%"
		whereClauses = append(whereClauses, "(s.name LIKE ? OR s.description LIKE ?)")
		args = append(args, pattern, pattern)
	}

	cat, _ := models.NormalizeShopCategory(filter.Category)
	if cond := shopTypeCondition(cat); cond != "" {
		whereClauses = append(whereClauses, cond)
	}

	whereSQL := "WHERE " + strings.Join(whereClauses, " AND ")
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM shop s LEFT JOIN users u ON s.creator_id = u.id %s`, whereSQL)

	var total int
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return []*models.ShopItem{}, 0, nil
	}

	offset := (filter.Page - 1) * filter.Limit
	if offset < 0 {
		offset = 0
	}

	selectQuery := fmt.Sprintf(`
		SELECT s.id, COALESCE(s.creator_id, 0) AS cid, 'user' AS creator_type, COALESCE(u.username, 'VERTEXIA') AS creator_name,
		       s.name, COALESCE(s.description, '') AS description, s.type, 0 AS bucks, 0 AS bits,
		       'false' AS special, 0 AS stock, 0 AS deleted, 'false' AS ugc, 'approved' AS approval_state,
		       'true' AS onsale, NULL AS originaltitle, NULL AS originaldescription, NOW() AS createdat, NOW() AS updatedat,
		       0 AS original_stock, 0 AS rap, 'false' AS offsale, NULL AS transform
		FROM shop s
		LEFT JOIN users u ON s.creator_id = u.id
		%s ORDER BY s.id DESC LIMIT ? OFFSET ?`, whereSQL)

	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.Limit, offset)

	rows, err := r.db.Query(selectQuery, queryArgs...)
	if err != nil {
		return []*models.ShopItem{}, 0, nil
	}
	defer rows.Close()

	var items []*models.ShopItem
	for rows.Next() {
		item, err := r.scanShopItem(rows)
		if err == nil {
			items = append(items, item)
		}
	}

	return items, total, nil
}

func (r *ShopRepository) Create(cid int, name, description, itemType string, bucks, bits int, transform string) (int, error) {
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

	var maxID int
	if err := tx.QueryRow(`SELECT COALESCE(MAX(id), 0) FROM shopitem FOR UPDATE`).Scan(&maxID); err != nil {
		return 0, err
	}

	var transformArg any
	if transform != "" {
		transformArg = transform
	}
	now := time.Now()
	_, err = tx.Exec(
		`INSERT INTO shopitem (id, cid, creator_type, name, description, type, bucks, bits, special, stock, deleted, ugc, approval_state, onsale, createdat, updatedat, original_stock, rap, offsale, transform) VALUES (?, ?, 'user', ?, ?, ?, ?, ?, 'false', 0, 0, 'true', 'pending', 'false', ?, ?, 0, 0, 'false', ?)`,
		maxID+1, cid, name, description, itemType, bucks, bits, now, now, transformArg,
	)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	committed = true
	return maxID + 1, nil
}

func (r *ShopRepository) GetByCreator(cid int) ([]*models.ShopItem, error) {
	if r.db == nil {
		return nil, nil
	}

	query := `
		SELECT s.id, s.cid, s.creator_type, COALESCE(u.username, 'VERTEXIA') AS creator_name,
		       s.name, COALESCE(s.description, '') AS description, s.type, s.bucks, s.bits, s.special,
		       s.stock, s.deleted, s.ugc, s.approval_state, s.onsale, s.originaltitle, s.originaldescription,
		       s.createdat, s.updatedat, s.original_stock, s.rap, s.offsale, s.transform
		FROM shopitem s
		LEFT JOIN users u ON s.cid = u.id AND s.creator_type = 'user'
		WHERE s.cid = ? AND s.creator_type = 'user' AND s.deleted = 0
		ORDER BY s.createdat DESC, s.id DESC`
	rows, err := r.db.Query(query, cid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*models.ShopItem
	for rows.Next() {
		item, err := r.scanShopItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *ShopRepository) GetApprovalQueue(category string, limit, offset int) ([]*models.ShopItem, int, error) {
	if r.db == nil {
		return nil, 0, errors.New("database connection is offline")
	}

	where := `WHERE s.deleted = 0 AND s.approval_state = 'pending'`
	var args []any
	if cat, _ := models.NormalizeShopCategory(category); cat != "all" {
		if cond := shopTypeCondition(cat); cond != "" {
			where += ` AND ` + cond
		}
	}

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM shopitem s `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT s.id, s.cid, s.creator_type, COALESCE(u.username, 'VERTEXIA') AS creator_name,
		       s.name, COALESCE(s.description, '') AS description, s.type, s.bucks, s.bits, s.special,
		       s.stock, s.deleted, s.ugc, s.approval_state, s.onsale, s.originaltitle, s.originaldescription,
		       s.createdat, s.updatedat, s.original_stock, s.rap, s.offsale, s.transform
		FROM shopitem s
		LEFT JOIN users u ON s.cid = u.id AND s.creator_type = 'user'
		` + where + ` ORDER BY s.createdat ASC, s.id ASC LIMIT ? OFFSET ?`
	queryArgs := append(append([]any{}, args...), limit, offset)
	rows, err := r.db.Query(query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []*models.ShopItem
	for rows.Next() {
		item, err := r.scanShopItem(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, nil
}

func (r *ShopRepository) UpdateApproval(id int, state string) error {
	if r.db == nil {
		return errors.New("database connection is offline")
	}

	onsale := "false"
	if state == "approved" {
		onsale = "true"
	}
	_, err := r.db.Exec(
		`UPDATE shopitem SET approval_state = ?, onsale = ?, updatedat = ? WHERE id = ?`,
		state, onsale, time.Now(), id,
	)
	return err
}

func (r *ShopRepository) DeletePending(id, cid int) error {
	if r.db == nil {
		return errors.New("database connection is offline")
	}

	_, err := r.db.Exec(
		`DELETE FROM shopitem WHERE id = ? AND cid = ? AND creator_type = 'user' AND approval_state = 'pending'`,
		id, cid,
	)
	return err
}

func (r *ShopRepository) GetItemByID(id int) (*models.ShopItem, error) {
	if r.db == nil {
		return nil, errors.New("database connection is offline")
	}

	query := `
		SELECT s.id, s.cid, s.creator_type, COALESCE(u.username, 'VERTEXIA') AS creator_name,
		       s.name, COALESCE(s.description, '') AS description, s.type, s.bucks, s.bits, s.special,
		       s.stock, s.deleted, s.ugc, s.approval_state, s.onsale, s.originaltitle, s.originaldescription,
		       s.createdat, s.updatedat, s.original_stock, s.rap, s.offsale, s.transform
		FROM shopitem s
		LEFT JOIN users u ON s.cid = u.id AND s.creator_type = 'user'
		WHERE s.id = ? AND s.deleted = 0`

	row := r.db.QueryRow(query, id)
	item, err := r.scanShopItem(row)
	if err != nil {
		fallbackQuery := `
			SELECT s.id, COALESCE(s.creator_id, 0) AS cid, 'user' AS creator_type, COALESCE(u.username, 'VERTEXIA') AS creator_name,
			       s.name, COALESCE(s.description, '') AS description, s.type, 0 AS bucks, 0 AS bits,
			       'false' AS special, 0 AS stock, 0 AS deleted, 'false' AS ugc, 'approved' AS approval_state,
			       'true' AS onsale, NULL AS originaltitle, NULL AS originaldescription, NOW() AS createdat, NOW() AS updatedat,
			       0 AS original_stock, 0 AS rap, 'false' AS offsale, NULL AS transform
			FROM shop s
			LEFT JOIN users u ON s.creator_id = u.id
			WHERE s.id = ?`
		fallbackRow := r.db.QueryRow(fallbackQuery, id)
		return r.scanShopItem(fallbackRow)
	}

	return item, nil
}

func (r *ShopRepository) GetRelatedItems(item *models.ShopItem, limit int) ([]*models.ShopItem, error) {
	if r.db == nil || item == nil {
		return []*models.ShopItem{}, nil
	}
	if limit <= 0 {
		limit = 6
	}
	if limit > 12 {
		limit = 12
	}
	normType, _ := models.NormalizeShopCategory(item.Type)
	filter := &models.ShopFilter{
		Category: normType,
		Page:     1,
		Limit:    limit + 1,
		Sort:     "newest",
	}
	whereSQL, args := r.buildWhereClause(filter)
	whereSQL = whereSQL + " AND s.id != ?"
	args = append(args, item.ID)
	orderBySQL := r.buildOrderBy(filter.Sort)
	selectQuery := fmt.Sprintf(`
		SELECT s.id, s.cid, s.creator_type, COALESCE(u.username, 'VERTEXIA') AS creator_name,
		       s.name, COALESCE(s.description, '') AS description, s.type, s.bucks, s.bits, s.special,
		       s.stock, s.deleted, s.ugc, s.approval_state, s.onsale, s.originaltitle, s.originaldescription,
		       s.createdat, s.updatedat, s.original_stock, s.rap, s.offsale, s.transform
		FROM shopitem s
		LEFT JOIN users u ON s.cid = u.id AND s.creator_type = 'user'
		%s %s LIMIT ?`, whereSQL, orderBySQL)
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.Limit)
	rows, err := r.db.Query(selectQuery, queryArgs...)
	if err != nil {
		return []*models.ShopItem{}, nil
	}
	defer rows.Close()
	var items []*models.ShopItem
	for rows.Next() {
		it, err := r.scanShopItem(rows)
		if err == nil {
			items = append(items, it)
			if len(items) >= limit {
				break
			}
		}
	}
	return items, nil
}

func (r *ShopRepository) HasItem(userID, itemID int) bool {
	if r.db == nil || userID <= 0 || itemID <= 0 {
		return false
	}
	var cnt int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM crate WHERE uid = ? AND iid = ?`, userID, itemID).Scan(&cnt)
	if err != nil {
		err2 := r.db.QueryRow(`SELECT COUNT(*) FROM inventory WHERE user_id = ? AND item_id = ?`, userID, itemID).Scan(&cnt)
		if err2 != nil {
			return false
		}
		return cnt > 0
	}
	return cnt > 0
}

func (r *ShopRepository) IsEquipped(userID, itemID int, itemType string) bool {
	if r.db == nil || userID <= 0 || itemID <= 0 {
		return false
	}
	avatarRepo := &AvatarRepository{db: r.db}
	av, err := avatarRepo.GetAvatar(userID)
	if err != nil || av == nil {
		return false
	}
	norm, _ := models.NormalizeShopCategory(itemType)
	switch norm {
	case "hat":
		return av.Hat1 == itemID || av.Hat2 == itemID || av.Hat3 == itemID || av.Hat4 == itemID || av.Hat5 == itemID
	case "faces":
		return av.Face == itemID
	case "shirts":
		return av.Shirt == itemID
	case "tshirts":
		return av.TShirt == itemID
	case "pants":
		return av.Pants == itemID
	case "gear":
		return av.Tool == itemID
	default:
		return false
	}
}

func (r *ShopRepository) GetSalesCount(itemID int) int {
	if r.db == nil || itemID <= 0 {
		return 0
	}
	var cnt int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM crate WHERE iid = ?`, itemID).Scan(&cnt)
	if err != nil {
		_ = r.db.QueryRow(`SELECT COUNT(*) FROM inventory WHERE item_id = ?`, itemID).Scan(&cnt)
	}
	return cnt
}

func (r *ShopRepository) GetCrateEntry(userID, itemID int) (bool, error) {
	if r.db == nil {
		return false, errors.New("database connection is offline")
	}
	var cnt int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM crate WHERE uid = ? AND iid = ?`, userID, itemID).Scan(&cnt)
	if err != nil {
		return false, err
	}
	return cnt > 0, nil
}

func (r *ShopRepository) GrantItem(userID, itemID int, bv string, aid int, price int) error {
	if r.db == nil {
		return errors.New("database connection is offline")
	}
	if bv == "" {
		bv = "awarded"
	}
	if _, ok := map[string]bool{"trade": true, "free": true, "vertices": true, "tickets": true, "awarded": true}[bv]; !ok {
		bv = "awarded"
	}
	_, err := r.db.Exec(`INSERT INTO crate (iid, uid, bv, aid, price, date) VALUES (?, ?, ?, ?, ?, NOW())`, itemID, userID, bv, aid, price)
	return err
}

func (r *ShopRepository) PurchaseItem(userID, itemID int, currency string) (*models.Crate, error) {
	if r.db == nil {
		return nil, errors.New("database connection is offline")
	}
	if userID <= 0 || itemID <= 0 {
		return nil, errors.New("invalid id")
	}
	currency = strings.ToLower(strings.TrimSpace(currency))
	if currency == "bucks" {
		currency = "vertices"
	}
	if currency == "bits" {
		currency = "tickets"
	}

	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	var bucksPrice, bitsPrice int
	var deleted int
	var onsale, offsale string
	var approval string
	var stock, originalStock int
	var creatorID int
	var creatorType string
	err = tx.QueryRow(`SELECT bucks, bits, deleted, onsale, offsale, approval_state, stock, original_stock, cid, creator_type FROM shopitem WHERE id = ? FOR UPDATE`, itemID).Scan(&bucksPrice, &bitsPrice, &deleted, &onsale, &offsale, &approval, &stock, &originalStock, &creatorID, &creatorType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("item not found")
		}
		return nil, err
	}
	if deleted != 0 {
		return nil, errors.New("item not found")
	}
	if onsale == "false" || offsale == "true" {
		return nil, errors.New("item is offsale")
	}
	if approval != "approved" {
		return nil, errors.New("item is not approved for sale")
	}
	if originalStock > 0 && stock <= 0 {
		return nil, errors.New("item is out of stock")
	}

	var cnt int
	err = tx.QueryRow(`SELECT COUNT(*) FROM crate WHERE uid = ? AND iid = ? FOR UPDATE`, userID, itemID).Scan(&cnt)
	if err != nil {
		return nil, err
	}
	if cnt > 0 {
		return nil, errors.New("already owned")
	}

	isFree := bucksPrice == 0 && bitsPrice == 0
	hasBucksPrice := bucksPrice > 0
	hasBitsPrice := bitsPrice > 0
	var bv string
	var price int
	var deductBucks, deductBits int

	if isFree {
		bv = models.BoughtViaFree
		price = 0
		if currency != "" && currency != "free" && currency != models.BoughtViaFree {
			return nil, errors.New("this item is free")
		}
	} else if hasBucksPrice && hasBitsPrice {
		if currency == "" {
			return nil, errors.New("choose currency: vertices or tickets")
		}
		switch currency {
		case models.BoughtViaVertices:
			bv = models.BoughtViaVertices
			price = bucksPrice
			deductBucks = bucksPrice
		case models.BoughtViaTickets:
			bv = models.BoughtViaTickets
			price = bitsPrice
			deductBits = bitsPrice
		default:
			return nil, errors.New("invalid currency: choose vertices or tickets")
		}
	} else if hasBucksPrice {
		if currency != "" && currency != models.BoughtViaVertices {
			return nil, errors.New("this item can only be purchased with vertices")
		}
		bv = models.BoughtViaVertices
		price = bucksPrice
		deductBucks = bucksPrice
	} else {
		if currency != "" && currency != models.BoughtViaTickets {
			return nil, errors.New("this item can only be purchased with tickets")
		}
		bv = models.BoughtViaTickets
		price = bitsPrice
		deductBits = bitsPrice
	}

	var curBucks, curBits int
	err = tx.QueryRow(`SELECT bucks, bits FROM users WHERE id = ? FOR UPDATE`, userID).Scan(&curBucks, &curBits)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	if deductBucks > 0 && curBucks < deductBucks {
		return nil, errors.New("insufficient vertices")
	}
	if deductBits > 0 && curBits < deductBits {
		return nil, errors.New("insufficient tickets")
	}

	if deductBucks > 0 {
		_, err = tx.Exec(`UPDATE users SET bucks = bucks - ? WHERE id = ?`, deductBucks, userID)
		if err != nil {
			return nil, err
		}
	}
	if deductBits > 0 {
		_, err = tx.Exec(`UPDATE users SET bits = bits - ? WHERE id = ?`, deductBits, userID)
		if err != nil {
			return nil, err
		}
	}

	if creatorType == "user" && creatorID > 0 {
		if deductBucks > 0 {
			_, err = tx.Exec(`UPDATE users SET bucks = bucks + ? WHERE id = ?`, deductBucks, creatorID)
			if err != nil {
				return nil, err
			}
		}
		if deductBits > 0 {
			_, err = tx.Exec(`UPDATE users SET bits = bits + ? WHERE id = ?`, deductBits, creatorID)
			if err != nil {
				return nil, err
			}
		}
	}

	if originalStock > 0 {
		res, e := tx.Exec(`UPDATE shopitem SET stock = stock - 1 WHERE id = ? AND stock > 0 LIMIT 1`, itemID)
		if e != nil {
			res2, e2 := tx.Exec(`UPDATE shopitem SET stock = stock - 1 WHERE id = ? AND stock > 0`, itemID)
			if e2 != nil {
				return nil, e
			}
			res = res2
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil, errors.New("item is out of stock")
		}
	}

	res, err := tx.Exec(`INSERT INTO crate (iid, uid, bv, aid, price, date) VALUES (?, ?, ?, 0, ?, NOW())`, itemID, userID, bv, price)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return nil, errors.New("already owned")
		}
		return nil, err
	}
	lastID, _ := res.LastInsertId()

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	err = nil

	crateID := int(lastID)
	cDate := time.Now()
	var dbAID int
	_ = r.db.QueryRow(`SELECT id, iid, uid, bv, aid, price, date FROM crate WHERE id = ?`, crateID).Scan(&crateID, &itemID, &userID, &bv, &dbAID, &price, &cDate)
	return &models.Crate{
		ID:    crateID,
		IID:   itemID,
		UID:   userID,
		BV:    bv,
		AID:   dbAID,
		Price: price,
		Date:  cDate,
	}, nil
}