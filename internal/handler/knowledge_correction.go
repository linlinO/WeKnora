package handler

import (
	"fmt"
	"net/http"
	"strings"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// KnowledgeCorrectionHandler publishes an administrator-approved correction as
// a manual knowledge entry. Reusing the normal manual-ingestion path ensures
// the correction is chunked, embedded, and searchable by the existing RAG
// pipeline without introducing a second index.
type KnowledgeCorrectionHandler struct {
	knowledgeService interfaces.KnowledgeService
	kbService        interfaces.KnowledgeBaseService
}

func NewKnowledgeCorrectionHandler(
	knowledgeService interfaces.KnowledgeService,
	kbService interfaces.KnowledgeBaseService,
) *KnowledgeCorrectionHandler {
	return &KnowledgeCorrectionHandler{
		knowledgeService: knowledgeService,
		kbService:        kbService,
	}
}

type knowledgeCorrectionRequest struct {
	Question         string   `json:"question" binding:"required"`
	Answer           string   `json:"answer" binding:"required"`
	SimilarQuestions []string `json:"similar_questions"`
	Title            string   `json:"title"`
	Publish          *bool    `json:"publish"`
}

// Create publishes a correction into the target document knowledge base.
// A correction is intentionally a normal manual knowledge document so that it
// follows the same storage, parsing, embedding, and deletion lifecycle as
// every other document in the knowledge base.
func (h *KnowledgeCorrectionHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	kbID := strings.TrimSpace(c.Param("id"))
	var req knowledgeCorrectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewBadRequestError("纠错请求不合法").WithDetails(err.Error()))
		return
	}

	question := strings.TrimSpace(req.Question)
	answer := strings.TrimSpace(req.Answer)
	if question == "" || answer == "" {
		c.Error(apperrors.NewValidationError("问题和标准答案不能为空"))
		return
	}
	if h.kbService == nil || h.knowledgeService == nil {
		c.Error(apperrors.NewInternalServerError("纠错服务未配置"))
		return
	}
	if _, err := h.kbService.GetKnowledgeBaseByID(ctx, kbID); err != nil {
		c.Error(err)
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "知识纠错：" + question
	}

	var content strings.Builder
	content.WriteString("# 权威纠错条目\n\n")
	content.WriteString("## 用户问题\n")
	content.WriteString(question)
	content.WriteString("\n\n## 标准答案\n")
	content.WriteString(answer)
	if len(req.SimilarQuestions) > 0 {
		content.WriteString("\n\n## 相似问法\n")
		for _, similar := range req.SimilarQuestions {
			if similar = strings.TrimSpace(similar); similar != "" {
				content.WriteString("- ")
				content.WriteString(similar)
				content.WriteString("\n")
			}
		}
	}

	status := types.ManualKnowledgeStatusPublish
	if req.Publish != nil && !*req.Publish {
		status = types.ManualKnowledgeStatusDraft
	}
	knowledge, err := h.knowledgeService.CreateKnowledgeFromManual(ctx, kbID, &types.ManualKnowledgePayload{
		Title:   title,
		Content: content.String(),
		Status:  status,
		Channel: types.ChannelAPI,
	}, types.ChannelAPI)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{"knowledge_base_id": kbID})
		c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": gin.H{
			"knowledge_id":      knowledge.ID,
			"knowledge_base_id": kbID,
			"status":            status,
			"message":           fmt.Sprintf("纠错内容已%s，索引任务将在后台执行", correctionStatusText(status)),
		},
	})
}

func correctionStatusText(status string) string {
	if status == types.ManualKnowledgeStatusDraft {
		return "保存为草稿"
	}
	return "发布"
}
