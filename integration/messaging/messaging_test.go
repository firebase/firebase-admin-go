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

func TestSendInvalidToken(t *testing.T) {
	ctx := context.Background()
	msg := &messaging.Message{
		Token: "INVALID_TOKEN",
		Notification: messaging.Notification{
			Title: "My Title",
			Body:  "This is a Notification",
		},
	}
	_, err := client.Send(ctx, msg)

	if err == nil {
		log.Fatal(err)
	}
}

func TestSendDryRun(t *testing.T) {
	ctx := context.Background()
	msg := &messaging.Message{
		Token: testFixtures.token,
		Notification: messaging.Notification{
			Title: "My Title",
			Body:  "This is a Notification",
		},
	}
	name, err := client.SendDryRun(ctx, msg)

	if err != nil {
		log.Fatal(err)
	}

	if name != fmt.Sprintf("projects/%s/messages/fake_message_id", projectID) {
		t.Errorf("Name : %s; want : projects/%s/messages/fake_message_id", name, projectID)
	}
}

func TestSendToToken(t *testing.T) {
	ctx := context.Background()
	msg := &messaging.Message{
		Token: testFixtures.token,
		Notification: messaging.Notification{
			Title: "My Title",
			Body:  "This is a Notification",
		},
	}
	name, err := client.Send(ctx, msg)

	if err != nil {
		log.Fatal(err)
	}

	if name == "" {
		t.Errorf("Name : %s; want : projects/%s/messages/#id#", name, projectID)
	}
}

func TestSendToTopic(t *testing.T) {
	ctx := context.Background()
	msg := &messaging.Message{
		Topic: testFixtures.topic,
		Notification: messaging.Notification{
			Title: "My Title",
			Body:  "This is a Notification",
		},
	}
	name, err := client.Send(ctx, msg)

	if err != nil {
		log.Fatal(err)
	}

	if name == "" {
		t.Errorf("Name : %s; want : projects/%s/messages/#id#", name, projectID)
	}
}

func TestSendToCondition(t *testing.T) {
	ctx := context.Background()
	msg := &messaging.Message{
		Condition: testFixtures.condition,
		Notification: messaging.Notification{
			Title: "My Title",
			Body:  "This is a Notification",
		},
	}
	name, err := client.Send(ctx, msg)

	if err != nil {
		log.Fatal(err)
	}

	if name == "" {
		t.Errorf("Name : %s; want : projects/%s/messages/#id#", name, projectID)
	}
}

func TestSendNotification(t *testing.T) {
	ctx := context.Background()
	msg := &messaging.Message{
		Token: testFixtures.token,
		Notification: messaging.Notification{
			Title: "My Title",
			Body:  "This is a Notification",
		},
	}
	name, err := client.Send(ctx, msg)

	if err != nil {
		log.Fatal(err)
	}

	if name == "" {
		t.Errorf("Name : %s; want : projects/%s/messages/#id#", name, projectID)
	}
}

func TestSendData(t *testing.T) {
	ctx := context.Background()
	msg := &messaging.Message{
		Token: testFixtures.token,
		Data: map[string]interface{}{
			"private_key":  "foo",
			"client_email": "bar@test.com",
		},
	}
	name, err := client.Send(ctx, msg)

	if err != nil {
		log.Fatal(err)
	}

	if name == "" {
		t.Errorf("Name : %s; want : projects/%s/messages/#id#", name, projectID)
	}
}

func TestSendAndroidNotification(t *testing.T) {
	ctx := context.Background()
	msg := &messaging.Message{
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
	}
	name, err := client.Send(ctx, msg)

	if err != nil {
		log.Fatal(err)
	}

	if name == "" {
		t.Errorf("Name : %s; want : projects/%s/messages/#id#", name, projectID)
	}
}

func TestSendAndroidData(t *testing.T) {
	ctx := context.Background()
	msg := &messaging.Message{
		Token: testFixtures.token,
		Notification: messaging.Notification{
			Title: "My Title",
			Body:  "This is a Notification",
		},
		Android: messaging.AndroidConfig{
			CollapseKey: "Collapse",
			Priority:    "HIGH",
			TTL:         "3.5s",
			Data: map[string]string{
				"private_key":  "foo",
				"client_email": "bar@test.com",
			},
		},
	}
	name, err := client.Send(ctx, msg)

	if err != nil {
		log.Fatal(err)
	}

	if name == "" {
		t.Errorf("Name : %s; want : projects/%s/messages/#id#", name, projectID)
	}
}

func TestSendApnsNotification(t *testing.T) {
	ctx := context.Background()
	msg := &messaging.Message{
		Token: testFixtures.token,
		Notification: messaging.Notification{
			Title: "My Title",
			Body:  "This is a Notification",
		},
		Apns: messaging.ApnsConfig{
			Payload: map[string]string{
				"title": "APNS Title ",
				"body":  "APNS bodym",
			},
		},
	}
	name, err := client.Send(ctx, msg)

	if err != nil {
		log.Fatal(err)
	}

	if name == "" {
		t.Errorf("Name : %s; want : projects/%s/messages/#id#", name, projectID)
	}
}

func TestSendApnsData(t *testing.T) {
	ctx := context.Background()
	msg := &messaging.Message{
		Token: testFixtures.token,
		Notification: messaging.Notification{
			Title: "My Title",
			Body:  "This is a Notification",
		},
		Apns: messaging.ApnsConfig{
			Headers: map[string]string{
				"private_key":  "foo",
				"client_email": "bar@test.com",
			},
		},
	}
	name, err := client.Send(ctx, msg)

	if err != nil {
		log.Fatal(err)
	}

	if name == "" {
		t.Errorf("Name : %s; want : projects/%s/messages/#id#", name, projectID)
	}
}
