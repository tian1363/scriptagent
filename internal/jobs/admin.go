package jobs

import (
	"errors"
	"strings"
)

// SetSoleAdministrator is server-only configuration, never a registration action.
// An empty ID revokes all admin roles. Invalid IDs leave existing roles unchanged.
func (s *Store) SetSoleAdministrator(id string) error {
	id = strings.TrimSpace(id)
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if id != "" {
		var count int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM users WHERE id=? AND status='active'`, id).Scan(&count); err != nil {
			return err
		}
		if count != 1 {
			return errors.New("administrator must be an existing active account")
		}
	}
	if _, err := tx.Exec(`UPDATE users SET role=CASE WHEN id=? THEN 'admin' ELSE 'member' END WHERE role='admin' OR id=?`, id, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) AdminOverview() (map[string]any, error) {
	totals := map[string]int64{}
	queries := map[string]string{
		"users":          `SELECT COUNT(*) FROM users`,
		"spaces":         `SELECT COUNT(*) FROM spaces`,
		"jobs":           `SELECT COUNT(*) FROM jobs`,
		"runs":           `SELECT COUNT(*) FROM agent_runs`,
		"model_calls":    `SELECT COUNT(*) FROM model_calls`,
		"total_tokens":   `SELECT COALESCE(SUM(total_tokens),0) FROM model_calls`,
		"published_jobs": `SELECT COUNT(*) FROM jobs WHERE status='published'`,
	}
	for key, query := range queries {
		var n int64
		if err := s.db.QueryRow(query).Scan(&n); err != nil {
			return nil, err
		}
		totals[key] = n
	}
	rows, err := s.db.Query(`SELECT s.id,s.title,
 (SELECT COUNT(*) FROM agent_runs r WHERE r.space_id=s.id),
 (SELECT COUNT(*) FROM agent_runs r WHERE r.space_id=s.id AND r.status='failed'),
 (SELECT COUNT(*) FROM model_calls m WHERE m.space_id=s.id),
 (SELECT COALESCE(SUM(total_tokens),0) FROM model_calls m WHERE m.space_id=s.id)
 FROM spaces s ORDER BY s.updated_at DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	spaces := []map[string]any{}
	for rows.Next() {
		var id, title string
		var runs, failed, calls, tokens int64
		if err := rows.Scan(&id, &title, &runs, &failed, &calls, &tokens); err != nil {
			rows.Close()
			return nil, err
		}
		spaces = append(spaces, map[string]any{"id": id, "title": title, "runs": runs, "failed_runs": failed, "model_calls": calls, "total_tokens": tokens})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = s.db.Query(`SELECT r.id,s.title,r.status,r.started_at FROM agent_runs r JOIN spaces s ON s.id=r.space_id ORDER BY r.started_at DESC LIMIT 20`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := []map[string]any{}
	for rows.Next() {
		var id, title, status, started string
		if err := rows.Scan(&id, &title, &status, &started); err != nil {
			return nil, err
		}
		runs = append(runs, map[string]any{"ID": id, "SpaceTitle": title, "Status": status, "StartedAt": started})
	}
	return map[string]any{"totals": totals, "spaces": spaces, "recent_runs": runs}, rows.Err()
}
