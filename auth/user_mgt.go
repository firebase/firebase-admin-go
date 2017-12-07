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

package auth

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"google.golang.org/api/iterator"

	"golang.org/x/net/context"
)

const maxReturnedResults = 1000
const maxLenPayloadCC = 1000

var (
	// Order matters only first error per field is reported.
	commonValidators = []struct {
		fieldName string
		testFun   func(map[string]interface{}) error
	}{
		{"disableUser", validateTrue},
		{"displayName", validateNonEmptyDisplayName},
		{"email", validateNonEmptyEmail},
		{"email", validateEmail},
		{"emailVerified", validateTrue},
		{"phoneNumber", validateNonEmptyPhoneNumber},
		{"phoneNumber", validatePhone},
		{"password", validatePassword},
		{"photoUrl", validateNonEmptyPhotoURL},
		{"localId", validateNonEmptyUID},
		{"localId", validateUID},
	}
)

// UserInfo is a collection of standard profile information for a user.
type UserInfo struct {
	DisplayName string `json:"displayName,omitempty"`
	Email       string `json:"email,omitempty"`
	PhoneNumber string `json:"phoneNumber,omitempty"`
	PhotoURL    string `json:"photoUrl,omitempty"`
	// ProviderID can be a short domain name (e.g. google.com),
	// or the identity of an OpenID identity provider.
	ProviderID string `json:"providerId,omitempty"`
	UID        string `json:"localId,omitempty"`
}

// UserMetadata contains additional metadata associated with a user account.
type UserMetadata struct {
	CreationTimestamp  int64
	LastLogInTimestamp int64
}

// UserRecord contains metadata associated with a Firebase user account.
type UserRecord struct {
	*UserInfo
	CustomClaims     map[string]interface{}
	Disabled         bool
	EmailVerified    bool
	ProviderUserInfo []*UserInfo
	UserMetadata     *UserMetadata
}

// ExportedUserRecord is the returned user value used when listing all the users.
type ExportedUserRecord struct {
	*UserRecord
	PasswordHash string
	PasswordSalt string
}

// UserIterator is an iterator over Users.
//
// Also see: https://github.com/GoogleCloudPlatform/google-cloud-go/wiki/Iterator-Guidelines
type UserIterator struct {
	client   *Client
	ctx      context.Context
	nextFunc func() error
	pageInfo *iterator.PageInfo
	users    []*ExportedUserRecord
}

type commonParams struct {
	payload map[string]interface{}
}

// UserToCreate is the parameter struct for the CreateUser function.
type UserToCreate struct {
	commonParams
}

// Disabled setter.
func (u *UserToCreate) Disabled(d bool) *UserToCreate { u.set("disableUser", d); return u }

// DisplayName setter.
func (u *UserToCreate) DisplayName(dn string) *UserToCreate { u.set("displayName", dn); return u }

// Email setter.
func (u *UserToCreate) Email(e string) *UserToCreate { u.set("email", e); return u }

// EmailVerified setter.
func (u *UserToCreate) EmailVerified(ev bool) *UserToCreate { u.set("emailVerified", ev); return u }

// Password setter.
func (u *UserToCreate) Password(pw string) *UserToCreate { u.set("password", pw); return u }

// PhoneNumber setter.
func (u *UserToCreate) PhoneNumber(phone string) *UserToCreate { u.set("phoneNumber", phone); return u }

// PhotoURL setter.
func (u *UserToCreate) PhotoURL(url string) *UserToCreate { u.set("photoUrl", url); return u }

// UID setter.
func (u *UserToCreate) UID(uid string) *UserToCreate { u.set("localId", uid); return u }

// UserToUpdate is the parameter struct for the UpdateUser function.
type UserToUpdate struct {
	commonParams
}

// CustomClaims setter.
func (u *UserToUpdate) CustomClaims(cc map[string]interface{}) *UserToUpdate {
	u.set("customClaims", cc)
	return u
}

// Disabled setter.
func (u *UserToUpdate) Disabled(d bool) *UserToUpdate { u.set("disableUser", d); return u }

// DisplayName setter.
func (u *UserToUpdate) DisplayName(dn string) *UserToUpdate { u.set("displayName", dn); return u }

// Email setter.
func (u *UserToUpdate) Email(e string) *UserToUpdate { u.set("email", e); return u }

// EmailVerified setter.
func (u *UserToUpdate) EmailVerified(ev bool) *UserToUpdate { u.set("emailVerified", ev); return u }

// Password setter.
func (u *UserToUpdate) Password(pw string) *UserToUpdate { u.set("password", pw); return u }

// PhoneNumber setter.
func (u *UserToUpdate) PhoneNumber(phone string) *UserToUpdate { u.set("phoneNumber", phone); return u }

// PhotoURL setter.
func (u *UserToUpdate) PhotoURL(url string) *UserToUpdate { u.set("photoUrl", url); return u }

// CreateUser creates a new user with the specified properties.
func (c *Client) CreateUser(ctx context.Context, params *UserToCreate) (*UserRecord, error) {
	if params == nil {
		params = &UserToCreate{}
	}

	payload, err := params.preparePayload()
	if err != nil {
		return nil, err
	}
	return c.createOrUpdateUser(ctx, "signupNewUser", payload)
}

// UpdateUser updates an existing user account with the specified properties.
//
// DisplayName, PhotoURL and PhoneNumber will be set to "" to signify deleting them from the record.
func (c *Client) UpdateUser(ctx context.Context, uid string, params *UserToUpdate) (ur *UserRecord, err error) {
	if params == nil || params.payload == nil {
		return nil, fmt.Errorf("params must not be empty for update")
	}
	params.payload["localId"] = uid

	payload, err := params.preparePayload()
	if err != nil {
		return nil, err
	}
	return c.createOrUpdateUser(ctx, "setAccountInfo", payload)
}

// DeleteUser deletes the user by the given UID.
func (c *Client) DeleteUser(ctx context.Context, uid string) error {
	var resp map[string]interface{}
	deleteParams := map[string]interface{}{"localId": []string{uid}}
	return c.makeHTTPCall(ctx, "deleteAccount", deleteParams, &resp)
}

// GetUser gets the user data corresponding to the specified user ID.
func (c *Client) GetUser(ctx context.Context, uid string) (*UserRecord, error) {
	return c.getUser(ctx, map[string]interface{}{"localId": []string{uid}})
}

// GetUserByPhoneNumber gets the user data corresponding to the specified user phone number.
func (c *Client) GetUserByPhoneNumber(ctx context.Context, phone string) (*UserRecord, error) {
	return c.getUser(ctx, map[string]interface{}{"phoneNumber": []string{phone}})
}

// GetUserByEmail gets the user data corresponding to the specified email.
func (c *Client) GetUserByEmail(ctx context.Context, email string) (*UserRecord, error) {
	return c.getUser(ctx, map[string]interface{}{"email": []string{email}})
}

// Users returns an iterator over Users.
//
// If nextPageToken is empty, the iterator will start at the beginning.
// If the nextPageToken is not empty, the iterator starts after the token.
func (c *Client) Users(ctx context.Context, nextPageToken string) *UserIterator {
	it := &UserIterator{
		ctx:    ctx,
		client: c,
	}
	it.pageInfo, it.nextFunc = iterator.NewPageInfo(
		it.fetch,
		func() int { return len(it.users) },
		func() interface{} { b := it.users; it.users = nil; return b })
	it.pageInfo.MaxSize = maxReturnedResults
	it.pageInfo.Token = nextPageToken
	return it
}

func (it *UserIterator) fetch(pageSize int, pageToken string) (string, error) {
	params := map[string]interface{}{"maxResults": pageSize}
	if pageToken != "" {
		params["nextPageToken"] = pageToken
	}

	var lur listUsersResponse
	err := it.client.makeHTTPCall(it.ctx, "downloadAccount", params, &lur)
	if err != nil {
		return "", err
	}
	for _, u := range lur.Users {
		eu, err := makeExportedUser(u)
		if err != nil {
			return "", err
		}
		it.users = append(it.users, eu)
	}
	it.pageInfo.Token = lur.NextPage
	return lur.NextPage, nil
}

// PageInfo supports pagination. See the google.golang.org/api/iterator package for details.
// Page size can be determined by the NewPager(...) function described there.
func (it *UserIterator) PageInfo() *iterator.PageInfo { return it.pageInfo }

// Next returns the next result. Its second return value is [iterator.Done] if
// there are no more results. Once Next returns [iterator.Done], all subsequent
// calls will return [iterator.Done].
func (it *UserIterator) Next() (*ExportedUserRecord, error) {
	if err := it.nextFunc(); err != nil {
		return nil, err
	}
	user := it.users[0]
	it.users = it.users[1:]
	return user, nil
}

// SetCustomUserClaims sets additional claims on an existing user account.
//
// Custom claims set via this function can be used to define user roles and privilege levels.
// These claims propagate to all the devices where the user is already signed in (after token
// expiration or when token refresh is forced), and next time the user signs in. The claims
// can be accessed via the user's ID token JWT. If a reserved OIDC claim is specified (sub, iat,
// iss, etc), an error is thrown. Claims payload must also not be larger then 1000 characters
// when serialized into a JSON string.
func (c *Client) SetCustomUserClaims(ctx context.Context, uid string, customClaims map[string]interface{}) error {
	if customClaims == nil || len(customClaims) == 0 {
		customClaims = map[string]interface{}{}
	}
	_, err := c.UpdateUser(ctx, uid, (&UserToUpdate{}).CustomClaims(customClaims))
	return err
}

// ------------------------------------------------------------
// Utilities for Create and Update input structs.

func (p *commonParams) set(key string, value interface{}) {
	if p.payload == nil {
		p.payload = make(map[string]interface{})
	}
	p.payload[key] = value
}

// assumes that payloadName is a string field in p.payload
func processDeletion(p map[string]interface{}, payloadName string) {
	deletionSpecs := map[string]struct {
		deleteListName  string
		deleteFieldName string
	}{
		"displayName": {"deleteAttribute", "DISPLAY_NAME"},
		"phoneNumber": {"deleteProvider", "phone"},
		"photoUrl":    {"deleteAttribute", "PHOTO_URL"},
	}

	if dn, ok := p[payloadName]; ok && len(dn.(string)) == 0 {
		delSpec := deletionSpecs[payloadName]
		addToListParam(p, delSpec.deleteListName, delSpec.deleteFieldName)
		delete(p, payloadName)
	}
}

func addToListParam(p map[string]interface{}, listname, param string) {
	if _, ok := p[listname]; ok {
		p[listname] = append(p[listname].([]string), param)
	} else {
		p[listname] = []string{param}
	}
}

func processClaims(p map[string]interface{}) error {
	if _, ok := p["customClaims"]; !ok {
		return nil
	}
	cc := p["customClaims"]
	claims, ok := cc.(map[string]interface{})
	if !ok {
		return fmt.Errorf("CustomClaims(): unexpected type")
	}
	for _, key := range reservedClaims {
		if _, ok := claims[key]; ok {
			return fmt.Errorf("CustomClaims(%q: ...): claim %q is reserved, and must not be set", key, key)
		}
	}
	b, err := json.Marshal(claims)
	if err != nil {
		return fmt.Errorf("CustomClaims() Marshaling error: %v", err)
	}
	s := string(b)
	if s == "null" {
		s = "{}"
	}
	p["customAttributes"] = s
	delete(p, "customClaims")
	wantLength := maxLenPayloadCC
	if val, ok := p["customAttributes"]; ok {
		if len(val.(string)) > wantLength {
			return fmt.Errorf("stringified JSON of CustomClaims must be a string at most %d characters long", wantLength)
		}
	}
	return nil
}

// Validators.

// No validation needed. Used for bool fields.
func validateTrue(p map[string]interface{}) error {
	return nil
}

func validateNonEmptyDisplayName(p map[string]interface{}) error {
	return validateNonEmpty(p, "displayName")
}
func validateNonEmptyEmail(p map[string]interface{}) error { return validateNonEmpty(p, "email") }
func validateNonEmptyPhoneNumber(p map[string]interface{}) error {
	return validateNonEmpty(p, "phoneNumber")
}
func validateNonEmptyPhotoURL(p map[string]interface{}) error { return validateNonEmpty(p, "photoUrl") }
func validateNonEmptyUID(p map[string]interface{}) error      { return validateNonEmpty(p, "localId") }

func validateNonEmpty(p map[string]interface{}, fieldName string) error {
	if val, ok := p[fieldName]; ok {
		if len(val.(string)) == 0 {
			return fmt.Errorf("%s must be a non-empty string", fieldName)
		}
	}
	return nil
}

func validateEmail(p map[string]interface{}) error {
	fieldName := "email"
	if val, ok := p[fieldName]; ok {
		if parts := strings.Split(val.(string), "@"); len(parts) != 2 || len(parts[0]) == 0 || len(parts[1]) == 0 {
			return fmt.Errorf("malformed %s string: %q", fieldName, val)
		}
	}
	return nil
}

func validatePassword(p map[string]interface{}) error {
	wantLength := 6
	fieldName := "password"
	if val, ok := p[fieldName]; ok {
		if len(val.(string)) < wantLength {
			return fmt.Errorf("%s must be a string at least %d characters long", fieldName, wantLength)
		}
	}
	return nil
}

func validateUID(p map[string]interface{}) error {
	fieldName := "localId"
	wantLength := 128
	if val, ok := p[fieldName]; ok {
		if len(val.(string)) > wantLength {
			return fmt.Errorf("%s must be a string at most %d characters long", fieldName, wantLength)
		}
	}
	return nil
}

/*
func validateCustomAttributes(p map[string]interface{}) error {

}
*/
func validatePhone(p map[string]interface{}) error {
	fieldName := "phoneNumber"
	if val, ok := p[fieldName]; ok {
		if !regexp.MustCompile(`\+.*[0-9A-Za-z]`).MatchString(val.(string)) {
			return fmt.Errorf(
				"invalid %s %q. Must be a valid, E.164 compliant identifier", fieldName, val)
		}
	}
	return nil
}

func (u *UserToCreate) preparePayload() (map[string]interface{}, error) {
	if u.payload == nil {
		return map[string]interface{}{}, nil
	}
	params := map[string]interface{}{}
	for k, v := range u.payload {
		params[k] = v
	}
	for _, test := range commonValidators {
		if err := test.testFun(params); err != nil {
			return nil, err
		}
	}
	return params, nil
}

func (u *UserToUpdate) preparePayload() (map[string]interface{}, error) {
	if u.payload == nil {
		return nil, fmt.Errorf("update with no params") // This was caught in the caller not here.
	}
	params := map[string]interface{}{}
	for k, v := range u.payload {
		params[k] = v
	}
	processDeletion(params, "displayName")
	processDeletion(params, "phoneNumber")
	processDeletion(params, "photoUrl")
	if err := processClaims(params); err != nil {
		return nil, err
	}

	for _, test := range commonValidators {
		if err := test.testFun(params); err != nil {
			return nil, err
		}
	}
	return params, nil
}

// End of validators

// Respose Types -------------------------------

type getUserResponse struct {
	RequestType string               `json:"kind,omitempty"`
	Users       []responseUserRecord `json:"users,omitempty"`
}

type responseUserRecord struct {
	UID                string      `json:"localId,omitempty"`
	DisplayName        string      `json:"displayName,omitempty"`
	Email              string      `json:"email,omitempty"`
	PhoneNumber        string      `json:"phoneNumber,omitempty"`
	PhotoURL           string      `json:"photoUrl,omitempty"`
	CreationTimestamp  int64       `json:"createdAt,string,omitempty"`
	LastLogInTimestamp int64       `json:"lastLoginAt,string,omitempty"`
	ProviderID         string      `json:"providerId,omitempty"`
	CustomClaims       string      `json:"customAttributes,omitempty"`
	Disabled           bool        `json:"disabled,omitempty"`
	EmailVerified      bool        `json:"emailVerified,omitempty"`
	ProviderUserInfo   []*UserInfo `json:"providerUserInfo,omitempty"`
	PasswordHash       string      `json:"passwordHash,omitempty"`
	PasswordSalt       string      `json:"salt,omitempty"`
	ValidSince         int64       `json:"validSince,string,omitempty"`
}

type listUsersResponse struct {
	RequestType string               `json:"kind,omitempty"`
	Users       []responseUserRecord `json:"users,omitempty"`
	NextPage    string               `json:"nextPageToken,omitempty"`
}

// Helper functions for retrieval and HTTP calls.

func (c *Client) getUser(ctx context.Context, params map[string]interface{}) (*UserRecord, error) {
	var gur getUserResponse
	err := c.makeHTTPCall(ctx, "getAccountInfo", params, &gur)
	if err != nil {
		return nil, err
	}
	if len(gur.Users) == 0 {
		return nil, fmt.Errorf("cannot find user %v", params)
	}
	if l := len(gur.Users); l > 1 {
		return nil, fmt.Errorf("getUser(%v) got %d users; want: one user, ", params, l)
	}
	eu, err := makeExportedUser(gur.Users[0])
	return eu.UserRecord, err
}

func (c *Client) createOrUpdateUser(ctx context.Context, action string, params map[string]interface{}) (*UserRecord, error) {
	var rur responseUserRecord
	err := c.makeHTTPCall(ctx, action, params, &rur)
	if err != nil {
		return nil, err
	}
	return c.GetUser(ctx, rur.UID)
}

func makeExportedUser(r responseUserRecord) (*ExportedUserRecord, error) {
	var cc map[string]interface{}
	if r.CustomClaims != "" {
		err := json.Unmarshal([]byte(r.CustomClaims), &cc)
		if err != nil {
			return nil, err
		}
		if len(cc) == 0 {
			cc = nil
		}
	}

	resp := &ExportedUserRecord{
		UserRecord: &UserRecord{
			UserInfo: &UserInfo{
				DisplayName: r.DisplayName,
				Email:       r.Email,
				PhoneNumber: r.PhoneNumber,
				PhotoURL:    r.PhotoURL,
				ProviderID:  r.ProviderID,
				UID:         r.UID,
			},
			CustomClaims:     cc,
			Disabled:         r.Disabled,
			EmailVerified:    r.EmailVerified,
			ProviderUserInfo: r.ProviderUserInfo,
			UserMetadata: &UserMetadata{
				LastLogInTimestamp: r.LastLogInTimestamp,
				CreationTimestamp:  r.CreationTimestamp,
			},
		},
		PasswordHash: r.PasswordHash,
		PasswordSalt: r.PasswordSalt,
	}
	return resp, nil
}
