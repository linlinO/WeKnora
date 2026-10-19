package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/types"
)

func TestKnowledgeOnlyOverridesFallbackAndWebSearch(t *testing.T) {
	s := &sessionService{cfg: &config.Config{Conversation: &config.ConversationConfig{}}}
	agent := &types.CustomAgent{Config: types.CustomAgentConfig{KnowledgeOnly: true, FallbackStrategy: "model", WebSearchEnabled: true}}
	cm := &types.ChatManage{PipelineRequest: types.PipelineRequest{WebSearchEnabled: true}}
	s.applyAgentOverridesToChatManage(context.Background(), agent, cm)
	if !cm.KnowledgeOnly || cm.WebSearchEnabled || cm.FallbackStrategy != types.FallbackStrategyFixed {
		t.Fatalf("knowledge boundary not enforced: %+v", cm.PipelineRequest)
	}
}
