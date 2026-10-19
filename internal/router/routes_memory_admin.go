package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

func RegisterMemoryAdminRoutes(r *gin.RouterGroup, h *handler.MemoryAdminHandler, g *rbacGuards) {
	if h == nil {
		return
	}
	admin := g.apiKeyGroup(r.Group("/admin/memory", g.FeedbackAdmin()), apiKeyFullAccess())
	admin.GET("/access", h.Access)
	admin.PUT("/access", h.Grant)
	admin.GET("/subjects", h.Subjects)
	admin.GET("/subjects/:id", h.Detail)
	admin.POST("/subjects/:id/content", h.Content)
}
