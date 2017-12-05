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
		errorName string
		testFun   func(*commonParams, string, string)
	}{
		{"displayName", "DisplayName", processDeletion},
		{"phoneNumber", "PhoneNumber", processDeletion},
		{"photoUrl", "PhotoURL", processDeletion},
		{"customClaims", "CustomClaims", processClaims},
	}
	deletionSpecs = map[string]struct {
		deleteListName  string
		deleteFieldName string
	}{
		"displayName": {"deleteAttribute", "DISPLAY_NAME"},
		"phoneNumber": {"deleteProvider", "phone"},
		"photoUrl":    {"deleteAttribute", "PHOTO_URL"},
	}

	commonValidators = []struct {
		fieldName string
		errorName string
		testFun   func(*commonParams, string, string)
	}{
		{"disableUser", "Disabled", allowed},
		{"displayName", "DisplayName", nonEmpty},
		{"email", "Email", nonEmpty},
		{"email", "Email", validEmail},
		{"emailVerified", "EmailVerified", allowed},
		{"password", "Password", strlenGTE(6)},
		{"phoneNumber", "PhoneNumber", validPhone},
		{"phoneNumber", "PhoneNumber", nonEmpty},
		{"photoUrl", "PhotoUrl", nonEmpty},
		{"localId ", "UID", strlenLTE(128)},
		{"localId ", "UID", nonEmpty},
	}

	updateValidators = []struct {
		fieldName string
		errorName string
		testFun   func(*commonParams, string, string)
	}{
		{"customAttributes", "CustomClaims", strlenLTE(maxLenPayloadCC)},
	}
	createValidators = []struct {
		fieldName string
		errorName string
		testFun   func(*commonParams, string, string)
	}{}
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

type userParams interface {
	preparePayload()
}
type commonParams struct {
	payload map[string]interface{}
	errors  []string
}

// UserToCreate is the parameter struct for the CreateUser function.
type UserToCreate struct {
	commonParams
}

// UserToUpdate is the parameter struct for the UpdateUser function.
type UserToUpdate struct {
	commonParams
}

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

// CreateUser creates a new user with the specified properties.
func (c *Client) CreateUser(ctx context.Context, params *UserToCreate) (*UserRecord, error) {
	if params == nil {
		params = &UserToCreate{}
	}
	params.preparePayload()
	if len(params.errors) > 0 {
		return nil, fmt.Errorf(strings.Join(params.errors, ", "))
	}
	u, err := c.createOrUpdateUser(ctx, "signupNewUser", params.payload)
	if err != nil {
		return nil, err
	}
	return c.GetUser(ctx, u.UID)
}

// UpdateUser updates an existing user account with the specified properties.
//
// DisplayName, PhotoURL and PhoneNumber will be set to "" to signify deleting them from the record.
func (c *Client) UpdateUser(ctx context.Context, uid string, params *UserToUpdate) (ur *UserRecord, err error) {
	if uid == "" {
		return nil, fmt.Errorf("uid must not be empty")
	}
	if params == nil {
		return nil, fmt.Errorf("params must not be empty for update")
	}
	params.payload["localId"] = uid
	params.preparePayload()

	if len(params.errors) > 0 {
		return nil, fmt.Errorf(strings.Join(params.errors, ", "))
	}
	return c.createOrUpdateUser(ctx, "setAccountInfo", params.payload)
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
	if err != nil {
		return err
	}
	return err
}

// ------------------------------------------------------------
// Setters and utilities for Create and Update input structs.

func (p *commonParams) appendErrString(s string, i ...interface{}) {
	p.errors = append(p.errors, fmt.Sprintf(s, i...))
}

func (p *commonParams) payloadInitialized() {
	if p.payload == nil {
		p.payload = make(map[string]interface{})
	}
}

func (p *commonParams) set(key string, value interface{}) {
	p.payloadInitialized()
	p.payload[key] = value
}

// assumes that payloadName is a string field in p.payload
func processDeletion(p *commonParams, payloadName, errorName string) {
	if dn, ok := p.payload[payloadName]; ok && len(dn.(string)) == 0 {
		p.addToListParam(deletionSpecs[payloadName].deleteListName, deletionSpecs[payloadName].deleteFieldName)
		delete(p.payload, payloadName)
	}
}

func processClaims(p *commonParams, payloadName, errorName string) {
	if _, ok := p.payload[payloadName]; !ok {
		return
	}
	p.checkReservedClaims()
	p.setCustomAttributes()
}
func (p *commonParams) checkReservedClaims() {
	cc := p.payload["customClaims"]
	switch claims := cc.(type) {
	case map[string]interface{}:
		for _, key := range reservedClaims {
			if _, ok := claims[key]; ok {
				p.appendErrString("claim %q is reserved, and must not be set", key)
			}
		}
	}
}

func (p *commonParams) setCustomAttributes() {
	cc := p.payload["customClaims"]
	b, err := json.Marshal(cc)
	if err != nil {
		p.appendErrString("invalid custom claims Marshaling error: %v wanted %v", err, cc)
		return
	} //else if len(b) > maxLenPayloadCC {
	//		p.appendErrString("Custom Claims payload must not exceed %d characters", maxLenPayloadCC)
	//	} else {
	s := string(b)
	if s == "null" {
		s = "{}"
	}
}

func (p *commonParams) addToListParam(listname, param string) {
	if _, ok := p.payload[listname]; ok {
		p.payload[listname] = append(p.payload[listname].([]string), param)
	} else {
		p.set(listname, []string{param})
	}
}

// Validators.

// No validation needed. used for bool fields
func allowed(p *commonParams, _, _ string) {
	return
}

func nonEmpty(p *commonParams, fieldName, errorName string) {
	if val, ok := p.payload[fieldName]; ok {
		if len(val.(string)) == 0 {
			p.appendErrString("invalid %s:: %q. The %s must be a non-empty string", errorName, val)
		}
	}
}

func validEmail(p *commonParams, fieldName, errorName string) {
	if val, ok := p.payload[fieldName]; ok {
		if parts := strings.Split(val.(string), "@"); len(parts) != 2 || len(parts[0]) == 0 || len(parts[1]) == 0 {
			p.appendErrString("malformed %s string: %q", val, errorName)
		}
	}
}

func strlenGTE(i int) func(*commonParams, string, string) {
	return func(p *commonParams, fieldName, errorName string) {
		if val, ok := p.payload[fieldName]; ok {
			if len(val.(string)) < i {
				p.appendErrString("invalid %s string. %s must be a string at least %d characters long", errorName, errorName, i)
			}
		}
	}
}

func strlenLTE(i int) func(*commonParams, string, string) {
	return func(p *commonParams, fieldName, errorName string) {
		if val, ok := p.payload[fieldName]; ok {
			if len(val.(string)) > i {
				p.appendErrString("%s must be a string at most %d characters long", errorName, i)
			}
		}
	}
}
func validPhone(p *commonParams, fieldName, errorName string) {
	if val, ok := p.payload[fieldName]; ok {
		if !regexp.MustCompile(`\+.*[0-9A-Za-z]`).MatchString(val.(string)) {
			p.appendErrString("invalid %s: %q. %s must be a valid, E.164 compliant identifier", errorName, val, errorName)
		}
	}
}

func (p *UserToCreate) preparePayload() {
	if p.payload == nil {
		p.payload = map[string]interface{}{}
	}
	for _, test := range append(commonValidators, createValidators...) {
		test.testFun(&p.commonParams, test.fieldName, test.errorName)
	}
}

func (p *UserToUpdate) preparePayload() {
	if p.payload == nil {
		p.payload = map[string]interface{}{}
	}
	for _, proc := range updatePreProcess {
		proc.testFun(&p.commonParams, proc.fieldName, proc.errorName)
	}
	for _, test := range append(commonValidators, updateValidators...) {
		test.testFun(&p.commonParams, test.fieldName, test.errorName)
	}
}

// ------  Disabled: ------------------------------
func (p *commonParams) setDisabled(d bool) {
	p.set("disableUser", d)
}

// Disabled field setter.
func (p *UserToCreate) Disabled(d bool) *UserToCreate {
	p.setDisabled(d)
	return p
}

// Disabled field setter.
func (p *UserToUpdate) Disabled(d bool) *UserToUpdate {
	p.setDisabled(d)
	return p
}

// ------  DisplayName: ------------------------------
func (p *commonParams) setDisplayName(dn string) {
	p.set("displayName", dn)
}

// DisplayName field setter.
func (p *UserToCreate) DisplayName(dn string) *UserToCreate {
	/*	if len(dn) == 0 {

		} else {*/
	p.setDisplayName(dn)
	//}
	return p
}

// DisplayName field setter.
func (p *UserToUpdate) DisplayName(dn string) *UserToUpdate {
	p.setDisplayName(dn)
	/*
		if len(dn) == 0 {
			p.addToListParam("deleteAttribute", "DISPLAY_NAME")
		} else {
			p.set("displayName", dn)
		}*/
	return p
}

// ------  Email: ------------------------------

func (p *commonParams) setEmail(e string) {
	/*if len(e) == 0 {
		p.appendErrString(`invalid Email: %q Email must be a non-empty string`, e)
	} else if parts := strings.Split(e, "@"); len(parts) != 2 || len(parts[0]) == 0 || len(parts[1]) == 0 {
		p.appendErrString(`malformed email address string: %q`, e)
	} else {
	*/
	p.set("email", e)
	//}
}

// Email field setter.
func (p *UserToCreate) Email(e string) *UserToCreate {
	p.setEmail(e)
	return p
}

// Email field setter.
func (p *UserToUpdate) Email(e string) *UserToUpdate {
	p.setEmail(e)
	return p
}

// ------  EmailVerified: ------------------------------

func (p *commonParams) setEmailVerified(ev bool) {
	p.set("emailVerified", ev)
}

// EmailVerified field setter.
func (p *UserToCreate) EmailVerified(ev bool) *UserToCreate {
	p.setEmailVerified(ev)
	return p
}

// EmailVerified field setter.
func (p *UserToUpdate) EmailVerified(ev bool) *UserToUpdate {
	p.setEmailVerified(ev)
	return p
}

// ------  Password: ------------------------------

func (p *commonParams) setPassword(pw string) {
	/*if len(pw) < 6 {
		p.appendErrString("invalid Password string. Password must be a string at least 6 characters long")
	} else {
	/*/
	p.set("password", pw)
	//}
}

// Password field setter.
func (p *UserToCreate) Password(pw string) *UserToCreate {
	p.setPassword(pw)
	return p
}

// Password field setter.
func (p *UserToUpdate) Password(pw string) *UserToUpdate {
	p.setPassword(pw)
	return p
}

// ------  PhoneNumber: ------------------------------
func (p *commonParams) setPhoneNumber(pn string) {
	p.set("phoneNumber", pn)
}

// PhoneNumber field setter.
func (p *UserToCreate) PhoneNumber(phone string) *UserToCreate {
	/*	if len(phone) == 0 {
			p.appendErrString(`invalid PhoneNumber: %q. PhoneNumber must be a non-empty string`, phone)
		} else if !regexp.MustCompile(`\+.*[0-9A-Za-z]`).MatchString(phone) {
			p.appendErrString(`invalid phone number: %q. Phone number must be a valid, E.164 compliant identifier`, phone)
		} else {*/
	p.setPhoneNumber(phone)
	//}
	return p
}

// PhoneNumber field setter.
func (p *UserToUpdate) PhoneNumber(phone string) *UserToUpdate {
	/*if len(phone) > 0 && !regexp.MustCompile(`\+.*[0-9A-Za-z]`).MatchString(phone) {
		p.appendErrString(`invalid phone number: %q. Phone number must be a valid, E.164 compliant identifier`, phone)
	} else if len(phone) == 0 {
		p.addToListParam("deleteProvider", "phone")
	} else {
		p.set("phoneNumber", phone)
	}*/
	p.setPhoneNumber(phone)

	return p
}

// ------  PhoneNumber: ------------------------------
func (p *commonParams) setPhotoURL(pu string) {

}

// PhotoURL field setter.
func (p *UserToCreate) PhotoURL(url string) *UserToCreate {
	/*	if len(url) == 0 {
		p.appendErrString(`invalid photo URL: %q. PhotoURL must be a non-empty string`, url)
	} else {*/
	p.setPhotoURL(url)

	//	p.set("photoUrl", url)
	return p
}

// PhotoURL field setter.
func (p *UserToUpdate) PhotoURL(url string) *UserToUpdate {
	/*if len(url) == 0 {
		p.addToListParam("deleteAttribute", "PHOTO_URL")
	} else {
	}*/
	p.setPhotoURL(url)
	return p
}

// UID field setter ------------------------------
func (p *UserToCreate) UID(uid string) *UserToCreate {
	/*	if len(uid) == 0 || len(uid) > 128 {
		p.appendErrString(`invalid uid: %q. The uid must be a non-empty string with no more than 128 characters`, uid)
	}*/
	p.set("localId", uid)
	return p
}

// CustomClaims setter: ------------------------------
func (p *UserToUpdate) CustomClaims(cc map[string]interface{}) *UserToUpdate {
	/*	if cc == nil {
			cc = make(map[string]interface{})
		}
		for _, key := range reservedClaims {
			if _, ok := cc[key]; ok {
				p.appendErrString(`claim %q is reserved, and must not be set`, key)
			}
		}
		b, err := json.Marshal(cc)
		if err != nil {
			p.appendErrString("invalid custom claims Marshaling error: %v", err)
		} else if len(b) > maxLenPayloadCC {
			p.appendErrString(`Custom Claims payload must not exceed %d characters`, maxLenPayloadCC)
		} else {
			s := string(b)
			if cc == nil || len(cc) == 0 {
				s = "{}"
			}*/
	p.set("customClaims", cc)
	//}
	return p
}

// ------------------------------------------------------------
// ------------------------------------------------------------ End of setters

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
