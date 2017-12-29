package messaging

import (
	"context"
	"flag"
	"log"
	"os"
	"testing"

	"firebase.google.com/go/integration/internal"
	"firebase.google.com/go/messaging"
)

var client *messaging.Client

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

	client, err = app.Messaging(ctx)
	if err != nil {
		log.Fatalln(err)
	}
	os.Exit(m.Run())
}

func TestSendMessageToToken(t *testing.T) {
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
