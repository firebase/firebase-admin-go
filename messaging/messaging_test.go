package messaging

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/api/option"

	"firebase.google.com/go/internal"
)

var testMessagingConfig = &internal.MessagingConfig{
	ProjectID: "test-project",
	Opts: []option.ClientOption{
		option.WithTokenSource(&internal.MockTokenSource{AccessToken: "test-token"}),
	},
}

func TestNoProjectID(t *testing.T) {
	client, err := NewClient(context.Background(), &internal.MessagingConfig{})
	if client != nil || err == nil {
		t.Errorf("NewClient() = (%v, %v); want = (nil, error)", client, err)
	}
}

func TestEmptyTarget(t *testing.T) {
	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.SendMessage(ctx, RequestMessage{})
	if err == nil {
		t.Errorf("SendMessage(Message{empty}) = nil; want error")
	}
}

func TestSendMessage(t *testing.T) {
	var tr *http.Request
	msgName := "projects/test-project/messages/0:1500415314455276%31bd1c9631bd1c96"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr = r
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{ \"Name\":\"" + msgName + "\" }"))
	}))
	defer ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.endpoint = ts.URL
	msg, err := client.SendMessage(ctx, RequestMessage{Message: Message{Topic: "my-topic"}})
	if err != nil {
		t.Errorf("SendMessage() = %v; want nil", err)
	}

	if msg.Name != msgName {
		t.Errorf("response Name = %q; want = %q", msg.Name, msgName)
	}

	if tr.Body == nil {
		t.Fatalf("Request = nil; want non-nil")
	}
	if tr.Method != "POST" {
		t.Errorf("Method = %q; want = %q", tr.Method, "POST")
	}
	if tr.URL.Path != "/project/test-project/messages:send" {
		t.Errorf("Path = %q; want = %q", tr.URL.Path, "/project/test-project/messages:send")
	}
	if h := tr.Header.Get("Authorization"); h != "Bearer test-token" {
		t.Errorf("Authorization = %q; want = %q", h, "Bearer test-token")
	}
}
