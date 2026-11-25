package models

import "errors"

const (
	ForumImageMaxCount = 3
	ForumImageMaxBytes = 2 << 20
)

type ForumImageUpload struct {
	SavedID     int64
	Name        string
	ContentType string
	Content     []byte
}

var ErrSavedImageUnavailable = errors.New("a selected image is unavailable or still awaiting approval")

type SavedImage struct {
	ID       int64  `json:"id,string"`
	Name     string `json:"name"`
	Approved bool   `json:"approved"`
}

type PendingForumImage struct {
	ID, ThreadID, ReplyID int
	Title, Author, Body   string
}
