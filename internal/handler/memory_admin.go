package handler

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MemoryAdminHandler struct{ db *gorm.DB }

func NewMemoryAdminHandler(db *gorm.DB) *MemoryAdminHandler { return &MemoryAdminHandler{db: db} }

// Even when global RBAC is disabled these administrative reads stay protected.
func (h *MemoryAdminHandler) caller(c *gin.Context) (types.Caller, bool) {
	p := types.CallerFromContext(c.Request.Context())
	if p.TenantID == 0 || p.UserID == "" || !p.Role.HasPermission(types.TenantRoleAdmin) {
		c.JSON(403, gin.H{"message": "仅空间管理员可访问"})
		return p, false
	}
	return p, true
}
func (h *MemoryAdminHandler) audit(c *gin.Context, tx *gorm.DB, action, target string, details any) error {
	p := types.CallerFromContext(c.Request.Context())
	b, err := json.Marshal(details)
	if err != nil {
		return err
	}
	return tx.Create(&types.AuditLog{TenantID: p.TenantID, ActorUserID: p.UserID, ActorRole: string(p.Role), Action: types.AuditAction(action), ScopeType: "memory", ScopeID: target, TargetType: "memory_subject", TargetID: target, RequestPath: c.Request.URL.Path, RequestMethod: c.Request.Method, Details: types.JSON(b), CreatedAt: time.Now().UTC()}).Error
}
func (h *MemoryAdminHandler) granted(c *gin.Context, p types.Caller) bool {
	var n int64
	return h.db.WithContext(c.Request.Context()).Model(&types.MemoryAdminGrant{}).Where("tenant_id=? AND user_id=?", p.TenantID, p.UserID).Count(&n).Error == nil && n == 1
}
func (h *MemoryAdminHandler) Access(c *gin.Context) {
	p, ok := h.caller(c)
	if !ok {
		return
	}
	var members []struct {
		UserID  string `json:"user_id"`
		Name    string `json:"name"`
		Role    string `json:"role"`
		Granted bool   `json:"granted"`
	}
	if p.Role == types.TenantRoleOwner {
		err := h.db.WithContext(c.Request.Context()).Table("tenant_members m").Select("m.user_id,u.username AS name,m.role,CASE WHEN g.user_id IS NULL THEN false ELSE true END AS granted").Joins("JOIN users u ON u.id=m.user_id").Joins("LEFT JOIN memory_admin_grants g ON g.tenant_id=m.tenant_id AND g.user_id=m.user_id").Where("m.tenant_id=? AND m.status='active' AND m.role IN ('owner','admin') AND m.deleted_at IS NULL", p.TenantID).Scan(&members).Error
		if err != nil {
			c.JSON(500, gin.H{"message": "读取授权成员失败"})
			return
		}
	}
	c.JSON(200, gin.H{"data": gin.H{"granted": h.granted(c, p), "owner": p.Role == types.TenantRoleOwner, "members": members}})
}
func (h *MemoryAdminHandler) Grant(c *gin.Context) {
	p, ok := h.caller(c)
	if !ok {
		return
	}
	if p.Role != types.TenantRoleOwner {
		c.JSON(403, gin.H{"message": "仅空间所有者可授权记忆正文查看"})
		return
	}
	var req struct {
		UserID  string `json:"user_id"`
		Enabled bool   `json:"enabled"`
	}
	if c.ShouldBindJSON(&req) != nil || req.UserID == "" {
		c.JSON(400, gin.H{"message": "请选择管理员"})
		return
	}
	var count int64
	if err := h.db.WithContext(c.Request.Context()).Table("tenant_members").Where("tenant_id=? AND user_id=? AND status='active' AND role IN ('owner','admin') AND deleted_at IS NULL", p.TenantID, req.UserID).Count(&count).Error; err != nil || count != 1 {
		c.JSON(400, gin.H{"message": "目标必须是本空间有效管理员"})
		return
	}
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if e := h.audit(c, tx, "memory.access_changed", req.UserID, gin.H{"enabled": req.Enabled}); e != nil {
			return e
		}
		if !req.Enabled {
			return tx.Where("tenant_id=? AND user_id=?", p.TenantID, req.UserID).Delete(&types.MemoryAdminGrant{}).Error
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&types.MemoryAdminGrant{TenantID: p.TenantID, UserID: req.UserID, GrantedBy: p.UserID}).Error
	})
	if err != nil {
		c.JSON(500, gin.H{"message": "授权未保存"})
		return
	}
	c.JSON(200, gin.H{"success": true})
}

type memorySubjectRow struct {
	ID              string     `json:"id"`
	SubjectID       string     `json:"-"`
	Name            string     `json:"name"`
	UserID          string     `json:"user_id"`
	ChannelID       string     `json:"channel_id"`
	Enabled         bool       `json:"enabled"`
	Active          int64      `json:"active"`
	Pending         int64      `json:"pending"`
	Queued          int64      `json:"queued"`
	Failed          int64      `json:"failed"`
	LastExtractedAt *time.Time `json:"last_extracted_at"`
}

func (h *MemoryAdminHandler) Subjects(c *gin.Context) {
	p, ok := h.caller(c)
	if !ok {
		return
	}
	page, size := qaPage(c)
	q := h.db.WithContext(c.Request.Context()).Table("memory_subjects s").Where("s.tenant_id=? AND s.subject_id LIKE ?", p.TenantID, fmt.Sprintf("im_user:%d:%%", p.TenantID))
	if v := strings.TrimSpace(c.Query("keyword")); v != "" {
		q = q.Where("(s.subject_id LIKE ? OR EXISTS (SELECT 1 FROM im_feedback_user_names n WHERE n.tenant_id=s.tenant_id AND s.subject_id='im_user:' || CAST(n.tenant_id AS TEXT) || ':' || n.channel_id || ':wecom:' || REPLACE(n.user_id,'wecom:','') AND n.name LIKE ?))", "%"+v+"%", "%"+v+"%")
	}
	var total int64
	var rows []memorySubjectRow
	err := q.Count(&total).Error
	if err == nil {
		err = q.Select(`s.id,s.subject_id,s.enabled,s.last_extracted_at,
 (SELECT COUNT(*) FROM memory_items i WHERE i.tenant_id=s.tenant_id AND i.subject_id=s.subject_id AND i.status='active' AND (i.expires_at IS NULL OR i.expires_at>CURRENT_TIMESTAMP)) AS active,
 (SELECT COUNT(*) FROM memory_items i WHERE i.tenant_id=s.tenant_id AND i.subject_id=s.subject_id AND i.status='pending') AS pending,
 (SELECT COUNT(*) FROM memory_extraction_sessions e WHERE e.tenant_id=s.tenant_id AND e.subject_id=s.subject_id AND e.pending=true) AS queued,
 (SELECT COUNT(*) FROM memory_extraction_sessions e WHERE e.tenant_id=s.tenant_id AND e.subject_id=s.subject_id AND e.failure_count>0) AS failed`).Order("s.updated_at DESC,s.id").Offset((page - 1) * size).Limit(size).Scan(&rows).Error
	}
	if err != nil {
		c.JSON(500, gin.H{"message": "读取员工记忆状态失败"})
		return
	}
	for i := range rows {
		parts := strings.SplitN(rows[i].SubjectID, ":", 5)
		if len(parts) != 5 {
			continue
		}
		rows[i].ChannelID = parts[2]
		rows[i].UserID = parts[4]
		_ = h.db.Table("im_feedback_user_names").Select("name").Where("tenant_id=? AND channel_id=? AND user_id IN ?", p.TenantID, parts[2], []string{parts[4], parts[3] + ":" + parts[4]}).Limit(1).Scan(&rows[i].Name).Error
	}
	c.JSON(200, gin.H{"data": rows, "total": total})
}
func (h *MemoryAdminHandler) subject(c *gin.Context, p types.Caller) (types.MemorySubject, bool) {
	var s types.MemorySubject
	err := h.db.WithContext(c.Request.Context()).Select("id", "tenant_id", "subject_id", "enabled", "last_extracted_at").Where("id=? AND tenant_id=? AND subject_id LIKE ?", c.Param("id"), p.TenantID, fmt.Sprintf("im_user:%d:%%", p.TenantID)).First(&s).Error
	if err != nil {
		c.JSON(404, gin.H{"message": "员工记忆不存在"})
		return s, false
	}
	return s, true
}
func (h *MemoryAdminHandler) Detail(c *gin.Context) {
	p, ok := h.caller(c)
	if !ok {
		return
	}
	s, ok := h.subject(c, p)
	if !ok {
		return
	}
	var jobs []struct {
		SessionID    string    `json:"session_id"`
		Pending      bool      `json:"pending"`
		FailureCount int       `json:"failure_count"`
		FailureCode  string    `json:"failure_code"`
		UpdatedAt    time.Time `json:"updated_at"`
	}
	if err := h.db.WithContext(c.Request.Context()).Model(&types.MemoryExtractionSession{}).Select("session_id", "pending", "failure_count", "failure_code", "updated_at").Where("tenant_id=? AND subject_id=?", p.TenantID, s.SubjectID).Order("updated_at DESC").Limit(50).Scan(&jobs).Error; err != nil {
		c.JSON(500, gin.H{"message": "读取提炼状态失败"})
		return
	}
	c.JSON(200, gin.H{"data": gin.H{"id": s.ID, "enabled": s.Enabled, "last_extracted_at": s.LastExtractedAt, "jobs": jobs}})
}
func (h *MemoryAdminHandler) Content(c *gin.Context) {
	p, ok := h.caller(c)
	if !ok {
		return
	}
	if !h.granted(c, p) {
		c.JSON(403, gin.H{"message": "尚未获得本空间的记忆正文查看授权"})
		return
	}
	s, ok := h.subject(c, p)
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
		Page   int    `json:"page"`
	}
	if c.ShouldBindJSON(&req) != nil || len([]rune(strings.TrimSpace(req.Reason))) < 4 || len([]rune(req.Reason)) > 200 {
		c.JSON(400, gin.H{"message": "请填写4至200字的查看原因"})
		return
	}
	if req.Page < 1 {
		req.Page = 1
	}
	var items []types.MemoryItem
	var total int64
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		q := tx.Model(&types.MemoryItem{}).Where("tenant_id=? AND subject_id=?", p.TenantID, s.SubjectID)
		if e := q.Count(&total).Error; e != nil {
			return e
		}
		if e := q.Order("updated_at DESC,id").Offset((req.Page - 1) * 20).Limit(20).Find(&items).Error; e != nil {
			return e
		}
		return h.audit(c, tx, "memory.content_viewed", s.ID, gin.H{"reason": strings.TrimSpace(req.Reason), "page": req.Page, "count": len(items)})
	})
	if err != nil {
		c.JSON(500, gin.H{"message": "读取或审计记录失败，未返回记忆正文"})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"data": items, "total": total})
}
