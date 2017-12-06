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
	updatePreProcess = []struct {
		fieldName string
		testFun   func(*commonParams, string) error
	}{
		{"displayName", processDeletion},
		{"phoneNumber", processDeletion},
		{"photoUrl", processDeletion},
		{"customClaims", processClaims},
	}
	deletionSpecs = map[string]struct {
		deleteListName  string
		deleteFieldName string
	}{
		"displayName": {"deleteAttribute", "DISPLAY_NAME"},
		"phoneNumber": {"deleteProvider", "phone"},
		"photoUrl":    {"deleteAttribute", "PHOTO_URL"},
	}

	// Order matters only first error per field is reported.
	commonValidators = []struct {
		fieldName string
		testFun   func(*commonParams, string) error
	}{
		{"disableUser", validateTrue},
		{"displayName", validateNonEmpty},
		{"email", validateNonEmpty},
		{"email", validateEmail},
		{"emailVerified", validateTrue},
		{"phoneNumber", validateNonEmpty},
		{"phoneNumber", validatePhone},
		{"password", validatePassword},
		{"photoUrl", validateNonEmpty},
		{"localId", validateNonEmpty},
		{"localId", validateUID},
	}

	updateValidators = []struct {
		fieldName string
		testFun   func(*commonParams, string) error
	}{
		{"customAttributes", validateCustomAttributes},
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
func (p *UserToCreate) Disabled(d bool) *UserToCreate { p.set("disableUser", d); return p }

// DisplayName setter.
func (p *UserToCreate) DisplayName(dn string) *UserToCreate { p.set("displayName", dn); return p }

// Email setter.
func (p *UserToCreate) Email(e string) *UserToCreate { p.set("email", e); return p }

// EmailVerified setter.
func (p *UserToCreate) EmailVerified(ev bool) *UserToCreate { p.set("emailVerified", ev); return p }

// Password setter.
func (p *UserToCreate) Password(pw string) *UserToCreate { p.set("password", pw); return p }

// PhoneNumber setter.
func (p *UserToCreate) PhoneNumber(phone string) *UserToCreate { p.set("phoneNumber", phone); return p }

// PhotoURL setter.
func (p *UserToCreate) PhotoURL(url string) *UserToCreate { p.set("photoUrl", url); return p }

// UID setter.
func (p *UserToCreate) UID(uid string) *UserToCreate { p.set("localId", uid); return p }

// UserToUpdate is the parameter struct for the UpdateUser function.
type UserToUpdate struct {
	commonParams
}

// CustomClaims setter.
func (p *UserToUpdate) CustomClaims(cc map[string]interface{}) *UserToUpdate {
	p.set("customClaims", cc)
	return p
}

// Disabled setter.
func (p *UserToUpdate) Disabled(d bool) *UserToUpdate { p.set("disableUser", d); return p }

// DisplayName setter.
func (p *UserToUpdate) DisplayName(dn string) *UserToUpdate { p.set("displayName", dn); return p }

// Email setter.
func (p *UserToUpdate) Email(e string) *UserToUpdate { p.set("email", e); return p }

// EmailVerified setter.
func (p *UserToUpdate) EmailVerified(ev bool) *UserToUpdate { p.set("emailVerified", ev); return p }

// Password setter.
func (p *UserToUpdate) Password(pw string) *UserToUpdate { p.set("password", pw); return p }

// PhoneNumber setter.
func (p *UserToUpdate) PhoneNumber(phone string) *UserToUpdate { p.set("phoneNumber", phone); return p }

// PhotoURL setter.
func (p *UserToUpdate) PhotoURL(url string) *UserToUpdate { p.set("photoUrl", url); return p }

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
	var gur getUserResponse
	deleteParams := map[string]interface{}{"localId": []string{uid}}
	return c.makeHTTPCall(ctx, "deleteAccount", deleteParams, &gur)
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
// If startToken is empty, the iterator will start at the beginning.
// If the startToken is not empty, the iterator starts after the token.
func (c *Client) Users(ctx context.Context, startToken string) *UserIterator {
	it := &UserIterator{
		ctx:    ctx,
		client: c,
	}
	it.pageInfo, it.nextFunc = iterator.NewPageInfo(
		it.fetch,
		func() int { return len(it.users) },
		func() interface{} { b := it.users; it.users = nil; return b })
	it.pageInfo.MaxSize = maxReturnedResults
	it.pageInfo.Token = startToken
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
func processDeletion(p *commonParams, payloadName string) error {
	if dn, ok := p.payload[payloadName]; ok && len(dn.(string)) == 0 {
		delSpec := deletionSpecs[payloadName]
		p.addToListParam(delSpec.deleteListName, delSpec.deleteFieldName)
		delete(p.payload, payloadName)
	}
	return nil
}

func (p *commonParams) addToListParam(listname, param string) {
	if _, ok := p.payload[listname]; ok {
		p.payload[listname] = append(p.payload[listname].([]string), param)
	} else {
		p.set(listname, []string{param})
	}
}

func processClaims(p *commonParams, payloadName string) error {
	if _, ok := p.payload["customClaims"]; !ok {
		return nil
	}
	cc := p.payload["customClaims"]
	claims, ok := cc.(map[string]interface{})
	if !ok {
		return fmt.Errorf("CustomClaims: unexpected type")
	}
	for _, key := range reservedClaims {
		if _, ok := claims[key]; ok {
			return fmt.Errorf("CustomClaims, claim %q is reserved, and must not be set", key)
		}
	}
	b, err := json.Marshal(claims)
	if err != nil {
		return fmt.Errorf("CustomClaims Marshaling error: %v", err)
	}
	s := string(b)
	if s == "null" {
		s = "{}"
	}
	p.payload["customAttributes"] = s
	delete(p.payload, "customClaims")
	return nil
}

// Validators.

// No validation needed. Used for bool fields.
func validateTrue(p *commonParams, _ string) error {
	return nil
}

func validateNonEmpty(p *commonParams, fieldName string) error {
	if val, ok := p.payload[fieldName]; ok {
		if len(val.(string)) == 0 {
			return fmt.Errorf("%s must be a non-empty string", fieldName)
		}
	}
	return nil
}

func validateEmail(p *commonParams, fieldName string) error {
	if val, ok := p.payload[fieldName]; ok {
		if parts := strings.Split(val.(string), "@"); len(parts) != 2 || len(parts[0]) == 0 || len(parts[1]) == 0 {
			return fmt.Errorf("malformed Email string: %q", val)
		}
	}
	return nil
}

func validatePassword(p *commonParams, fieldName string) error {
	wantLength := 6
	if val, ok := p.payload[fieldName]; ok {
		if len(val.(string)) < wantLength {
			return fmt.Errorf("Password must be a string at least %d characters long", wantLength)
		}
	}
	return nil
}

func validateUID(p *commonParams, fieldName string) error {
	wantLength := 128
	if val, ok := p.payload[fieldName]; ok {
		if len(val.(string)) > wantLength {
			return fmt.Errorf("localId must be a string at most %d characters long", wantLength)
		}
	}
	return nil
}

func validateCustomAttributes(p *commonParams, fieldName string) error {
	wantLength := maxLenPayloadCC
	if val, ok := p.payload[fieldName]; ok {
		if len(val.(string)) > wantLength {
			return fmt.Errorf("CustomClaims must be a string at most %d characters long", wantLength)
		}
	}
	return nil
}

func validatePhone(p *commonParams, fieldName string) error {
	if val, ok := p.payload[fieldName]; ok {
		if !regexp.MustCompile(`\+.*[0-9A-Za-z]`).MatchString(val.(string)) {
			return fmt.Errorf(
				"invalid PhoneNumber %q. Must be a valid, E.164 compliant identifier", val)
		}
	}
	return nil
}

func (p *UserToCreate) preparePayload() (map[string]interface{}, error) {
	if p.payload == nil {
		p.payload = map[string]interface{}{}
	}

	for _, test := range commonValidators {
		if err := test.testFun(&p.commonParams, test.fieldName); err != nil {
			return nil, err
		}
	}
	return p.payload, nil
}

func (p *UserToUpdate) preparePayload() (map[string]interface{}, error) {
	if p.payload == nil {
		return nil, fmt.Errorf("update with no params") // This is caught in the caller.
	}
	procs := append(updatePreProcess, commonValidators...)
	procs = append(procs, updateValidators...)

	for _, test := range procs {
		if err := test.testFun(&p.commonParams, test.fieldName); err != nil {
			return nil, err
		}
	}
	return p.payload, nil
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
		return nil, fmt.Errorf("expecting only one user, got %d, %v ", l, params)
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
