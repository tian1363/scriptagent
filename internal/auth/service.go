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

type Service struct{ store *jobs.Store }

func NewService(store *jobs.Store) *Service { return &Service{store: store} }
func CookieName() string                    { return sessionCookieName }

func (s *Service) RegistrationAvailable() (bool, error) {
	count, err := s.store.CountUsers()
	return count == 0, err
}

func (s *Service) Register(email, password, name string) (*jobs.User, *jobs.Session, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, nil, errors.New("请输入邮箱")
	}
	if len(password) < 8 {
		return nil, nil, errors.New("密码至少需要 8 位")
	}
	existingCount, err := s.store.CountUsers()
	if err != nil {
		return nil, nil, err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return nil, nil, err
	}
	user, err := s.store.CreateUser(jobs.CreateUserInput{Email: email, Name: strings.TrimSpace(name), Role: "admin", Status: "active", PasswordHash: hash})
	if err != nil {
		return nil, nil, err
	}
	if existingCount == 0 {
		if err := s.store.AdoptLegacyData(user.ID); err != nil {
			return nil, nil, err
		}
	}
	session, err := s.createSession(user.ID)
	return user, session, err
}

func (s *Service) Login(email, password string) (*jobs.User, *jobs.Session, error) {
	user, err := s.store.GetUserByEmail(strings.ToLower(strings.TrimSpace(email)))
	if err != nil || !checkPassword(user.PasswordHash, password) {
		return nil, nil, errors.New("邮箱或密码不正确")
	}
	if user.Status != "active" {
		return nil, nil, errors.New("账号已停用")
	}
	session, err := s.createSession(user.ID)
	return user, session, err
}

func (s *Service) Authenticate(token string) (*jobs.User, error) {
	session, err := s.store.GetSession(strings.TrimSpace(token))
	if err != nil {
		return nil, errors.New("登录已失效")
	}
	if time.Now().UTC().After(session.ExpiresAt) {
		_ = s.store.DeleteSession(token)
		return nil, errors.New("登录已过期")
	}
	return s.store.GetUser(session.UserID)
}

func (s *Service) Logout(token string) error { return s.store.DeleteSession(strings.TrimSpace(token)) }

func (s *Service) createSession(userID string) (*jobs.Session, error) {
	token, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	return s.store.CreateSession(jobs.CreateSessionInput{UserID: userID, Token: token, ExpiresAt: time.Now().UTC().Add(30 * 24 * time.Hour)})
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, passwordIters, passwordKeyLen)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("pbkdf2_sha256$%d$%s$%s", passwordIters, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func checkPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2_sha256" {
		return false
	}
	var iterations int
	if _, err := fmt.Sscanf(parts[1], "%d", &iterations); err != nil || iterations < 1 {
		return false
	}
	salt, err1 := base64.RawStdEncoding.DecodeString(parts[2])
	expected, err2 := base64.RawStdEncoding.DecodeString(parts[3])
	if err1 != nil || err2 != nil {
		return false
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(expected))
	return err == nil && subtle.ConstantTimeCompare(key, expected) == 1
}

func randomToken(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
