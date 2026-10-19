package im

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type correctionKnowledgeServiceStub struct {
	interfaces.KnowledgeService
	gotKB      string
	gotPayload *types.ManualKnowledgePayload
}

func (s *correctionKnowledgeServiceStub) CreateKnowledgeFromManual(_ context.Context, kbID string, payload *types.ManualKnowledgePayload, _ string) (*types.Knowledge, error) {
	s.gotKB = kbID
	s.gotPayload = payload
	return &types.Knowledge{ID: "correction-1"}, nil
}

type correctionKBServiceStub struct {
	interfaces.KnowledgeBaseService
	kb *types.KnowledgeBase
}

func (s *correctionKBServiceStub) GetKnowledgeBaseByID(context.Context, string) (*types.KnowledgeBase, error) {
	return s.kb, nil
}

func TestCorrectCommandPublishesManualKnowledge(t *testing.T) {
	knowledge := &correctionKnowledgeServiceStub{}
	kb := &correctionKBServiceStub{kb: &types.KnowledgeBase{ID: "kb-1", Name: "XBROTHER", Type: types.KnowledgeBaseTypeDocument}}
	cmd := newCorrectCommand("correct", knowledge, kb)
	result, err := cmd.Execute(context.Background(), &CommandContext{
		CorrectionAdmin: true,
		CustomAgent: &types.CustomAgent{Config: types.CustomAgentConfig{
			KBSelectionMode: "selected", KnowledgeBases: []string{"kb-1"},
		}},
	}, []string{"服务器负载高怎么定位?", "||", "先判断CPU还是IO。", "||", "负载高定位"})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if knowledge.gotKB != "kb-1" || knowledge.gotPayload == nil {
		t.Fatalf("correction was not persisted: kb=%q payload=%#v", knowledge.gotKB, knowledge.gotPayload)
	}
	if knowledge.gotPayload.Status != types.ManualKnowledgeStatusPublish {
		t.Fatalf("status = %q, want publish", knowledge.gotPayload.Status)
	}
	if result == nil || result.Content == "" {
		t.Fatal("expected a confirmation reply")
	}
}

func TestCorrectCommandRequiresAllowListedAdmin(t *testing.T) {
	cmd := newCorrectCommand("correct", &correctionKnowledgeServiceStub{}, &correctionKBServiceStub{kb: &types.KnowledgeBase{ID: "kb-1", Type: types.KnowledgeBaseTypeDocument}})
	result, err := cmd.Execute(context.Background(), &CommandContext{}, []string{"q", "a"})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result == nil || result.Content != "该指令仅对已配置的知识库管理员开放。" {
		t.Fatalf("unexpected reply: %#v", result)
	}
}

func TestCorrectCommandValidatesSyntax(t *testing.T) {
	cmd := newCorrectCommand("correct", &correctionKnowledgeServiceStub{}, &correctionKBServiceStub{kb: &types.KnowledgeBase{ID: "kb-1", Type: types.KnowledgeBaseTypeDocument}})
	result, err := cmd.Execute(context.Background(), &CommandContext{CorrectionAdmin: true}, []string{"q-only"})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result == nil || result.Content == "" {
		t.Fatal("expected usage reply")
	}
}
