package watch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendTelegram(t *testing.T) {
	var seenPath, seenBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		b := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(b)
		seenBody = string(b)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	err := SendTelegram(context.Background(), TelegramConfig{
		BotToken: "token",
		ChatID:   "123",
		Endpoint: srv.URL,
	}, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if seenPath != "/bottoken/sendMessage" {
		t.Fatalf("path = %s", seenPath)
	}
	if !strings.Contains(seenBody, "chat_id=123") || !strings.Contains(seenBody, "text=hello") {
		t.Fatalf("body = %s", seenBody)
	}
}
