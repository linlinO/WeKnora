package handler

import (
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestMemoryAdminContentRequiresGrantTenantAndAudit(t *testing.T) {
	db, e := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "memory.db")), &gorm.Config{})
	if e != nil {
		t.Fatal(e)
	}
	if e = db.AutoMigrate(&types.MemorySubject{}, &types.MemoryItem{}, &types.MemoryAdminGrant{}, &types.AuditLog{}); e != nil {
		t.Fatal(e)
	}
	db.Create(&types.MemorySubject{ID: "alice", TenantID: 42, SubjectID: "im_user:42:bot:wecom:alice", Enabled: true})
	db.Create(&types.MemorySubject{ID: "foreign", TenantID: 43, SubjectID: "im_user:43:bot:wecom:bob", Enabled: true})
	db.Create(&types.MemoryItem{ID: "item", TenantID: 42, SubjectID: "im_user:42:bot:wecom:alice", Content: "private content", Kind: "preference", Status: "active"})
	h := NewMemoryAdminHandler(db)
	role := types.TenantRoleAdmin
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(types.WithCaller(c.Request.Context(), types.Caller{TenantID: 42, UserID: "admin", Role: role}))
	})
	r.POST("/:id/content", h.Content)
	call := func(id, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/"+id+"/content", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}
	body := `{"reason":"排查未生效问题"}`
	if w := call("alice", body); w.Code != 403 || strings.Contains(w.Body.String(), "private content") {
		t.Fatal("ungranted access")
	}
	db.Create(&types.MemoryAdminGrant{TenantID: 42, UserID: "admin", GrantedBy: "owner"})
	if w := call("foreign", body); w.Code != 404 {
		t.Fatal("cross tenant access")
	}
	if w := call("alice", `{}`); w.Code != 400 {
		t.Fatal("reason required")
	}
	if w := call("alice", body); w.Code != 200 || !strings.Contains(w.Body.String(), "private content") {
		t.Fatalf("granted read: %d %s", w.Code, w.Body)
	}
	var logs []types.AuditLog
	db.Find(&logs)
	if len(logs) != 1 || strings.Contains(string(logs[0].Details), "private content") {
		t.Fatal("audit missing or contains private text")
	}
	role = types.TenantRoleViewer
	if w := call("alice", body); w.Code != 403 {
		t.Fatal("grant bypassed role")
	}
	role = types.TenantRoleAdmin
	db.Migrator().DropTable(&types.AuditLog{})
	if w := call("alice", body); w.Code != 500 || strings.Contains(w.Body.String(), "private content") {
		t.Fatal("audit failure exposed content")
	}
}

func TestMemoryAdminOnlyOwnerCanGrantAndRevoke(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "access.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&types.TenantMember{}, &types.MemoryAdminGrant{}, &types.AuditLog{}); err != nil {
		t.Fatal(err)
	}
	db.Create(&types.TenantMember{TenantID: 42, UserID: "admin", Role: types.TenantRoleAdmin, Status: types.TenantMemberStatusActive})
	db.Create(&types.TenantMember{TenantID: 43, UserID: "foreign", Role: types.TenantRoleAdmin, Status: types.TenantMemberStatusActive})
	h := NewMemoryAdminHandler(db)
	role := types.TenantRoleAdmin
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(types.WithCaller(c.Request.Context(), types.Caller{TenantID: 42, UserID: "owner", Role: role}))
	})
	r.PUT("/access", h.Grant)
	call := func(body string) int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/access", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w.Code
	}
	if call(`{"user_id":"admin","enabled":true}`) != 403 {
		t.Fatal("admin granted access")
	}
	role = types.TenantRoleOwner
	if call(`{"user_id":"foreign","enabled":true}`) != 400 {
		t.Fatal("cross-space grant")
	}
	if call(`{"user_id":"admin","enabled":true}`) != 200 {
		t.Fatal("owner grant failed")
	}
	var n int64
	db.Model(&types.MemoryAdminGrant{}).Count(&n)
	if n != 1 {
		t.Fatal("grant missing")
	}
	if call(`{"user_id":"admin","enabled":false}`) != 200 {
		t.Fatal("revoke failed")
	}
	db.Model(&types.MemoryAdminGrant{}).Count(&n)
	if n != 0 {
		t.Fatal("grant not revoked")
	}
	db.Model(&types.AuditLog{}).Count(&n)
	if n != 2 {
		t.Fatal("access audit missing")
	}
}
