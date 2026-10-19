package im

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// CorrectCommand publishes an administrator correction into the agent's
// document knowledge base. The command deliberately uses the normal manual
// knowledge ingestion path so the next retrieval sees the correction after
// the background embedding task completes.
type CorrectCommand struct {
	name             string
	knowledgeService interfaces.KnowledgeService
	kbService        interfaces.KnowledgeBaseService
}

func newCorrectCommand(name string, knowledgeService interfaces.KnowledgeService, kbService interfaces.KnowledgeBaseService) *CorrectCommand {
	return &CorrectCommand{name: name, knowledgeService: knowledgeService, kbService: kbService}
}

func (c *CorrectCommand) Name() string { return c.name }

func (c *CorrectCommand) Description() string {
	return "管理员发布纠错：/correct 问题 || 标准答案 || 相似问法（可选）"
}

func (c *CorrectCommand) Execute(ctx context.Context, cmdCtx *CommandContext, args []string) (*CommandResult, error) {
	if cmdCtx == nil || !cmdCtx.CorrectionAdmin {
		return &CommandResult{Content: "该指令仅对已配置的知识库管理员开放。"}, nil
	}
	parts := strings.SplitN(strings.Join(args, " "), "||", 3)
	if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return &CommandResult{Content: "用法：`/correct 问题 || 标准答案 || 相似问法1,相似问法2`"}, nil
	}
	if c.knowledgeService == nil || c.kbService == nil {
		return nil, fmt.Errorf("correction services are not configured")
	}

	question := strings.TrimSpace(parts[0])
	answer := strings.TrimSpace(parts[1])
	var similar []string
	if len(parts) == 3 {
		for _, item := range strings.Split(parts[2], ",") {
			if item = strings.TrimSpace(item); item != "" {
				similar = append(similar, item)
			}
		}
	}

	kb, err := correctionKnowledgeBase(ctx, cmdCtx, c.kbService)
	if err != nil {
		return nil, err
	}
	var content strings.Builder
	content.WriteString("# 权威纠错条目\n\n## 用户问题\n")
	content.WriteString(question)
	content.WriteString("\n\n## 标准答案\n")
	content.WriteString(answer)
	if len(similar) > 0 {
		content.WriteString("\n\n## 相似问法\n")
		for _, item := range similar {
			content.WriteString("- ")
			content.WriteString(item)
			content.WriteString("\n")
		}
	}

	knowledge, err := c.knowledgeService.CreateKnowledgeFromManual(ctx, kb.ID, &types.ManualKnowledgePayload{
		Title:   "知识纠错：" + question,
		Content: content.String(),
		Status:  types.ManualKnowledgeStatusPublish,
		Channel: types.ChannelWecom,
	}, types.ChannelWecom)
	if err != nil {
		return nil, err
	}
	return &CommandResult{Content: fmt.Sprintf("✅ 纠错已发布到知识库「%s」。知识条目 `%s` 正在重新索引，完成后后续相同问题会优先使用这条标准答案。", kb.Name, knowledge.ID)}, nil
}

func correctionKnowledgeBase(ctx context.Context, cmdCtx *CommandContext, service interfaces.KnowledgeBaseService) (*types.KnowledgeBase, error) {
	targetName := strings.TrimSpace(os.Getenv("WEKNORA_IM_CORRECTION_KB_NAME"))
	if targetName != "" {
		all, err := service.ListKnowledgeBases(ctx)
		if err != nil {
			return nil, err
		}
		for _, kb := range all {
			if kb != nil && kb.Type == types.KnowledgeBaseTypeDocument &&
				(strings.EqualFold(kb.Name, targetName) || kb.ID == targetName) {
				return kb, nil
			}
		}
		return nil, fmt.Errorf("纠错目标知识库不存在：%s", targetName)
	}
	if cmdCtx.CustomAgent != nil && cmdCtx.CustomAgent.Config.KBSelectionMode == "selected" {
		for _, id := range cmdCtx.CustomAgent.Config.KnowledgeBases {
			if kb, err := service.GetKnowledgeBaseByID(ctx, id); err == nil && kb != nil && kb.Type == types.KnowledgeBaseTypeDocument {
				return kb, nil
			}
		}
	}
	all, err := service.ListKnowledgeBases(ctx)
	if err != nil {
		return nil, err
	}
	for _, kb := range all {
		if kb != nil && kb.Type == types.KnowledgeBaseTypeDocument {
			return kb, nil
		}
	}
	return nil, fmt.Errorf("没有可写入的文档知识库")
}
