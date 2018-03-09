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

// Package auth contains integration tests for the firebase.google.com/go/auth package.
package auth

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	"golang.org/x/net/context"

	"firebase.google.com/go/auth"
	"firebase.google.com/go/integration/internal"
)

const apiURL = "https://www.googleapis.com/identitytoolkit/v3/relyingparty/verifyCustomToken?key=%s"

var client *auth.Client

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		log.Println("skipping auth integration tests in short mode.")
		os.Exit(0)
	}

	ctx := context.Background()
	app, err := internal.NewTestApp(ctx, nil)
	if err != nil {
		log.Fatalln(err)
	}

	client, err = app.Auth(ctx)
	if err != nil {
		log.Fatalln(err)
	}
	os.Exit(m.Run())
}

func TestCustomToken(t *testing.T) {
	ct, err := client.CustomToken("user1")

	if err != nil {
		t.Fatal(err)
	}
	idt, err := signInWithCustomToken(ct)
	if err != nil {
		t.Fatal(err)
	}

	vt, err := client.VerifyIDToken(idt)
	if err != nil {
		t.Fatal(err)
	}
	if vt.UID != "user1" {
		t.Errorf("UID = %q; want UID = %q", vt.UID, "user1")
	}
	if err = client.DeleteUser(context.Background(), "user1"); err != nil {
		t.Error(err)
	}
}

func TestVerifyIDTokenAndCheckRevoked(t *testing.T) {
	uid := "user_revoked"
	ct, err := client.CustomToken(uid)

	if err != nil {
		t.Fatal(err)
	}
	idt, err := signInWithCustomToken(ct)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	vt, err := client.VerifyIDTokenAndCheckRevoked(ctx, idt)
	if err != nil {
		t.Fatal(err)
	}
	if vt.UID != uid {
		t.Errorf("UID = %q; want UID = %q", vt.UID, uid)
	}
	// The backend stores the validSince property in seconds since the epoch.
	// The issuedAt property of the token is also in seconds. If a token was
	// issued, and then in the same second tokens were revoked, the token will
	// have the same timestamp as the tokensValidAfterMillis, and will therefore
	// not be considered revoked. Hence we wait one second before revoking.
	time.Sleep(time.Second)
	if err = client.RevokeRefreshTokens(ctx, uid); err != nil {
		t.Fatal(err)
	}

	vt, err = client.VerifyIDTokenAndCheckRevoked(ctx, idt)
	we := "ID token has been revoked"
	if vt != nil || err == nil || err.Error() != we {
		t.Errorf("tok, err := VerifyIDTokenAndCheckRevoked(); got (%v, %s) ; want (%v, %v)",
			vt, err, nil, we)
	}

	// Does not return error for revoked token.
	if _, err = client.VerifyIDToken(idt); err != nil {
		t.Errorf("VerifyIDToken(); err = %s; want err = <nil>", err)
	}

	// Sign in after revocation.
	if idt, err = signInWithCustomToken(ct); err != nil {
		t.Fatal(err)
	}

	if _, err = client.VerifyIDTokenAndCheckRevoked(ctx, idt); err != nil {
		t.Errorf("VerifyIDTokenAndCheckRevoked(); err = %s; want err = <nil>", err)
	}

	err = client.DeleteUser(ctx, uid)
	if err != nil {
		t.Error(err)
	}
}

func TestCustomTokenWithClaims(t *testing.T) {
	ct, err := client.CustomTokenWithClaims("user2", map[string]interface{}{
		"premium": true,
		"package": "gold",
	})
	if err != nil {
		t.Fatal(err)
	}

	idt, err := signInWithCustomToken(ct)
	if err != nil {
		t.Fatal(err)
	}

	vt, err := client.VerifyIDToken(idt)
	if err != nil {
		t.Fatal(err)
	}
	if vt.UID != "user2" {
		t.Errorf("UID = %q; want UID = %q", vt.UID, "user2")
	}
	if premium, ok := vt.Claims["premium"].(bool); !ok || !premium {
		t.Errorf("Claims['premium'] = %v; want Claims['premium'] = true", vt.Claims["premium"])
	}
	if pkg, ok := vt.Claims["package"].(string); !ok || pkg != "gold" {
		t.Errorf("Claims['package'] = %v; want Claims['package'] = \"gold\"", vt.Claims["package"])
	}
	if err = client.DeleteUser(context.Background(), "user2"); err != nil {
		t.Error(err)
	}
}

func signInWithCustomToken(token string) (string, error) {
	req, err := json.Marshal(map[string]interface{}{
		"token":             token,
		"returnSecureToken": true,
	})
	if err != nil {
		return "", err
	}

	apiKey, err := internal.APIKey()
	if err != nil {
		return "", err
	}
	resp, err := postRequest(fmt.Sprintf(apiURL, apiKey), req)
	if err != nil {
		return "", err
	}
	var respBody struct {
		IDToken string `json:"idToken"`
	}
	if err := json.Unmarshal(resp, &respBody); err != nil {
		return "", err
	}
	return respBody.IDToken, err
}

func postRequest(url string, req []byte) ([]byte, error) {
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(req))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected http status code: %d", resp.StatusCode)
	}
	return ioutil.ReadAll(resp.Body)
}
