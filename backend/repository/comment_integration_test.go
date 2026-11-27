package repository

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

func TestCommentsMySQL(t *testing.T) {
	if os.Getenv("VERTIGO_TEST_MYSQL") != "1" {
		t.Skip("set VERTIGO_TEST_MYSQL=1 to run the MySQL integration test")
	}
	_ = godotenv.Load("../../.env")
	cfg := mysql.NewConfig()
	cfg.User, cfg.Passwd, cfg.Net = os.Getenv("DB_USER"), os.Getenv("DB_PASS"), "tcp"
	cfg.Addr = os.Getenv("DB_HOST") + ":" + os.Getenv("DB_PORT")
	cfg.ParseTime = true
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	name := fmt.Sprintf("vertigo_comments_test_%d", time.Now().UnixNano())
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
	execImageTestSQL(t, db, "CREATE TABLE users (id INT PRIMARY KEY, username VARCHAR(64), displayname VARCHAR(64)) ENGINE=InnoDB")
	migration, err := os.ReadFile("../database/migrations/003_comments.sql")
	if err != nil {
		t.Fatal(err)
	}
	execImageTestSQL(t, db, string(migration))
	execImageTestSQL(t, db, string(migration))
	if _, err := db.Exec("INSERT INTO users (id, username) VALUES (1, 'alice'), (2, 'bob')"); err != nil {
		t.Fatal(err)
	}

	repo := NewCommentRepository(db)
	mustCreate := func(kind string, target, user, parent int, body string) int {
		t.Helper()
		id, err := repo.Create(kind, target, user, parent, body)
		if err != nil || id <= 0 {
			t.Fatalf("create %q: %d %v", body, id, err)
		}
		return id
	}
	first := mustCreate("shop", 10, 1, 0, "first")
	second := mustCreate("shop", 10, 2, 0, "second")
	reply := mustCreate("shop", 10, 1, second, "reply")
	mustCreate("game", 10, 1, 0, "same id, other kind")
	mustCreate("shop", 11, 1, 0, "other item")

	top, err := repo.ListTop("shop", 10, 0, 10)
	if err != nil || len(top) != 2 || top[0].ID != second || top[1].ID != first {
		t.Fatalf("ListTop = %+v, %v", top, err)
	}
	if top[0].Username != "bob" || top[0].Body != "second" || top[0].ParentID != 0 || top[0].CreationDate.IsZero() {
		t.Errorf("scanned comment = %+v", top[0])
	}
	if page, _ := repo.ListTop("shop", 10, second, 10); len(page) != 1 || page[0].ID != first {
		t.Errorf("ListTop before %d = %+v", second, page)
	}
	if page, _ := repo.ListTop("shop", 10, 0, 1); len(page) != 1 {
		t.Errorf("limit not applied: %d rows", len(page))
	}
	replies, err := repo.ListReplies([]int{first, second})
	if err != nil || len(replies) != 1 || replies[0].ID != reply || replies[0].ParentID != second {
		t.Fatalf("ListReplies = %+v, %v", replies, err)
	}
	if got, _ := repo.ListReplies(nil); got != nil {
		t.Errorf("ListReplies(nil) = %+v", got)
	}

	if err := repo.Remove(second); err != nil {
		t.Fatal(err)
	}
	if top, _ := repo.ListTop("shop", 10, 0, 10); len(top) != 1 || top[0].ID != first {
		t.Errorf("removed comment still listed: %+v", top)
	}
	got, err := repo.Get(second)
	if err != nil || got == nil || !got.Removed || got.Body != "second" {
		t.Errorf("removed row must be kept: %+v %v", got, err)
	}
	if r, _ := repo.Get(reply); r == nil || r.Removed {
		t.Errorf("reply row changed by removing its parent: %+v", r)
	}
	if missing, err := repo.Get(9999); missing != nil || err != nil {
		t.Errorf("Get(missing) = %+v, %v", missing, err)
	}
	if _, err := repo.Create("wall", 1, 1, 0, "bad kind"); err == nil {
		t.Error("target_type outside shop/game should be rejected")
	}
}
