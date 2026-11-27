package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"vertexia-frontend/backend/models"
)

type fakeCommentStore struct {
	rows   []*models.Comment
	nextID int
}

func (f *fakeCommentStore) Create(kind string, targetID, userID, parentID int, body string) (int, error) {
	f.nextID++
	f.rows = append(f.rows, &models.Comment{ID: f.nextID, TargetType: kind, TargetID: targetID, UserID: userID,
		ParentID: parentID, Username: "u", Body: body, CreationDate: time.Now()})
	return f.nextID, nil
}

func (f *fakeCommentStore) Get(id int) (*models.Comment, error) {
	for _, c := range f.rows {
		if c.ID == id {
			cp := *c
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *fakeCommentStore) ListTop(kind string, targetID, beforeID, limit int) ([]*models.Comment, error) {
	var out []*models.Comment
	for i := len(f.rows) - 1; i >= 0 && len(out) < limit; i-- {
		c := f.rows[i]
		if c.TargetType == kind && c.TargetID == targetID && !c.Removed && c.ParentID == 0 && (beforeID == 0 || c.ID < beforeID) {
			cp := *c
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (f *fakeCommentStore) ListReplies(parentIDs []int) ([]*models.Comment, error) {
	var out []*models.Comment
	for _, c := range f.rows {
		for _, p := range parentIDs {
			if c.ParentID == p && !c.Removed {
				cp := *c
				out = append(out, &cp)
			}
		}
	}
	return out, nil
}

func (f *fakeCommentStore) Remove(id int) error {
	for _, c := range f.rows {
		if c.ID == id {
			c.Removed = true
		}
	}
	return nil
}

func TestCommentPostValidation(t *testing.T) {
	s := newCommentServiceWithStore(&fakeCommentStore{}, nil)
	user := &models.User{ID: 1, Username: "alice"}
	item := CommentTarget{Kind: models.CommentTargetShop, ID: 10}

	if _, err := s.Post(nil, item, 0, "hello"); err == nil {
		t.Error("anonymous post should fail")
	}
	for _, body := range []string{"", "  ", "hi", strings.Repeat("x", models.CommentMaxLength+1)} {
		if _, err := s.Post(user, item, 0, body); !errors.Is(err, ErrCommentLength) {
			t.Errorf("body of %d chars: want ErrCommentLength, got %v", len(body), err)
		}
	}
	if _, err := s.Post(user, CommentTarget{Kind: "wall", ID: 1}, 0, "hello"); !errors.Is(err, ErrCommentTarget) {
		t.Errorf("unknown target: got %v", err)
	}
	c, err := s.Post(user, item, 0, "  héllo wörld  ")
	if err != nil {
		t.Fatal(err)
	}
	if c.Body != "héllo wörld" || c.Username != "alice" || !c.CanDelete || c.ParentID != 0 {
		t.Errorf("unexpected comment %+v", c)
	}
	if _, err := s.Post(user, item, 0, strings.Repeat("é", models.CommentMaxLength)); err != nil {
		t.Errorf("500 multi-byte characters should be allowed: %v", err)
	}
}

func TestCommentRepliesAreOneLevel(t *testing.T) {
	store := &fakeCommentStore{}
	s := newCommentServiceWithStore(store, nil)
	user := &models.User{ID: 1, Username: "alice"}
	game := CommentTarget{Kind: models.CommentTargetGame, ID: 5}

	root, _ := s.Post(user, game, 0, "root comment")
	reply, err := s.Post(user, game, root.ID, "first reply")
	if err != nil || reply.ParentID != root.ID {
		t.Fatalf("reply: %+v %v", reply, err)
	}
	nested, err := s.Post(user, game, reply.ID, "reply to the reply")
	if err != nil || nested.ParentID != root.ID {
		t.Fatalf("reply to a reply should attach to the top-level comment: %+v %v", nested, err)
	}
	if _, err := s.Post(user, CommentTarget{Kind: models.CommentTargetShop, ID: 5}, root.ID, "wrong target"); !errors.Is(err, ErrCommentNotFound) {
		t.Errorf("parent on another target: got %v", err)
	}
	if _, err := s.Post(user, game, 999, "missing parent"); !errors.Is(err, ErrCommentNotFound) {
		t.Errorf("missing parent: got %v", err)
	}

	list, more, err := s.List(nil, game, 0)
	if err != nil || more || len(list) != 1 || len(list[0].Replies) != 2 {
		t.Fatalf("list = %+v more=%v err=%v", list, more, err)
	}
	if list[0].CanDelete || list[0].Replies[0].CanDelete {
		t.Error("anonymous viewer should not be offered remove")
	}

	if err := s.Remove(user, list[0], game); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Post(user, game, root.ID, "reply to removed"); !errors.Is(err, ErrCommentNotFound) {
		t.Errorf("reply to a removed comment: got %v", err)
	}
	if list, _, _ := s.List(nil, game, 0); len(list) != 0 {
		t.Errorf("removed comment still listed: %+v", list)
	}
	if got, _ := store.Get(reply.ID); got == nil || got.Removed {
		t.Error("removing a comment must not touch its replies' rows")
	}
}

func TestCommentListPaging(t *testing.T) {
	s := newCommentServiceWithStore(&fakeCommentStore{}, nil)
	user := &models.User{ID: 1, Username: "alice"}
	item := CommentTarget{Kind: models.CommentTargetShop, ID: 1}
	for i := 0; i < models.CommentPageSize+3; i++ {
		if _, err := s.Post(user, item, 0, "comment"); err != nil {
			t.Fatal(err)
		}
	}
	first, more, _ := s.List(user, item, 0)
	if len(first) != models.CommentPageSize || !more || first[0].ID != models.CommentPageSize+3 {
		t.Fatalf("first page: %d comments, more=%v, newest=%d", len(first), more, first[0].ID)
	}
	second, more, _ := s.List(user, item, first[len(first)-1].ID)
	if len(second) != 3 || more || second[2].ID != 1 {
		t.Fatalf("second page: %d comments, more=%v", len(second), more)
	}
}

func TestCommentRemovePermissions(t *testing.T) {
	author := &models.User{ID: 1}
	other := &models.User{ID: 2}
	owner := &models.User{ID: 3}
	mod := &models.User{ID: 4, Power: models.PowerModerator}
	trainee := &models.User{ID: 5, Power: models.PowerTrainee}
	c := &models.Comment{ID: 7, UserID: 1, TargetType: models.CommentTargetShop, TargetID: 10}
	target := CommentTarget{Kind: models.CommentTargetShop, ID: 10, OwnerID: 3}

	for _, tc := range []struct {
		name string
		user *models.User
		want bool
	}{{"author", author, true}, {"other", other, false}, {"owner", owner, true}, {"moderator", mod, true}, {"trainee", trainee, false}, {"anonymous", nil, false}} {
		if got := CanRemoveComment(tc.user, c, target); got != tc.want {
			t.Errorf("%s: CanRemoveComment = %v, want %v", tc.name, got, tc.want)
		}
	}
	if CanRemoveComment(owner, c, CommentTarget{Kind: models.CommentTargetShop, ID: 10}) {
		t.Error("an item without a user creator has no owner")
	}

	s := newCommentServiceWithStore(&fakeCommentStore{rows: []*models.Comment{c}}, nil)
	if err := s.Remove(other, c, target); !errors.Is(err, ErrCommentPermission) {
		t.Errorf("other user: got %v", err)
	}
	if err := s.Remove(mod, c, CommentTarget{Kind: models.CommentTargetGame, ID: 10}); !errors.Is(err, ErrCommentNotFound) {
		t.Errorf("wrong target: got %v", err)
	}
	if err := s.Remove(mod, c, target); err != nil {
		t.Errorf("moderator: %v", err)
	}
}

func TestCommentCooldown(t *testing.T) {
	s := newCommentServiceWithStore(&fakeCommentStore{}, NewCooldownService(time.Second))
	user := &models.User{ID: 1, Username: "alice"}
	item := CommentTarget{Kind: models.CommentTargetShop, ID: 1}
	if _, err := s.Post(user, item, 0, "first one"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Post(user, item, 0, "second one"); err == nil || !strings.Contains(err.Error(), "slow down") {
		t.Errorf("second post inside the cooldown: got %v", err)
	}
}
