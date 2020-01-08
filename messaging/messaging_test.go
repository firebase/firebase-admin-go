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
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"firebase.google.com/go/internal"
	"google.golang.org/api/option"
)

const testMessageID = "projects/test-project/messages/msg_id"

var (
	testMessagingConfig = &internal.MessagingConfig{
		ProjectID: "test-project",
		Opts: []option.ClientOption{
			option.WithTokenSource(&internal.MockTokenSource{AccessToken: "test-token"}),
		},
		Version: "test-version",
	}

	ttlWithNanos = time.Duration(1500) * time.Millisecond
	ttl          = time.Duration(10) * time.Second
	invalidTTL   = time.Duration(-10) * time.Second

	badge           = 42
	badgeZero       = 0
	timestampMillis = int64(12345)
	timestamp       = time.Unix(0, 1546304523123*1000000).UTC()
)

var validMessages = []struct {
	name string
	req  *Message
	want map[string]interface{}
}{
	{
		name: "TokenOnly",
		req:  &Message{Token: "test-token"},
		want: map[string]interface{}{"token": "test-token"},
	},
	{
		name: "TopicOnly",
		req:  &Message{Topic: "test-topic"},
		want: map[string]interface{}{"topic": "test-topic"},
	},
	{
		name: "PrefixedTopicOnly",
		req:  &Message{Topic: "/topics/test-topic"},
		want: map[string]interface{}{"topic": "test-topic"},
	},
	{
		name: "ConditionOnly",
		req:  &Message{Condition: "test-condition"},
		want: map[string]interface{}{"condition": "test-condition"},
	},
	{
		name: "DataMessage",
		req: &Message{
			Data: map[string]string{
				"k1": "v1",
				"k2": "v2",
			},
			FCMOptions: &FCMOptions{
				AnalyticsLabel: "Analytics",
			},
			Topic: "test-topic",
		},
		want: map[string]interface{}{
			"data": map[string]interface{}{
				"k1": "v1",
				"k2": "v2",
			},
			"fcm_options": map[string]interface{}{
				"analytics_label": "Analytics",
			},
			"topic": "test-topic",
		},
	},
	{
		name: "NotificationMessage",
		req: &Message{
			Notification: &Notification{
				Title:    "t",
				Body:     "b",
				ImageURL: "http://image.jpg",
			},
			Topic: "test-topic",
		},
		want: map[string]interface{}{
			"notification": map[string]interface{}{
				"title": "t",
				"body":  "b",
				"image": "http://image.jpg",
			},
			"topic": "test-topic",
		},
	},
	{
		name: "AndroidDataMessage",
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
		name: "AndroidNotificationMessage",
		req: &Message{
			Android: &AndroidConfig{
				RestrictedPackageName: "rpn",
				Notification: &AndroidNotification{
					Title:                 "t",
					Body:                  "b",
					Color:                 "#112233",
					Sound:                 "s",
					TitleLocKey:           "tlk",
					TitleLocArgs:          []string{"t1", "t2"},
					BodyLocKey:            "blk",
					BodyLocArgs:           []string{"b1", "b2"},
					ChannelID:             "channel",
					ImageURL:              "http://image.jpg",
					Ticker:                "tkr",
					Sticky:                true,
					EventTimestamp:        &timestamp,
					LocalOnly:             true,
					Priority:              PriorityMax,
					VibrateTimingMillis:   []int64{100, 50, 100},
					DefaultVibrateTimings: true,
					DefaultSound:          true,
					LightSettings: &LightSettings{
						Color:                  "#33669966",
						LightOnDurationMillis:  100,
						LightOffDurationMillis: 50,
					},
					Visibility:           VisibilityPrivate,
					DefaultLightSettings: true,
				},
				TTL: &ttlWithNanos,
				FCMOptions: &AndroidFCMOptions{
					AnalyticsLabel: "Analytics",
				},
			},
			Topic: "test-topic",
		},
		want: map[string]interface{}{
			"android": map[string]interface{}{
				"restricted_package_name": "rpn",
				"notification": map[string]interface{}{
					"title":                   "t",
					"body":                    "b",
					"color":                   "#112233",
					"sound":                   "s",
					"title_loc_key":           "tlk",
					"title_loc_args":          []interface{}{"t1", "t2"},
					"body_loc_key":            "blk",
					"body_loc_args":           []interface{}{"b1", "b2"},
					"channel_id":              "channel",
					"image":                   "http://image.jpg",
					"ticker":                  "tkr",
					"sticky":                  true,
					"event_time":              "2019-01-01T01:02:03.123000000Z",
					"local_only":              true,
					"notification_priority":   "PRIORITY_MAX",
					"vibrate_timings":         []interface{}{"0.100000000s", "0.050000000s", "0.100000000s"},
					"default_vibrate_timings": true,
					"default_sound":           true,
					"light_settings": map[string]interface{}{
						"color": map[string]interface{}{
							"red":   float64(0.2),
							"green": float64(0.4),
							"blue":  float64(0.6),
							"alpha": float64(0.4),
						},
						"light_on_duration":  "0.100000000s",
						"light_off_duration": "0.050000000s",
					},
					"visibility":             "PRIVATE",
					"default_light_settings": true,
				},
				"ttl": "1.500000000s",
				"fcm_options": map[string]interface{}{
					"analytics_label": "Analytics",
				},
			},
			"topic": "test-topic",
		},
	},
	{
		name: "AndroidNotificationLightSettings",
		req: &Message{
			Android: &AndroidConfig{
				Notification: &AndroidNotification{
					LightSettings: &LightSettings{
						Color:                  "#336699",
						LightOnDurationMillis:  100,
						LightOffDurationMillis: 50,
					},
				},
			},
			Topic: "test-topic",
		},
		want: map[string]interface{}{
			"android": map[string]interface{}{
				"notification": map[string]interface{}{
					"light_settings": map[string]interface{}{
						"color": map[string]interface{}{
							"red":   float64(0.2),
							"green": float64(0.4),
							"blue":  float64(0.6),
							"alpha": float64(1.0),
						},
						"light_on_duration":  "0.100000000s",
						"light_off_duration": "0.050000000s",
					},
				},
			},
			"topic": "test-topic",
		},
	},
	{
		name: "AndroidNoTTL",
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
	{
		name: "WebpushMessage",
		req: &Message{
			Webpush: &WebpushConfig{
				Headers: map[string]string{
					"h1": "v1",
					"h2": "v2",
				},
				Data: map[string]string{
					"k1": "v1",
					"k2": "v2",
				},
				Notification: &WebpushNotification{
					Title: "title",
					Body:  "body",
					Icon:  "icon",
					Actions: []*WebpushNotificationAction{
						{
							Action: "a1",
							Title:  "a1-title",
						},
						{
							Action: "a2",
							Title:  "a2-title",
							Icon:   "a2-icon",
						},
					},
					Badge:              "badge",
					Data:               "data",
					Image:              "image",
					Language:           "lang",
					Renotify:           true,
					RequireInteraction: true,
					Silent:             true,
					Tag:                "tag",
					TimestampMillis:    &timestampMillis,
					Vibrate:            []int{100, 200, 100},
					CustomData:         map[string]interface{}{"k1": "v1", "k2": "v2"},
				},
				FcmOptions: &WebpushFcmOptions{
					Link: "https://link.com",
				},
			},
			Topic: "test-topic",
		},
		want: map[string]interface{}{
			"webpush": map[string]interface{}{
				"headers": map[string]interface{}{"h1": "v1", "h2": "v2"},
				"data":    map[string]interface{}{"k1": "v1", "k2": "v2"},
				"notification": map[string]interface{}{
					"title": "title",
					"body":  "body",
					"icon":  "icon",
					"actions": []interface{}{
						map[string]interface{}{"action": "a1", "title": "a1-title"},
						map[string]interface{}{"action": "a2", "title": "a2-title", "icon": "a2-icon"},
					},
					"badge":              "badge",
					"data":               "data",
					"image":              "image",
					"lang":               "lang",
					"renotify":           true,
					"requireInteraction": true,
					"silent":             true,
					"tag":                "tag",
					"timestamp":          float64(12345),
					"vibrate":            []interface{}{float64(100), float64(200), float64(100)},
					"k1":                 "v1",
					"k2":                 "v2",
				},
				"fcm_options": map[string]interface{}{
					"link": "https://link.com",
				},
			},
			"topic": "test-topic",
		},
	},
	{
		name: "APNSHeadersOnly",
		req: &Message{
			APNS: &APNSConfig{
				Headers: map[string]string{
					"h1": "v1",
					"h2": "v2",
				},
			},
			Topic: "test-topic",
		},
		want: map[string]interface{}{
			"apns": map[string]interface{}{
				"headers": map[string]interface{}{"h1": "v1", "h2": "v2"},
			},
			"topic": "test-topic",
		},
	},
	{
		name: "APNSAlertString",
		req: &Message{
			APNS: &APNSConfig{
				Headers: map[string]string{
					"h1": "v1",
					"h2": "v2",
				},
				Payload: &APNSPayload{
					Aps: &Aps{
						AlertString:      "a",
						Badge:            &badge,
						Category:         "c",
						Sound:            "s",
						ThreadID:         "t",
						ContentAvailable: true,
						MutableContent:   true,
					},
					CustomData: map[string]interface{}{
						"k1": "v1",
						"k2": true,
					},
				},
				FCMOptions: &APNSFCMOptions{
					AnalyticsLabel: "Analytics",
					ImageURL:       "http://image.jpg",
				},
			},
			Topic: "test-topic",
		},
		want: map[string]interface{}{
			"apns": map[string]interface{}{
				"headers": map[string]interface{}{"h1": "v1", "h2": "v2"},
				"payload": map[string]interface{}{
					"aps": map[string]interface{}{
						"alert":             "a",
						"badge":             float64(badge),
						"category":          "c",
						"sound":             "s",
						"thread-id":         "t",
						"content-available": float64(1),
						"mutable-content":   float64(1),
					},
					"k1": "v1",
					"k2": true,
				},
				"fcm_options": map[string]interface{}{
					"analytics_label": "Analytics",
					"image":           "http://image.jpg",
				},
			},
			"topic": "test-topic",
		},
	},
	{
		name: "APNSAlertCrticalSound",
		req: &Message{
			APNS: &APNSConfig{
				Headers: map[string]string{
					"h1": "v1",
					"h2": "v2",
				},
				Payload: &APNSPayload{
					Aps: &Aps{
						AlertString: "a",
						Badge:       &badge,
						Category:    "c",
						CriticalSound: &CriticalSound{
							Critical: true,
							Name:     "n",
							Volume:   0.7,
						},
						ThreadID:         "t",
						ContentAvailable: true,
						MutableContent:   true,
					},
					CustomData: map[string]interface{}{
						"k1": "v1",
						"k2": true,
					},
				},
			},
			Topic: "test-topic",
		},
		want: map[string]interface{}{
			"apns": map[string]interface{}{
				"headers": map[string]interface{}{"h1": "v1", "h2": "v2"},
				"payload": map[string]interface{}{
					"aps": map[string]interface{}{
						"alert":    "a",
						"badge":    float64(badge),
						"category": "c",
						"sound": map[string]interface{}{
							"critical": float64(1),
							"name":     "n",
							"volume":   float64(0.7),
						},
						"thread-id":         "t",
						"content-available": float64(1),
						"mutable-content":   float64(1),
					},
					"k1": "v1",
					"k2": true,
				},
			},
			"topic": "test-topic",
		},
	},
	{
		name: "APNSBadgeZero",
		req: &Message{
			APNS: &APNSConfig{
				Payload: &APNSPayload{
					Aps: &Aps{
						Badge:            &badgeZero,
						Category:         "c",
						Sound:            "s",
						ThreadID:         "t",
						ContentAvailable: true,
						MutableContent:   true,
						CustomData:       map[string]interface{}{"k1": "v1", "k2": float64(1)},
					},
				},
			},
			Topic: "test-topic",
		},
		want: map[string]interface{}{
			"apns": map[string]interface{}{
				"payload": map[string]interface{}{
					"aps": map[string]interface{}{
						"badge":             float64(badgeZero),
						"category":          "c",
						"sound":             "s",
						"thread-id":         "t",
						"content-available": float64(1),
						"mutable-content":   float64(1),
						"k1":                "v1",
						"k2":                float64(1),
					},
				},
			},
			"topic": "test-topic",
		},
	},
	{
		name: "APNSAlertObject",
		req: &Message{
			APNS: &APNSConfig{
				Payload: &APNSPayload{
					Aps: &Aps{
						Alert: &ApsAlert{
							Title:           "t",
							SubTitle:        "st",
							Body:            "b",
							TitleLocKey:     "tlk",
							TitleLocArgs:    []string{"t1", "t2"},
							SubTitleLocKey:  "stlk",
							SubTitleLocArgs: []string{"t1", "t2"},
							LocKey:          "blk",
							LocArgs:         []string{"b1", "b2"},
							ActionLocKey:    "alk",
							LaunchImage:     "li",
						},
					},
				},
			},
			Topic: "test-topic",
		},
		want: map[string]interface{}{
			"apns": map[string]interface{}{
				"payload": map[string]interface{}{
					"aps": map[string]interface{}{
						"alert": map[string]interface{}{
							"title":             "t",
							"subtitle":          "st",
							"body":              "b",
							"title-loc-key":     "tlk",
							"title-loc-args":    []interface{}{"t1", "t2"},
							"subtitle-loc-key":  "stlk",
							"subtitle-loc-args": []interface{}{"t1", "t2"},
							"loc-key":           "blk",
							"loc-args":          []interface{}{"b1", "b2"},
							"action-loc-key":    "alk",
							"launch-image":      "li",
						},
					},
				},
			},
			"topic": "test-topic",
		},
	},
}

var invalidMessages = []struct {
	name string
	req  *Message
	want string
}{
	{
		name: "NilMessage",
		req:  nil,
		want: "message must not be nil",
	},
	{
		name: "NoTargets",
		req:  &Message{},
		want: "exactly one of token, topic or condition must be specified",
	},
	{
		name: "MultipleTargets",
		req: &Message{
			Token: "token",
			Topic: "topic",
		},
		want: "exactly one of token, topic or condition must be specified",
	},
	{
		name: "InvalidPrefixedTopicName",
		req: &Message{
			Topic: "/topics/",
		},
		want: "malformed topic name",
	},
	{
		name: "InvalidTopicName",
		req: &Message{
			Topic: "foo*bar",
		},
		want: "malformed topic name",
	},
	{
		name: "InvalidNotificationImage",
		req: &Message{
			Notification: &Notification{
				ImageURL: "image.jpg",
			},
			Topic: "topic",
		},
		want: `invalid image URL: "image.jpg"`,
	},
	{
		name: "InvalidAndroidTTL",
		req: &Message{
			Android: &AndroidConfig{
				TTL: &invalidTTL,
			},
			Topic: "topic",
		},
		want: "ttl duration must not be negative",
	},
	{
		name: "InvalidAndroidPriority",
		req: &Message{
			Android: &AndroidConfig{
				Priority: "not normal",
			},
			Topic: "topic",
		},
		want: "priority must be 'normal' or 'high'",
	},
	{
		name: "InvalidAndroidColor1",
		req: &Message{
			Android: &AndroidConfig{
				Notification: &AndroidNotification{
					Color: "112233",
				},
			},
			Topic: "topic",
		},
		want: "color must be in the #RRGGBB form",
	},
	{
		name: "InvalidAndroidColor2",
		req: &Message{
			Android: &AndroidConfig{
				Notification: &AndroidNotification{
					Color: "#112233X",
				},
			},
			Topic: "topic",
		},
		want: "color must be in the #RRGGBB form",
	},
	{
		name: "InvalidAndroidTitleLocArgs",
		req: &Message{
			Android: &AndroidConfig{
				Notification: &AndroidNotification{
					TitleLocArgs: []string{"a1"},
				},
			},
			Topic: "topic",
		},
		want: "titleLocKey is required when specifying titleLocArgs",
	},
	{
		name: "InvalidAndroidBodyLocArgs",
		req: &Message{
			Android: &AndroidConfig{
				Notification: &AndroidNotification{
					BodyLocArgs: []string{"a1"},
				},
			},
			Topic: "topic",
		},
		want: "bodyLocKey is required when specifying bodyLocArgs",
	},
	{
		name: "InvalidAndroidImage",
		req: &Message{
			Android: &AndroidConfig{
				Notification: &AndroidNotification{
					ImageURL: "image.jpg",
				},
			},
			Topic: "topic",
		},
		want: `invalid image URL: "image.jpg"`,
	},
	{
		name: "InvalidLightSettingsColor1",
		req: &Message{
			Android: &AndroidConfig{
				Notification: &AndroidNotification{
					LightSettings: &LightSettings{
						Color: "112233",
					},
				},
			},
			Topic: "topic",
		},
		want: "color must be in #RRGGBB or #RRGGBBAA form",
	},
	{
		name: "InvalidLightSettingsColor2",
		req: &Message{
			Android: &AndroidConfig{
				Notification: &AndroidNotification{
					LightSettings: &LightSettings{
						Color: "#11223X",
					},
				},
			},
			Topic: "topic",
		},
		want: "color must be in #RRGGBB or #RRGGBBAA form",
	},
	{
		name: "InvalidLightSettingsColor3",
		req: &Message{
			Android: &AndroidConfig{
				Notification: &AndroidNotification{
					LightSettings: &LightSettings{
						Color: "#112234X",
					},
				},
			},
			Topic: "topic",
		},
		want: "color must be in #RRGGBB or #RRGGBBAA form",
	},
	{
		name: "InvalidLightSettingsOnDuration",
		req: &Message{
			Android: &AndroidConfig{
				Notification: &AndroidNotification{
					LightSettings: &LightSettings{
						Color:                 "#112233",
						LightOnDurationMillis: -1,
					},
				},
			},
			Topic: "topic",
		},
		want: "lightOnDuration must not be negative",
	},
	{
		name: "InvalidLightSettingsOffDuration",
		req: &Message{
			Android: &AndroidConfig{
				Notification: &AndroidNotification{
					LightSettings: &LightSettings{
						Color:                  "#112233",
						LightOffDurationMillis: -1,
					},
				},
			},
			Topic: "topic",
		},
		want: "lightOffDuration must not be negative",
	},
	{
		name: "InvalidVibrateTimings",
		req: &Message{
			Android: &AndroidConfig{
				Notification: &AndroidNotification{
					VibrateTimingMillis: []int64{100, -1, 100},
				},
			},
			Topic: "topic",
		},
		want: "vibrateTimingMillis must not be negative",
	},
	{
		name: "APNSMultipleAps",
		req: &Message{
			APNS: &APNSConfig{
				Payload: &APNSPayload{
					Aps: &Aps{
						AlertString: "alert",
					},
					CustomData: map[string]interface{}{
						"aps": map[string]interface{}{"key": "value"},
					},
				},
			},
			Topic: "topic",
		},
		want: `multiple specifications for the key "aps"`,
	},
	{
		name: "APNSMultipleAlerts",
		req: &Message{
			APNS: &APNSConfig{
				Payload: &APNSPayload{
					Aps: &Aps{
						Alert:       &ApsAlert{},
						AlertString: "alert",
					},
				},
			},
			Topic: "topic",
		},
		want: "multiple alert specifications",
	},
	{
		name: "APNSMultipleFieldSpecifications",
		req: &Message{
			APNS: &APNSConfig{
				Payload: &APNSPayload{
					Aps: &Aps{
						Category:   "category",
						CustomData: map[string]interface{}{"category": "category"},
					},
				},
			},
			Topic: "topic",
		},
		want: `multiple specifications for the key "category"`,
	},
	{
		name: "InvalidAPNSTitleLocArgs",
		req: &Message{
			APNS: &APNSConfig{
				Payload: &APNSPayload{
					Aps: &Aps{
						Alert: &ApsAlert{
							TitleLocArgs: []string{"a1"},
						},
					},
				},
			},
			Topic: "topic",
		},
		want: "titleLocKey is required when specifying titleLocArgs",
	},
	{
		name: "InvalidAPNSSubTitleLocArgs",
		req: &Message{
			APNS: &APNSConfig{
				Payload: &APNSPayload{
					Aps: &Aps{
						Alert: &ApsAlert{
							SubTitleLocArgs: []string{"a1"},
						},
					},
				},
			},
			Topic: "topic",
		},
		want: "subtitleLocKey is required when specifying subtitleLocArgs",
	},
	{
		name: "InvalidAPNSLocArgs",
		req: &Message{
			APNS: &APNSConfig{
				Payload: &APNSPayload{
					Aps: &Aps{
						Alert: &ApsAlert{
							LocArgs: []string{"a1"},
						},
					},
				},
			},
			Topic: "topic",
		},
		want: "locKey is required when specifying locArgs",
	},
	{
		name: "InvalidAPNSImage",
		req: &Message{
			APNS: &APNSConfig{
				FCMOptions: &APNSFCMOptions{
					ImageURL: "image.jpg",
				},
			},
			Topic: "topic",
		},
		want: `invalid image URL: "image.jpg"`,
	},
	{
		name: "MultipleSoundSpecifications",
		req: &Message{
			APNS: &APNSConfig{
				Payload: &APNSPayload{
					Aps: &Aps{
						Sound: "s",
						CriticalSound: &CriticalSound{
							Name: "s",
						},
					},
				},
			},
			Topic: "topic",
		},
		want: "multiple sound specifications",
	},
	{
		name: "VolumeTooLow",
		req: &Message{
			APNS: &APNSConfig{
				Payload: &APNSPayload{
					Aps: &Aps{
						CriticalSound: &CriticalSound{
							Name:   "s",
							Volume: -0.1,
						},
					},
				},
			},
			Topic: "topic",
		},
		want: "critical sound volume must be in the interval [0, 1]",
	},
	{
		name: "VolumeTooHigh",
		req: &Message{
			APNS: &APNSConfig{
				Payload: &APNSPayload{
					Aps: &Aps{
						CriticalSound: &CriticalSound{
							Name:   "s",
							Volume: 1.1,
						},
					},
				},
			},
			Topic: "topic",
		},
		want: "critical sound volume must be in the interval [0, 1]",
	},
	{
		name: "InvalidWebpushNotificationDirection",
		req: &Message{
			Webpush: &WebpushConfig{
				Notification: &WebpushNotification{
					Direction: "invalid",
				},
			},
			Topic: "topic",
		},
		want: "direction must be 'ltr', 'rtl' or 'auto'",
	},
	{
		name: "WebpushNotificationMultipleFieldSpecifications",
		req: &Message{
			Webpush: &WebpushConfig{
				Notification: &WebpushNotification{
					Direction:  "ltr",
					CustomData: map[string]interface{}{"dir": "rtl"},
				},
			},
			Topic: "topic",
		},
		want: `multiple specifications for the key "dir"`,
	},
	{
		name: "InvalidWebpushFcmOptionsLink",
		req: &Message{
			Webpush: &WebpushConfig{
				Notification: &WebpushNotification{},
				FcmOptions: &WebpushFcmOptions{
					Link: "link",
				},
			},
			Topic: "topic",
		},
		want: `invalid link URL: "link"`,
	},
	{
		name: "InvalidWebpushFcmOptionsLinkScheme",
		req: &Message{
			Webpush: &WebpushConfig{
				Notification: &WebpushNotification{},
				FcmOptions: &WebpushFcmOptions{
					Link: "http://link.com",
				},
			},
			Topic: "topic",
		},
		want: `invalid link URL: "http://link.com"; want scheme: "https"`,
	},
}

func TestNoProjectID(t *testing.T) {
	client, err := NewClient(context.Background(), &internal.MessagingConfig{})
	if client != nil || err == nil {
		t.Errorf("NewClient() = (%v, %v); want = (nil, error)", client, err)
	}
}

func TestJSONUnmarshal(t *testing.T) {
	for _, tc := range validMessages {
		if tc.name == "PrefixedTopicOnly" {
			continue
		}
		b, err := json.Marshal(tc.req)
		if err != nil {
			t.Errorf("Marshal(%s) = %v; want = nil", tc.name, err)
		}
		var target Message
		if err := json.Unmarshal(b, &target); err != nil {
			t.Errorf("Unmarshal(%s) = %v; want = nil", tc.name, err)
		}
		if !reflect.DeepEqual(tc.req, &target) {
			t.Errorf("Unmarshal(%s) result = %#v; want = %#v", tc.name, tc.req, target)
		}
	}
}

func TestInvalidJSONUnmarshal(t *testing.T) {
	cases := []struct {
		name   string
		req    map[string]interface{}
		target interface{}
	}{
		{
			name: "InvalidTTLSegments",
			req: map[string]interface{}{
				"ttl": "1.2.3s",
			},
			target: &AndroidConfig{},
		},
		{
			name: "IncorrectTTLSeconds",
			req: map[string]interface{}{
				"ttl": "abcs",
			},
			target: &AndroidConfig{},
		},
		{
			name: "IncorrectTTLNanoseconds",
			req: map[string]interface{}{
				"ttl": "10.abcs",
			},
			target: &AndroidConfig{},
		},
		{
			name: "InvalidApsAlert",
			req: map[string]interface{}{
				"alert": 10,
			},
			target: &Aps{},
		},
		{
			name: "InvalidApsSound",
			req: map[string]interface{}{
				"sound": 10,
			},
			target: &Aps{},
		},
		{
			name: "InvalidPriority",
			req: map[string]interface{}{
				"notification_priority": "invalid",
			},
			target: &AndroidNotification{},
		},
		{
			name: "InvalidVisibility",
			req: map[string]interface{}{
				"visibility": "invalid",
			},
			target: &AndroidNotification{},
		},
		{
			name: "InvalidEventTimestamp",
			req: map[string]interface{}{
				"event_time": "invalid",
			},
			target: &AndroidNotification{},
		},
		{
			name: "IncorrectLightOnDuration",
			req: map[string]interface{}{
				"light_on_duration": "10.abcs",
			},
			target: &LightSettings{},
		},
		{
			name: "IncorrectLightOffDuration",
			req: map[string]interface{}{
				"light_on_duration":  "1s",
				"light_off_duration": "10.abcs",
			},
			target: &LightSettings{},
		},
	}
	for _, tc := range cases {
		b, err := json.Marshal(tc.req)
		if err != nil {
			t.Errorf("Marshal(%s) = %v; want = nil", tc.name, err)
		}
		if err := json.Unmarshal(b, tc.target); err == nil {
			t.Errorf("Unmarshal(%s) = %v; want = error", tc.name, err)
		}
	}
}

func TestSend(t *testing.T) {
	var tr *http.Request
	var b []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr = r
		b, _ = ioutil.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{ \"name\":\"" + testMessageID + "\" }"))
	}))
	defer ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmEndpoint = ts.URL

	for _, tc := range validMessages {
		t.Run(tc.name, func(t *testing.T) {
			name, err := client.Send(ctx, tc.req)
			if name != testMessageID || err != nil {
				t.Errorf("Send(%s) = (%q, %v); want = (%q, nil)", tc.name, name, err, testMessageID)
			}
			checkFCMRequest(t, b, tr, tc.want, false)
		})
	}
}

func TestSendDryRun(t *testing.T) {
	var tr *http.Request
	var b []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr = r
		b, _ = ioutil.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{ \"name\":\"" + testMessageID + "\" }"))
	}))
	defer ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmEndpoint = ts.URL

	for _, tc := range validMessages {
		t.Run(tc.name, func(t *testing.T) {
			name, err := client.SendDryRun(ctx, tc.req)
			if name != testMessageID || err != nil {
				t.Errorf("SendDryRun(%s) = (%q, %v); want = (%q, nil)", tc.name, name, err, testMessageID)
			}
			checkFCMRequest(t, b, tr, tc.want, true)
		})
	}
}

func TestSendError(t *testing.T) {
	var resp string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(resp))
	}))
	defer ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmEndpoint = ts.URL
	client.fcmClient.httpClient.RetryConfig = nil

	for _, tc := range httpErrors {
		resp = tc.resp
		name, err := client.Send(ctx, &Message{Topic: "topic"})
		if err == nil || err.Error() != tc.want || !tc.check(err) {
			t.Errorf("Send() = (%q, %v); want = (%q, %q)", name, err, "", tc.want)
		}
	}
}

func TestInvalidMessage(t *testing.T) {
	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range invalidMessages {
		t.Run(tc.name, func(t *testing.T) {
			name, err := client.Send(ctx, tc.req)
			if err == nil || err.Error() != tc.want {
				t.Errorf("Send(%s) = (%q, %v); want = (%q, %q)", tc.name, name, err, "", tc.want)
			}
		})
	}
}

func checkFCMRequest(t *testing.T, b []byte, tr *http.Request, want map[string]interface{}, dryRun bool) {
	var parsed map[string]interface{}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(parsed["message"], want) {
		t.Errorf("Body = %#v; want = %#v", parsed["message"], want)
	}

	validate, ok := parsed["validate_only"]
	if dryRun {
		if !ok || validate != true {
			t.Errorf("ValidateOnly = %v; want = true", validate)
		}
	} else if ok {
		t.Errorf("ValidateOnly = %v; want none", validate)
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
	if h := tr.Header.Get("X-GOOG-API-FORMAT-VERSION"); h != "2" {
		t.Errorf("X-GOOG-API-FORMAT-VERSION = %q; want = %q", h, "2")
	}

	clientVersion := "fire-admin-go/" + testMessagingConfig.Version
	if h := tr.Header.Get("X-FIREBASE-CLIENT"); h != clientVersion {
		t.Errorf("X-FIREBASE-CLIENT = %q; want = %q", h, clientVersion)
	}
}

var httpErrors = []struct {
	resp, want string
	check      func(error) bool
}{
	{
		resp:  "{}",
		want:  "http error status: 500; reason: server responded with an unknown error; response: {}",
		check: IsUnknown,
	},
	{
		resp:  "{\"error\": {\"status\": \"INVALID_ARGUMENT\", \"message\": \"test error\"}}",
		want:  "http error status: 500; reason: request contains an invalid argument; code: invalid-argument; details: test error",
		check: IsInvalidArgument,
	},
	{
		resp: "{\"error\": {\"status\": \"NOT_FOUND\", \"message\": \"test error\"}}",
		want: "http error status: 500; reason: app instance has been unregistered; code: registration-token-not-registered; " +
			"details: test error",
		check: IsRegistrationTokenNotRegistered,
	},
	{
		resp: "{\"error\": {\"status\": \"QUOTA_EXCEEDED\", \"message\": \"test error\"}}",
		want: "http error status: 500; reason: messaging service quota exceeded; code: message-rate-exceeded; " +
			"details: test error",
		check: IsMessageRateExceeded,
	},
	{
		resp: "{\"error\": {\"status\": \"UNAVAILABLE\", \"message\": \"test error\"}}",
		want: "http error status: 500; reason: backend servers are temporarily unavailable; code: server-unavailable; " +
			"details: test error",
		check: IsServerUnavailable,
	},
	{
		resp: "{\"error\": {\"status\": \"INTERNAL\", \"message\": \"test error\"}}",
		want: "http error status: 500; reason: backend servers encountered an unknown internl error; code: internal-error; " +
			"details: test error",
		check: IsInternal,
	},
	{
		resp: "{\"error\": {\"status\": \"APNS_AUTH_ERROR\", \"message\": \"test error\"}}",
		want: "http error status: 500; reason: apns certificate or auth key was invalid; code: invalid-apns-credentials; " +
			"details: test error",
		check: IsInvalidAPNSCredentials,
	},
	{
		resp: "{\"error\": {\"status\": \"SENDER_ID_MISMATCH\", \"message\": \"test error\"}}",
		want: "http error status: 500; reason: sender id does not match registration token; code: mismatched-credential; " +
			"details: test error",
		check: IsMismatchedCredential,
	},
	{
		resp: `{"error": {"status": "INVALID_ARGUMENT", "message": "test error", "details": [` +
			`{"@type": "type.googleapis.com/google.firebase.fcm.v1.FcmError", "errorCode": "UNREGISTERED"}]}}`,
		want: "http error status: 500; reason: app instance has been unregistered; code: registration-token-not-registered; " +
			"details: test error",
		check: IsRegistrationTokenNotRegistered,
	},
	{
		resp:  "not json",
		want:  "http error status: 500; reason: server responded with an unknown error; response: not json",
		check: IsUnknown,
	},
}
