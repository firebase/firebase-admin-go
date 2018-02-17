// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package snippets

import (
	"context"
	"log"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/auth"
)

// ==================================================================
// https://firebase.google.com/docs/auth/admin/create-custom-tokens
// ==================================================================

func createCustomToken(app *firebase.App) string {
	// [START create_custom_token_golang]
	client, err := app.Auth(context.Background())
	if err != nil {
		log.Fatalf("error getting Auth client: %v\n", err)
	}

	token, err := client.CustomToken("some-uid")
	if err != nil {
		log.Fatalf("error minting custom token: %v\n", err)
	}

	log.Printf("Got custom token: %v\n", token)
	// [END create_custom_token_golang]

	return token
}

func createCustomTokenWithClaims(app *firebase.App) string {
	// [START create_custom_token_claims_golang]
	client, err := app.Auth(context.Background())
	if err != nil {
		log.Fatalf("error getting Auth client: %v\n", err)
	}

	claims := map[string]interface{}{
		"premiumAccount": true,
	}

	token, err := client.CustomTokenWithClaims("some-uid", claims)
	if err != nil {
		log.Fatalf("error minting custom token: %v\n", err)
	}

	log.Printf("Got custom token: %v\n", token)
	// [END create_custom_token_claims_golang]

	return token
}

// ==================================================================
// https://firebase.google.com/docs/auth/admin/verify-id-tokens
// ==================================================================

func verifyIDToken(app *firebase.App, idToken string) *auth.Token {
	// [START verify_id_token_golang]
	client, err := app.Auth(context.Background())
	if err != nil {
		log.Fatalf("error getting Auth client: %v\n", err)
	}

	token, err := client.VerifyIDToken(idToken)
	if err != nil {
		log.Fatalf("error verifying ID token: %v\n", err)
	}

	log.Printf("Verified ID token: %v\n", token)
	// [END verify_id_token_golang]

	return token
}

// ==================================================================
// https://firebase.google.com/docs/auth/admin/manage-sessions
// ==================================================================

func revokeRefreshTokens(app *firebase.App, uid string) {

	// [START revoke_tokens_golang]
	ctx := context.Background()
	client, err := app.Auth(ctx)
	if err != nil {
		log.Fatalf("error getting Auth client: %v\n", err)
	}
	if err := client.RevokeRefreshTokens(ctx, uid); err != nil {
		log.Fatalf("error revoking tokens for user: %v, %v\n", uid, err)
	}
	// accessing the user's TokenValidAfter
	u, err := client.GetUser(ctx, uid)
	if err != nil {
		log.Fatalf("error getting user %s: %v\n", uid, err)
	}
	timestamp := u.TokensValidAfterMillis / 1000
	log.Printf("the refresh tokens were revoked at: %d (UTC seconds) ", timestamp)
	// [END revoke_tokens_golang]
}

func verifyIDTokenAndCheckRevoked(app *firebase.App, idToken string) *auth.Token {
	ctx := context.Background()
	// [START verify_id_token_and_check_revoked_golang]
	client, err := app.Auth(ctx)
	if err != nil {
		log.Fatalf("error getting Auth client: %v\n", err)
	}
	token, err := client.VerifyIDTokenAndCheckRevoked(ctx, idToken)
	if err != nil {
		if err.Error() == "ID token has been revoked" {
			// Token is revoked. Inform the user to reauthenticate or signOut() the user.
		} else {
			// Token is invalid
		}
	}
	log.Printf("Verified ID token: %v\n", token)
	// [END verify_id_token_and_check_revoked_golang]

	return token
}

func tokenMain() {
	app := initializeAppWithServiceAccount()
	_ = createCustomToken(app)
	_ = createCustomTokenWithClaims(app)
	_ = verifyIDToken(app, "some-token")
	revokeRefreshTokens(app, "tok")
	_ = verifyIDTokenAndCheckRevoked(app, "tok")
}
