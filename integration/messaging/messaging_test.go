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
	"context"
	"flag"
	"log"
	"os"
	"regexp"
	"testing"
	"time"

	"firebase.google.com/go/integration/internal"
	"firebase.google.com/go/messaging"
)

var client *messaging.Client

var testFixtures = struct {
	token     string
	topic     string
	condition string
}{}

var ttl = time.Duration(3) * time.Second

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

	client, err = app.Messaging(ctx)
	if err != nil {
		log.Fatalln(err)
	}
	os.Exit(m.Run())
}

func TestSend(t *testing.T) {
	ctx := context.Background()
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
	name, err := client.SendDryRun(ctx, msg)
	if err != nil {
		log.Fatal(err)
	}
	const pattern = "^projects/.*/messages/.*$"
	if !regexp.MustCompile(pattern).MatchString(name) {
		t.Errorf("Send() = %q; want = %q", name, pattern)
	}
}

func TestSendInvalidToken(t *testing.T) {
	ctx := context.Background()
	msg := &messaging.Message{Token: "INVALID_TOKEN"}
	_, err := client.Send(ctx, msg)
	if err == nil {
		t.Errorf("Send() = nil; want error")
	}
}
