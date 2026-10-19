package router

import (
	"net/http"

	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// FeedbackAdmin always enforces the human review boundary, including during
// the general RBAC rollout. API keys must not bypass this approval workflow.
func (g *rbacGuards) FeedbackAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		caller := types.CallerFromContext(ctx)
		_, machine := types.TenantAPIKeyScopeFromContext(ctx)
		if !machine && caller.UserID != "" && caller.TenantID != 0 &&
			(caller.Role.HasPermission(types.TenantRoleAdmin) || middleware.IsCrossTenantSuperuser(ctx, g.cfg)) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": "仅工作空间管理员可管理回答反馈"})
	}
}
