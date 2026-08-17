package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

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
	)
	if err != nil {
		return nil, err
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

func (r *ShopRepository) buildWhereClause(filter *models.ShopFilter) (string, []any) {
	whereClauses := []string{"s.deleted = 0"}
	var args []any

	if filter.Search != "" {
		pattern := "%" + filter.Search + "%"
		whereClauses = append(whereClauses, "(s.name LIKE ? OR s.description LIKE ?)")
		args = append(args, pattern, pattern)
	}

	cat, _ := models.NormalizeShopCategory(filter.Category)
	switch cat {
	case "hat":
		whereClauses = append(whereClauses, "LOWER(s.type) IN ('hat', 'hats')")
	case "faces":
		whereClauses = append(whereClauses, "LOWER(s.type) IN ('face', 'faces')")
	case "shirts":
		whereClauses = append(whereClauses, "LOWER(s.type) IN ('shirt', 'shirts')")
	case "tshirts":
		whereClauses = append(whereClauses, "LOWER(s.type) IN ('tshirt', 'tshirts')")
	case "pants":
		whereClauses = append(whereClauses, "LOWER(s.type) IN ('pants', 'pant')")
	case "gear":
		whereClauses = append(whereClauses, "LOWER(s.type) IN ('gear', 'tool', 'tools')")
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
		       s.createdat, s.updatedat, s.original_stock, s.rap, s.offsale
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
	switch cat {
	case "hat":
		whereClauses = append(whereClauses, "LOWER(s.type) IN ('hat', 'hats')")
	case "faces":
		whereClauses = append(whereClauses, "LOWER(s.type) IN ('face', 'faces')")
	case "shirts":
		whereClauses = append(whereClauses, "LOWER(s.type) IN ('shirt', 'shirts')")
	case "tshirts":
		whereClauses = append(whereClauses, "LOWER(s.type) IN ('tshirt', 'tshirts')")
	case "pants":
		whereClauses = append(whereClauses, "LOWER(s.type) IN ('pants', 'pant')")
	case "gear":
		whereClauses = append(whereClauses, "LOWER(s.type) IN ('gear', 'tool', 'tools')")
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
		       0 AS original_stock, 0 AS rap, 'false' AS offsale
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

func (r *ShopRepository) GetItemByID(id int) (*models.ShopItem, error) {
	if r.db == nil {
		return nil, errors.New("database connection is offline")
	}

	query := `
		SELECT s.id, s.cid, s.creator_type, COALESCE(u.username, 'VERTEXIA') AS creator_name,
		       s.name, COALESCE(s.description, '') AS description, s.type, s.bucks, s.bits, s.special,
		       s.stock, s.deleted, s.ugc, s.approval_state, s.onsale, s.originaltitle, s.originaldescription,
		       s.createdat, s.updatedat, s.original_stock, s.rap, s.offsale
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
			       0 AS original_stock, 0 AS rap, 'false' AS offsale
			FROM shop s
			LEFT JOIN users u ON s.creator_id = u.id
			WHERE s.id = ?`
		fallbackRow := r.db.QueryRow(fallbackQuery, id)
		return r.scanShopItem(fallbackRow)
	}

	return item, nil
}