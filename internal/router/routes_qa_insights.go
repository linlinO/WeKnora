package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

func RegisterQAInsightsRoutes(r *gin.RouterGroup, h *handler.QAInsightsHandler, g *rbacGuards) {
	if h == nil {
		return
	}
	admin := g.apiKeyGroup(r.Group("/admin/qa-insights", g.FeedbackAdmin()), apiKeyFullAccess())
	admin.GET("/questions", h.List)
	admin.GET("/questions/:id", h.Detail)
	admin.GET("/topics", h.Topics)
	admin.GET("/topics/:id", h.TopicDetail)
	admin.POST("/topics/:id/reclassify", h.Reclassify)
	admin.POST("/answers/:id/review", h.Review)
	admin.GET("/memory-status", h.MemoryStatus)
}
