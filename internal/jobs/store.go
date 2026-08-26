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
	if err := s.ensureColumn("jobs", "space_id", "TEXT"); err != nil {
		return err
	}
	if err := s.ensureColumn("jobs", "parent_job_id", "TEXT"); err != nil {
		return err
	}
	if err := s.ensureColumn("jobs", "context_snapshot", "TEXT"); err != nil {
		return err
	}
	_, err = s.db.Exec(`
CREATE TABLE IF NOT EXISTS chat_conversations (
  id TEXT PRIMARY KEY,
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
  title TEXT NOT NULL,
  md_path TEXT NOT NULL,
  md_name TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS creative_reports (
  id TEXT PRIMARY KEY,
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
  api_key TEXT,
  endpoint TEXT NOT NULL,
  model TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS spaces (
  id TEXT PRIMARY KEY, title TEXT NOT NULL, summary TEXT, product_id TEXT,
  agent_brief TEXT, status TEXT NOT NULL, origin_space_id TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
  FOREIGN KEY(product_id) REFERENCES products(id) ON DELETE RESTRICT
);
CREATE TABLE IF NOT EXISTS product_assets (
  id TEXT PRIMARY KEY, product_id TEXT NOT NULL, kind TEXT NOT NULL, path TEXT NOT NULL,
  original_name TEXT NOT NULL, mime_type TEXT NOT NULL, size_bytes INTEGER NOT NULL, created_at TEXT NOT NULL,
  FOREIGN KEY(product_id) REFERENCES products(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS agent_runs (
  id TEXT PRIMARY KEY, space_id TEXT NOT NULL, job_id TEXT, status TEXT NOT NULL,
  started_at TEXT NOT NULL, finished_at TEXT, error_message TEXT,
  FOREIGN KEY(space_id) REFERENCES spaces(id) ON DELETE CASCADE,
  FOREIGN KEY(job_id) REFERENCES jobs(id) ON DELETE SET NULL
);
CREATE TABLE IF NOT EXISTS agent_steps (
  id TEXT PRIMARY KEY, run_id TEXT NOT NULL, step_index INTEGER NOT NULL, step_key TEXT NOT NULL,
  kind TEXT NOT NULL, status TEXT NOT NULL, input_summary TEXT, output_summary TEXT,
  error_message TEXT, started_at TEXT NOT NULL, finished_at TEXT,
  FOREIGN KEY(run_id) REFERENCES agent_runs(id) ON DELETE CASCADE,
  UNIQUE(run_id, step_index)
);
CREATE TABLE IF NOT EXISTS memory_events (
  id TEXT PRIMARY KEY, space_id TEXT NOT NULL, run_id TEXT NOT NULL, kind TEXT NOT NULL,
  payload_json TEXT, created_at TEXT NOT NULL,
  FOREIGN KEY(space_id) REFERENCES spaces(id) ON DELETE CASCADE,
  FOREIGN KEY(run_id) REFERENCES agent_runs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_products_updated ON products(updated_at);
CREATE INDEX IF NOT EXISTS idx_creative_reports_product ON creative_reports(product_id, created_at);
CREATE INDEX IF NOT EXISTS idx_product_chunks_product ON product_chunks(product_id, chunk_index);
CREATE INDEX IF NOT EXISTS idx_spaces_updated ON spaces(updated_at);
CREATE INDEX IF NOT EXISTS idx_jobs_space ON jobs(space_id, created_at);
CREATE INDEX IF NOT EXISTS idx_product_assets_product ON product_assets(product_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_runs_space ON agent_runs(space_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_steps_run ON agent_steps(run_id, step_index);
CREATE INDEX IF NOT EXISTS idx_memory_events_space_run ON memory_events(space_id, run_id, created_at);
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
	if err := s.ensureColumn("model_settings", "provider", "TEXT NOT NULL DEFAULT 'dashscope'"); err != nil {
		return err
	}
	if err := s.ensureColumn("model_calls", "space_id", "TEXT"); err != nil {
		return err
	}
	if err := s.ensureColumn("model_calls", "run_id", "TEXT"); err != nil {
		return err
	}
	if _, err := s.db.Exec(`
CREATE INDEX IF NOT EXISTS idx_model_calls_space ON model_calls(space_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_model_calls_run ON model_calls(run_id, created_at);
UPDATE model_calls
SET space_id = (SELECT jobs.space_id FROM jobs WHERE jobs.id = model_calls.ref_id)
WHERE COALESCE(space_id, '') = '' AND scope = 'job';`); err != nil {
		return err
	}
	if err := s.makeSpaceProductOptional(); err != nil {
		return err
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_spaces_updated ON spaces(updated_at)`); err != nil {
		return err
	}
	return err
}

func (s *Store) makeSpaceProductOptional() error {
	rows, err := s.db.Query(`PRAGMA table_info(spaces)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	mandatory := false
	for rows.Next() {
		var cid, notNull, pk int
		var name, typ string
		var fallback any
		if err := rows.Scan(&cid, &name, &typ, &notNull, &fallback, &pk); err != nil {
			return err
		}
		if name == "product_id" {
			mandatory = notNull == 1
		}
	}
	if !mandatory {
		return rows.Err()
	}
	if _, err := s.db.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		return err
	}
	defer s.db.Exec(`PRAGMA foreign_keys = ON`)
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`CREATE TABLE spaces_next (id TEXT PRIMARY KEY, title TEXT NOT NULL, summary TEXT, product_id TEXT, agent_brief TEXT, status TEXT NOT NULL, origin_space_id TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, FOREIGN KEY(product_id) REFERENCES products(id) ON DELETE RESTRICT); INSERT INTO spaces_next SELECT id,title,summary,product_id,agent_brief,status,origin_space_id,created_at,updated_at FROM spaces; DROP TABLE spaces; ALTER TABLE spaces_next RENAME TO spaces;`); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CreateProductAsset(asset ProductAsset) (*ProductAsset, error) {
	asset.ID, asset.CreatedAt = newID(), time.Now().UTC()
	_, err := s.db.Exec(`INSERT INTO product_assets (id,product_id,kind,path,original_name,mime_type,size_bytes,created_at) VALUES (?,?,?,?,?,?,?,?)`, asset.ID, asset.ProductID, asset.Kind, asset.Path, asset.OriginalName, asset.MimeType, asset.SizeBytes, asset.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	return &asset, nil
}

func (s *Store) ListProductAssets(productID string) ([]ProductAsset, error) {
	rows, err := s.db.Query(`SELECT id,product_id,kind,path,original_name,mime_type,size_bytes,created_at FROM product_assets WHERE product_id=? ORDER BY created_at DESC`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ProductAsset{}
	for rows.Next() {
		var item ProductAsset
		var created string
		if err := rows.Scan(&item.ID, &item.ProductID, &item.Kind, &item.Path, &item.OriginalName, &item.MimeType, &item.SizeBytes, &created); err != nil {
			return nil, err
		}
		item.CreatedAt = parseTime(created)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetProductAsset(id string) (*ProductAsset, error) {
	var item ProductAsset
	var created string
	err := s.db.QueryRow(`SELECT id,product_id,kind,path,original_name,mime_type,size_bytes,created_at FROM product_assets WHERE id=?`, id).Scan(&item.ID, &item.ProductID, &item.Kind, &item.Path, &item.OriginalName, &item.MimeType, &item.SizeBytes, &created)
	if err != nil {
		return nil, err
	}
	item.CreatedAt = parseTime(created)
	return &item, nil
}

func (s *Store) CreateProduct(input CreateProductInput) (*Product, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	now := time.Now().UTC()
	product := &Product{
		ID:        newID(),
		Title:     normalizeTitle(valueOr(input.Title, defaultProductTitle(input.MDName))),
		MDPath:    input.MDPath,
		MDName:    input.MDName,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := s.db.Exec(`
INSERT INTO products (id, title, md_path, md_name, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)`,
		product.ID, product.Title, product.MDPath, product.MDName,
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

func (s *Store) ListProducts() ([]Product, error) {
	rows, err := s.db.Query(`
SELECT id, title, md_path, md_name, created_at, updated_at
FROM products
ORDER BY updated_at DESC`)
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

func (s *Store) GetProduct(id string) (*Product, error) {
	return scanProduct(s.db.QueryRow(`
SELECT id, title, md_path, md_name, created_at, updated_at
FROM products WHERE id = ?`, id))
}

func (s *Store) UpdateProduct(id string, input UpdateProductInput) (*Product, error) {
	product, err := s.GetProduct(id)
	if err != nil {
		return nil, err
	}
	title := normalizeTitle(valueOr(input.Title, product.Title))
	now := time.Now().UTC()
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`UPDATE products SET title=?, updated_at=? WHERE id=?`, title, now.Format(time.RFC3339), id); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`DELETE FROM product_chunks WHERE product_id=?`, id); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	product.Title, product.UpdatedAt = title, now
	return product, nil
}

func (s *Store) CreateSpace(input CreateSpaceInput) (*Space, error) {
	productID := strings.TrimSpace(input.ProductID)
	if productID != "" {
		if _, err := s.GetProduct(productID); err != nil {
			return nil, fmt.Errorf("product not found: %w", err)
		}
	}
	now := time.Now().UTC()
	space := &Space{ID: newID(), Title: normalizeTitle(input.Title), Summary: strings.TrimSpace(input.Summary), ProductID: productID, AgentBrief: strings.TrimSpace(input.AgentBrief), Status: SpaceStatusActive, OriginSpaceID: strings.TrimSpace(input.OriginSpaceID), CreatedAt: now, UpdatedAt: now}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`INSERT INTO spaces (id,title,summary,product_id,agent_brief,status,origin_space_id,created_at,updated_at) VALUES (?,?,?,NULLIF(?,''),?,?,NULLIF(?,''),?,?)`, space.ID, space.Title, space.Summary, space.ProductID, space.AgentBrief, space.Status, space.OriginSpaceID, now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	return space, nil
}

func (s *Store) ListSpaces() ([]Space, error) {
	rows, err := s.db.Query(`SELECT id,title,COALESCE(summary,''),COALESCE(product_id,''),COALESCE(agent_brief,''),status,COALESCE(origin_space_id,''),created_at,updated_at FROM spaces ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Space{}
	for rows.Next() {
		var x Space
		var c, u string
		if err := rows.Scan(&x.ID, &x.Title, &x.Summary, &x.ProductID, &x.AgentBrief, &x.Status, &x.OriginSpaceID, &c, &u); err != nil {
			return nil, err
		}
		x.CreatedAt = parseTime(c)
		x.UpdatedAt = parseTime(u)
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Store) GetSpace(id string) (*Space, error) {
	var space Space
	var createdAt, updatedAt string
	err := s.db.QueryRow(`SELECT id,title,COALESCE(summary,''),COALESCE(product_id,''),COALESCE(agent_brief,''),status,COALESCE(origin_space_id,''),created_at,updated_at FROM spaces WHERE id=?`, id).Scan(&space.ID, &space.Title, &space.Summary, &space.ProductID, &space.AgentBrief, &space.Status, &space.OriginSpaceID, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	space.CreatedAt = parseTime(createdAt)
	space.UpdatedAt = parseTime(updatedAt)
	return &space, nil
}

func (s *Store) CreateCreativeReport(input CreateCreativeReportInput) (*CreativeReport, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	now := time.Now().UTC()
	report := &CreativeReport{
		ID:               newID(),
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
  id, product_id, product_title, source_config_json, report_markdown, report_summary, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		report.ID, report.ProductID, report.ProductTitle, report.SourceConfigJSON, report.ReportMarkdown,
		report.ReportSummary, now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	return report, nil
}

func (s *Store) ListCreativeReports(productID string) ([]CreativeReport, error) {
	rows, err := s.db.Query(`
SELECT id, product_id, product_title, source_config_json, report_markdown, report_summary, created_at, updated_at
FROM creative_reports
WHERE product_id = ?
ORDER BY created_at DESC`, productID)
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

func (s *Store) GetCreativeReport(id string) (*CreativeReport, error) {
	return scanCreativeReport(s.db.QueryRow(`
SELECT id, product_id, product_title, source_config_json, report_markdown, report_summary, created_at, updated_at
FROM creative_reports WHERE id = ?`, id))
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

func (s *Store) SaveModelSettings(settings ModelSettings) (*ModelSettings, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	existing, _ := s.GetModelSettings()
	apiKey := strings.TrimSpace(settings.APIKey)
	if apiKey == "" && existing != nil {
		apiKey = existing.APIKey
	}
	provider := strings.TrimSpace(settings.Provider)
	if provider == "" {
		provider = "dashscope"
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
INSERT INTO model_settings (id, api_key, endpoint, model, provider, updated_at)
VALUES ('default', ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  api_key = excluded.api_key,
  endpoint = excluded.endpoint,
  model = excluded.model,
  provider = excluded.provider,
  updated_at = excluded.updated_at`,
		apiKey, endpoint, modelName, provider, now.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	return &ModelSettings{APIKey: apiKey, Provider: provider, Endpoint: endpoint, Model: modelName, UpdatedAt: now}, nil
}

func (s *Store) GetModelSettings() (*ModelSettings, error) {
	settings := &ModelSettings{}
	var updatedAt string
	var apiKey sql.NullString
	err := s.db.QueryRow(`
SELECT api_key, endpoint, model, provider, updated_at
FROM model_settings WHERE id = 'default'`).Scan(&apiKey, &settings.Endpoint, &settings.Model, &settings.Provider, &updatedAt)
	if err != nil {
		return nil, err
	}
	settings.APIKey = apiKey.String
	settings.UpdatedAt = parseTime(updatedAt)
	return settings, nil
}

func (s *Store) GetModelRuntimeConfig() (model.RuntimeConfig, error) {
	settings, err := s.GetModelSettings()
	if err != nil {
		return model.RuntimeConfig{}, err
	}
	return model.RuntimeConfig{
		APIKey:   settings.APIKey,
		Provider: settings.Provider,
		Endpoint: settings.Endpoint,
		Model:    settings.Model,
		Source:   "user",
	}, nil
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
		SpaceID:           input.SpaceID,
		ParentJobID:       input.ParentJobID,
		ContextSnapshot:   input.ContextSnapshot,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	_, err := s.db.Exec(`
INSERT INTO jobs (
  id, title, status, video_path, video_original_name, product_md_path, product_md_name,
  requirement, industry, fission_count, fission_directions, space_id, parent_job_id, context_snapshot, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.Title, job.Status, job.VideoPath, job.VideoOriginalName,
		job.ProductMDPath, job.ProductMDName, job.Requirement, job.Industry,
		job.FissionCount, job.FissionDirections, job.SpaceID, job.ParentJobID, job.ContextSnapshot, job.CreatedAt.Format(time.RFC3339), job.UpdatedAt.Format(time.RFC3339),
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
       fission_scripts_json, creatibi_result_json, error_message, run_log, space_id, parent_job_id, context_snapshot, created_at, updated_at
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
       fission_scripts_json, creatibi_result_json, error_message, run_log, space_id, parent_job_id, context_snapshot, created_at, updated_at
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
       fission_scripts_json, creatibi_result_json, error_message, run_log, space_id, parent_job_id, context_snapshot, created_at, updated_at
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
SELECT id, title, summary, summary_message_id, created_at, updated_at
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
SELECT id, title, summary, summary_message_id, created_at, updated_at
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

func (s *Store) RecordModelCall(_ context.Context, record model.CallRecord) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	now := time.Now().UTC()
	_, err := s.db.Exec(`
INSERT INTO model_calls (
  id, scope, ref_id, space_id, run_id, step, model, input_json, output_text, response_json,
  prompt_tokens, output_tokens, total_tokens, latency_ms, error_message, created_at
) VALUES (?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		newID(), valueOr(record.Scope, "unknown"), record.RefID, record.SpaceID, record.RunID, record.Step, record.Model,
		record.InputJSON, record.OutputText, record.ResponseJSON,
		record.PromptTokens, record.OutputTokens, record.TotalTokens, record.LatencyMS,
		record.ErrorMessage, now.Format(time.RFC3339),
	)
	return err
}

func (s *Store) ListModelCalls(refID, spaceID string, limit int) ([]ModelCall, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	query := `
SELECT id, scope, ref_id, COALESCE(space_id, ''), COALESCE(run_id, ''), step, model, input_json, output_text, response_json,
       prompt_tokens, output_tokens, total_tokens, latency_ms, error_message, created_at
FROM model_calls`
	args := []any{}
	conditions := []string{}
	if strings.TrimSpace(refID) != "" {
		conditions = append(conditions, `ref_id = ?`)
		args = append(args, refID)
	}
	if strings.TrimSpace(spaceID) != "" {
		conditions = append(conditions, `space_id = ?`)
		args = append(args, spaceID)
	}
	if len(conditions) > 0 {
		query += ` WHERE ` + strings.Join(conditions, ` AND `)
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

func (s *Store) StartAgentRun(job Job) (*AgentRun, error) {
	if strings.TrimSpace(job.SpaceID) == "" {
		return nil, errors.New("agent run requires a space")
	}
	now := time.Now().UTC()
	run := &AgentRun{ID: newID(), SpaceID: job.SpaceID, JobID: job.ID, Status: "running", StartedAt: now}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var active int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM agent_runs WHERE job_id=? AND status='running'`, job.ID).Scan(&active); err != nil {
		return nil, err
	}
	if active > 0 {
		return nil, errors.New("agent run is already active for job")
	}
	if _, err := tx.Exec(`INSERT INTO agent_runs (id,space_id,job_id,status,started_at) VALUES (?,?,?,?,?)`, run.ID, run.SpaceID, run.JobID, run.Status, now.Format(time.RFC3339)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return run, nil
}

func (s *Store) FailActiveAgentRuns(jobID, reason string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`UPDATE agent_runs SET status='failed', finished_at=?, error_message=? WHERE job_id=? AND status='running'`, time.Now().UTC().Format(time.RFC3339), strings.TrimSpace(reason), jobID)
	return err
}

func (s *Store) FinishAgentRun(runID, status, errorMessage string) error {
	now := time.Now().UTC()
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`UPDATE agent_runs SET status=?, finished_at=?, error_message=NULLIF(?,'') WHERE id=?`, status, now.Format(time.RFC3339), errorMessage, runID)
	return err
}

func (s *Store) StartAgentStep(runID string, index int, key, kind, inputSummary string) (*AgentRunStep, error) {
	if strings.TrimSpace(runID) == "" || index <= 0 || strings.TrimSpace(key) == "" {
		return nil, errors.New("agent step requires run, positive index, and key")
	}
	if strings.TrimSpace(kind) == "" {
		kind = "workflow"
	}
	now := time.Now().UTC()
	step := &AgentRunStep{ID: newID(), RunID: runID, Index: index, Key: strings.TrimSpace(key), Kind: strings.TrimSpace(kind), Status: "running", InputSummary: strings.TrimSpace(inputSummary), StartedAt: now}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`INSERT INTO agent_steps (id,run_id,step_index,step_key,kind,status,input_summary,started_at) VALUES (?,?,?,?,?,?,NULLIF(?,''),?)`, step.ID, step.RunID, step.Index, step.Key, step.Kind, step.Status, step.InputSummary, now.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	return step, nil
}

func (s *Store) FinishAgentStep(stepID, status, outputSummary, errorMessage string) error {
	if strings.TrimSpace(status) == "" {
		status = "completed"
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	res, err := s.db.Exec(`UPDATE agent_steps SET status=?, output_summary=NULLIF(?,''), error_message=NULLIF(?,''), finished_at=? WHERE id=? AND status='running'`, status, strings.TrimSpace(outputSummary), strings.TrimSpace(errorMessage), time.Now().UTC().Format(time.RFC3339), stepID)
	if err != nil {
		return err
	}
	return requireOne(res)
}

func (s *Store) ListAgentRunSteps(spaceID, runID string, limit int) ([]AgentRunStep, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	query := `SELECT s.id,s.run_id,s.step_index,s.step_key,s.kind,s.status,COALESCE(s.input_summary,''),COALESCE(s.output_summary,''),COALESCE(s.error_message,''),s.started_at,s.finished_at FROM agent_steps s JOIN agent_runs r ON r.id=s.run_id`
	conditions := []string{}
	args := []any{}
	if strings.TrimSpace(spaceID) != "" {
		conditions = append(conditions, "r.space_id=?")
		args = append(args, spaceID)
	}
	if strings.TrimSpace(runID) != "" {
		conditions = append(conditions, "s.run_id=?")
		args = append(args, runID)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY s.started_at DESC, s.step_index DESC LIMIT ?"
	args = append(args, limit)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []AgentRunStep{}
	for rows.Next() {
		var step AgentRunStep
		var started string
		var finished sql.NullString
		if err := rows.Scan(&step.ID, &step.RunID, &step.Index, &step.Key, &step.Kind, &step.Status, &step.InputSummary, &step.OutputSummary, &step.Error, &started, &finished); err != nil {
			return nil, err
		}
		step.StartedAt = parseTime(started)
		if finished.Valid {
			value := parseTime(finished.String)
			step.FinishedAt = &value
		}
		result = append(result, step)
	}
	return result, rows.Err()
}

func (s *Store) ListAgentRuns(spaceID string, limit int) ([]AgentRun, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT id,space_id,COALESCE(job_id,''),status,started_at,finished_at,COALESCE(error_message,'') FROM agent_runs WHERE space_id=? ORDER BY started_at DESC LIMIT ?`, spaceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []AgentRun{}
	for rows.Next() {
		var run AgentRun
		var started string
		var finished sql.NullString
		if err := rows.Scan(&run.ID, &run.SpaceID, &run.JobID, &run.Status, &started, &finished, &run.Error); err != nil {
			return nil, err
		}
		run.StartedAt = parseTime(started)
		if finished.Valid {
			value := parseTime(finished.String)
			run.FinishedAt = &value
		}
		result = append(result, run)
	}
	return result, rows.Err()
}

func (s *Store) ListMemoryEvents(spaceID string, limit int) ([]MemoryEvent, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT id,space_id,run_id,kind,COALESCE(payload_json,''),created_at FROM memory_events WHERE space_id=? ORDER BY created_at DESC LIMIT ?`, spaceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []MemoryEvent{}
	for rows.Next() {
		var event MemoryEvent
		var created string
		if err := rows.Scan(&event.ID, &event.SpaceID, &event.RunID, &event.Kind, &event.Payload, &created); err != nil {
			return nil, err
		}
		event.CreatedAt = parseTime(created)
		result = append(result, event)
	}
	return result, rows.Err()
}

func (s *Store) RecordMemoryEvent(spaceID, runID, kind, payload string) (*MemoryEvent, error) {
	if strings.TrimSpace(spaceID) == "" || strings.TrimSpace(runID) == "" || strings.TrimSpace(kind) == "" {
		return nil, errors.New("memory event requires space, run, and kind")
	}
	event := &MemoryEvent{ID: newID(), SpaceID: spaceID, RunID: runID, Kind: kind, Payload: payload, CreatedAt: time.Now().UTC()}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`INSERT INTO memory_events (id,space_id,run_id,kind,payload_json,created_at) VALUES (?,?,?,?,NULLIF(?,''),?)`, event.ID, event.SpaceID, event.RunID, event.Kind, event.Payload, event.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	return event, nil
}

func (s *Store) GetSpaceObservability(spaceID string, limit int) (*SpaceObservability, error) {
	if _, err := s.GetSpace(spaceID); err != nil {
		return nil, err
	}
	runs, err := s.ListAgentRuns(spaceID, limit)
	if err != nil {
		return nil, err
	}
	calls, err := s.ListModelCalls("", spaceID, limit)
	if err != nil {
		return nil, err
	}
	events, err := s.ListMemoryEvents(spaceID, limit)
	if err != nil {
		return nil, err
	}
	steps, err := s.ListAgentRunSteps(spaceID, "", limit)
	if err != nil {
		return nil, err
	}
	return &SpaceObservability{Runs: runs, Steps: steps, ModelCalls: calls, MemoryEvents: events}, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanJob(row scanner) (*Job, error) {
	var job Job
	var createdAt, updatedAt string
	var analysisMarkdown, replicaScriptJSON, fissionScriptsJSON sql.NullString
	var fissionDirections, creatibiResultJSON, errorMessage, runLog, spaceID, parentJobID, contextSnapshot sql.NullString
	err := row.Scan(
		&job.ID, &job.Title, &job.Status, &job.VideoPath, &job.VideoOriginalName,
		&job.ProductMDPath, &job.ProductMDName, &job.Requirement, &job.Industry,
		&job.FissionCount, &fissionDirections, &analysisMarkdown, &replicaScriptJSON,
		&fissionScriptsJSON, &creatibiResultJSON, &errorMessage, &runLog, &spaceID, &parentJobID, &contextSnapshot,
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
	job.SpaceID = spaceID.String
	job.ParentJobID = parentJobID.String
	job.ContextSnapshot = contextSnapshot.String
	job.CreatedAt = parseTime(createdAt)
	job.UpdatedAt = parseTime(updatedAt)
	return &job, nil
}

func scanChatConversation(row scanner) (*ChatConversation, error) {
	var conversation ChatConversation
	var createdAt, updatedAt string
	var summary, summaryMessageID sql.NullString
	if err := row.Scan(&conversation.ID, &conversation.Title, &summary, &summaryMessageID, &createdAt, &updatedAt); err != nil {
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
	if err := row.Scan(&product.ID, &product.Title, &product.MDPath, &product.MDName, &createdAt, &updatedAt); err != nil {
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
		&call.ID, &call.Scope, &call.RefID, &call.SpaceID, &call.RunID, &call.Step, &call.Model,
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
