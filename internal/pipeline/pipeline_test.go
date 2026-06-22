package pipeline

import (
	"testing"
)

func TestSDDPipeline(t *testing.T) {
	p := SDDPipeline()
	if p.ID != "sdd" {
		t.Errorf("expected ID 'sdd', got %q", p.ID)
	}
	if len(p.Stages) != 4 {
		t.Errorf("expected 4 stages, got %d", len(p.Stages))
	}
	if p.Stages[0].ID != "spec" {
		t.Errorf("expected first stage 'spec', got %q", p.Stages[0].ID)
	}
	if len(p.Stages[1].DependsOn) != 1 || p.Stages[1].DependsOn[0] != "spec" {
		t.Error("design stage should depend on spec")
	}
}

func TestCodeReviewPipeline(t *testing.T) {
	p := CodeReviewPipeline()
	if p.ID != "code-review" {
		t.Errorf("expected ID 'code-review', got %q", p.ID)
	}
	if len(p.Stages) != 4 {
		t.Errorf("expected 4 stages, got %d", len(p.Stages))
	}
	for _, s := range p.Stages[:3] {
		if !s.Parallel {
			t.Errorf("stage %q should be parallel", s.ID)
		}
	}
}

func TestAllPipelines(t *testing.T) {
	pipelines := AllPipelines()
	if len(pipelines) < 3 {
		t.Errorf("expected at least 3 pipelines, got %d", len(pipelines))
	}
	ids := make(map[string]bool)
	for _, p := range pipelines {
		if ids[p.ID] {
			t.Errorf("duplicate pipeline ID: %q", p.ID)
		}
		ids[p.ID] = true
	}
}

func TestPipelineRun_InitialState(t *testing.T) {
	run := &PipelineRun{
		PipelineID:   "test",
		Status:       PipelinePending,
		StageResults: make(map[string]*StageResult),
	}
	if run.Status != PipelinePending {
		t.Errorf("expected pending status, got %q", run.Status)
	}
}

func TestStageResult_Failed(t *testing.T) {
	sr := &StageResult{
		StageID: "test",
		Status:  StageFailed,
		Error:   "something went wrong",
	}
	if sr.Status != StageFailed {
		t.Error("expected failed status")
	}
	if sr.Error == "" {
		t.Error("expected error message")
	}
}
