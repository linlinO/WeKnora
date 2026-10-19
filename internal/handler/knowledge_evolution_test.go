package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func evolutionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "evolution.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(
		&types.AnswerFeedback{}, &types.KnowledgeEvolutionProposal{}, &types.KnowledgeVersionLink{},
		&types.KnowledgeRegressionCase{}, &types.KnowledgeRegressionRun{}, &types.KnowledgeRegressionResult{},
		&types.Knowledge{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func evolutionContext(c *gin.Context) {
	c.Request = c.Request.WithContext(types.WithCaller(c.Request.Context(), types.Caller{TenantID: 42, UserID: "reviewer"}))
}

func TestKnowledgeEvolutionDraftRegressionAndPublish(t *testing.T) {
	db := evolutionTestDB(t)
	feedback := &types.AnswerFeedback{ID: "feedback-1", TenantID: 42, UserID: "u", MessageID: "m", Question: "服务器负载高怎么定位？", Answer: "先检查 CPU 和 IO。", Reason: "内容有错误", Rating: types.AnswerFeedbackNotUseful, Status: types.AnswerFeedbackPending}
	if err := db.Create(feedback).Error; err != nil {
		t.Fatal(err)
	}
	knowledge := &correctionKnowledgeServiceStub{}
	h := NewKnowledgeEvolutionHandler(db, nil, knowledge, correctionKBServiceStub{})
	h.runCase = func(ctx context.Context, p *types.KnowledgeEvolutionProposal, a *types.CustomAgent, q, kind string) (replayAnswer, error) {
		return replayAnswer{Answer: p.CandidateAnswer, References: []*types.SearchResult{{KnowledgeID: "preview:" + p.ID}}}, nil
	}
	h.judge = func(context.Context, string, string, string, string) (bool, string, error) {
		return true, "事实一致", nil
	}
	r := gin.New()
	r.Use(evolutionContext)
	r.POST("/feedback/:feedback_id/draft", h.CreateDraft)
	r.POST("/:id/regression", h.RunRegression)
	r.POST("/:id/draft", h.SaveDraft)
	r.POST("/:id/publish", h.Publish)

	call := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)
		if resp.Code == 202 {
			var queued struct {
				Data types.KnowledgeRegressionRun `json:"data"`
			}
			json.Unmarshal(resp.Body.Bytes(), &queued)
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				var run types.KnowledgeRegressionRun
				db.First(&run, "id=?", queued.Data.ID)
				if run.Status != "pending" {
					resp = httptest.NewRecorder()
					resp.WriteHeader(200)
					json.NewEncoder(resp).Encode(map[string]any{"data": run})
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
		}
		return resp
	}
	resp := call(http.MethodPost, "/feedback/feedback-1/draft", `{"knowledge_base_id":"kb-1"}`)
	if resp.Code != http.StatusCreated {
		t.Fatalf("draft status=%d body=%s", resp.Code, resp.Body.String())
	}
	var draft struct {
		Data types.KnowledgeEvolutionProposal `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &draft); err != nil {
		t.Fatal(err)
	}
	if draft.Data.Status != types.EvolutionProposalDraft || draft.Data.CandidateAnswer == "" {
		t.Fatalf("unexpected draft: %+v", draft.Data)
	}
	resp = call(http.MethodPost, "/"+draft.Data.ID+"/regression", `{"answer":"先检查 CPU 和 IO。增加监控。"}`)
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), `"status":"passed"`) {
		t.Fatalf("regression status=%d body=%s", resp.Code, resp.Body.String())
	}
	var tested types.KnowledgeEvolutionProposal
	if err := db.First(&tested, "id = ?", draft.Data.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tested.CandidateAnswer, "增加监控") {
		t.Fatalf("regression did not persist edited answer: %q", tested.CandidateAnswer)
	}
	for _, body := range []string{`{"answer":"正确答案：先检查 CPU 和 IO。"}`, `{"answer":"正确答案：先检查 CPU 和 IO。","force":true}`} {
		resp = call(http.MethodPost, "/"+draft.Data.ID+"/publish", body)
		if resp.Code != http.StatusConflict || knowledge.payload != nil {
			t.Fatalf("untested edit accepted: %d %s", resp.Code, resp.Body)
		}
	}
	resp = call(http.MethodPost, "/"+draft.Data.ID+"/draft", `{"answer":"先检查 CPU 和 IO。增加监控。记录趋势。"}`)
	if resp.Code != 200 {
		t.Fatalf("save: %d %s", resp.Code, resp.Body)
	}
	if err := db.First(&tested, "id = ?", draft.Data.ID).Error; err != nil {
		t.Fatal(err)
	}
	if tested.RegressionRunID != "" || tested.Status != types.EvolutionProposalDraft {
		t.Fatalf("stale gate remains: %+v", tested)
	}
	resp = call(http.MethodPost, "/"+draft.Data.ID+"/publish", `{}`)
	if resp.Code != 409 {
		t.Fatalf("saved untested draft published: %d", resp.Code)
	}
	resp = call(http.MethodPost, "/"+draft.Data.ID+"/regression", `{"answer":"先检查 CPU 和 IO。增加监控。记录趋势。"}`)
	if resp.Code != 200 || !strings.Contains(resp.Body.String(), `"status":"passed"`) {
		t.Fatalf("retest: %d %s", resp.Code, resp.Body)
	}
	resp = call(http.MethodPost, "/"+draft.Data.ID+"/publish", `{"answer":"先检查 CPU 和 IO。增加监控。记录趋势。"}`)
	if resp.Code != http.StatusOK {
		t.Fatalf("publish status=%d body=%s", resp.Code, resp.Body.String())
	}
	var saved types.KnowledgeEvolutionProposal
	if err := db.First(&saved, "id = ?", draft.Data.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Status != types.EvolutionProposalPublished || saved.CorrectionKnowledgeID != "knowledge-1" {
		t.Fatalf("unexpected published proposal: %+v", saved)
	}
	var updated types.AnswerFeedback
	if err := db.First(&updated, "id = ?", feedback.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updated.Status != types.AnswerFeedbackResolved || updated.CorrectionKnowledgeID != "knowledge-1" {
		t.Fatalf("feedback was not resolved: %+v", updated)
	}
	for _, action := range []string{"draft", "regression", "publish"} {
		resp = call(http.MethodPost, "/"+draft.Data.ID+"/"+action, `{"answer":"changed"}`)
		if resp.Code != 409 {
			t.Fatalf("published version modified through %s: %d", action, resp.Code)
		}
	}
}

func TestKnowledgeEvolutionDuplicateQuestionIsBlockedUntilExplicitForce(t *testing.T) {
	db := evolutionTestDB(t)
	for i, answer := range []string{"答案一", "答案二"} {
		feedback := &types.AnswerFeedback{ID: "feedback-" + string(rune('1'+i)), TenantID: 42, UserID: "u", MessageID: "m" + string(rune('1'+i)), Question: "同一个问题", Answer: answer, Rating: types.AnswerFeedbackNotUseful, Status: types.AnswerFeedbackPending}
		if err := db.Create(feedback).Error; err != nil {
			t.Fatal(err)
		}
	}
	h := NewKnowledgeEvolutionHandler(db, nil, &correctionKnowledgeServiceStub{}, correctionKBServiceStub{})
	r := gin.New()
	r.Use(evolutionContext)
	r.POST("/feedback/:feedback_id/draft", h.CreateDraft)
	call := func(id string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/feedback/"+id+"/draft", strings.NewReader(`{"knowledge_base_id":"kb-1"}`))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)
		return resp
	}
	if call("feedback-1").Code != http.StatusCreated || call("feedback-2").Code != http.StatusCreated {
		t.Fatal("draft creation failed")
	}
	var count int64
	if err := db.Model(&types.KnowledgeEvolutionProposal{}).Where("conflict_status = ?", types.EvolutionConflictConfirmed).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("conflict count=%d, want 1", count)
	}
}
