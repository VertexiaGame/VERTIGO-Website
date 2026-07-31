package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"vertexia-frontend/backend/models"
)

type AvatarRepository struct {
	db *sql.DB
}

func NewAvatarRepository(db *sql.DB) *AvatarRepository {
	return &AvatarRepository{db: db}
}

func (r *AvatarRepository) GetAvatar(userID int) (*models.Avatar, error) {
	if r.db == nil {
		return &models.Avatar{
			ID:         userID,
			HeadColor:  "f3b700",
			TorsoColor: "c60000",
			LArmColor:  "f3b700",
			RArmColor:  "f3b700",
			LLegColor:  "650013",
			RLegColor:  "650013",
		}, nil
	}

	query := `SELECT head_color, larm_color, rarm_color, torso_color, lleg_color, rleg_color,
	                 hat1, hat2, hat3, hat4, hat5, tool, shirt, tshirt, pants, face
              FROM avatar WHERE id = ?`

	var a models.Avatar
	a.ID = userID
	err := r.db.QueryRow(query, userID).Scan(
		&a.HeadColor, &a.LArmColor, &a.RArmColor, &a.TorsoColor, &a.LLegColor, &a.RLegColor,
		&a.Hat1, &a.Hat2, &a.Hat3, &a.Hat4, &a.Hat5, &a.Tool, &a.Shirt, &a.TShirt, &a.Pants, &a.Face,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			a.HeadColor = "f3b700"
			a.TorsoColor = "c60000"
			a.LArmColor = "f3b700"
			a.RArmColor = "f3b700"
			a.LLegColor = "650013"
			a.RLegColor = "650013"
			return &a, nil
		}
		return nil, err
	}

	formatColor := func(hex string) string {
		hex = strings.TrimPrefix(hex, "#")
		if hex == "" {
			return "f3b700"
		}
		return hex
	}

	a.HeadColor = formatColor(a.HeadColor)
	a.TorsoColor = formatColor(a.TorsoColor)
	a.LArmColor = formatColor(a.LArmColor)
	a.RArmColor = formatColor(a.RArmColor)
	a.LLegColor = formatColor(a.LLegColor)
	a.RLegColor = formatColor(a.RLegColor)

	return &a, nil
}

func (r *AvatarRepository) UpdateBodyColor(userID int, bodyPart string, color string) error {
	if r.db == nil {
		return nil
	}
	color = strings.TrimPrefix(color, "#")
	if len(color) != 6 {
		return errors.New("invalid color format")
	}

	bodyPart = strings.ToLower(strings.TrimSpace(bodyPart))

	switch bodyPart {
	case "head":
		_, err := r.db.Exec("UPDATE avatar SET head_color = ? WHERE id = ?", color, userID)
		return err
	case "torso":
		_, err := r.db.Exec("UPDATE avatar SET torso_color = ? WHERE id = ?", color, userID)
		return err
	case "larm", "leftarm", "left_arm":
		_, err := r.db.Exec("UPDATE avatar SET larm_color = ? WHERE id = ?", color, userID)
		return err
	case "rarm", "rightarm", "right_arm":
		_, err := r.db.Exec("UPDATE avatar SET rarm_color = ? WHERE id = ?", color, userID)
		return err
	case "lleg", "leftleg", "left_leg":
		_, err := r.db.Exec("UPDATE avatar SET lleg_color = ? WHERE id = ?", color, userID)
		return err
	case "rleg", "rightleg", "right_leg":
		_, err := r.db.Exec("UPDATE avatar SET rleg_color = ? WHERE id = ?", color, userID)
		return err
	case "all":
		_, err := r.db.Exec("UPDATE avatar SET head_color = ?, torso_color = ?, larm_color = ?, rarm_color = ?, lleg_color = ?, rleg_color = ? WHERE id = ?", color, color, color, color, color, color, userID)
		return err
	default:
		return errors.New("invalid body part")
	}
}

func (r *AvatarRepository) EquipItem(userID int, itemType string, itemID int) error {
	if r.db == nil || itemID <= 0 {
		return nil
	}
	av, err := r.GetAvatar(userID)
	if err != nil {
		return err
	}

	itemType = strings.ToLower(strings.TrimSpace(itemType))

	switch itemType {
	case "hat", "hats":
		if av.Hat1 == itemID || av.Hat2 == itemID || av.Hat3 == itemID || av.Hat4 == itemID || av.Hat5 == itemID {
			return nil
		}
		if av.Hat1 == 0 {
			_, err = r.db.Exec("UPDATE avatar SET hat1 = ? WHERE id = ?", itemID, userID)
		} else if av.Hat2 == 0 {
			_, err = r.db.Exec("UPDATE avatar SET hat2 = ? WHERE id = ?", itemID, userID)
		} else if av.Hat3 == 0 {
			_, err = r.db.Exec("UPDATE avatar SET hat3 = ? WHERE id = ?", itemID, userID)
		} else if av.Hat4 == 0 {
			_, err = r.db.Exec("UPDATE avatar SET hat4 = ? WHERE id = ?", itemID, userID)
		} else if av.Hat5 == 0 {
			_, err = r.db.Exec("UPDATE avatar SET hat5 = ? WHERE id = ?", itemID, userID)
		} else {
			_, err = r.db.Exec("UPDATE avatar SET hat1 = ? WHERE id = ?", itemID, userID)
		}
	case "shirt", "shirts":
		_, err = r.db.Exec("UPDATE avatar SET shirt = ? WHERE id = ?", itemID, userID)
	case "pants":
		_, err = r.db.Exec("UPDATE avatar SET pants = ? WHERE id = ?", itemID, userID)
	case "tshirt", "tshirts":
		_, err = r.db.Exec("UPDATE avatar SET tshirt = ? WHERE id = ?", itemID, userID)
	case "face", "faces":
		_, err = r.db.Exec("UPDATE avatar SET face = ? WHERE id = ?", itemID, userID)
	case "gear", "tool", "tools":
		_, err = r.db.Exec("UPDATE avatar SET tool = ? WHERE id = ?", itemID, userID)
	default:
		return errors.New("invalid item type")
	}

	return err
}

func (r *AvatarRepository) UnequipItem(userID int, itemType string, itemID int) error {
	if r.db == nil || itemID <= 0 {
		return nil
	}
	itemType = strings.ToLower(strings.TrimSpace(itemType))

	switch itemType {
	case "hat", "hats":
		query := "UPDATE avatar SET hat1 = IF(hat1 = ?, 0, hat1), hat2 = IF(hat2 = ?, 0, hat2), hat3 = IF(hat3 = ?, 0, hat3), hat4 = IF(hat4 = ?, 0, hat4), hat5 = IF(hat5 = ?, 0, hat5) WHERE id = ?"
		_, err := r.db.Exec(query, itemID, itemID, itemID, itemID, itemID, userID)
		return err
	case "shirt", "shirts":
		_, err := r.db.Exec("UPDATE avatar SET shirt = 0 WHERE id = ? AND shirt = ?", userID, itemID)
		return err
	case "pants":
		_, err := r.db.Exec("UPDATE avatar SET pants = 0 WHERE id = ? AND pants = ?", userID, itemID)
		return err
	case "tshirt", "tshirts":
		_, err := r.db.Exec("UPDATE avatar SET tshirt = 0 WHERE id = ? AND tshirt = ?", userID, itemID)
		return err
	case "face", "faces":
		_, err := r.db.Exec("UPDATE avatar SET face = 0 WHERE id = ? AND face = ?", userID, itemID)
		return err
	case "gear", "tool", "tools":
		_, err := r.db.Exec("UPDATE avatar SET tool = 0 WHERE id = ? AND tool = ?", userID, itemID)
		return err
	default:
		return errors.New("invalid item type")
	}
}

func normalizeItemType(item *models.InventoryItem) {
	switch strings.ToLower(item.Type) {
	case "hat", "hats":
		item.TypeName = "Hat"
		item.Type = "hat"
	case "shirt", "shirts":
		item.TypeName = "Shirt"
		item.Type = "shirts"
	case "pants":
		item.TypeName = "Pants"
		item.Type = "pants"
	case "tshirt", "tshirts":
		item.TypeName = "T-Shirt"
		item.Type = "tshirts"
	case "face", "faces":
		item.TypeName = "Face"
		item.Type = "faces"
	case "gear", "tool", "tools":
		item.TypeName = "Gear"
		item.Type = "gear"
	}
}

func (r *AvatarRepository) lookupShopItems(ids []int) map[int]*models.InventoryItem {
	found := make(map[int]*models.InventoryItem)
	if r.db == nil || len(ids) == 0 {
		return found
	}

	placeholders := make([]string, 0, len(ids))
	args := make([]interface{}, 0, len(ids))
	for _, id := range ids {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}

	query := `SELECT s.id, s.name, s.type, COALESCE(s.description, ''), COALESCE(u.username, 'Polyoria')
              FROM shop s
              LEFT JOIN users u ON s.creator_id = u.id
              WHERE s.id IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return found
	}
	defer rows.Close()

	for rows.Next() {
		var item models.InventoryItem
		if err := rows.Scan(&item.ID, &item.Name, &item.Type, &item.Description, &item.CreatorName); err == nil {
			normalizeItemType(&item)
			found[item.ID] = &item
		}
	}
	return found
}

func (r *AvatarRepository) GetInventory(userID int, category, search string) ([]*models.InventoryItem, error) {
	avatar, err := r.GetAvatar(userID)
	if err != nil {
		avatar = &models.Avatar{ID: userID}
	}

	equippedMap := make(map[string]bool)
	if avatar.Hat1 > 0 { equippedMap[fmt.Sprintf("hat:%d", avatar.Hat1)] = true }
	if avatar.Hat2 > 0 { equippedMap[fmt.Sprintf("hat:%d", avatar.Hat2)] = true }
	if avatar.Hat3 > 0 { equippedMap[fmt.Sprintf("hat:%d", avatar.Hat3)] = true }
	if avatar.Hat4 > 0 { equippedMap[fmt.Sprintf("hat:%d", avatar.Hat4)] = true }
	if avatar.Hat5 > 0 { equippedMap[fmt.Sprintf("hat:%d", avatar.Hat5)] = true }
	if avatar.Shirt > 0 { equippedMap[fmt.Sprintf("shirts:%d", avatar.Shirt)] = true }
	if avatar.Pants > 0 { equippedMap[fmt.Sprintf("pants:%d", avatar.Pants)] = true }
	if avatar.TShirt > 0 { equippedMap[fmt.Sprintf("tshirts:%d", avatar.TShirt)] = true }
	if avatar.Face > 0 { equippedMap[fmt.Sprintf("faces:%d", avatar.Face)] = true }
	if avatar.Tool > 0 { equippedMap[fmt.Sprintf("gear:%d", avatar.Tool)] = true }

	var items []*models.InventoryItem

	if r.db != nil {
		query := `SELECT s.id, s.name, s.type, COALESCE(s.description, ''), COALESCE(u.username, 'Polyoria')
	              FROM inventory i
	              JOIN shop s ON i.item_id = s.id
	              LEFT JOIN users u ON s.creator_id = u.id
	              WHERE i.user_id = ?`
		rows, err := r.db.Query(query, userID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var item models.InventoryItem
				if err := rows.Scan(&item.ID, &item.Name, &item.Type, &item.Description, &item.CreatorName); err == nil {
					items = append(items, &item)
				}
			}
		}
	}

	category = strings.ToLower(strings.TrimSpace(category))
	search = strings.ToLower(strings.TrimSpace(search))

	var filtered []*models.InventoryItem
	for _, item := range items {
		normalizeItemType(item)

		item.IsEquipped = equippedMap[fmt.Sprintf("%s:%d", item.Type, item.ID)]

		if category != "" && category != "all" {
			matchCat := false
			switch category {
			case "hat", "hats":
				matchCat = (item.Type == "hat")
			case "shirt", "shirts":
				matchCat = (item.Type == "shirts")
			case "pants":
				matchCat = (item.Type == "pants")
			case "tshirt", "tshirts":
				matchCat = (item.Type == "tshirts")
			case "face", "faces":
				matchCat = (item.Type == "faces")
			case "gear", "tool", "tools":
				matchCat = (item.Type == "gear")
			}
			if !matchCat {
				continue
			}
		}

		if search != "" {
			if !strings.Contains(strings.ToLower(item.Name), search) &&
				!strings.Contains(strings.ToLower(item.CreatorName), search) {
				continue
			}
		}

		filtered = append(filtered, item)
	}

	return filtered, nil
}

func (r *AvatarRepository) GetEquippedItems(userID int) ([]*models.InventoryItem, error) {
	avatar, err := r.GetAvatar(userID)
	if err != nil {
		return nil, err
	}

	var ids []int
	for _, id := range []int{avatar.Hat1, avatar.Hat2, avatar.Hat3, avatar.Hat4, avatar.Hat5, avatar.Shirt, avatar.Pants, avatar.TShirt, avatar.Face, avatar.Tool} {
		if id > 0 {
			ids = append(ids, id)
		}
	}
	shopMap := r.lookupShopItems(ids)

	var equipped []*models.InventoryItem

	addEquipped := func(itemType string, id int) {
		if id <= 0 { return }
		if item, ok := shopMap[id]; ok {
			cp := *item
			cp.IsEquipped = true
			equipped = append(equipped, &cp)
		} else {
			typeName := "Item"
			switch itemType {
			case "hat": typeName = "Hat"
			case "shirts": typeName = "Shirt"
			case "pants": typeName = "Pants"
			case "tshirts": typeName = "T-Shirt"
			case "faces": typeName = "Face"
			case "gear": typeName = "Gear"
			}
			equipped = append(equipped, &models.InventoryItem{
				ID:          id,
				Name:        fmt.Sprintf("%s #%d", typeName, id),
				Type:        itemType,
				TypeName:    typeName,
				CreatorName: "Polyoria",
				IsEquipped:  true,
			})
		}
	}

	addEquipped("hat", avatar.Hat1)
	addEquipped("hat", avatar.Hat2)
	addEquipped("hat", avatar.Hat3)
	addEquipped("hat", avatar.Hat4)
	addEquipped("hat", avatar.Hat5)
	addEquipped("shirts", avatar.Shirt)
	addEquipped("pants", avatar.Pants)
	addEquipped("tshirts", avatar.TShirt)
	addEquipped("faces", avatar.Face)
	addEquipped("gear", avatar.Tool)

	return equipped, nil
}