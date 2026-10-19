package handler

import (
	"context"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

func (h *AnswerFeedbackHandler) resolveFeedbackUserNames(ctx context.Context, tenantID uint64, rows []*types.AnswerFeedback) error {
	var webIDs, imIDs []string
	for _, row := range rows {
		if row.Source == "wecom" {
			imIDs = append(imIDs, row.UserID)
		} else {
			webIDs = append(webIDs, strings.TrimPrefix(row.UserID, "admin-review:"))
		}
	}
	webNames := make(map[string]string)
	if len(webIDs) > 0 {
		var users []types.User
		// Only authors of the already tenant-scoped feedback can be resolved.
		// A workspace member's home tenant may differ from this workspace.
		if err := h.db.WithContext(ctx).Select("id", "username").Where("id IN ?", webIDs).Find(&users).Error; err != nil {
			return err
		}
		for _, user := range users {
			webNames[user.ID] = user.Username
		}
	}
	imNames := make(map[[2]string]string)
	if len(imIDs) > 0 {
		var names []types.IMFeedbackUserName
		if err := h.db.WithContext(ctx).Where("tenant_id = ? AND user_id IN ?", tenantID, imIDs).Find(&names).Error; err != nil {
			return err
		}
		for _, name := range names {
			imNames[[2]string{name.ChannelID, name.UserID}] = name.Name
		}
	}
	for _, row := range rows {
		if row.Source == "wecom" {
			row.UserName = imNames[[2]string{row.ChannelID, row.UserID}]
		} else {
			row.UserName = webNames[strings.TrimPrefix(row.UserID, "admin-review:")]
		}
	}
	return nil
}

// SetReporterName changes only the display label for a known WeCom feedback
// author. The admin guard and tenant/channel scope prevent identity spoofing.
func (h *AnswerFeedbackHandler) SetReporterName(c *gin.Context) {
	ctx := c.Request.Context()
	caller := types.CallerFromContext(ctx)
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "姓名格式不合法"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if utf8.RuneCountInString(name) > 100 || strings.ContainsFunc(name, unicode.IsControl) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "姓名最多 100 个字符，不能包含控制字符"})
		return
	}
	var row types.AnswerFeedback
	if err := h.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", c.Param("id"), caller.TenantID).Take(&row).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "反馈记录不存在"})
		return
	}
	if row.Source != "wecom" || row.ChannelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Web 用户名由平台账号维护"})
		return
	}
	label := types.IMFeedbackUserName{
		TenantID: row.TenantID, ChannelID: row.ChannelID, UserID: row.UserID,
		Name: name, UpdatedBy: caller.UserID,
	}
	var err error
	if name == "" {
		err = h.db.WithContext(ctx).Where("tenant_id = ? AND channel_id = ? AND user_id = ?", row.TenantID, row.ChannelID, row.UserID).
			Delete(&types.IMFeedbackUserName{}).Error
	} else {
		err = h.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "channel_id"}, {Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "updated_by", "updated_at"}),
		}).Create(&label).Error
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "保存姓名失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"user_name": name}})
}
