package jobs

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func OpenStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
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
	return s.ensureColumn("jobs", "run_log", "TEXT")
}

func (s *Store) CreateJob(input CreateJobInput) (*Job, error) {
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
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	_, err := s.db.Exec(`
INSERT INTO jobs (
  id, title, status, video_path, video_original_name, product_md_path, product_md_name,
  requirement, industry, fission_count, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.Title, job.Status, job.VideoPath, job.VideoOriginalName,
		job.ProductMDPath, job.ProductMDName, job.Requirement, job.Industry,
		job.FissionCount, job.CreatedAt.Format(time.RFC3339), job.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	return job, nil
}

func (s *Store) ListJobs() ([]Job, error) {
	rows, err := s.db.Query(`
SELECT id, title, status, video_path, video_original_name, product_md_path, product_md_name,
       requirement, industry, fission_count, analysis_markdown, replica_script_json,
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
       requirement, industry, fission_count, analysis_markdown, replica_script_json,
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
       requirement, industry, fission_count, analysis_markdown, replica_script_json,
       fission_scripts_json, creatibi_result_json, error_message, run_log, created_at, updated_at
FROM jobs WHERE id = ?`, id)
	return scanJob(row)
}

func (s *Store) UpdateStatus(id, status, errorMessage string) error {
	res, err := s.db.Exec(`UPDATE jobs SET status = ?, error_message = ?, updated_at = ? WHERE id = ?`,
		status, errorMessage, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return err
	}
	return requireOne(res)
}

func (s *Store) AppendLog(id, message string) error {
	line := fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), strings.TrimSpace(message))
	res, err := s.db.Exec(`UPDATE jobs SET run_log = COALESCE(run_log, '') || ?, updated_at = ? WHERE id = ?`,
		line, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return err
	}
	return requireOne(res)
}

func (s *Store) SaveResult(id string, result ScriptResult) error {
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

type scanner interface {
	Scan(dest ...any) error
}

func scanJob(row scanner) (*Job, error) {
	var job Job
	var createdAt, updatedAt string
	var analysisMarkdown, replicaScriptJSON, fissionScriptsJSON sql.NullString
	var creatibiResultJSON, errorMessage, runLog sql.NullString
	err := row.Scan(
		&job.ID, &job.Title, &job.Status, &job.VideoPath, &job.VideoOriginalName,
		&job.ProductMDPath, &job.ProductMDName, &job.Requirement, &job.Industry,
		&job.FissionCount, &analysisMarkdown, &replicaScriptJSON,
		&fissionScriptsJSON, &creatibiResultJSON, &errorMessage, &runLog,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	job.AnalysisMarkdown = analysisMarkdown.String
	job.ReplicaScriptJSON = replicaScriptJSON.String
	job.FissionScriptsJSON = fissionScriptsJSON.String
	job.CreatiBIResultJSON = creatibiResultJSON.String
	job.ErrorMessage = errorMessage.String
	job.RunLog = runLog.String
	job.CreatedAt = parseTime(createdAt)
	job.UpdatedAt = parseTime(updatedAt)
	return &job, nil
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
