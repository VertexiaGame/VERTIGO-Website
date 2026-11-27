package service

import (
	"testing"

	"vertexia-frontend/backend/models"
)

func TestCanAssignRoleRequiresPermissionSubset(t *testing.T) {
	officer := &ClanAccess{IsMember: true, Role: &models.ClanRole{Rank: 50, Permissions: 0b0011}}
	cases := []struct {
		name string
		role *models.ClanRole
		want bool
	}{
		{"lower rank, subset", &models.ClanRole{Rank: 10, Permissions: 0b0001}, true},
		{"lower rank, extra permission", &models.ClanRole{Rank: 10, Permissions: 0b0100}, false},
		{"equal rank", &models.ClanRole{Rank: 50, Permissions: 0}, false},
		{"owner role", &models.ClanRole{Rank: 1, IsOwner: true}, false},
	}
	for _, tc := range cases {
		if got := canAssignRole(officer, tc.role); got != tc.want {
			t.Errorf("%s: canAssignRole = %v, want %v", tc.name, got, tc.want)
		}
	}
	owner := &ClanAccess{IsOwner: true}
	if !canAssignRole(owner, &models.ClanRole{Rank: 10, Permissions: 0b1111}) {
		t.Error("clan owner should assign any non-owner role")
	}
}
