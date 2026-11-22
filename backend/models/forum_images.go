package models

const (
	ForumImageMaxCount = 3
	ForumImageMaxBytes = 2 << 20
)

type ForumImageUpload struct {
	ContentType string
	Content     []byte
}

type PendingForumImage struct {
	ID, ThreadID, ReplyID int
	Title, Author, Body   string
}
