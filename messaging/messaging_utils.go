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
	"fmt"
	"regexp"
	"strings"
)

var (
	bareTopicNamePattern = regexp.MustCompile("^[a-zA-Z0-9-_.~%]+$")
	colorPattern         = regexp.MustCompile("^#[0-9a-fA-F]{6}$")
)

func validateMessage(message *Message) error {
	if message == nil {
		return fmt.Errorf("message must not be nil")
	}

	targets := countNonEmpty(message.Token, message.Condition, message.Topic)
	if targets != 1 {
		return fmt.Errorf("exactly one of token, topic or condition must be specified")
	}

	// validate topic
	if message.Topic != "" {
		bt := strings.TrimPrefix(message.Topic, "/topics/")
		if !bareTopicNamePattern.MatchString(bt) {
			return fmt.Errorf("malformed topic name")
		}
	}

	// validate AndroidConfig
	if err := validateAndroidConfig(message.Android); err != nil {
		return err
	}

	// validate WebpushConfig
	if err := validateWebpushConfig(message.Webpush); err != nil {
		return err
	}

	// validate APNSConfig
	return validateAPNSConfig(message.APNS)
}

func validateAndroidConfig(config *AndroidConfig) error {
	if config == nil {
		return nil
	}

	if config.TTL != nil && config.TTL.Seconds() < 0 {
		return fmt.Errorf("ttl duration must not be negative")
	}
	if config.Priority != "" && config.Priority != "normal" && config.Priority != "high" {
		return fmt.Errorf("priority must be 'normal' or 'high'")
	}
	// validate AndroidNotification
	return validateAndroidNotification(config.Notification)
}

func validateAndroidNotification(notification *AndroidNotification) error {
	if notification == nil {
		return nil
	}
	if notification.Color != "" && !colorPattern.MatchString(notification.Color) {
		return fmt.Errorf("color must be in the #RRGGBB form")
	}
	if len(notification.TitleLocArgs) > 0 && notification.TitleLocKey == "" {
		return fmt.Errorf("titleLocKey is required when specifying titleLocArgs")
	}
	if len(notification.BodyLocArgs) > 0 && notification.BodyLocKey == "" {
		return fmt.Errorf("bodyLocKey is required when specifying bodyLocArgs")
	}
	return nil
}

func validateAPNSConfig(config *APNSConfig) error {
	if config != nil {
		return validateAPNSPayload(config.Payload)
	}
	return nil
}

func validateAPNSPayload(payload *APNSPayload) error {
	if payload != nil {
		return validateAps(payload.Aps)
	}
	return nil
}

func validateAps(aps *Aps) error {
	if aps != nil {
		if aps.Alert != nil && aps.AlertString != "" {
			return fmt.Errorf("multiple alert specifications")
		}
		m := aps.standardFields()
		for k := range aps.CustomData {
			if _, contains := m[k]; contains {
				return fmt.Errorf("multiple specifications for the key %q", k)
			}
		}
		return validateApsAlert(aps.Alert)
	}
	return nil
}

func validateApsAlert(alert *ApsAlert) error {
	if alert == nil {
		return nil
	}
	if len(alert.TitleLocArgs) > 0 && alert.TitleLocKey == "" {
		return fmt.Errorf("titleLocKey is required when specifying titleLocArgs")
	}
	if len(alert.LocArgs) > 0 && alert.LocKey == "" {
		return fmt.Errorf("locKey is required when specifying locArgs")
	}
	return nil
}

func validateWebpushConfig(webpush *WebpushConfig) error {
	if webpush == nil || webpush.Notification == nil {
		return nil
	}
	dir := webpush.Notification.Direction
	if dir != "" && dir != "ltr" && dir != "rtl" && dir != "auto" {
		return fmt.Errorf("direction must be 'ltr', 'rtl' or 'auto'")
	}
	m := webpush.Notification.standardFields()
	for k := range webpush.Notification.CustomData {
		if _, contains := m[k]; contains {
			return fmt.Errorf("multiple specifications for the key %q", k)
		}
	}
	return nil
}

func countNonEmpty(strings ...string) int {
	count := 0
	for _, s := range strings {
		if s != "" {
			count++
		}
	}
	return count
}
