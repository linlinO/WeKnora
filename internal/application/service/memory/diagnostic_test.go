package memory

import (
	"context"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMemoryDiagnosticReasonsAndGroupIsolation(t *testing.T) {
	svc, _, tenants := newMemoryHarness(t)
	ctx := enabledCtx(t, tenants, 1, "alice")
	require.Equal(t, "no_subject", svc.Recall(ctx, "问题").Diagnostic.Reason)
	require.Equal(t, "agent_disabled", svc.Recall(types.WithMemoryDisabled(ctx), "问题").Diagnostic.Reason)
	_, err := svc.Remember(ctx, types.MemoryItem{Kind: "fact", Content: "私人项目预算二十万元", Origin: "explicit"})
	require.NoError(t, err)
	group := context.WithValue(ctx, types.MemoryPreferencesOnlyContextKey, true)
	d := svc.Recall(group, "预算").Diagnostic
	require.Equal(t, "group_preferences", d.Mode)
	require.Equal(t, "no_active_preference", d.Reason)
	require.Zero(t, d.Used)
	_, err = svc.Remember(ctx, types.MemoryItem{Kind: "preference", Content: "回答先给结论", Origin: "explicit"})
	require.NoError(t, err)
	recall := svc.Recall(group, "预算")
	require.Equal(t, "recalled", recall.Diagnostic.Reason)
	require.Equal(t, 1, recall.Diagnostic.Used)
	require.NotContains(t, recall.Prompt, "二十万元")
	require.NoError(t, svc.SetEnabled(ctx, false))
	require.Equal(t, "user_disabled", svc.Recall(ctx, "问题").Diagnostic.Reason)
	tenants.set(1, &types.MemoryConfig{Enabled: false})
	require.Equal(t, "workspace_disabled", svc.Recall(ctx, "问题").Diagnostic.Reason)
}
