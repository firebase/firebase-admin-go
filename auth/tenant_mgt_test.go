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

import "testing"

func TestAuthForTenantEmptyTenantID(t *testing.T) {
	tm := &TenantManager{}
	tc, err := tm.AuthForTenant("")
	if tc != nil || err == nil {
		t.Errorf("AuthForTenant() = (%v, %v); want = (nil, error)", tc, err)
	}
}

func TestTenantID(t *testing.T) {
	tm := &TenantManager{}
	tc, err := tm.AuthForTenant("tenantID")
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	tenantID := tc.TenantID()
	if tenantID != "tenantID" {
		t.Errorf("TenantID() = %q; want = %q", tenantID, "tenantID")
	}
}
