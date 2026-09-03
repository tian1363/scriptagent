package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/tian1363/scriptagent/internal/telemetry"
	"github.com/tian1363/scriptagent/internal/userctx"
)

type Agent interface {
	Run(ctx context.Context, run RunContext, job Job, progress Progress) (ScriptResult, error)
}

type Progress func(status, message string)

type Runner struct {
	store *Store
	agent Agent
}

func NewRunner(store *Store, agent Agent) *Runner {
	return &Runner{store: store, agent: agent}
}

func (r *Runner) Enqueue(jobID string) {
	go r.run(jobID)
}

func (r *Runner) ResumeUnfinished() {
	unfinished, err := r.store.ListUnfinishedJobs()
	if err != nil {
		log.Printf("list unfinished jobs: %v", err)
		return
	}
	for _, job := range unfinished {
		if err := r.store.FailActiveAgentRuns(job.ID, "服务重启后恢复任务"); err != nil {
			log.Printf("close stale agent run for job %s: %v", job.ID, err)
			continue
		}
		r.Enqueue(job.ID)
	}
}

func (r *Runner) run(jobID string) {
	job, err := r.store.GetJob(jobID)
	if err != nil {
		log.Printf("load job %s: %v", jobID, err)
		return
	}
	ctx := userctx.WithUser(context.Background(), userctx.User{ID: r.store.ResourceOwner("job", job.ID)})
	runContext := RunContext{Scope: "job", RefID: job.ID, SpaceID: job.SpaceID, JobID: job.ID}
	var agentRun *AgentRun
	if job.SpaceID != "" {
		agentRun, err = r.store.StartAgentRun(*job)
		if err != nil {
			log.Printf("start agent run for job %s: %v", job.ID, err)
			_ = r.store.UpdateStatus(job.ID, StatusFailed, err.Error())
			return
		}
		runContext.RunID = agentRun.ID
	}
	traceInput, _ := json.Marshal(map[string]any{"job_id": job.ID, "title": job.Title, "industry": job.Industry, "fission_count": job.FissionCount})
	ctx, runSpan := telemetry.StartAgentRun(ctx, telemetry.RunAttributes{
		Name: "script-generation-workflow", RunID: runContext.RunID, SpaceID: job.SpaceID,
		JobID: job.ID, SessionID: job.SpaceID, Input: string(traceInput),
	})
	runSpanEnded := false
	var activeStep *AgentRunStep
	activeStepSummary := ""
	stepIndex := 0
	finishStep := func(status, errorMessage string) {
		if activeStep == nil {
			return
		}
		if err := r.store.FinishAgentStep(activeStep.ID, status, activeStepSummary, errorMessage); err != nil {
			log.Printf("finish agent step %s: %v", activeStep.ID, err)
		}
		activeStep = nil
		activeStepSummary = ""
	}
	startStep := func(key, message string) {
		if agentRun == nil || key == "" {
			return
		}
		finishStep("completed", "")
		stepIndex++
		step, err := r.store.StartAgentStep(agentRun.ID, stepIndex, key, "workflow", message)
		if err != nil {
			log.Printf("start agent step for run %s: %v", agentRun.ID, err)
			return
		}
		activeStep = step
		activeStepSummary = message
	}
	finishRun := func(status, errorMessage string) {
		if agentRun != nil {
			if err := r.store.FinishAgentRun(agentRun.ID, status, errorMessage); err != nil {
				log.Printf("finish agent run %s: %v", agentRun.ID, err)
			}
		}
		if !runSpanEnded {
			output, _ := json.Marshal(map[string]string{"job_id": job.ID, "status": status})
			var runErr error
			if errorMessage != "" {
				runErr = errors.New(errorMessage)
			}
			telemetry.EndAgentRun(runSpan, string(output), runErr)
			runSpanEnded = true
		}
	}
	progress := func(status, message string) {
		if status != "" {
			if activeStep == nil || activeStep.Key != status {
				startStep(status, message)
			} else if message != "" {
				activeStepSummary = message
			}
			if err := r.store.UpdateStatus(job.ID, status, ""); err != nil {
				log.Printf("update job status %s: %v", job.ID, err)
			}
		}
		if message != "" {
			if err := r.store.AppendLog(job.ID, message); err != nil {
				log.Printf("append job log %s: %v", job.ID, err)
			}
		}
	}
	if err := r.store.UpdateStatus(job.ID, StatusRunning, ""); err != nil {
		log.Printf("set job running %s: %v", job.ID, err)
		finishRun("failed", err.Error())
		return
	}
	progress(StatusRunning, "任务开始运行。")

	result, err := r.agent.Run(ctx, runContext, *job, progress)
	if err != nil {
		_ = r.store.AppendLog(job.ID, "任务失败："+err.Error())
		_ = r.store.UpdateStatus(job.ID, StatusFailed, err.Error())
		finishStep("failed", err.Error())
		finishRun("failed", err.Error())
		return
	}
	progress(StatusValidating, "脚本结果已生成，正在保存。")
	if err := r.store.SaveResult(job.ID, result); err != nil {
		log.Printf("save job result %s: %v", job.ID, err)
		_ = r.store.AppendLog(job.ID, "保存结果失败："+err.Error())
		_ = r.store.UpdateStatus(job.ID, StatusFailed, err.Error())
		finishStep("failed", err.Error())
		finishRun("failed", err.Error())
		return
	}
	_ = r.store.AppendLog(job.ID, "任务完成。")
	finishStep("completed", "")
	finishRun("completed", "")
}
