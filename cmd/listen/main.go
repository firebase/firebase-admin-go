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

func main() {

	// opt := option.WithCredentialsFile("c:/users/username/.firebase/firebase.json") // Windows
	//
	opt := option.WithCredentialsFile("/home/username/.firebase/firebase.json") // Linux, edit 1.

	config := &firebase.Config{
		DatabaseURL: "https://mydb.firebaseio.com", // edit 2.
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
		return
	}

	fmt.Printf("client app | Ref Path: %s | iter.Snapshot = %v\n", ref.Path, iter.Snapshot)
	fmt.Printf("           | Ref Key: %s \n", ref.Key)

	defer iter.Stop()

	go func() {
		for {

			if iter.Done() {
				break
			}

			event, err := iter.Next()

			if err != nil {
				break
			}

			fmt.Printf("client app | Ref Path: %s | event.Path %s | event.Snapshot() = %v\n", ref.Path, event.Path, event.Snapshot())
			fmt.Printf("\n")
		}
	}()

	fmt.Printf("\n >>> edit value of any key from %s in firebase console to trigger event\n\n", testpath)
	fmt.Printf("\n >>> press <enter> to close http connection\n\n")
	fmt.Printf("Waiting for events...\n\n")

	fmt.Scanln()
	iter.Stop()

	fmt.Printf("\n >>> press <enter> to exit app\n\n\n")
	fmt.Scanln()
}

func triggerEvent(ctx context.Context, client *db.Client, testpath string, val string) {

	ref := client.NewRef(testpath + "/key1")

	if err := ref.Set(ctx, val); err != nil {
		log.Fatal(err)
	} else {
		fmt.Printf("OK - Set %s to %v\n", testpath+"/key1", val)
	}
}
