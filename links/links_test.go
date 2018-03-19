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

package links

import (
	"encoding/json"
	/*	"encoding/json"
		"errors"*/
	"fmt"
	"io/ioutil"
	"log"
	"os"
	//	"strings"
	"testing"

	"golang.org/x/net/context"

	"google.golang.org/api/option"
	//	"google.golang.org/api/transport"
	//	"google.golang.org/appengine"
	//	"google.golang.org/appengine/aetest"
	//	"firebase.google.com/go/internal"
)

var client *Client
var ctx context.Context
var testLinkStatsResponse []byte
var defaultTestOpts = []option.ClientOption{
	option.WithCredentialsFile("../testdata/service_account.json"),
}

func TestMain(m *testing.M) {
	var err error

	client, err = NewClient(ctx, defaultTestOpts...)
	if err != nil {
		log.Fatalln(err)
	}

	testLinkStatsResponse, err = ioutil.ReadFile("../testdata/get_link_stats.json")
	if err != nil {
		log.Fatalln(err)
	}
	os.Exit(m.Run())
}

func TestCreateEventStats(t *testing.T) {
	es := &EventStats{Platform: DESKTOP, ET: CLICK, Count: 4}
	fmt.Println(es)
	m, err := json.Marshal(es)
	t.Errorf("%v :: %#v  marshalled: -'- %s -'-  err:%v ", es, es, m, err)
}

func TestReadJSON(t *testing.T) {
	var ls LinkStats
	err := json.Unmarshal(testLinkStatsResponse, &ls)
	if err != nil {
		log.Fatalln(err)
	}
	t.Errorf("%v", ls)
}
