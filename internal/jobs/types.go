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
	AnalysisMarkdown   string    `json:"analysis_markdown,omitempty"`
	ReplicaScriptJSON  string    `json:"replica_script_json,omitempty"`
	FissionScriptsJSON string    `json:"fission_scripts_json,omitempty"`
	CreatiBIResultJSON string    `json:"creatibi_result_json,omitempty"`
	ErrorMessage       string    `json:"error_message,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
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
}

type ScriptResult struct {
	AnalysisMarkdown   string
	ReplicaScriptJSON  string
	FissionScriptsJSON string
}
