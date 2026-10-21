// Copyright 2017 Google LLC All Rights Reserved.
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

package firebase

import (
	"firebase.google.com/go/v4/internal"
	"google.golang.org/api/option"
)

// RetryConfig specifies how Admin SDK HTTP clients should retry failing requests.
type RetryConfig = internal.RetryConfig

// WithRetryConfig creates a ClientOption that configures HTTP retry behavior.
//
// Pass this option to NewApp() to configure retries for service clients.
// If set with a nil RetryConfig, retries are disabled.
func WithRetryConfig(retryConfig *RetryConfig) option.ClientOption {
	return internal.WithRetryConfig(retryConfig)
}
