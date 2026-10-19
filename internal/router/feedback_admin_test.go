package router

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/qainsights"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

func TestFeedbackAdminAlwaysEnforced(t *testing.T) {
	for _, tc := range []struct {
		role     types.TenantRole
		machine  bool
		expected int
	}{
		{types.TenantRoleViewer, false, 403}, {types.TenantRoleContributor, false, 403},
		{types.TenantRoleAdmin, false, 204}, {types.TenantRoleOwner, false, 204}, {types.TenantRoleOwner, true, 403},
	} {
		g := &rbacGuards{cfg: &config.Config{}}
		r := gin.New()
		r.Use(func(c *gin.Context) {
			ctx := types.WithCaller(c.Request.Context(), types.Caller{TenantID: 42, UserID: "test", Role: tc.role})
			if tc.machine {
				ctx = types.WithTenantAPIKeyScope(ctx, types.TenantAPIKeyScope{FullAccess: true})
			}
			c.Request = c.Request.WithContext(ctx)
		})
		r.GET("/", g.FeedbackAdmin(), func(c *gin.Context) { c.Status(204) })
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, httptest.NewRequest("GET", "/", nil))
		if resp.Code != tc.expected {
			t.Fatalf("role=%s machine=%v code=%d", tc.role, tc.machine, resp.Code)
		}
	}
}

func TestFeedbackAdminEveryManagementRouteRejectsOrdinaryUser(t *testing.T) {
	g := &rbacGuards{cfg: &config.Config{}}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(types.WithCaller(c.Request.Context(), types.Caller{TenantID: 42, UserID: "ordinary", Role: types.TenantRoleContributor}))
	})
	RegisterAnswerFeedbackRoutes(r.Group("/api/v1"), handler.NewAnswerFeedbackHandler(nil, nil, nil, nil), g)
	RegisterKnowledgeEvolutionRoutes(r.Group("/api/v1"), handler.NewKnowledgeEvolutionHandler(nil, nil, nil, nil), g)
	RegisterQAInsightsRoutes(r.Group("/api/v1"), handler.NewQAInsightsHandler(&qainsights.Service{}), g)
	RegisterMemoryAdminRoutes(r.Group("/api/v1"), handler.NewMemoryAdminHandler(nil), g)
	checked := 0
	for _, route := range r.Routes() {
		if !strings.Contains(route.Path, "/admin/") {
			continue
		}
		path := strings.ReplaceAll(strings.ReplaceAll(route.Path, ":feedback_id", "feedback"), ":id", "id")
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, httptest.NewRequest(route.Method, path, strings.NewReader(`{}`)))
		if resp.Code != 403 {
			t.Fatalf("%s %s returned %d", route.Method, path, resp.Code)
		}
		checked++
	}
	if checked != 25 {
		t.Fatalf("unexpected route count %d", checked)
	}
}
