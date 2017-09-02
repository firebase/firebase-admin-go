// Copyright 2017 Google Inc. All Rights Reserved.
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
package integration

import (
	"flag"
	"fmt"
	"os"
	"testing"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/integration/internal"
	"golang.org/x/net/context"
)

var ctx context.Context
var app *firebase.App

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		fmt.Println("skipping auth integration tests in short mode.")
		os.Exit(0)
	}

	ctx = context.Background()
	var err error
	app, err = internal.NewTestApp(ctx)
	if err != nil {
		os.Exit(1)
	}
	os.Exit(m.Run())
}
