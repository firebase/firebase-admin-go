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
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"firebase.google.com/go/internal"
	"google.golang.org/api/iterator"
)


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
	*baseClient
}

// TenantID returns the ID of the tenant to which this TenantClient instance belongs.
func (tc *TenantClient) TenantID() string {
	return tc.tenantID
}

// CustomToken returns an error the SDK fails to discover a viable mechanism for signing tokens.
func (c *TenantClient) CustomToken(ctx context.Context, uid string) (string, error) {
	return c.CustomTokenWithClaims(ctx, uid, nil)
}

// CustomTokenWithClaims is similar to CustomToken, but in addition to the user ID, it also encodes
// all the key-value pairs in the provided map as claims in the resulting JWT.
func (c *TenantClient) CustomTokenWithClaims(ctx context.Context, uid string, devClaims map[string]interface{}) (string, error) {
	iss, err := c.signer.Email(ctx)
	if err != nil {
		return "", err
	}

	if len(uid) == 0 || len(uid) > 128 {
		return "", errors.New("uid must be non-empty, and not longer than 128 characters")
	}

	var disallowed []string
	for _, k := range reservedClaims {
		if _, contains := devClaims[k]; contains {
			disallowed = append(disallowed, k)
		}
	}
	if len(disallowed) == 1 {
		return "", fmt.Errorf("developer claim %q is reserved and cannot be specified", disallowed[0])
	} else if len(disallowed) > 1 {
		return "", fmt.Errorf("developer claims %q are reserved and cannot be specified", strings.Join(disallowed, ", "))
	}

	now := c.clock.Now().Unix()
	info := &jwtInfo{
		header: jwtHeader{Algorithm: "RS256", Type: "JWT"},
		payload: &customToken{
			Iss:    iss,
			Sub:    iss,
			Aud:    firebaseAudience,
			UID:    uid,
			Iat:    now,
			Exp:    now + oneHourInSeconds,
			Claims: devClaims,
		},
	}
	return info.Token(ctx, c.signer)
}
