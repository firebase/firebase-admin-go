package messaging

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"

	"firebase.google.com/go/integration/internal"
	"firebase.google.com/go/messaging"
)

var projectID string
var client *messaging.Client

var testFixtures = struct {
	token     string
	topic     string
	condition string
}{}

// Enable API before testing
// https://console.developers.google.com/apis/library/fcm.googleapis.com/?project=
func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		log.Println("skipping Messaging integration tests in short mode.")
		return
	}

	token, err := ioutil.ReadFile(internal.Resource("integration_token.txt"))
	if err != nil {
		log.Fatalln(err)
	}
	testFixtures.token = string(token)

	topic, err := ioutil.ReadFile(internal.Resource("integration_topic.txt"))
	if err != nil {
		log.Fatalln(err)
	}
	testFixtures.topic = string(topic)

	condition, err := ioutil.ReadFile(internal.Resource("integration_condition.txt"))
	if err != nil {
		log.Fatalln(err)
	}
	testFixtures.condition = string(condition)

	ctx := context.Background()
	app, err := internal.NewTestApp(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	projectID, err = internal.ProjectID()
	if err != nil {
		log.Fatalln(err)
	}

	client, err = app.Messaging(ctx)

	if err != nil {
		log.Fatalln(err)
	}
	os.Exit(m.Run())
}

func TestSendMessageInvalidToken(t *testing.T) {
	ctx := context.Background()
	msg := &messaging.RequestMessage{
		Message: messaging.Message{
			Token: "INVALID_TOKEN",
			Notification: messaging.Notification{
				Title: "My Title",
				Body:  "This is a Notification",
			},
		},
	}
	_, err := client.SendMessage(ctx, msg)

	if err == nil {
		log.Fatal(err)
	}
}

func TestSendMessageValidateOnly(t *testing.T) {
	ctx := context.Background()
	msg := &messaging.RequestMessage{
		ValidateOnly: true,
		Message: messaging.Message{
			Token: testFixtures.token,
			Notification: messaging.Notification{
				Title: "My Title",
				Body:  "This is a Notification",
			},
		},
	}
	resp, err := client.SendMessage(ctx, msg)

	if err != nil {
		log.Fatal(err)
	}

	if resp.Name != fmt.Sprintf("projects/%s/messages/fake_message_id", projectID) {
		t.Errorf("Name : %s; want : projects/%s/messages/fake_message_id", resp.Name, projectID)
	}
}

func TestSendMessageToToken(t *testing.T) {
	ctx := context.Background()
	msg := &messaging.RequestMessage{
		Message: messaging.Message{
			Token: testFixtures.token,
			Notification: messaging.Notification{
				Title: "My Title",
				Body:  "This is a Notification",
			},
		},
	}
	resp, err := client.SendMessage(ctx, msg)

	if err != nil {
		log.Fatal(err)
	}

	if resp.Name == "" {
		t.Errorf("Name : %s; want : projects/%s/messages/#id#", resp.Name, projectID)
	}
}

func TestSendMessageToTopic(t *testing.T) {
	ctx := context.Background()
	msg := &messaging.RequestMessage{
		Message: messaging.Message{
			Topic: testFixtures.topic,
			Notification: messaging.Notification{
				Title: "My Title",
				Body:  "This is a Notification",
			},
		},
	}
	resp, err := client.SendMessage(ctx, msg)

	if err != nil {
		log.Fatal(err)
	}

	if resp.Name == "" {
		t.Errorf("Name : %s; want : projects/%s/messages/#id#", resp.Name, projectID)
	}
}

func TestSendMessageToCondition(t *testing.T) {
	ctx := context.Background()
	msg := &messaging.RequestMessage{
		Message: messaging.Message{
			Condition: testFixtures.condition,
			Notification: messaging.Notification{
				Title: "My Title",
				Body:  "This is a Notification",
			},
		},
	}
	resp, err := client.SendMessage(ctx, msg)

	if err != nil {
		log.Fatal(err)
	}

	if resp.Name == "" {
		t.Errorf("Name : %s; want : projects/%s/messages/#id#", resp.Name, projectID)
	}
}

func TestSendNotificationMessage(t *testing.T) {
	ctx := context.Background()
	msg := &messaging.RequestMessage{
		Message: messaging.Message{
			Token: testFixtures.token,
			Notification: messaging.Notification{
				Title: "My Title",
				Body:  "This is a Notification",
			},
		},
	}
	resp, err := client.SendMessage(ctx, msg)

	if err != nil {
		log.Fatal(err)
	}

	if resp.Name == "" {
		t.Errorf("Name : %s; want : projects/%s/messages/#id#", resp.Name, projectID)
	}
}

func TestSendDataMessage(t *testing.T) {
	ctx := context.Background()
	msg := &messaging.RequestMessage{
		Message: messaging.Message{
			Token: testFixtures.token,
			Data: map[string]interface{}{
				"private_key":  "foo",
				"client_email": "bar@test.com",
			},
		},
	}
	resp, err := client.SendMessage(ctx, msg)

	if err != nil {
		log.Fatal(err)
	}

	if resp.Name == "" {
		t.Errorf("Name : %s; want : projects/%s/messages/#id#", resp.Name, projectID)
	}
}

func TestSendAndroidNotificationMessage(t *testing.T) {
	ctx := context.Background()
	msg := &messaging.RequestMessage{
		Message: messaging.Message{
			Token: testFixtures.token,
			Notification: messaging.Notification{
				Title: "My Title",
				Body:  "This is a Notification",
			},
			Android: messaging.AndroidConfig{
				CollapseKey: "Collapse",
				Priority:    "HIGH",
				TTL:         "3.5s",
				Notification: messaging.AndroidNotification{
					Title: "Android Title",
					Body:  "Android body",
				},
			},
		},
	}
	resp, err := client.SendMessage(ctx, msg)

	if err != nil {
		log.Fatal(err)
	}

	if resp.Name == "" {
		t.Errorf("Name : %s; want : projects/%s/messages/#id#", resp.Name, projectID)
	}
}

func TestSendAndroidDataMessage(t *testing.T) {
	ctx := context.Background()
	msg := &messaging.RequestMessage{
		Message: messaging.Message{
			Token: testFixtures.token,
			Notification: messaging.Notification{
				Title: "My Title",
				Body:  "This is a Notification",
			},
			Android: messaging.AndroidConfig{
				CollapseKey: "Collapse",
				Priority:    "HIGH",
				TTL:         "3.5s",
				Data: map[string]interface{}{
					"private_key":  "foo",
					"client_email": "bar@test.com",
				},
			},
		},
	}
	resp, err := client.SendMessage(ctx, msg)

	if err != nil {
		log.Fatal(err)
	}

	if resp.Name == "" {
		t.Errorf("Name : %s; want : projects/%s/messages/#id#", resp.Name, projectID)
	}
}

func TestSendApnsNotificationMessage(t *testing.T) {
	ctx := context.Background()
	msg := &messaging.RequestMessage{
		Message: messaging.Message{
			Token: testFixtures.token,
			Notification: messaging.Notification{
				Title: "My Title",
				Body:  "This is a Notification",
			},
			Apns: messaging.ApnsConfig{
				Payload: map[string]interface{}{
					"title": "APNS Title ",
					"body":  "APNS bodym",
				},
			},
		},
	}
	resp, err := client.SendMessage(ctx, msg)

	if err != nil {
		log.Fatal(err)
	}

	if resp.Name == "" {
		t.Errorf("Name : %s; want : projects/%s/messages/#id#", resp.Name, projectID)
	}
}

func TestSendApnsDataMessage(t *testing.T) {
	ctx := context.Background()
	msg := &messaging.RequestMessage{
		Message: messaging.Message{
			Token: testFixtures.token,
			Notification: messaging.Notification{
				Title: "My Title",
				Body:  "This is a Notification",
			},
			Apns: messaging.ApnsConfig{
				Headers: map[string]interface{}{
					"private_key":  "foo",
					"client_email": "bar@test.com",
				},
			},
		},
	}
	resp, err := client.SendMessage(ctx, msg)

	if err != nil {
		log.Fatal(err)
	}

	if resp.Name == "" {
		t.Errorf("Name : %s; want : projects/%s/messages/#id#", resp.Name, projectID)
	}
}
