package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

func RegisterAnswerFeedbackRoutes(r *gin.RouterGroup, h *handler.AnswerFeedbackHandler, g *rbacGuards) {
	if h == nil {
		return
	}
	feedback := g.apiKeyGroup(r.Group("/answer-feedback"), apiKeyFullAccess())
	feedback.POST("/:session_id/:message_id", g.Viewer(), h.Create)
	feedback.GET("/:session_id/:message_id", g.Viewer(), h.Get)
	admin := g.apiKeyGroup(r.Group("/admin/answer-feedback"), apiKeyFullAccess())
	admin.GET("", g.FeedbackAdmin(), h.List)
	admin.GET("/:id", g.FeedbackAdmin(), h.Detail)
	admin.POST("/:id/reporter-name", g.FeedbackAdmin(), h.SetReporterName)
	admin.POST("/:id/review", g.FeedbackAdmin(), h.Review)
}
