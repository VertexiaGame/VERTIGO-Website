package repository

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
)

func applySQLSections(t *testing.T, db *sql.DB, names ...string) {
	t.Helper()

	data, err := os.ReadFile("../../fat.sql")
	if err != nil {
		t.Fatal(err)
	}
	source := strings.ReplaceAll(string(data), "\r\n", "\n")
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	for _, name := range names {
		_, body, found := strings.Cut(source, "-- BEGIN "+name+"\n")
		body, _, ended := strings.Cut(body, "-- END "+name+"\n")
		if !found || !ended {
			t.Fatalf("missing SQL section %q in fat.sql", name)
		}
		for _, statement := range strings.Split(body, ";\n") {
			statement = strings.TrimSpace(statement)
			if statement == "" {
				continue
			}
			if _, err := conn.ExecContext(ctx, statement); err != nil {
				t.Fatalf("apply %q from fat.sql: %v", name, err)
			}
		}
	}
}
