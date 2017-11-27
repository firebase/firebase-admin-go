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

	"firebase.google.com/go/ptr"
	"google.golang.org/api/iterator"

	"golang.org/x/net/context"
)

const maxResults = 1000

// UserInfo A collection of standard profile information for a user.
//
// Used to expose profile information returned by an identity provider.
type UserInfo struct {
	DisplayName string `json:"displayName,omitempty"`
	Email       string `json:"email,omitempty"`
	PhoneNumber string `json:"phoneNumber,omitempty"`
	PhotoURL    string `json:"photoUrl,omitempty"`
	// ProviderID can be short domain name (e.g. google.com),
	// or the identity of an OpenID identity provider.
	ProviderID string `json:"providerId,omitempty"`
	UID        string `json:"localId,omitempty"`
}

//UserMetadata contains additional metadata associated with a user account.
type UserMetadata struct {
	CreationTimestamp  int64
	LastLogInTimestamp int64
}

// UserRecord contains metadata associated with a Firebase user account.
type UserRecord struct {
	*UserInfo
	CustomClaims     *map[string]interface{}
	Disabled         bool
	EmailVerified    bool
	ProviderUserInfo []*UserInfo
	UserMetadata     *UserMetadata
}

// UserParams encapsulates the named calling params for CreateUser and UpdateUser
// Update User also calls a distinct UID field, the one in the struct must match or be empty
type UserParams struct {
	CustomClaims  *map[string]interface{} `json:"-"` // https://play.golang.org/p/JB1_jHu1mm
	Disabled      *bool                   `json:"disableUser,omitempty"`
	DisplayName   *string                 `json:"displayName,omitempty"`
	Email         *string                 `json:"email,omitempty"`
	EmailVerified *bool                   `json:"emailVerified,omitempty"`
	Password      *string                 `json:"password,omitempty"`
	PhoneNumber   *string                 `json:"phoneNumber,omitempty"`
	PhotoURL      *string                 `json:"photoUrl,omitempty"`
	UID           *string                 `json:"localId,omitempty"`
}

// userParams, is the iternal struct that will be passed on to the create function
type userParams struct {
	*UserParams
	DeleteAttributeList []string `json:"deleteAttribute,omitempty"`
	DeleteProviderList  []string `json:"deleteProvider,omitempty"`
	CustomAttributes    *string  `json:"customAttributes,omitempty"`
}

// CreateUser creates a new user with the specified properties.
func (c *Client) CreateUser(ctx context.Context, p *UserParams) (ur *UserRecord, err error) {
	if p == nil {
		p = &UserParams{}
	}

	var cc *map[string]interface{}
	if p.CustomClaims != nil {
		cc = p.CustomClaims
		p.CustomClaims = nil
	}
	up := &userParams{UserParams: p}
	up.setClaimsField()

	u, err := c.updateCreateUser(ctx, "signupNewUser", up)
	if err != nil {
		return nil, err
	}
	if cc != nil {
		err = c.SetCustomUserClaims(ctx, u.UID, cc)
		if err != nil {
			return nil, err
		}
		expu, err := c.GetUser(ctx, u.UID)
		return expu.UserRecord, err
	}
	return u, err
}

// UpdateUser updates a user with the given params
// DisplayName, PhotoURL and PhoneNumber will be set to "" to signify deleting them from the record
// nil pointers in the UserParams will remain unchanged.
func (c *Client) UpdateUser(ctx context.Context, uid string, params *UserParams) (ur *UserRecord, err error) {
	if uid == "" {
		return nil, fmt.Errorf("uid must not be empty")
	}
	if params == nil {
		params = &UserParams{}
	}
	if params.UID != nil && *params.UID != uid {
		return nil, fmt.Errorf("uid mismatch")
	}
	params.UID = &uid
	up := &userParams{
		UserParams: params,
	}
	up.setClaimsField()
	up.setDeleteFields()
	//
	return c.updateCreateUser(ctx, "setAccountInfo", up)
}
func isEmptyString(ps *string) bool {
	return ps != nil && *ps == ""
}

// DeleteUser deletes the user by the given UID
func (c *Client) DeleteUser(ctx context.Context, uid string) error {

	_, err := c.makeUserRequest(ctx, "deleteAccount",
		map[string]interface{}{"localId": []string{uid}})
	return err
}

// ExportedUserRecord is the returned user value used when listing all the users.
type ExportedUserRecord struct {
	*UserRecord
	PasswordHash string
	PasswordSalt string
}

//GetUser returns the user by UID
func (c *Client) GetUser(ctx context.Context, uid string) (*ExportedUserRecord, error) {
	return c.getUser(ctx, map[string]interface{}{"localId": []string{uid}})
}

//GetUserByPhone returns the user by phone number
func (c *Client) GetUserByPhone(ctx context.Context, phone string) (*ExportedUserRecord, error) {
	return c.getUser(ctx, map[string]interface{}{"phoneNumber": []string{phone}})
}

//GetUserByEmail returns the user by the email
func (c *Client) GetUserByEmail(ctx context.Context, email string) (*ExportedUserRecord, error) {
	return c.getUser(ctx, map[string]interface{}{"email": []string{email}})
}

// UserIterator is the struct behind the Users Iterator
// https://github.com/GoogleCloudPlatform/google-cloud-go/wiki/Iterator-Guidelines
type UserIterator struct {
	client   *Client
	ctx      context.Context
	nextFunc func() error
	pageInfo *iterator.PageInfo
	users    []*ExportedUserRecord
}

// Users returns an iterator over the Users
func (c *Client) Users(ctx context.Context, startToken string) *UserIterator {
	it := &UserIterator{
		ctx:    ctx,
		client: c,
	}
	it.pageInfo, it.nextFunc = iterator.NewPageInfo(
		it.fetch,
		func() int { return len(it.users) },
		func() interface{} { b := it.users; it.users = nil; return b })
	it.pageInfo.MaxSize = maxResults
	it.pageInfo.Token = startToken
	return it
}

// PageInfo supports pagination. See the google.golang.org/api/iterator package for details.
// This makes UserIterator comply with the Pageable interface.
func (it *UserIterator) PageInfo() *iterator.PageInfo { return it.pageInfo }

// Next returns the next result. Its second return value is iterator.Done if
// there are no more results. Once Next returns iterator.Done, all subsequent
// calls will return iterator.Done.
func (it *UserIterator) Next() (*ExportedUserRecord, error) {
	if err := it.nextFunc(); err != nil {
		return nil, err
	}
	user := it.users[0]
	it.users = it.users[1:]
	return user, nil
}

// SetCustomUserClaims sets the user claims (received as a *map[string]:interface{})
func (c *Client) SetCustomUserClaims(ctx context.Context, uid string, customClaims *map[string]interface{}) error {
	if customClaims == nil || *customClaims == nil || len(*customClaims) == 0 {
		customClaims = &map[string]interface{}{}
	}
	ur, err := c.UpdateUser(ctx, uid, &UserParams{CustomClaims: customClaims})
	if err != nil {
		return err
	}
	if ur.UID == uid {
		return nil
	}

	return fmt.Errorf("uid mismatch on returned user")
}

func (c *Client) getUser(ctx context.Context, params map[string]interface{}) (*ExportedUserRecord, error) {
	resp, err := c.makeUserRequest(ctx, "getAccountInfo", params)
	if err != nil {
		return nil, err
	}
	var gur getUserResponse
	err = json.Unmarshal(resp, &gur)
	if err != nil {
		return nil, err
	}
	if len(gur.Users) == 0 {
		return nil, fmt.Errorf("cannot find user %v", params)
	}
	return makeExportedUser(gur.Users[0])
}

func (up *userParams) setDeleteFields() {
	var deleteProvList, deleteAttrList []string
	if isEmptyString(up.DisplayName) {
		deleteAttrList = append(deleteAttrList, "DISPLAY_NAME")
		up.DisplayName = nil
	}
	if isEmptyString(up.PhotoURL) {
		deleteAttrList = append(deleteAttrList, "PHOTO_URL")
		up.PhotoURL = nil
	}
	if isEmptyString(up.PhoneNumber) {
		deleteProvList = append(deleteProvList, "phone")
		up.PhoneNumber = nil
	}
	if deleteAttrList != nil {
		up.DeleteAttributeList = deleteAttrList
	}
	if deleteProvList != nil {
		up.DeleteProviderList = deleteProvList
	}
}

func (up *userParams) setClaimsField() error {
	if up.CustomClaims == nil {
		return nil
	}
	b, err := json.Marshal(*up.CustomClaims)
	if err != nil {
		return err
	}
	s := string(b)
	if *up.CustomClaims == nil || len(*up.CustomClaims) == 0 {
		s = "{}"
		up.CustomClaims = nil
	}

	up.CustomAttributes = &s
	return nil
}

func (c *Client) updateCreateUser(ctx context.Context, action string, params *userParams) (ur *UserRecord, err error) {
	//	return nil, nil
	if ok, err := validated(params); !ok || err != nil {
		return nil, err
	}

	resp, err := c.makeUserRequest(ctx, action, params)
	if err != nil {
		return nil, fmt.Errorf("bad request %s, %s", string(resp), err)
	}

	jsonMap, err := parseResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("bad json %s, %s", string(resp), err)
	}
	uid := jsonMap["localId"].(string)

	user, err := c.GetUser(ctx, uid)
	if err != nil {
		return nil, err
	}
	return user.UserRecord, nil
	/**/
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
	CustomClaims       string      `json:"customAttributes,omitempty"` // https://play.golang.org/p/JB1_jHu1mm
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

func makeExportedUser(rur responseUserRecord) (*ExportedUserRecord, error) {
	var ccp *map[string]interface{}
	if rur.CustomClaims != "" {
		var cc map[string]interface{}
		err := json.Unmarshal([]byte(rur.CustomClaims), &cc)
		if err != nil {
			return nil, err
		}
		if len(cc) > 0 {
			ccp = &cc
		}
	}

	resp := &ExportedUserRecord{
		UserRecord: &UserRecord{
			UserInfo: &UserInfo{
				DisplayName: rur.DisplayName,
				Email:       rur.Email,
				PhoneNumber: rur.PhoneNumber,
				PhotoURL:    rur.PhotoURL,
				ProviderID:  rur.ProviderID,
				UID:         rur.UID,
			},
			CustomClaims:     ccp,
			Disabled:         rur.Disabled,
			EmailVerified:    rur.EmailVerified,
			ProviderUserInfo: rur.ProviderUserInfo,
			UserMetadata: &UserMetadata{
				LastLogInTimestamp: rur.LastLogInTimestamp,
				CreationTimestamp:  rur.CreationTimestamp,
			},
		},
		PasswordHash: rur.PasswordHash,
		PasswordSalt: rur.PasswordSalt,
	}
	return resp, nil
}

func (it *UserIterator) fetch(pageSize int, pageToken string) (string, error) {
	params := map[string]interface{}{"maxResults": pageSize}
	if pageToken != "" {
		params["nextPageToken"] = pageToken
	}
	resp, err := it.client.makeUserRequest(it.ctx, "downloadAccount", params)
	if err != nil {
		// remove this line before submission ,see b/69406469
		if pageToken != "" &&
			strings.Contains(err.Error(), "\"code\": 400") &&
			strings.Contains(err.Error(), "\"message\": \"INVALID_PAGE_SELECTION\"") {
			return it.fetch(pageSize, "")
		}
		return "", err
	}
	var lur listUsersResponse
	err = json.Unmarshal(resp, &lur)
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

func validateString(s *string, condition func(string) bool, message string) *string {
	if s == nil || condition(*s) {
		return nil
	}
	return &message
}
func validateStringLenGTE(s *string, name string, length int) *string {
	return validateString(s, func(st string) bool { return len(st) >= length }, fmt.Sprintf("%s must be at least %d chars long", name, length))
}

func validateStringLenLTE(s *string, name string, length int) *string {
	return validateString(s, func(st string) bool { return len(st) <= length }, fmt.Sprintf("%s must be at most %d chars long", name, length))
}

func validateCustomClaims(up *userParams) *string {
	if up.CustomClaims == nil {
		return nil
	}
	cc := up.CustomClaims
	for _, key := range reservedClaims {
		if _, ok := (*cc)[key]; ok {
			return ptr.String(key + " is a reserved claim")
		}
	}
	/*	if up.CustomAttributes == "" {
		return ptr.String("attributes were not set, for non nil custom claims")
	}*/
	return validateStringLenLTE(up.CustomAttributes, "stringified JSON claims", 1000)
}
func validatePhoneNumber(phone *string) *string {
	if phone == nil {
		return nil
	}
	if len(*phone) == 0 {
		return ptr.String("PhoneNumber cannot be empty")
	}
	if !strings.HasPrefix(*phone, "+") {
		return ptr.String("PhoneNumber must begin with a +")
	}
	isAlphaNum := regexp.MustCompile(`[0-9A-Za-z]`).MatchString
	if !isAlphaNum(*phone) {
		return ptr.String("PhoneNumber must contain an alphanumeric character")
	}
	return nil
}

func validateEmail(email *string) *string {
	if empty := validateStringLenGTE(email, "Email", 1); empty != nil {
		return empty
	}
	if noAt := validateString(email,
		func(s string) bool { return strings.Count(s, "@") == 1 },
		"Email must contain exactly one '@' sign"); noAt != nil {
		return noAt
	}
	return validateString(email,
		func(s string) bool { return strings.Index(s, "@") > 0 && strings.LastIndex(s, "@") < (len(s)-1) },
		"Email must have non empty account and domain")
}

func validateUID(uid *string) *string {

	if tooLong := validateStringLenLTE(uid, "UID", 128); tooLong != nil {
		return tooLong
	}
	return validateStringLenGTE(uid, "UID", 1)

}
func validated(up *userParams) (bool, error) {
	errors := []*string{
		validateCustomClaims(up),
		validatePhoneNumber(up.PhoneNumber),
		validateEmail(up.Email),
		validateStringLenGTE(up.Password, "Password", 6),
		validateUID(up.UID),
		validateStringLenGTE(up.DisplayName, "DisplayName", 1),
		validateStringLenGTE(up.PhotoURL, "PhotoURL", 1),
	}
	var res []string
	for _, e := range errors {
		if e != nil {
			res = append(res, *e)
		}
	}
	if res == nil {
		return true, nil
	}

	return false, fmt.Errorf("error in params: %s", strings.Join(res, ", "))
}
