package messaging

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"testing"

	"firebase.google.com/go/integration/internal"
	"firebase.google.com/go/messaging"
)

var projectID string
var client *messaging.Client

// Enable API before testing
// https://console.developers.google.com/apis/library/fcm.googleapis.com/?project=
func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		log.Println("skipping Messaging integration tests in short mode.")
		return
	}

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
			Token: "TODO integration_messaging.json",
			Notification: messaging.Notification{
				Title: "My Title",
				Body:  "This is a Notification",
			},
		},
	}
	resp, err := client.SendMessage(ctx, msg)

	if err != nil {
		log.Fatal(resp)
	}

	if resp.Name != fmt.Sprintf("projects/%s/messages/fake_message_id", projectID) {
		t.Errorf("Name : %s; want : projects/%s/messages/fake_message_id", resp.Name, projectID)
	}
}

func TestSendMessageToToken(t *testing.T) {
	ctx := context.Background()
	msg := &messaging.RequestMessage{
		Message: messaging.Message{
			Token: "TODO integration_messaging.json",
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
}

func TestSendMessageToCondition(t *testing.T) {
}

func TestSendNotificationMessage(t *testing.T) {
}

func TestSendDataMessage(t *testing.T) {
}

func TestSendAndroidNotificationMessage(t *testing.T) {
}

func TestSendAndroidDataMessage(t *testing.T) {
}

func TestSendApnsNotificationMessage(t *testing.T) {
}

func TestSendApnsDataMessage(t *testing.T) {
}

func TestSendWebPushNotificationMessage(t *testing.T) {
}

func TestSendWebPushDataMessage(t *testing.T) {
}

func TestSendMultiotificationMessage(t *testing.T) {
}

func TestSendMultiDataMessage(t *testing.T) {
}
