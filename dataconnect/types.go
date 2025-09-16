// Copyright 2024 Google Inc. All Rights Reserved.
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

// Package dataconnect provides functions for interacting with the Firebase Data Connect service.
package dataconnect

// ConnectorConfig is the configuration for the Data Connect service.
type ConnectorConfig struct {
	Location  string `json:"location"`
	ServiceID string `json:"serviceId"`
}

// GraphqlOptions represents the options for a GraphQL query.
type GraphqlOptions struct {
	Variables     map[string]interface{} `json:"variables,omitempty"`
	OperationName string                 `json:"operationName,omitempty"`
}

// ExecuteGraphqlResponse is the response from a GraphQL query.
type ExecuteGraphqlResponse struct {
	Data map[string]interface{} `json:"data"`
}
