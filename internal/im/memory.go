package im

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
)

func subscribeIMMemory(bus *event.EventBus, answer *types.Message, mu *sync.Mutex) {
	bus.On(event.EventAgentFinalAnswer, func(_ context.Context, evt event.Event) error {
		if data, ok := evt.Data.(event.AgentFinalAnswerData); ok && data.ReadinessKind != "" {
			mu.Lock()
			answer.GenerationStatus = data.ReadinessKind
			mu.Unlock()
		}
		if data, ok := evt.Data.(event.AgentFinalAnswerData); ok && data.Truncated {
			mu.Lock()
			answer.GenerationStatus = "truncated"
			mu.Unlock()
		}
		return nil
	})
	bus.On(event.EventMemoryRecalled, func(_ context.Context, evt event.Event) error {
		data, ok := evt.Data.(event.MemoryRecalledData)
		if !ok {
			return nil
		}
		var items []types.UsedMemory
		switch value := data.Memories.(type) {
		case []types.UsedMemory:
			items = value
		case types.UsedMemories:
			items = value
		}
		mu.Lock()
		if data.Diagnostic != nil {
			raw, _ := json.Marshal(data.Diagnostic)
			answer.MemoryDiagnostic = types.JSON(raw)
		}
		answer.UsedMemories = types.MergeUsedMemories(answer.UsedMemories, items)
		mu.Unlock()
		return nil
	})
}

func imGenerationStatus(err error, previous ...string) string {
	if err == nil {
		if len(previous) > 0 && previous[0] != "" {
			return previous[0]
		}
		return "completed"
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "interrupted"
	}
	return "failed"
}

func (s *Service) rememberIMTurn(ctx context.Context, user, answer *types.Message, agent *types.CustomAgent) {
	if s.memoryService == nil || user == nil || answer == nil || !user.MemoryEligible || answer.GenerationStatus != "completed" {
		return
	}
	if agent != nil {
		ctx = types.ApplyAgentMemoryPreference(ctx, agent.Config.MemoryEnabled)
	}
	principal, ok := types.PrincipalFromContext(ctx)
	if !ok || principal.StorageID() != user.MemoryOwnerID {
		return
	}
	refs := make([]types.MemoryDocAffinity, 0, len(answer.KnowledgeReferences))
	seen := map[string]bool{}
	for _, ref := range answer.KnowledgeReferences {
		if ref != nil && ref.KnowledgeID != "" && !seen[ref.KnowledgeID] {
			seen[ref.KnowledgeID] = true
			refs = append(refs, types.MemoryDocAffinity{KnowledgeID: ref.KnowledgeID, KnowledgeBaseID: ref.KnowledgeBaseID, Title: ref.KnowledgeTitle})
		}
	}
	s.memoryService.RecordAnswerSources(ctx, refs)
	s.memoryService.ScheduleExtraction(ctx, user.SessionID, answer.ID, answer.ModelID)
}

type clearMemoryChallenge struct {
	Code    string
	Expires time.Time
}

// Management stays in the sender's private chat and bypasses generation entirely.
func (s *Service) handleMemoryMessage(ctx context.Context, msg *IncomingMessage, adapter Adapter) (bool, error) {
	if s.memoryService == nil || msg == nil {
		return false, nil
	}
	text := strings.TrimSpace(msg.Content)
	statement, explicit := types.DetectExplicitMemory(text)
	verb, arg, _ := strings.Cut(text, " ")
	verbs := map[string]bool{"我的记忆": true, "暂停记忆": true, "恢复记忆": true, "清空我的记忆": true, "确认清空记忆": true, "确认记忆": true, "拒绝记忆": true, "删除记忆": true}
	if !explicit && !verbs[verb] {
		return false, nil
	}
	reply := func(text string) (bool, error) {
		return true, adapter.SendReply(ctx, msg, &ReplyMessage{Content: text, IsFinal: true})
	}
	if msg.ChatType != ChatTypeDirect {
		return reply("请私聊小济管理个人记忆。群聊不会展示或修改你的私人记忆。")
	}
	if explicit {
		kind := types.MemoryKindFact
		if strings.HasPrefix(statement, "偏好：") || strings.HasPrefix(statement, "偏好:") {
			kind = types.MemoryKindPreference
			statement = strings.TrimSpace(strings.TrimLeft(statement[len("偏好"):], "：:"))
		}
		item, err := s.memoryService.Remember(ctx, types.MemoryItem{Kind: kind, Content: statement, Origin: types.MemoryOriginExplicit, Importance: 4, SourceMessageID: msg.MessageID})
		if err != nil {
			return reply("未保存记忆：请确认空间、智能体和个人记忆已开启；删除过的内容不会自动重新保存。发送“我的记忆”查看。")
		}
		return reply("已记住：" + item.Content + "\n发送“我的记忆”可查看、删除或暂停使用。")
	}
	if verb == "暂停记忆" || verb == "恢复记忆" {
		enabled := verb == "恢复记忆"
		if err := s.memoryService.SetEnabled(ctx, enabled); err != nil {
			return reply("记忆设置未保存，请稍后重试。")
		}
		if enabled {
			return reply("已恢复个人记忆，将从之后的新问题继续提炼；仍受空间和智能体开关控制。")
		}
		return reply("已暂停个人记忆的提炼和使用，已有条目保留。问答记录和空间统计仍按平台规则保留。")
	}
	p, ok := types.PrincipalFromContext(ctx)
	if !ok {
		return reply("无法识别你的身份。")
	}
	key := "im:memory:clear:" + p.StorageID()
	if verb == "清空我的记忆" {
		code := uuid.NewString()[:8]
		if s.redis != nil {
			if err := s.redis.Set(ctx, key, code, 5*time.Minute).Err(); err != nil {
				return reply("无法生成清空确认，请稍后重试。")
			}
		} else {
			s.memoryClear.Store(key, clearMemoryChallenge{code, time.Now().Add(5 * time.Minute)})
		}
		return reply("这会删除你的个人记忆、关注方向和常用资料记录。若确认，请在 5 分钟内发送：\n确认清空记忆 " + code + "\n不会删除聊天记录或知识库。")
	}
	if verb == "确认清空记忆" {
		valid := false
		if s.redis != nil {
			result, err := s.redis.Eval(ctx, `if redis.call('GET', KEYS[1]) == ARGV[1] then redis.call('DEL', KEYS[1]); return 1 end; return 0`, []string{key}, strings.TrimSpace(arg)).Int()
			valid = err == nil && result == 1
		} else if v, ok := s.memoryClear.LoadAndDelete(key); ok {
			c := v.(clearMemoryChallenge)
			valid = c.Code == strings.TrimSpace(arg) && time.Now().Before(c.Expires)
		}
		if !valid {
			return reply("确认码不正确或已过期。请先发送“清空我的记忆”。")
		}
		n, err := s.memoryService.Clear(ctx)
		if err != nil {
			return reply("清空失败，请稍后重试。")
		}
		return reply(fmt.Sprintf("已清空 %d 条个人记忆。后续新对话仍可形成新记忆；如不希望继续记录，请发送“暂停记忆”。", n))
	}
	items, total, err := s.memoryService.ListItems(ctx, "", 200, 0)
	if err != nil {
		logger.Warnf(ctx, "IM memory list: %v", err)
		return reply("读取记忆失败，请稍后重试。")
	}
	if verb == "我的记忆" {
		settings, err := s.memoryService.GetSettings(ctx)
		if err != nil {
			return reply("读取记忆设置失败，请稍后重试。")
		}
		b := strings.Builder{}
		fmt.Fprintf(&b, "**我的记忆**\n共 %d 条，仅属于你在当前机器人和空间的身份。\n", total)
		if !settings.UserEnabled {
			b.WriteString("个人记忆已暂停。\n")
		}
		if !settings.WorkspaceEnabled {
			b.WriteString("空间长期记忆未开启。\n")
		}
		count := 0
		page, _ := strconv.Atoi(strings.TrimSpace(arg))
		if page < 1 {
			page = 1
		}
		position := 0
		for _, item := range items {
			if item.Status != "active" && item.Status != "pending" {
				continue
			}
			position++
			if position <= (page-1)*20 {
				continue
			}
			if count >= 20 {
				break
			}
			count++
			label := "生效中"
			if item.Status == "pending" {
				label = "待确认"
			}
			id := item.ID
			if len(id) > 8 {
				id = id[:8]
			}
			fmt.Fprintf(&b, "\n`%s` [%s] %s\n", id, label, item.Content)
		}
		if count == 0 {
			b.WriteString("\n本页暂无生效或待确认记忆。可以说“记住：偏好：回答先给结论再给步骤”。\n")
		}
		if count == 20 {
			fmt.Fprintf(&b, "\n第 %d 页，查看下一页请发送：我的记忆 %d\n", page, page+1)
		}
		topics, _, _ := s.memoryService.ListTopics(ctx, 5, 0)
		if len(topics) > 0 {
			b.WriteString("\n关注话题正在观察中；达到空间设置的次数后再形成长期关注。\n")
		}
		b.WriteString("\n操作：确认记忆 编号／拒绝记忆 编号／删除记忆 编号\n暂停记忆／恢复记忆／清空我的记忆\n自动提炼会延迟处理，推测内容需确认后才生效。")
		return reply(b.String())
	}
	var target *types.MemoryItem
	arg = strings.TrimSpace(arg)
	if len(arg) < 8 {
		return reply("请带上记忆编号，例如“删除记忆 a1b2c3d4”。发送“我的记忆”查看编号。")
	}
	for _, item := range items {
		if strings.HasPrefix(item.ID, arg) {
			if target != nil {
				return reply("编号不唯一，请提供完整编号。")
			}
			target = item
		}
	}
	if target == nil {
		return reply("没有找到你的这条记忆，请发送“我的记忆”核对编号。")
	}
	switch verb {
	case "确认记忆":
		_, err = s.memoryService.ConfirmItem(ctx, target.ID)
	case "拒绝记忆":
		err = s.memoryService.RejectItem(ctx, target.ID)
	case "删除记忆":
		err = s.memoryService.DeleteItem(ctx, target.ID)
	}
	if err != nil {
		return reply("操作未完成，请核对记忆状态后重试。")
	}
	return reply("已完成。发送“我的记忆”查看最新记录。")
}
