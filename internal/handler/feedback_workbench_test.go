package handler

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

func TestAnswerFeedbackWorkbenchFiltersDetailAndTriage(t *testing.T) {
	db := answerFeedbackTestDB(t)
	if err := db.AutoMigrate(&types.Session{}, &types.KnowledgeEvolutionProposal{}, &types.KnowledgeRegressionRun{}); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 10, 10, 2, 0, 0, 0, time.UTC)
	for _, row := range []types.AnswerFeedback{
		{ID: "a", TenantID: 42, UserID: "web-user", Source: "web", MessageID: "a", Question: "CPU 定位", Answer: "完整回答", Status: "pending", Rating: "not_useful", CreatedAt: stamp},
		{ID: "b", TenantID: 42, UserID: "wecom:u", ChannelID: "ch", Source: "wecom", MessageID: "b", Question: "CPU 排查", Answer: "original", Status: "correcting", Rating: "not_useful", CreatedAt: stamp},
		{ID: "c", TenantID: 42, UserID: "web-user", Source: "web", MessageID: "c", Question: "CPU useful", Status: "pending", Rating: "useful", CreatedAt: stamp},
		{ID: "d", TenantID: 42, UserID: "web-user", Source: "web", MessageID: "d", Question: "CPU old", Status: "resolved", Rating: "not_useful", CreatedAt: stamp.AddDate(0, 0, -10)},
		{ID: "other", TenantID: 99, UserID: "wecom:u", ChannelID: "ch", Source: "wecom", MessageID: "other", Question: "CPU private", Status: "pending", Rating: "not_useful", CreatedAt: stamp},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&types.User{ID: "web-user", Username: "测试用户", Email: "workbench@example.com"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&types.IMFeedbackUserName{TenantID: 42, ChannelID: "ch", UserID: "wecom:u", Name: "方绍羽"}).Error; err != nil {
		t.Fatal(err)
	}
	h := NewAnswerFeedbackHandler(db, nil, nil, nil)
	evolution := NewKnowledgeEvolutionHandler(db, nil, nil, correctionKBServiceStub{})
	r := gin.New()
	r.Use(evolutionContext)
	r.GET("/feedback", h.List)
	r.GET("/feedback/:id", h.Detail)
	r.POST("/feedback/:id/review", h.Review)
	r.POST("/draft/:feedback_id", evolution.CreateDraft)
	call := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)
		return resp
	}
	for _, tc := range []struct {
		query string
		total int
		ids   string
	}{
		{"status=all&rating=not_useful&page_size=1&page=1", 3, "b"},
		{"status=all&rating=not_useful&page_size=1&page=2", 3, "a"},
		{"status=pending&rating=not_useful", 1, "a"},
		{"status=correcting&rating=not_useful", 1, "b"},
		{"rating=useful", 1, "c"},
		{"status=all&source=wecom", 1, "b"},
		{"status=all&reporter=方绍羽", 1, "b"},
		{"rating=not_useful&reporter=测试用户", 1, "a"},
		{"status=all&keyword=useful", 1, "c"},
		{"status=all&rating=not_useful&from=2026-10-10T00:00:00Z&to=2026-10-11T00:00:00Z", 2, "ba"},
	} {
		resp := call("GET", "/feedback?"+tc.query, "")
		var got struct {
			Data  []types.AnswerFeedback `json:"data"`
			Total int                    `json:"total"`
		}
		if resp.Code != 200 || json.Unmarshal(resp.Body.Bytes(), &got) != nil {
			t.Fatalf("list: %d %s", resp.Code, resp.Body)
		}
		ids := ""
		for _, row := range got.Data {
			ids += row.ID
			if row.Answer != "" {
				t.Fatal("list fetched full answer")
			}
		}
		if got.Total != tc.total || ids != tc.ids {
			t.Fatalf("%s => total=%d ids=%s", tc.query, got.Total, ids)
		}
	}
	if resp := call("GET", "/feedback?from=invalid", ""); resp.Code != 400 {
		t.Fatal("invalid date accepted")
	}
	for _, status := range []string{"pending", "correcting"} {
		resp := call("GET", "/feedback?status="+status+"&rating=not_useful&from=2026-10-10T00:00:00Z", "")
		var got struct {
			Counts map[string]int64 `json:"status_counts"`
		}
		if json.Unmarshal(resp.Body.Bytes(), &got) != nil || got.Counts["all"] != 2 || got.Counts["pending"] != 1 || got.Counts["correcting"] != 1 || got.Counts["resolved"] != 0 {
			t.Fatalf("counts must be disjoint, tenant-scoped and independent of selected tab: %s", resp.Body)
		}
	}
	if resp := call("GET", "/feedback/a", ""); resp.Code != 200 || !strings.Contains(resp.Body.String(), "完整回答") {
		t.Fatalf("detail: %s", resp.Body)
	}
	if resp := call("GET", "/feedback/other", ""); resp.Code != 404 {
		t.Fatal("cross tenant detail exposed")
	}
	if resp := call("POST", "/draft/c", `{"knowledge_base_id":"kb-1"}`); resp.Code != 409 {
		t.Fatal("useful feedback automatically corrected")
	}
	if resp := call("POST", "/feedback/a/review", `{"action":"acknowledge"}`); resp.Code != 400 {
		t.Fatal("negative vote acknowledged")
	}
	for _, action := range []string{"acknowledge", "correct"} {
		resp := call("POST", "/feedback/c/review", `{"action":"`+action+`"}`)
		if resp.Code != 200 || !strings.Contains(resp.Body.String(), `"rating":"useful"`) {
			t.Fatalf("triage: %d %s", resp.Code, resp.Body)
		}
	}
	var saved types.AnswerFeedback
	if err := db.First(&saved, "id = ?", "c").Error; err != nil {
		t.Fatal(err)
	}
	if saved.Status != "correcting" || saved.Rating != "useful" {
		t.Fatalf("original vote changed: %+v", saved)
	}
}
