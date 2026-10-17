package models

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const MaxShopTransformBytes = 2048

func ValidateShopTransform(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if len(raw) > MaxShopTransformBytes {
		return "", errors.New("transform data is too large")
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return "", errors.New("invalid transform data")
	}
	checkVector := func(value any, positive bool) error {
		vec, ok := value.(map[string]any)
		if !ok {
			return errors.New("invalid transform data")
		}
		for _, axis := range []string{"x", "y", "z"} {
			num, ok := vec[axis].(float64)
			if !ok || num < -1000 || num > 1000 {
				return errors.New("invalid transform data")
			}
			if positive && (num <= 0 || num > 1000) {
				return errors.New("invalid transform data")
			}
		}
		return nil
	}
	for key, value := range decoded {
		switch key {
		case "position", "rotation":
			if err := checkVector(value, false); err != nil {
				return "", err
			}
		case "scale":
			if err := checkVector(value, true); err != nil {
				return "", err
			}
		case "placement":
			placed, ok := value.(map[string]any)
			if !ok {
				return "", errors.New("invalid transform data")
			}
			for _, sub := range []string{"position", "rotation", "scale"} {
				subVal, ok := placed[sub]
				if !ok {
					return "", errors.New("invalid transform data")
				}
				if err := checkVector(subVal, sub == "scale"); err != nil {
					return "", err
				}
			}
		case "weld", "snap":
			if value == nil {
				continue
			}
			str, ok := value.(string)
			if !ok || len(str) > 64 {
				return "", errors.New("invalid transform data")
			}
		default:
			return "", errors.New("invalid transform data")
		}
	}
	canonical, err := json.Marshal(decoded)
	if err != nil {
		return "", errors.New("invalid transform data")
	}
	return string(canonical), nil
}

type ShopItem struct {
	ID                  int            `json:"id"`
	CID                 int            `json:"cid"`
	CreatorType         string         `json:"creator_type"`
	CreatorName         string         `json:"creator_name"`
	Name                string         `json:"name"`
	Description         string         `json:"description"`
	Type                string         `json:"type"`
	TypeName            string         `json:"type_name"`
	Bucks               int            `json:"bucks"`
	Bits                int            `json:"bits"`
	Special             string         `json:"special"`
	Stock               int            `json:"stock"`
	Deleted             int            `json:"deleted"`
	UGC                 string         `json:"ugc"`
	ApprovalState       string         `json:"approval_state"`
	OnSale              string         `json:"onsale"`
	OriginalTitle       sql.NullString `json:"original_title"`
	OriginalDescription sql.NullString `json:"original_description"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	OriginalStock       int            `json:"original_stock"`
	RAP                 int            `json:"rap"`
	OffSale             string         `json:"offsale"`
	Transform           string         `json:"transform,omitempty"`
}

type ShopFilter struct {
	Search   string `json:"search"`
	Category string `json:"category"`
	Currency string `json:"currency"`
	MinPrice int    `json:"min_price"`
	MaxPrice int    `json:"max_price"`
	Creator  string `json:"creator"`
	Sort     string `json:"sort"`
	Page     int    `json:"page"`
	Limit    int    `json:"limit"`
}

const (
	ShopApprovalApproved = "approved"
	ShopApprovalDeclined = "declined"
	ShopApprovalPending  = "pending"
)

func (s *ShopItem) ApprovalLabel() string {
	switch s.ApprovalState {
	case ShopApprovalApproved:
		return "Approved"
	case ShopApprovalDeclined, "rejected":
		return "Rejected"
	default:
		return "Pending"
	}
}

func (s *ShopItem) IsPending() bool {
	return s.ApprovalState != ShopApprovalApproved && s.ApprovalState != ShopApprovalDeclined && s.ApprovalState != "rejected"
}

func NormalizeShopCategory(cat string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(cat)) {
	case "hat", "hats":
		return "hat", "Hat"
	case "face", "faces":
		return "faces", "Face"
	case "shirt", "shirts":
		return "shirts", "Shirt"
	case "tshirt", "tshirts":
		return "tshirts", "T-Shirt"
	case "pants":
		return "pants", "Pants"
	case "gear", "tool", "tools":
		return "gear", "Gear"
	default:
		return "all", "All Items"
	}
}