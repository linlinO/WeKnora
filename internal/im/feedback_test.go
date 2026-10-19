package im

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
)

func TestIMFeedbackPersistsAndSurvivesServiceRestart(t *testing.T) {
	db := newLifecycleTestDB(t)
	for _, file := range []string{"000028_answer_feedback.up.sql", "000029_im_answer_feedback.up.sql"} {
		sql, err := os.ReadFile(filepath.Join("../../migrations/sqlite", file))
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Exec(string(sql)).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.AutoMigrate(&types.Message{}); err != nil {
		t.Fatal(err)
	}
	channel := &IMChannel{ID: "channel", TenantID: 42, AgentID: "agent", Platform: "wecom", Mode: "websocket"}
	if err := db.Create(channel).Error; err != nil {
		t.Fatal(err)
	}
	s := &Service{db: db}
	msg := &IncomingMessage{Platform: PlatformWeCom, UserID: "asker", ChatID: "group"}
	req := &qaRequest{msg: msg, channel: channel, channelID: channel.ID, session: &types.Session{ID: "session"}}
	ctx := s.prepareAnswerFeedback(context.Background(), req)
	requestID := imRequestID(ctx)
	if requestID != msg.Extra["feedback_id"] {
		t.Fatal("first reply is not paired with QA")
	}
	for _, message := range []*types.Message{
		{SessionID: "session", RequestID: requestID, Role: "user", Content: "question", IsCompleted: true},
		{SessionID: "session", RequestID: requestID, Role: "assistant", Content: "answer", IsCompleted: true},
	} {
		if err := db.Create(message).Error; err != nil {
			t.Fatal(err)
		}
	}
	// No in-memory mapping is reused; any member of the original group can vote.
	s = &Service{db: db}
	event := &IncomingMessage{Platform: PlatformWeCom, MessageID: "event", UserID: "reviewer", ChatID: "group", Feedback: &AnswerFeedbackEvent{ID: requestID, Type: 2, CreatedAt: 100, Content: "fix this", InaccurateReasonList: []int{3, 3, 2}}}
	if err := s.HandleMessage(context.Background(), event, channel.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.HandleMessage(context.Background(), event, channel.ID); err != nil {
		t.Fatal(err)
	}
	var rows []types.AnswerFeedback
	db.Find(&rows)
	if len(rows) != 1 || rows[0].Question != "question" || rows[0].Answer != "answer" || rows[0].Source != "wecom" || rows[0].TenantID != 42 || !strings.Contains(rows[0].Reason, "内容有错误；内容不完整；fix this") {
		t.Fatalf("bad feedback: %+v", rows)
	}
	for _, wrong := range []struct{ channel, chat string }{{"other", "group"}, {"channel", "other-group"}} {
		event.ChatID = wrong.chat
		if err := s.HandleMessage(context.Background(), event, wrong.channel); err == nil {
			t.Fatal("accepted wrong channel/chat")
		}
	}
	event.ChatID = "group"
	event.MessageID, event.Feedback.Type, event.Feedback.CreatedAt = "cancel", 3, 200
	if err := s.HandleMessage(context.Background(), event, channel.ID); err != nil {
		t.Fatal(err)
	}
	db.Find(&rows)
	if rows[0].Status != "withdrawn" {
		t.Fatal("cancel not persisted")
	}
	// A direct reply cannot be rated by another user.
	db.Model(&types.IMFeedbackTarget{}).Where("id = ?", requestID).Update("chat_id", "")
	event.ChatID = ""
	if err := s.HandleMessage(context.Background(), event, channel.ID); err == nil {
		t.Fatal("accepted another user's private reply")
	}
}
