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
	"reflect"
	"testing"

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

	t.Run("UserManagement", func(t *testing.T) {
		testTenantAwareUserManagement(t, id)
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
