package messaging

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"google.golang.org/api/option"

	"firebase.google.com/go/internal"
)

var testMessagingConfig = &internal.MessagingConfig{
	ProjectID: "test-project",
	Opts: []option.ClientOption{
		option.WithTokenSource(&internal.MockTokenSource{AccessToken: "test-token"}),
	},
}

var ttlWithNanos = time.Duration(1500) * time.Millisecond
var ttl = time.Duration(10) * time.Second

var validMessages = []struct {
	name string
	req  *Message
	want map[string]interface{}
}{
	{
		name: "token only",
		req:  &Message{Token: "test-token"},
		want: map[string]interface{}{"token": "test-token"},
	},
	{
		name: "topic only",
		req:  &Message{Topic: "test-topic"},
		want: map[string]interface{}{"topic": "test-topic"},
	},
	{
		name: "condition only",
		req:  &Message{Condition: "test-condition"},
		want: map[string]interface{}{"condition": "test-condition"},
	},
	{
		name: "data message",
		req: &Message{
			Data: map[string]string{
				"k1": "v1",
				"k2": "v2",
			},
			Topic: "test-topic",
		},
		want: map[string]interface{}{
			"data": map[string]interface{}{
				"k1": "v1",
				"k2": "v2",
			},
			"topic": "test-topic",
		},
	},
	{
		name: "notification message",
		req: &Message{
			Notification: &Notification{
				Title: "t",
				Body:  "b",
			},
			Topic: "test-topic",
		},
		want: map[string]interface{}{
			"notification": map[string]interface{}{
				"title": "t",
				"body":  "b",
			},
			"topic": "test-topic",
		},
	},
	{
		name: "android 1",
		req: &Message{
			Android: &AndroidConfig{
				CollapseKey: "ck",
				Data: map[string]string{
					"k1": "v1",
					"k2": "v2",
				},
				Priority: "normal",
				TTL:      &ttl,
			},
			Topic: "test-topic",
		},
		want: map[string]interface{}{
			"android": map[string]interface{}{
				"collapse_key": "ck",
				"data": map[string]interface{}{
					"k1": "v1",
					"k2": "v2",
				},
				"priority": "normal",
				"ttl":      "10s",
			},
			"topic": "test-topic",
		},
	},
	{
		name: "android 2",
		req: &Message{
			Android: &AndroidConfig{
				RestrictedPackageName: "rpn",
				Notification: &AndroidNotification{
					Title:        "t",
					Body:         "b",
					Color:        "#112233",
					Sound:        "s",
					TitleLocKey:  "tlk",
					TitleLocArgs: []string{"t1", "t2"},
					BodyLocKey:   "blk",
					BodyLocArgs:  []string{"b1", "b2"},
				},
				TTL: &ttlWithNanos,
			},
			Topic: "test-topic",
		},
		want: map[string]interface{}{
			"android": map[string]interface{}{
				"restricted_package_name": "rpn",
				"notification": map[string]interface{}{
					"title":          "t",
					"body":           "b",
					"color":          "#112233",
					"sound":          "s",
					"title_loc_key":  "tlk",
					"title_loc_args": []interface{}{"t1", "t2"},
					"body_loc_key":   "blk",
					"body_loc_args":  []interface{}{"b1", "b2"},
				},
				"ttl": "1.500000000s",
			},
			"topic": "test-topic",
		},
	},
	{
		name: "android 3",
		req: &Message{
			Android: &AndroidConfig{
				Priority: "high",
			},
			Topic: "test-topic",
		},
		want: map[string]interface{}{
			"android": map[string]interface{}{
				"priority": "high",
			},
			"topic": "test-topic",
		},
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

	_, err = client.Send(ctx, &Message{})
	if err == nil {
		t.Errorf("SendMessage(Message{empty}) = nil; want error")
	}
}

func TestSend(t *testing.T) {
	var tr *http.Request
	var b []byte
	msgName := "projects/test-project/messages/msg_id"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr = r
		b, _ = ioutil.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{ \"name\":\"" + msgName + "\" }"))
	}))
	defer ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.endpoint = ts.URL

	for _, tc := range validMessages {
		name, err := client.Send(ctx, tc.req)
		if err != nil {
			t.Errorf("[%s] Send() = %v; want nil", tc.name, err)
		}
		if name != msgName {
			t.Errorf("[%s] Response = %q; want = %q", tc.name, name, msgName)
		}

		var parsed map[string]interface{}
		if err := json.Unmarshal(b, &parsed); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(parsed["message"], tc.want) {
			t.Errorf("[%s] Body = %v; want = %v", tc.name, parsed["message"], tc.want)
		}

		if tr.Method != http.MethodPost {
			t.Errorf("[%s] Method = %q; want = %q", tc.name, tr.Method, http.MethodPost)
		}
		if tr.URL.Path != "/projects/test-project/messages:send" {
			t.Errorf("[%s] Path = %q; want = %q", tc.name, tr.URL.Path, "/projects/test-project/messages:send")
		}
		if h := tr.Header.Get("Authorization"); h != "Bearer test-token" {
			t.Errorf("[%s] Authorization = %q; want = %q", tc.name, h, "Bearer test-token")
		}
	}
}

func TestSendDryRun(t *testing.T) {
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
	name, err := client.SendDryRun(ctx, &Message{Topic: "my-topic"})
	if err != nil {
		t.Errorf("SendMessage() = %v; want nil", err)
	}

	if name != msgName {
		t.Errorf("response Name = %q; want = %q", name, msgName)
	}

	if tr.Body == nil {
		t.Fatalf("Request = nil; want non-nil")
	}
	if tr.Method != http.MethodPost {
		t.Errorf("Method = %q; want = %q", tr.Method, http.MethodPost)
	}
	if tr.URL.Path != "/projects/test-project/messages:send" {
		t.Errorf("Path = %q; want = %q", tr.URL.Path, "/projects/test-project/messages:send")
	}
	if h := tr.Header.Get("Authorization"); h != "Bearer test-token" {
		t.Errorf("Authorization = %q; want = %q", h, "Bearer test-token")
	}
}
