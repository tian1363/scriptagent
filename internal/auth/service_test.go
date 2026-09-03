package auth

import (
	"path/filepath"
	"testing"

	"github.com/tian1363/scriptagent/internal/jobs"
)

func TestSingleAccountRegistrationAndSession(t *testing.T) {
	store, err := jobs.OpenStore(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	service := NewService(store)
	available, err := service.RegistrationAvailable()
	if err != nil || !available {
		t.Fatalf("expected registration to be available: available=%v err=%v", available, err)
	}

	user, session, err := service.Register("Admin@Example.com", "safe-password", "管理员")
	if err != nil {
		t.Fatal(err)
	}
	if user.Email != "admin@example.com" || user.PasswordHash == "" {
		t.Fatalf("unexpected public user: %+v", user)
	}
	second, _, err := service.Register("other@example.com", "safe-password", "Other")
	if err != nil || second.ID == user.ID {
		t.Fatalf("expected a distinct second account: user=%+v err=%v", second, err)
	}
	if _, _, err := service.Login(user.Email, "wrong-password"); err == nil {
		t.Fatal("expected an invalid password to be rejected")
	}

	authenticated, err := service.Authenticate(session.Token)
	if err != nil || authenticated.ID != user.ID {
		t.Fatalf("expected session to authenticate: user=%+v err=%v", authenticated, err)
	}
	if err := service.Logout(session.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(session.Token); err == nil {
		t.Fatal("expected logged-out session to be rejected")
	}
}
