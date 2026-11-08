package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/session"
)

type clanNotice string

const (
	clanNoticeCreated             clanNotice = "created"
	clanNoticeUpdated             clanNotice = "updated"
	clanNoticeJoined              clanNotice = "joined"
	clanNoticeRequested           clanNotice = "requested"
	clanNoticeLeft                clanNotice = "left"
	clanNoticeRequestCancelled    clanNotice = "request-cancelled"
	clanNoticeTagOn               clanNotice = "tag-on"
	clanNoticeTagOff              clanNotice = "tag-off"
	clanNoticeAnnouncementPosted  clanNotice = "announcement-posted"
	clanNoticeAnnouncementDeleted clanNotice = "announcement-deleted"
	clanNoticeRoleSaved           clanNotice = "role-saved"
	clanNoticeRoleDeleted         clanNotice = "role-deleted"
	clanNoticeRequestApproved     clanNotice = "request-approved"
	clanNoticeRequestDenied       clanNotice = "request-denied"
	clanNoticeMemberUpdated       clanNotice = "member-updated"
	clanNoticeMemberRemoved       clanNotice = "member-removed"
)

const clanNoticeSessionKey = "clan_notice"

var clanNoticeText = map[clanNotice]string{
	clanNoticeCreated:             "Clan created.",
	clanNoticeUpdated:             "Clan updated.",
	clanNoticeJoined:              "Welcome to the clan.",
	clanNoticeRequested:           "Request sent. An admin will review it.",
	clanNoticeLeft:                "You left the clan.",
	clanNoticeRequestCancelled:    "Request cancelled.",
	clanNoticeTagOn:               "Clan tag equipped.",
	clanNoticeTagOff:              "Clan tag removed.",
	clanNoticeAnnouncementPosted:  "Announcement posted.",
	clanNoticeAnnouncementDeleted: "Announcement deleted.",
	clanNoticeRoleSaved:           "Role saved.",
	clanNoticeRoleDeleted:         "Role deleted.",
	clanNoticeRequestApproved:     "Member approved.",
	clanNoticeRequestDenied:       "Request denied.",
	clanNoticeMemberUpdated:       "Member updated.",
	clanNoticeMemberRemoved:       "Member removed.",
}

func setClanNotice(c fiber.Ctx, notice clanNotice) {
	sess := session.FromContext(c)
	if sess == nil {
		return
	}
	if _, ok := clanNoticeText[notice]; !ok {
		return
	}
	sess.Set(clanNoticeSessionKey, string(notice))
}

func takeClanNotice(c fiber.Ctx) (string, string) {
	sess := session.FromContext(c)
	if sess == nil {
		return "", ""
	}
	raw, _ := sess.Get(clanNoticeSessionKey).(string)
	sess.Set(clanNoticeSessionKey, "")
	if raw == "" {
		return "", ""
	}
	notice := clanNotice(raw)
	text, ok := clanNoticeText[notice]
	if !ok {
		return "", ""
	}
	return string(notice), text
}
