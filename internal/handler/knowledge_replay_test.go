package handler

import (
	"context"
	"errors"
	"github.com/Tencent/WeKnora/internal/types"
	"testing"
	"time"
)

func TestReplayChecksActualAnswerAndReferences(t *testing.T) {
	for _, tc := range []struct {
		name, answer, ref, forbidden string
		truncated                    bool
		judgeErr                     bool
		pass                         bool
	}{
		{name: "new knowledge and correct answer", answer: "使用新方案", ref: "new", pass: true},
		{name: "missing new source", answer: "使用新方案", ref: "other"},
		{name: "old source", answer: "使用新方案", ref: "old"},
		{name: "old conclusion remains", answer: "仍用旧命令", ref: "new", forbidden: "旧命令"},
		{name: "truncated", answer: "使用新方案", ref: "new", truncated: true},
		{name: "judge unavailable", answer: "使用新方案", ref: "new", judgeErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := evolutionTestDB(t)
			h := NewKnowledgeEvolutionHandler(db, nil, nil, nil)
			p := types.KnowledgeEvolutionProposal{ID: "p", TenantID: 42, CorrectionKnowledgeID: "new", SupersedesKnowledgeID: "old", CandidateAnswer: "使用新方案", Status: "published"}
			db.Create(&p)
			run := types.KnowledgeRegressionRun{ID: "run", TenantID: 42, ProposalID: p.ID, Kind: "published", Total: 1, Status: "pending", StartedAt: time.Now()}
			db.Create(&run)
			h.runCase = func(context.Context, *types.KnowledgeEvolutionProposal, *types.CustomAgent, string, string) (replayAnswer, error) {
				return replayAnswer{Answer: tc.answer, Truncated: tc.truncated, References: []*types.SearchResult{{KnowledgeID: tc.ref}}}, nil
			}
			h.judge = func(context.Context, string, string, string, string) (bool, string, error) {
				if tc.judgeErr {
					return false, "", errors.New("offline")
				}
				return true, "一致", nil
			}
			input := replayInput{Answer: p.CandidateAnswer, Questions: []string{"问题"}}
			if tc.forbidden != "" {
				input.Forbidden = []string{tc.forbidden}
			}
			h.executeReplay(context.Background(), p, types.CustomAgent{}, run, input)
			db.First(&run, "id=?", "run")
			if (run.Status == "passed") != tc.pass {
				t.Fatalf("status=%s reason=%s", run.Status, run.FailureSummary)
			}
			var result types.KnowledgeRegressionResult
			db.First(&result, "run_id=?", run.ID)
			if result.Answer != tc.answer {
				t.Fatal("did not record actual model output")
			}
			var current types.KnowledgeEvolutionProposal
			db.First(&current, "id=?", p.ID)
			if current.Status != "published" {
				t.Fatal("verification changed published version")
			}
		})
	}
}

func TestReplayEditedDraftCannotRestoreOldPublishGate(t *testing.T) {
	db := evolutionTestDB(t)
	h := NewKnowledgeEvolutionHandler(db, nil, nil, nil)
	p := types.KnowledgeEvolutionProposal{ID: "p", TenantID: 42, CandidateAnswer: "旧待测答案", Status: "testing", RegressionRunID: "run"}
	db.Create(&p)
	run := types.KnowledgeRegressionRun{ID: "run", TenantID: 42, ProposalID: p.ID, Kind: "preview", Total: 1, Status: "pending"}
	db.Create(&run)
	h.runCase = func(context.Context, *types.KnowledgeEvolutionProposal, *types.CustomAgent, string, string) (replayAnswer, error) {
		db.Model(&p).Updates(map[string]any{"candidate_answer": "测试期间已修改", "regression_run_id": "", "status": "draft"})
		return replayAnswer{Answer: "旧待测答案", References: []*types.SearchResult{{KnowledgeID: "preview:p"}}}, nil
	}
	h.judge = func(context.Context, string, string, string, string) (bool, string, error) {
		return true, "一致", nil
	}
	h.executeReplay(context.Background(), p, types.CustomAgent{}, run, replayInput{Answer: "旧待测答案", Questions: []string{"问题"}})
	var current types.KnowledgeEvolutionProposal
	db.First(&current, "id=?", "p")
	if current.Status != "draft" || current.RegressionRunID != "" {
		t.Fatal("stale result restored publish gate")
	}
}

func TestReplayClarificationIsAValidBehaviorButNotPublishApproval(t *testing.T) {
	for _, kind := range []string{"clarification", "unavailable", "handoff", ""} {
		t.Run(kind, func(t *testing.T) {
			db := evolutionTestDB(t)
			h := NewKnowledgeEvolutionHandler(db, nil, nil, nil)
			p := types.KnowledgeEvolutionProposal{ID: "p", TenantID: 42, Status: "testing", RegressionRunID: "r", CandidateAnswer: "确认数据库后按对应方案处理"}
			db.Create(&p)
			run := types.KnowledgeRegressionRun{ID: "r", TenantID: 42, ProposalID: p.ID, Kind: "preview", Total: 1, Status: "pending"}
			db.Create(&run)
			h.runCase = func(_ context.Context, _ *types.KnowledgeEvolutionProposal, _ *types.CustomAgent, q, _ string) (replayAnswer, error) {
				if q != "升级失败怎么办\n已确认的场景补充：KE" {
					t.Fatalf("missing scenario: %s", q)
				}
				return replayAnswer{Answer: "请补充报错原文", ReadinessKind: kind}, nil
			}
			h.judge = func(context.Context, string, string, string, string) (bool, string, error) {
				t.Fatal("clarification must not be judged as a repair answer")
				return false, "", nil
			}
			h.executeReplay(context.Background(), p, types.CustomAgent{}, run, replayInput{Answer: p.CandidateAnswer, Questions: []string{"升级失败怎么办"}, Context: "KE", ExpectedBehavior: "clarification", Required: []string{"报错"}})
			db.First(&run, "id=?", "r")
			if (run.Status == "passed") != (kind == "clarification") {
				t.Fatalf("unexpected status %s: %s", run.Status, run.FailureSummary)
			}
			db.First(&p, "id=?", "p")
			if p.Status == "ready" {
				t.Fatal("clarification opened publish gate")
			}
		})
	}
}
