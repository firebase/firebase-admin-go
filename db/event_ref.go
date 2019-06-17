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
	"errors"
	"fmt"
	"net/http"
	"strings"

	"firebase.google.com/go/internal"
)

// Listen ...
func (r *Ref) Listen(ctx context.Context) (*SnapshotIterator, error) {

	sseDataChan := make(chan string) // server-sent event data channel

	var opts []internal.HTTPOption
	opts = append(opts, internal.WithHeader("Cache-Control", "no-cache"))
	opts = append(opts, internal.WithHeader("Accept", "text/event-stream"))
	opts = append(opts, internal.WithHeader("Connection", "keep-alive"))

	resp, err := r.sendListen(ctx, "GET", opts...)

	if err != nil {
		return &SnapshotIterator{active: false}, err
	}

	snapshot, err := getInitialNodeSnapshot(resp)

	if err != nil {
		return &SnapshotIterator{active: false}, err
	}

	go r.startListeningWithReconnect(ctx, opts, resp, sseDataChan)

	return &SnapshotIterator{
		Snapshot:    snapshot,
		SSEDataChan: sseDataChan,
		resp:        resp, //*http.Response
		active:      true,
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

				// https://firebase.google.com/docs/reference/rest/database/#section-streaming
				// JSON encoded data payload
				// sample
				// s = data: {"path":"/","data":{"test3":{"test4":4}}}

				// sse data = path + snapshot
				// we only want snapshot
				// path is always root for initial json payload, so 1st 25 char/byte is conistent
				// data: {"path":"/","data":
				// 1234567890123456789012345
				if s[:25] != "data: {\"path\":\"/\",\"data\":" {
					return "", errors.New("sse data json error, 25 char/byte sequence")
				}

				if s[:5] == "data:" {
					ss := s[6:]         // trim first 6 chars:'data: '
					ss = ss[19:]        // trim first 19 chars:'{path":"/","data":'
					ss = ss[:len(ss)-1] // trim last char }
					return ss, nil      // {"test3":{"test4":4}} = snapshot
				}
			}
		}

	}

	return "", errors.New("sse data json error, event: put")
}

// called with goroutine
func (r *Ref) startListeningWithReconnect(ctx context.Context, opts []internal.HTTPOption, resp *http.Response, sseDataChan chan<- string) {

	scanner := bufio.NewScanner(resp.Body)

	var b []byte

	for {
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
				resp, err := r.sendListen(ctx, "GET", opts...)

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
			resp, err := r.sendListen(ctx, "GET", opts...)

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
func splitSSEData(json string) (string, string, error) {

	// sse data = path + snapshot
	// expected json payload string is similar to this:
	// {"path":"/test2","data":{"test3":{"test4":4}}}

	// IMPORTANT: quote and comma are valid in path, but are escaped in json payload
	//            so the 3 char/byte sequence used in count "," should only occur once

	count := strings.Count(json, "\",\"") // unique 3 char/byte sequence in json payload
	index := strings.Index(json, "\",\"")

	if count != 1 {
		// count must equal to 1 or json payload is incorrect
		return "", "", errors.New("sse data json count error")
	}

	if index < 11 { // 11 comes from root path which is minimum '{"path":"/"'
		return "", "", errors.New("sse data json index error")
	}

	if json[:8] == "{\"path\":" {
		path := json
		path = path[:index]
		path = path[9:] // 9 = {"path":"

		ss := json
		ss = ss[index+9:]    // trim  '..."/","data":'
		ss = ss[:len(ss)-1]  // trim last char }
		return path, ss, nil // {"test3":{"test4":4}} = snapshot
	}

	return "", "", errors.New("sse data json {\"path\": not found error")
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
