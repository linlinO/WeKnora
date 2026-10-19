package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterKnowledgeEvolutionRoutes exposes the review-gated correction loop.
// Ordinary users never reach these routes; the existing answer-feedback
// endpoints remain the only write surface for end-user votes.
func RegisterKnowledgeEvolutionRoutes(r *gin.RouterGroup, h *handler.KnowledgeEvolutionHandler, g *rbacGuards) {
	if h == nil {
		return
	}
	admin := g.apiKeyGroup(r.Group("/admin/knowledge-evolution"), apiKeyFullAccess())
	admin.GET("", g.FeedbackAdmin(), h.List)
	admin.GET("/:id", g.FeedbackAdmin(), h.Get)
	admin.POST("/:id/draft", g.FeedbackAdmin(), h.SaveDraft)
	admin.POST("/feedback/:feedback_id/draft", g.FeedbackAdmin(), h.CreateDraft)
	admin.POST("/:id/regression", g.FeedbackAdmin(), h.RunRegression)
	admin.POST("/:id/verify", g.FeedbackAdmin(), h.VerifyPublished)
	admin.GET("/:id/runs", g.FeedbackAdmin(), h.ReplayRuns)
	admin.POST("/:id/publish", g.FeedbackAdmin(), h.Publish)
	admin.POST("/:id/rollback", g.FeedbackAdmin(), h.Rollback)
}
