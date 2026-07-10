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
	"encoding/json"
	"errors"
	"net/http"
)

// SSE = Sever-Sent Events = ssevent

// EventType specific event type changes
type EventType uint

// EventType ...
const (
	ChildChanged EventType = iota
	ChildAdded             // to be implemented
	ChildRemoved           // to be implemented
	ValueChanged           // to be implemented
)

// Event Sever-Sent Event object
type Event struct {
	EventType EventType // ChildChanged, ValueChanged, ChildAdded, ChildRemoved

	Data string // JSON-encoded snapshot
	Path string // snapshot path
}

// SnapshotIterator iterator for continuous events
type SnapshotIterator struct {
	Snapshot    string         // initial snapshot, JSON-encoded, returned from http Respoonse, server sent event, data part
	SSEDataChan <-chan string  // continuous event snapshot, channel receive only, directional channel
	done        *bool          // Done listening to events, also used to prevent channel block
	resp        *http.Response // http connection keep alive
}

// Snapshot ssevent data, data part
func (e *Event) Snapshot() string {

	return e.Data // ssevent data (snapshot), snapshot only, data part of ssevent data
}

// Unmarshal current snapshot Event.Data
func (e *Event) Unmarshal(v interface{}) error {

	return json.Unmarshal([]byte(e.Data), v)
}

// Next realtime event
func (it *SnapshotIterator) Next() (*Event, error) {

	// prevent channel block
	if *it.done == true {
		return nil, errors.New("SnapshotIterator is done or no longer active")
	}

	sseDataString := <-it.SSEDataChan

	// todo: determine EventType

	path, snapshot, err := splitSSEData(sseDataString)

	return &Event{
		EventType: ChildChanged,
		Data:      snapshot,
		Path:      path,
	}, err

} // Next()

// Stop listening for realtime events
// close http connection
func (it *SnapshotIterator) Stop() {
	*it.done = true
	if it.resp != nil {
		it.resp.Body.Close()
	}
}

// Done can be used to check if Stop() have been called
func (it *SnapshotIterator) Done() bool {
	return *it.done
}
