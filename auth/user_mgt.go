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

	"firebase.google.com/go/p"
	"google.golang.org/api/iterator"

	"golang.org/x/net/context"
)

const maxResults = 1000

// CustomClaimsMap is an alias for readability.
type CustomClaimsMap map[string]interface{}

// UserCreateParams encapsulates the named calling params for CreateUser
// the json tags are used to convert this to the format expected by the API with the right names
type UserCreateParams struct {
	CustomClaims  *CustomClaimsMap `json:"customAttributes,omitempty"` // https://play.golang.org/p/JB1_jHu1mm
	Disabled      *bool            `json:"disabled,omitempty"`
	DisplayName   *string          `json:"displayName,omitempty"`
	Email         *string          `json:"email,omitempty"`
	EmailVerified *bool            `json:"emailVerified,omitempty"`
	Password      *string          `json:"password,omitempty"`
	PhoneNumber   *string          `json:"phoneNumber,omitempty"`
	PhotoURL      *string          `json:"photoURL,omitempty"`
	UID           *string          `json:"localId,omitempty"`
}

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
	CustomClaims     map[string]string
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

// CreateUser creates a new user with the specified properties.
func (c *Client) CreateUser(ctx context.Context, params *UserCreateParams) (ur *UserRecord, err error) {
	if params == nil {
		params = &UserCreateParams{}
	}
	return c.updateCreateUser(ctx, "signupNewUser", params)
}

// UserUpdateParams encapsulates the named calling params for UpdateUser
// the json tags are used to convert this to the format expected by the API with the right names
// This struct will be amended with other data before the call.
// DisplayName, PhotoURL and PhoneNumber will be set to "" to signify deleting them from the record
// nil pointers will remain unchanged.
type UserUpdateParams struct {
	CustomClaims  *CustomClaimsMap `json:"customAttributes,omitempty"` // https://play.golang.org/p/JB1_jHu1mm
	Disabled      *bool            `json:"disabled,omitempty"`
	DisplayName   *string          `json:"displayName,omitempty"`
	Email         *string          `json:"email,omitempty"`
	EmailVerified *bool            `json:"emailVerified,omitempty"`
	Password      *string          `json:"password,omitempty"`
	PhoneNumber   *string          `json:"phoneNumber,omitempty"`
	PhotoURL      *string          `json:"photoURL,omitempty"`
}

type userUpdateParams struct {
	*UserUpdateParams
	UID                 *string  `json:"localId,omitempty"`
	DeleteAttributeList []string `json:"deleteAttribute,omitempty"`
	DeleteProviderList  []string `json:"deleteProvider,omitempty"`
}

// UpdateUser updates a user with the given params
// DisplayName, PhotoURL and PhoneNumber will be set to "" to signify deleting them from the record
// nil pointers in the UserUpdateParams will remain unchanged.
func (c *Client) UpdateUser(ctx context.Context, uid string, params *UserUpdateParams) (ur *UserRecord, err error) {
	up := &userUpdateParams{
		UserUpdateParams: params,
		UID:              p.String(uid),
	}
	up.setDeleteFields()

	return c.updateCreateUser(ctx, "setAccountInfo", up)
}
func isEmptyString(ps *string) bool {
	return ps != nil && *ps == ""
}
func (up *userUpdateParams) setDeleteFields() {
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

type userParams interface {
	getUID() *string
	getDisplayName() *string
	getDisabled() *bool
	getEmail() *string
	getEmailVerified() *bool
	getCustomClaims() *CustomClaimsMap
	getPassword() *string
	getPhotoURL() *string
	getPhoneNumber() *string
}

func (cp *UserCreateParams) getUID() *string { return cp.UID }
func (up *userUpdateParams) getUID() *string { return up.UID }

func (cp *UserCreateParams) getDisplayName() *string { return cp.DisplayName }
func (up *userUpdateParams) getDisplayName() *string { return up.DisplayName }

func (cp *UserCreateParams) getDisabled() *bool { return cp.Disabled }
func (up *userUpdateParams) getDisabled() *bool { return up.Disabled }

func (cp *UserCreateParams) getEmail() *string { return cp.Email }
func (up *userUpdateParams) getEmail() *string { return up.Email }

func (cp *UserCreateParams) getEmailVerified() *bool { return cp.EmailVerified }
func (up *userUpdateParams) getEmailVerified() *bool { return up.EmailVerified }

func (cp *UserCreateParams) getCustomClaims() *CustomClaimsMap { return cp.CustomClaims }
func (up *userUpdateParams) getCustomClaims() *CustomClaimsMap { return up.CustomClaims }

func (cp *UserCreateParams) getPassword() *string { return cp.Password }
func (up *userUpdateParams) getPassword() *string { return up.Password }

func (cp *UserCreateParams) getPhotoURL() *string { return cp.PhotoURL }
func (up *userUpdateParams) getPhotoURL() *string { return up.PhotoURL }

func (cp *UserCreateParams) getPhoneNumber() *string { return cp.PhoneNumber }
func (up *userUpdateParams) getPhoneNumber() *string { return up.PhoneNumber }

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

func validateCustomClaims(cc *CustomClaimsMap) *string {
	if cc == nil {
		return nil
	}
	for _, key := range reservedClaims {
		if _, ok := (*cc)[key]; !ok {
			return p.String(key + " is a reserved claim")
		}
	}
	b, err := json.Marshal(*cc)
	if err != nil {
		return p.String(fmt.Sprintf("can't convert claims to json %v", *cc))
	}
	if len(b) > 1000 {
		return p.String("length of custom claims cannot exceed 1000 chars")
	}
	return nil
}
func validatePhoneNumber(phone *string) *string {
	if phone == nil {
		return nil
	}
	if !strings.HasPrefix(*phone, "+") {
		return p.String("phone # must begin with a +")
	}
	isAlphaNum := regexp.MustCompile(`[0-9A-Za-z]`).MatchString
	if !isAlphaNum(*phone) {
		return p.String("phone # must contain an alphanumeric character")
	}
	return nil
}

func validated(up userParams) (bool, error) {
	errors := []*string{
		validateCustomClaims(up.getCustomClaims()),
		validatePhoneNumber(up.getPhoneNumber()),

		validateStringLenGTE(up.getPassword(), "password", 6),
		validateStringLenLTE(up.getUID(), "uid", 128),
		validateStringLenGTE(up.getUID(), "uid", 0),
		validateStringLenGTE(up.getDisplayName(), "displayName", 0),
		validateStringLenGTE(up.getPhotoURL(), "photoURL", 0),

		validateStringLenGTE(up.getEmail(), "email", 0),
		validateString(up.getEmail(),
			func(s string) bool { return strings.Count(s, "@") == 1 },
			"email must contain exactly one '@' sign"),
		validateString(up.getEmail(),
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
func (c *Client) updateCreateUser(ctx context.Context, action string, p userParams) (ur *UserRecord, err error) {
	//	return nil, nil
	if ok, err := validated(p); !ok || err != nil {
		return nil, err
	}
	resp, err := c.makeUserRequest(ctx, action, p)

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
	UID                string            `json:"localId,omitempty"`
	DisplayName        string            `json:"displayName,omitempty"`
	Email              string            `json:"email,omitempty"`
	PhoneNumber        string            `json:"phoneNumber,omitempty"`
	PhotoURL           string            `json:"photoURL,omitempty"`
	CreationTimestamp  int64             `json:"createdAt,string,omitempty"`
	LastLogInTimestamp int64             `json:"lastLoginAt,string,omitempty"`
	ProviderID         string            `json:"providerId,omitempty"`
	CustomClaims       map[string]string `json:"customAttributes,omitempty"` // https://play.golang.org/p/JB1_jHu1mm
	Disabled           bool              `json:"disabled,omitempty"`
	EmailVerified      bool              `json:"emailVerified,omitempty"`
	ProviderUserInfo   []*UserInfo       `json:"providerMata,omitempty"`
	PasswordHash       string            `json:"passwordHash,omitempty"`
	PasswordSalt       string            `json:"salt,omitempty"`
	ValidSince         int64             `json:"validSince,string,omitempty"`
}

type listUsersResponse struct {
	RequestType string               `json:"kind,omitempty"`
	Users       []responseUserRecord `json:"users,omitempty"`
	NextPage    string               `json:"nextPageToken,omitempty"`
}

func makeExportedUser(rur responseUserRecord) *ExportedUserRecord {
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
			CustomClaims:     rur.CustomClaims,
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
	return resp
}

func (c *Client) getUser(ctx context.Context, m map[string]interface{}) (*ExportedUserRecord, error) {
	resp, err := c.makeUserRequest(ctx, "getAccountInfo", m)
	if err != nil {
		return nil, err
	}
	var gur getUserResponse
	err = json.Unmarshal(resp, &gur)
	if err != nil {
		return nil, err
	}
	if len(gur.Users) == 0 {
		return nil, fmt.Errorf("cannot find user %v", m)
	}
	return makeExportedUser(gur.Users[0]), nil
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

// SetCustomClaims sets the custom user claims, the json []byte of the custom claims map cannot exceed 1000 chars in length
func (c *Client) SetCustomClaims(ctx context.Context, uid string, claims *CustomClaimsMap) error {
	_, err := c.UpdateUser(ctx, uid, &UserUpdateParams{CustomClaims: claims})
	return err
}

// // // /// / /// // // // // / // / // / /// / / // / // / /  / // //

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
func (c *Client) Users(ctx context.Context, opts ...func(u *UserIterator)) *UserIterator {
	it := &UserIterator{
		ctx:    ctx,
		client: c,
	}
	it.pageInfo, it.nextFunc = iterator.NewPageInfo(
		it.fetch,
		func() int { return len(it.users) },
		func() interface{} { b := it.users; it.users = nil; return b })
	it.pageInfo.MaxSize = maxResults
	for _, opt := range opts {
		opt(it)
	}
	return it
}

// WithPageToken can be used concatenated to the Users constructor or it can be applied later. can be chained.
func WithPageToken(token string) func(u *UserIterator) {
	return func(u *UserIterator) { u.pageInfo.Token = token }
}

// WithMaxSize can be used concatenated to the Users constructor or it can be applied later. can be chained.
func WithMaxSize(size int) func(u *UserIterator) {
	return func(u *UserIterator) { u.pageInfo.MaxSize = size }
}

func (it *UserIterator) fetch(pageSize int, pageToken string) (string, error) {
	payload := map[string]interface{}{"maxResults": pageSize}
	if len(pageToken) > 0 {
		payload["nextPageToken"] = pageToken
	}
	resp, err := it.client.makeUserRequest(
		it.ctx,
		"downloadAccount",
		payload)
	if err != nil {
		return "", err
	}
	var lur listUsersResponse
	err = json.Unmarshal(resp, &lur)
	if err != nil {
		return "", err
	}
	usersList := make([]*ExportedUserRecord, 0)
	for _, u := range lur.Users {
		usersList = append(usersList, makeExportedUser(u))
	}
	it.users = usersList
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
