package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type feedbackMessageStub struct {
	interfaces.MessageService
	message *types.Message
	err     error
}

func (s feedbackMessageStub) GetMessage(context.Context, string, string) (*types.Message, error) {
	return s.message, s.err
}

func answerFeedbackTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "feedback.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	for _, file := range []string{"000028_answer_feedback.up.sql", "000029_im_answer_feedback.up.sql", "000030_feedback_user_names.up.sql"} {
		sql, err := os.ReadFile(filepath.Join("../../migrations/sqlite", file))
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Exec(string(sql)).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.AutoMigrate(&types.Message{}, &types.User{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestAnswerFeedbackWebCreateReadListAndAccess(t *testing.T) {
	db := answerFeedbackTestDB(t)
	u := &types.Message{SessionID: "s", RequestID: "turn", Role: "user", Content: "original question"}
	a := &types.Message{SessionID: "s", RequestID: "turn", Role: "assistant", Content: "original answer", IsCompleted: true}
	if err := db.Create(u).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(a).Error; err != nil {
		t.Fatal(err)
	}
	h := NewAnswerFeedbackHandler(db, feedbackMessageStub{message: a}, nil, nil)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(types.WithCaller(c.Request.Context(), types.Caller{TenantID: 42, UserID: "user"}))
		c.Next()
	})
	r.POST("/feedback/:session_id/:message_id", h.Create)
	r.GET("/feedback/:session_id/:message_id", h.Get)
	r.GET("/admin", h.List)
	call := func(method, path, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		r.ServeHTTP(response, request)
		return response
	}
	path := "/feedback/s/" + a.ID
	response := call(http.MethodPost, path, `{"rating":"not_useful","question":"forged","answer":"forged","reason":"wrong"}`)
	if response.Code != 200 || strings.Contains(response.Body.String(), "forged") {
		t.Fatalf("%d %s", response.Code, response.Body)
	}
	response = call(http.MethodGet, path, "")
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"rating":"not_useful"`) {
		t.Fatalf("GET: %s", response.Body)
	}
	response = call(http.MethodGet, "/admin?source=wecom", "")
	if !strings.Contains(response.Body.String(), `"total":0`) {
		t.Fatalf("source filter: %s", response.Body)
	}
	response = call(http.MethodPost, path, `{"rating":"cancel"}`)
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"status":"withdrawn"`) {
		t.Fatalf("cancel: %s", response.Body)
	}
	response = call(http.MethodGet, "/admin", "")
	if !strings.Contains(response.Body.String(), `"total":0`) {
		t.Fatal("withdrawn vote still pending")
	}
	h.messageService = feedbackMessageStub{err: errors.New("denied")}
	if got := call(http.MethodPost, path, `{"rating":"useful"}`); got.Code != 404 {
		t.Fatal("denied session accepted")
	}
	if got := call(http.MethodGet, path, ""); got.Code != 404 {
		t.Fatal("denied session exposed vote")
	}
}

func TestAnswerFeedbackReporterNamesAndIsolation(t *testing.T) {
	db := answerFeedbackTestDB(t)
	if err := db.Create(&types.User{ID: "web-user", Username: "Web name", TenantID: 99, Email: "test@example.com"}).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range []types.AnswerFeedback{
		{ID: "w", TenantID: 42, UserID: "web-user", Source: "web", MessageID: "w", Status: "pending", Rating: "useful"},
		{ID: "im1", TenantID: 42, UserID: "wecom:same", Source: "wecom", ChannelID: "a", MessageID: "m1", Status: "pending", Rating: "useful"},
		{ID: "im2", TenantID: 42, UserID: "wecom:same", Source: "wecom", ChannelID: "a", MessageID: "m2", Status: "resolved", Rating: "useful"},
		{ID: "im3", TenantID: 42, UserID: "wecom:same", Source: "wecom", ChannelID: "b", MessageID: "m3", Status: "pending", Rating: "useful"},
		{ID: "im4", TenantID: 99, UserID: "wecom:same", Source: "wecom", ChannelID: "a", MessageID: "m4", Status: "pending", Rating: "useful"},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	h := NewAnswerFeedbackHandler(db, nil, nil, nil)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(types.WithCaller(c.Request.Context(), types.Caller{TenantID: 42, UserID: "admin"}))
	})
	r.GET("/feedback", h.List)
	r.POST("/feedback/:id/name", h.SetReporterName)
	call := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		r.ServeHTTP(response, request)
		return response
	}
	for _, name := range []string{"张三", "李四"} {
		response := call("POST", "/feedback/im1/name", `{"name":"`+name+`"}`)
		if response.Code != 200 {
			t.Fatalf("save: %d %s", response.Code, response.Body)
		}
		response = call("GET", "/feedback?status=all", "")
		var result struct {
			Data []types.AnswerFeedback `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Data) != 4 {
			t.Fatalf("tenant isolation: %s", response.Body)
		}
		for _, row := range result.Data {
			want := ""
			if row.ID == "w" {
				want = "Web name"
			}
			if row.ID == "im1" || row.ID == "im2" {
				want = name
			}
			if row.UserName != want {
				t.Fatalf("%s name=%q want=%q", row.ID, row.UserName, want)
			}
		}
	}
	if got := call("POST", "/feedback/im4/name", `{"name":"wrong tenant"}`); got.Code != 404 {
		t.Fatal("cross-tenant write accepted")
	}
	if got := call("POST", "/feedback/w/name", `{"name":"spoof"}`); got.Code != 400 {
		t.Fatal("web identity override accepted")
	}
	for _, name := range []string{strings.Repeat("名", 101), "new\nline"} {
		body, _ := json.Marshal(map[string]string{"name": name})
		if got := call("POST", "/feedback/im1/name", string(body)); got.Code != 400 {
			t.Fatal("invalid name accepted")
		}
	}
	if got := call("POST", "/feedback/im1/name", `{"name":""}`); got.Code != 200 {
		t.Fatal("clear name failed")
	}
	var count int64
	if err := db.Model(&types.IMFeedbackUserName{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("clear: %d %v", count, err)
	}
}

func TestAnswerFeedbackLegacyPublishCannotBypassRegression(t *testing.T) {
	db := answerFeedbackTestDB(t)
	row := &types.AnswerFeedback{ID: "feedback", TenantID: 42, UserID: "u", MessageID: "m", Question: "question", Answer: "old answer", Rating: "not_useful", Status: "pending"}
	if err := db.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	knowledge := &correctionKnowledgeServiceStub{}
	h := NewAnswerFeedbackHandler(db, nil, knowledge, correctionKBServiceStub{})
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(types.WithCaller(c.Request.Context(), types.Caller{TenantID: 42, UserID: "reviewer"}))
	})
	r.POST("/:id/review", h.Review)
	request := httptest.NewRequest("POST", "/feedback/review", strings.NewReader(`{"action":"publish","knowledge_base_id":"kb-1","answer":"correct answer"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	if response.Code != 409 {
		t.Fatalf("%d %s", response.Code, response.Body)
	}
	if knowledge.payload != nil {
		t.Fatalf("legacy endpoint created knowledge: %+v", knowledge.payload)
	}
	if err := db.First(row, "id = ?", "feedback").Error; err != nil {
		t.Fatal(err)
	}
	if row.Answer != "old answer" || row.CorrectionKnowledgeID != "" || row.Status != "pending" {
		t.Fatalf("snapshot or link changed: %+v", row)
	}
}
