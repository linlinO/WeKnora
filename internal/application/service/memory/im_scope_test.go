package memory

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestIMExtractionUsesOnlyAttributedNewMessages(t *testing.T) {
	svc, tenants, messages, models, _ := newExtractionHarness(t)
	base := enabledCtx(t, tenants, 1, "alice")
	ctx := types.WithPrincipal(base, types.Principal{Type: types.PrincipalIMUser, ID: "1:bot:wecom:alice"})
	p, _ := types.PrincipalFromContext(ctx)
	now := time.Now()
	messages.messages = []*types.Message{
		{ID: "old", Role: "user", Content: "历史记录不能自动回填", CreatedAt: now.Add(-time.Hour)},
		{ID: "bob", Role: "user", MemoryOwnerID: "im_user:1:bot:wecom:bob", MemoryUserText: "我是数据库管理员", MemoryEligible: true, CreatedAt: now.Add(-time.Minute)},
		{ID: "alice", Role: "user", Content: "我喜欢简短回答。引用别人：我是财务", MemoryOwnerID: p.StorageID(), MemoryUserText: "我喜欢简短回答", MemoryEligible: true, CreatedAt: now},
	}
	models.response = `{"memories":[{"action":"add","kind":"preference","content":"喜欢简短回答","source":1}],"topics":[]}`
	require.NoError(t, svc.Handle(context.Background(), extractTask(t, types.MemoryExtractPayload{TenantID: 1, SubjectID: p.StorageID(), SessionID: "shared", ChatModelID: "model"})))
	items, total, err := svc.ListItems(ctx, "active", 20, 0)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, "alice", items[0].SourceMessageID)
	for _, m := range messages.messages {
		text, ok := extractionUserText(ctx, m)
		if m.ID == "alice" {
			require.True(t, ok)
			require.Equal(t, "我喜欢简短回答", text)
		} else {
			require.False(t, ok)
		}
	}
	bob := types.WithPrincipal(base, types.Principal{Type: types.PrincipalIMUser, ID: "1:bot:wecom:bob"})
	_, total, err = svc.ListItems(bob, "active", 20, 0)
	require.NoError(t, err)
	require.Zero(t, total)
}

func TestGroupMemoryOnlyExposesPreferencesThroughEveryReadPath(t *testing.T) {
	svc, _, tenants := newMemoryHarness(t)
	ctx := enabledCtx(t, tenants, 1, "alice")
	_, err := svc.Remember(ctx, types.MemoryItem{Kind: "preference", Content: "回答先给结论", Origin: "manual"})
	require.NoError(t, err)
	_, err = svc.Remember(ctx, types.MemoryItem{Kind: "fact", Content: "我的私人项目预算是二十万元", Origin: "explicit"})
	require.NoError(t, err)
	group := context.WithValue(ctx, types.MemoryPreferencesOnlyContextKey, true)
	result := svc.Recall(group, "私人项目预算")
	require.Contains(t, result.Prompt, "回答先给结论")
	require.NotContains(t, result.Prompt, "二十万元")
	for _, item := range svc.SearchMemory(group, "私人项目预算", 20).Items {
		require.Equal(t, "preference", item.Kind)
	}
	require.Empty(t, svc.RetrievalContextFor(group))
	require.Empty(t, svc.DocumentAffinity(group, []string{"doc"}))
	require.Contains(t, svc.Recall(ctx, "私人项目预算").Prompt, "二十万元")
}

func TestClearAndPauseRejectInFlightExtractionWrites(t *testing.T) {
	svc, _, tenants := newMemoryHarness(t)
	ctx := enabledCtx(t, tenants, 1, "alice")
	scope, _ := ResolveScope(ctx)
	_, err := svc.repo.EnsureSubject(ctx, scope)
	require.NoError(t, err)
	old := context.WithValue(ctx, types.MemorySourceTimeContextKey, time.Now().Add(-time.Minute))
	_, err = svc.Clear(ctx)
	require.NoError(t, err)
	_, err = svc.Remember(old, types.MemoryItem{Kind: "fact", Content: "来自清空前的后台任务", Origin: "extracted"})
	require.ErrorIs(t, err, types.ErrMemoryConflict)
	_, err = svc.Remember(ctx, types.MemoryItem{Kind: "preference", Content: "清空之后请简短回答", Origin: "explicit"})
	require.NoError(t, err)
	require.NoError(t, svc.SetEnabled(ctx, false))
	require.NoError(t, svc.SetEnabled(ctx, true))
	_, err = svc.Remember(old, types.MemoryItem{Kind: "fact", Content: "暂停之前的提炼结果", Origin: "extracted"})
	require.ErrorIs(t, err, types.ErrMemoryConflict)
}
