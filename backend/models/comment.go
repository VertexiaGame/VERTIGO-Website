package models

import (
	"strconv"
	"time"
)

const (
	CommentTargetShop = "shop"
	CommentTargetGame = "game"
)

const (
	CommentMinLength = 3
	CommentMaxLength = 500
	CommentPageSize  = 20
)

func ValidCommentTarget(kind string) bool {
	return kind == CommentTargetShop || kind == CommentTargetGame
}

type Comment struct {
	ID           int        `json:"id"`
	TargetType   string     `json:"target_type"`
	TargetID     int        `json:"target_id"`
	UserID       int        `json:"user_id"`
	ParentID     int        `json:"parent_id,omitempty"`
	Username     string     `json:"username"`
	Body         string     `json:"body"`
	Removed      bool       `json:"-"`
	Edited       bool       `json:"edited"`
	CreationDate time.Time  `json:"creation_date"`
	TimeAgo      string     `json:"time_ago"`
	FullDate     string     `json:"full_date"`
	CanDelete    bool       `json:"can_delete"`
	Replies      []*Comment `json:"replies,omitempty"`
}

type CommentSection struct {
	TargetType string
	TargetID   int
	Comments   []*Comment
	HasMore    bool
	CanPost    bool
	IsLoggedIn bool
	CSRF       string
	Error      string
}

func (s *CommentSection) PostURL() string {
	return CommentPostURL(s.TargetType, s.TargetID)
}

func CommentPostURL(kind string, id int) string {
	if kind == CommentTargetGame {
		return "/play/" + strconv.Itoa(id) + "/comments"
	}
	return "/shop/item/" + strconv.Itoa(id) + "/comments"
}

func (s *CommentSection) LastID() int {
	if len(s.Comments) == 0 {
		return 0
	}
	return s.Comments[len(s.Comments)-1].ID
}
