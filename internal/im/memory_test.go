package im

import (
	"context"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"strings"
	"sync"
	"testing"
)

type imMemoryStub struct {
	interfaces.MemoryService
	saved   map[string][]*types.MemoryItem
	cleared []string
}

func (m *imMemoryStub) Remember(ctx context.Context, item types.MemoryItem) (*types.MemoryItem, error) {
	p, _ := types.PrincipalFromContext(ctx)
	item.ID = "abcdefgh-1234"
	m.saved[p.StorageID()] = append(m.saved[p.StorageID()], &item)
	return &item, nil
}
func (m *imMemoryStub) Clear(ctx context.Context) (int64, error) {
	p, _ := types.PrincipalFromContext(ctx)
	m.cleared = append(m.cleared, p.StorageID())
	delete(m.saved, p.StorageID())
	return 1, nil
}

type memoryReply struct {
	Adapter
	text string
}

func (a *memoryReply) SendReply(_ context.Context, _ *IncomingMessage, r *ReplyMessage) error {
	a.text = r.Content
	return nil
}
func TestIMMemoryPrivateCommandsAndClearChallengeAreScoped(t *testing.T) {
	m := &imMemoryStub{saved: map[string][]*types.MemoryItem{}}
	s := &Service{memoryService: m}
	a := &memoryReply{}
	msg := &IncomingMessage{Platform: PlatformWeCom, UserID: "alice", MessageID: "one", ChatType: ChatTypeDirect, Content: "记住：偏好：回答先给结论"}
	ctx := withIMIdentity(context.Background(), 1, "bot", msg)
	handled, err := s.handleMemoryMessage(ctx, msg, a)
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, m.saved["im_user:1:bot:wecom:alice"], 1)
	require.Equal(t, types.MemoryKindPreference, m.saved["im_user:1:bot:wecom:alice"][0].Kind)
	msg.ChatType = ChatTypeGroup
	msg.Content = "我的记忆"
	handled, err = s.handleMemoryMessage(ctx, msg, a)
	require.NoError(t, err)
	require.True(t, handled)
	require.Contains(t, a.text, "请私聊")
	msg.ChatType = ChatTypeDirect
	msg.Content = "清空我的记忆"
	_, err = s.handleMemoryMessage(ctx, msg, a)
	require.NoError(t, err)
	code := strings.Split(strings.Split(a.text, "确认清空记忆 ")[1], "\n")[0]
	bob := withIMIdentity(context.Background(), 1, "bot", &IncomingMessage{Platform: PlatformWeCom, UserID: "bob", ChatType: ChatTypeDirect})
	msg.Content = "确认清空记忆 " + code
	_, err = s.handleMemoryMessage(bob, msg, a)
	require.NoError(t, err)
	require.Empty(t, m.cleared)
	_, err = s.handleMemoryMessage(ctx, msg, a)
	require.NoError(t, err)
	require.Equal(t, []string{"im_user:1:bot:wecom:alice"}, m.cleared)
	_, err = s.handleMemoryMessage(ctx, msg, a)
	require.NoError(t, err)
	require.Len(t, m.cleared, 1)
}
func TestIMGenerationPreservesTruncationAndCancellation(t *testing.T) {
	require.Equal(t, "truncated", imGenerationStatus(nil, "truncated"))
	require.Equal(t, "interrupted", imGenerationStatus(context.Canceled, "truncated"))
	b := event.NewEventBus()
	a := &types.Message{}
	mu := &sync.Mutex{}
	subscribeIMMemory(b, a, mu)
	require.NoError(t, b.EmitAndWait(context.Background(), event.Event{Type: event.EventAgentFinalAnswer, Data: event.AgentFinalAnswerData{Truncated: true, Done: true}}))
	require.Equal(t, "truncated", a.GenerationStatus)
	// The classification helper must not turn a token-limited answer into a completed answer.
	require.Equal(t, "completed", imGenerationStatus(nil))
}
