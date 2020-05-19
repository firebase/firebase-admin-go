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

package auth

import (
	"context"
	"log"
	"net/url"
	"reflect"
	"testing"
	"time"

	"firebase.google.com/go/auth"
	"google.golang.org/api/iterator"
)

func TestTenantManager(t *testing.T) {
	want := &auth.Tenant{
		DisplayName:           "admin-go-tenant",
		AllowPasswordSignUp:   true,
		EnableEmailLinkSignIn: true,
	}

	req := (&auth.TenantToCreate{}).
		DisplayName("admin-go-tenant").
		AllowPasswordSignUp(true).
		EnableEmailLinkSignIn(true)
	created, err := client.TenantManager.CreateTenant(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateTenant() = %v", err)
	}

	id := created.ID
	want.ID = id

	// Clean up action in the event of a panic
	defer func() {
		if id == "" {
			return
		}
		if err := client.TenantManager.DeleteTenant(context.Background(), id); err != nil {
			log.Printf("WARN: failed to delete tenant %q on tear down: %v", id, err)
		}
	}()

	t.Run("CreateTenant()", func(t *testing.T) {
		if !reflect.DeepEqual(created, want) {
			t.Errorf("CreateTenant() = %#v; want = %#v", created, want)
		}
	})

	t.Run("Tenant()", func(t *testing.T) {
		tenant, err := client.TenantManager.Tenant(context.Background(), id)
		if err != nil {
			t.Fatalf("Tenant() = %v", err)
		}

		if !reflect.DeepEqual(tenant, want) {
			t.Errorf("Tenant() = %#v; want = %#v", tenant, want)
		}
	})

	t.Run("Tenants()", func(t *testing.T) {
		iter := client.TenantManager.Tenants(context.Background(), "")
		var target *auth.Tenant
		for {
			tenant, err := iter.Next()
			if err == iterator.Done {
				break
			} else if err != nil {
				t.Fatalf("Tenants() = %v", err)
			}

			if tenant.ID == id {
				target = tenant
				break
			}
		}

		if target == nil {
			t.Fatalf("Tenants() did not return required tenant: %q", id)
		}
		if !reflect.DeepEqual(target, want) {
			t.Errorf("Tenants() = %#v; want = %#v", target, want)
		}
	})

	t.Run("CustomTokens", func(t *testing.T) {
		testTenantAwareCustomToken(t, id)
	})

	t.Run("UserManagement", func(t *testing.T) {
		testTenantAwareUserManagement(t, id)
	})

	t.Run("OIDCProviderConfig", func(t *testing.T) {
		tenantClient, err := client.TenantManager.AuthForTenant(id)
		if err != nil {
			t.Fatalf("AuthForTenant() = %v", err)
		}

		testOIDCProviderConfig(t, tenantClient)
	})

	t.Run("SAMLProviderConfig", func(t *testing.T) {
		tenantClient, err := client.TenantManager.AuthForTenant(id)
		if err != nil {
			t.Fatalf("AuthForTenant() = %v", err)
		}

		testSAMLProviderConfig(t, tenantClient)
	})

	t.Run("UpdateTenant()", func(t *testing.T) {
		want = &auth.Tenant{
			ID:                    id,
			DisplayName:           "updated-go-tenant",
			AllowPasswordSignUp:   false,
			EnableEmailLinkSignIn: false,
		}
		req := (&auth.TenantToUpdate{}).
			DisplayName("updated-go-tenant").
			AllowPasswordSignUp(false).
			EnableEmailLinkSignIn(false)
		tenant, err := client.TenantManager.UpdateTenant(context.Background(), id, req)
		if err != nil {
			t.Fatalf("UpdateTenant() = %v", err)
		}

		if !reflect.DeepEqual(tenant, want) {
			t.Errorf("UpdateTenant() = %#v; want = %#v", tenant, want)
		}
	})

	t.Run("DeleteTenant()", func(t *testing.T) {
		if err := client.TenantManager.DeleteTenant(context.Background(), id); err != nil {
			t.Fatalf("DeleteTenant() = %v", err)
		}

		_, err := client.TenantManager.Tenant(context.Background(), id)
		if err == nil || !auth.IsTenantNotFound(err) {
			t.Errorf("Tenant() = %v; want = TenantNotFound", err)
		}

		id = ""
	})
}

func testTenantAwareCustomToken(t *testing.T, id string) {
	tenantClient, err := client.TenantManager.AuthForTenant(id)
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	uid := randomUID()
	ct, err := tenantClient.CustomToken(context.Background(), uid)
	if err != nil {
		t.Fatal(err)
	}

	idToken, err := signInWithCustomTokenForTenant(ct, id)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		tenantClient.DeleteUser(context.Background(), uid)
	}()

	vt, err := tenantClient.VerifyIDToken(context.Background(), idToken)
	if err != nil {
		t.Fatal(err)
	}

	if vt.UID != uid {
		t.Errorf("UID = %q; want UID = %q", vt.UID, uid)
	}
	if vt.Firebase.Tenant != id {
		t.Errorf("Tenant = %q; want = %q", vt.Firebase.Tenant, id)
	}
}

func testTenantAwareUserManagement(t *testing.T, id string) {
	tenantClient, err := client.TenantManager.AuthForTenant(id)
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	user, err := tenantClient.CreateUser(context.Background(), nil)
	if err != nil {
		t.Fatalf("CreateUser() = %v", err)
	}

	t.Run("CreateUser()", func(t *testing.T) {
		if user.TenantID != id {
			t.Errorf("CreateUser().TenantID = %q; want = %q", user.TenantID, id)
		}
	})

	want := auth.UserInfo{
		UID:         user.UID,
		Email:       randomEmail(user.UID),
		PhoneNumber: randomPhoneNumber(),
		ProviderID:  "firebase",
	}
	t.Run("UpdateUser()", func(t *testing.T) {
		req := (&auth.UserToUpdate{}).
			Email(want.Email).
			PhoneNumber(want.PhoneNumber)
		updated, err := tenantClient.UpdateUser(context.Background(), user.UID, req)
		if err != nil {
			t.Fatalf("UpdateUser() = %v", err)
		}

		if updated.TenantID != id {
			t.Errorf("UpdateUser().TenantID = %q; want = %q", updated.TenantID, id)
		}

		if !reflect.DeepEqual(*updated.UserInfo, want) {
			t.Errorf("UpdateUser() = %v; want = %v", *updated.UserInfo, want)
		}
	})

	t.Run("GetUser()", func(t *testing.T) {
		got, err := tenantClient.GetUser(context.Background(), user.UID)
		if err != nil {
			t.Fatalf("GetUser() = %v", err)
		}

		if got.TenantID != id {
			t.Errorf("GetUser().TenantID = %q; want = %q", got.TenantID, id)
		}

		if !reflect.DeepEqual(*got.UserInfo, want) {
			t.Errorf("GetUser() = %v; want = %v", *got.UserInfo, want)
		}
	})

	t.Run("Users()", func(t *testing.T) {
		iter := tenantClient.Users(context.Background(), "")
		var target *auth.ExportedUserRecord
		for {
			got, err := iter.Next()
			if err == iterator.Done {
				break
			} else if err != nil {
				t.Fatalf("Users() = %v", err)
			}

			if got.UID == user.UID {
				target = got
				break
			}
		}

		if target == nil {
			t.Fatalf("Users() did not return required user: %q", user.UID)
		}

		if !reflect.DeepEqual(*target.UserInfo, want) {
			t.Errorf("Users() = %v; want = %v", *target.UserInfo, want)
		}
	})

	t.Run("SetCustomUserClaims()", func(t *testing.T) {
		claims := map[string]interface{}{
			"premium": true,
			"role":    "customer",
		}
		if err := tenantClient.SetCustomUserClaims(context.Background(), user.UID, claims); err != nil {
			t.Fatalf("SetCustomUserClaims() = %v", err)
		}

		got, err := tenantClient.GetUser(context.Background(), user.UID)
		if err != nil {
			t.Fatalf("GetUser() = %v", err)
		}

		if !reflect.DeepEqual(got.CustomClaims, claims) {
			t.Errorf("CustomClaims = %v; want = %v", got.CustomClaims, claims)
		}
	})

	t.Run("EmailVerificationLink()", func(t *testing.T) {
		link, err := tenantClient.EmailVerificationLink(context.Background(), want.Email)
		if err != nil {
			t.Fatalf("EmailVerificationLink() = %v", err)
		}

		tenant, err := extractTenantID(link)
		if err != nil {
			t.Fatalf("EmailVerificationLink() = %v", err)
		}

		if id != tenant {
			t.Fatalf("EmailVerificationLink() TenantID = %q; want = %q", tenant, id)
		}
	})

	t.Run("PasswordResetLink()", func(t *testing.T) {
		link, err := tenantClient.PasswordResetLink(context.Background(), want.Email)
		if err != nil {
			t.Fatalf("PasswordResetLink() = %v", err)
		}

		tenant, err := extractTenantID(link)
		if err != nil {
			t.Fatalf("PasswordResetLink() = %v", err)
		}

		if id != tenant {
			t.Fatalf("PasswordResetLink() TenantID = %q; want = %q", tenant, id)
		}
	})

	t.Run("EmailSignInLink()", func(t *testing.T) {
		link, err := tenantClient.EmailSignInLink(context.Background(), want.Email, &auth.ActionCodeSettings{
			URL:             continueURL,
			HandleCodeInApp: false,
		})
		if err != nil {
			t.Fatalf("EmailSignInLink() = %v", err)
		}

		tenant, err := extractTenantID(link)
		if err != nil {
			t.Fatalf("EmailSignInLink() = %v", err)
		}

		if id != tenant {
			t.Fatalf("EmailSignInLink() TenantID = %q; want = %q", tenant, id)
		}
	})

	t.Run("RevokeRefreshTokens()", func(t *testing.T) {
		validSinceMillis := time.Now().Unix() * 1000
		time.Sleep(1 * time.Second)
		if err := tenantClient.RevokeRefreshTokens(context.Background(), user.UID); err != nil {
			t.Fatalf("RevokeRefreshTokens() = %v", err)
		}

		got, err := tenantClient.GetUser(context.Background(), user.UID)
		if err != nil {
			t.Fatalf("GetUser() = %v", err)
		}

		if got.TokensValidAfterMillis < validSinceMillis {
			t.Fatalf("RevokeRefreshTokens() TokensValidAfterMillis (%d) < Now (%d)", got.TokensValidAfterMillis, validSinceMillis)
		}
	})

	t.Run("ImportUsers()", func(t *testing.T) {
		scrypt, passwordHash, err := newScryptHash()
		if err != nil {
			t.Fatalf("newScryptHash() = %v", err)
		}

		uid := randomUID()
		email := randomEmail(uid)
		user := (&auth.UserToImport{}).
			UID(uid).
			Email(email).
			PasswordHash(passwordHash).
			PasswordSalt([]byte("NaCl"))
		result, err := tenantClient.ImportUsers(context.Background(), []*auth.UserToImport{user}, auth.WithHash(scrypt))
		if err != nil {
			t.Fatalf("ImportUsers() = %v", err)
		}

		defer func() {
			tenantClient.DeleteUser(context.Background(), uid)
		}()

		if result.SuccessCount != 1 || result.FailureCount != 0 {
			t.Errorf("ImportUsers() = %#v; want = {SuccessCount: 1, FailureCount: 0}", result)
		}

		savedUser, err := tenantClient.GetUser(context.Background(), uid)
		if err != nil {
			t.Fatalf("GetUser() = %v", err)
		}
		if savedUser.Email != email {
			t.Errorf("ImportUser() Email = %q; want = %q", savedUser.Email, email)
		}
		if savedUser.TenantID != id {
			t.Errorf("ImportUser() TenantID = %q; want = %q", savedUser.TenantID, id)
		}
	})

	t.Run("DeleteUser()", func(t *testing.T) {
		if err := tenantClient.DeleteUser(context.Background(), user.UID); err != nil {
			t.Fatalf("DeleteUser() = %v", err)
		}

		_, err = tenantClient.GetUser(context.Background(), user.UID)
		if err == nil || !auth.IsUserNotFound(err) {
			t.Errorf("Tenant() = %v; want = UserNotFound", err)
		}
	})
}

func extractTenantID(actionLink string) (string, error) {
	u, err := url.Parse(actionLink)
	if err != nil {
		return "", err
	}

	q := u.Query()
	return q.Get("tenantId"), nil
}
