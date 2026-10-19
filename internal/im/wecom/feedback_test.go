package wecom

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/im"
	ws "github.com/gorilla/websocket"
)

func TestWeComFeedbackCallbackParsedWithoutQAReply(t *testing.T) {
	var got *im.IncomingMessage
	c := &LongConnClient{botID: "bot", handler: func(_ context.Context, msg *im.IncomingMessage) error { got = msg; return nil }}
	frame := wsFrame{Cmd: cmdEventCallback, Body: json.RawMessage(`{"msgid":"event","aibotid":"bot","chatid":"group","from":{"userid":"member"},"create_time":123,"msgtype":"event","event":{"eventtype":"feedback_event","feedback_event":{"id":"target","type":2,"content":"wrong","inaccurate_reason_list":[3]}}}`)}
	c.handleCallback(context.Background(), nil, frame)
	if got == nil || got.Feedback == nil || got.Feedback.ID != "target" || got.Feedback.CreatedAt != 123 || got.Feedback.Type != 2 || got.UserID != "member" || got.Content != "" {
		t.Fatalf("bad callback: %+v", got)
	}
}

func TestWeComFeedbackAttachedToFirstAndFinalFrames(t *testing.T) {
	frames := make(chan wsFrame, 8)
	u := ws.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := u.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			var f wsFrame
			if conn.ReadJSON(&f) != nil {
				return
			}
			frames <- f
		}
	}))
	defer server.Close()
	conn, _, err := ws.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	c := &LongConnClient{conn: conn}
	incoming := &im.IncomingMessage{ChatType: im.ChatTypeGroup, ChatID: "group", Extra: map[string]string{"req_id": "req", "feedback_id": "durable-id"}}
	ctx := context.Background()
	id, err := c.StartStream(ctx, incoming)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.UpdateStreamContent(ctx, incoming, id, "partial"); err != nil {
		t.Fatal(err)
	}
	if err := c.FinalizeStream(ctx, incoming, id, "answer"); err != nil {
		t.Fatal(err)
	}
	if err := c.EndStream(ctx, incoming, id); err != nil {
		t.Fatal(err)
	}
	if err := c.SendReply(ctx, incoming, &im.ReplyMessage{Content: "fallback"}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		select {
		case f := <-frames:
			var body streamReplyBody
			if err := json.Unmarshal(f.Body, &body); err != nil {
				t.Fatal(err)
			}
			if body.Stream.Feedback == nil || body.Stream.Feedback.ID != "durable-id" {
				t.Fatalf("frame %d missing feedback: %s", i, f.Body)
			}
			if i == 0 && (body.Stream.Finish || body.Stream.Content != "正在处理，请稍候…") {
				t.Fatalf("first reply must establish feedback before QA: %s", f.Body)
			}
			if i < 4 && body.Stream.ID != id {
				t.Fatal("stream identity changed before completion")
			}
			if i == 4 && body.Stream.ID == id {
				t.Fatal("fallback reused a completed stream")
			}
		case <-time.After(time.Second):
			t.Fatal("missing frame")
		}
	}
	// A replaced client must not collide with IDs retained in WeCom history.
	reconnected := &LongConnClient{conn: conn}
	nextID, err := reconnected.StartStream(ctx, incoming)
	if err != nil {
		t.Fatal(err)
	}
	if nextID == id {
		t.Fatal("new connection instance reused a historical stream ID")
	}
}
