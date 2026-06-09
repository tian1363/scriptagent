package jobs

import "time"

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
	AnalysisMarkdown   string    `json:"analysis_markdown,omitempty"`
	ReplicaScriptJSON  string    `json:"replica_script_json,omitempty"`
	FissionScriptsJSON string    `json:"fission_scripts_json,omitempty"`
	CreatiBIResultJSON string    `json:"creatibi_result_json,omitempty"`
	ErrorMessage       string    `json:"error_message,omitempty"`
	RunLog             string    `json:"run_log,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type Product struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	MDPath    string    `json:"md_path"`
	MDName    string    `json:"md_name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
}

type CreateProductInput struct {
	Title  string
	MDPath string
	MDName string
}

type ModelSettings struct {
	APIKey    string    `json:"-"`
	Endpoint  string    `json:"endpoint"`
	Model     string    `json:"model"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PublicModelSettings struct {
	Configured bool      `json:"configured"`
	Source     string    `json:"source"`
	APIKeyMask string    `json:"api_key_mask,omitempty"`
	Endpoint   string    `json:"endpoint"`
	Model      string    `json:"model"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
}

type ScriptResult struct {
	AnalysisMarkdown   string
	ReplicaScriptJSON  string
	FissionScriptsJSON string
}

type ChatConversation struct {
	ID               string    `json:"id"`
	Title            string    `json:"title"`
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
	Conversation ChatConversation  `json:"conversation"`
	Messages     []ChatMessage     `json:"messages"`
	Citations    []ProductCitation `json:"citations,omitempty"`
}

type ModelCall struct {
	ID           string    `json:"id"`
	Scope        string    `json:"scope"`
	RefID        string    `json:"ref_id"`
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
