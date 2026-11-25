package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"

	"vertexia-frontend/backend/models"
)

func TestForumImageModerationMySQL(t *testing.T) {
	if os.Getenv("VERTIGO_TEST_MYSQL") != "1" {
		t.Skip("set VERTIGO_TEST_MYSQL=1 to run the MySQL integration test")
	}
	if err := godotenv.Load("../../.env"); err != nil {
		t.Fatal(err)
	}
	cfg := mysql.NewConfig()
	cfg.User, cfg.Passwd, cfg.Net = os.Getenv("DB_USER"), os.Getenv("DB_PASS"), "tcp"
	cfg.Addr = os.Getenv("DB_HOST") + ":" + os.Getenv("DB_PORT")
	cfg.ParseTime = true
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	name := fmt.Sprintf("vertigo_forum_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec("DROP DATABASE " + name); err != nil {
			t.Errorf("remove test database: %v", err)
		}
	}()
	cfg.DBName = name
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	applyImageTestSchema(t, db)
	execImageTestSQL(t, db, `
		INSERT INTO forum_threads (id, board_id, user_id, title, body) VALUES (10, 1, 1, 'Legacy', 'Post');
		INSERT INTO forum_images (id, thread_id, content_type, content, approved)
		VALUES (10, 10, 'image/png', 'old forum image', TRUE);
		INSERT INTO ffeed (id, userid, status) VALUES (10, 2, 'Old friends post');
		INSERT INTO feed_images (id, feed_id, feed_type, content_type, content, approved)
		VALUES (10, 10, 'friends', 'image/png', 'old feed image', FALSE);
	`)
	applyImageLibraryMigration(t, db)

	repo := NewForumRepository(db)
	feedRepo := NewFeedRepository(db)
	legacyForum, err := repo.SavedImages(1, 0, 10)
	if err != nil || len(legacyForum) != 1 || !legacyForum[0].Approved {
		t.Fatalf("legacy forum library entry: %+v, %v", legacyForum, err)
	}
	if image, err := repo.GetForumImage(10, true); err != nil || string(image.Content) != "old forum image" {
		t.Fatalf("legacy forum attachment changed: %v", err)
	}
	legacyFeed, err := repo.SavedImages(2, 0, 10)
	if err != nil || len(legacyFeed) != 1 || legacyFeed[0].Approved {
		t.Fatalf("legacy feed library entry: %+v, %v", legacyFeed, err)
	}
	savedFeed := models.ForumImageUpload{SavedID: legacyFeed[0].ID}
	if _, err := repo.CreateReply(10, 2, "Pending", savedFeed); !errors.Is(err, models.ErrSavedImageUnavailable) {
		t.Fatalf("pending image accepted for reuse: %v", err)
	}
	if image, err := repo.SavedImage(2, savedFeed.SavedID); err != nil || string(image.Content) != "old feed image" {
		t.Fatalf("owner could not preview pending image: %v", err)
	}
	if image, err := feedRepo.GetPendingImage(10); err != nil || string(image.Content) != "old feed image" {
		t.Fatalf("legacy feed attachment changed: %v", err)
	}
	if ok, err := feedRepo.ReviewImage(10, true); err != nil || !ok {
		t.Fatalf("approve migrated feed image: %v, %v", ok, err)
	}
	if _, err := repo.SavedImage(1, savedFeed.SavedID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("library preview leaked to another user: %v", err)
	}
	if _, err := repo.CreateReply(10, 1, "Not my image", savedFeed); !errors.Is(err, models.ErrSavedImageUnavailable) {
		t.Fatalf("foreign image accepted for reuse: %v", err)
	}
	reusedReply, err := repo.CreateReply(10, 2, "Reused from the feed", savedFeed)
	if err != nil {
		t.Fatal(err)
	}
	oldThread := &models.ForumThread{ID: 10}
	replies, err := repo.GetReplies(10, 10, 0)
	if err != nil || len(replies) != 1 || replies[0].ID != reusedReply {
		t.Fatalf("rejected references left a reply: %+v, %v", replies, err)
	}
	if err := repo.LoadApprovedImages(oldThread, replies); err != nil || len(replies[0].Images) != 1 {
		t.Fatalf("approved feed image missing in forum reply: %+v, %v", replies[0], err)
	}
	if _, err := db.Exec("DELETE FROM forum_threads WHERE id = 10"); err != nil {
		t.Fatal(err)
	}
	savedForum := models.ForumImageUpload{SavedID: legacyForum[0].ID}
	reusedPost, err := feedRepo.CreatePost(1, "", "worldwide", []models.ForumImageUpload{savedForum})
	if err != nil || len(reusedPost.Images) != 1 {
		t.Fatalf("image-only feed post with saved forum image: %+v, %v", reusedPost, err)
	}
	if image, err := feedRepo.GetImage(reusedPost.Images[0], 0); err != nil || string(image.Content) != "old forum image" {
		t.Fatalf("reused public feed image missing: %v", err)
	}
	if queue, err := feedRepo.PendingImages(0, 10); err != nil || len(queue) != 0 {
		t.Fatalf("reused image re-entered review: %+v, %v", queue, err)
	}

	image := models.ForumImageUpload{ContentType: "image/png", Content: []byte("image data")}
	threadID, err := repo.CreateThread(1, 1, "Title", "Original post", image, image, image)
	if err != nil {
		t.Fatal(err)
	}
	replyID, err := repo.CreateReply(threadID, 2, "Reply", image)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateReply(threadID, 2, "Too many images", image, image, image, image); err == nil {
		t.Fatal("reply with four images was accepted")
	}
	pending, err := repo.PendingForumImages(0, 10)
	if err != nil || len(pending) != 4 || pending[0].ThreadID != threadID || pending[0].ReplyID != 0 || pending[3].ReplyID != replyID {
		t.Fatalf("pending queue: %+v, %v", pending, err)
	}

	thread, err := repo.GetThreadByID(threadID)
	if err != nil || thread == nil {
		t.Fatalf("thread: %+v, %v", thread, err)
	}
	replies, err = repo.GetReplies(threadID, 50, 0)
	if err != nil || len(replies) != 1 {
		t.Fatalf("replies: %+v, %v", replies, err)
	}
	if err := repo.LoadApprovedImages(thread, replies); err != nil || len(thread.Images) != 0 || len(replies[0].Images) != 0 {
		t.Fatalf("pending attachments leaked into the thread: %v", err)
	}
	if _, err := repo.GetForumImage(pending[0].ID, true); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("pending image leaked through the public image lookup: %v", err)
	}
	if preview, err := repo.GetForumImage(pending[0].ID, false); err != nil || string(preview.Content) != "image data" {
		t.Fatalf("pending preview: %+v, %v", preview, err)
	}

	for _, index := range []int{2, 0, 3} {
		if ok, err := repo.ReviewForumImage(pending[index].ID, true); err != nil || !ok {
			t.Fatalf("approve image: %v, %v", ok, err)
		}
	}
	if ok, err := repo.ReviewForumImage(pending[1].ID, false); err != nil || !ok {
		t.Fatalf("reject image: %v, %v", ok, err)
	}
	if ok, err := repo.ReviewForumImage(pending[1].ID, true); err != nil || ok {
		t.Fatalf("rejected image could be approved: %v, %v", ok, err)
	}
	if err := repo.LoadApprovedImages(thread, replies); err != nil {
		t.Fatal(err)
	}
	if len(thread.Images) != 2 || thread.Images[0] != pending[0].ID || thread.Images[1] != pending[2].ID ||
		len(replies[0].Images) != 1 || replies[0].Images[0] != pending[3].ID {
		t.Fatalf("approved images out of order or attached to the wrong post: %v, %v", thread.Images, replies[0].Images)
	}
	if approved, err := repo.GetForumImage(pending[0].ID, true); err != nil || approved.ContentType != "image/png" || string(approved.Content) != "image data" {
		t.Fatalf("approved public image: %+v, %v", approved, err)
	}
}
