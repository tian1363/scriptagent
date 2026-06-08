package jobs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/tian1363/scriptagent/internal/model"
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
	_, err = s.db.Exec(`
CREATE TABLE IF NOT EXISTS chat_conversations (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
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
`)
	return err
}

func (s *Store) CreateJob(input CreateJobInput) (*Job, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	now := time.Now().UTC()
	job := &Job{
		ID:                newID(),
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
  id, title, status, video_path, video_original_name, product_md_path, product_md_name,
  requirement, industry, fission_count, fission_directions, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.Title, job.Status, job.VideoPath, job.VideoOriginalName,
		job.ProductMDPath, job.ProductMDName, job.Requirement, job.Industry,
		job.FissionCount, job.FissionDirections, job.CreatedAt.Format(time.RFC3339), job.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	return job, nil
}

func (s *Store) ListJobs() ([]Job, error) {
	rows, err := s.db.Query(`
SELECT id, title, status, video_path, video_original_name, product_md_path, product_md_name,
       requirement, industry, fission_count, fission_directions, analysis_markdown, replica_script_json,
       fission_scripts_json, creatibi_result_json, error_message, run_log, created_at, updated_at
FROM jobs ORDER BY created_at DESC`)
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
SELECT id, title, status, video_path, video_original_name, product_md_path, product_md_name,
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
SELECT id, title, status, video_path, video_original_name, product_md_path, product_md_name,
       requirement, industry, fission_count, fission_directions, analysis_markdown, replica_script_json,
       fission_scripts_json, creatibi_result_json, error_message, run_log, created_at, updated_at
FROM jobs WHERE id = ?`, id)
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

func (s *Store) CreateChatConversation(title string) (*ChatConversation, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	now := time.Now().UTC()
	conversation := &ChatConversation{
		ID:        newID(),
		Title:     normalizeTitle(title),
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := s.db.Exec(`
INSERT INTO chat_conversations (id, title, created_at, updated_at)
VALUES (?, ?, ?, ?)`,
		conversation.ID, conversation.Title, now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	return conversation, nil
}

func (s *Store) ListChatConversations() ([]ChatConversation, error) {
	rows, err := s.db.Query(`
SELECT id, title, created_at, updated_at
FROM chat_conversations
ORDER BY updated_at DESC`)
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

func (s *Store) GetChatThread(id string) (*ChatThread, error) {
	conversation, err := scanChatConversation(s.db.QueryRow(`
SELECT id, title, created_at, updated_at
FROM chat_conversations WHERE id = ?`, id))
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

func (s *Store) RecordModelCall(_ context.Context, record model.CallRecord) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	now := time.Now().UTC()
	_, err := s.db.Exec(`
INSERT INTO model_calls (
  id, scope, ref_id, step, model, input_json, output_text, response_json,
  prompt_tokens, output_tokens, total_tokens, latency_ms, error_message, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		newID(), valueOr(record.Scope, "unknown"), record.RefID, record.Step, record.Model,
		record.InputJSON, record.OutputText, record.ResponseJSON,
		record.PromptTokens, record.OutputTokens, record.TotalTokens, record.LatencyMS,
		record.ErrorMessage, now.Format(time.RFC3339),
	)
	return err
}

func (s *Store) ListModelCalls(refID string, limit int) ([]ModelCall, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	query := `
SELECT id, scope, ref_id, step, model, input_json, output_text, response_json,
       prompt_tokens, output_tokens, total_tokens, latency_ms, error_message, created_at
FROM model_calls`
	args := []any{}
	if strings.TrimSpace(refID) != "" {
		query += ` WHERE ref_id = ?`
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
		&job.ID, &job.Title, &job.Status, &job.VideoPath, &job.VideoOriginalName,
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
	if err := row.Scan(&conversation.ID, &conversation.Title, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	conversation.CreatedAt = parseTime(createdAt)
	conversation.UpdatedAt = parseTime(updatedAt)
	return &conversation, nil
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
		&call.ID, &call.Scope, &call.RefID, &call.Step, &call.Model,
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
