package jobs

import "time"

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name,omitempty"`
	Role         string    `json:"role"`
	Status       string    `json:"status"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type CreateUserInput struct {
	Email, Name, Role, Status, PasswordHash string
}

type Session struct {
	Token, UserID string
	ExpiresAt     time.Time
	CreatedAt     time.Time
}

type CreateSessionInput struct {
	Token, UserID string
	ExpiresAt     time.Time
}

const (
	StatusPending             = "pending"
	StatusRunning             = "running"
	StatusAnalyzingVideo      = "analyzing_video"
	StatusExtractingStructure = "extracting_structure"
	StatusGeneratingReplica   = "generating_replica"
	StatusGeneratingFission   = "generating_fission"
	StatusValidating          = "validating"
	StatusCompleted           = "completed"
	StatusPublishing          = "publishing"
	StatusPublished           = "published"
	StatusFailed              = "failed"
)

type Job struct {
	ID                 string    `json:"id"`
	Title              string    `json:"title"`
	Status             string    `json:"status"`
	VideoPath          string    `json:"video_path"`
	VideoOriginalName  string    `json:"video_original_name"`
	ProductMDPath      string    `json:"product_md_path"`
	ProductMDName      string    `json:"product_md_name"`
	Requirement        string    `json:"requirement"`
	Industry           string    `json:"industry"`
	FissionCount       int       `json:"fission_count"`
	FissionDirections  string    `json:"fission_directions,omitempty"`
	SpaceID            string    `json:"space_id,omitempty"`
	ParentJobID        string    `json:"parent_job_id,omitempty"`
	ContextSnapshot    string    `json:"context_snapshot,omitempty"`
	AnalysisMarkdown   string    `json:"analysis_markdown,omitempty"`
	ReplicaScriptJSON  string    `json:"replica_script_json,omitempty"`
	FissionScriptsJSON string    `json:"fission_scripts_json,omitempty"`
	CreatiBIResultJSON string    `json:"creatibi_result_json,omitempty"`
	ErrorMessage       string    `json:"error_message,omitempty"`
	RunLog             string    `json:"run_log,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

const (
	SpaceStatusActive = "active"
)

// Space preserves the context of a long-running creative project.
type Space struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Summary       string    `json:"summary,omitempty"`
	ProductID     string    `json:"product_id"`
	AgentBrief    string    `json:"agent_brief,omitempty"`
	MarketingGoal string    `json:"marketing_goal,omitempty"`
	GoalStage     string    `json:"goal_stage,omitempty"`
	Status        string    `json:"status"`
	OriginSpaceID string    `json:"origin_space_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type CreateSpaceInput struct {
	Title         string `json:"title"`
	Summary       string `json:"summary"`
	ProductID     string `json:"product_id"`
	AgentBrief    string `json:"agent_brief"`
	MarketingGoal string `json:"marketing_goal"`
	GoalStage     string `json:"goal_stage"`
	OriginSpaceID string `json:"origin_space_id"`
}
type UpdateSpaceInput struct {
	Title         string `json:"title"`
	Summary       string `json:"summary"`
	ProductID     string `json:"product_id"`
	AgentBrief    string `json:"agent_brief"`
	MarketingGoal string `json:"marketing_goal"`
	GoalStage     string `json:"goal_stage"`
}
type ForkSpaceInput struct{ Title, Summary, AgentBrief string }

type Product struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	MDPath    string    `json:"md_path"`
	MDName    string    `json:"md_name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ProductAsset is a reusable image or video attached to product knowledge.
type ProductAsset struct {
	ID           string    `json:"id"`
	ProductID    string    `json:"product_id"`
	Kind         string    `json:"kind"`
	Path         string    `json:"-"`
	OriginalName string    `json:"original_name"`
	MimeType     string    `json:"mime_type"`
	SizeBytes    int64     `json:"size_bytes"`
	CreatedAt    time.Time `json:"created_at"`
}

// VideoGeneration is one user-owned asynchronous UGC video render.
type VideoGeneration struct {
	ID               string    `json:"id"`
	UserID           string    `json:"-"`
	ProductID        string    `json:"product_id,omitempty"`
	SpaceID          string    `json:"space_id,omitempty"`
	ConversationID   string    `json:"conversation_id,omitempty"`
	SourceAssetID    string    `json:"source_asset_id,omitempty"`
	SourceAssetIDs   []string  `json:"source_asset_ids,omitempty"`
	Mode             string    `json:"mode"`
	Prompt           string    `json:"prompt"`
	NegativePrompt   string    `json:"negative_prompt,omitempty"`
	Model            string    `json:"model"`
	Resolution       string    `json:"resolution"`
	Ratio            string    `json:"ratio"`
	Duration         int       `json:"duration"`
	SoundEnabled     bool      `json:"sound_enabled"`
	EstimatedCostCNY float64   `json:"estimated_cost_cny"`
	Status           string    `json:"status"`
	ProviderTaskID   string    `json:"provider_task_id,omitempty"`
	VideoURL         string    `json:"video_url,omitempty"`
	LocalPath        string    `json:"-"`
	ErrorMessage     string    `json:"error_message,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type CreateVideoGenerationInput struct {
	UserID, ProductID, SpaceID, ConversationID, SourceAssetID, Mode, Prompt, NegativePrompt, Model, Resolution, Ratio string
	SourceAssetIDs                                                                                                    []string
	Duration                                                                                                          int
	SoundEnabled                                                                                                      bool
	EstimatedCostCNY                                                                                                  float64
}

// ProactiveSuggestion is an explainable, user-owned next action proposed from
// existing product signals. Execution always remains behind user confirmation.
type ProactiveSuggestion struct {
	ID             string    `json:"id"`
	UserID         string    `json:"-"`
	SpaceID        string    `json:"space_id,omitempty"`
	ProductID      string    `json:"product_id,omitempty"`
	TriggerType    string    `json:"trigger_type"`
	Title          string    `json:"title"`
	Summary        string    `json:"summary"`
	ActionType     string    `json:"action_type"`
	ActionTargetID string    `json:"action_target_id,omitempty"`
	Priority       int       `json:"priority"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type CreativeReport struct {
	ID               string    `json:"id"`
	ProductID        string    `json:"product_id"`
	ProductTitle     string    `json:"product_title"`
	SourceConfigJSON string    `json:"source_config_json"`
	ReportMarkdown   string    `json:"report_markdown"`
	ReportSummary    string    `json:"report_summary"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type CreateCreativeReportInput struct {
	ProductID        string
	ProductTitle     string
	SourceConfigJSON string
	ReportMarkdown   string
	ReportSummary    string
}

type ProductChunk struct {
	ID             string    `json:"id"`
	ProductID      string    `json:"product_id"`
	ChunkIndex     int       `json:"chunk_index"`
	Heading        string    `json:"heading"`
	Content        string    `json:"content"`
	Embedding      []float64 `json:"-"`
	EmbeddingModel string    `json:"embedding_model"`
	EmbeddingDim   int       `json:"embedding_dim"`
	CreatedAt      time.Time `json:"created_at"`
}

type ProductCitation struct {
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	ChunkID     string  `json:"chunk_id,omitempty"`
	ChunkIndex  int     `json:"chunk_index"`
	Heading     string  `json:"heading"`
	Snippet     string  `json:"snippet"`
	Score       float64 `json:"score,omitempty"`
	Source      string  `json:"source"`
}

type ProductChunkInput struct {
	ChunkIndex     int
	Heading        string
	Content        string
	Embedding      []float64
	EmbeddingModel string
	EmbeddingDim   int
}

type CreateJobInput struct {
	Title             string
	VideoPath         string
	VideoOriginalName string
	ProductMDPath     string
	ProductMDName     string
	Requirement       string
	Industry          string
	FissionCount      int
	FissionDirections string
	SpaceID           string
	ParentJobID       string
	ContextSnapshot   string
}

type CreateProductInput struct {
	Title  string
	MDPath string
	MDName string
}
type UpdateProductInput struct{ Title string }

type ModelSettings struct {
	Capability string    `json:"capability"`
	Mode       string    `json:"mode"`
	APIKey     string    `json:"-"`
	Provider   string    `json:"provider"`
	Endpoint   string    `json:"endpoint"`
	Model      string    `json:"model"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type PublicModelSettings struct {
	Capability string    `json:"capability"`
	Mode       string    `json:"mode"`
	Configured bool      `json:"configured"`
	Source     string    `json:"source"`
	APIKeyMask string    `json:"api_key_mask,omitempty"`
	Provider   string    `json:"provider"`
	Endpoint   string    `json:"endpoint"`
	Model      string    `json:"model"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
}

type PublicModelConfiguration struct {
	Configured bool                  `json:"configured"`
	Source     string                `json:"source"`
	Profiles   []PublicModelSettings `json:"profiles"`
}

type ScriptResult struct {
	AnalysisMarkdown   string
	ReplicaScriptJSON  string
	FissionScriptsJSON string
}

// RunContext carries stable observability identifiers through one execution.
type RunContext struct {
	RunID   string
	Scope   string
	RefID   string
	SpaceID string
	JobID   string
}

type ChatConversation struct {
	ID               string    `json:"id"`
	Title            string    `json:"title"`
	SpaceID          string    `json:"space_id,omitempty"`
	ProductID        string    `json:"product_id,omitempty"`
	Summary          string    `json:"summary,omitempty"`
	SummaryMessageID string    `json:"summary_message_id,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type ChatMessage struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
}

type ChatThread struct {
	Conversation ChatConversation       `json:"conversation"`
	Messages     []ChatMessage          `json:"messages"`
	Citations    []ProductCitation      `json:"citations,omitempty"`
	AgentSteps   []AgentStep            `json:"agent_steps,omitempty"`
	AgentTraces  map[string][]AgentStep `json:"agent_traces,omitempty"`
}

type AgentStep struct {
	Index       int    `json:"index"`
	Kind        string `json:"kind"`
	Status      string `json:"status,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Tool        string `json:"tool,omitempty"`
	Input       string `json:"input,omitempty"`
	Observation string `json:"observation,omitempty"`
	Error       string `json:"error,omitempty"`
}

// AgentRun represents one task execution inside a creative space.
type AgentRun struct {
	ID         string     `json:"id"`
	SpaceID    string     `json:"space_id"`
	JobID      string     `json:"job_id,omitempty"`
	Status     string     `json:"status"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Error      string     `json:"error,omitempty"`
}

// AgentRunStep is a persisted workflow or harness step within an AgentRun.
type AgentRunStep struct {
	ID            string     `json:"id"`
	RunID         string     `json:"run_id"`
	Index         int        `json:"index"`
	Key           string     `json:"key"`
	Kind          string     `json:"kind"`
	Status        string     `json:"status"`
	InputSummary  string     `json:"input_summary,omitempty"`
	OutputSummary string     `json:"output_summary,omitempty"`
	Error         string     `json:"error,omitempty"`
	StartedAt     time.Time  `json:"started_at"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
}

// MemoryEvent is an observable memory lifecycle event associated with an agent run.
type MemoryEvent struct {
	ID        string    `json:"id"`
	SpaceID   string    `json:"space_id"`
	RunID     string    `json:"run_id"`
	Kind      string    `json:"kind"`
	Payload   string    `json:"payload,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// SpaceObservability groups the execution history shown by the space debugger.
type SpaceObservability struct {
	Runs         []AgentRun     `json:"runs"`
	Steps        []AgentRunStep `json:"steps"`
	ModelCalls   []ModelCall    `json:"model_calls"`
	MemoryEvents []MemoryEvent  `json:"memory_events"`
}

type ModelCall struct {
	ID           string    `json:"id"`
	Scope        string    `json:"scope"`
	RefID        string    `json:"ref_id"`
	SpaceID      string    `json:"space_id,omitempty"`
	RunID        string    `json:"run_id,omitempty"`
	Step         string    `json:"step"`
	Model        string    `json:"model"`
	InputJSON    string    `json:"input_json"`
	OutputText   string    `json:"output_text"`
	ResponseJSON string    `json:"response_json"`
	PromptTokens int       `json:"prompt_tokens"`
	OutputTokens int       `json:"output_tokens"`
	TotalTokens  int       `json:"total_tokens"`
	LatencyMS    int64     `json:"latency_ms"`
	ErrorMessage string    `json:"error_message"`
	CreatedAt    time.Time `json:"created_at"`
}

type CustomSkill struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	Category         string    `json:"category"`
	InvocationPrompt string    `json:"invocation_prompt"`
	Content          string    `json:"content"`
	Source           string    `json:"source"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type CreateCustomSkillInput struct {
	Name             string
	Title            string
	Description      string
	Category         string
	InvocationPrompt string
	Content          string
}
