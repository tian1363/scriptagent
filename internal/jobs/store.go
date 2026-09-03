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
  space_id TEXT,
  product_id TEXT,
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
CREATE TABLE IF NOT EXISTS chat_agent_steps (
  id TEXT PRIMARY KEY, conversation_id TEXT NOT NULL, message_id TEXT NOT NULL,
  step_index INTEGER NOT NULL, kind TEXT NOT NULL, reason TEXT, tool TEXT,
  input TEXT, observation TEXT, error TEXT, created_at TEXT NOT NULL,
  FOREIGN KEY(conversation_id) REFERENCES chat_conversations(id) ON DELETE CASCADE,
  FOREIGN KEY(message_id) REFERENCES chat_messages(id) ON DELETE CASCADE
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
CREATE INDEX IF NOT EXISTS idx_chat_agent_steps_message ON chat_agent_steps(message_id, step_index);
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
CREATE TABLE IF NOT EXISTS model_capability_settings (
  capability TEXT PRIMARY KEY,
  mode TEXT NOT NULL DEFAULT 'byok',
  api_key TEXT,
  endpoint TEXT NOT NULL,
  model TEXT NOT NULL,
  provider TEXT NOT NULL DEFAULT 'dashscope',
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS custom_skills (
  id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, title TEXT NOT NULL, description TEXT NOT NULL,
  category TEXT NOT NULL, invocation_prompt TEXT NOT NULL, content TEXT NOT NULL,
  created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY, email TEXT NOT NULL UNIQUE, name TEXT, role TEXT NOT NULL,
  status TEXT NOT NULL, password_hash TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
  token TEXT PRIMARY KEY, user_id TEXT NOT NULL, expires_at TEXT NOT NULL, created_at TEXT NOT NULL,
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
CREATE TABLE IF NOT EXISTS resource_owners (
  resource_type TEXT NOT NULL, resource_id TEXT NOT NULL, user_id TEXT NOT NULL,
  created_at TEXT NOT NULL, PRIMARY KEY(resource_type, resource_id),
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_resource_owners_user ON resource_owners(user_id, resource_type);
CREATE TABLE IF NOT EXISTS user_model_capability_settings (
  user_id TEXT NOT NULL, capability TEXT NOT NULL, mode TEXT NOT NULL DEFAULT 'byok',
  api_key TEXT, endpoint TEXT NOT NULL, model TEXT NOT NULL, provider TEXT NOT NULL DEFAULT 'dashscope',
  updated_at TEXT NOT NULL, PRIMARY KEY(user_id, capability),
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS spaces (
  id TEXT PRIMARY KEY, title TEXT NOT NULL, summary TEXT, product_id TEXT,
  agent_brief TEXT, marketing_goal TEXT, goal_stage TEXT, status TEXT NOT NULL, origin_space_id TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
  FOREIGN KEY(product_id) REFERENCES products(id) ON DELETE RESTRICT
);
CREATE TABLE IF NOT EXISTS product_assets (
  id TEXT PRIMARY KEY, product_id TEXT NOT NULL, kind TEXT NOT NULL, path TEXT NOT NULL,
  original_name TEXT NOT NULL, mime_type TEXT NOT NULL, size_bytes INTEGER NOT NULL, created_at TEXT NOT NULL,
  FOREIGN KEY(product_id) REFERENCES products(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS video_generations (
  id TEXT PRIMARY KEY, user_id TEXT NOT NULL, product_id TEXT, space_id TEXT, conversation_id TEXT, source_asset_id TEXT,
  source_asset_ids_json TEXT,
  mode TEXT NOT NULL, prompt TEXT NOT NULL, negative_prompt TEXT, model TEXT NOT NULL,
  resolution TEXT NOT NULL, ratio TEXT NOT NULL, duration INTEGER NOT NULL, status TEXT NOT NULL,
  sound_enabled INTEGER NOT NULL DEFAULT 1,
  estimated_cost_cny REAL NOT NULL DEFAULT 0,
  provider_task_id TEXT, video_url TEXT, local_path TEXT, error_message TEXT,
  created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
  FOREIGN KEY(product_id) REFERENCES products(id) ON DELETE SET NULL,
  FOREIGN KEY(space_id) REFERENCES spaces(id) ON DELETE SET NULL,
  FOREIGN KEY(source_asset_id) REFERENCES product_assets(id) ON DELETE SET NULL
);
CREATE TABLE IF NOT EXISTS proactive_suggestions (
  id TEXT PRIMARY KEY, user_id TEXT NOT NULL, space_id TEXT, product_id TEXT,
  trigger_type TEXT NOT NULL, title TEXT NOT NULL, summary TEXT NOT NULL,
  action_type TEXT NOT NULL, action_target_id TEXT, priority INTEGER NOT NULL DEFAULT 50,
  status TEXT NOT NULL DEFAULT 'pending', dedupe_key TEXT NOT NULL,
  created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
  UNIQUE(user_id, dedupe_key),
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
  FOREIGN KEY(space_id) REFERENCES spaces(id) ON DELETE SET NULL,
  FOREIGN KEY(product_id) REFERENCES products(id) ON DELETE SET NULL
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
CREATE TABLE IF NOT EXISTS intelligence_connections (
  id TEXT PRIMARY KEY, user_id TEXT NOT NULL, space_id TEXT, source_type TEXT NOT NULL,
  name TEXT NOT NULL, status TEXT NOT NULL, config_json TEXT, last_synced_at TEXT,
  created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
  FOREIGN KEY(space_id) REFERENCES spaces(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS intelligence_signals (
  id TEXT PRIMARY KEY, user_id TEXT NOT NULL, space_id TEXT, connection_id TEXT,
  signal_type TEXT NOT NULL, title TEXT NOT NULL, summary TEXT NOT NULL,
  evidence_json TEXT NOT NULL, confidence REAL NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'new', observed_at TEXT NOT NULL, created_at TEXT NOT NULL,
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
  FOREIGN KEY(space_id) REFERENCES spaces(id) ON DELETE CASCADE,
  FOREIGN KEY(connection_id) REFERENCES intelligence_connections(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS creative_memories (
  id TEXT PRIMARY KEY, user_id TEXT NOT NULL, space_id TEXT NOT NULL, signal_id TEXT,
  title TEXT NOT NULL, finding TEXT NOT NULL, evidence_json TEXT NOT NULL,
  confidence REAL NOT NULL DEFAULT 0, status TEXT NOT NULL DEFAULT 'active',
  last_verified_at TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
  FOREIGN KEY(space_id) REFERENCES spaces(id) ON DELETE CASCADE,
  FOREIGN KEY(signal_id) REFERENCES intelligence_signals(id) ON DELETE SET NULL
);
CREATE TABLE IF NOT EXISTS competitor_monitors (
  id TEXT PRIMARY KEY, user_id TEXT NOT NULL, space_id TEXT, name TEXT NOT NULL,
  platform TEXT NOT NULL, account_url TEXT, keywords TEXT, source_type TEXT NOT NULL,
  schedule TEXT NOT NULL DEFAULT 'manual', status TEXT NOT NULL DEFAULT 'active',
  last_scanned_at TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
  FOREIGN KEY(space_id) REFERENCES spaces(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_products_updated ON products(updated_at);
CREATE INDEX IF NOT EXISTS idx_creative_reports_product ON creative_reports(product_id, created_at);
CREATE INDEX IF NOT EXISTS idx_product_chunks_product ON product_chunks(product_id, chunk_index);
CREATE INDEX IF NOT EXISTS idx_spaces_updated ON spaces(updated_at);
CREATE INDEX IF NOT EXISTS idx_jobs_space ON jobs(space_id, created_at);
CREATE INDEX IF NOT EXISTS idx_product_assets_product ON product_assets(product_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_video_generations_user ON video_generations(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_proactive_suggestions_user ON proactive_suggestions(user_id, status, priority DESC, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_runs_space ON agent_runs(space_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_steps_run ON agent_steps(run_id, step_index);
CREATE INDEX IF NOT EXISTS idx_memory_events_space_run ON memory_events(space_id, run_id, created_at);
CREATE INDEX IF NOT EXISTS idx_intelligence_connections_user ON intelligence_connections(user_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_intelligence_signals_user ON intelligence_signals(user_id, observed_at);
CREATE INDEX IF NOT EXISTS idx_creative_memories_space ON creative_memories(user_id, space_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_competitor_monitors_user ON competitor_monitors(user_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_custom_skills_updated ON custom_skills(updated_at DESC);
`)
	if err != nil {
		return err
	}
	if err := s.ensureColumn("chat_conversations", "summary", "TEXT"); err != nil {
		return err
	}
	if err := s.ensureColumn("chat_conversations", "space_id", "TEXT"); err != nil {
		return err
	}
	if err := s.ensureColumn("chat_conversations", "product_id", "TEXT"); err != nil {
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
	if err := s.ensureColumn("model_calls", "user_id", "TEXT"); err != nil {
		return err
	}
	if err := s.ensureColumn("video_generations", "sound_enabled", "INTEGER NOT NULL DEFAULT 1"); err != nil {
		return err
	}
	if err := s.ensureColumn("video_generations", "conversation_id", "TEXT"); err != nil {
		return err
	}
	if err := s.ensureColumn("video_generations", "estimated_cost_cny", "REAL NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn("video_generations", "source_asset_ids_json", "TEXT"); err != nil {
		return err
	}
	if _, err := s.db.Exec(`UPDATE video_generations SET estimated_cost_cny = duration * CASE
		WHEN model='wan3.0-video-prime' AND resolution='480P' THEN 0.45
		WHEN model='wan3.0-video-prime' AND resolution='720P' THEN 0.9
		WHEN model='wan3.0-video-prime' AND resolution='1080P' THEN 1.8
		WHEN model='wan3.0-video' AND resolution='480P' THEN 0.3
		WHEN model='wan3.0-video' AND resolution='720P' THEN 0.6
		WHEN model='wan3.0-video' AND resolution='1080P' THEN 1.2
		ELSE 0 END WHERE estimated_cost_cny=0 AND status!='failed'`); err != nil {
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
	if err := s.ensureColumn("spaces", "marketing_goal", "TEXT"); err != nil {
		return err
	}
	if err := s.ensureColumn("spaces", "goal_stage", "TEXT"); err != nil {
		return err
	}
	if _, err := s.db.Exec(`UPDATE spaces SET marketing_goal='conversion' WHERE COALESCE(marketing_goal,'')=''; UPDATE spaces SET goal_stage='action' WHERE COALESCE(goal_stage,'')=''`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_spaces_updated ON spaces(updated_at)`); err != nil {
		return err
	}
	var firstUser string
	if scanErr := s.db.QueryRow(`SELECT id FROM users ORDER BY created_at ASC LIMIT 1`).Scan(&firstUser); scanErr == nil {
		if err := s.AdoptLegacyData(firstUser); err != nil {
			return err
		}
	}
	return nil
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

func (s *Store) DeleteProductAsset(id string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	result, err := s.db.Exec(`DELETE FROM product_assets WHERE id=?`, id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return sql.ErrNoRows
	}
	return nil
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
	space := &Space{ID: newID(), Title: normalizeTitle(input.Title), Summary: strings.TrimSpace(input.Summary), ProductID: productID, AgentBrief: strings.TrimSpace(input.AgentBrief), MarketingGoal: strings.TrimSpace(input.MarketingGoal), GoalStage: strings.TrimSpace(input.GoalStage), Status: SpaceStatusActive, OriginSpaceID: strings.TrimSpace(input.OriginSpaceID), CreatedAt: now, UpdatedAt: now}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`INSERT INTO spaces (id,title,summary,product_id,agent_brief,marketing_goal,goal_stage,status,origin_space_id,created_at,updated_at) VALUES (?,?,?,NULLIF(?,''),?,?,?,?,NULLIF(?,''),?,?)`, space.ID, space.Title, space.Summary, space.ProductID, space.AgentBrief, space.MarketingGoal, space.GoalStage, space.Status, space.OriginSpaceID, now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	return space, nil
}

func (s *Store) ListSpaces() ([]Space, error) {
	rows, err := s.db.Query(`SELECT id,title,COALESCE(summary,''),COALESCE(product_id,''),COALESCE(agent_brief,''),COALESCE(marketing_goal,''),COALESCE(goal_stage,''),status,COALESCE(origin_space_id,''),created_at,updated_at FROM spaces ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Space{}
	for rows.Next() {
		var x Space
		var c, u string
		if err := rows.Scan(&x.ID, &x.Title, &x.Summary, &x.ProductID, &x.AgentBrief, &x.MarketingGoal, &x.GoalStage, &x.Status, &x.OriginSpaceID, &c, &u); err != nil {
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
	err := s.db.QueryRow(`SELECT id,title,COALESCE(summary,''),COALESCE(product_id,''),COALESCE(agent_brief,''),COALESCE(marketing_goal,''),COALESCE(goal_stage,''),status,COALESCE(origin_space_id,''),created_at,updated_at FROM spaces WHERE id=?`, id).Scan(&space.ID, &space.Title, &space.Summary, &space.ProductID, &space.AgentBrief, &space.MarketingGoal, &space.GoalStage, &space.Status, &space.OriginSpaceID, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	space.CreatedAt = parseTime(createdAt)
	space.UpdatedAt = parseTime(updatedAt)
	return &space, nil
}

func (s *Store) UpdateSpace(id string, input UpdateSpaceInput) (*Space, error) {
	productID := strings.TrimSpace(input.ProductID)
	if productID != "" {
		if _, err := s.GetProduct(productID); err != nil {
			return nil, fmt.Errorf("product not found: %w", err)
		}
	}
	current, err := s.GetSpace(id)
	if err != nil {
		return nil, err
	}
	title := normalizeTitle(input.Title)
	if strings.TrimSpace(input.Title) == "" {
		title = current.Title
	}
	now := time.Now().UTC()
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	res, err := s.db.Exec(`UPDATE spaces SET title=?,summary=?,product_id=NULLIF(?,''),agent_brief=?,marketing_goal=?,goal_stage=?,updated_at=? WHERE id=?`, title, strings.TrimSpace(input.Summary), productID, strings.TrimSpace(input.AgentBrief), strings.TrimSpace(input.MarketingGoal), strings.TrimSpace(input.GoalStage), now.Format(time.RFC3339), id)
	if err != nil {
		return nil, err
	}
	if err := requireOne(res); err != nil {
		return nil, err
	}
	current.Title, current.Summary, current.ProductID, current.AgentBrief, current.MarketingGoal, current.GoalStage, current.UpdatedAt = title, strings.TrimSpace(input.Summary), productID, strings.TrimSpace(input.AgentBrief), strings.TrimSpace(input.MarketingGoal), strings.TrimSpace(input.GoalStage), now
	return current, nil
}

func (s *Store) DeleteSpace(id string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	result, err := s.db.Exec(`DELETE FROM spaces WHERE id=?`, strings.TrimSpace(id))
	if err != nil {
		return err
	}
	return requireOne(result)
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

	capability := strings.TrimSpace(settings.Capability)
	if capability == "" {
		capability = "text"
	}
	existing, _ := s.GetModelSettingsForCapability(capability)
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
	mode := strings.TrimSpace(settings.Mode)
	if mode == "" {
		mode = "byok"
	}
	_, err := s.db.Exec(`INSERT INTO model_capability_settings (capability, mode, api_key, endpoint, model, provider, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(capability) DO UPDATE SET mode=excluded.mode, api_key=excluded.api_key, endpoint=excluded.endpoint, model=excluded.model, provider=excluded.provider, updated_at=excluded.updated_at`,
		capability, mode, apiKey, endpoint, modelName, provider, now.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	if capability != "text" {
		return &ModelSettings{Capability: capability, Mode: mode, APIKey: apiKey, Provider: provider, Endpoint: endpoint, Model: modelName, UpdatedAt: now}, nil
	}
	_, err = s.db.Exec(`
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
	return &ModelSettings{Capability: capability, Mode: mode, APIKey: apiKey, Provider: provider, Endpoint: endpoint, Model: modelName, UpdatedAt: now}, nil
}

func (s *Store) GetModelSettings() (*ModelSettings, error) {
	if settings, err := s.GetModelSettingsForCapability("text"); err == nil {
		return settings, nil
	}
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

func (s *Store) GetModelSettingsForCapability(capability string) (*ModelSettings, error) {
	settings := &ModelSettings{Capability: capability}
	var updatedAt string
	var apiKey sql.NullString
	err := s.db.QueryRow(`SELECT mode, api_key, endpoint, model, provider, updated_at FROM model_capability_settings WHERE capability=?`, capability).
		Scan(&settings.Mode, &apiKey, &settings.Endpoint, &settings.Model, &settings.Provider, &updatedAt)
	if err != nil {
		return nil, err
	}
	settings.APIKey, settings.UpdatedAt = apiKey.String, parseTime(updatedAt)
	return settings, nil
}

func (s *Store) ListModelSettings() ([]ModelSettings, error) {
	rows, err := s.db.Query(`SELECT capability, mode, api_key, endpoint, model, provider, updated_at FROM model_capability_settings ORDER BY capability`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []ModelSettings{}
	for rows.Next() {
		var item ModelSettings
		var key sql.NullString
		var updated string
		if err := rows.Scan(&item.Capability, &item.Mode, &key, &item.Endpoint, &item.Model, &item.Provider, &updated); err != nil {
			return nil, err
		}
		item.APIKey, item.UpdatedAt = key.String, parseTime(updated)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) GetModelRuntimeConfig(ctx context.Context, capability string) (model.RuntimeConfig, error) {
	userID := userctx.UserID(ctx)
	settings, err := s.GetUserModelSettingsForCapability(userID, capability)
	if userID == "" {
		settings, err = s.GetModelSettingsForCapability(capability)
	}
	if err != nil && capability != "text" {
		if userID != "" {
			settings, err = s.GetUserModelSettingsForCapability(userID, "text")
		} else {
			settings, err = s.GetModelSettings()
		}
	}
	if err != nil {
		return model.RuntimeConfig{}, err
	}
	return model.RuntimeConfig{
		APIKey:   settings.APIKey,
		Provider: settings.Provider,
		Endpoint: settings.Endpoint,
		Model:    settings.Model,
		Source:   valueOrModelMode(settings.Mode),
	}, nil
}

func valueOrModelMode(value string) string {
	if strings.TrimSpace(value) == "managed" {
		return "managed"
	}
	return "byok"
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
	return s.CreateChatConversationWithContext(title, "", "")
}

func (s *Store) CreateChatConversationWithContext(title, spaceID, productID string) (*ChatConversation, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	now := time.Now().UTC()
	conversation := &ChatConversation{
		ID: newID(), Title: normalizeTitle(title), SpaceID: strings.TrimSpace(spaceID), ProductID: strings.TrimSpace(productID), CreatedAt: now, UpdatedAt: now,
	}
	_, err := s.db.Exec(`
INSERT INTO chat_conversations (id, title, space_id, product_id, created_at, updated_at)
VALUES (?, ?, NULLIF(?,''), NULLIF(?,''), ?, ?)`,
		conversation.ID, conversation.Title, conversation.SpaceID, conversation.ProductID, now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	return conversation, nil
}

func (s *Store) ListChatConversations() ([]ChatConversation, error) {
	rows, err := s.db.Query(`
SELECT id, title, COALESCE(space_id,''), COALESCE(product_id,''), summary, summary_message_id, created_at, updated_at
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
SELECT id, title, COALESCE(space_id,''), COALESCE(product_id,''), summary, summary_message_id, created_at, updated_at
FROM chat_conversations WHERE id = ?`, id))
	if err != nil {
		return nil, err
	}
	messages, err := s.ListChatMessages(id)
	if err != nil {
		return nil, err
	}
	traces, err := s.ListChatAgentTraces(id)
	if err != nil {
		return nil, err
	}
	return &ChatThread{Conversation: *conversation, Messages: messages, AgentTraces: traces}, nil
}

func (s *Store) SaveChatAgentSteps(conversationID, messageID string, steps []AgentStep) error {
	if len(steps) == 0 {
		return nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	for _, step := range steps {
		if _, err = tx.Exec(`INSERT INTO chat_agent_steps(id,conversation_id,message_id,step_index,kind,reason,tool,input,observation,error,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, newID(), conversationID, messageID, step.Index, step.Kind, step.Reason, step.Tool, step.Input, step.Observation, step.Error, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ListChatAgentTraces(conversationID string) (map[string][]AgentStep, error) {
	rows, err := s.db.Query(`SELECT message_id,step_index,kind,COALESCE(reason,''),COALESCE(tool,''),COALESCE(input,''),COALESCE(observation,''),COALESCE(error,'') FROM chat_agent_steps WHERE conversation_id=? ORDER BY created_at,step_index`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string][]AgentStep{}
	for rows.Next() {
		var messageID string
		var step AgentStep
		if err := rows.Scan(&messageID, &step.Index, &step.Kind, &step.Reason, &step.Tool, &step.Input, &step.Observation, &step.Error); err != nil {
			return nil, err
		}
		result[messageID] = append(result[messageID], step)
	}
	return result, rows.Err()
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
	_, err := s.db.Exec(`
INSERT INTO model_calls (
  id, user_id, scope, ref_id, space_id, run_id, step, model, input_json, output_text, response_json,
  prompt_tokens, output_tokens, total_tokens, latency_ms, error_message, created_at
) VALUES (?, NULLIF(?,''), ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		newID(), userctx.UserID(ctx), valueOr(record.Scope, "unknown"), record.RefID, record.SpaceID, record.RunID, record.Step, record.Model,
		record.InputJSON, record.OutputText, record.ResponseJSON,
		record.PromptTokens, record.OutputTokens, record.TotalTokens, record.LatencyMS,
		record.ErrorMessage, now.Format(time.RFC3339),
	)
	return err
}

func (s *Store) ListUserModelCalls(userID, refID, spaceID string, limit int) ([]ModelCall, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	query := `SELECT id,scope,ref_id,COALESCE(space_id,''),COALESCE(run_id,''),step,model,input_json,output_text,response_json,prompt_tokens,output_tokens,total_tokens,latency_ms,error_message,created_at FROM model_calls WHERE user_id=?`
	args := []any{userID}
	if strings.TrimSpace(refID) != "" {
		query += ` AND ref_id=?`
		args = append(args, refID)
	}
	if strings.TrimSpace(spaceID) != "" {
		query += ` AND space_id=?`
		args = append(args, spaceID)
	}
	query += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ModelCall{}
	for rows.Next() {
		x, err := scanModelCall(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *x)
	}
	return out, rows.Err()
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
	if err := row.Scan(&conversation.ID, &conversation.Title, &conversation.SpaceID, &conversation.ProductID, &summary, &summaryMessageID, &createdAt, &updatedAt); err != nil {
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

func (s *Store) CreateCustomSkill(input CreateCustomSkillInput) (*CustomSkill, error) {
	now := time.Now().UTC()
	skill := &CustomSkill{
		ID: newID(), Name: strings.TrimSpace(input.Name), Title: strings.TrimSpace(input.Title),
		Description: strings.TrimSpace(input.Description), Category: strings.TrimSpace(input.Category),
		InvocationPrompt: strings.TrimSpace(input.InvocationPrompt), Content: strings.TrimSpace(input.Content),
		Source: "custom", CreatedAt: now, UpdatedAt: now,
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`INSERT INTO custom_skills (id,name,title,description,category,invocation_prompt,content,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		skill.ID, skill.Name, skill.Title, skill.Description, skill.Category, skill.InvocationPrompt, skill.Content,
		now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	return skill, nil
}

func (s *Store) UpdateCustomSkill(id string, input CreateCustomSkillInput) (*CustomSkill, error) {
	now := time.Now().UTC()
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	result, err := s.db.Exec(`UPDATE custom_skills SET name=?,title=?,description=?,category=?,invocation_prompt=?,content=?,updated_at=? WHERE id=?`,
		strings.TrimSpace(input.Name), strings.TrimSpace(input.Title), strings.TrimSpace(input.Description), strings.TrimSpace(input.Category),
		strings.TrimSpace(input.InvocationPrompt), strings.TrimSpace(input.Content), now.Format(time.RFC3339), strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	if count, countErr := result.RowsAffected(); countErr != nil || count == 0 {
		return nil, sql.ErrNoRows
	}
	var item CustomSkill
	var createdAt, updatedAt string
	err = s.db.QueryRow(`SELECT id,name,title,description,category,invocation_prompt,content,created_at,updated_at FROM custom_skills WHERE id=?`, strings.TrimSpace(id)).Scan(
		&item.ID, &item.Name, &item.Title, &item.Description, &item.Category, &item.InvocationPrompt, &item.Content, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	item.Source, item.CreatedAt, item.UpdatedAt = "custom", parseTime(createdAt), parseTime(updatedAt)
	return &item, nil
}

func (s *Store) ListCustomSkills() ([]CustomSkill, error) {
	rows, err := s.db.Query(`SELECT id,name,title,description,category,invocation_prompt,content,created_at,updated_at FROM custom_skills ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []CustomSkill{}
	for rows.Next() {
		var item CustomSkill
		var createdAt, updatedAt string
		if err := rows.Scan(&item.ID, &item.Name, &item.Title, &item.Description, &item.Category, &item.InvocationPrompt, &item.Content, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		item.Source, item.CreatedAt, item.UpdatedAt = "custom", parseTime(createdAt), parseTime(updatedAt)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetCustomSkillByName(name string) (*CustomSkill, error) {
	var item CustomSkill
	var createdAt, updatedAt string
	err := s.db.QueryRow(`SELECT id,name,title,description,category,invocation_prompt,content,created_at,updated_at FROM custom_skills WHERE name=?`, strings.TrimSpace(name)).Scan(
		&item.ID, &item.Name, &item.Title, &item.Description, &item.Category, &item.InvocationPrompt, &item.Content, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	item.Source, item.CreatedAt, item.UpdatedAt = "custom", parseTime(createdAt), parseTime(updatedAt)
	return &item, nil
}

func (s *Store) CountUsers() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query(`SELECT id,email,name,role,status,password_hash,created_at,updated_at FROM users ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []User
	for rows.Next() {
		var x User
		var name sql.NullString
		var created, updated string
		if err := rows.Scan(&x.ID, &x.Email, &name, &x.Role, &x.Status, &x.PasswordHash, &created, &updated); err != nil {
			return nil, err
		}
		x.Name, x.CreatedAt, x.UpdatedAt = name.String, parseTime(created), parseTime(updated)
		result = append(result, x)
	}
	return result, rows.Err()
}

func (s *Store) CreateUser(input CreateUserInput) (*User, error) {
	now := time.Now().UTC()
	user := &User{ID: newID(), Email: strings.ToLower(strings.TrimSpace(input.Email)), Name: strings.TrimSpace(input.Name), Role: valueOr(input.Role, "admin"), Status: valueOr(input.Status, "active"), PasswordHash: input.PasswordHash, CreatedAt: now, UpdatedAt: now}
	_, err := s.db.Exec(`INSERT INTO users (id,email,name,role,status,password_hash,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?)`, user.ID, user.Email, user.Name, user.Role, user.Status, user.PasswordHash, now.Format(time.RFC3339), now.Format(time.RFC3339))
	return user, err
}

func (s *Store) GetUser(id string) (*User, error) {
	var user User
	var createdAt, updatedAt string
	err := s.db.QueryRow(`SELECT id,email,COALESCE(name,''),role,status,password_hash,created_at,updated_at FROM users WHERE id=?`, id).Scan(&user.ID, &user.Email, &user.Name, &user.Role, &user.Status, &user.PasswordHash, &createdAt, &updatedAt)
	user.CreatedAt, user.UpdatedAt = parseTime(createdAt), parseTime(updatedAt)
	return &user, err
}

func (s *Store) GetUserByEmail(email string) (*User, error) {
	var user User
	var createdAt, updatedAt string
	err := s.db.QueryRow(`SELECT id,email,COALESCE(name,''),role,status,password_hash,created_at,updated_at FROM users WHERE email=?`, strings.ToLower(strings.TrimSpace(email))).Scan(&user.ID, &user.Email, &user.Name, &user.Role, &user.Status, &user.PasswordHash, &createdAt, &updatedAt)
	user.CreatedAt, user.UpdatedAt = parseTime(createdAt), parseTime(updatedAt)
	return &user, err
}

func (s *Store) CreateSession(input CreateSessionInput) (*Session, error) {
	now := time.Now().UTC()
	_, err := s.db.Exec(`INSERT INTO sessions (token,user_id,expires_at,created_at) VALUES (?,?,?,?)`, input.Token, input.UserID, input.ExpiresAt.Format(time.RFC3339), now.Format(time.RFC3339))
	return &Session{Token: input.Token, UserID: input.UserID, ExpiresAt: input.ExpiresAt, CreatedAt: now}, err
}

func (s *Store) GetSession(token string) (*Session, error) {
	var session Session
	var expiresAt, createdAt string
	err := s.db.QueryRow(`SELECT token,user_id,expires_at,created_at FROM sessions WHERE token=?`, token).Scan(&session.Token, &session.UserID, &expiresAt, &createdAt)
	session.ExpiresAt, session.CreatedAt = parseTime(expiresAt), parseTime(createdAt)
	return &session, err
}

func (s *Store) DeleteSession(token string) error {
	if token == "" {
		return nil
	}
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token=?`, token)
	return err
}

func (s *Store) ClaimResource(userID, resourceType, resourceID string) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(resourceID) == "" {
		return errors.New("resource owner is required")
	}
	_, err := s.db.Exec(`INSERT INTO resource_owners(resource_type,resource_id,user_id,created_at) VALUES(?,?,?,?) ON CONFLICT(resource_type,resource_id) DO UPDATE SET user_id=excluded.user_id`, resourceType, resourceID, userID, time.Now().UTC().Format(time.RFC3339))
	return err
}

func (s *Store) OwnsResource(userID, resourceType, resourceID string) bool {
	var one int
	return s.db.QueryRow(`SELECT 1 FROM resource_owners WHERE user_id=? AND resource_type=? AND resource_id=?`, userID, resourceType, resourceID).Scan(&one) == nil
}
func (s *Store) ResourceOwner(resourceType, resourceID string) string {
	var id string
	_ = s.db.QueryRow(`SELECT user_id FROM resource_owners WHERE resource_type=? AND resource_id=?`, resourceType, resourceID).Scan(&id)
	return id
}

func (s *Store) SaveUserModelSettings(userID string, settings ModelSettings) (*ModelSettings, error) {
	capability := valueOr(strings.TrimSpace(settings.Capability), "text")
	existing, _ := s.GetUserModelSettingsForCapability(userID, capability)
	key := strings.TrimSpace(settings.APIKey)
	if key == "" && existing != nil {
		key = existing.APIKey
	}
	mode := valueOr(strings.TrimSpace(settings.Mode), "byok")
	provider := valueOr(strings.TrimSpace(settings.Provider), "dashscope")
	endpoint := valueOr(strings.TrimSpace(settings.Endpoint), "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation")
	modelName := valueOr(strings.TrimSpace(settings.Model), "qwen3.8-flash")
	now := time.Now().UTC()
	_, err := s.db.Exec(`INSERT INTO user_model_capability_settings(user_id,capability,mode,api_key,endpoint,model,provider,updated_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(user_id,capability) DO UPDATE SET mode=excluded.mode,api_key=excluded.api_key,endpoint=excluded.endpoint,model=excluded.model,provider=excluded.provider,updated_at=excluded.updated_at`, userID, capability, mode, key, endpoint, modelName, provider, now.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	return &ModelSettings{Capability: capability, Mode: mode, APIKey: key, Endpoint: endpoint, Model: modelName, Provider: provider, UpdatedAt: now}, nil
}
func (s *Store) GetUserModelSettingsForCapability(userID, capability string) (*ModelSettings, error) {
	var x ModelSettings
	var key sql.NullString
	var updated string
	err := s.db.QueryRow(`SELECT capability,mode,api_key,endpoint,model,provider,updated_at FROM user_model_capability_settings WHERE user_id=? AND capability=?`, userID, capability).Scan(&x.Capability, &x.Mode, &key, &x.Endpoint, &x.Model, &x.Provider, &updated)
	x.APIKey = key.String
	x.UpdatedAt = parseTime(updated)
	return &x, err
}
func (s *Store) ListUserModelSettings(userID string) ([]ModelSettings, error) {
	rows, err := s.db.Query(`SELECT capability,mode,api_key,endpoint,model,provider,updated_at FROM user_model_capability_settings WHERE user_id=? ORDER BY capability`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ModelSettings{}
	for rows.Next() {
		var x ModelSettings
		var key sql.NullString
		var updated string
		if err := rows.Scan(&x.Capability, &x.Mode, &key, &x.Endpoint, &x.Model, &x.Provider, &updated); err != nil {
			return nil, err
		}
		x.APIKey = key.String
		x.UpdatedAt = parseTime(updated)
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Store) FilterOwnedIDs(userID, resourceType string) (map[string]bool, error) {
	rows, err := s.db.Query(`SELECT resource_id FROM resource_owners WHERE user_id=? AND resource_type=?`, userID, resourceType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, rows.Err()
}

// AdoptLegacyData gives the first registered account ownership of records that
// predate authentication. Later accounts start with an empty workspace.
func (s *Store) AdoptLegacyData(userID string) error {
	resources := map[string]string{"product": "products", "space": "spaces", "job": "jobs", "chat": "chat_conversations", "skill": "custom_skills"}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	for kind, table := range resources {
		query := `INSERT OR IGNORE INTO resource_owners(resource_type,resource_id,user_id,created_at) SELECT ?,id,?,? FROM ` + table
		if _, err := tx.Exec(query, kind, userID, now); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO user_model_capability_settings(user_id,capability,mode,api_key,endpoint,model,provider,updated_at) SELECT ?,capability,mode,api_key,endpoint,model,provider,updated_at FROM model_capability_settings`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE model_calls SET user_id=? WHERE COALESCE(user_id,'')=''`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ListUserProducts(userID string) ([]Product, error) {
	items, err := s.ListProducts()
	if err != nil {
		return nil, err
	}
	owned, err := s.FilterOwnedIDs(userID, "product")
	if err != nil {
		return nil, err
	}
	out := []Product{}
	for _, item := range items {
		if owned[item.ID] {
			out = append(out, item)
		}
	}
	return out, nil
}
func (s *Store) GetUserProduct(userID, id string) (*Product, error) {
	if !s.OwnsResource(userID, "product", id) {
		return nil, sql.ErrNoRows
	}
	return s.GetProduct(id)
}
func (s *Store) ListUserSpaces(userID string) ([]Space, error) {
	items, err := s.ListSpaces()
	if err != nil {
		return nil, err
	}
	owned, err := s.FilterOwnedIDs(userID, "space")
	if err != nil {
		return nil, err
	}
	out := []Space{}
	for _, x := range items {
		if owned[x.ID] {
			out = append(out, x)
		}
	}
	return out, nil
}
func (s *Store) GetUserSpace(userID, id string) (*Space, error) {
	if !s.OwnsResource(userID, "space", id) {
		return nil, sql.ErrNoRows
	}
	return s.GetSpace(id)
}
func (s *Store) ListUserJobs(userID string) ([]Job, error) {
	items, err := s.ListJobs()
	if err != nil {
		return nil, err
	}
	owned, err := s.FilterOwnedIDs(userID, "job")
	if err != nil {
		return nil, err
	}
	out := []Job{}
	for _, x := range items {
		if owned[x.ID] {
			out = append(out, x)
		}
	}
	return out, nil
}
func (s *Store) GetUserJob(userID, id string) (*Job, error) {
	if !s.OwnsResource(userID, "job", id) {
		return nil, sql.ErrNoRows
	}
	return s.GetJob(id)
}
func (s *Store) ListUserChats(userID string) ([]ChatConversation, error) {
	items, err := s.ListChatConversations()
	if err != nil {
		return nil, err
	}
	owned, err := s.FilterOwnedIDs(userID, "chat")
	if err != nil {
		return nil, err
	}
	out := []ChatConversation{}
	for _, x := range items {
		if owned[x.ID] {
			out = append(out, x)
		}
	}
	return out, nil
}
func (s *Store) GetUserChatThread(userID, id string) (*ChatThread, error) {
	if !s.OwnsResource(userID, "chat", id) {
		return nil, sql.ErrNoRows
	}
	return s.GetChatThread(id)
}
func (s *Store) ListUserCustomSkills(userID string) ([]CustomSkill, error) {
	items, err := s.ListCustomSkills()
	if err != nil {
		return nil, err
	}
	owned, err := s.FilterOwnedIDs(userID, "skill")
	if err != nil {
		return nil, err
	}
	out := []CustomSkill{}
	for _, x := range items {
		if owned[x.ID] {
			out = append(out, x)
		}
	}
	return out, nil
}
func (s *Store) GetUserCustomSkillByName(userID, name string) (*CustomSkill, error) {
	item, err := s.GetCustomSkillByName(name)
	if err != nil {
		return nil, err
	}
	if !s.OwnsResource(userID, "skill", item.ID) {
		return nil, sql.ErrNoRows
	}
	return item, nil
}

func (s *Store) CreateVideoGeneration(input CreateVideoGenerationInput) (*VideoGeneration, error) {
	now := time.Now().UTC()
	assetIDs := append([]string(nil), input.SourceAssetIDs...)
	if len(assetIDs) == 0 && input.SourceAssetID != "" {
		assetIDs = []string{input.SourceAssetID}
	}
	assetJSON, _ := json.Marshal(assetIDs)
	x := &VideoGeneration{ID: newID(), UserID: input.UserID, ProductID: input.ProductID, SpaceID: input.SpaceID, ConversationID: input.ConversationID, SourceAssetID: input.SourceAssetID, SourceAssetIDs: assetIDs, Mode: input.Mode, Prompt: input.Prompt, NegativePrompt: input.NegativePrompt, Model: input.Model, Resolution: input.Resolution, Ratio: input.Ratio, Duration: input.Duration, SoundEnabled: input.SoundEnabled, EstimatedCostCNY: input.EstimatedCostCNY, Status: "pending", CreatedAt: now, UpdatedAt: now}
	_, err := s.db.Exec(`INSERT INTO video_generations(id,user_id,product_id,space_id,conversation_id,source_asset_id,source_asset_ids_json,mode,prompt,negative_prompt,model,resolution,ratio,duration,sound_enabled,estimated_cost_cny,status,created_at,updated_at) VALUES(?,?,NULLIF(?,''),NULLIF(?,''),NULLIF(?,''),NULLIF(?,''),?,?,?,?,?,?,?,?,?,?,?,?,?)`, x.ID, x.UserID, x.ProductID, x.SpaceID, x.ConversationID, x.SourceAssetID, string(assetJSON), x.Mode, x.Prompt, x.NegativePrompt, x.Model, x.Resolution, x.Ratio, x.Duration, x.SoundEnabled, x.EstimatedCostCNY, x.Status, now.Format(time.RFC3339), now.Format(time.RFC3339))
	return x, err
}

func (s *Store) UpdateVideoGeneration(id, status, taskID, videoURL, localPath, errorMessage string) error {
	_, err := s.db.Exec(`UPDATE video_generations SET status=?,provider_task_id=NULLIF(?,''),video_url=NULLIF(?,''),local_path=NULLIF(?,''),error_message=NULLIF(?,''),updated_at=? WHERE id=?`, status, taskID, videoURL, localPath, errorMessage, time.Now().UTC().Format(time.RFC3339), id)
	return err
}

func (s *Store) GetUserVideoGeneration(userID, id string) (*VideoGeneration, error) {
	var x VideoGeneration
	var productID, spaceID, conversationID, assetID, assetIDsJSON, negative, taskID, videoURL, localPath, errorMessage sql.NullString
	var created, updated string
	err := s.db.QueryRow(`SELECT id,user_id,product_id,space_id,conversation_id,source_asset_id,source_asset_ids_json,mode,prompt,negative_prompt,model,resolution,ratio,duration,sound_enabled,estimated_cost_cny,status,provider_task_id,video_url,local_path,error_message,created_at,updated_at FROM video_generations WHERE user_id=? AND id=?`, userID, id).Scan(&x.ID, &x.UserID, &productID, &spaceID, &conversationID, &assetID, &assetIDsJSON, &x.Mode, &x.Prompt, &negative, &x.Model, &x.Resolution, &x.Ratio, &x.Duration, &x.SoundEnabled, &x.EstimatedCostCNY, &x.Status, &taskID, &videoURL, &localPath, &errorMessage, &created, &updated)
	x.ProductID, x.SpaceID, x.ConversationID, x.SourceAssetID, x.NegativePrompt, x.ProviderTaskID, x.VideoURL, x.LocalPath, x.ErrorMessage = productID.String, spaceID.String, conversationID.String, assetID.String, negative.String, taskID.String, videoURL.String, localPath.String, errorMessage.String
	_ = json.Unmarshal([]byte(assetIDsJSON.String), &x.SourceAssetIDs)
	if len(x.SourceAssetIDs) == 0 && x.SourceAssetID != "" {
		x.SourceAssetIDs = []string{x.SourceAssetID}
	}
	x.CreatedAt, x.UpdatedAt = parseTime(created), parseTime(updated)
	return &x, err
}

func (s *Store) ListUserVideoGenerations(userID string) ([]VideoGeneration, error) {
	rows, err := s.db.Query(`SELECT id FROM video_generations WHERE user_id=? ORDER BY created_at DESC LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	result := make([]VideoGeneration, 0, len(ids))
	for _, id := range ids {
		x, err := s.GetUserVideoGeneration(userID, id)
		if err != nil {
			return nil, err
		}
		result = append(result, *x)
	}
	return result, rows.Err()
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
