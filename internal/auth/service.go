package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"crypto/pbkdf2"

	"github.com/tian1363/scriptagent/internal/jobs"
)

const (
	sessionCookieName = "scriptagent_session"
	passwordIters     = 210000
	passwordKeyLen    = 32
)

type Service struct {
	store *jobs.Store
}

func NewService(store *jobs.Store) *Service {
	return &Service{store: store}
}

func CookieName() string {
	return sessionCookieName
}

func (s *Service) Register(email, password, name string) (*jobs.User, *jobs.Session, error) {
	email = normalizeEmail(email)
	if email == "" {
		return nil, nil, errors.New("email is required")
	}
	if len(password) < 8 {
		return nil, nil, errors.New("password must be at least 8 characters")
	}
	existingCount, err := s.store.CountUsers()
	if err != nil {
		return nil, nil, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return nil, nil, err
	}
	user, err := s.store.CreateUser(jobs.CreateUserInput{
		Email:        email,
		Name:         strings.TrimSpace(name),
		Role:         firstUserRole(existingCount),
		Status:       "active",
		PasswordHash: hash,
	})
	if err != nil {
		return nil, nil, err
	}
	if existingCount == 0 {
		_ = s.store.AdoptLegacyData(user.ID)
	}
	session, err := s.createSession(user.ID)
	if err != nil {
		return nil, nil, err
	}
	return user, session, nil
}

func (s *Service) Login(email, password string) (*jobs.User, *jobs.Session, error) {
	user, err := s.store.GetUserByEmail(normalizeEmail(email))
	if err != nil {
		return nil, nil, errors.New("email or password is incorrect")
	}
	if !CheckPassword(user.PasswordHash, password) {
		return nil, nil, errors.New("email or password is incorrect")
	}
	if user.Status != "active" {
		return nil, nil, errors.New("account is disabled")
	}
	session, err := s.createSession(user.ID)
	if err != nil {
		return nil, nil, err
	}
	return user, session, nil
}

func (s *Service) Authenticate(token string) (*jobs.User, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("session token is required")
	}
	session, err := s.store.GetSession(token)
	if err != nil {
		return nil, err
	}
	if time.Now().UTC().After(session.ExpiresAt) {
		_ = s.store.DeleteSession(token)
		return nil, errors.New("session expired")
	}
	user, err := s.store.GetUser(session.UserID)
	if err != nil {
		return nil, err
	}
	if user.Status != "active" {
		return nil, errors.New("account is disabled")
	}
	return user, nil
}

func (s *Service) Logout(token string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	return s.store.DeleteSession(token)
}

func (s *Service) createSession(userID string) (*jobs.Session, error) {
	token, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	return s.store.CreateSession(jobs.CreateSessionInput{
		UserID:    userID,
		Token:     token,
		ExpiresAt: time.Now().UTC().Add(30 * 24 * time.Hour),
	})
}

func HashPassword(password string) (string, error) {
	salt, err := randomBytes(16)
	if err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, passwordIters, passwordKeyLen)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("pbkdf2_sha256$%d$%s$%s", passwordIters, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func CheckPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2_sha256" {
		return false
	}
	var iters int
	if _, err := fmt.Sscanf(parts[1], "%d", &iters); err != nil || iters <= 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, iters, len(expected))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(key, expected) == 1
}

func randomToken(size int) (string, error) {
	value, err := randomBytes(size)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func randomBytes(size int) ([]byte, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return nil, err
	}
	return value, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func firstUserRole(existingCount int) string {
	if existingCount == 0 {
		return "admin"
	}
	return "member"
}
