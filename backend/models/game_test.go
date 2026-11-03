package models

import (
	"database/sql"
	"testing"
)

func TestGameVisibleTo(t *testing.T) {
	creatorID := sql.NullInt64{Int64: 5, Valid: true}
	cases := []struct {
		name    string
		game    Game
		viewer  int
		visible bool
	}{
		{"public to guest", Game{}, 0, true},
		{"public to other user", Game{}, 6, true},
		{"private hidden from guest", Game{Private: true}, 0, false},
		{"private hidden from other user", Game{Private: true, CreatorID: creatorID}, 6, false},
		{"private visible to creator", Game{Private: true, CreatorID: creatorID}, 5, true},
		{"locked hidden from creator", Game{Private: true, Locked: true, CreatorID: creatorID}, 5, false},
	}
	for _, tc := range cases {
		if got := tc.game.VisibleTo(tc.viewer); got != tc.visible {
			t.Fatalf("%s: VisibleTo(%d) = %t, want %t", tc.name, tc.viewer, got, tc.visible)
		}
	}
}
