package jobs

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type IntelligenceConnection struct {
	ID           string     `json:"id"`
	SpaceID      string     `json:"space_id,omitempty"`
	SourceType   string     `json:"source_type"`
	Name         string     `json:"name"`
	Status       string     `json:"status"`
	ConfigJSON   string     `json:"config_json,omitempty"`
	LastSyncedAt *time.Time `json:"last_synced_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type IntelligenceSignal struct {
	ID           string    `json:"id"`
	SpaceID      string    `json:"space_id,omitempty"`
	ConnectionID string    `json:"connection_id,omitempty"`
	SignalType   string    `json:"signal_type"`
	Title        string    `json:"title"`
	Summary      string    `json:"summary"`
	EvidenceJSON string    `json:"evidence_json"`
	Status       string    `json:"status"`
	Confidence   float64   `json:"confidence"`
	ObservedAt   time.Time `json:"observed_at"`
	CreatedAt    time.Time `json:"created_at"`
}

type CreativeMemory struct {
	ID             string    `json:"id"`
	SpaceID        string    `json:"space_id"`
	SignalID       string    `json:"signal_id,omitempty"`
	Title          string    `json:"title"`
	Finding        string    `json:"finding"`
	EvidenceJSON   string    `json:"evidence_json"`
	Status         string    `json:"status"`
	Confidence     float64   `json:"confidence"`
	LastVerifiedAt time.Time `json:"last_verified_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type UpdateCreativeMemoryInput struct {
	Title      string  `json:"title"`
	Finding    string  `json:"finding"`
	Confidence float64 `json:"confidence"`
}

type IntelligenceDashboard struct {
	Connections []IntelligenceConnection `json:"connections"`
	Signals     []IntelligenceSignal     `json:"signals"`
	Memories    []CreativeMemory         `json:"memories"`
	Monitors    []CompetitorMonitor      `json:"monitors"`
}

type CompetitorMonitor struct {
	ID            string     `json:"id"`
	SpaceID       string     `json:"space_id,omitempty"`
	Name          string     `json:"name"`
	Platform      string     `json:"platform"`
	AccountURL    string     `json:"account_url,omitempty"`
	Keywords      string     `json:"keywords,omitempty"`
	SourceType    string     `json:"source_type"`
	Schedule      string     `json:"schedule"`
	Status        string     `json:"status"`
	LastScannedAt *time.Time `json:"last_scanned_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type CreateCompetitorMonitorInput struct {
	SpaceID    string `json:"space_id"`
	Name       string `json:"name"`
	Platform   string `json:"platform"`
	AccountURL string `json:"account_url"`
	Keywords   string `json:"keywords"`
	SourceType string `json:"source_type"`
	Schedule   string `json:"schedule"`
}

func (s *Store) SeedIntelligenceDemo(userID, spaceID string) error {
	if strings.TrimSpace(userID) == "" {
		return fmt.Errorf("user is required")
	}
	if spaceID != "" {
		if _, err := s.GetUserSpace(userID, spaceID); err != nil {
			return err
		}
	}
	now := time.Now().UTC()
	synced := now.Add(-12 * time.Minute)
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	connID := "demo-market-" + userID
	_, err = tx.Exec(`INSERT INTO intelligence_connections(id,user_id,space_id,source_type,name,status,config_json,last_synced_at,created_at,updated_at)
	VALUES(?,?,?,?,?,'connected',?,?,?,?) ON CONFLICT(id) DO UPDATE SET space_id=excluded.space_id,status='connected',last_synced_at=excluded.last_synced_at,updated_at=excluded.updated_at`,
		connID, userID, nullString(spaceID), "demo", "演示数据源：防晒品类", `{"capabilities":["market_hits","competitor_content","ad_performance","comments","search_terms"],"data_label":"synthetic"}`, synced.Format(time.RFC3339), now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		return err
	}
	signals := []struct {
		kind, title, summary string
		confidence           float64
		evidence             any
	}{
		{"market_opportunity", "通勤补涂场景正在增长", "近 14 天竞品内容中“通勤、补涂、不搓泥”组合增长明显，但多数内容仍只强调便携。", 0.78, map[string]any{"source": "market_and_competitor", "sample_size": 48, "window": "14d", "growth_rate": 0.36, "crowding": "medium"}},
		{"winning_creative", "结果前置钩子值得复测", "演示投放中，结果前置版本 CTR 高于空间基线 24%，转化成本下降 11%。", 0.84, map[string]any{"source": "synthetic_ad_performance", "impressions": 38600, "baseline_ctr": 0.021, "variant_ctr": 0.026, "changed_variable": "first_3s_hook"}},
		{"fatigue", "痛点口播版本出现疲劳", "频次升至 3.8 后 CTR 连续 5 天下滑，建议保留卖点并更换钩子和首帧。", 0.81, map[string]any{"source": "synthetic_ad_performance", "frequency": 3.8, "ctr_change": -0.29, "days_declining": 5}},
		{"audience_voice", "评论集中询问妆后补涂", "样本评论中“会不会花妆、搓泥、泛白”重复出现，可转成证据型内容实验。", 0.73, map[string]any{"source": "competitor_comments", "sample_size": 126, "top_questions": []string{"妆后会花吗", "会不会搓泥", "是否泛白"}}},
	}
	for i, item := range signals {
		evidence, _ := json.Marshal(item.evidence)
		id := fmt.Sprintf("demo-signal-%s-%d", userID, i+1)
		_, err = tx.Exec(`INSERT INTO intelligence_signals(id,user_id,space_id,connection_id,signal_type,title,summary,evidence_json,confidence,status,observed_at,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,'new',?,?) ON CONFLICT(id) DO UPDATE SET space_id=excluded.space_id,summary=excluded.summary,evidence_json=excluded.evidence_json,confidence=excluded.confidence,observed_at=excluded.observed_at`,
			id, userID, nullString(spaceID), connID, item.kind, item.title, item.summary, string(evidence), item.confidence, now.Add(time.Duration(-i)*24*time.Hour).Format(time.RFC3339), now.Format(time.RFC3339))
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func nullString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}

func (s *Store) IntelligenceDashboard(userID, spaceID string) (*IntelligenceDashboard, error) {
	out := &IntelligenceDashboard{Connections: []IntelligenceConnection{}, Signals: []IntelligenceSignal{}, Memories: []CreativeMemory{}, Monitors: []CompetitorMonitor{}}
	filter := "user_id=?"
	args := []any{userID}
	if spaceID != "" {
		filter += " AND (space_id=? OR space_id IS NULL)"
		args = append(args, spaceID)
	}
	rows, err := s.db.Query(`SELECT id,COALESCE(space_id,''),source_type,name,status,COALESCE(config_json,''),last_synced_at,created_at,updated_at FROM intelligence_connections WHERE `+filter+` ORDER BY updated_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var x IntelligenceConnection
		var last sql.NullString
		var c, u string
		if err = rows.Scan(&x.ID, &x.SpaceID, &x.SourceType, &x.Name, &x.Status, &x.ConfigJSON, &last, &c, &u); err != nil {
			rows.Close()
			return nil, err
		}
		if last.Valid {
			t := parseTime(last.String)
			x.LastSyncedAt = &t
		}
		x.CreatedAt = parseTime(c)
		x.UpdatedAt = parseTime(u)
		out.Connections = append(out.Connections, x)
	}
	rows.Close()
	rows, err = s.db.Query(`SELECT id,COALESCE(space_id,''),COALESCE(connection_id,''),signal_type,title,summary,evidence_json,confidence,status,observed_at,created_at FROM intelligence_signals WHERE `+filter+` ORDER BY observed_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var x IntelligenceSignal
		var o, c string
		if err = rows.Scan(&x.ID, &x.SpaceID, &x.ConnectionID, &x.SignalType, &x.Title, &x.Summary, &x.EvidenceJSON, &x.Confidence, &x.Status, &o, &c); err != nil {
			rows.Close()
			return nil, err
		}
		x.ObservedAt = parseTime(o)
		x.CreatedAt = parseTime(c)
		out.Signals = append(out.Signals, x)
	}
	rows.Close()
	rows, err = s.db.Query(`SELECT id,space_id,COALESCE(signal_id,''),title,finding,evidence_json,confidence,status,last_verified_at,created_at,updated_at FROM creative_memories WHERE user_id=?`+func() string {
		if spaceID != "" {
			return " AND space_id=?"
		}
		return ""
	}()+` ORDER BY updated_at DESC`, func() []any {
		if spaceID != "" {
			return []any{userID, spaceID}
		}
		return []any{userID}
	}()...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var x CreativeMemory
		var v, c, u string
		if err = rows.Scan(&x.ID, &x.SpaceID, &x.SignalID, &x.Title, &x.Finding, &x.EvidenceJSON, &x.Confidence, &x.Status, &v, &c, &u); err != nil {
			rows.Close()
			return nil, err
		}
		x.LastVerifiedAt = parseTime(v)
		x.CreatedAt = parseTime(c)
		x.UpdatedAt = parseTime(u)
		out.Memories = append(out.Memories, x)
	}
	rows.Close()
	rows, err = s.db.Query(`SELECT id,COALESCE(space_id,''),name,platform,COALESCE(account_url,''),COALESCE(keywords,''),source_type,schedule,status,last_scanned_at,created_at,updated_at FROM competitor_monitors WHERE `+filter+` ORDER BY updated_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var x CompetitorMonitor
		var last sql.NullString
		var c, u string
		if err = rows.Scan(&x.ID, &x.SpaceID, &x.Name, &x.Platform, &x.AccountURL, &x.Keywords, &x.SourceType, &x.Schedule, &x.Status, &last, &c, &u); err != nil {
			rows.Close()
			return nil, err
		}
		if last.Valid {
			t := parseTime(last.String)
			x.LastScannedAt = &t
		}
		x.CreatedAt = parseTime(c)
		x.UpdatedAt = parseTime(u)
		out.Monitors = append(out.Monitors, x)
	}
	rows.Close()
	return out, nil
}

func (s *Store) CreateCompetitorMonitor(userID string, input CreateCompetitorMonitorInput) (*CompetitorMonitor, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Platform = strings.TrimSpace(input.Platform)
	input.SpaceID = strings.TrimSpace(input.SpaceID)
	if input.Name == "" || input.Platform == "" {
		return nil, fmt.Errorf("competitor name and platform are required")
	}
	if input.SpaceID != "" {
		if _, err := s.GetUserSpace(userID, input.SpaceID); err != nil {
			return nil, err
		}
	}
	allowed := map[string]bool{"demo": true, "web_search": true, "platform_library": true, "third_party": true, "file_import": true}
	if !allowed[input.SourceType] {
		input.SourceType = "demo"
	}
	if input.Schedule == "" {
		input.Schedule = "manual"
	}
	now := time.Now().UTC()
	x := &CompetitorMonitor{ID: newID(), SpaceID: input.SpaceID, Name: input.Name, Platform: input.Platform, AccountURL: strings.TrimSpace(input.AccountURL), Keywords: strings.TrimSpace(input.Keywords), SourceType: input.SourceType, Schedule: input.Schedule, Status: "active", CreatedAt: now, UpdatedAt: now}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`INSERT INTO competitor_monitors(id,user_id,space_id,name,platform,account_url,keywords,source_type,schedule,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, x.ID, userID, nullString(x.SpaceID), x.Name, x.Platform, nullString(x.AccountURL), x.Keywords, x.SourceType, x.Schedule, x.Status, now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	return x, nil
}

func (s *Store) ScanCompetitorMonitorDemo(userID, monitorID string) (*IntelligenceSignal, error) {
	var m CompetitorMonitor
	var spaceID sql.NullString
	if err := s.db.QueryRow(`SELECT id,space_id,name,platform,COALESCE(account_url,''),COALESCE(keywords,''),source_type,schedule,status,created_at,updated_at FROM competitor_monitors WHERE id=? AND user_id=?`, monitorID, userID).Scan(&m.ID, &spaceID, &m.Name, &m.Platform, &m.AccountURL, &m.Keywords, &m.SourceType, &m.Schedule, &m.Status, new(string), new(string)); err != nil {
		return nil, err
	}
	m.SpaceID = spaceID.String
	now := time.Now().UTC()
	evidence, _ := json.Marshal(map[string]any{"source": "synthetic_competitor_scan", "monitor_id": m.ID, "platform": m.Platform, "account_url": m.AccountURL, "keywords": strings.FieldsFunc(m.Keywords, func(r rune) bool { return r == ',' || r == '，' }), "sample_size": 24, "window": "30d", "data_label": "synthetic"})
	signal := &IntelligenceSignal{ID: newID(), SpaceID: m.SpaceID, SignalType: "competitor_change", Title: m.Name + "近期强化场景化对比内容", Summary: "演示扫描发现该竞品近 30 天增加了场景化对比和结果前置内容。请连接真实数据源后再用于正式决策。", EvidenceJSON: string(evidence), Confidence: .62, Status: "new", ObservedAt: now, CreatedAt: now}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`INSERT INTO intelligence_signals(id,user_id,space_id,signal_type,title,summary,evidence_json,confidence,status,observed_at,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, signal.ID, userID, nullString(signal.SpaceID), signal.SignalType, signal.Title, signal.Summary, signal.EvidenceJSON, signal.Confidence, signal.Status, now.Format(time.RFC3339), now.Format(time.RFC3339)); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`UPDATE competitor_monitors SET last_scanned_at=?,updated_at=? WHERE id=? AND user_id=?`, now.Format(time.RFC3339), now.Format(time.RFC3339), m.ID, userID); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	signal.ConnectionID = ""
	return signal, nil
}

func (s *Store) PromoteSignalToMemory(userID, signalID, spaceID string) (*CreativeMemory, error) {
	var title, summary, evidence string
	var confidence float64
	if err := s.db.QueryRow(`SELECT title,summary,evidence_json,confidence FROM intelligence_signals WHERE id=? AND user_id=?`, signalID, userID).Scan(&title, &summary, &evidence, &confidence); err != nil {
		return nil, err
	}
	if _, err := s.GetUserSpace(userID, spaceID); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	x := &CreativeMemory{ID: newID(), SpaceID: spaceID, SignalID: signalID, Title: title, Finding: summary, EvidenceJSON: evidence, Confidence: confidence, Status: "active", LastVerifiedAt: now, CreatedAt: now, UpdatedAt: now}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`INSERT INTO creative_memories(id,user_id,space_id,signal_id,title,finding,evidence_json,confidence,status,last_verified_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, x.ID, userID, spaceID, signalID, title, summary, evidence, confidence, x.Status, now.Format(time.RFC3339), now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	return x, nil
}

func (s *Store) UpdateCreativeMemory(userID, id string, input UpdateCreativeMemoryInput) (*CreativeMemory, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Finding = strings.TrimSpace(input.Finding)
	if input.Title == "" || input.Finding == "" {
		return nil, fmt.Errorf("memory title and finding are required")
	}
	if input.Confidence < 0 || input.Confidence > 1 {
		return nil, fmt.Errorf("memory confidence must be between 0 and 1")
	}
	now := time.Now().UTC()
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	result, err := s.db.Exec(`UPDATE creative_memories SET title=?,finding=?,confidence=?,last_verified_at=?,updated_at=? WHERE id=? AND user_id=?`, input.Title, input.Finding, input.Confidence, now.Format(time.RFC3339), now.Format(time.RFC3339), id, userID)
	if err != nil {
		return nil, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return nil, sql.ErrNoRows
	}
	var x CreativeMemory
	var verified, created, updated string
	if err := s.db.QueryRow(`SELECT id,space_id,COALESCE(signal_id,''),title,finding,evidence_json,confidence,status,last_verified_at,created_at,updated_at FROM creative_memories WHERE id=? AND user_id=?`, id, userID).Scan(&x.ID, &x.SpaceID, &x.SignalID, &x.Title, &x.Finding, &x.EvidenceJSON, &x.Confidence, &x.Status, &verified, &created, &updated); err != nil {
		return nil, err
	}
	x.LastVerifiedAt, x.CreatedAt, x.UpdatedAt = parseTime(verified), parseTime(created), parseTime(updated)
	return &x, nil
}

func (s *Store) DeleteCreativeMemory(userID, id string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	result, err := s.db.Exec(`DELETE FROM creative_memories WHERE id=? AND user_id=?`, id, userID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) CreativeMemoryContext(userID, spaceID string, limit int) string {
	if limit <= 0 || limit > 12 {
		limit = 6
	}
	rows, err := s.db.Query(`SELECT title,finding,confidence,last_verified_at FROM creative_memories WHERE user_id=? AND space_id=? AND status='active' ORDER BY updated_at DESC LIMIT ?`, userID, spaceID, limit)
	if err != nil {
		return ""
	}
	defer rows.Close()
	lines := []string{}
	for rows.Next() {
		var title, finding, verified string
		var confidence float64
		if rows.Scan(&title, &finding, &confidence, &verified) == nil {
			lines = append(lines, fmt.Sprintf("- %s：%s（置信度 %.0f%%，验证于 %s）", title, finding, confidence*100, parseTime(verified).Format("2006-01-02")))
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return "经用户确认的创意记忆：\n" + strings.Join(lines, "\n") + "\n使用规则：只作为有时间边界的经验，不得替代当前数据；与新证据冲突时应指出冲突。"
}
