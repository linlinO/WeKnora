package wecom

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/im"
	ws "github.com/gorilla/websocket"
)

// Diagnostic reproduction of the deployed sender, not a regression assertion
// for desired behavior. Run via the adjacent overlay, never against real WeCom.
// A future ACK-aware sender should make this reproduction fail.
func TestDiagnosticRejectedFinishIsReportedSuccessful(t *testing.T) {
	for _, chatType := range []im.ChatType{im.ChatTypeDirect, im.ChatTypeGroup} {
		t.Run(string(chatType), func(t *testing.T) {
			release := make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			finalFrame := make(chan wsFrame, 1)
			upgrader := ws.Upgrader{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer conn.Close()
				for {
					var frame wsFrame
					if conn.ReadJSON(&frame) != nil {
						return
					}
					var body streamReplyBody
					if json.Unmarshal(frame.Body, &body) != nil {
						return
					}
					if body.Stream.Finish {
						finalFrame <- frame
						<-release
						_ = conn.WriteJSON(wsFrame{Headers: frame.Headers, ErrCode: 6000, ErrMsg: "data version conflict"})
						return
					}
				}
			}))
			defer server.Close()
			conn, _, err := ws.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			client := &LongConnClient{conn: conn}
			incoming := &im.IncomingMessage{ChatType: chatType, Extra: map[string]string{"req_id": "diagnostic-only", "feedback_id": "diagnostic-feedback"}}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			streamID, err := client.StartStream(ctx, incoming)
			if err != nil {
				t.Fatal(err)
			}
			if err := client.FinalizeStream(ctx, incoming, streamID, "diagnostic answer"); err != nil {
				t.Fatal(err)
			}
			endErr := client.EndStream(ctx, incoming, streamID)
			if endErr != nil {
				t.Fatalf("deployed defect no longer reproduced: %v", endErr)
			}
			select {
			case frame := <-finalFrame:
				var body streamReplyBody
				if err := json.Unmarshal(frame.Body, &body); err != nil {
					t.Fatal(err)
				}
				if body.Stream.Feedback == nil || body.Stream.Feedback.ID != "diagnostic-feedback" || body.Stream.ID != streamID {
					t.Fatal("feedback identity was not preserved")
				}
			case <-ctx.Done():
				t.Fatal("final frame was not received")
			}
			client.streamBufsMu.Lock()
			_, bufferRetained := client.streamBufs[streamID]
			client.streamBufsMu.Unlock()
			if bufferRetained {
				t.Fatal("deployed behavior changed: buffer retained")
			}
			unblock()
			_ = conn.SetReadDeadline(time.Now().Add(time.Second))
			var ack wsFrame
			if err := conn.ReadJSON(&ack); err != nil {
				t.Fatal(err)
			}
			if ack.ErrCode != 6000 {
				t.Fatalf("unexpected mock response: %+v", ack)
			}
			t.Logf("REPRODUCED: EndStream returned nil before ACK, buffer already removed; platform then rejected finish with code=%d; feedback ID was present", ack.ErrCode)
		})
	}
}
