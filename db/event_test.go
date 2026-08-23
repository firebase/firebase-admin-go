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
	"context"
	"testing"
)

func TestListen(t *testing.T) {
	mock := &mockServer{}
	srv := mock.Start(client)
	defer srv.Close()

	// mostly just a place holder, currenty test for crash only

	iter, err := testref.Listen(context.Background())
	_ = iter
	_ = err

	defer iter.Stop()

	event, err := iter.Next()
	_ = event
	_ = err

	if iter.Done() {
		// break
	}

	if err == nil {
		iter.Stop()
	}

	b := iter.Done()
	_ = b

}
