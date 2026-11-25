package repository

import (
	"database/sql"
	"os"
	"strings"
	"testing"
)

const imageTestSchema = `
CREATE TABLE users (
	id INT PRIMARY KEY, username VARCHAR(64), displayname VARCHAR(64)
);
CREATE TABLE forum_threads (
	id INT AUTO_INCREMENT PRIMARY KEY, board_id INT, user_id INT, title VARCHAR(100), body TEXT,
	pinned BOOLEAN DEFAULT FALSE, locked BOOLEAN DEFAULT FALSE,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE forum_replies (
	id INT AUTO_INCREMENT PRIMARY KEY, thread_id INT, user_id INT, body TEXT,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE forum_images (
	id INT AUTO_INCREMENT PRIMARY KEY, thread_id INT, reply_id INT,
	content_type VARCHAR(32) NOT NULL, content MEDIUMBLOB NOT NULL, approved BOOLEAN NOT NULL DEFAULT FALSE,
	KEY idx_forum_images_queue (approved, id),
	FOREIGN KEY (thread_id) REFERENCES forum_threads (id) ON DELETE CASCADE,
	FOREIGN KEY (reply_id) REFERENCES forum_replies (id) ON DELETE CASCADE
);
CREATE TABLE feed (
	id INT PRIMARY KEY, user_id INT, status TEXT NOT NULL, removed VARCHAR(5) DEFAULT 'false',
	edited VARCHAR(5) DEFAULT 'false', edit_date TIMESTAMP NULL, creation_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE ffeed (
	id INT PRIMARY KEY, userid INT, status TEXT NOT NULL, removed VARCHAR(5) DEFAULT 'false',
	edited VARCHAR(5) DEFAULT 'false', editdate TIMESTAMP NULL, creationdate TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE feed_images (
	id INT AUTO_INCREMENT PRIMARY KEY, feed_id INT NOT NULL, feed_type ENUM('worldwide', 'friends') NOT NULL,
	content_type VARCHAR(32) NOT NULL, content MEDIUMBLOB NOT NULL, approved BOOLEAN NOT NULL DEFAULT FALSE,
	KEY idx_feed_images_queue (approved, id)
);
CREATE TABLE friends (uid INT, fid INT, state VARCHAR(16));
CREATE TABLE freact (uid INT, fid INT);
CREATE TABLE ffreact LIKE freact;
`

func applyImageTestSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	execImageTestSQL(t, db, imageTestSchema)
}

func applyImageLibraryMigration(t *testing.T, db *sql.DB) {
	t.Helper()
	migration, err := os.ReadFile("../database/migrations/001_community_images.sql")
	if err != nil {
		t.Fatal(err)
	}
	execImageTestSQL(t, db, string(migration))
}

func execImageTestSQL(t *testing.T, db *sql.DB, source string) {
	t.Helper()
	for _, statement := range strings.Split(source, ";") {
		statement = strings.TrimSpace(statement)
		if statement != "" {
			if _, err := db.Exec(statement); err != nil {
				t.Fatalf("execute %q: %v", statement, err)
			}
		}
	}
}
