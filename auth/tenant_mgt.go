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

import "errors"

// Tenant represents a tenant in a multi-tenant application.
//
// Multi-tenancy support requires Google Cloud's Identity Platform (GCIP). To learn more about GCIP,
// including pricing and features, see https://cloud.google.com/identity-platform.
//
// Before multi-tenancy can be used in a Google Cloud Identity Platform project, tenants must be
// enabled in that project via the Cloud Console UI.
//
// A tenant configuration provides information such as the display name, tenant identifier and email
// authentication configuration. For OIDC/SAML provider configuration management, TenantClient
// instances should be used instead of a Tenant to retrieve the list of configured IdPs on a tenant.
// When configuring these providers, note that tenants will inherit whitelisted domains and
// authenticated redirect URIs of their parent project.
//
// All other settings of a tenant will also be inherited. These will need to be managed from the
// Cloud Console UI.
type Tenant struct {
	ID                    string
	DisplayName           string
	AllowPasswordSignUp   bool
	EnableEmailLinkSignIn bool
}

// TenantClient is used for managing users, configuring SAML/OIDC providers, and generating email
// links for specific tenants.
//
// Before multi-tenancy can be used in a Google Cloud Identity Platform project, tenants must be
// enabled in that project via the Cloud Console UI.
//
// Each tenant contains its own identity providers, settings and users. TenantClient enables
// managing users and SAML/OIDC configurations of specific tenants. It also supports verifying ID
// tokens issued to users who are signed into specific tenants.
//
// TenantClient instances for a specific tenantID can be instantiated by calling
// [TenantManager.AuthForTenant(tenantID)].
type TenantClient struct {
	tenantID string
	*userManagementClient
}

// TenantID returns the ID of the tenant to which this TenantClient instance belongs.
func (tc *TenantClient) TenantID() string {
	return tc.tenantID
}

// TenantManager is the interface used to manage tenants in a multi-tenant application.
//
// This supports creating, updating, listing, deleting the tenants of a Firebase project. It also
// supports creating new TenantClient instances scoped to specific tenant IDs.
type TenantManager struct {
	userManagementClient *userManagementClient
}

// AuthForTenant creates a new TenantClient scoped to a given tenantID.
func (tm *TenantManager) AuthForTenant(tenantID string) (*TenantClient, error) {
	if tenantID == "" {
		return nil, errors.New("tenantID must not be empty")
	}

	return &TenantClient{
		tenantID:             tenantID,
		userManagementClient: tm.userManagementClient.withTenantID(tenantID),
	}, nil
}
