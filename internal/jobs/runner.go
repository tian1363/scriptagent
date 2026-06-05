package jobs

import (
	"context"
	"log"
)

type Agent interface {
	Run(ctx context.Context, job Job) (ScriptResult, error)
}

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
	ctx := context.Background()
	job, err := r.store.GetJob(jobID)
	if err != nil {
		log.Printf("load job %s: %v", jobID, err)
		return
	}
	if err := r.store.UpdateStatus(job.ID, StatusRunning, ""); err != nil {
		log.Printf("set job running %s: %v", job.ID, err)
		return
	}

	result, err := r.agent.Run(ctx, *job)
	if err != nil {
		_ = r.store.UpdateStatus(job.ID, StatusFailed, err.Error())
		return
	}
	if err := r.store.SaveResult(job.ID, result); err != nil {
		log.Printf("save job result %s: %v", job.ID, err)
		_ = r.store.UpdateStatus(job.ID, StatusFailed, err.Error())
	}
}
