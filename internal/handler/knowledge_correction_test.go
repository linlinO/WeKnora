package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

type correctionKnowledgeServiceStub struct {
	interfaces.KnowledgeService
	payload *types.ManualKnowledgePayload
}

func (s *correctionKnowledgeServiceStub) CreateKnowledgeFromManual(_ context.Context, _ string, payload *types.ManualKnowledgePayload, _ string) (*types.Knowledge, error) {
	s.payload = payload
	return &types.Knowledge{ID: "knowledge-1"}, nil
}

type correctionKBServiceStub struct {
	interfaces.KnowledgeBaseService
}

func (correctionKBServiceStub) GetKnowledgeBaseByID(context.Context, string) (*types.KnowledgeBase, error) {
	return &types.KnowledgeBase{ID: "kb-1", Type: types.KnowledgeBaseTypeDocument}, nil
}

func TestKnowledgeCorrectionHandlerCreatesPublishedManualKnowledge(t *testing.T) {
	gin.SetMode(gin.TestMode)
	knowledge := &correctionKnowledgeServiceStub{}
	h := NewKnowledgeCorrectionHandler(knowledge, correctionKBServiceStub{})
	r := gin.New()
	r.POST("/knowledge-bases/:id/corrections", h.Create)
	body, _ := json.Marshal(map[string]any{
		"question": "服务器负载高怎么定位？",
		"answer":   "先区分 CPU 计算打满与 IO 等待。",
	})
	req := httptest.NewRequest(http.MethodPost, "/knowledge-bases/kb-1/corrections", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d, body=%s", resp.Code, resp.Body.String())
	}
	if knowledge.payload == nil || knowledge.payload.Status != types.ManualKnowledgeStatusPublish {
		t.Fatalf("unexpected payload: %#v", knowledge.payload)
	}
}
