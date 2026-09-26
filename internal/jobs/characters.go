package jobs

import (
	"encoding/json"
	"errors"
	"time"
)

type Character struct {
	ID          string          `json:"id"`
	UserID      string          `json:"-"`
	Name        string          `json:"name"`
	Source      string          `json:"source"`
	ReferenceID string          `json:"reference_id"`
	Status      string          `json:"status"`
	Path        string          `json:"-"`
	Controls    json.RawMessage `json:"controls"`
	Error       string          `json:"error"`
	CreatedAt   string          `json:"created_at"`
}

func (s *Store) CreateCharacter(userID, name, source, reference, status string, controls json.RawMessage) (*Character, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	var count int
	if err := s.db.QueryRow(`SELECT count(*) FROM characters WHERE user_id=?`, userID).Scan(&count); err != nil {
		return nil, err
	}
	if count >= 200 {
		return nil, errors.New("角色库已达到 200 项上限")
	}
	if len(controls) == 0 {
		controls = json.RawMessage(`{}`)
	}
	x := Character{ID: newID(), UserID: userID, Name: name, Source: source, ReferenceID: reference, Status: status, Controls: controls, CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	_, err := s.db.Exec(`INSERT INTO characters(id,user_id,name,source,reference_id,status,path,controls_json,error_message,created_at) VALUES(?,?,?,?,?,?,'',?,'',?)`, x.ID, userID, name, source, reference, status, string(controls), x.CreatedAt)
	return &x, err
}
func (s *Store) ListCharacters(userID string) ([]Character, error) {
	rows, err := s.db.Query(`SELECT id,name,source,reference_id,status,controls_json,error_message,created_at FROM characters WHERE user_id=? ORDER BY created_at DESC,id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Character{}
	for rows.Next() {
		var x Character
		var controls string
		if err = rows.Scan(&x.ID, &x.Name, &x.Source, &x.ReferenceID, &x.Status, &controls, &x.Error, &x.CreatedAt); err != nil {
			return nil, err
		}
		x.Controls = json.RawMessage(controls)
		result = append(result, x)
	}
	return result, rows.Err()
}
func (s *Store) GetCharacter(userID, id string) (*Character, error) {
	var x Character
	var controls string
	err := s.db.QueryRow(`SELECT id,user_id,name,source,reference_id,status,path,controls_json,error_message,created_at FROM characters WHERE user_id=? AND id=?`, userID, id).Scan(&x.ID, &x.UserID, &x.Name, &x.Source, &x.ReferenceID, &x.Status, &x.Path, &controls, &x.Error, &x.CreatedAt)
	x.Controls = json.RawMessage(controls)
	return &x, err
}
func (s *Store) FinishCharacter(userID, id, path, message string) error {
	status := "ready"
	if message != "" {
		status = "failed"
	}
	_, err := s.db.Exec(`UPDATE characters SET status=?,path=?,error_message=? WHERE user_id=? AND id=?`, status, path, message, userID, id)
	return err
}
func (s *Store) InterruptCharacterGenerations() error {
	_, err := s.db.Exec(`UPDATE characters SET status='failed',error_message='服务已重启，生成被中断。请重新创建，避免重复扣费不会自动重试。' WHERE status='generating'`)
	return err
}
