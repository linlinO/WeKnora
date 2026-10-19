package im

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/feedback"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
)

type feedbackRequestKey struct{}

func imRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(feedbackRequestKey{}).(string); ok && id != "" {
		return id
	}
	return uuid.NewString()
}

func (s *Service) prepareAnswerFeedback(ctx context.Context, req *qaRequest) context.Context {
	ctx = context.WithValue(ctx, feedbackRequestKey{}, uuid.NewString())
	if req.msg.Platform != PlatformWeCom || ResolveMode(req.channel, "websocket") != "websocket" || s.db == nil {
		return ctx
	}
	target := &types.IMFeedbackTarget{
		ID: uuid.NewString(), TenantID: req.channel.TenantID, ChannelID: req.channelID,
		SessionID: req.session.ID, AgentID: req.channel.AgentID,
		UserID: req.msg.UserID, ChatID: req.msg.ChatID,
	}
	if err := s.db.WithContext(ctx).Create(target).Error; err != nil {
		logger.Errorf(ctx, "[IM] Feedback target persistence failed: %v", err)
		return ctx
	}
	if req.msg.Extra == nil {
		req.msg.Extra = make(map[string]string)
	}
	req.msg.Extra["feedback_id"] = target.ID
	return context.WithValue(ctx, feedbackRequestKey{}, target.ID)
}

func (s *Service) handleAnswerFeedback(ctx context.Context, msg *IncomingMessage, channelID string) error {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	e := msg.Feedback
	if msg.Platform != PlatformWeCom || e == nil || e.ID == "" || msg.UserID == "" || msg.MessageID == "" || e.Type < 1 || e.Type > 3 {
		return fmt.Errorf("invalid IM feedback event")
	}
	channel, err := s.GetChannelByID(channelID)
	if err != nil {
		return err
	}
	var target types.IMFeedbackTarget
	if err := s.db.WithContext(ctx).Where("id = ? AND channel_id = ? AND tenant_id = ?", e.ID, channelID, channel.TenantID).Take(&target).Error; err != nil {
		return fmt.Errorf("feedback target not found: %w", err)
	}
	// A group member may rate a group reply, but another group or direct user cannot.
	if target.ChatID != msg.ChatID || (target.ChatID == "" && target.UserID != msg.UserID) {
		return fmt.Errorf("feedback chat does not match original answer")
	}
	var answer types.Message
	if err := s.db.WithContext(ctx).Where("session_id = ? AND request_id = ? AND role = ? AND is_completed = ?", target.SessionID, target.ID, "assistant", true).Take(&answer).Error; err != nil {
		return fmt.Errorf("feedback answer not ready: %w", err)
	}
	question, err := feedback.Question(ctx, s.db, &answer)
	if err != nil {
		return err
	}
	rating, status := types.AnswerFeedbackUseful, types.AnswerFeedbackPending
	if e.Type == 2 {
		rating = types.AnswerFeedbackNotUseful
	}
	if e.Type == 3 {
		rating, status = "", types.AnswerFeedbackWithdrawn
	}
	row := &types.AnswerFeedback{
		TenantID: target.TenantID, UserID: "wecom:" + msg.UserID, Source: "wecom", ChannelID: channelID,
		SessionID: target.SessionID, MessageID: answer.ID, AgentID: target.AgentID,
		Question: question, Answer: answer.Content, Rating: rating, Status: status,
		Reason: feedbackReason(e), LastEventID: msg.MessageID, LastEventAt: e.CreatedAt,
	}
	if err := feedback.Save(ctx, s.db, row); err != nil {
		return err
	}
	logger.Infof(ctx, "[IM] Feedback stored: channel=%s feedback=%s source=wecom status=%s", channelID, row.ID, row.Status)
	return nil
}

func feedbackReason(e *AnswerFeedbackEvent) string {
	if e.Type != 2 {
		return ""
	}
	names := map[int]string{1: "与问题无关", 2: "内容不完整", 3: "内容有错误", 4: "数据分析错误"}
	parts := []string{}
	seen := map[int]bool{}
	for _, code := range e.InaccurateReasonList {
		if label := names[code]; label != "" && !seen[code] {
			parts = append(parts, label)
			seen[code] = true
		}
	}
	if content := strings.TrimSpace(e.Content); content != "" {
		parts = append(parts, content)
	}
	return strings.Join(parts, "；")
}
