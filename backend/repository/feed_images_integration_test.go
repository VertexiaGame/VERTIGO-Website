package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"

	"vertexia-frontend/backend/models"
)

func TestFeedImagesMySQL(t *testing.T) {
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
	name := fmt.Sprintf("vertigo_feed_test_%d", time.Now().UnixNano())
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
	applyImageLibraryMigration(t, db)
	for _, stmt := range []string{
		"INSERT INTO users (id, username) VALUES (1, 'owner'), (2, 'friend'), (3, 'other')",
		"INSERT INTO friends VALUES (2, 1, 'accepted'), (1, 3, 'pending')",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	repo := NewFeedRepository(db)
	image := models.ForumImageUpload{ContentType: "image/png", Content: make([]byte, models.ForumImageMaxBytes)}
	world, err := repo.CreatePost(1, "Public", "worldwide", []models.ForumImageUpload{image, image, image})
	if err != nil || world.ID != 1 {
		t.Fatalf("create public post: %+v, %v", world, err)
	}
	friends, err := repo.CreatePost(1, "", "friends", []models.ForumImageUpload{image})
	if err != nil || friends.ID != world.ID {
		t.Fatalf("create friends post: %+v, %v", friends, err)
	}
	worldID, friendID := world.ID, friends.ID

	queue, err := repo.PendingImages(0, 10)
	if err != nil || len(queue) != 4 || queue[0].FeedType != "worldwide" || queue[0].FeedID != worldID ||
		queue[0].Body != "Public" || queue[0].Author != "owner" || queue[3].FeedType != "friends" || queue[3].FeedID != friendID {
		t.Fatalf("admin queue: %+v, %v", queue, err)
	}
	nextPage, err := repo.PendingImages(queue[1].ID, 1)
	if err != nil || len(nextPage) != 1 || nextPage[0].ID != queue[2].ID {
		t.Fatalf("admin queue next page: %+v, %v", nextPage, err)
	}
	worldImages := []int{queue[0].ID, queue[2].ID}
	friendImages := []int{queue[3].ID}

	pendingWorld, err := repo.GetFeedPostByID(worldID, 0)
	if err != nil || pendingWorld == nil || len(pendingWorld.Images) != 0 {
		t.Fatalf("pending images leaked through public post lookup: %+v, %v", pendingWorld, err)
	}
	pendingWorldList, err := repo.GetRecentFeedPostsPaginated(10, 0, 0)
	if err != nil || len(pendingWorldList) != 1 || len(pendingWorldList[0].Images) != 0 {
		t.Fatalf("pending images leaked through public pagination: %+v, %v", pendingWorldList, err)
	}
	pendingFriend, err := repo.GetFriendsFeedPostByID(friendID, 1)
	if err != nil || pendingFriend != nil {
		t.Fatalf("image-only pending post was visible to its owner: %+v, %v", pendingFriend, err)
	}
	pendingFriendList, err := repo.GetRecentFriendsFeedPostsPaginated(10, 0, 2)
	if err != nil || len(pendingFriendList) != 0 {
		t.Fatalf("image-only pending post leaked through friends pagination: %+v, %v", pendingFriendList, err)
	}
	for _, entry := range queue {
		if _, err := repo.GetImage(entry.ID, 1); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("pending image accessible through direct URL: %v", err)
		}
		if preview, err := repo.GetPendingImage(entry.ID); err != nil || len(preview.Content) != len(image.Content) {
			t.Fatalf("pending admin preview: %v", err)
		}
	}
	for _, index := range []int{2, 0, 3} {
		if ok, err := repo.ReviewImage(queue[index].ID, true); err != nil || !ok {
			t.Fatalf("approve image: %v, %v", ok, err)
		}
	}
	if ok, err := repo.ReviewImage(queue[1].ID, false); err != nil || !ok {
		t.Fatalf("reject image: %v, %v", ok, err)
	}
	if _, err := repo.GetImage(queue[1].ID, 0); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("rejected image was accessible: %v", err)
	}
	if ok, err := repo.ReviewImage(queue[1].ID, true); err != nil || ok {
		t.Fatalf("rejected image could be approved: %v, %v", ok, err)
	}
	if ok, err := repo.ReviewImage(queue[0].ID, false); err != nil || ok {
		t.Fatalf("approved image could be reviewed twice: %v, %v", ok, err)
	}
	if _, err := repo.GetPendingImage(queue[0].ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("approved image remained available through pending preview: %v", err)
	}
	if queue, err := repo.PendingImages(0, 10); err != nil || len(queue) != 0 {
		t.Fatalf("reviewed images remained in the queue: %+v, %v", queue, err)
	}

	worldPosts, err := repo.GetRecentFeedPostsPaginated(10, 0, 3)
	if err != nil || len(worldPosts) != 1 || !slices.Equal(worldPosts[0].Images, worldImages) {
		t.Fatalf("public feed images: %v, %v", worldPosts, err)
	}
	friendPosts, err := repo.GetRecentFriendsFeedPostsPaginated(10, 0, 2)
	if err != nil || len(friendPosts) != 1 || !slices.Equal(friendPosts[0].Images, friendImages) {
		t.Fatalf("friends feed images: %v, %v", friendPosts, err)
	}
	if post, err := repo.GetFriendsFeedPostByID(friendID, 3); err != nil || post != nil {
		t.Fatalf("friends post visible to nonfriend: %+v, %v", post, err)
	}
	for _, viewer := range []int{0, 3} {
		if _, err := repo.GetImage(friendImages[0], viewer); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("friends image visible to viewer %d: %v", viewer, err)
		}
	}
	for _, viewer := range []int{1, 2} {
		if actual, err := repo.GetImage(friendImages[0], viewer); err != nil || len(actual.Content) != len(image.Content) {
			t.Fatalf("viewer %d could not fetch friends image: %v", viewer, err)
		}
	}
	if _, err := repo.GetImage(worldImages[0], 0); err != nil {
		t.Fatalf("public image not visible: %v", err)
	}
	if _, err := db.Exec("UPDATE ffeed SET removed = 'true' WHERE id = ?", friendID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetImage(friendImages[0], 1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("removed post's image still visible: %v", err)
	}

	invalid := models.ForumImageUpload{ContentType: image.ContentType}
	if _, err := repo.CreatePost(1, "Failed upload", "worldwide", []models.ForumImageUpload{image, invalid}); err == nil {
		t.Fatal("invalid image was stored")
	}
	next, err := repo.CreatePost(1, "Text only", "worldwide", nil)
	if err != nil || next.ID != worldID+1 {
		t.Fatalf("failed upload did not roll back: %+v, %v", next, err)
	}
	post, err := repo.GetFeedPostByID(next.ID, 1)
	if err != nil || post == nil || len(post.Images) != 0 {
		t.Fatalf("failed upload left an orphaned image: %+v, %v", post, err)
	}
}
