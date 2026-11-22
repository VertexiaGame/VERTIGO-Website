package repository

import (
	"database/sql"
	"errors"

	"vertexia-frontend/backend/models"
)

func (r *ShopRepository) backfillMissingSpecialCopies() error {
	rows, err := r.db.Query(`SELECT DISTINCT s.id FROM shopitem s
		JOIN crate c ON c.iid = s.id LEFT JOIN shop_special_copies sc ON sc.crate_id = c.id
		WHERE s.special = 'true' AND sc.crate_id IS NULL ORDER BY s.id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()
	for _, id := range ids {
		if err := r.backfillSpecialCopies(id); err != nil {
			return err
		}
	}
	return nil
}

func (r *ShopRepository) backfillSpecialCopies(itemID int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var stock, originalStock int
	if err := tx.QueryRow(`SELECT stock, original_stock FROM shopitem WHERE id = ? FOR UPDATE`, itemID).
		Scan(&stock, &originalStock); err != nil {
		return err
	}
	var lastSerial int
	if err := tx.QueryRow(`SELECT COALESCE(MAX(serial), 0) FROM shop_special_copies WHERE item_id = ?`, itemID).Scan(&lastSerial); err != nil {
		return err
	}
	res, err := tx.Exec(`INSERT INTO shop_special_copies (crate_id, item_id, serial)
		SELECT c.id, c.iid, ? + ROW_NUMBER() OVER (ORDER BY c.id)
		FROM crate c LEFT JOIN shop_special_copies sc ON sc.crate_id = c.id
		WHERE c.iid = ? AND sc.crate_id IS NULL`, lastSerial, itemID)
	if err != nil {
		return err
	}
	inserted, err := res.RowsAffected()
	if err != nil {
		return err
	}
	lastSerial += int(inserted)
	originalStock = max(originalStock, lastSerial)
	stock = max(0, min(stock, originalStock-lastSerial))
	if _, err := tx.Exec(`UPDATE shopitem SET stock = ?, original_stock = ?, updatedat = updatedat WHERE id = ?`, stock, originalStock, itemID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *ShopRepository) specialAvailable() error {
	if r.db == nil || r.specialSetupErr != nil {
		return models.ErrSpecialUnavailable
	}
	return nil
}

func (r *ShopRepository) GetSpecialCopy(itemID, userID int) (*models.SpecialCopy, error) {
	if err := r.specialAvailable(); err != nil {
		return nil, err
	}
	var copy models.SpecialCopy
	err := r.db.QueryRow(`SELECT sc.serial, sc.listing_price, sc.listing_version
		FROM crate c JOIN shop_special_copies sc ON sc.crate_id = c.id
		WHERE c.uid = ? AND c.iid = ?`, userID, itemID).Scan(&copy.Serial, &copy.Price, &copy.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &copy, err
}

func (r *ShopRepository) GetSpecialOwners(itemID, limit, offset int) ([]models.SpecialOwner, error) {
	if err := r.specialAvailable(); err != nil {
		return nil, err
	}
	rows, err := r.db.Query(`SELECT sc.serial, u.id, u.username
		FROM shop_special_copies sc JOIN crate c ON c.id = sc.crate_id JOIN users u ON u.id = c.uid
		WHERE sc.item_id = ? AND u.hide_inventory = 0
		ORDER BY sc.serial LIMIT ? OFFSET ?`, itemID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var owners []models.SpecialOwner
	for rows.Next() {
		var owner models.SpecialOwner
		if err := rows.Scan(&owner.Serial, &owner.UserID, &owner.Username); err != nil {
			return nil, err
		}
		owners = append(owners, owner)
	}
	return owners, rows.Err()
}

func (r *ShopRepository) GetSpecialListings(itemID, limit, offset int) ([]models.SpecialListing, error) {
	if err := r.specialAvailable(); err != nil {
		return nil, err
	}
	rows, err := r.db.Query(`SELECT sc.serial, sc.listing_price, sc.listing_version,
			IF(u.hide_inventory = 0, u.id, 0), IF(u.hide_inventory = 0, u.username, '')
		FROM shop_special_copies sc JOIN crate c ON c.id = sc.crate_id JOIN users u ON u.id = c.uid
		WHERE sc.item_id = ? AND sc.listing_price > 0
		ORDER BY sc.listing_price, sc.serial LIMIT ? OFFSET ?`, itemID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var listings []models.SpecialListing
	for rows.Next() {
		var listing models.SpecialListing
		if err := rows.Scan(&listing.Serial, &listing.Price, &listing.Version, &listing.SellerID, &listing.SellerName); err != nil {
			return nil, err
		}
		listings = append(listings, listing)
	}
	return listings, rows.Err()
}

func lockResaleItem(tx *sql.Tx, itemID int) (*models.ShopItem, error) {
	item := &models.ShopItem{ID: itemID}
	err := tx.QueryRow(`SELECT special, stock, original_stock, deleted, approval_state, offsale, type
		FROM shopitem WHERE id = ? FOR UPDATE`, itemID).
		Scan(&item.Special, &item.Stock, &item.OriginalStock, &item.Deleted, &item.ApprovalState, &item.OffSale, &item.Type)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, models.ErrSpecialNotResellable
	}
	if err != nil {
		return nil, err
	}
	if !item.CanResell() {
		return nil, models.ErrSpecialNotResellable
	}
	return item, nil
}

func lockSpecialCopy(tx *sql.Tx, itemID, serial int) (crateID, ownerID, price int, version int64, err error) {
	err = tx.QueryRow(`SELECT sc.crate_id, c.uid, sc.listing_price, sc.listing_version
		FROM shop_special_copies sc JOIN crate c ON c.id = sc.crate_id
		WHERE sc.item_id = ? AND sc.serial = ? FOR UPDATE`, itemID, serial).Scan(&crateID, &ownerID, &price, &version)
	return
}

func (r *ShopRepository) SetSpecialListing(userID, itemID, serial, price int, version int64) error {
	if err := r.specialAvailable(); err != nil {
		return err
	}
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := lockResaleItem(tx, itemID); err != nil {
		return err
	}
	crateID, ownerID, _, currentVersion, err := lockSpecialCopy(tx, itemID, serial)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && ownerID != userID) {
		return models.ErrSpecialNotOwner
	}
	if err != nil {
		return err
	}
	if version != currentVersion {
		return models.ErrSpecialListingChanged
	}
	if _, err := tx.Exec(`UPDATE shop_special_copies SET listing_price = ?, listing_version = listing_version + 1
		WHERE crate_id = ?`, price, crateID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *ShopRepository) BuySpecialListing(buyerID, itemID, serial int, version int64) (int, error) {
	if err := r.specialAvailable(); err != nil {
		return 0, err
	}
	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	item, err := lockResaleItem(tx, itemID)
	if err != nil {
		return 0, err
	}
	crateID, sellerID, price, currentVersion, err := lockSpecialCopy(tx, itemID, serial)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (price <= 0 || version != currentVersion)) {
		return 0, models.ErrSpecialListingChanged
	}
	if err != nil {
		return 0, err
	}
	if sellerID == buyerID {
		return 0, models.ErrSpecialOwnListing
	}

	var lowBalance, highBalance int
	lowID, highID := min(buyerID, sellerID), max(buyerID, sellerID)
	if err := tx.QueryRow(`SELECT bucks FROM users WHERE id = ? FOR UPDATE`, lowID).Scan(&lowBalance); err != nil {
		return 0, err
	}
	if err := tx.QueryRow(`SELECT bucks FROM users WHERE id = ? FOR UPDATE`, highID).Scan(&highBalance); err != nil {
		return 0, err
	}
	buyerBalance := lowBalance
	if buyerID == highID {
		buyerBalance = highBalance
	}
	var owned bool
	if err := tx.QueryRow(`SELECT EXISTS (SELECT 1 FROM crate WHERE uid = ? AND iid = ?)`, buyerID, itemID).Scan(&owned); err != nil {
		return 0, err
	}
	if owned {
		return 0, models.ErrSpecialAlreadyOwned
	}
	if buyerBalance < price {
		return 0, models.ErrSpecialInsufficientFunds
	}

	if _, err := tx.Exec(`UPDATE users SET bucks = bucks - ? WHERE id = ?`, price, buyerID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`UPDATE users SET bucks = bucks + ? WHERE id = ?`, price, sellerID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`UPDATE crate SET uid = ?, bv = 'vertices', aid = 0, price = ?, date = NOW() WHERE id = ?`, buyerID, price, crateID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`UPDATE shop_special_copies SET listing_price = 0, listing_version = listing_version + 1
		WHERE crate_id = ?`, crateID); err != nil {
		return 0, err
	}
	if err := unequipAvatarItem(tx, sellerID, item.Type, itemID); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return sellerID, nil
}
