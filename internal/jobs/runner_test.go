package jobs

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

type recordingAgent struct {
	run    RunContext
	result ScriptResult
	err    error
}

func (a *recordingAgent) Run(_ context.Context, run RunContext, _ Job, _ Progress) (ScriptResult, error) {
	a.run = run
	return a.result, a.err
}

func TestRunnerPersistsCompletedAgentRun(t *testing.T) {
	store, job := createRunnerTestJob(t)
	defer store.Close()
	agent := &recordingAgent{result: ScriptResult{AnalysisMarkdown: "analysis", ReplicaScriptJSON: `{}`, FissionScriptsJSON: `[]`}}
	runner := NewRunner(store, agent)

	runner.run(job.ID)

	runs, err := store.ListAgentRuns(job.SpaceID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected one run, got %d", len(runs))
	}
	if runs[0].Status != "completed" || runs[0].FinishedAt == nil {
		t.Fatalf("expected completed run, got %+v", runs[0])
	}
	if agent.run.RunID != runs[0].ID || agent.run.SpaceID != job.SpaceID || agent.run.JobID != job.ID {
		t.Fatalf("agent received incorrect run context: %+v", agent.run)
	}
	steps, err := store.ListAgentRunSteps(job.SpaceID, runs[0].ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected running and validating steps, got %+v", steps)
	}
	for _, step := range steps {
		if step.Status != "completed" || step.FinishedAt == nil {
			t.Fatalf("expected completed persisted step, got %+v", step)
		}
	}
}

func TestRunnerPersistsFailedAgentRun(t *testing.T) {
	store, job := createRunnerTestJob(t)
	defer store.Close()
	agent := &recordingAgent{err: errors.New("model unavailable")}
	runner := NewRunner(store, agent)

	runner.run(job.ID)

	runs, err := store.ListAgentRuns(job.SpaceID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].Status != "failed" || runs[0].Error != "model unavailable" || runs[0].FinishedAt == nil {
		t.Fatalf("expected failed run with error, got %+v", runs)
	}
	steps, err := store.ListAgentRunSteps(job.SpaceID, runs[0].ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 || steps[0].Status != "failed" || steps[0].Error != "model unavailable" {
		t.Fatalf("expected failed persisted step, got %+v", steps)
	}
}

func createRunnerTestJob(t *testing.T) (*Store, *Job) {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), "runner.db"))
	if err != nil {
		t.Fatal(err)
	}
	space, err := store.CreateSpace(CreateSpaceInput{Title: "Runtime test"})
	if err != nil {
		store.Close()
		t.Fatal(err)
	}
	job, err := store.CreateJob(CreateJobInput{Title: "Job", Industry: "game", FissionCount: 1, SpaceID: space.ID})
	if err != nil {
		store.Close()
		t.Fatal(err)
	}
	return store, job
}
