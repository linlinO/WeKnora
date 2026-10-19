package chatpipeline

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type visionProbeChat struct {
	response *types.ChatResponse
	err      error
	messages []chat.Message
	options  *chat.ChatOptions
}

func (m *visionProbeChat) GetModelName() string { return "vision-test" }
func (m *visionProbeChat) GetModelID() string   { return "vision-test" }
func (m *visionProbeChat) ChatStream(context.Context, []chat.Message, *chat.ChatOptions) (<-chan types.StreamResponse, error) {
	return nil, errors.New("unexpected stream call")
}

func (m *visionProbeChat) Chat(_ context.Context, messages []chat.Message, options *chat.ChatOptions) (*types.ChatResponse, error) {
	m.messages = messages
	m.options = options
	return m.response, m.err
}

func TestImageUnderstandingFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name            string
		vision          bool
		content, finish string
		err             error
	}{
		{"no vision", false, `{"image_description":"invented"}`, "stop", nil},
		{"provider failure", true, "", "", errors.New("provider unavailable")},
		{"unreadable", true, `{"image_description":""}`, "stop", nil},
		{"invalid JSON", true, "unstructured reply", "stop", nil},
		{"truncated", true, `{"image_description":"partial"}`, "length", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := &visionProbeChat{response: &types.ChatResponse{Content: tc.content, FinishReason: tc.finish}, err: tc.err}
			p := &PluginQueryUnderstand{modelService: &stubModelService{model: model}, config: &config.Config{Conversation: &config.ConversationConfig{}}}
			cm := &types.ChatManage{PipelineRequest: types.PipelineRequest{Images: []string{"data:image/png;base64,test"}, ChatModelSupportsVision: tc.vision}, PipelineState: types.PipelineState{ImageDescription: "old description"}}
			nextCalled := false
			err := p.OnEvent(context.Background(), types.QUERY_UNDERSTAND, cm, func() *PluginError { nextCalled = true; return nil })
			if err == nil || err.ErrorType != "image_analysis_unavailable" || nextCalled {
				t.Fatalf("continued without image evidence: err=%+v next=%v", err, nextCalled)
			}
			if !tc.vision && model.messages != nil {
				t.Fatal("sent image task to text-only model")
			}
		})
	}
}

func TestKnowledgeOnlyImageAndFollowUpMustRetrieve(t *testing.T) {
	for _, withImage := range []bool{false, true} {
		model := &visionProbeChat{response: &types.ChatResponse{Content: `{"rewrite_query":"ulimit-check max process 61812","intent":"follow_up","image_description":"ulimit-check max process 61812"}`, FinishReason: "stop"}}
		p := &PluginQueryUnderstand{modelService: &stubModelService{model: model}, config: &config.Config{Conversation: &config.ConversationConfig{}}}
		cm := &types.ChatManage{PipelineRequest: types.PipelineRequest{Query: "How to fix it?", KnowledgeOnly: true, EnableRewrite: true, ChatModelSupportsVision: true}}
		if withImage {
			cm.Images = []string{"data:image/png;base64,test"}
		}
		err := p.OnEvent(context.Background(), types.QUERY_UNDERSTAND, cm, func() *PluginError { return nil })
		if err != nil || !cm.NeedsRetrieval() || cm.Intent != types.IntentKBSearch || cm.SystemPromptOverride != "" {
			t.Fatalf("retrieval guard failed: %+v, %v", cm, err)
		}
		if withImage && len(model.messages[1].Images) != 1 {
			t.Fatal("image not sent to model")
		}
		if !cm.Clone().KnowledgeOnly {
			t.Fatal("clone lost retrieval guard")
		}
		if model.options.MaxCompletionTokens < 1024 {
			t.Fatal("query understanding budget too small for structured follow-up queries")
		}
	}
}

func TestKnowledgeOnlyTruncatedFollowUpDoesNotSearchVagueQuery(t *testing.T) {
	model := &visionProbeChat{response: &types.ChatResponse{Content: `{"rewrite_query":"the previous`, FinishReason: "length"}}
	p := &PluginQueryUnderstand{modelService: &stubModelService{model: model}, config: &config.Config{Conversation: &config.ConversationConfig{}}}
	cm := &types.ChatManage{PipelineRequest: types.PipelineRequest{KnowledgeOnly: true, EnableRewrite: true, Query: "What about that parameter?"}}
	err := p.OnEvent(context.Background(), types.QUERY_UNDERSTAND, cm, func() *PluginError {
		t.Fatal("searched without resolving the follow-up")
		return nil
	})
	if err == nil || err.ErrorType != "query_analysis_incomplete" {
		t.Fatalf("expected clarification fallback, got %+v", err)
	}
}

type attachmentCaptionService struct {
	interfaces.MessageService
	message *types.Message
	updated *types.Message
}

func (s *attachmentCaptionService) GetMessage(context.Context, string, string) (*types.Message, error) {
	return s.message, nil
}
func (s *attachmentCaptionService) UpdateMessage(_ context.Context, message *types.Message) error {
	s.updated = message
	return nil
}

func TestIMImageCaptionPersistsWithoutImageBytes(t *testing.T) {
	s := &attachmentCaptionService{message: &types.Message{ID: "m1", SessionID: "s1", Content: "original question", Attachments: types.MessageAttachments{{FileName: "image.png", FileType: ".png", Content: "![image](images/a.png)"}}}}
	p := &PluginQueryUnderstand{messageService: s}
	cm := &types.ChatManage{PipelineRequest: types.PipelineRequest{SessionID: "s1"}, PipelineContext: types.PipelineContext{UserMessageID: "m1"}, PipelineState: types.PipelineState{ImageDescription: "ulimit-check max process 61812"}}
	p.updateUserMessageImageCaption(context.Background(), cm)
	if s.updated == nil || s.updated.Content != "" || s.updated.Attachments[0].Content != cm.ImageDescription || strings.Contains(s.updated.Attachments[0].Content, "base64") {
		t.Fatalf("caption persistence failed: %+v", s.updated)
	}
}

func TestKnowledgeOnlyCannotAnswerWithoutEvidence(t *testing.T) {
	cm := &types.ChatManage{PipelineRequest: types.PipelineRequest{KnowledgeOnly: true}, PipelineState: types.PipelineState{Intent: types.IntentFollowUp}}
	err := (&PluginIntoChatMessage{}).OnEvent(context.Background(), types.INTO_CHAT_MESSAGE, cm, func() *PluginError { t.Fatal("generated answer with no evidence"); return nil })
	if err != ErrSearchNothing {
		t.Fatalf("want fixed no-evidence response, got %+v", err)
	}
}
