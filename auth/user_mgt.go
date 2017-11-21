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

// ExportedUserRecord is the returned user value used when listing all the users.
type ExportedUserRecord struct {
	*UserRecord
	PasswordHash string
	PasswordSalt string
}

// UserParams encapsulates the named calling params for CreateUser and UpdateUser
// Update User also calls a distinct UID field, the one in the struct must match or be empty
type UserParams struct {
	CustomClaims  *map[string]interface{} `json:"-"` // https://play.golang.org/p/JB1_jHu1mm
	Disabled      *bool                   `json:"disabled,omitempty"`
	DisplayName   *string                 `json:"displayName,omitempty"`
	Email         *string                 `json:"email,omitempty"`
	EmailVerified *bool                   `json:"emailVerified,omitempty"`
	Password      *string                 `json:"password,omitempty"`
	PhoneNumber   *string                 `json:"phoneNumber,omitempty"`
	PhotoURL      *string                 `json:"photoURL,omitempty"`
	UID           *string                 `json:"localId,omitempty"`
}

func (up *userParams) setClaimsField() error {
	if up.CustomClaims == nil {
		return nil
	}
	b, err := json.Marshal(*up.CustomClaims)
	if err != nil {
		return err
	}
	up.CustomAttributes = string(b)
	return nil
}

// CreateUser creates a new user with the specified properties.
func (c *Client) CreateUser(ctx context.Context, p *UserParams) (ur *UserRecord, err error) {
	if p == nil {
		p = &UserParams{}
	}
	up := &userParams{UserParams: p}
	up.setClaimsField()
	return c.updateCreateUser(ctx, "signupNewUser", up)
}

type userParams struct {
	*UserParams
	DeleteAttributeList []string `json:"deleteAttribute,omitempty"`
	DeleteProviderList  []string `json:"deleteProvider,omitempty"`
	CustomAttributes    string   `json:"customAttributes,omitempty"`
}

// UpdateUser updates a user with the given params
// DisplayName, PhotoURL and PhoneNumber will be set to "" to signify deleting them from the record
// nil pointers in the UserParams will remain unchanged.
func (c *Client) UpdateUser(ctx context.Context, uid string, params *UserParams) (ur *UserRecord, err error) {
	if uid == "" {
		return nil, fmt.Errorf("uid must not be empty")
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

	return c.updateCreateUser(ctx, "setAccountInfo", up)
}
func isEmptyString(ps *string) bool {
	return ps != nil && *ps == ""
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
	if up.CustomAttributes == "" {
		return ptr.String("attributes were not set, for non nil custom claims")
	}
	return validateStringLenLTE(&up.CustomAttributes, "stringified JSON claims", 1000)
}
func validatePhoneNumber(phone *string) *string {
	if phone == nil {
		return nil
	}
	if !strings.HasPrefix(*phone, "+") {
		return ptr.String("phone # must begin with a +")
	}
	isAlphaNum := regexp.MustCompile(`[0-9A-Za-z]`).MatchString
	if !isAlphaNum(*phone) {
		return ptr.String("phone # must contain an alphanumeric character")
	}
	return nil
}

func validated(up *userParams) (bool, error) {
	errors := []*string{
		validateCustomClaims(up),
		validatePhoneNumber(up.PhoneNumber),

		validateStringLenGTE(up.Password, "password", 6),
		validateStringLenLTE(up.UID, "uid", 128),
		validateStringLenGTE(up.UID, "uid", 0),
		validateStringLenGTE(up.DisplayName, "displayName", 0),
		validateStringLenGTE(up.PhotoURL, "photoURL", 0),

		validateStringLenGTE(up.Email, "email", 0),
		validateString(up.Email,
			func(s string) bool { return strings.Count(s, "@") == 1 },
			"email must contain exactly one '@' sign"),
		validateString(up.Email,
			func(s string) bool { return strings.Index(s, "@") > 0 && strings.LastIndex(s, "@") < (len(s)-1) },
			"email must have non empty account and domain"),
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

// DeleteUser deletes the user by the given UID
func (c *Client) DeleteUser(ctx context.Context, uid string) error {

	_, err := c.makeUserRequest(ctx, "deleteAccount",
		map[string]interface{}{"localId": []string{uid}})
	return err
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
	PhotoURL           string      `json:"photoURL,omitempty"`
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
	cc := make(map[string]interface{})
	if rur.CustomClaims != "" {
		err := json.Unmarshal([]byte(rur.CustomClaims), &cc)
		if err != nil {
			return nil, err
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
			CustomClaims:     &cc,
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

func (it *UserIterator) fetch(pageSize int, pageToken string) (string, error) {
	params := map[string]interface{}{"maxResults": pageSize}
	if pageToken != "" {
		params["nextPageToken"] = pageToken
	}
	resp, err := it.client.makeUserRequest(it.ctx, "downloadAccount", params)
	if err != nil {
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
