package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/qainsights"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type QAInsightsHandler struct{ db *gorm.DB }

func NewQAInsightsHandler(s *qainsights.Service) *QAInsightsHandler {
	return &QAInsightsHandler{db: s.DB}
}

func qaPage(c *gin.Context) (int, int) {
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	n, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if p < 1 {
		p = 1
	}
	if n < 1 || n > 100 {
		n = 20
	}
	return p, n
}

func (h *QAInsightsHandler) questions(c *gin.Context) (*gorm.DB, error) {
	tenant := types.CallerFromContext(c.Request.Context()).TenantID
	if tenant == 0 {
		return nil, errors.New("未识别到工作空间")
	}
	q := h.db.WithContext(c.Request.Context()).Table("qa_questions AS q").Where("q.tenant_id = ?", tenant)
	zone := time.FixedZone("CST", 8*3600)
	start := c.Query("start")
	end := c.Query("end")
	if start == "" && end == "" {
		q = q.Where("q.created_at >= ?", time.Now().AddDate(0, 0, -7))
	}
	if start != "" {
		t, e := time.ParseInLocation("2006-01-02", start, zone)
		if e != nil {
			return nil, errors.New("开始日期不正确")
		}
		q = q.Where("q.created_at >= ?", t)
	}
	if end != "" {
		t, e := time.ParseInLocation("2006-01-02", end, zone)
		if e != nil {
			return nil, errors.New("结束日期不正确")
		}
		q = q.Where("q.created_at < ?", t.AddDate(0, 0, 1))
	}
	for key, column := range map[string]string{"source": "q.source", "channel": "q.channel_id", "chat_type": "q.chat_type", "topic": "q.topic_id", "classification": "q.classification"} {
		if v := c.Query(key); v != "" && v != "all" {
			q = q.Where(column+" = ?", v)
		}
	}
	if v := strings.TrimSpace(c.Query("keyword")); v != "" {
		q = q.Where("q.question LIKE ?", "%"+v+"%")
	}
	if v := strings.TrimSpace(c.Query("reporter")); v != "" {
		q = q.Where("(q.user_id LIKE ? OR q.user_name LIKE ? OR EXISTS (SELECT 1 FROM im_feedback_user_names n WHERE n.tenant_id=q.tenant_id AND n.channel_id=q.channel_id AND (n.user_id=q.user_id OR n.user_id='wecom:' || q.user_id) AND n.name LIKE ?) OR EXISTS (SELECT 1 FROM users u WHERE q.source='web' AND u.id=q.user_id AND u.username LIKE ?))", "%"+v+"%", "%"+v+"%", "%"+v+"%", "%"+v+"%")
	}
	return q, nil
}

func (h *QAInsightsHandler) List(c *gin.Context) {
	q, e := h.questions(c)
	if e != nil {
		c.JSON(400, gin.H{"message": e.Error()})
		return
	}
	p, n := qaPage(c)
	var total int64
	var rows []types.QAQuestion
	if e = q.Count(&total).Error; e == nil {
		e = q.Select("q.*").Order("q.created_at DESC, q.id DESC").Offset((p - 1) * n).Limit(n).Scan(&rows).Error
	}
	if e != nil {
		c.JSON(500, gin.H{"message": "读取问答失败"})
		return
	}
	h.names(c, rows)
	c.JSON(200, gin.H{"data": rows, "total": total})
}

func (h *QAInsightsHandler) names(c *gin.Context, rows []types.QAQuestion) {
	for i := range rows {
		r := &rows[i]
		if r.Source == "web" {
			var name string
			_ = h.db.Table("users").Where("id = ?", r.UserID).Select("username").Scan(&name).Error
			if name != "" {
				r.UserName = name
			}
		}
		if r.ChannelID != "" {
			var name string
			_ = h.db.Model(&types.IMFeedbackUserName{}).Where("tenant_id = ? AND channel_id = ? AND user_id IN ?", r.TenantID, r.ChannelID, []string{r.UserID, "wecom:" + r.UserID}).Select("name").Scan(&name).Error
			if name != "" {
				r.UserName = name
			}
		}
	}
}

type qaTopicStats struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Questions int64  `json:"questions"`
	People    int64  `json:"people"`
	NotUseful int64  `json:"not_useful"`
	Useful    int64  `json:"useful"`
	Failures  int64  `json:"failures"`
	LastAsked string `json:"last_asked"`
}

func (h *QAInsightsHandler) Topics(c *gin.Context) {
	q, e := h.questions(c)
	if e != nil {
		c.JSON(400, gin.H{"message": e.Error()})
		return
	}
	q = q.Joins("JOIN qa_topics t ON t.id=q.topic_id AND t.tenant_id=q.tenant_id").Select(`t.id, t.title, COUNT(*) AS questions, COUNT(DISTINCT q.subject_id) AS people, MAX(q.created_at) AS last_asked,
 SUM(CASE WHEN EXISTS (SELECT 1 FROM qa_answers a JOIN answer_feedback f ON f.message_id=a.id AND f.tenant_id=a.tenant_id WHERE a.question_id=q.id AND f.rating='not_useful' AND f.status<>'withdrawn' AND f.source<>'admin_review') THEN 1 ELSE 0 END) AS not_useful,
 SUM(CASE WHEN EXISTS (SELECT 1 FROM qa_answers a JOIN answer_feedback f ON f.message_id=a.id AND f.tenant_id=a.tenant_id WHERE a.question_id=q.id AND f.rating='useful' AND f.status<>'withdrawn' AND f.source<>'admin_review') THEN 1 ELSE 0 END) AS useful,
 SUM(CASE WHEN EXISTS (SELECT 1 FROM qa_answers a WHERE a.question_id=q.id AND a.generation_status IN ('failed','interrupted','truncated')) THEN 1 ELSE 0 END) AS failures`).Group("t.id, t.title")
	if c.DefaultQuery("frequent", "true") == "true" {
		count, _ := strconv.Atoi(c.DefaultQuery("min_count", "3"))
		people, _ := strconv.Atoi(c.DefaultQuery("min_people", "2"))
		if count < 1 {
			count = 3
		}
		if people < 1 {
			people = 2
		}
		q = q.Having("COUNT(*) >= ? AND COUNT(DISTINCT q.subject_id) >= ?", count, people)
	}
	var total int64
	p, n := qaPage(c)
	if e = h.db.Table("(?) AS grouped", q).Count(&total).Error; e != nil {
		c.JSON(500, gin.H{"message": "统计失败"})
		return
	}
	var rows []qaTopicStats
	if e = q.Order("questions DESC, last_asked DESC, t.id ASC").Offset((p - 1) * n).Limit(n).Scan(&rows).Error; e != nil {
		c.JSON(500, gin.H{"message": "统计失败"})
		return
	}
	c.JSON(200, gin.H{"data": rows, "total": total})
}

func (h *QAInsightsHandler) Detail(c *gin.Context) {
	tenant := types.CallerFromContext(c.Request.Context()).TenantID
	var q types.QAQuestion
	if h.db.Where("tenant_id = ? AND id = ?", tenant, c.Param("id")).First(&q).Error != nil {
		c.JSON(404, gin.H{"message": "问答不存在"})
		return
	}
	rows := []types.QAQuestion{q}
	h.names(c, rows)
	q = rows[0]
	answers := []types.QAAnswer{}
	if h.db.Where("tenant_id = ? AND question_id = ?", tenant, q.ID).Order("created_at ASC").Find(&answers).Error != nil {
		c.JSON(500, gin.H{"message": "读取回答失败"})
		return
	}
	ids := []string{}
	for _, a := range answers {
		ids = append(ids, a.ID)
	}
	feedback := []types.AnswerFeedback{}
	if len(ids) > 0 {
		if h.db.Where("tenant_id = ? AND message_id IN ?", tenant, ids).Order("created_at DESC").Find(&feedback).Error != nil {
			c.JSON(500, gin.H{"message": "读取反馈失败"})
			return
		}
	}
	c.JSON(200, gin.H{"data": q, "answers": answers, "feedback": feedback})
}

func (h *QAInsightsHandler) TopicDetail(c *gin.Context) {
	tenant := types.CallerFromContext(c.Request.Context()).TenantID
	var topic types.QATopic
	if h.db.Where("tenant_id = ? AND id = ?", tenant, c.Param("id")).First(&topic).Error != nil {
		c.JSON(404, gin.H{"message": "主题不存在"})
		return
	}
	c.JSON(200, gin.H{"data": topic})
}

// Reclassify is an explicit administrator operation. The worker never undoes it.
func (h *QAInsightsHandler) Reclassify(c *gin.Context) {
	tenant := types.CallerFromContext(c.Request.Context()).TenantID
	var req struct {
		Action      string   `json:"action"`
		TargetID    string   `json:"target_id"`
		QuestionIDs []string `json:"question_ids"`
		Title       string   `json:"title"`
	}
	if c.ShouldBindJSON(&req) != nil || (req.Action != "merge" && req.Action != "split") || len(req.QuestionIDs) > 100 {
		c.JSON(400, gin.H{"message": "参数不正确"})
		return
	}
	newID := ""
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var source types.QATopic
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ? AND merged_into = ''", tenant, c.Param("id")).First(&source).Error; e != nil {
			return e
		}
		if req.Action == "merge" {
			if req.TargetID == source.ID {
				return errors.New("不能合并到自身")
			}
			var target types.QATopic
			if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ? AND merged_into = ''", tenant, req.TargetID).First(&target).Error; e != nil {
				return e
			}
			if e := tx.Model(&types.QAQuestion{}).Where("tenant_id = ? AND topic_id = ?", tenant, source.ID).Updates(map[string]interface{}{"topic_id": target.ID, "manual_topic": true}).Error; e != nil {
				return e
			}
			newID = target.ID
			return tx.Model(&source).Updates(map[string]interface{}{"merged_into": target.ID, "manual": true}).Error
		}
		if len(req.QuestionIDs) == 0 || strings.TrimSpace(req.Title) == "" || len([]rune(req.Title)) > 200 {
			return errors.New("请选择问答并填写新主题（最多200字）")
		}
		var count int64
		if e := tx.Model(&types.QAQuestion{}).Where("tenant_id = ? AND topic_id = ? AND id IN ?", tenant, source.ID, req.QuestionIDs).Count(&count).Error; e != nil {
			return e
		}
		if count != int64(len(req.QuestionIDs)) {
			return errors.New("所选问答不属于当前主题")
		}
		topic := types.QATopic{ID: uuid.NewString(), TenantID: tenant, Title: strings.TrimSpace(req.Title), NormalizedKey: uuid.NewString(), Manual: true, Embedding: types.JSON("[]")}
		if e := tx.Create(&topic).Error; e != nil {
			return e
		}
		newID = topic.ID
		return tx.Model(&types.QAQuestion{}).Where("tenant_id = ? AND id IN ?", tenant, req.QuestionIDs).Updates(map[string]interface{}{"topic_id": topic.ID, "manual_topic": true, "classification": "classified"}).Error
	})
	if err != nil {
		c.JSON(400, gin.H{"message": "归类未保存，请核对主题和问答范围"})
		return
	}
	c.JSON(200, gin.H{"success": true, "topic_id": newID})
}

func (h *QAInsightsHandler) Review(c *gin.Context) {
	caller := types.CallerFromContext(c.Request.Context())
	var a types.QAAnswer
	var q types.QAQuestion
	if h.db.Where("tenant_id = ? AND id = ?", caller.TenantID, c.Param("id")).First(&a).Error != nil || h.db.Where("tenant_id = ? AND id = ?", caller.TenantID, a.QuestionID).First(&q).Error != nil {
		c.JSON(404, gin.H{"message": "回答不存在"})
		return
	}
	if a.Answer == "" || a.GenerationStatus == "generating" {
		c.JSON(400, gin.H{"message": "回答尚未结束"})
		return
	}
	var f types.AnswerFeedback
	err := h.db.Transaction(func(tx *gorm.DB) error {
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", caller.TenantID, a.ID).First(&a).Error; e != nil {
			return e
		}
		if a.FeedbackID != "" {
			return tx.Where("tenant_id = ? AND id = ?", caller.TenantID, a.FeedbackID).First(&f).Error
		}
		f = types.AnswerFeedback{ID: uuid.NewString(), TenantID: caller.TenantID, UserID: "admin-review:" + caller.UserID, Source: "admin_review", SessionID: a.SessionID, MessageID: a.ID, AgentID: a.AgentID, Rating: types.AnswerFeedbackNotUseful, Question: q.Question, Answer: a.Answer, Reason: "管理员从问答统计主动发起纠错（非用户评价）", Status: types.AnswerFeedbackCorrecting, ReviewerID: caller.UserID, CreatedAt: time.Now(), UpdatedAt: time.Now()}
		if e := tx.Create(&f).Error; e != nil {
			return e
		}
		return tx.Model(&a).Update("feedback_id", f.ID).Error
	})
	if err != nil {
		c.JSON(500, gin.H{"message": "创建纠错记录失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"feedback_id": f.ID})
}

func (h *QAInsightsHandler) MemoryStatus(c *gin.Context) {
	tenant := types.CallerFromContext(c.Request.Context()).TenantID
	var subjects, items, pending, failed int64
	h.db.Model(&types.MemorySubject{}).Where("tenant_id = ? AND subject_id LIKE ?", tenant, "im_user:%").Count(&subjects)
	h.db.Model(&types.MemoryItem{}).Where("tenant_id = ? AND subject_id LIKE ? AND status = ?", tenant, "im_user:%", "active").Count(&items)
	h.db.Model(&types.MemoryExtractionSession{}).Where("tenant_id = ? AND subject_id LIKE ? AND pending = ?", tenant, "im_user:%", true).Count(&pending)
	h.db.Model(&types.MemoryExtractionSession{}).Where("tenant_id = ? AND subject_id LIKE ? AND failure_count > 0", tenant, "im_user:%").Count(&failed)
	c.JSON(200, gin.H{"subjects": subjects, "items": items, "pending": pending, "failed": failed})
}
