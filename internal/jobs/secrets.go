package jobs

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const encryptedSecretPrefix = "enc:v1:"

type secretCipher struct{ aead cipher.AEAD }

func (s *Store) ConfigureSecretEncryption(encodedKey string) error {
	encodedKey = strings.TrimSpace(encodedKey)
	if encodedKey == "" {
		return nil
	}
	key, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil || len(key) != 32 {
		return errors.New("SCRIPT_AGENT_ENCRYPTION_KEY must be a base64-encoded 32-byte key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	s.secrets = &secretCipher{aead: aead}
	return s.migratePlaintextSecrets()
}

func (s *Store) encryptSecret(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, encryptedSecretPrefix) {
		return value, nil
	}
	if s.secrets == nil {
		return "", errors.New("cannot save an API key until SCRIPT_AGENT_ENCRYPTION_KEY is configured")
	}
	nonce := make([]byte, s.secrets.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := s.secrets.aead.Seal(nil, nonce, []byte(value), nil)
	return encryptedSecretPrefix + base64.RawURLEncoding.EncodeToString(append(nonce, sealed...)), nil
}

func (s *Store) decryptSecret(value string) (string, error) {
	if value == "" || !strings.HasPrefix(value, encryptedSecretPrefix) {
		return value, nil
	}
	if s.secrets == nil {
		return "", errors.New("encrypted API key cannot be read without SCRIPT_AGENT_ENCRYPTION_KEY")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, encryptedSecretPrefix))
	if err != nil || len(raw) < s.secrets.aead.NonceSize() {
		return "", errors.New("invalid encrypted API key")
	}
	nonceSize := s.secrets.aead.NonceSize()
	plain, err := s.secrets.aead.Open(nil, raw[:nonceSize], raw[nonceSize:], nil)
	if err != nil {
		return "", errors.New("unable to decrypt API key")
	}
	return string(plain), nil
}

func (s *Store) migratePlaintextSecrets() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	targets := []struct{ table, keys string }{
		{"model_settings", "id"},
		{"model_capability_settings", "capability"},
		{"user_model_capability_settings", "user_id || ':' || capability"},
	}
	for _, target := range targets {
		query := fmt.Sprintf(`SELECT %s,api_key FROM %s WHERE COALESCE(api_key,'')!='' AND api_key NOT LIKE 'enc:v1:%%'`, target.keys, target.table)
		rows, err := s.db.Query(query)
		if err != nil {
			return err
		}
		var updates [][2]string
		for rows.Next() {
			var id, value string
			if err := rows.Scan(&id, &value); err != nil {
				rows.Close()
				return err
			}
			encrypted, err := s.encryptSecret(value)
			if err != nil {
				rows.Close()
				return err
			}
			updates = append(updates, [2]string{id, encrypted})
		}
		rows.Close()
		for _, update := range updates {
			var result error
			if target.table == "user_model_capability_settings" {
				parts := strings.SplitN(update[0], ":", 2)
				if len(parts) != 2 {
					return errors.New("invalid model setting identity")
				}
				_, result = s.db.Exec(`UPDATE user_model_capability_settings SET api_key=? WHERE user_id=? AND capability=?`, update[1], parts[0], parts[1])
			} else {
				_, result = s.db.Exec(fmt.Sprintf(`UPDATE %s SET api_key=? WHERE %s=?`, target.table, target.keys), update[1], update[0])
			}
			if result != nil {
				return result
			}
		}
	}
	return nil
}
