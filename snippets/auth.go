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
	"log"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/auth"
	"golang.org/x/net/context"
	"google.golang.org/api/iterator"
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

// ==================================================================
// https://firebase.google.com/docs/auth/admin/manage-users
// ==================================================================

func getUser(ctx context.Context, app *firebase.App) *auth.UserRecord {
	uid := "some_string_uid"

	// [START get_user_golang]
	// Get an auth client from the firebase.App
	client, err := app.Auth(context.Background())
	if err != nil {
		log.Fatalf("error getting Auth client: %v\n", err)
	}

	u, err := client.GetUser(ctx, uid)
	if err != nil {
		log.Fatalf("error getting user %s: %v\n", uid, err)
	}
	log.Printf("Successfully fetched user data: %v\n", u)
	// [END get_user_golang]
	return u
}

func getUserByEmail(ctx context.Context, client *auth.Client) *auth.UserRecord {
	email := "some@email.com"
	// [START get_user_by_email_golang]
	u, err := client.GetUserByEmail(ctx, email)
	if err != nil {
		log.Fatalf("error getting user by email %s: %v\n", email, err)
	}
	log.Printf("Successfully fetched user data: %v\n", u)
	// [END get_user_by_email_golang]
	return u
}

func getUserByPhone(ctx context.Context, client *auth.Client) *auth.UserRecord {
	phone := "+13214567890"
	// [START get_user_by_phone_golang]
	u, err := client.GetUserByPhoneNumber(ctx, phone)
	if err != nil {
		log.Fatalf("error getting user by phone %s: %v\n", phone, err)
	}
	log.Printf("Successfully fetched user data: %v\n", u)
	// [END get_user_by_phone_golang]
	return u
}

func createUser(ctx context.Context, client *auth.Client) *auth.UserRecord {
	// [START create_user_golang]
	params := (&auth.UserToCreate{}).
		Email("user@example.com").
		EmailVerified(false).
		PhoneNumber("+15555550100").
		Password("secretPassword").
		DisplayName("John Doe").
		PhotoURL("http://www.example.com/12345678/photo.png").
		Disabled(false)
	u, err := client.CreateUser(context.Background(), params)
	if err != nil {
		log.Fatalf("error creating user: %v\n", err)
	}
	log.Printf("Successfully created user: %v\n", u)
	// [END create_user_golang]
	return u
}

func createUserWithUID(ctx context.Context, client *auth.Client) *auth.UserRecord {
	uid := "something"
	// [START create_user_with_uid_golang]
	params := (&auth.UserToCreate{}).
		UID(uid).
		Email("user@example.com").
		PhoneNumber("+15555550100")
	u, err := client.CreateUser(context.Background(), params)
	if err != nil {
		log.Fatalf("error creating user: %v\n", err)
	}
	log.Printf("Successfully created user: %v\n", u)
	// [END create_user_with_uid_golang]
	return u
}

func updateUser(ctx context.Context, client *auth.Client) {
	uid := "d"
	// [START update_user_golang]
	params := (&auth.UserToUpdate{}).
		Email("user@example.com").
		EmailVerified(true).
		PhoneNumber("+15555550100").
		Password("newPassword").
		DisplayName("John Doe").
		PhotoURL("http://www.example.com/12345678/photo.png").
		Disabled(true)
	u, err := client.UpdateUser(context.Background(), uid, params)
	if err != nil {
		log.Fatalf("error updating user: %v\n", err)
	}
	log.Printf("Successfully updated user: %v\n", u)
	// [END update_user_golang]
}

func deleteUser(ctx context.Context, client *auth.Client) {
	uid := "d"
	// [START delete_user_golang]
	err := client.DeleteUser(context.Background(), uid)
	if err != nil {
		log.Fatalf("error deleting user: %v\n", err)
	}
	log.Printf("Successfully deleted user: %s\n", uid)
	// [END delete_user_golang]
}

func customClaimsSet(ctx context.Context, app *firebase.App) {
	uid := "uid"
	// [START set_custom_user_claims_golang]
	// Get an auth client from the firebase.App
	client, err := app.Auth(context.Background())
	if err != nil {
		log.Fatalf("error getting Auth client: %v\n", err)
	}

	// Set admin privilege on the user corresponding to uid.
	claims := map[string]interface{}{"admin": true}
	err = client.SetCustomUserClaims(context.Background(), uid, claims)
	if err != nil {
		log.Fatalf("error setting custom claims %v\n", err)
	}
	// The new custom claims will propagate to the user's ID token the
	// next time a new one is issued.
	// [END set_custom_user_claims_golang]
	// erase all existing custom claims
}

func customClaimsVerify(ctx context.Context, client *auth.Client) {
	idToken := "token"
	// [START verify_custom_claims_golang]
	// Verify the ID token first.
	token, err := client.VerifyIDToken(idToken)
	if err != nil {
		log.Fatal(err)
	}

	claims := token.Claims
	if admin, ok := claims["admin"]; ok {
		if admin.(bool) {
			//Allow access to requested admin resource.
		}
	}
	// [END verify_custom_claims_golang]
}

func customClaimsRead(ctx context.Context, client *auth.Client) {
	uid := "uid"
	// [START read_custom_user_claims_golang]
	// Lookup the user associated with the specified uid.
	user, err := client.GetUser(ctx, uid)
	if err != nil {
		log.Fatal(err)
	}
	// The claims can be accessed on the user record.
	if admin, ok := user.CustomClaims["admin"]; ok {
		if admin.(bool) {
			log.Println(admin)
		}
	}
	// [END read_custom_user_claims_golang]
}

func customClaimsScript(ctx context.Context, client *auth.Client) {
	// [START set_custom_user_claims_script_golang]
	user, err := client.GetUserByEmail(ctx, "user@admin.example.com")
	if err != nil {
		log.Fatal(err)
	}
	// Confirm user is verified
	if user.EmailVerified {
		// Add custom claims for additional privileges.
		// This will be picked up by the user on token refresh or next sign in on new device.
		err := client.SetCustomUserClaims(ctx, user.UID, map[string]interface{}{"admin": true})
		if err != nil {
			log.Fatalf("error setting custom claims %v\n", err)
		}

	}
	// [END set_custom_user_claims_script_golang]
}

func customClaimsIncremental(ctx context.Context, client *auth.Client) {
	// [START set_custom_user_claims_incremental_golang]
	user, err := client.GetUserByEmail(ctx, "user@admin.example.com")
	if err != nil {
		log.Fatal(err)
	}
	// Add incremental custom claim without overwriting existing claims.
	currentCustomClaims := user.CustomClaims
	if currentCustomClaims == nil {
		currentCustomClaims = map[string]interface{}{}
	}

	if _, found := currentCustomClaims["admin"]; found {
		// Add level.
		currentCustomClaims["accessLevel"] = 10
		// Add custom claims for additional privileges.
		err := client.SetCustomUserClaims(ctx, user.UID, currentCustomClaims)
		if err != nil {
			log.Fatalf("error setting custom claims %v\n", err)
		}

	}
	// [END set_custom_user_claims_incremental_golang]
}

func listUsers(ctx context.Context, client *auth.Client) {
	// [START list_all_users_golang]
	// Note, behind the scenes, the Users() iterator will retrive 1000 Users at a time through the API
	iter := client.Users(context.Background(), "")
	for {
		user, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalf("error listing users: %s\n", err)
		}
		log.Printf("read user user: %v\n", user)
	}

	// Iterating by pages 100 users at a time.
	// Note that using both the Next() function on an iterator and the NextPage()
	// on a Pager wrapping that same iterator will result in an error.
	pager := iterator.NewPager(client.Users(context.Background(), ""), 100, "")
	for {
		var users []*auth.ExportedUserRecord
		nextPageToken, err := pager.NextPage(&users)
		if err != nil {
			log.Fatalf("paging error %v\n", err)
		}
		for _, u := range users {
			log.Printf("read user user: %v\n", u)
		}
		if nextPageToken == "" {
			break
		}
	}
	// [END list_all_users_golang]
}
