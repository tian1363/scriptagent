package creatibi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tian1363/scriptagent/internal/jobs"
)

type CLIPublisher struct {
	bin       string
	projectID int
	timeout   time.Duration
}

type Config struct {
	Bin       string
	ProjectID int
	Timeout   time.Duration
}

func NewCLIPublisher(cfg Config) *CLIPublisher {
	bin := cfg.Bin
	if bin == "" {
		bin = "cbi"
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	return &CLIPublisher{
		bin:       bin,
		projectID: cfg.ProjectID,
		timeout:   timeout,
	}
}

func (p *CLIPublisher) Publish(job jobs.Job) (string, error) {
	if job.ReplicaScriptJSON == "" {
		return "", errors.New("replica script is empty")
	}
	if job.FissionScriptsJSON == "" {
		return "", errors.New("fission scripts are empty")
	}

	projectID, projectName, err := p.resolveProject()
	if err != nil {
		return "", err
	}

	var replica scriptPayload
	if err := json.Unmarshal([]byte(job.ReplicaScriptJSON), &replica); err != nil {
		return "", fmt.Errorf("parse replica script: %w", err)
	}
	var fissions []scriptPayload
	if err := json.Unmarshal([]byte(job.FissionScriptsJSON), &fissions); err != nil {
		return "", fmt.Errorf("parse fission scripts: %w", err)
	}

	created := publishResult{
		Mode:        "cbi-cli",
		ProjectID:   projectID,
		ProjectName: projectName,
		Scripts:     []publishedScript{},
	}

	replicaTitle := strings.TrimSpace(replica.Title)
	if replicaTitle == "" {
		replicaTitle = job.Title + " 复刻脚本"
	}
	parentID, raw, err := p.createScript(projectID, replicaTitle, 0, "scriptagent:"+job.ID+":replica")
	if err != nil {
		return "", fmt.Errorf("create replica script: %w", err)
	}
	if err := p.saveStoryboard(parentID, replica); err != nil {
		return "", fmt.Errorf("save replica script: %w", err)
	}
	created.Scripts = append(created.Scripts, publishedScript{
		ID:          parentID,
		Title:       replicaTitle,
		Type:        "replica",
		ParentID:    0,
		CreateRaw:   raw,
		StoryboardN: len(replica.Storyboards),
	})

	for i, fission := range fissions {
		title := fission.Title
		if strings.TrimSpace(title) == "" {
			title = fmt.Sprintf("%s 裂变脚本 %02d", job.Title, i+1)
		}
		childID, raw, err := p.createScript(projectID, title, parentID, "scriptagent:"+job.ID+":fission")
		if err != nil {
			return "", fmt.Errorf("create fission script %d: %w", i+1, err)
		}
		if err := p.saveStoryboard(childID, fission); err != nil {
			return "", fmt.Errorf("save fission script %d: %w", i+1, err)
		}
		created.Scripts = append(created.Scripts, publishedScript{
			ID:          childID,
			Title:       title,
			Type:        "fission",
			ParentID:    parentID,
			CreateRaw:   raw,
			StoryboardN: len(fission.Storyboards),
		})
	}

	out, err := json.MarshalIndent(created, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (p *CLIPublisher) resolveProject() (int, string, error) {
	if p.projectID > 0 {
		return p.projectID, "", nil
	}
	out, err := p.run("project", "list", "--format", "json", "-q")
	if err != nil {
		return 0, "", fmt.Errorf("list projects: %w", err)
	}
	var parsed struct {
		Projects []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"projects"`
	}
	if err := unmarshalEmbeddedJSON(out, &parsed); err != nil {
		return 0, "", fmt.Errorf("parse project list: %w", err)
	}
	if len(parsed.Projects) == 0 {
		return 0, "", errors.New("no CreatiBI projects available")
	}
	return parsed.Projects[0].ID, parsed.Projects[0].Name, nil
}

func (p *CLIPublisher) createScript(projectID int, name string, parentID int, sourceObject string) (int, string, error) {
	args := []string{"project", "script-create", "--project-id", strconv.Itoa(projectID), "--name", name, "--source-object", sourceObject, "--format", "json", "-q"}
	if parentID > 0 {
		args = append(args, "--parent-id", strconv.Itoa(parentID))
	}
	out, err := p.run(args...)
	if err != nil {
		return 0, out, err
	}
	id, err := extractID(out)
	if err != nil {
		return 0, out, fmt.Errorf("parse created script id: %w; output: %s", err, out)
	}
	return id, out, nil
}

func (p *CLIPublisher) saveStoryboard(scriptID int, script scriptPayload) error {
	content, err := json.Marshal(toCreatiBIStoryboard(script.Storyboards))
	if err != nil {
		return err
	}
	_, err = p.run("project", "script-save", "--script-id", strconv.Itoa(scriptID), "--format", "2", "--script", string(content), "-q")
	return err
}

func (p *CLIPublisher) run(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, p.bin, args...)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if ctx.Err() == context.DeadlineExceeded {
		return text, fmt.Errorf("cbi command timed out")
	}
	if err != nil {
		return text, fmt.Errorf("%w: %s", err, text)
	}
	return text, nil
}

func toCreatiBIStoryboard(items []storyboard) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		copyText := strings.TrimSpace(strings.Join(nonEmpty([]string{
			"画面：" + item.Visual,
			"动作：" + item.Action,
			"旁白：" + item.Voiceover,
			"字幕：" + item.Subtitle,
			"镜头：" + item.ShotSize,
			"镜头动机：" + item.CameraIntent,
			"道具场景：" + item.PropsScene,
			"音频：" + item.Audio,
			"目的：" + item.Purpose,
		}), "\n"))
		result = append(result, map[string]any{
			"Copy":       copyText,
			"duration":   durationSeconds(item.TimeRange),
			"time_range": item.TimeRange,
		})
	}
	return result
}

func nonEmpty(values []string) []string {
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || strings.HasSuffix(value, "：") {
			continue
		}
		result = append(result, value)
	}
	return result
}

func durationSeconds(timeRange string) int {
	re := regexp.MustCompile(`(\d{2}):(\d{2})`)
	matches := re.FindAllStringSubmatch(timeRange, -1)
	if len(matches) < 2 {
		return 5
	}
	start := atoi(matches[0][1])*60 + atoi(matches[0][2])
	end := atoi(matches[1][1])*60 + atoi(matches[1][2])
	if end <= start {
		return 5
	}
	return end - start
}

func atoi(value string) int {
	parsed, _ := strconv.Atoi(value)
	return parsed
}

func extractID(text string) (int, error) {
	var obj map[string]any
	if err := unmarshalEmbeddedJSON(text, &obj); err == nil {
		for _, key := range []string{"id", "scriptId", "script_id", "taskId", "task_id"} {
			if id, ok := numberFrom(obj[key]); ok {
				return id, nil
			}
		}
		for _, value := range obj {
			if nested, ok := value.(map[string]any); ok {
				for _, key := range []string{"id", "scriptId", "script_id", "taskId", "task_id"} {
					if id, ok := numberFrom(nested[key]); ok {
						return id, nil
					}
				}
			}
		}
	}
	re := regexp.MustCompile(`(?i)(?:scriptId|script_id|taskId|task_id|id)[^\d]{0,12}(\d+)`)
	match := re.FindStringSubmatch(text)
	if len(match) >= 2 {
		return strconv.Atoi(match[1])
	}
	return 0, errors.New("script id not found")
}

func numberFrom(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		return int(typed), typed > 0
	case int:
		return typed, typed > 0
	case string:
		parsed, err := strconv.Atoi(typed)
		return parsed, err == nil && parsed > 0
	default:
		return 0, false
	}
}

func unmarshalEmbeddedJSON(text string, v any) error {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return errors.New("json object not found")
	}
	return json.Unmarshal([]byte(text[start:end+1]), v)
}

type scriptPayload struct {
	Title       string       `json:"title"`
	ScriptType  string       `json:"script_type"`
	Storyboards []storyboard `json:"storyboards"`
}

type storyboard struct {
	TimeRange    string `json:"time_range"`
	Visual       string `json:"visual"`
	Action       string `json:"action"`
	Voiceover    string `json:"voiceover"`
	Subtitle     string `json:"subtitle"`
	ShotSize     string `json:"shot_size"`
	CameraIntent string `json:"camera_intent"`
	PropsScene   string `json:"props_scene"`
	Audio        string `json:"audio"`
	Purpose      string `json:"purpose"`
}

type publishResult struct {
	Mode        string            `json:"mode"`
	ProjectID   int               `json:"project_id"`
	ProjectName string            `json:"project_name,omitempty"`
	Scripts     []publishedScript `json:"scripts"`
}

type publishedScript struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Type        string `json:"type"`
	ParentID    int    `json:"parent_id,omitempty"`
	StoryboardN int    `json:"storyboard_count"`
	CreateRaw   string `json:"create_raw,omitempty"`
}
