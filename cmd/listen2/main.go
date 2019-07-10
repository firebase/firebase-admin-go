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

package main

import (
	"fmt"
	"log"
	"os"

	"golang.org/x/net/context"
	"google.golang.org/api/option"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/db"
)

// Key is a json-serializable type.
type Key struct {
	Key1 string `json:"key1"`
}

func main() {

	// opt := option.WithCredentialsFile("c:/users/username/.firebase/firebase.json") // Windows
	//
	opt := option.WithCredentialsFile("/home/username/.firebase/firebase.json") // Linux, edit 1.

	config := &firebase.Config{
		DatabaseURL: "https://databaseName.firebaseio.com", // edit 2.
	}

	app, err := firebase.NewApp(context.Background(), config, opt)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// DatabaseWithURL
	client, err := app.Database(ctx)

	if err != nil {
		log.Fatal(err)
	}

	// https://firebase.google.com/docs/reference/js/firebase.database.Reference.html#key
	//
	// key = The last part of the Reference's path.

	testpath := "user1/path1"
	ref := client.NewRef(testpath)

	args := os.Args
	if len(args) > 1 {
		triggerEvent(ctx, client, testpath, args[1])
		return // exit app
	}

	// SnapshotIterator
	iter, err := ref.Listen(ctx)
	if err != nil {
		fmt.Printf(" Error: failed to create Listener %v\n", err)
		return // exit app
	}

	fmt.Printf("Initial snapshots:\n")

	fmt.Printf("1st Listener | Ref Path: %s | iter.Snapshot = %v\n", ref.Path, iter.Snapshot)
	fmt.Printf("             | Ref Key: %s \n", ref.Key)

	defer iter.Stop()

	var key Key

	go func() {
		for {

			if iter.Done() {
				break
			}

			event, err := iter.Next()

			if err != nil {
				break
			}

			err = event.Unmarshal(&key)

			if err != nil {
				fmt.Printf("1st Listener | Error: Unmarshal %v\n", err)
			} else {
				fmt.Printf("1st Listener | Ref Path: %s | event.Path %s | event.Unmarshal(&key) key.Key1 = %s\n", ref.Path, event.Path, key.Key1)
				fmt.Printf("1st Listener | Ref Path: %s | event.Path %s | event.Unmarshal(&key) key = %v\n", ref.Path, event.Path, key)
			}

			fmt.Printf("1st Listener | Ref Path: %s | event.Path %s | event.Snapshot() = %v\n", ref.Path, event.Path, event.Snapshot())
			fmt.Printf("\n")
		}
	}()

	// 2nd listener
	testpath2 := "user1/path1/path2"
	ref2 := client.NewRef(testpath2)

	iter2, err := ref2.Listen(ctx)
	if err != nil {
		fmt.Printf(" Error: failed to create Listener %v\n", err)
		return
	}

	fmt.Printf("2nd Listener | Ref Path: %s | iter.Snapshot = %v\n", ref2.Path, iter2.Snapshot)
	fmt.Printf("             | Ref Key: %s \n", ref2.Key)

	defer iter2.Stop()

	go func() {
		for {

			if iter2.Done() {
				break
			}

			event, err := iter2.Next()

			if err != nil {
				break
			}

			fmt.Printf("2nd Listener | Ref Path: %s | event.Path %s | event.Snapshot() = %v\n", ref2.Path, event.Path, event.Snapshot())
			fmt.Printf("\n")
		}
	}()

	fmt.Printf("\n >>> open a new separate command line terminal, to trigger events, run: go run . anyvalue\n")
	fmt.Printf("\n >>> OR edit value of any key from %s in firebase console to trigger events\n\n", testpath)
	fmt.Printf("\n >>> press <enter> to stop 1st Listener and close http connection\n\n")
	fmt.Printf("Waiting for events...\n\n")

	fmt.Scanln()
	iter.Stop()

	fmt.Printf("\n >>> press <enter> to stop 2nd Listener and close http connection\n\n")
	fmt.Scanln()
	iter2.Stop()

	fmt.Printf("\n >>> press <enter> to exit app\n\n\n")
	fmt.Scanln()
}

func triggerEvent(ctx context.Context, client *db.Client, testpath string, val string) {

	var key Key

	key.Key1 = val

	ref := client.NewRef(testpath + "/path2/path3")

	if err := ref.Set(ctx, key); err != nil {
		log.Fatal(err)
	} else {
		fmt.Printf("OK - Set %s to key.Key1=%v\n", testpath+"/path2/path3", val)
	}
}
