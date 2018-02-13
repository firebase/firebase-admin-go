// Copyright 2018 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package messaging

import (
	"flag"
	"log"
	"os"
	"regexp"
	"testing"

	"golang.org/x/net/context"

	"firebase.google.com/go/integration/internal"
	"firebase.google.com/go/messaging"
)

// The registration token has the proper format, but is not valid (i.e. expired). The intention of
// these integration tests is to verify that the endpoints return the proper payload, but it is
// hard to ensure this token remains valid. The tests below should still pass regardless.
const testRegistrationToken = "fGw0qy4TGgk:APA91bGtWGjuhp4WRhHXgbabIYp1jxEKI08ofj_v1bKhWAGJQ4e3a" +
	"rRCWzeTfHaLz83mBnDh0aPWB1AykXAVUUGl2h1wT4XI6XazWpvY7RBUSYfoxtqSWGIm2nvWh2BOP1YG501SsRoE"

var client *messaging.Client

// Enable API before testing
// https://console.developers.google.com/apis/library/fcm.googleapis.com
func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		log.Println("Skipping messaging integration tests in short mode.")
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

func TestSend(t *testing.T) {
	msg := &messaging.Message{
		Topic: "foo-bar",
		Notification: &messaging.Notification{
			Title: "Title",
			Body:  "Body",
		},
		Android: &messaging.AndroidConfig{
			Notification: &messaging.AndroidNotification{
				Title: "Android Title",
				Body:  "Android Body",
			},
		},
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Alert: &messaging.ApsAlert{
						Title: "APNS Title",
						Body:  "APNS Body",
					},
				},
			},
		},
		Webpush: &messaging.WebpushConfig{
			Notification: &messaging.WebpushNotification{
				Title: "Webpush Title",
				Body:  "Webpush Body",
			},
		},
	}
	name, err := client.SendDryRun(context.Background(), msg)
	if err != nil {
		log.Fatalln(err)
	}
	const pattern = "^projects/.*/messages/.*$"
	if !regexp.MustCompile(pattern).MatchString(name) {
		t.Errorf("Send() = %q; want = %q", name, pattern)
	}
}

func TestSendInvalidToken(t *testing.T) {
	msg := &messaging.Message{Token: "INVALID_TOKEN"}
	if _, err := client.Send(context.Background(), msg); err == nil {
		t.Errorf("Send() = nil; want error")
	}
}

func TestSubscribe(t *testing.T) {
	tmr, err := client.SubscribeToTopic(context.Background(), []string{testRegistrationToken}, "mock-topic")
	if err != nil {
		t.Fatal(err)
	}
	if tmr.SuccessCount+tmr.FailureCount != 1 {
		t.Errorf("SubscribeToTopic() = %v; want total 1", tmr)
	}
}

func TestUnsubscribe(t *testing.T) {
	tmr, err := client.UnsubscribeFromTopic(context.Background(), []string{testRegistrationToken}, "mock-topic")
	if err != nil {
		t.Fatal(err)
	}
	if tmr.SuccessCount+tmr.FailureCount != 1 {
		t.Errorf("UnsubscribeFromTopic() = %v; want total 1", tmr)
	}
}
