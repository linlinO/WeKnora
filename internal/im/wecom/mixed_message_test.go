package wecom

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Tencent/WeKnora/internal/im"
)

func TestMixedImageAndTextPreserved(t *testing.T) {
	for _, chatType := range []string{"single", "group"} {
		t.Run(chatType, func(t *testing.T) {
			var got *im.IncomingMessage
			client := &LongConnClient{handler: func(_ context.Context, msg *im.IncomingMessage) error { got = msg; return nil }}
			body := `{"msgid":"m1","chatid":"room","chattype":"` + chatType + `","from":{"userid":"u1"},"msgtype":"mixed","mixed":{"msg_item":[{"msgtype":"image","image":{"url":"https://example.com/encrypted.png","aeskey":"secret"}},{"msgtype":"text","text":{"content":"ulimit-check max process 61812"}}]}}`
			client.handleCallback(context.Background(), nil, wsFrame{Headers: map[string]string{"req_id": "r1"}, Body: json.RawMessage(body)})
			if got == nil || got.MessageType != im.MessageTypeImage || got.Content != "ulimit-check max process 61812" || got.FileKey == "" || got.Extra["aes_key"] != "secret" || got.Extra["req_id"] != "r1" {
				t.Fatalf("image or question lost: %+v", got)
			}
			if chatType == "group" && (got.ChatID != "room" || got.ChatType != im.ChatTypeGroup) {
				t.Fatalf("group routing changed: %+v", got)
			}
		})
	}
}

func TestMixedTextAndImageOnly(t *testing.T) {
	for _, test := range []struct {
		body  string
		want  im.MessageType
		count string
	}{
		{`{"msg_item":[{"msgtype":"text","text":{"content":"question"}}]}`, im.MessageTypeText, ""},
		{`{"msg_item":[{"msgtype":"image","image":{"url":"https://example.com/a"}}]}`, im.MessageTypeImage, "1"},
		{`{"msg_item":[{"msgtype":"image","image":{}},{"msgtype":"text","text":{"content":"question"}}]}`, im.MessageTypeImage, "1"},
		{`{"msg_item":[{"msgtype":"image","image":{"url":"https://example.com/a"}},{"msgtype":"image","image":{"url":"https://example.com/b"}}]}`, im.MessageTypeImage, "2"},
	} {
		var msg botMessage
		if err := json.Unmarshal([]byte(`{"mixed":`+test.body+`}`), &msg); err != nil {
			t.Fatal(err)
		}
		got := (&LongConnClient{}).convertMixedMessage(&msg, "", im.ChatTypeDirect, "r1")
		if got == nil || got.MessageType != test.want || got.Extra["image_count"] != test.count {
			t.Fatalf("unexpected conversion: %+v", got)
		}
	}
}
