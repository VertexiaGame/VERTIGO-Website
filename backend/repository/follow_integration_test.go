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

func TestFollowRelationshipsMySQL(t *testing.T) {
	if os.Getenv("VERTIGO_TEST_MYSQL") != "1" {
		t.Skip("set VERTIGO_TEST_MYSQL=1 to run the MySQL integration test")
	}
	if err := godotenv.Load("../../.env"); err != nil {
		t.Fatal(err)
	}
	cfg := mysql.NewConfig()
	cfg.User, cfg.Passwd, cfg.Net = os.Getenv("DB_USER"), os.Getenv("DB_PASS"), "tcp"
	cfg.Addr = os.Getenv("DB_HOST") + ":" + os.Getenv("DB_PORT")
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	name := fmt.Sprintf("vertigo_follow_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE DATABASE `" + name + "`"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec("DROP DATABASE `" + name + "`"); err != nil {
			t.Errorf("remove test database: %v", err)
		}
	}()
	cfg.DBName = name
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	applySQLSections(t, db, "follow fixtures")
	for _, q := range []string{
		`INSERT INTO users VALUES (1, 'alice', 'Alice'), (2, 'bob', ''), (3, 'carol', 'Carol')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	repo := NewUserRepository(db)
	for _, pair := range [][2]int{{1, 2}, {1, 2}, {3, 2}, {2, 3}} {
		if err := repo.SetFollowing(pair[0], pair[1], true); err != nil {
			t.Fatal(err)
		}
	}
	followers, err := repo.GetFollowList(2, true)
	if err != nil || len(followers) != 2 || followers[0].Username != "alice" || followers[1].Username != "carol" {
		t.Fatalf("followers: %+v, %v", followers, err)
	}
	following, err := repo.GetFollowList(1, false)
	if err != nil || len(following) != 1 || following[0].Username != "bob" || following[0].DisplayName != "bob" {
		t.Fatalf("following: %+v, %v", following, err)
	}
	if _, f, g := repo.GetUserSocialCounts(2); f != 2 || g != 1 {
		t.Fatalf("counts: followers %d, following %d", f, g)
	}
	if err := repo.SetFollowing(1, 2, false); err != nil {
		t.Fatal(err)
	}
	isFollowing, err := repo.IsFollowing(1, 2)
	if err != nil || isFollowing {
		t.Fatalf("unfollow: following=%v err=%v", isFollowing, err)
	}
	followers, err = repo.GetFollowList(2, true)
	if err != nil || len(followers) != 1 || followers[0].Username != "carol" {
		t.Fatalf("followers after unfollow: %+v, %v", followers, err)
	}
}
