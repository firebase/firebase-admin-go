// Copyright 2018 Google LLC All Rights Reserved.
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
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"testing"
	"time"

	"firebase.google.com/go/v4/integration/internal"
	"firebase.google.com/go/v4/messaging"
)

// The registration token has the proper format, but is not valid (i.e. expired). The intention of
// these integration tests is to verify that the endpoints return the proper payload, but it is
// hard to ensure this token remains valid. The tests below should still pass regardless.
const testRegistrationToken = "fGw0qy4TGgk:APA91bGtWGjuhp4WRhHXgbabIYp1jxEKI08ofj_v1bKhWAGJQ4e3a" +
	"rRCWzeTfHaLz83mBnDh0aPWB1AykXAVUUGl2h1wT4XI6XazWpvY7RBUSYfoxtqSWGIm2nvWh2BOP1YG501SsRoE"

var messageIDPattern = regexp.MustCompile("^projects/.*/messages/.*$")
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
	app, err := internal.NewTestApp(ctx, nil)
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
		t.Fatal(err)
	}
	if !messageIDPattern.MatchString(name) {
		t.Errorf("Send() = %q; want = %q", name, messageIDPattern.String())
	}
}

func fullAndroidNotificationV2() *messaging.AndroidNotificationV2 {
	count := 1
	id := 42
	eventTimestamp := time.Now()
	return &messaging.AndroidNotificationV2{
		Title:                 "test.title",
		Body:                  "test.body",
		Icon:                  "test.icon",
		Color:                 "#AABBCC",
		Sound:                 "test.sound",
		Tag:                   "test.tag",
		ClickAction:           "test.click.action",
		BodyLocKey:            "test.body.loc.key",
		BodyLocArgs:           []string{"body.arg1", "body.arg2"},
		TitleLocKey:           "test.title.loc.key",
		TitleLocArgs:          []string{"title.arg1", "title.arg2"},
		ChannelID:             "test.channel.id",
		ImageURL:              "https://example.com/image.png",
		Ticker:                "test.ticker",
		Sticky:                true,
		EventTimestamp:        &eventTimestamp,
		LocalOnly:             true,
		Priority:              messaging.PriorityHigh,
		VibrateTimingMillis:   []int64{100, 50, 250},
		DefaultVibrateTimings: true,
		DefaultSound:          true,
		LightSettings: &messaging.LightSettings{
			Color:                  "#AABBCC",
			LightOnDurationMillis:  200,
			LightOffDurationMillis: 300,
		},
		DefaultLightSettings: true,
		Visibility:           messaging.VisibilityPrivate,
		NotificationCount:    &count,
		ID:                   &id,
	}
}

// fullAndroidConfigV2Base populates all shared AndroidConfigV2 fields.
func fullAndroidConfigV2Base() *messaging.AndroidConfigV2 {
	ttl := 5 * time.Second
	return &messaging.AndroidConfigV2{
		CollapseKey:            "test-key",
		TTL:                    &ttl,
		RestrictedPackageName:  "com.google.firebase.testing",
		Data:                   map[string]string{"androidFoo": "androidBar"},
		FCMOptions:             &messaging.AndroidFCMOptions{AnalyticsLabel: "test-analytics"},
		DirectBootOK:           true,
		BandwidthConstrainedOK: true,
		RestrictedSatelliteOK:  true,
	}
}

// androidV2RemoteNotificationMessage populates RemoteNotification and all shared AndroidConfigV2 fields.
func androidV2RemoteNotificationMessage() *messaging.Message {
	config := fullAndroidConfigV2Base()
	config.RemoteNotification = &messaging.AndroidRemoteNotification{
		MutableContent:     true,
		UseAsV1DataMessage: true,
		Notification:       fullAndroidNotificationV2(),
	}
	return &messaging.Message{
		Topic: "foo-bar",
		Notification: &messaging.Notification{
			Title: "Title",
			Body:  "Body",
		},
		AndroidV2: config,
	}
}

// androidV2BackgroundSyncMessage populates BackgroundSync and all shared AndroidConfigV2 fields.
func androidV2BackgroundSyncMessage() *messaging.Message {
	config := fullAndroidConfigV2Base()
	config.BackgroundSync = &messaging.AndroidBackgroundSyncMessage{}
	return &messaging.Message{
		Topic:     "foo-bar",
		AndroidV2: config,
	}
}

func androidV2MinimalRemoteNotificationMessage() *messaging.Message {
	return &messaging.Message{
		Topic: "foo-bar",
		AndroidV2: &messaging.AndroidConfigV2{
			RemoteNotification: &messaging.AndroidRemoteNotification{
				Notification: &messaging.AndroidNotificationV2{
					Title: "test.title",
					Body:  "test.body",
				},
			},
		},
	}
}

func androidV2MinimalBackgroundSyncMessage() *messaging.Message {
	return &messaging.Message{
		Topic: "foo-bar",
		AndroidV2: &messaging.AndroidConfigV2{
			BackgroundSync: &messaging.AndroidBackgroundSyncMessage{},
		},
	}
}

func TestSendAndroidV2(t *testing.T) {
	cases := []struct {
		name string
		msg  *messaging.Message
	}{
		{"FullRemoteNotification", androidV2RemoteNotificationMessage()},
		{"MinimalRemoteNotification", androidV2MinimalRemoteNotificationMessage()},
		{"FullBackgroundSync", androidV2BackgroundSyncMessage()},
		{"MinimalBackgroundSync", androidV2MinimalBackgroundSyncMessage()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name, err := client.SendDryRun(context.Background(), tc.msg)
			if err != nil {
				t.Fatal(err)
			}
			if !messageIDPattern.MatchString(name) {
				t.Errorf("Send() = %q; want = %q", name, messageIDPattern.String())
			}
		})
	}
}

func TestSendEachAndroidV2(t *testing.T) {
	messages := []*messaging.Message{
		androidV2RemoteNotificationMessage(),
		androidV2MinimalRemoteNotificationMessage(),
		androidV2BackgroundSyncMessage(),
		androidV2MinimalBackgroundSyncMessage(),
	}

	br, err := client.SendEachDryRun(context.Background(), messages)
	if err != nil {
		t.Fatal(err)
	}

	if len(br.Responses) != len(messages) {
		t.Errorf("len(Responses) = %d; want = %d", len(br.Responses), len(messages))
	}
	if br.SuccessCount != len(messages) {
		t.Errorf("SuccessCount = %d; want = %d", br.SuccessCount, len(messages))
	}
	if br.FailureCount != 0 {
		t.Errorf("FailureCount = %d; want = 0", br.FailureCount)
	}

	for i, sr := range br.Responses {
		if err := checkSuccessfulSendResponse(sr); err != nil {
			t.Errorf("Responses[%d]: %v", i, err)
		}
	}
}

func TestSendInvalidToken(t *testing.T) {
	msg := &messaging.Message{Token: "INVALID_TOKEN"}
	if _, err := client.Send(context.Background(), msg); err == nil || !messaging.IsInvalidArgument(err) {
		t.Errorf("Send() = %v; want InvalidArgumentError", err)
	}
}

func TestSendInvalidFid(t *testing.T) {
	msg := &messaging.Message{Fid: "INVALID_FID"}
	if _, err := client.Send(context.Background(), msg); err == nil || !messaging.IsUnregistered(err) {
		t.Errorf("Send() = %v; want UnregisteredError", err)
	}
}

func TestSendEach(t *testing.T) {
	messages := []*messaging.Message{
		{
			Notification: &messaging.Notification{
				Title: "Title 1",
				Body:  "Body 1",
			},
			Topic: "foo-bar",
		},
		{
			Notification: &messaging.Notification{
				Title: "Title 2",
				Body:  "Body 2",
			},
			Topic: "foo-bar",
		},
		{
			Notification: &messaging.Notification{
				Title: "Title 3",
				Body:  "Body 3",
			},
			Token: "INVALID_TOKEN",
		},
	}

	br, err := client.SendEachDryRun(context.Background(), messages)
	if err != nil {
		t.Fatal(err)
	}

	if len(br.Responses) != 3 {
		t.Errorf("len(Responses) = %d; want = 3", len(br.Responses))
	}
	if br.SuccessCount != 2 {
		t.Errorf("SuccessCount = %d; want = 2", br.SuccessCount)
	}
	if br.FailureCount != 1 {
		t.Errorf("FailureCount = %d; want = 1", br.FailureCount)
	}

	for i := 0; i < 2; i++ {
		sr := br.Responses[i]
		if err := checkSuccessfulSendResponse(sr); err != nil {
			t.Errorf("Responses[%d]: %v", i, err)
		}
	}

	sr := br.Responses[2]
	if sr.Success {
		t.Errorf("Responses[2]: Success = true; want = false")
	}
	if sr.MessageID != "" {
		t.Errorf("Responses[2]: MessageID = %q; want = %q", sr.MessageID, "")
	}
	if sr.Error == nil || !messaging.IsInvalidArgument(sr.Error) {
		t.Errorf("Responses[2]: Error = %v; want = InvalidArgumentError", sr.Error)
	}
}

func TestSendEachFiveHundred(t *testing.T) {
	var messages []*messaging.Message
	const limit = 500
	for i := 0; i < limit; i++ {
		m := &messaging.Message{
			Topic: fmt.Sprintf("foo-bar-%d", i%10),
		}
		messages = append(messages, m)
	}

	br, err := client.SendEachDryRun(context.Background(), messages)
	if err != nil {
		t.Fatal(err)
	}

	if len(br.Responses) != limit {
		t.Errorf("len(Responses) = %d; want = %d", len(br.Responses), limit)
	}
	if br.SuccessCount != limit {
		t.Errorf("SuccessCount = %d; want = %d", br.SuccessCount, limit)
	}
	if br.FailureCount != 0 {
		t.Errorf("FailureCount = %d; want = 0", br.FailureCount)
	}

	for i := 0; i < limit; i++ {
		sr := br.Responses[i]
		if err := checkSuccessfulSendResponse(sr); err != nil {
			t.Errorf("Responses[%d]: %v", i, err)
		}
	}
}

func TestSendEachForMulticast(t *testing.T) {
	message := &messaging.MulticastMessage{
		Notification: &messaging.Notification{
			Title: "title",
			Body:  "body",
		},
		Tokens: []string{"INVALID_TOKEN", "ANOTHER_INVALID_TOKEN"},
	}

	br, err := client.SendEachForMulticastDryRun(context.Background(), message)
	if err != nil {
		t.Fatal(err)
	}

	if len(br.Responses) != 2 {
		t.Errorf("len(Responses) = %d; want = 2", len(br.Responses))
	}
	if br.SuccessCount != 0 {
		t.Errorf("SuccessCount = %d; want = 0", br.SuccessCount)
	}
	if br.FailureCount != 2 {
		t.Errorf("FailureCount = %d; want = 2", br.FailureCount)
	}

	for i := 0; i < 2; i++ {
		sr := br.Responses[i]
		if err := checkErrorSendResponse(sr); err != nil {
			t.Errorf("Responses[%d]: %v", i, err)
		}
	}
}

func TestSendEachForMulticastFids(t *testing.T) {
	message := &messaging.MulticastMessage{
		Notification: &messaging.Notification{
			Title: "title",
			Body:  "body",
		},
		Fids: []string{"INVALID_FID", "ANOTHER_INVALID_FID"},
	}

	br, err := client.SendEachForMulticastDryRun(context.Background(), message)
	if err != nil {
		t.Fatal(err)
	}

	if len(br.Responses) != 2 {
		t.Errorf("len(Responses) = %d; want = 2", len(br.Responses))
	}
	if br.SuccessCount != 0 {
		t.Errorf("SuccessCount = %d; want = 0", br.SuccessCount)
	}
	if br.FailureCount != 2 {
		t.Errorf("FailureCount = %d; want = 2", br.FailureCount)
	}

	for i := 0; i < 2; i++ {
		sr := br.Responses[i]
		if sr.Success {
			t.Errorf("Responses[%d]: Success = true; want = false", i)
		}
		if sr.MessageID != "" {
			t.Errorf("Responses[%d]: MessageID = %q; want = %q", i, sr.MessageID, "")
		}
		if sr.Error == nil || !messaging.IsUnregistered(sr.Error) {
			t.Errorf("Responses[%d]: Error = %v; want = UnregisteredError", i, sr.Error)
		}
	}
}

func TestSendAll(t *testing.T) {
	t.Skip("Skipping integration tests for deprecated sendAll() API")
	messages := []*messaging.Message{
		{
			Notification: &messaging.Notification{
				Title: "Title 1",
				Body:  "Body 1",
			},
			Topic: "foo-bar",
		},
		{
			Notification: &messaging.Notification{
				Title: "Title 2",
				Body:  "Body 2",
			},
			Topic: "foo-bar",
		},
		{
			Notification: &messaging.Notification{
				Title: "Title 3",
				Body:  "Body 3",
			},
			Token: "INVALID_TOKEN",
		},
	}

	br, err := client.SendAllDryRun(context.Background(), messages)
	if err != nil {
		t.Fatal(err)
	}

	if len(br.Responses) != 3 {
		t.Errorf("len(Responses) = %d; want = 3", len(br.Responses))
	}
	if br.SuccessCount != 2 {
		t.Errorf("SuccessCount = %d; want = 2", br.SuccessCount)
	}
	if br.FailureCount != 1 {
		t.Errorf("FailureCount = %d; want = 1", br.FailureCount)
	}

	for i := 0; i < 2; i++ {
		sr := br.Responses[i]
		if err := checkSuccessfulSendResponse(sr); err != nil {
			t.Errorf("Responses[%d]: %v", i, err)
		}
	}

	sr := br.Responses[2]
	if sr.Success {
		t.Errorf("Responses[2]: Success = true; want = false")
	}
	if sr.MessageID != "" {
		t.Errorf("Responses[2]: MessageID = %q; want = %q", sr.MessageID, "")
	}
	if sr.Error == nil || !messaging.IsInvalidArgument(sr.Error) {
		t.Errorf("Responses[2]: Error = %v; want = InvalidArgumentError", sr.Error)
	}
}

func TestSendFiveHundred(t *testing.T) {
	t.Skip("Skipping integration tests for deprecated sendAll() API")
	var messages []*messaging.Message
	const limit = 500
	for i := 0; i < limit; i++ {
		m := &messaging.Message{
			Topic: fmt.Sprintf("foo-bar-%d", i%10),
		}
		messages = append(messages, m)
	}

	br, err := client.SendAllDryRun(context.Background(), messages)
	if err != nil {
		t.Fatal(err)
	}

	if len(br.Responses) != limit {
		t.Errorf("len(Responses) = %d; want = %d", len(br.Responses), limit)
	}
	if br.SuccessCount != limit {
		t.Errorf("SuccessCount = %d; want = %d", br.SuccessCount, limit)
	}
	if br.FailureCount != 0 {
		t.Errorf("FailureCount = %d; want = 0", br.FailureCount)
	}

	for i := 0; i < limit; i++ {
		sr := br.Responses[i]
		if err := checkSuccessfulSendResponse(sr); err != nil {
			t.Errorf("Responses[%d]: %v", i, err)
		}
	}
}

func TestSendMulticast(t *testing.T) {
	t.Skip("Skipping integration tests for deprecated SendMulticast() API")
	message := &messaging.MulticastMessage{
		Notification: &messaging.Notification{
			Title: "title",
			Body:  "body",
		},
		Tokens: []string{"INVALID_TOKEN", "ANOTHER_INVALID_TOKEN"},
	}

	br, err := client.SendMulticastDryRun(context.Background(), message)
	if err != nil {
		t.Fatal(err)
	}

	if len(br.Responses) != 2 {
		t.Errorf("len(Responses) = %d; want = 2", len(br.Responses))
	}
	if br.SuccessCount != 0 {
		t.Errorf("SuccessCount = %d; want = 0", br.SuccessCount)
	}
	if br.FailureCount != 2 {
		t.Errorf("FailureCount = %d; want = 2", br.FailureCount)
	}

	for i := 0; i < 2; i++ {
		sr := br.Responses[i]
		if err := checkErrorSendResponse(sr); err != nil {
			t.Errorf("Responses[%d]: %v", i, err)
		}
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

func checkSuccessfulSendResponse(sr *messaging.SendResponse) error {
	if !sr.Success {
		return errors.New("Success = false; want = true")
	}
	if !messageIDPattern.MatchString(sr.MessageID) {
		return fmt.Errorf("MessageID = %q; want = %q", sr.MessageID, messageIDPattern.String())
	}
	if sr.Error != nil {
		return fmt.Errorf("Error = %v; want = nil", sr.Error)
	}

	return nil
}

func checkErrorSendResponse(sr *messaging.SendResponse) error {
	if sr.Success {
		return fmt.Errorf("Success = true; want = false")
	}
	if sr.MessageID != "" {
		return fmt.Errorf("MessageID = %q; want = %q", sr.MessageID, "")
	}
	if sr.Error == nil || !messaging.IsInvalidArgument(sr.Error) {
		return fmt.Errorf("Error = %v; want = InvalidArgumentError", sr.Error)
	}

	return nil
}
