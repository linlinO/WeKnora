package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterKnowledgeCorrectionRoutes exposes the administrator correction loop
// on a document knowledge base. The normal KB ownership/write guards remain in
// force, so only the KB creator or an administrator can publish corrections.
func RegisterKnowledgeCorrectionRoutes(r *gin.RouterGroup, h *handler.KnowledgeCorrectionHandler, g *rbacGuards) {
	if h == nil {
		return
	}
	corrections := g.apiKeyGroup(r.Group("/knowledge-bases/:id/corrections"), apiKeyIngest(apiKeyFullAccess()))
	corrections.POST("", g.OwnedKBOrAdmin(), g.KBAccessWrite("id"), h.Create)
}
