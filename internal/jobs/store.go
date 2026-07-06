package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/tian1363/scriptagent/internal/model"
	"github.com/tian1363/scriptagent/internal/userctx"
	_ "modernc.org/sqlite"
)

type Store struct {
	db      *sql.DB
	writeMu sync.Mutex
}

func OpenStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	store := &Store{db: db}
	if err := store.configure(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) configure() error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA foreign_keys=ON",
	}
	for _, pragma := range pragmas {
		if _, err := s.db.Exec(pragma); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) migrate() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS jobs (
  id TEXT PRIMARY KEY,
  user_id TEXT,
  title TEXT NOT NULL,
  status TEXT NOT NULL,
  video_path TEXT NOT NULL,
  video_original_name TEXT NOT NULL,
  product_md_path TEXT NOT NULL,
  product_md_name TEXT NOT NULL,
  requirement TEXT,
  industry TEXT NOT NULL,
  fission_count INTEGER NOT NULL,
  analysis_markdown TEXT,
  replica_script_json TEXT,
  fission_scripts_json TEXT,
  creatibi_result_json TEXT,
  error_message TEXT,
  run_log TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
`)
	if err != nil {
		return err
	}
	if err := s.ensureColumn("jobs", "run_log", "TEXT"); err != nil {
		return err
	}
	if err := s.ensureColumn("jobs", "fission_directions", "TEXT"); err != nil {
		return err
	}
	if err := s.ensureColumn("jobs", "user_id", "TEXT"); err != nil {
		return err
	}
	_, err = s.db.Exec(`
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  name TEXT,
  role TEXT NOT NULL DEFAULT 'member',
  status TEXT NOT NULL DEFAULT 'active',
  password_hash TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
  token TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS chat_conversations (
  id TEXT PRIMARY KEY,
  user_id TEXT,
  title TEXT NOT NULL,
  summary TEXT,
  summary_message_id TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS chat_messages (
  id TEXT PRIMARY KEY,
  conversation_id TEXT NOT NULL,
  role TEXT NOT NULL,
  content TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY(conversation_id) REFERENCES chat_conversations(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS model_calls (
  id TEXT PRIMARY KEY,
  user_id TEXT,
  scope TEXT NOT NULL,
  ref_id TEXT NOT NULL,
  step TEXT NOT NULL,
  model TEXT NOT NULL,
  input_json TEXT,
  output_text TEXT,
  response_json TEXT,
  prompt_tokens INTEGER NOT NULL DEFAULT 0,
  output_tokens INTEGER NOT NULL DEFAULT 0,
  total_tokens INTEGER NOT NULL DEFAULT 0,
  latency_ms INTEGER NOT NULL DEFAULT 0,
  error_message TEXT,
  created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_chat_messages_conversation ON chat_messages(conversation_id, created_at);
CREATE INDEX IF NOT EXISTS idx_model_calls_ref ON model_calls(ref_id, created_at);
CREATE INDEX IF NOT EXISTS idx_model_calls_created ON model_calls(created_at);

CREATE TABLE IF NOT EXISTS products (
  id TEXT PRIMARY KEY,
  user_id TEXT,
  title TEXT NOT NULL,
  md_path TEXT NOT NULL,
  md_name TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS creative_reports (
  id TEXT PRIMARY KEY,
  user_id TEXT,
  product_id TEXT NOT NULL,
  product_title TEXT NOT NULL,
  source_config_json TEXT NOT NULL,
  report_markdown TEXT NOT NULL,
  report_summary TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY(product_id) REFERENCES products(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS product_chunks (
  id TEXT PRIMARY KEY,
  product_id TEXT NOT NULL,
  chunk_index INTEGER NOT NULL,
  heading TEXT,
  content TEXT NOT NULL,
  embedding_json TEXT NOT NULL,
  embedding_model TEXT NOT NULL,
  embedding_dim INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY(product_id) REFERENCES products(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS model_settings (
  id TEXT PRIMARY KEY,
  user_id TEXT,
  api_key TEXT,
  endpoint TEXT NOT NULL,
  model TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_products_updated ON products(updated_at);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_creative_reports_product ON creative_reports(product_id, created_at);
CREATE INDEX IF NOT EXISTS idx_product_chunks_product ON product_chunks(product_id, chunk_index);
`)
	if err != nil {
		return err
	}
	if err := s.ensureColumn("chat_conversations", "summary", "TEXT"); err != nil {
		return err
	}
	if err := s.ensureColumn("chat_conversations", "summary_message_id", "TEXT"); err != nil {
		return err
	}
	if err := s.ensureColumn("users", "role", "TEXT NOT NULL DEFAULT 'member'"); err != nil {
		return err
	}
	if err := s.ensureColumn("users", "status", "TEXT NOT NULL DEFAULT 'active'"); err != nil {
		return err
	}
	for _, table := range []string{"chat_conversations", "model_calls", "products", "creative_reports", "model_settings"} {
		if err := s.ensureColumn(table, "user_id", "TEXT"); err != nil {
			return err
		}
	}
	if err := s.ensureAdminUser(); err != nil {
		return err
	}
	_, err = s.db.Exec(`
CREATE INDEX IF NOT EXISTS idx_products_user_updated ON products(user_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_jobs_user_created ON jobs(user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_chat_conversations_user_updated ON chat_conversations(user_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_model_calls_user_created ON model_calls(user_id, created_at);
`)
	return err
}

func (s *Store) ensureAdminUser() error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err := s.db.Exec(`
UPDATE users
SET role = 'admin'
WHERE id = (
  SELECT id FROM users ORDER BY created_at ASC LIMIT 1
)`)
	return err
}

func (s *Store) CountUsers() (int, error) {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) CreateUser(input CreateUserInput) (*User, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	now := time.Now().UTC()
	user := &User{
		ID:           newID(),
		Email:        strings.ToLower(strings.TrimSpace(input.Email)),
		Name:         strings.TrimSpace(input.Name),
		Role:         valueOr(input.Role, "member"),
		Status:       valueOr(input.Status, "active"),
		PasswordHash: input.PasswordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	_, err := s.db.Exec(`
INSERT INTO users (id, email, name, role, status, password_hash, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		user.ID, user.Email, user.Name, user.Role, user.Status, user.PasswordHash, now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Store) GetUser(id string) (*User, error) {
	return scanUser(s.db.QueryRow(`
SELECT id, email, name, role, status, password_hash, created_at, updated_at
FROM users WHERE id = ?`, id))
}

func (s *Store) GetUserByEmail(email string) (*User, error) {
	return scanUser(s.db.QueryRow(`
SELECT id, email, name, role, status, password_hash, created_at, updated_at
FROM users WHERE email = ?`, strings.ToLower(strings.TrimSpace(email))))
}

func (s *Store) ListAdminUsers() ([]AdminUser, error) {
	rows, err := s.db.Query(`
SELECT
  u.id,
  u.email,
  u.name,
  u.role,
  u.status,
  CASE WHEN COALESCE(ms.api_key, '') != '' THEN 1 ELSE 0 END AS model_configured,
  (SELECT COUNT(*) FROM products p WHERE p.user_id = u.id) AS product_count,
  (SELECT COUNT(*) FROM jobs j WHERE j.user_id = u.id) AS job_count,
  (SELECT COUNT(*) FROM chat_conversations c WHERE c.user_id = u.id) AS chat_count,
  u.created_at,
  u.updated_at
FROM users u
LEFT JOIN model_settings ms ON ms.user_id = u.id
ORDER BY u.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []AdminUser{}
	for rows.Next() {
		var user AdminUser
		var createdAt, updatedAt string
		var name sql.NullString
		var modelConfigured int
		if err := rows.Scan(
			&user.ID, &user.Email, &name, &user.Role, &user.Status, &modelConfigured,
			&user.ProductCount, &user.JobCount, &user.ChatCount, &createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}
		user.Name = name.String
		user.ModelConfigured = modelConfigured == 1
		user.CreatedAt = parseTime(createdAt)
		user.UpdatedAt = parseTime(updatedAt)
		result = append(result, user)
	}
	return result, rows.Err()
}

func (s *Store) UpdateUserStatus(id, status string) error {
	status = strings.TrimSpace(status)
	if status != "active" && status != "disabled" {
		return errors.New("invalid user status")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	res, err := s.db.Exec(`UPDATE users SET status = ?, updated_at = ? WHERE id = ?`, status, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return err
	}
	return requireOne(res)
}

func (s *Store) CreateSession(input CreateSessionInput) (*Session, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	now := time.Now().UTC()
	session := &Session{
		Token:     strings.TrimSpace(input.Token),
		UserID:    strings.TrimSpace(input.UserID),
		ExpiresAt: input.ExpiresAt,
		CreatedAt: now,
	}
	_, err := s.db.Exec(`
INSERT INTO sessions (token, user_id, expires_at, created_at)
VALUES (?, ?, ?, ?)`,
		session.Token, session.UserID, session.ExpiresAt.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (s *Store) GetSession(token string) (*Session, error) {
	var session Session
	var expiresAt, createdAt string
	if err := s.db.QueryRow(`
SELECT token, user_id, expires_at, created_at
FROM sessions WHERE token = ?`, strings.TrimSpace(token)).Scan(&session.Token, &session.UserID, &expiresAt, &createdAt); err != nil {
		return nil, err
	}
	session.ExpiresAt = parseTime(expiresAt)
	session.CreatedAt = parseTime(createdAt)
	return &session, nil
}

func (s *Store) DeleteSession(token string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	_, err := s.db.Exec(`DELETE FROM sessions WHERE token = ?`, strings.TrimSpace(token))
	return err
}

func (s *Store) AdoptLegacyData(userID string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}
	if _, err := s.db.Exec(`UPDATE model_settings SET id = ?, user_id = ? WHERE id = 'default' AND (user_id IS NULL OR user_id = '')`, userID, userID); err != nil {
		return err
	}
	for _, table := range []string{"products", "jobs", "chat_conversations", "creative_reports", "model_calls"} {
		if _, err := s.db.Exec(`UPDATE `+table+` SET user_id = ? WHERE user_id IS NULL OR user_id = ''`, userID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CreateProduct(input CreateProductInput) (*Product, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	now := time.Now().UTC()
	product := &Product{
		ID:        newID(),
		UserID:    strings.TrimSpace(input.UserID),
		Title:     normalizeTitle(valueOr(input.Title, defaultProductTitle(input.MDName))),
		MDPath:    input.MDPath,
		MDName:    input.MDName,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := s.db.Exec(`
INSERT INTO products (id, user_id, title, md_path, md_name, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
		product.ID, product.UserID, product.Title, product.MDPath, product.MDName,
		now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	return product, nil
}

func defaultProductTitle(fileName string) string {
	title := strings.TrimSuffix(fileName, ".markdown")
	title = strings.TrimSuffix(title, ".md")
	return title
}

func (s *Store) ListProducts(userID string) ([]Product, error) {
	rows, err := s.db.Query(`
SELECT id, user_id, title, md_path, md_name, created_at, updated_at
FROM products
WHERE user_id = ?
ORDER BY updated_at DESC`, strings.TrimSpace(userID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []Product{}
	for rows.Next() {
		product, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *product)
	}
	return result, rows.Err()
}

func (s *Store) GetProduct(userID, id string) (*Product, error) {
	return scanProduct(s.db.QueryRow(`
SELECT id, user_id, title, md_path, md_name, created_at, updated_at
FROM products WHERE user_id = ? AND id = ?`, strings.TrimSpace(userID), id))
}

func (s *Store) CreateCreativeReport(input CreateCreativeReportInput) (*CreativeReport, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	now := time.Now().UTC()
	report := &CreativeReport{
		ID:               newID(),
		UserID:           strings.TrimSpace(input.UserID),
		ProductID:        strings.TrimSpace(input.ProductID),
		ProductTitle:     normalizeTitle(input.ProductTitle),
		SourceConfigJSON: strings.TrimSpace(input.SourceConfigJSON),
		ReportMarkdown:   strings.TrimSpace(input.ReportMarkdown),
		ReportSummary:    strings.TrimSpace(input.ReportSummary),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	_, err := s.db.Exec(`
INSERT INTO creative_reports (
  id, user_id, product_id, product_title, source_config_json, report_markdown, report_summary, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		report.ID, report.UserID, report.ProductID, report.ProductTitle, report.SourceConfigJSON, report.ReportMarkdown,
		report.ReportSummary, now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	return report, nil
}

func (s *Store) ListCreativeReports(userID, productID string) ([]CreativeReport, error) {
	rows, err := s.db.Query(`
SELECT id, user_id, product_id, product_title, source_config_json, report_markdown, report_summary, created_at, updated_at
FROM creative_reports
WHERE user_id = ? AND product_id = ?
ORDER BY created_at DESC`, strings.TrimSpace(userID), productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []CreativeReport{}
	for rows.Next() {
		report, err := scanCreativeReport(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *report)
	}
	return result, rows.Err()
}

func (s *Store) GetCreativeReport(userID, id string) (*CreativeReport, error) {
	return scanCreativeReport(s.db.QueryRow(`
SELECT id, user_id, product_id, product_title, source_config_json, report_markdown, report_summary, created_at, updated_at
FROM creative_reports WHERE user_id = ? AND id = ?`, strings.TrimSpace(userID), id))
}

func (s *Store) ReplaceProductChunks(productID string, chunks []ProductChunkInput) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM product_chunks WHERE product_id = ?`, productID); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, chunk := range chunks {
		embeddingJSON, err := json.Marshal(chunk.Embedding)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`
INSERT INTO product_chunks (
  id, product_id, chunk_index, heading, content, embedding_json, embedding_model, embedding_dim, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			newID(), productID, chunk.ChunkIndex, chunk.Heading, chunk.Content, string(embeddingJSON),
			chunk.EmbeddingModel, chunk.EmbeddingDim, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ListProductChunks(productID string) ([]ProductChunk, error) {
	rows, err := s.db.Query(`
SELECT id, product_id, chunk_index, heading, content, embedding_json, embedding_model, embedding_dim, created_at
FROM product_chunks
WHERE product_id = ?
ORDER BY chunk_index ASC`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []ProductChunk{}
	for rows.Next() {
		chunk, err := scanProductChunk(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *chunk)
	}
	return result, rows.Err()
}

func (s *Store) SaveModelSettings(userID string, settings ModelSettings) (*ModelSettings, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	userID = strings.TrimSpace(userID)
	existing, _ := s.GetModelSettings(userID)
	apiKey := strings.TrimSpace(settings.APIKey)
	if apiKey == "" && existing != nil {
		apiKey = existing.APIKey
	}
	endpoint := strings.TrimSpace(settings.Endpoint)
	if endpoint == "" {
		endpoint = "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"
	}
	modelName := strings.TrimSpace(settings.Model)
	if modelName == "" {
		modelName = "qwen3.6-plus"
	}
	now := time.Now().UTC()
	_, err := s.db.Exec(`
INSERT INTO model_settings (id, user_id, api_key, endpoint, model, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  user_id = excluded.user_id,
  api_key = excluded.api_key,
  endpoint = excluded.endpoint,
  model = excluded.model,
  updated_at = excluded.updated_at`,
		userID, userID, apiKey, endpoint, modelName, now.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	return &ModelSettings{APIKey: apiKey, Endpoint: endpoint, Model: modelName, UpdatedAt: now}, nil
}

func (s *Store) GetModelSettings(userID string) (*ModelSettings, error) {
	settings := &ModelSettings{}
	var updatedAt string
	var apiKey sql.NullString
	err := s.db.QueryRow(`
SELECT api_key, endpoint, model, updated_at
FROM model_settings WHERE id = ?`, strings.TrimSpace(userID)).Scan(&apiKey, &settings.Endpoint, &settings.Model, &updatedAt)
	if err != nil {
		return nil, err
	}
	settings.APIKey = apiKey.String
	settings.UpdatedAt = parseTime(updatedAt)
	return settings, nil
}

func (s *Store) CreateJob(input CreateJobInput) (*Job, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	now := time.Now().UTC()
	job := &Job{
		ID:                newID(),
		UserID:            strings.TrimSpace(input.UserID),
		Title:             input.Title,
		Status:            StatusPending,
		VideoPath:         input.VideoPath,
		VideoOriginalName: input.VideoOriginalName,
		ProductMDPath:     input.ProductMDPath,
		ProductMDName:     input.ProductMDName,
		Requirement:       input.Requirement,
		Industry:          input.Industry,
		FissionCount:      input.FissionCount,
		FissionDirections: input.FissionDirections,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	_, err := s.db.Exec(`
INSERT INTO jobs (
  id, user_id, title, status, video_path, video_original_name, product_md_path, product_md_name,
  requirement, industry, fission_count, fission_directions, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.UserID, job.Title, job.Status, job.VideoPath, job.VideoOriginalName,
		job.ProductMDPath, job.ProductMDName, job.Requirement, job.Industry,
		job.FissionCount, job.FissionDirections, job.CreatedAt.Format(time.RFC3339), job.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	return job, nil
}

func (s *Store) ListJobs(userID string) ([]Job, error) {
	rows, err := s.db.Query(`
SELECT id, user_id, title, status, video_path, video_original_name, product_md_path, product_md_name,
       requirement, industry, fission_count, fission_directions, analysis_markdown, replica_script_json,
       fission_scripts_json, creatibi_result_json, error_message, run_log, created_at, updated_at
FROM jobs WHERE user_id = ? ORDER BY created_at DESC`, strings.TrimSpace(userID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []Job{}
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *job)
	}
	return result, rows.Err()
}

func (s *Store) ListUnfinishedJobs() ([]Job, error) {
	rows, err := s.db.Query(`
SELECT id, user_id, title, status, video_path, video_original_name, product_md_path, product_md_name,
       requirement, industry, fission_count, fission_directions, analysis_markdown, replica_script_json,
       fission_scripts_json, creatibi_result_json, error_message, run_log, created_at, updated_at
FROM jobs
WHERE status IN (?, ?, ?, ?, ?, ?, ?)
ORDER BY created_at ASC`,
		StatusPending, StatusRunning, StatusAnalyzingVideo, StatusExtractingStructure,
		StatusGeneratingReplica, StatusGeneratingFission, StatusValidating)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []Job{}
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *job)
	}
	return result, rows.Err()
}

func (s *Store) GetJob(id string) (*Job, error) {
	row := s.db.QueryRow(`
SELECT id, user_id, title, status, video_path, video_original_name, product_md_path, product_md_name,
       requirement, industry, fission_count, fission_directions, analysis_markdown, replica_script_json,
       fission_scripts_json, creatibi_result_json, error_message, run_log, created_at, updated_at
FROM jobs WHERE id = ?`, id)
	return scanJob(row)
}

func (s *Store) GetUserJob(userID, id string) (*Job, error) {
	row := s.db.QueryRow(`
SELECT id, user_id, title, status, video_path, video_original_name, product_md_path, product_md_name,
       requirement, industry, fission_count, fission_directions, analysis_markdown, replica_script_json,
       fission_scripts_json, creatibi_result_json, error_message, run_log, created_at, updated_at
FROM jobs WHERE user_id = ? AND id = ?`, strings.TrimSpace(userID), id)
	return scanJob(row)
}

func (s *Store) UpdateStatus(id, status, errorMessage string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	res, err := s.db.Exec(`UPDATE jobs SET status = ?, error_message = ?, updated_at = ? WHERE id = ?`,
		status, errorMessage, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return err
	}
	return requireOne(res)
}

func (s *Store) ResetForRetry(id string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	res, err := s.db.Exec(`
UPDATE jobs
SET status = ?, analysis_markdown = '', replica_script_json = '', fission_scripts_json = '',
    creatibi_result_json = '', error_message = '', updated_at = ?
WHERE id = ?`,
		StatusPending, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return err
	}
	return requireOne(res)
}

func (s *Store) AppendLog(id, message string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	line := fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), strings.TrimSpace(message))
	res, err := s.db.Exec(`UPDATE jobs SET run_log = COALESCE(run_log, '') || ?, updated_at = ? WHERE id = ?`,
		line, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return err
	}
	return requireOne(res)
}

func (s *Store) SaveResult(id string, result ScriptResult) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	res, err := s.db.Exec(`
UPDATE jobs
SET status = ?, analysis_markdown = ?, replica_script_json = ?, fission_scripts_json = ?,
    error_message = '', updated_at = ?
WHERE id = ?`,
		StatusCompleted, result.AnalysisMarkdown, result.ReplicaScriptJSON,
		result.FissionScriptsJSON, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return err
	}
	return requireOne(res)
}

func (s *Store) SavePublishResult(id, status, resultJSON, errorMessage string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	res, err := s.db.Exec(`
UPDATE jobs
SET status = ?, creatibi_result_json = ?, error_message = ?, updated_at = ?
WHERE id = ?`,
		status, resultJSON, errorMessage, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return err
	}
	return requireOne(res)
}

func (s *Store) CreateChatConversation(userID, title string) (*ChatConversation, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	now := time.Now().UTC()
	conversation := &ChatConversation{
		ID:        newID(),
		UserID:    strings.TrimSpace(userID),
		Title:     normalizeTitle(title),
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := s.db.Exec(`
INSERT INTO chat_conversations (id, user_id, title, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)`,
		conversation.ID, conversation.UserID, conversation.Title, now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	return conversation, nil
}

func (s *Store) ListChatConversations(userID string) ([]ChatConversation, error) {
	rows, err := s.db.Query(`
SELECT id, user_id, title, summary, summary_message_id, created_at, updated_at
FROM chat_conversations
WHERE user_id = ?
ORDER BY updated_at DESC`, strings.TrimSpace(userID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []ChatConversation{}
	for rows.Next() {
		conversation, err := scanChatConversation(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *conversation)
	}
	return result, rows.Err()
}

func (s *Store) GetChatThread(userID, id string) (*ChatThread, error) {
	conversation, err := scanChatConversation(s.db.QueryRow(`
SELECT id, user_id, title, summary, summary_message_id, created_at, updated_at
FROM chat_conversations WHERE user_id = ? AND id = ?`, strings.TrimSpace(userID), id))
	if err != nil {
		return nil, err
	}
	messages, err := s.ListChatMessages(id)
	if err != nil {
		return nil, err
	}
	return &ChatThread{Conversation: *conversation, Messages: messages}, nil
}

func (s *Store) ListChatMessages(conversationID string) ([]ChatMessage, error) {
	rows, err := s.db.Query(`
SELECT id, conversation_id, role, content, created_at
FROM chat_messages
WHERE conversation_id = ?
ORDER BY created_at ASC`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []ChatMessage{}
	for rows.Next() {
		message, err := scanChatMessage(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *message)
	}
	return result, rows.Err()
}

func (s *Store) SaveChatSummary(conversationID, summary, summaryMessageID string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	res, err := s.db.Exec(`
UPDATE chat_conversations
SET summary = ?, summary_message_id = ?
WHERE id = ?`,
		summary, summaryMessageID, conversationID)
	if err != nil {
		return err
	}
	return requireOne(res)
}

func (s *Store) AddChatMessage(conversationID, role, content string) (*ChatMessage, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	now := time.Now().UTC()
	message := &ChatMessage{
		ID:             newID(),
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
		CreatedAt:      now,
	}
	res, err := s.db.Exec(`
INSERT INTO chat_messages (id, conversation_id, role, content, created_at)
VALUES (?, ?, ?, ?, ?)`,
		message.ID, message.ConversationID, message.Role, message.Content, now.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	if err := requireOne(res); err != nil {
		return nil, err
	}
	if _, err := s.db.Exec(`UPDATE chat_conversations SET updated_at = ? WHERE id = ?`, now.Format(time.RFC3339), conversationID); err != nil {
		return nil, err
	}
	return message, nil
}

func (s *Store) RecordModelCall(ctx context.Context, record model.CallRecord) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	now := time.Now().UTC()
	userID := userctx.UserID(ctx)
	_, err := s.db.Exec(`
INSERT INTO model_calls (
  id, user_id, scope, ref_id, step, model, input_json, output_text, response_json,
  prompt_tokens, output_tokens, total_tokens, latency_ms, error_message, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		newID(), userID, valueOr(record.Scope, "unknown"), record.RefID, record.Step, record.Model,
		record.InputJSON, record.OutputText, record.ResponseJSON,
		record.PromptTokens, record.OutputTokens, record.TotalTokens, record.LatencyMS,
		record.ErrorMessage, now.Format(time.RFC3339),
	)
	return err
}

func (s *Store) ListModelCalls(userID, refID string, limit int) ([]ModelCall, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	query := `
SELECT id, user_id, scope, ref_id, step, model, input_json, output_text, response_json,
       prompt_tokens, output_tokens, total_tokens, latency_ms, error_message, created_at
FROM model_calls WHERE user_id = ?`
	args := []any{strings.TrimSpace(userID)}
	if strings.TrimSpace(refID) != "" {
		query += ` AND ref_id = ?`
		args = append(args, refID)
	}
	query += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []ModelCall{}
	for rows.Next() {
		call, err := scanModelCall(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *call)
	}
	return result, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanJob(row scanner) (*Job, error) {
	var job Job
	var createdAt, updatedAt string
	var analysisMarkdown, replicaScriptJSON, fissionScriptsJSON sql.NullString
	var fissionDirections, creatibiResultJSON, errorMessage, runLog sql.NullString
	err := row.Scan(
		&job.ID, &job.UserID, &job.Title, &job.Status, &job.VideoPath, &job.VideoOriginalName,
		&job.ProductMDPath, &job.ProductMDName, &job.Requirement, &job.Industry,
		&job.FissionCount, &fissionDirections, &analysisMarkdown, &replicaScriptJSON,
		&fissionScriptsJSON, &creatibiResultJSON, &errorMessage, &runLog,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	job.AnalysisMarkdown = analysisMarkdown.String
	job.FissionDirections = fissionDirections.String
	job.ReplicaScriptJSON = replicaScriptJSON.String
	job.FissionScriptsJSON = fissionScriptsJSON.String
	job.CreatiBIResultJSON = creatibiResultJSON.String
	job.ErrorMessage = errorMessage.String
	job.RunLog = runLog.String
	job.CreatedAt = parseTime(createdAt)
	job.UpdatedAt = parseTime(updatedAt)
	return &job, nil
}

func scanChatConversation(row scanner) (*ChatConversation, error) {
	var conversation ChatConversation
	var createdAt, updatedAt string
	var summary, summaryMessageID sql.NullString
	if err := row.Scan(&conversation.ID, &conversation.UserID, &conversation.Title, &summary, &summaryMessageID, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	conversation.Summary = summary.String
	conversation.SummaryMessageID = summaryMessageID.String
	conversation.CreatedAt = parseTime(createdAt)
	conversation.UpdatedAt = parseTime(updatedAt)
	return &conversation, nil
}

func scanProduct(row scanner) (*Product, error) {
	var product Product
	var createdAt, updatedAt string
	if err := row.Scan(&product.ID, &product.UserID, &product.Title, &product.MDPath, &product.MDName, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	product.CreatedAt = parseTime(createdAt)
	product.UpdatedAt = parseTime(updatedAt)
	return &product, nil
}

func scanCreativeReport(row scanner) (*CreativeReport, error) {
	var report CreativeReport
	var createdAt, updatedAt string
	if err := row.Scan(
		&report.ID,
		&report.UserID,
		&report.ProductID,
		&report.ProductTitle,
		&report.SourceConfigJSON,
		&report.ReportMarkdown,
		&report.ReportSummary,
		&createdAt,
		&updatedAt,
	); err != nil {
		return nil, err
	}
	report.CreatedAt = parseTime(createdAt)
	report.UpdatedAt = parseTime(updatedAt)
	return &report, nil
}

func scanUser(row scanner) (*User, error) {
	var user User
	var createdAt, updatedAt string
	var name sql.NullString
	if err := row.Scan(&user.ID, &user.Email, &name, &user.Role, &user.Status, &user.PasswordHash, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	user.Name = name.String
	user.CreatedAt = parseTime(createdAt)
	user.UpdatedAt = parseTime(updatedAt)
	return &user, nil
}

func scanProductChunk(row scanner) (*ProductChunk, error) {
	var chunk ProductChunk
	var embeddingJSON, createdAt string
	if err := row.Scan(
		&chunk.ID, &chunk.ProductID, &chunk.ChunkIndex, &chunk.Heading, &chunk.Content,
		&embeddingJSON, &chunk.EmbeddingModel, &chunk.EmbeddingDim, &createdAt,
	); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(embeddingJSON), &chunk.Embedding); err != nil {
		return nil, err
	}
	chunk.CreatedAt = parseTime(createdAt)
	return &chunk, nil
}

func scanChatMessage(row scanner) (*ChatMessage, error) {
	var message ChatMessage
	var createdAt string
	if err := row.Scan(&message.ID, &message.ConversationID, &message.Role, &message.Content, &createdAt); err != nil {
		return nil, err
	}
	message.CreatedAt = parseTime(createdAt)
	return &message, nil
}

func scanModelCall(row scanner) (*ModelCall, error) {
	var call ModelCall
	var createdAt string
	var inputJSON, outputText, responseJSON, errorMessage sql.NullString
	if err := row.Scan(
		&call.ID, &call.UserID, &call.Scope, &call.RefID, &call.Step, &call.Model,
		&inputJSON, &outputText, &responseJSON,
		&call.PromptTokens, &call.OutputTokens, &call.TotalTokens, &call.LatencyMS,
		&errorMessage, &createdAt,
	); err != nil {
		return nil, err
	}
	call.InputJSON = inputJSON.String
	call.OutputText = outputText.String
	call.ResponseJSON = responseJSON.String
	call.ErrorMessage = errorMessage.String
	call.CreatedAt = parseTime(createdAt)
	return &call, nil
}

func (s *Store) ensureColumn(table, column, columnType string) error {
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` ` + columnType)
	return err
}

func parseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func requireOne(res sql.Result) error {
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.New("job not found")
	}
	return nil
}

func normalizeTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "新对话"
	}
	if len([]rune(title)) > 32 {
		return string([]rune(title)[:32])
	}
	return title
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
