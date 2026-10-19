package chatpipeline

import (
	"context"
	"github.com/Tencent/WeKnora/internal/clarification"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"testing"
)

func TestClarificationMultiTurnAndIsolation(t *testing.T) {
	db, e := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "guard.db")), &gorm.Config{})
	require.NoError(t, e)
	require.NoError(t, db.AutoMigrate(&clarification.State{}))
	model := &visionProbeChat{}
	p := &PluginClarificationGuard{models: &stubModelService{model: model}, store: clarification.Store{DB: db}}
	ctx := types.WithPrincipal(context.Background(), types.Principal{Type: types.PrincipalIMUser, ID: "alice"})
	makeCM := func(q string) *types.ChatManage {
		return &types.ChatManage{PipelineRequest: types.PipelineRequest{Query: q, KnowledgeOnly: true, TenantID: 1, SessionID: "s", AgentID: "a"}, PipelineContext: types.PipelineContext{UserMessageID: "u"}}
	}
	first := makeCM("KE升级失败了怎么办")
	model.response = &types.ChatResponse{Content: `{"mode":"incident","relation":"new","facts":{"component":"KE"}}`}
	reached := false
	next := func() *PluginError { reached = true; return nil }
	require.Equal(t, "readiness_stop", p.OnEvent(ctx, types.QUESTION_READINESS, first, next).ErrorType)
	require.False(t, reached)
	require.Equal(t, "clarification", first.ReadinessKind)
	second := makeCM("端口占用")
	second.History = []*types.History{{Query: "other", Answer: "危险的旧猜测"}}
	model.response = &types.ChatResponse{Content: `{"mode":"incident","relation":"continue","facts":{"component":"KE","symptom":"端口占用"}}`}
	require.Nil(t, p.OnEvent(ctx, types.QUESTION_READINESS, second, next))
	require.True(t, reached)
	require.Contains(t, second.Query, "KE升级失败")
	require.Empty(t, second.History)
	second.MergeResult = []*types.SearchResult{{Content: "PostgreSQL数据库适用：流程A"}, {Content: "MongoDB数据库适用：流程B"}}
	model.response = &types.ChatResponse{Content: `{"decision":"clarify","matches":[],"conditions":[{"field":"database","source":1,"quote":"PostgreSQL数据库适用"}]}`}
	require.Equal(t, "readiness_stop", p.OnEvent(ctx, types.EVIDENCE_READINESS, second, next).ErrorType)
	require.Contains(t, second.ReadinessReply, "数据库")
	third := makeCM("使用PostgreSQL")
	model.response = &types.ChatResponse{Content: `{"mode":"incident","relation":"continue","facts":{"component":"KE","symptom":"端口占用","database":"PostgreSQL"}}`}
	require.Nil(t, p.OnEvent(ctx, types.QUESTION_READINESS, third, next))
	third.MergeResult = second.MergeResult
	model.response = &types.ChatResponse{Content: `{"decision":"answer","matches":[1],"conditions":[]}`}
	require.Nil(t, p.OnEvent(ctx, types.EVIDENCE_READINESS, third, next))
	require.Len(t, third.MergeResult, 1)
	require.Contains(t, third.MergeResult[0].Content, "流程A")
	// New unrelated factual question clears the stored incident.
	model.response = &types.ChatResponse{Content: `{"mode":"general","relation":"new","facts":{}}`}
	require.Nil(t, p.OnEvent(ctx, types.QUESTION_READINESS, makeCM("xtwin是什么"), next))
	state, err := p.store.Load(ctx, first.IncidentScope)
	require.NoError(t, err)
	require.Empty(t, state.Input)
}
func TestClarificationFailClosedAndNoAssumedImageContent(t *testing.T) {
	for _, body := range []string{"broken json", `{"mode":"invalid","relation":"new","facts":{"component":"invented"}}`} {
		model := &visionProbeChat{response: &types.ChatResponse{Content: body}}
		p := &PluginClarificationGuard{models: &stubModelService{model: model}}
		cm := &types.ChatManage{PipelineRequest: types.PipelineRequest{Query: "升级失败", KnowledgeOnly: true}}
		err := p.OnEvent(context.Background(), types.QUESTION_READINESS, cm, func() *PluginError { t.Fatal("unsafe generation"); return nil })
		require.NotNil(t, err)
		require.Equal(t, "unavailable", cm.ReadinessKind)
	}
	model := &visionProbeChat{response: &types.ChatResponse{Content: `{"mode":"incident","relation":"new","facts":{"component":"KE","symptom":"ulimit-check max process过小"}}`}}
	p := &PluginClarificationGuard{models: &stubModelService{model: model}}
	cm := &types.ChatManage{PipelineRequest: types.PipelineRequest{Query: "KE安装这个错误怎么解决", KnowledgeOnly: true}, PipelineState: types.PipelineState{ImageDescription: "ulimit-check max process过小"}}
	require.Nil(t, p.OnEvent(context.Background(), types.QUESTION_READINESS, cm, func() *PluginError { return nil }))
	require.Contains(t, cm.Query, "ulimit-check max process过小")
}
