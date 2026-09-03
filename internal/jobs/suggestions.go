package jobs

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type suggestionSeed struct {
	spaceID, productID, triggerType, title, summary, actionType, targetID, dedupeKey string
	priority                                                                         int
}

// RefreshProactiveSuggestions derives low-cost, explainable next actions from
// existing state. UNIQUE(user_id, dedupe_key) makes refresh safe and prevents
// dismissed suggestions from returning.
func (s *Store) RefreshProactiveSuggestions(userID string) ([]ProactiveSuggestion, error) {
	spaces, err := s.ListUserSpaces(userID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	seeds := make([]suggestionSeed, 0)
	for _, space := range spaces {
		if space.Status != SpaceStatusActive {
			continue
		}
		if space.ProductID == "" {
			seeds = append(seeds, suggestionSeed{spaceID: space.ID, triggerType: "missing_product", title: "补充产品资料", summary: "这个空间还没有产品上下文，补充后 Agent 才能稳定引用卖点和素材。", actionType: "open_space", targetID: space.ID, dedupeKey: "missing_product:" + space.ID, priority: 90})
		} else if assets, assetErr := s.ListProductAssets(space.ProductID); assetErr == nil && len(assets) == 0 {
			seeds = append(seeds, suggestionSeed{spaceID: space.ID, productID: space.ProductID, triggerType: "missing_assets", title: "为产品补充参考素材", summary: "当前资料没有图片或视频。上传后，脚本和分镜可以直接引用真实产品视觉。", actionType: "open_product", targetID: space.ProductID, dedupeKey: "missing_assets:" + space.ID + ":" + space.ProductID, priority: 85})
		}
		if now.Sub(space.UpdatedAt) >= 7*24*time.Hour {
			week := now.Format("2006-01-02")
			seeds = append(seeds, suggestionSeed{spaceID: space.ID, productID: space.ProductID, triggerType: "stale_space", title: "继续推进「" + space.Title + "」", summary: "这个创作目标已有一段时间没有推进，可以从现有上下文继续，不必重新说明。", actionType: "continue_space", targetID: space.ID, dedupeKey: "stale_space:" + space.ID + ":" + week, priority: 55})
		}
	}
	videos, err := s.ListUserVideoGenerations(userID)
	if err != nil {
		return nil, err
	}
	for _, video := range videos {
		if video.Status != StatusCompleted {
			continue
		}
		seeds = append(seeds, suggestionSeed{spaceID: video.SpaceID, productID: video.ProductID, triggerType: "video_completed", title: "视频已经生成", summary: "查看结果并决定是否下载、调整提示词或继续生成变体。", actionType: "review_video", targetID: video.ID, dedupeKey: "video_completed:" + video.ID, priority: 100})
	}
	for _, seed := range seeds {
		_, err = s.db.Exec(`INSERT OR IGNORE INTO proactive_suggestions(id,user_id,space_id,product_id,trigger_type,title,summary,action_type,action_target_id,priority,status,dedupe_key,created_at,updated_at) VALUES(?,?,NULLIF(?,''),NULLIF(?,''),?,?,?,?,?,?, 'pending',?,?,?)`, newID(), userID, seed.spaceID, seed.productID, seed.triggerType, seed.title, seed.summary, seed.actionType, seed.targetID, seed.priority, seed.dedupeKey, now.Format(time.RFC3339), now.Format(time.RFC3339))
		if err != nil {
			return nil, err
		}
	}
	return s.ListProactiveSuggestions(userID)
}

func (s *Store) ListProactiveSuggestions(userID string) ([]ProactiveSuggestion, error) {
	rows, err := s.db.Query(`SELECT id,user_id,space_id,product_id,trigger_type,title,summary,action_type,action_target_id,priority,status,created_at,updated_at FROM proactive_suggestions WHERE user_id=? AND status IN ('pending','accepted') ORDER BY priority DESC,created_at DESC LIMIT 12`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ProactiveSuggestion{}
	for rows.Next() {
		var x ProactiveSuggestion
		var spaceID, productID, targetID sql.NullString
		var created, updated string
		if err := rows.Scan(&x.ID, &x.UserID, &spaceID, &productID, &x.TriggerType, &x.Title, &x.Summary, &x.ActionType, &targetID, &x.Priority, &x.Status, &created, &updated); err != nil {
			return nil, err
		}
		x.SpaceID, x.ProductID, x.ActionTargetID = spaceID.String, productID.String, targetID.String
		x.CreatedAt, x.UpdatedAt = parseTime(created), parseTime(updated)
		items = append(items, x)
	}
	return items, rows.Err()
}

func (s *Store) UpdateProactiveSuggestionStatus(userID, id, status string) (*ProactiveSuggestion, error) {
	status = strings.TrimSpace(status)
	if status != "accepted" && status != "dismissed" && status != "completed" {
		return nil, fmt.Errorf("unsupported suggestion status")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.Exec(`UPDATE proactive_suggestions SET status=?,updated_at=? WHERE id=? AND user_id=?`, status, now, id, userID)
	if err != nil {
		return nil, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return nil, sql.ErrNoRows
	}
	var x ProactiveSuggestion
	var spaceID, productID, targetID sql.NullString
	var created, updated string
	err = s.db.QueryRow(`SELECT id,user_id,space_id,product_id,trigger_type,title,summary,action_type,action_target_id,priority,status,created_at,updated_at FROM proactive_suggestions WHERE id=? AND user_id=?`, id, userID).Scan(&x.ID, &x.UserID, &spaceID, &productID, &x.TriggerType, &x.Title, &x.Summary, &x.ActionType, &targetID, &x.Priority, &x.Status, &created, &updated)
	if err != nil {
		return nil, err
	}
	x.SpaceID, x.ProductID, x.ActionTargetID = spaceID.String, productID.String, targetID.String
	x.CreatedAt, x.UpdatedAt = parseTime(created), parseTime(updated)
	return &x, nil
}
