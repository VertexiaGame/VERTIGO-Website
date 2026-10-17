package models

import (
	"fmt"
	"html/template"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	BannerTypeColor    = "color"
	BannerTypeGradient = "gradient"
	BannerTypeImage    = "image"
)

const (
	BannerStatusPending  = "pending"
	BannerStatusApproved = "approved"
	BannerStatusDeclined = "declined"
)

const (
	BannerEffectNone   = "none"
	BannerEffectBlur   = "blur"
	BannerEffectDarken = "darken"
	BannerEffectZoom   = "zoom"
	BannerEffectDrift  = "drift"
)

const BannerImageUploadDir = "./static/uploads/banners"

type Banner struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Username  string    `json:"username"`
	Type      string    `json:"banner_type"`
	ColorA    string    `json:"color_a"`
	ColorB    string    `json:"color_b"`
	Angle     int       `json:"angle"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

var bannerHexColor = regexp.MustCompile(`^[0-9a-fA-F]{6}$`)

func NormalizeBannerType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case BannerTypeGradient:
		return BannerTypeGradient
	case BannerTypeImage:
		return BannerTypeImage
	default:
		return BannerTypeColor
	}
}

func NormalizeBannerHex(value, fallback string) string {
	cleaned := strings.TrimSpace(strings.TrimPrefix(value, "#"))
	if !bannerHexColor.MatchString(cleaned) {
		return fallback
	}
	return strings.ToUpper(cleaned)
}

func NormalizeBannerAngle(raw string, fallback int) int {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fallback
	}
	angle, err := strconv.Atoi(trimmed)
	if err != nil {
		return fallback
	}
	if angle < 0 {
		return 0
	}
	if angle > 360 {
		return 360
	}
	return angle
}

func NormalizeBannerEffect(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case BannerEffectBlur:
		return BannerEffectBlur
	case BannerEffectDarken:
		return BannerEffectDarken
	case BannerEffectZoom:
		return BannerEffectZoom
	case BannerEffectDrift:
		return BannerEffectDrift
	default:
		return BannerEffectNone
	}
}

func BannerEffectName(effect string) string {
	switch NormalizeBannerEffect(effect) {
	case BannerEffectBlur:
		return "Blur"
	case BannerEffectDarken:
		return "Darken"
	case BannerEffectZoom:
		return "Zoom"
	case BannerEffectDrift:
		return "Drift"
	default:
		return "None"
	}
}

func BannerEffectFilter(effect string) template.CSS {
	switch NormalizeBannerEffect(effect) {
	case BannerEffectBlur:
		return template.CSS("filter:blur(8px);transform:scale(1.05);")
	case BannerEffectDarken:
		return template.CSS("filter:brightness(0.65);")
	case BannerEffectZoom:
		return template.CSS("animation:prfbnr-zoom 18s cubic-bezier(0.25, 1, 0.5, 1) infinite alternate;")
	case BannerEffectDrift:
		return template.CSS("animation:prfbnr-drift 24s linear infinite alternate;")
	default:
		return template.CSS("")
	}
}

func BannerImageURL(id int) string {
	return "/static/uploads/banners/" + strconv.Itoa(id) + ".png"
}

func BannerImagePath(id int) string {
	return filepath.Join(BannerImageUploadDir, strconv.Itoa(id)+".png")
}

func (b *Banner) TypeName() string {
	switch b.Type {
	case BannerTypeGradient:
		return "Gradient"
	case BannerTypeImage:
		return "Image"
	default:
		return "Color"
	}
}

func (u *User) TextBgStyle() template.CSS {
	hex := NormalizeBannerHex(u.BannerTextBgColor, "F0F0F0")
	opacity := u.BannerTextBgOpacity
	if opacity < 0 {
		opacity = 0
	}
	if opacity > 100 {
		opacity = 100
	}
	var r, g, b int
	fmt.Sscanf(hex, "%02X%02X%02X", &r, &g, &b)
	border := NormalizeBannerHex(u.BannerTextBgBorder, "A9A9A9")
	text := NormalizeBannerHex(u.BannerTextBgText, "1A202C")
	return template.CSS(fmt.Sprintf("background:rgba(%d,%d,%d,%.2f);border-color:#%s;color:#%s;", r, g, b, float64(opacity)/100, border, text))
}

func (b *Banner) StatusLabel() string {
	switch b.Status {
	case BannerStatusApproved:
		return "Approved"
	case BannerStatusDeclined:
		return "Declined"
	default:
		return "Pending"
	}
}

func (b *Banner) Style() template.CSS {
	switch b.Type {
	case BannerTypeGradient:
		return template.CSS(fmt.Sprintf("background:linear-gradient(%ddeg,#%s,#%s);", b.Angle, b.ColorA, b.ColorB))
	case BannerTypeImage:
		return template.CSS(fmt.Sprintf("background:url('%s') center/cover no-repeat;", BannerImageURL(b.ID)))
	default:
		return template.CSS(fmt.Sprintf("background:#%s;", b.ColorA))
	}
}
