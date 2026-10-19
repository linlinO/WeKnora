package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/feedback"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AnswerFeedbackHandler implements the simple end-user feedback loop:
// users submit a rating, while tenant administrators review and optionally
// publish a corrected answer into an ordinary knowledge base.
type AnswerFeedbackHandler struct {
	db               *gorm.DB
	messageService   interfaces.MessageService
	knowledgeService interfaces.KnowledgeService
	kbService        interfaces.KnowledgeBaseService
}

func NewAnswerFeedbackHandler(
	db *gorm.DB,
	messageService interfaces.MessageService,
	knowledgeService interfaces.KnowledgeService,
	kbService interfaces.KnowledgeBaseService,
) *AnswerFeedbackHandler {
	return &AnswerFeedbackHandler{db: db, messageService: messageService, knowledgeService: knowledgeService, kbService: kbService}
}

type answerFeedbackRequest struct {
	Rating   string `json:"rating" binding:"required"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
	Reason   string `json:"reason"`
}

// Create stores or updates the current user's judgement for one answer.
func (h *AnswerFeedbackHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	caller := types.CallerFromContext(ctx)
	if caller.TenantID == 0 || strings.TrimSpace(caller.UserID) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "未识别到登录用户"})
		return
	}
	var req answerFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "反馈内容不合法"})
		return
	}
	rating := strings.TrimSpace(req.Rating)
	if rating != types.AnswerFeedbackUseful && rating != types.AnswerFeedbackNotUseful && rating != "cancel" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "反馈类型只能是 useful 或 not_useful"})
		return
	}
	if h.db == nil || h.messageService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "反馈服务未配置"})
		return
	}
	sessionID := strings.TrimSpace(c.Param("session_id"))
	messageID := strings.TrimSpace(c.Param("message_id"))
	message, err := h.messageService.GetMessage(ctx, sessionID, messageID)
	if err != nil || message == nil || message.Role != "assistant" || !message.IsCompleted {
		c.JSON(http.StatusNotFound, gin.H{"message": "回答不存在或尚未完成"})
		return
	}
	question, err := feedback.Question(ctx, h.db, message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "读取原问题失败"})
		return
	}
	status := types.AnswerFeedbackPending
	if rating == "cancel" {
		rating, status = "", types.AnswerFeedbackWithdrawn
	}
	row := &types.AnswerFeedback{
		TenantID: caller.TenantID, UserID: caller.UserID, Source: "web",
		SessionID: sessionID, MessageID: messageID, AgentID: message.AgentID,
		Rating: rating, Question: question, Answer: message.Content,
		Reason: strings.TrimSpace(req.Reason), Status: status,
	}
	if err := feedback.Save(ctx, h.db, row); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "保存反馈失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": row})
}

// Get returns only the caller's own vote, after checking session access.
func (h *AnswerFeedbackHandler) Get(c *gin.Context) {
	ctx := c.Request.Context()
	caller := types.CallerFromContext(ctx)
	if _, err := h.messageService.GetMessage(ctx, c.Param("session_id"), c.Param("message_id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "回答不存在"})
		return
	}
	var row types.AnswerFeedback
	err := h.db.WithContext(ctx).Where("tenant_id = ? AND user_id = ? AND message_id = ?", caller.TenantID, caller.UserID, c.Param("message_id")).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusOK, gin.H{"data": nil})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "读取反馈失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

// List returns feedback for the current tenant. The route is admin-only.
func (h *AnswerFeedbackHandler) List(c *gin.Context) {
	caller := types.CallerFromContext(c.Request.Context())
	status := strings.TrimSpace(c.DefaultQuery("status", types.AnswerFeedbackPending))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("page_size", "100"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 100
	}
	var rows []*types.AnswerFeedback
	query := h.db.WithContext(c.Request.Context()).Model(&types.AnswerFeedback{}).Where("tenant_id = ?", caller.TenantID)
	statusCounts := map[string]int64{"pending": 0, "correcting": 0, "reviewed": 0, "resolved": 0, "ignored": 0, "withdrawn": 0, "all": 0}
	if _, valid := statusCounts[status]; !valid {
		c.JSON(http.StatusBadRequest, gin.H{"message": "处理状态不合法"})
		return
	}
	if source := c.Query("source"); source == "web" || source == "wecom" {
		query = query.Where("source = ?", source)
	}
	if rating := c.Query("rating"); rating != "" && rating != "all" {
		if rating != types.AnswerFeedbackUseful && rating != types.AnswerFeedbackNotUseful {
			c.JSON(http.StatusBadRequest, gin.H{"message": "反馈类型不合法"})
			return
		}
		query = query.Where("rating = ?", rating)
	}
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		query = query.Where("LOWER(question) LIKE ?", "%"+strings.ToLower(keyword)+"%")
	}
	if reporter := strings.TrimSpace(c.Query("reporter")); reporter != "" {
		pattern := "%" + strings.ToLower(reporter) + "%"
		query = query.Where(`LOWER(user_id) LIKE ? OR EXISTS (SELECT 1 FROM users u WHERE u.id = answer_feedback.user_id AND LOWER(u.username) LIKE ?) OR EXISTS (SELECT 1 FROM im_feedback_user_names n WHERE n.tenant_id = answer_feedback.tenant_id AND n.channel_id = answer_feedback.channel_id AND n.user_id = answer_feedback.user_id AND LOWER(n.name) LIKE ?)`, pattern, pattern, pattern)
	}
	for _, param := range []string{"from", "to"} {
		if value := c.Query(param); value != "" {
			date, err := time.Parse(time.RFC3339, value)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"message": "时间格式不合法"})
				return
			}
			if param == "from" {
				query = query.Where("created_at >= ?", date)
			} else {
				query = query.Where("created_at < ?", date)
			}
		}
	}
	// Counts share every search condition except the selected status tab.
	var grouped []struct {
		Status string
		Count  int64
	}
	if err := query.Session(&gorm.Session{}).Select("status, COUNT(*) AS count").Group("status").Scan(&grouped).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "统计处理状态失败"})
		return
	}
	for _, group := range grouped {
		statusCounts[group.Status] = group.Count
		statusCounts["all"] += group.Count
	}
	if status != "all" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "统计反馈失败"})
		return
	}
	if err := query.Omit("answer").Order("created_at DESC, id DESC").Offset((page - 1) * limit).Limit(limit).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "读取反馈失败"})
		return
	}
	if err := h.resolveFeedbackUserNames(c.Request.Context(), caller.TenantID, rows); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "读取反馈人姓名失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "total": total, "page": page, "page_size": limit, "status_counts": statusCounts})
}

// Detail loads the full snapshot only when an administrator opens a row.
func (h *AnswerFeedbackHandler) Detail(c *gin.Context) {
	ctx := c.Request.Context()
	caller := types.CallerFromContext(ctx)
	var row types.AnswerFeedback
	if err := h.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", c.Param("id"), caller.TenantID).First(&row).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "反馈记录不存在"})
		return
	}
	if err := h.resolveFeedbackUserNames(ctx, caller.TenantID, []*types.AnswerFeedback{&row}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "读取反馈人失败"})
		return
	}
	var proposal types.KnowledgeEvolutionProposal
	var proposalData *types.KnowledgeEvolutionProposal
	err := h.db.WithContext(ctx).Where("tenant_id = ? AND feedback_id = ?", caller.TenantID, row.ID).Order("created_at DESC").First(&proposal).Error
	if err == nil {
		proposalData = &proposal
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "读取草稿失败"})
		return
	}
	var run *types.KnowledgeRegressionRun
	if proposalData != nil && proposal.RegressionRunID != "" {
		var result types.KnowledgeRegressionRun
		if h.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", caller.TenantID, proposal.RegressionRunID).First(&result).Error == nil {
			run = &result
		}
	}
	var message types.Message
	// Join the owning session to avoid trusting client-provided message IDs.
	h.db.WithContext(ctx).Model(&types.Message{}).Select("messages.knowledge_references").Joins("JOIN sessions ON sessions.id = messages.session_id").
		Where("messages.id = ? AND messages.session_id = ? AND sessions.tenant_id = ?", row.MessageID, row.SessionID, caller.TenantID).Find(&message)
	c.JSON(http.StatusOK, gin.H{"data": row, "proposal": proposalData, "regression": run, "references": message.KnowledgeReferences})
}

type answerFeedbackReviewRequest struct {
	Action           string   `json:"action" binding:"required"`
	Answer           string   `json:"answer"`
	Title            string   `json:"title"`
	KnowledgeBaseID  string   `json:"knowledge_base_id"`
	SimilarQuestions []string `json:"similar_questions"`
	Note             string   `json:"note"`
}

// Review triages feedback. Publishing must use the versioned, tested proposal.
func (h *AnswerFeedbackHandler) Review(c *gin.Context) {
	ctx := c.Request.Context()
	caller := types.CallerFromContext(ctx)
	var req answerFeedbackReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "审核内容不合法"})
		return
	}
	var row types.AnswerFeedback
	if err := h.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", c.Param("id"), caller.TenantID).First(&row).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "反馈记录不存在"})
		return
	}
	action := strings.TrimSpace(req.Action)
	if row.Status != types.AnswerFeedbackPending && row.Status != types.AnswerFeedbackCorrecting && !(action == "correct" && row.Status == types.AnswerFeedbackReviewed) {
		c.JSON(http.StatusConflict, gin.H{"message": "反馈已处理或已撤回，请刷新后重试"})
		return
	}
	if action == "ignore" || action == "acknowledge" || action == "correct" {
		if action == "acknowledge" && row.Rating != types.AnswerFeedbackUseful {
			c.JSON(http.StatusBadRequest, gin.H{"message": "仅有用反馈可标记已阅"})
			return
		}
		row.Status = map[string]string{"ignore": types.AnswerFeedbackIgnored, "acknowledge": types.AnswerFeedbackReviewed, "correct": types.AnswerFeedbackCorrecting}[action]
		row.ReviewerID = caller.UserID
		row.ReviewNote = strings.TrimSpace(req.Note)
		if err := h.db.WithContext(ctx).Save(&row).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "更新审核状态失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": row})
		return
	}
	if action != "publish" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "不支持的处理操作"})
		return
	}
	c.JSON(http.StatusConflict, gin.H{"message": "请打开纠错草稿，完成回归检查后再发布"})
}

func buildFeedbackCorrectionMarkdown(question, answer string, similar []string) string {
	var b strings.Builder
	b.WriteString("# 权威纠错条目\n\n## 用户问题\n")
	b.WriteString(strings.TrimSpace(question))
	b.WriteString("\n\n## 标准答案\n")
	b.WriteString(strings.TrimSpace(answer))
	if len(similar) > 0 {
		b.WriteString("\n\n## 相似问法\n")
		for _, item := range similar {
			if item = strings.TrimSpace(item); item != "" {
				b.WriteString("- ")
				b.WriteString(item)
				b.WriteString("\n")
			}
		}
	}
	return b.String()
}
