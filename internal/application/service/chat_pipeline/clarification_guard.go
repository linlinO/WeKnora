package chatpipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/clarification"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

type PluginClarificationGuard struct {
	models interfaces.ModelService
	store  clarification.Store
}

func NewPluginClarificationGuard(events *EventManager, models interfaces.ModelService, db *gorm.DB) *PluginClarificationGuard {
	p := &PluginClarificationGuard{models: models, store: clarification.Store{DB: db}}
	events.Register(p)
	return p
}
func (*PluginClarificationGuard) ActivationEvents() []types.EventType {
	return []types.EventType{types.QUESTION_READINESS, types.EVIDENCE_READINESS}
}
func (p *PluginClarificationGuard) OnEvent(ctx context.Context, kind types.EventType, cm *types.ChatManage, next func() *PluginError) *PluginError {
	if !cm.KnowledgeOnly {
		return next()
	}
	if kind == types.QUESTION_READINESS {
		if cm.ReadinessChecked || len(cm.Images) > 0 && cm.ImageDescription == "" {
			return next()
		}
		cm.ReadinessChecked = true
		return p.assess(ctx, cm, next)
	}
	if !cm.IncidentActive {
		return next()
	}
	return p.checkEvidence(ctx, cm, next)
}
func (p *PluginClarificationGuard) call(ctx context.Context, cm *types.ChatManage, prompt string, input any, out any) error {
	callCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	id := cm.QueryUnderstandModelID
	if id == "" {
		id = cm.ChatModelID
	}
	model, err := p.models.GetChatModel(callCtx, id)
	if err != nil {
		return err
	}
	data, _ := json.Marshal(input)
	off := false
	response, err := model.Chat(types.WithLLMCallMetadata(callCtx, "question_readiness", ""), []chat.Message{{Role: "system", Content: prompt}, {Role: "user", Content: string(data)}}, &chat.ChatOptions{Temperature: 0, MaxCompletionTokens: 2048, Thinking: &off})
	if err != nil {
		return err
	}
	if response == nil || response.FinishReason == "length" || response.FinishReason == types.FinishReasonIncomplete {
		return errors.New("incomplete readiness assessment")
	}
	content := strings.TrimSpace(response.Content)
	fence := strings.Repeat(string(rune(96)), 3)
	content = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(content, fence+"json"), fence), fence))
	return json.Unmarshal([]byte(content), out)
}
func (p *PluginClarificationGuard) assess(ctx context.Context, cm *types.ChatManage, next func() *PluginError) *PluginError {
	current := cm.Query
	if cm.ImageDescription != "" {
		current += "\n当前图片识别：" + cm.ImageDescription
	}
	// Attachments and quoted assistant text may contain KB solutions. They are
	// deliberately not trusted as facts about the current incident.
	if len([]rune(current)) > 8000 {
		return p.unavailable(cm)
	}
	principal, ok := types.PrincipalFromContext(ctx)
	if ok && cm.UserMessageID != "" {
		tenant := cm.TenantID
		if id, ok := ctx.Value(types.SessionTenantIDContextKey).(uint64); ok {
			tenant = id
		}
		cm.IncidentScope = clarification.Scope(tenant, cm.SessionID, principal.StorageID(), cm.AgentID)
	}
	old, err := p.store.Load(ctx, cm.IncidentScope)
	if err != nil {
		return p.unavailable(cm)
	}
	var a clarification.Assessment
	if err = p.call(ctx, cm, clarification.AssessmentPrompt, map[string]any{"current": current, "previous": old.Input, "missing": old.Missing}, &a); err != nil {
		pipelineWarn(ctx, "Clarification", "assessment_unavailable", map[string]interface{}{"session_id": cm.SessionID})
		return p.unavailable(cm)
	}
	input, facts, missing, err := clarification.ValidateAssessment(a, current, old.Input, old.Facts)
	if err != nil {
		pipelineWarn(ctx, "Clarification", "invalid_assessment", map[string]interface{}{"session_id": cm.SessionID, "reason": err.Error()})
		return p.unavailable(cm)
	}
	pipelineInfo(ctx, "Clarification", "assessed", map[string]interface{}{"mode": a.Mode, "relation": a.Relation, "has_previous": old.Input != "", "missing": missing})
	if a.Mode == "general" {
		if err = p.store.Clear(ctx, cm.IncidentScope); err != nil {
			return p.unavailable(cm)
		}
		return next()
	}
	cm.IncidentActive = true
	cm.IncidentInput = input
	cm.IncidentFacts = facts
	if a.Relation == "continue" {
		cm.IncidentRounds = old.Rounds
		// Only repeated lack of progress triggers handoff; a new decisive
		// field after successful supplementation starts a fresh question.
		for _, field := range strings.Split(old.Missing, ",") {
			if facts[field] != "" {
				cm.IncidentRounds = 0
				break
			}
		}
	}
	// Only user-supplied incident facts may determine the retrieval query or
	// answer. Previously generated advice is not evidence for a new diagnosis.
	cm.Query = input
	cm.RewriteQuery = input
	cm.History = nil
	cm.QuotedContext = ""
	cm.SummaryConfig.Prompt += "\n故障处理必须符合用户本次明确提供的事实。不能用历史回答、个人记忆或其他案例补齐用户的版本、环境、报错。只使用本次提供的适用资料，缺少处理依据时明确说明，禁止猜测修复命令。"
	if len(missing) > 0 {
		return p.ask(ctx, cm, missing, false)
	}
	// Preserve the factual context across a follow-up, including a no-match turn.
	if err = p.save(ctx, cm, nil, 0); err != nil {
		return p.unavailable(cm)
	}
	return next()
}
func (p *PluginClarificationGuard) checkEvidence(ctx context.Context, cm *types.ChatManage, next func() *PluginError) *PluginError {
	if len(cm.MergeResult) == 0 {
		return ErrSearchNothing
	}
	sources := []string{}
	numbered := []map[string]any{}
	for i, r := range cm.MergeResult {
		if r == nil {
			return p.unavailable(cm)
		}
		text := clarification.Clip(r.KnowledgeTitle+"\n"+r.Content, 3500)
		sources = append(sources, text)
		numbered = append(numbered, map[string]any{"id": i + 1, "text": text})
		if i == 9 {
			break
		}
	}
	var e clarification.Evidence
	if err := p.call(ctx, cm, clarification.EvidencePrompt, map[string]any{"incident": cm.IncidentInput, "facts": cm.IncidentFacts, "sources": numbered}, &e); err != nil {
		return p.unavailable(cm)
	}
	ids, missing, err := clarification.ValidateEvidence(e, sources, cm.IncidentFacts)
	if err != nil {
		pipelineWarn(ctx, "Clarification", "invalid_evidence", map[string]interface{}{"session_id": cm.SessionID, "reason": err.Error()})
		return p.unavailable(cm)
	}
	if e.Decision == "clarify" {
		return p.ask(ctx, cm, missing, true)
	}
	if e.Decision == "no_match" {
		cm.ReadinessReply = "你提供的信息已能说明故障，但本次知识库检索没有找到适用于该场景的处理依据。请交由运维人员核对具体报错，确认方案后可通过回答反馈补充纠错知识。"
		cm.ReadinessKind = "no_match"
		return &PluginError{ErrorType: "readiness_stop"}
	}
	filtered := make([]*types.SearchResult, 0, len(ids))
	for _, i := range ids {
		filtered = append(filtered, cm.MergeResult[i])
	}
	cm.MergeResult = filtered
	if err = p.save(ctx, cm, nil, 0); err != nil {
		return p.unavailable(cm)
	}
	return next()
}
func (p *PluginClarificationGuard) save(ctx context.Context, cm *types.ChatManage, missing []string, rounds int) error {
	return p.store.Save(ctx, clarification.State{Key: cm.IncidentScope, Input: cm.IncidentInput, Facts: cm.IncidentFacts, Missing: strings.Join(missing, ","), Rounds: rounds})
}
func (p *PluginClarificationGuard) ask(ctx context.Context, cm *types.ChatManage, missing []string, evidence bool) *PluginError {
	cm.IncidentRounds++
	if err := p.save(ctx, cm, missing, cm.IncidentRounds); err != nil {
		return p.unavailable(cm)
	}
	cm.ReadinessKind = "clarification"
	if cm.IncidentRounds >= 3 {
		cm.ReadinessKind = "handoff"
	}
	cm.ReadinessReply = clarification.Reply(missing, cm.IncidentRounds, evidence)
	pipelineInfo(ctx, "Clarification", "awaiting_user", map[string]interface{}{"session_id": cm.SessionID, "fields": missing, "round": cm.IncidentRounds})
	return &PluginError{ErrorType: "readiness_stop"}
}
func (p *PluginClarificationGuard) unavailable(cm *types.ChatManage) *PluginError {
	cm.ReadinessKind = "unavailable"
	cm.ReadinessReply = "本次问题信息或方案适用性检查未完成，暂不提供可能不适用的处理步骤。请稍后重试；也可以补充具体产品、失败步骤和报错原文后再问。"
	return &PluginError{ErrorType: "readiness_stop", Err: fmt.Errorf("readiness check unavailable")}
}
