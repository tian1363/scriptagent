package jobs

import (
	"context"
	"log"

	"github.com/tian1363/scriptagent/internal/userctx"
)

type Agent interface {
	Run(ctx context.Context, job Job, progress Progress) (ScriptResult, error)
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
		r.Enqueue(job.ID)
	}
}

func (r *Runner) run(jobID string) {
	job, err := r.store.GetJob(jobID)
	if err != nil {
		log.Printf("load job %s: %v", jobID, err)
		return
	}
	ctx := userctx.WithUser(context.Background(), userctx.User{ID: job.UserID})
	progress := func(status, message string) {
		if status != "" {
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
		return
	}
	progress(StatusRunning, "任务开始运行。")

	result, err := r.agent.Run(ctx, *job, progress)
	if err != nil {
		_ = r.store.AppendLog(job.ID, "任务失败："+err.Error())
		_ = r.store.UpdateStatus(job.ID, StatusFailed, err.Error())
		return
	}
	progress(StatusValidating, "脚本结果已生成，正在保存。")
	if err := r.store.SaveResult(job.ID, result); err != nil {
		log.Printf("save job result %s: %v", job.ID, err)
		_ = r.store.AppendLog(job.ID, "保存结果失败："+err.Error())
		_ = r.store.UpdateStatus(job.ID, StatusFailed, err.Error())
		return
	}
	_ = r.store.AppendLog(job.ID, "任务完成。")
}
