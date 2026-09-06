package jobs

import (
	"crypto/sha256"
	"encoding/base64"
	"path/filepath"
	"testing"
)

func TestInviteLifecycle(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	code, err := s.GenerateInvite("tester", 7)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(code))
	id := base64.RawURLEncoding.EncodeToString(sum[:])
	user, err := s.CreateUserWithInvite(CreateUserInput{Email: "tester@example.com", PasswordHash: "test"}, id)
	if err != nil {
		t.Fatal(err)
	}
	items, err := s.ListInvites()
	if err != nil || len(items) != 1 {
		t.Fatalf("list: %v", err)
	}
	if items[0].UsedBy != user.ID || items[0].Email != user.Email || items[0].Status != "used" || items[0].UsedAt == "" {
		t.Fatalf("missing usage: %+v", items[0])
	}
	if _, err := s.CreateUserWithInvite(CreateUserInput{Email: "reuse@example.com"}, id); err == nil {
		t.Fatal("reused code accepted")
	}
	code, _ = s.GenerateInvite("revoke", 7)
	sum = sha256.Sum256([]byte(code))
	id = base64.RawURLEncoding.EncodeToString(sum[:])
	if err := s.RevokeInvite(id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUserWithInvite(CreateUserInput{Email: "revoked@example.com"}, id); err == nil {
		t.Fatal("revoked code accepted")
	}
	code, _ = s.GenerateInvite("expiry", 7)
	sum = sha256.Sum256([]byte(code))
	id = base64.RawURLEncoding.EncodeToString(sum[:])
	if _, err := s.db.Exec(`UPDATE registration_invites SET expires_at='2000-01-01T00:00:00Z' WHERE code_hash=?`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUserWithInvite(CreateUserInput{Email: "expired@example.com"}, id); err == nil {
		t.Fatal("expired code accepted")
	}
	if available, err := s.HasAvailableRegistrationInvite(); err != nil || available {
		t.Fatalf("invalid availability: %v %v", available, err)
	}
	code, _ = s.GenerateInvite("rollback", 7)
	sum = sha256.Sum256([]byte(code))
	id = base64.RawURLEncoding.EncodeToString(sum[:])
	if _, err := s.CreateUserWithInvite(CreateUserInput{Email: user.Email}, id); err == nil {
		t.Fatal("duplicate user accepted")
	}
	if _, err := s.CreateUserWithInvite(CreateUserInput{Email: "fresh@example.com"}, id); err != nil {
		t.Fatalf("failed registration consumed invite: %v", err)
	}
}
