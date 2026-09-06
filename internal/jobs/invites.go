package jobs

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"time"
)

type Invite struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at"`
	UsedBy    string `json:"used_by"`
	UsedAt    string `json:"used_at"`
	Email     string `json:"email"`
}

func (s *Store) GenerateInvite(label string, days int) (string, error) {
	if days < 1 || days > 365 || len([]rune(label)) > 100 {
		return "", errors.New("有效期应为 1–365 天，备注不超过 100 字")
	}
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	code := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(code))
	now := time.Now().UTC()
	_, err := s.db.Exec(`INSERT INTO registration_invites(code_hash,max_uses,uses,created_at,label,expires_at) VALUES(?,1,0,?,?,?)`, base64.RawURLEncoding.EncodeToString(hash[:]), now.Format(time.RFC3339), strings.TrimSpace(label), now.Add(time.Duration(days)*24*time.Hour).Format(time.RFC3339))
	return code, err
}

func (s *Store) ListInvites() ([]Invite, error) {
	rows, err := s.db.Query(`SELECT i.code_hash,i.label,i.created_at,i.expires_at,i.used_by,i.used_at,COALESCE(u.email,''),i.uses,i.revoked FROM registration_invites i LEFT JOIN users u ON u.id=i.used_by ORDER BY i.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Invite{}
	for rows.Next() {
		var item Invite
		var uses, revoked int
		if err := rows.Scan(&item.ID, &item.Label, &item.CreatedAt, &item.ExpiresAt, &item.UsedBy, &item.UsedAt, &item.Email, &uses, &revoked); err != nil {
			return nil, err
		}
		item.Status = "unused"
		if uses > 0 {
			item.Status = "used"
		} else if revoked != 0 {
			item.Status = "revoked"
		} else if item.ExpiresAt != "" && item.ExpiresAt <= time.Now().UTC().Format(time.RFC3339) {
			item.Status = "expired"
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) RevokeInvite(id string) error {
	result, err := s.db.Exec(`UPDATE registration_invites SET revoked=1 WHERE code_hash=? AND uses=0`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return errors.New("邀请码不存在或已使用")
	}
	return nil
}
