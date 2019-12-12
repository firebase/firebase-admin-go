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
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/auth"
	"firebase.google.com/go/auth/hash"
	"google.golang.org/api/iterator"
)

// ==================================================================
// https://firebase.google.com/docs/auth/admin/create-custom-tokens
// ==================================================================

func createCustomToken(ctx context.Context, app *firebase.App) string {
	// [START create_custom_token_golang]
	client, err := app.Auth(context.Background())
	if err != nil {
		log.Fatalf("error getting Auth client: %v\n", err)
	}

	token, err := client.CustomToken(ctx, "some-uid")
	if err != nil {
		log.Fatalf("error minting custom token: %v\n", err)
	}

	log.Printf("Got custom token: %v\n", token)
	// [END create_custom_token_golang]

	return token
}

func createCustomTokenWithClaims(ctx context.Context, app *firebase.App) string {
	// [START create_custom_token_claims_golang]
	client, err := app.Auth(context.Background())
	if err != nil {
		log.Fatalf("error getting Auth client: %v\n", err)
	}

	claims := map[string]interface{}{
		"premiumAccount": true,
	}

	token, err := client.CustomTokenWithClaims(ctx, "some-uid", claims)
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

func verifyIDToken(ctx context.Context, app *firebase.App, idToken string) *auth.Token {
	// [START verify_id_token_golang]
	client, err := app.Auth(ctx)
	if err != nil {
		log.Fatalf("error getting Auth client: %v\n", err)
	}

	token, err := client.VerifyIDToken(ctx, idToken)
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

func revokeRefreshTokens(ctx context.Context, app *firebase.App, uid string) {
	// [START revoke_tokens_golang]
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

func verifyIDTokenAndCheckRevoked(ctx context.Context, app *firebase.App, idToken string) *auth.Token {
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
	client, err := app.Auth(ctx)
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
	u, err := client.CreateUser(ctx, params)
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
	u, err := client.CreateUser(ctx, params)
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
	u, err := client.UpdateUser(ctx, uid, params)
	if err != nil {
		log.Fatalf("error updating user: %v\n", err)
	}
	log.Printf("Successfully updated user: %v\n", u)
	// [END update_user_golang]
}

func deleteUser(ctx context.Context, client *auth.Client) {
	uid := "d"
	// [START delete_user_golang]
	err := client.DeleteUser(ctx, uid)
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
	client, err := app.Auth(ctx)
	if err != nil {
		log.Fatalf("error getting Auth client: %v\n", err)
	}

	// Set admin privilege on the user corresponding to uid.
	claims := map[string]interface{}{"admin": true}
	err = client.SetCustomUserClaims(ctx, uid, claims)
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
	token, err := client.VerifyIDToken(ctx, idToken)
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
	iter := client.Users(ctx, "")
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
	pager := iterator.NewPager(client.Users(ctx, ""), 100, "")
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

func importUsers(ctx context.Context, app *firebase.App) {
	// [START build_user_list]
	// Up to 1000 users can be imported at once.
	var users []*auth.UserToImport
	users = append(users, (&auth.UserToImport{}).
		UID("uid1").
		Email("user1@example.com").
		PasswordHash([]byte("passwordHash1")).
		PasswordSalt([]byte("salt1")))
	users = append(users, (&auth.UserToImport{}).
		UID("uid2").
		Email("user2@example.com").
		PasswordHash([]byte("passwordHash2")).
		PasswordSalt([]byte("salt2")))
	// [END build_user_list]

	// [START import_users]
	client, err := app.Auth(ctx)
	if err != nil {
		log.Fatalln("Error initializing Auth client", err)
	}

	h := hash.HMACSHA256{
		Key: []byte("secretKey"),
	}
	result, err := client.ImportUsers(ctx, users, auth.WithHash(h))
	if err != nil {
		log.Fatalln("Unrecoverable error prevented the operation from running", err)
	}

	log.Printf("Successfully imported %d users\n", result.SuccessCount)
	log.Printf("Failed to import %d users\n", result.FailureCount)
	for _, e := range result.Errors {
		log.Printf("Failed to import user at index: %d due to error: %s\n", e.Index, e.Reason)
	}
	// [END import_users]
}

func importWithHMAC(ctx context.Context, client *auth.Client) {
	// [START import_with_hmac]
	users := []*auth.UserToImport{
		(&auth.UserToImport{}).
			UID("some-uid").
			Email("user@example.com").
			PasswordHash([]byte("password-hash")).
			PasswordSalt([]byte("salt")),
	}
	h := hash.HMACSHA256{
		Key: []byte("secret"),
	}
	result, err := client.ImportUsers(ctx, users, auth.WithHash(h))
	if err != nil {
		log.Fatalln("Error importing users", err)
	}
	for _, e := range result.Errors {
		log.Println("Failed to import user", e.Reason)
	}
	// [END import_with_hmac]
}

func importWithPBKDF(ctx context.Context, client *auth.Client) {
	// [START import_with_pbkdf]
	users := []*auth.UserToImport{
		(&auth.UserToImport{}).
			UID("some-uid").
			Email("user@example.com").
			PasswordHash([]byte("password-hash")).
			PasswordSalt([]byte("salt")),
	}
	h := hash.PBKDF2SHA256{
		Rounds: 100000,
	}
	result, err := client.ImportUsers(ctx, users, auth.WithHash(h))
	if err != nil {
		log.Fatalln("Error importing users", err)
	}
	for _, e := range result.Errors {
		log.Println("Failed to import user", e.Reason)
	}
	// [END import_with_pbkdf]
}

func importWithStandardScrypt(ctx context.Context, client *auth.Client) {
	// [START import_with_standard_scrypt]
	users := []*auth.UserToImport{
		(&auth.UserToImport{}).
			UID("some-uid").
			Email("user@example.com").
			PasswordHash([]byte("password-hash")).
			PasswordSalt([]byte("salt")),
	}
	h := hash.StandardScrypt{
		MemoryCost:       1024,
		Parallelization:  16,
		BlockSize:        8,
		DerivedKeyLength: 64,
	}
	result, err := client.ImportUsers(ctx, users, auth.WithHash(h))
	if err != nil {
		log.Fatalln("Error importing users", err)
	}
	for _, e := range result.Errors {
		log.Println("Failed to import user", e.Reason)
	}
	// [END import_with_standard_scrypt]
}

func importWithBcrypt(ctx context.Context, client *auth.Client) {
	// [START import_with_bcrypt]
	users := []*auth.UserToImport{
		(&auth.UserToImport{}).
			UID("some-uid").
			Email("user@example.com").
			PasswordHash([]byte("password-hash")).
			PasswordSalt([]byte("salt")),
	}
	h := hash.Bcrypt{}
	result, err := client.ImportUsers(ctx, users, auth.WithHash(h))
	if err != nil {
		log.Fatalln("Error importing users", err)
	}
	for _, e := range result.Errors {
		log.Println("Failed to import user", e.Reason)
	}
	// [END import_with_bcrypt]
}

func importWithScrypt(ctx context.Context, client *auth.Client) {
	// [START import_with_scrypt]
	users := []*auth.UserToImport{
		(&auth.UserToImport{}).
			UID("some-uid").
			Email("user@example.com").
			PasswordHash([]byte("password-hash")).
			PasswordSalt([]byte("salt")),
	}
	b64decode := func(s string) []byte {
		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			log.Fatalln("Failed to decode string", err)
		}
		return b
	}

	// All the parameters below can be obtained from the Firebase Console's "Users"
	// section. Base64 encoded parameters must be decoded into raw bytes.
	h := hash.Scrypt{
		Key:           b64decode("base64-secret"),
		SaltSeparator: b64decode("base64-salt-separator"),
		Rounds:        8,
		MemoryCost:    14,
	}
	result, err := client.ImportUsers(ctx, users, auth.WithHash(h))
	if err != nil {
		log.Fatalln("Error importing users", err)
	}
	for _, e := range result.Errors {
		log.Println("Failed to import user", e.Reason)
	}
	// [END import_with_scrypt]
}

func importWithoutPassword(ctx context.Context, client *auth.Client) {
	// [START import_without_password]
	users := []*auth.UserToImport{
		(&auth.UserToImport{}).
			UID("some-uid").
			DisplayName("John Doe").
			Email("johndoe@gmail.com").
			PhotoURL("http://www.example.com/12345678/photo.png").
			EmailVerified(true).
			PhoneNumber("+11234567890").
			CustomClaims(map[string]interface{}{"admin": true}). // set this user as admin
			ProviderData([]*auth.UserProvider{                   // user with Google provider
				{
					UID:         "google-uid",
					Email:       "johndoe@gmail.com",
					DisplayName: "John Doe",
					PhotoURL:    "http://www.example.com/12345678/photo.png",
					ProviderID:  "google.com",
				},
			}),
	}
	result, err := client.ImportUsers(ctx, users)
	if err != nil {
		log.Fatalln("Error importing users", err)
	}
	for _, e := range result.Errors {
		log.Println("Failed to import user", e.Reason)
	}
	// [END import_without_password]
}

func loginHandler(client *auth.Client) http.HandlerFunc {
	// [START session_login]
	return func(w http.ResponseWriter, r *http.Request) {
		// Get the ID token sent by the client
		defer r.Body.Close()
		idToken, err := getIDTokenFromBody(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Set session expiration to 5 days.
		expiresIn := time.Hour * 24 * 5

		// Create the session cookie. This will also verify the ID token in the process.
		// The session cookie will have the same claims as the ID token.
		// To only allow session cookie setting on recent sign-in, auth_time in ID token
		// can be checked to ensure user was recently signed in before creating a session cookie.
		cookie, err := client.SessionCookie(r.Context(), idToken, expiresIn)
		if err != nil {
			http.Error(w, "Failed to create a session cookie", http.StatusInternalServerError)
			return
		}

		// Set cookie policy for session cookie.
		http.SetCookie(w, &http.Cookie{
			Name:     "session",
			Value:    cookie,
			MaxAge:   int(expiresIn.Seconds()),
			HttpOnly: true,
			Secure:   true,
		})
		w.Write([]byte(`{"status": "success"}`))
	}
	// [END session_login]
}

func loginWithAuthTimeCheckHandler(client *auth.Client) http.HandlerFunc {
	// [START check_auth_time]
	return func(w http.ResponseWriter, r *http.Request) {
		// Get the ID token sent by the client
		defer r.Body.Close()
		idToken, err := getIDTokenFromBody(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		decoded, err := client.VerifyIDToken(r.Context(), idToken)
		if err != nil {
			http.Error(w, "Invalid ID token", http.StatusUnauthorized)
			return
		}
		// Return error if the sign-in is older than 5 minutes.
		if time.Now().Unix()-decoded.Claims["auth_time"].(int64) > 5*60 {
			http.Error(w, "Recent sign-in required", http.StatusUnauthorized)
			return
		}

		expiresIn := time.Hour * 24 * 5
		cookie, err := client.SessionCookie(r.Context(), idToken, expiresIn)
		if err != nil {
			http.Error(w, "Failed to create a session cookie", http.StatusInternalServerError)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     "session",
			Value:    cookie,
			MaxAge:   int(expiresIn.Seconds()),
			HttpOnly: true,
			Secure:   true,
		})
		w.Write([]byte(`{"status": "success"}`))
	}
	// [END check_auth_time]
}

func userProfileHandler(client *auth.Client) http.HandlerFunc {
	serveContentForUser := func(w http.ResponseWriter, r *http.Request, claims *auth.Token) {
		log.Println("Serving content")
	}

	// [START session_verify]
	return func(w http.ResponseWriter, r *http.Request) {
		// Get the ID token sent by the client
		cookie, err := r.Cookie("session")
		if err != nil {
			// Session cookie is unavailable. Force user to login.
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// Verify the session cookie. In this case an additional check is added to detect
		// if the user's Firebase session was revoked, user deleted/disabled, etc.
		decoded, err := client.VerifySessionCookieAndCheckRevoked(r.Context(), cookie.Value)
		if err != nil {
			// Session cookie is invalid. Force user to login.
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		serveContentForUser(w, r, decoded)
	}
	// [END session_verify]
}

func adminUserHandler(client *auth.Client) http.HandlerFunc {
	serveContentForAdmin := func(w http.ResponseWriter, r *http.Request, claims *auth.Token) {
		log.Println("Serving content")
	}

	// [START session_verify_with_permission_check]
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err != nil {
			// Session cookie is unavailable. Force user to login.
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		decoded, err := client.VerifySessionCookieAndCheckRevoked(r.Context(), cookie.Value)
		if err != nil {
			// Session cookie is invalid. Force user to login.
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// Check custom claims to confirm user is an admin.
		if decoded.Claims["admin"] != true {
			http.Error(w, "Insufficient permissions", http.StatusUnauthorized)
			return
		}

		serveContentForAdmin(w, r, decoded)
	}
	// [END session_verify_with_permission_check]
}

func sessionLogoutHandler() http.HandlerFunc {
	// [START session_clear]
	return func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:   "session",
			Value:  "",
			MaxAge: 0,
		})
		http.Redirect(w, r, "/login", http.StatusFound)
	}
	// [END session_clear]
}

func sessionLogoutHandlerWithRevocation(client *auth.Client) http.HandlerFunc {
	// [START session_clear_and_revoke]
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err != nil {
			// Session cookie is unavailable. Force user to login.
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		decoded, err := client.VerifySessionCookie(r.Context(), cookie.Value)
		if err != nil {
			// Session cookie is invalid. Force user to login.
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		if err := client.RevokeRefreshTokens(r.Context(), decoded.UID); err != nil {
			http.Error(w, "Failed to revoke refresh token", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:   "session",
			Value:  "",
			MaxAge: 0,
		})
		http.Redirect(w, r, "/login", http.StatusFound)
	}
	// [END session_clear_and_revoke]
}

func getIDTokenFromBody(r *http.Request) (string, error) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return "", err
	}

	var parsedBody struct {
		IDToken string `json:"idToken"`
	}
	err = json.Unmarshal(b, &parsedBody)
	return parsedBody.IDToken, err
}

func newActionCodeSettings() *auth.ActionCodeSettings {
	// [START init_action_code_settings]
	actionCodeSettings := &auth.ActionCodeSettings{
		URL:                   "https://www.example.com/checkout?cartId=1234",
		HandleCodeInApp:       true,
		IOSBundleID:           "com.example.ios",
		AndroidPackageName:    "com.example.android",
		AndroidInstallApp:     true,
		AndroidMinimumVersion: "12",
		DynamicLinkDomain:     "coolapp.page.link",
	}
	// [END init_action_code_settings]
	return actionCodeSettings
}

func generatePasswordResetLink(ctx context.Context, client *auth.Client) {
	actionCodeSettings := newActionCodeSettings()
	displayName := "Example User"
	// [START password_reset_link]
	email := "user@example.com"
	link, err := client.PasswordResetLinkWithSettings(ctx, email, actionCodeSettings)
	if err != nil {
		log.Fatalf("error generating email link: %v\n", err)
	}

	// Construct password reset template, embed the link and send
	// using custom SMTP server.
	sendCustomEmail(email, displayName, link)
	// [END password_reset_link]
}

func generateEmailVerificationLink(ctx context.Context, client *auth.Client) {
	actionCodeSettings := newActionCodeSettings()
	displayName := "Example User"
	// [START email_verification_link]
	email := "user@example.com"
	link, err := client.EmailVerificationLinkWithSettings(ctx, email, actionCodeSettings)
	if err != nil {
		log.Fatalf("error generating email link: %v\n", err)
	}

	// Construct email verification template, embed the link and send
	// using custom SMTP server.
	sendCustomEmail(email, displayName, link)
	// [END email_verification_link]
}

func generateEmailSignInLink(ctx context.Context, client *auth.Client) {
	actionCodeSettings := newActionCodeSettings()
	displayName := "Example User"
	// [START sign_in_with_email_link]
	email := "user@example.com"
	link, err := client.EmailSignInLink(ctx, email, actionCodeSettings)
	if err != nil {
		log.Fatalf("error generating email link: %v\n", err)
	}

	// Construct sign-in with email link template, embed the link and send
	// using custom SMTP server.
	sendCustomEmail(email, displayName, link)
	// [END sign_in_with_email_link]
}

// Place holder function to make the compiler happy. This is referenced by all email action
// link snippets.
func sendCustomEmail(email, displayName, link string) {}

// =====================================================================================
// https://cloud.google.com/identity-platform/docs/managing-providers-programmatically
// =====================================================================================

func createSAMLProviderConfig(ctx context.Context, client *auth.Client) {
	// [START create_saml_provider]
	newConfig := (&auth.SAMLProviderConfigToCreate{}).
		DisplayName("SAML provider name").
		Enabled(true).
		ID("saml.myProvider").
		IDPEntityID("IDP_ENTITY_ID").
		SSOURL("https://example.com/saml/sso/1234/").
		X509Certificates([]string{
			"-----BEGIN CERTIFICATE-----\nCERT1...\n-----END CERTIFICATE-----",
			"-----BEGIN CERTIFICATE-----\nCERT2...\n-----END CERTIFICATE-----",
		}).
		RPEntityID("RP_ENTITY_ID").
		CallbackURL("https://project-id.firebaseapp.com/__/auth/handler")
	saml, err := client.CreateSAMLProviderConfig(ctx, newConfig)
	if err != nil {
		log.Fatalf("error creating SAML provider: %v\n", err)
	}

	log.Printf("Created new SAML provider: %s", saml.ID)
	// [END create_saml_provider]
}

func updateSAMLProviderConfig(ctx context.Context, client *auth.Client) {
	// [START update_saml_provider]
	updatedConfig := (&auth.SAMLProviderConfigToUpdate{}).
		X509Certificates([]string{
			"-----BEGIN CERTIFICATE-----\nCERT2...\n-----END CERTIFICATE-----",
			"-----BEGIN CERTIFICATE-----\nCERT3...\n-----END CERTIFICATE-----",
		})
	saml, err := client.UpdateSAMLProviderConfig(ctx, "saml.myProvider", updatedConfig)
	if err != nil {
		log.Fatalf("error updating SAML provider: %v\n", err)
	}

	log.Printf("Updated SAML provider: %s", saml.ID)
	// [END update_saml_provider]
}

func getSAMLProviderConfig(ctx context.Context, client *auth.Client) {
	// [START get_saml_provider]
	saml, err := client.SAMLProviderConfig(ctx, "saml.myProvider")
	if err != nil {
		log.Fatalf("error retrieving SAML provider: %v\n", err)
	}

	log.Printf("%s %t", saml.DisplayName, saml.Enabled)
	// [END get_saml_provider]
}

func deleteSAMLProviderConfig(ctx context.Context, client *auth.Client) {
	// [START delete_saml_provider]
	if err := client.DeleteSAMLProviderConfig(ctx, "saml.myProvider"); err != nil {
		log.Fatalf("error deleting SAML provider: %v\n", err)
	}
	// [END delete_saml_provider]
}

func listSAMLProviderConfigs(ctx context.Context, client *auth.Client) {
	// [START list_saml_providers]
	iter := client.SAMLProviderConfigs(ctx, "nextPageToken")
	for {
		saml, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalf("error retrieving SAML providers: %v\n", err)
		}

		log.Printf("%s\n", saml.ID)
	}
	// [END list_saml_providers]
}

func createOIDCProviderConfig(ctx context.Context, client *auth.Client) {
	// [START create_oidc_provider]
	newConfig := (&auth.OIDCProviderConfigToCreate{}).
		DisplayName("OIDC provider name").
		Enabled(true).
		ID("oidc.myProvider").
		ClientID("CLIENT_ID2").
		Issuer("https://oidc.com/CLIENT_ID2")
	oidc, err := client.CreateOIDCProviderConfig(ctx, newConfig)
	if err != nil {
		log.Fatalf("error creating OIDC provider: %v\n", err)
	}

	log.Printf("Created new OIDC provider: %s", oidc.ID)
	// [END create_oidc_provider]
}

func updateOIDCProviderConfig(ctx context.Context, client *auth.Client) {
	// [START update_oidc_provider]
	updatedConfig := (&auth.OIDCProviderConfigToUpdate{}).
		DisplayName("OIDC provider name").
		Enabled(true).
		ClientID("CLIENT_ID").
		Issuer("https://oidc.com")
	oidc, err := client.UpdateOIDCProviderConfig(ctx, "oidc.myProvider", updatedConfig)
	if err != nil {
		log.Fatalf("error updating OIDC provider: %v\n", err)
	}

	log.Printf("Updated OIDC provider: %s", oidc.ID)
	// [END update_oidc_provider]
}

func getOIDCProviderConfig(ctx context.Context, client *auth.Client) {
	// [START get_oidc_provider]
	oidc, err := client.OIDCProviderConfig(ctx, "oidc.myProvider")
	if err != nil {
		log.Fatalf("error retrieving OIDC provider: %v\n", err)
	}

	log.Printf("%s %t", oidc.DisplayName, oidc.Enabled)
	// [END get_oidc_provider]
}

func deleteOIDCProviderConfig(ctx context.Context, client *auth.Client) {
	// [START delete_oidc_provider]
	if err := client.DeleteOIDCProviderConfig(ctx, "oidc.myProvider"); err != nil {
		log.Fatalf("error deleting OIDC provider: %v\n", err)
	}
	// [END delete_oidc_provider]
}

func listOIDCProviderConfigs(ctx context.Context, client *auth.Client) {
	// [START list_oidc_providers]
	iter := client.OIDCProviderConfigs(ctx, "nextPageToken")
	for {
		oidc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalf("error retrieving OIDC providers: %v\n", err)
		}

		log.Printf("%s\n", oidc.ID)
	}
	// [END list_oidc_providers]
}

// ================================================================================
// https://cloud.google.com/identity-platform/docs/multi-tenancy-managing-tenants
// =================================================================================

func getTenantClient(ctx context.Context, app *firebase.App, tenantID string) *auth.TenantClient {
	// [START get_tenant_client]
	client, err := app.Auth(ctx)
	if err != nil {
		log.Fatalf("error initializing auth client: %v\n", err)
	}

	tm := client.TenantManager
	tenantClient, err := tm.AuthForTenant(tenantID)
	if err != nil {
		log.Fatalf("error initializing tenant-aware auth client: %v\n", err)
	}

	// [END get_tenant_client]
	return tenantClient
}

func getTenant(ctx context.Context, client *auth.Client, tenantID string) {
	// [START get_tenant]
	tenant, err := client.TenantManager.Tenant(ctx, tenantID)
	if err != nil {
		log.Fatalf("error retrieving tenant: %v\n", err)
	}

	log.Printf("Retreieved tenant: %q\n", tenant.ID)
	// [END get_tenant]
}

func createTenant(ctx context.Context, client *auth.Client) {
	// [START create_tenant]
	config := (&auth.TenantToCreate{}).
		DisplayName("myTenant1").
		EnableEmailLinkSignIn(true).
		AllowPasswordSignUp(true)
	tenant, err := client.TenantManager.CreateTenant(ctx, config)
	if err != nil {
		log.Fatalf("error creating tenant: %v\n", err)
	}

	log.Printf("Created tenant: %q\n", tenant.ID)
	// [END create_tenant]
}

func updateTenant(ctx context.Context, client *auth.Client, tenantID string) {
	// [START update_tenant]
	config := (&auth.TenantToUpdate{}).
		DisplayName("updatedName").
		AllowPasswordSignUp(false) // Disable email provider
	tenant, err := client.TenantManager.UpdateTenant(ctx, tenantID, config)
	if err != nil {
		log.Fatalf("error updating tenant: %v\n", err)
	}

	log.Printf("Updated tenant: %q\n", tenant.ID)
	// [END update_tenant]
}

func deleteTenant(ctx context.Context, client *auth.Client, tenantID string) {
	// [START delete_tenant]
	if err := client.TenantManager.DeleteTenant(ctx, tenantID); err != nil {
		log.Fatalf("error deleting tenant: %v\n", tenantID)
	}
	// [END delete_tenant]
}

func listTenants(ctx context.Context, client *auth.Client) {
	// [START list_tenants]
	iter := client.TenantManager.Tenants(ctx, "")
	for {
		tenant, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalf("error listing tenants: %v\n", err)
		}

		log.Printf("Retrieved tenant: %q\n", tenant.ID)
	}
	// [END list_tenants]
}

func createProviderTenant(ctx context.Context, client *auth.Client) {
	// [START get_tenant_client_short]
	tenantClient, err := client.TenantManager.AuthForTenant("TENANT-ID")
	if err != nil {
		log.Fatalf("error initializing tenant client: %v\n", err)
	}
	// [END get_tenant_client_short]

	// [START create_saml_provider_tenant]
	newConfig := (&auth.SAMLProviderConfigToCreate{}).
		DisplayName("SAML provider name").
		Enabled(true).
		ID("saml.myProvider").
		IDPEntityID("IDP_ENTITY_ID").
		SSOURL("https://example.com/saml/sso/1234/").
		X509Certificates([]string{
			"-----BEGIN CERTIFICATE-----\nCERT1...\n-----END CERTIFICATE-----",
			"-----BEGIN CERTIFICATE-----\nCERT2...\n-----END CERTIFICATE-----",
		}).
		RPEntityID("RP_ENTITY_ID").
		// Using the default callback URL.
		CallbackURL("https://project-id.firebaseapp.com/__/auth/handler")
	saml, err := tenantClient.CreateSAMLProviderConfig(ctx, newConfig)
	if err != nil {
		log.Fatalf("error creating SAML provider: %v\n", err)
	}

	log.Printf("Created new SAML provider: %s", saml.ID)
	// [END create_saml_provider_tenant]
}

func updateProviderTenant(ctx context.Context, tenantClient *auth.TenantClient) {
	// [START update_saml_provider_tenant]
	updatedConfig := (&auth.SAMLProviderConfigToUpdate{}).
		X509Certificates([]string{
			"-----BEGIN CERTIFICATE-----\nCERT2...\n-----END CERTIFICATE-----",
			"-----BEGIN CERTIFICATE-----\nCERT3...\n-----END CERTIFICATE-----",
		})
	saml, err := tenantClient.UpdateSAMLProviderConfig(ctx, "saml.myProvider", updatedConfig)
	if err != nil {
		log.Fatalf("error updating SAML provider: %v\n", err)
	}

	log.Printf("Updated SAML provider: %s", saml.ID)
	// [END update_saml_provider_tenant]
}

func getProviderTenant(ctx context.Context, tenantClient *auth.TenantClient) {
	// [START get_saml_provider_tenant]
	saml, err := tenantClient.SAMLProviderConfig(ctx, "saml.myProvider")
	if err != nil {
		log.Fatalf("error retrieving SAML provider: %v\n", err)
	}

	// Get display name and whether it is enabled.
	log.Printf("%s %t", saml.DisplayName, saml.Enabled)
	// [END get_saml_provider_tenant]
}

func listProviderConfigsTenant(ctx context.Context, tenantClient *auth.TenantClient) {
	// [START list_saml_providers_tenant]
	iter := tenantClient.SAMLProviderConfigs(ctx, "nextPageToken")
	for {
		saml, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalf("error retrieving SAML providers: %v\n", err)
		}

		log.Printf("%s\n", saml.ID)
	}
	// [END list_saml_providers_tenant]
}

func deleteProviderConfigTenant(ctx context.Context, tenantClient *auth.TenantClient) {
	// [START delete_saml_provider_tenant]
	if err := tenantClient.DeleteSAMLProviderConfig(ctx, "saml.myProvider"); err != nil {
		log.Fatalf("error deleting SAML provider: %v\n", err)
	}
	// [END delete_saml_provider_tenant]
}

func getUserTenant(ctx context.Context, tenantClient *auth.TenantClient) *auth.UserRecord {
	uid := "some_string_uid"

	// [START get_user_tenant]
	// Get an auth client from the firebase.App
	u, err := tenantClient.GetUser(ctx, uid)
	if err != nil {
		log.Fatalf("error getting user %s: %v\n", uid, err)
	}

	log.Printf("Successfully fetched user data: %v\n", u)
	// [END get_user_tenant]
	return u
}

func getUserByEmailTenant(ctx context.Context, tenantClient *auth.TenantClient) *auth.UserRecord {
	email := "some@email.com"
	// [START get_user_by_email_tenant]
	u, err := tenantClient.GetUserByEmail(ctx, email)
	if err != nil {
		log.Fatalf("error getting user by email %s: %v\n", email, err)
	}
	log.Printf("Successfully fetched user data: %v\n", u)
	// [END get_user_by_email_tenant]
	return u
}

func createUserTenant(ctx context.Context, tenantClient *auth.TenantClient) *auth.UserRecord {
	// [START create_user_tenant]
	params := (&auth.UserToCreate{}).
		Email("user@example.com").
		EmailVerified(false).
		PhoneNumber("+15555550100").
		Password("secretPassword").
		DisplayName("John Doe").
		PhotoURL("http://www.example.com/12345678/photo.png").
		Disabled(false)
	u, err := tenantClient.CreateUser(ctx, params)
	if err != nil {
		log.Fatalf("error creating user: %v\n", err)
	}

	log.Printf("Successfully created user: %v\n", u)
	// [END create_user_tenant]
	return u
}

func updateUserTenant(ctx context.Context, tenantClient *auth.TenantClient, uid string) {
	// [START update_user_tenant]
	params := (&auth.UserToUpdate{}).
		Email("user@example.com").
		EmailVerified(true).
		PhoneNumber("+15555550100").
		Password("newPassword").
		DisplayName("John Doe").
		PhotoURL("http://www.example.com/12345678/photo.png").
		Disabled(true)
	u, err := tenantClient.UpdateUser(ctx, uid, params)
	if err != nil {
		log.Fatalf("error updating user: %v\n", err)
	}

	log.Printf("Successfully updated user: %v\n", u)
	// [END update_user_tenant]
}

func deleteUserTenant(ctx context.Context, tenantClient *auth.TenantClient, uid string) {
	// [START delete_user_tenant]
	if err := tenantClient.DeleteUser(ctx, uid); err != nil {
		log.Fatalf("error deleting user: %v\n", err)
	}

	log.Printf("Successfully deleted user: %s\n", uid)
	// [END delete_user_tenant]
}

func listUsersTenant(ctx context.Context, tenantClient *auth.TenantClient) {
	// [START list_all_users_tenant]
	// Note, behind the scenes, the Users() iterator will retrive 1000 Users at a time through the API
	iter := tenantClient.Users(ctx, "")
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
	pager := iterator.NewPager(tenantClient.Users(ctx, ""), 100, "")
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
	// [END list_all_users_tenant]
}

func importWithHMACTenant(ctx context.Context, tenantClient *auth.TenantClient) {
	// [START import_with_hmac_tenant]
	users := []*auth.UserToImport{
		(&auth.UserToImport{}).
			UID("uid1").
			Email("user1@example.com").
			PasswordHash([]byte("password-hash-1")).
			PasswordSalt([]byte("salt1")),
		(&auth.UserToImport{}).
			UID("uid2").
			Email("user2@example.com").
			PasswordHash([]byte("password-hash-2")).
			PasswordSalt([]byte("salt2")),
	}
	h := hash.HMACSHA256{
		Key: []byte("secret"),
	}
	result, err := tenantClient.ImportUsers(ctx, users, auth.WithHash(h))
	if err != nil {
		log.Fatalln("Error importing users", err)
	}

	for _, e := range result.Errors {
		log.Println("Failed to import user", e.Reason)
	}
	// [END import_with_hmac_tenant]
}

func importWithoutPasswordTenant(ctx context.Context, tenantClient *auth.TenantClient) {
	// [START import_without_password_tenant]
	users := []*auth.UserToImport{
		(&auth.UserToImport{}).
			UID("some-uid").
			DisplayName("John Doe").
			Email("johndoe@acme.com").
			PhotoURL("https://www.example.com/12345678/photo.png").
			EmailVerified(true).
			PhoneNumber("+11234567890").
			// Set this user as admin.
			CustomClaims(map[string]interface{}{"admin": true}).
			// User with SAML provider.
			ProviderData([]*auth.UserProvider{
				{
					UID:         "saml-uid",
					Email:       "johndoe@acme.com",
					DisplayName: "John Doe",
					PhotoURL:    "https://www.example.com/12345678/photo.png",
					ProviderID:  "saml.acme",
				},
			}),
	}
	result, err := tenantClient.ImportUsers(ctx, users)
	if err != nil {
		log.Fatalln("Error importing users", err)
	}
	for _, e := range result.Errors {
		log.Println("Failed to import user", e.Reason)
	}
	// [END import_without_password_tenant]
}

func verifyIDTokenTenant(ctx context.Context, tenantClient *auth.TenantClient, idToken string) {
	// [START verify_id_token_tenant]
	// idToken comes from the client app
	token, err := tenantClient.VerifyIDToken(ctx, idToken)
	if err != nil {
		log.Fatalf("error verifying ID token: %v\n", err)
	}

	// This should be set to TENANT-ID. Otherwise auth/mismatching-tenant-id error thrown.
	log.Printf("Verified ID token from tenant: %q\n", token.Firebase.Tenant)
	// [END verify_id_token_tenant]
}

func verifyIDTokenAccessControlTenant(ctx context.Context, tenantClient *auth.TenantClient, idToken string) {
	// [START id_token_access_control_tenant]
	token, err := tenantClient.VerifyIDToken(ctx, idToken)
	if err != nil {
		log.Fatalf("error verifying ID token: %v\n", err)
	}

	if token.Firebase.Tenant == "TENANT-ID1" {
		// Allow appropriate level of access for TENANT-ID1.
	} else if token.Firebase.Tenant == "TENANT-ID2" {
		// Allow appropriate level of access for TENANT-ID2.
	} else {
		// Access not allowed -- Handle error
	}
	// [END id_token_access_control_tenant]
}

func revokeRefreshTokensTenant(ctx context.Context, tenantClient *auth.TenantClient, uid string) {
	// [START revoke_tokens_tenant]
	// Revoke all refresh tokens for a specified user in a specified tenant for whatever reason.
	// Retrieve the timestamp of the revocation, in seconds since the epoch.
	if err := tenantClient.RevokeRefreshTokens(ctx, uid); err != nil {
		log.Fatalf("error revoking tokens for user: %v, %v\n", uid, err)
	}

	// accessing the user's TokenValidAfter
	u, err := tenantClient.GetUser(ctx, uid)
	if err != nil {
		log.Fatalf("error getting user %s: %v\n", uid, err)
	}

	timestamp := u.TokensValidAfterMillis / 1000
	log.Printf("the refresh tokens were revoked at: %d (UTC seconds) ", timestamp)
	// [END revoke_tokens_tenant]
}

func verifyIDTokenAndCheckRevokedTenant(ctx context.Context, tenantClient *auth.TenantClient, idToken string) {
	// [START verify_id_token_and_check_revoked_tenant]
	// Verify the ID token for a specific tenant while checking if the token is revoked.
	token, err := tenantClient.VerifyIDTokenAndCheckRevoked(ctx, idToken)
	if err != nil {
		if auth.IsIDTokenRevoked(err) {
			// Token is revoked. Inform the user to reauthenticate or signOut() the user.
		} else {
			// Token is invalid
		}
	}
	log.Printf("Verified ID token: %v\n", token)
	// [END verify_id_token_and_check_revoked_tenant]
}

func customClaimsSetTenant(ctx context.Context, tenantClient *auth.TenantClient, uid string) {
	// [START set_custom_user_claims_tenant]
	// Set admin privilege on the user corresponding to uid.
	claims := map[string]interface{}{"admin": true}
	if err := tenantClient.SetCustomUserClaims(ctx, uid, claims); err != nil {
		log.Fatalf("error setting custom claims %v\n", err)
	}
	// The new custom claims will propagate to the user's ID token the
	// next time a new one is issued.
	// [END set_custom_user_claims_tenant]
}

func customClaimsVerifyTenant(ctx context.Context, tenantClient *auth.TenantClient, idToken string) {
	// [START verify_custom_claims_tenant]
	// Verify the ID token first.
	token, err := tenantClient.VerifyIDToken(ctx, idToken)
	if err != nil {
		log.Fatal(err)
	}

	claims := token.Claims
	if admin, ok := claims["admin"]; ok {
		if admin.(bool) {
			//Allow access to requested admin resource.
		}
	}
	// [END verify_custom_claims_tenant]
}

func customClaimsReadTenant(ctx context.Context, tenantClient *auth.TenantClient, uid string) {
	// [START read_custom_user_claims_tenant]
	// Lookup the user associated with the specified uid.
	user, err := tenantClient.GetUser(ctx, uid)
	if err != nil {
		log.Fatal(err)
	}

	// The claims can be accessed on the user record.
	if admin, ok := user.CustomClaims["admin"]; ok {
		if admin.(bool) {
			log.Println(admin)
		}
	}
	// [END read_custom_user_claims_tenant]
}

func generateEmailVerificationLinkTenant(ctx context.Context, tenantClient *auth.TenantClient) {
	displayName := "Example User"
	email := "user@example.com"

	// [START email_verification_link_tenant]
	actionCodeSettings := &auth.ActionCodeSettings{
		// URL you want to redirect back to. The domain (www.example.com) for
		// this URL must be whitelisted in the GCP Console.
		URL: "https://www.example.com/checkout?cartId=1234",
		// This must be true for email link sign-in.
		HandleCodeInApp:       true,
		IOSBundleID:           "com.example.ios",
		AndroidPackageName:    "com.example.android",
		AndroidInstallApp:     true,
		AndroidMinimumVersion: "12",
		// FDL custom domain.
		DynamicLinkDomain: "coolapp.page.link",
	}

	link, err := tenantClient.EmailVerificationLinkWithSettings(ctx, email, actionCodeSettings)
	if err != nil {
		log.Fatalf("error generating email link: %v\n", err)
	}

	// Construct email verification template, embed the link and send
	// using custom SMTP server.
	sendCustomEmail(email, displayName, link)
	// [END email_verification_link_tenant]
}
