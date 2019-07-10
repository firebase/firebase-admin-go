// Copyright 2019 Google Inc. All Rights Reserved.
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

package db

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"firebase.google.com/go/internal"
)

// Listen returns an iterator that listens to realtime events
func (r *Ref) Listen(ctx context.Context) (*SnapshotIterator, error) {

	sseDataChan := make(chan string) // server-sent event data channel

	var opts []internal.HTTPOption
	opts = append(opts, internal.WithHeader("Cache-Control", "no-cache"))
	opts = append(opts, internal.WithHeader("Accept", "text/event-stream"))
	opts = append(opts, internal.WithHeader("Connection", "keep-alive"))

	resp, err := r.sendListen(ctx, "GET", opts...)

	done := true

	if err != nil {
		return &SnapshotIterator{done: &done}, err
	}

	snapshot, err := getInitialNodeSnapshot(resp)

	if err != nil {
		return &SnapshotIterator{done: &done}, err
	}

	done = false
	go r.startListeningWithReconnect(ctx, opts, resp, &done, sseDataChan)

	return &SnapshotIterator{
		Snapshot:    snapshot,
		SSEDataChan: sseDataChan,
		done:        &done,
		resp:        resp, // *http.Response
	}, err

} // Listen()

// return initial snapshot (JSON-encoded string) from Ref.Path node location
func getInitialNodeSnapshot(resp *http.Response) (string, error) {

	var b []byte

	scanner := bufio.NewScanner(resp.Body)

	if scanner.Scan() == true {

		b = scanner.Bytes()

		if "event: put" == string(b) {

			if scanner.Scan() == true {
				b = scanner.Bytes()
				s := string(b)

				// sample sse data
				// s = 'data: {"path":"/","data":{"test3":{"test4":4}}}'

				var snapshotMap map[string]interface{}

				// We only want the well formed json payload
				// exclude or trim the first 6 chars 'data: '
				err := json.Unmarshal([]byte(s[6:]), &snapshotMap)
				if err != nil {
					return "", err
				}

				snapshotByte, err := json.Marshal(snapshotMap["data"])
				if err != nil {
					return "", err
				}

				return string(snapshotByte), nil
			}
		}

	}

	return "", errors.New("sse data json error, event: put")
}

// called with goroutine
func (r *Ref) startListeningWithReconnect(
	ctx context.Context,
	opts []internal.HTTPOption,
	resp *http.Response,
	done *bool,
	sseDataChan chan<- string) {

	scanner := bufio.NewScanner(resp.Body)

	var b []byte

	for {

		if *done {
			break
		}

		if scanner.Scan() == true {

			b = scanner.Bytes()

			if "event: put" == string(b) {

				if scanner.Scan() == true {
					b = scanner.Bytes()
					s := string(b)

					// sample data
					// s = 'data: {"path":"/","data":{"test3":{"test4":4}}}'

					// sse data = path + snapshot
					if s[:5] == "data:" {
						// trim 'data: '
						sseDataChan <- s[6:] // {"path":"/","data":{"test3":{"test4":4}}}
					}
				}
			} else if "event: auth_revoked" == string(b) {

				// reconnect to re-establish authentication every hour
				var err error
				resp, err = r.sendListen(ctx, "GET", opts...)

				if err == nil {
					// not part of existing continuing listening events, so we don't send to the listening channel
					snapshot, err := getInitialNodeSnapshot(resp)
					_ = snapshot
					_ = err
				}

				scanner = bufio.NewScanner(resp.Body)
			}
		} else {

			// attemp to reconnect for other connection problems
			var err error
			resp, err = r.sendListen(ctx, "GET", opts...)

			if err == nil {
				// not part of existing continuing listening events, so we don't send to the listening channel
				snapshot, err := getInitialNodeSnapshot(resp)
				_ = snapshot
				_ = err
			}

			scanner = bufio.NewScanner(resp.Body)
		}
	}
}

// returns path and snapshot
func splitSSEData(sseData string) (path string, snapshot string, err error) {

	var sseDataMap map[string]interface{}

	err = json.Unmarshal([]byte(sseData), &sseDataMap)
	if err != nil {
		return "", "", err
	}

	pathByte, err := json.Marshal(sseDataMap["path"])
	if err != nil {
		return "", "", err
	}

	snapshotByte, err := json.Marshal(sseDataMap["data"])
	if err != nil {
		return "", "", err
	}

	path = string(pathByte)
	snapshot = string(snapshotByte)

	return
}

func (c *Client) sendListen(
	ctx context.Context,
	method, path string,
	body internal.HTTPEntity,
	opts ...internal.HTTPOption) (*http.Response, error) {

	if strings.ContainsAny(path, invalidChars) {
		return nil, fmt.Errorf("invalid path with illegal characters: %q", path)
	}
	if c.authOverride != "" {
		opts = append(opts, internal.WithQueryParam(authVarOverride, c.authOverride))
	}

	resp, err := c.hc.DoListen(ctx, &internal.Request{
		Method: method,
		URL:    fmt.Sprintf("%s%s.json", c.url, path),
		Body:   body,
		Opts:   opts,
	})

	return resp, err
}

func (r *Ref) sendListen(
	ctx context.Context,
	method string,
	opts ...internal.HTTPOption) (*http.Response, error) {

	return r.client.sendListen(ctx, method, r.Path, nil, opts...)
}
