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
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"time"

	"firebase.google.com/go/internal"
	"golang.org/x/net/context"

	"google.golang.org/api/googleapi"
	"google.golang.org/api/identitytoolkit/v3"
	"google.golang.org/api/iterator"
)

const maxReturnedResults = 1000
const maxLenPayloadCC = 1000

const defaultProviderID = "firebase"

var commonValidators = map[string]func(interface{}) error{
	"displayName": validateDisplayName,
	"email":       validateEmail,
	"phoneNumber": validatePhone,
	"password":    validatePassword,
	"photoUrl":    validatePhotoURL,
	"localId":     validateUID,
	"validSince":  func(interface{}) error { return nil }, // Needed for preparePayload.
}

// Create a new interface
type identitytoolkitCall interface {
	Header() http.Header
}

// set header
func (c *Client) setHeader(ic identitytoolkitCall) {
	ic.Header().Set("X-Client-Version", c.version)
}

// UserInfo is a collection of standard profile information for a user.
type UserInfo struct {
	DisplayName string
	Email       string
	PhoneNumber string
	PhotoURL    string
	// In the ProviderUserInfo[] ProviderID can be a short domain name (e.g. google.com),
	// or the identity of an OpenID identity provider.
	// In UserRecord.UserInfo it will return the constant string "firebase".
	ProviderID string
	UID        string
}

// UserMetadata contains additional metadata associated with a user account.
// Timestamps are in milliseconds since epoch.
type UserMetadata struct {
	CreationTimestamp  int64
	LastLogInTimestamp int64
}

// UserRecord contains metadata associated with a Firebase user account.
type UserRecord struct {
	*UserInfo
	CustomClaims           map[string]interface{}
	Disabled               bool
	EmailVerified          bool
	ProviderUserInfo       []*UserInfo
	TokensValidAfterMillis int64 // milliseconds since epoch.
	UserMetadata           *UserMetadata
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

// UserToCreate is the parameter struct for the CreateUser function.
type UserToCreate struct {
	params map[string]interface{}
}

func (u *UserToCreate) set(key string, value interface{}) {
	if u.params == nil {
		u.params = make(map[string]interface{})
	}
	u.params[key] = value
}

// Disabled setter.
func (u *UserToCreate) Disabled(d bool) *UserToCreate { u.set("disabled", d); return u }

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
	params map[string]interface{}
}

func (u *UserToUpdate) set(key string, value interface{}) {
	if u.params == nil {
		u.params = make(map[string]interface{})
	}
	u.params[key] = value
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

// revokeRefreshTokens revokes all refresh tokens for a user by setting the validSince property
// to the present in epoch seconds.
func (u *UserToUpdate) revokeRefreshTokens() *UserToUpdate {
	u.set("validSince", time.Now().Unix())
	return u
}

// CreateUser creates a new user with the specified properties.
func (c *Client) CreateUser(ctx context.Context, user *UserToCreate) (*UserRecord, error) {
	uid, err := c.createUser(ctx, user)
	if err != nil {
		return nil, err
	}
	return c.GetUser(ctx, uid)
}

// UpdateUser updates an existing user account with the specified properties.
//
// DisplayName, PhotoURL and PhoneNumber will be set to "" to signify deleting them from the record.
func (c *Client) UpdateUser(ctx context.Context, uid string, user *UserToUpdate) (ur *UserRecord, err error) {
	if err := c.updateUser(ctx, uid, user); err != nil {
		return nil, err
	}
	return c.GetUser(ctx, uid)
}

// DeleteUser deletes the user by the given UID.
func (c *Client) DeleteUser(ctx context.Context, uid string) error {
	if err := validateUID(uid); err != nil {
		return err
	}
	request := &identitytoolkit.IdentitytoolkitRelyingpartyDeleteAccountRequest{
		LocalId: uid,
	}

	call := c.is.Relyingparty.DeleteAccount(request)
	c.setHeader(call)
	if _, err := call.Context(ctx).Do(); err != nil {
		return handleServerError(err)
	}
	return nil
}

// GetUser gets the user data corresponding to the specified user ID.
func (c *Client) GetUser(ctx context.Context, uid string) (*UserRecord, error) {
	if err := validateUID(uid); err != nil {
		return nil, err
	}
	request := &identitytoolkit.IdentitytoolkitRelyingpartyGetAccountInfoRequest{
		LocalId: []string{uid},
	}
	return c.getUser(ctx, request)
}

// GetUserByPhoneNumber gets the user data corresponding to the specified user phone number.
func (c *Client) GetUserByPhoneNumber(ctx context.Context, phone string) (*UserRecord, error) {
	if err := validatePhone(phone); err != nil {
		return nil, err
	}
	request := &identitytoolkit.IdentitytoolkitRelyingpartyGetAccountInfoRequest{
		PhoneNumber: []string{phone},
	}
	return c.getUser(ctx, request)
}

// GetUserByEmail gets the user data corresponding to the specified email.
func (c *Client) GetUserByEmail(ctx context.Context, email string) (*UserRecord, error) {
	if err := validateEmail(email); err != nil {
		return nil, err
	}
	request := &identitytoolkit.IdentitytoolkitRelyingpartyGetAccountInfoRequest{
		Email: []string{email},
	}
	return c.getUser(ctx, request)
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
	request := &identitytoolkit.IdentitytoolkitRelyingpartyDownloadAccountRequest{
		MaxResults:    int64(pageSize),
		NextPageToken: pageToken,
	}
	call := it.client.is.Relyingparty.DownloadAccount(request)
	it.client.setHeader(call)
	resp, err := call.Context(it.ctx).Do()
	if err != nil {
		return "", handleServerError(err)
	}

	for _, u := range resp.Users {
		eu, err := makeExportedUser(u)
		if err != nil {
			return "", err
		}
		it.users = append(it.users, eu)
	}
	it.pageInfo.Token = resp.NextPageToken
	return resp.NextPageToken, nil
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
	return c.updateUser(ctx, uid, (&UserToUpdate{}).CustomClaims(customClaims))
}

func processDeletion(p map[string]interface{}, field, listKey, listVal string) {
	if dn, ok := p[field]; ok && len(dn.(string)) == 0 {
		addToListParam(p, listKey, listVal)
		delete(p, field)
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
	cc, ok := p["customClaims"]
	if !ok {
		return nil
	}

	claims := cc.(map[string]interface{})
	for _, key := range reservedClaims {
		if _, ok := claims[key]; ok {
			return fmt.Errorf("claim %q is reserved and must not be set", key)
		}
	}

	b, err := json.Marshal(claims)
	if err != nil {
		return fmt.Errorf("custom claims marshaling error: %v", err)
	}
	s := string(b)
	if s == "null" {
		s = "{}"
	}
	if len(s) > maxLenPayloadCC {
		return fmt.Errorf("serialized custom claims must not exceed %d characters", maxLenPayloadCC)
	}

	p["customAttributes"] = s
	delete(p, "customClaims")
	return nil
}

// Error handlers.

const (
	emailAlredyExists        = "email-already-exists"
	idTokenRevoked           = "id-token-revoked"
	insufficientPermission   = "insufficient-permission"
	phoneNumberAlreadyExists = "phone-number-already-exists"
	projectNotFound          = "project-not-found"
	uidAlreadyExists         = "uid-already-exists"
	unknown                  = "unknown-error"
	userNotFound             = "user-not-found"
)

// IsEmailAlreadyExists checks if the given error was due to a duplicate email.
func IsEmailAlreadyExists(err error) bool {
	return internal.HasErrorCode(err, emailAlredyExists)
}

// IsIDTokenRevoked checks if the given error was due to a revoked ID token.
func IsIDTokenRevoked(err error) bool {
	return internal.HasErrorCode(err, idTokenRevoked)
}

// IsInsufficientPermission checks if the given error was due to insufficient permissions.
func IsInsufficientPermission(err error) bool {
	return internal.HasErrorCode(err, insufficientPermission)
}

// IsPhoneNumberAlreadyExists checks if the given error was due to a duplicate phone number.
func IsPhoneNumberAlreadyExists(err error) bool {
	return internal.HasErrorCode(err, phoneNumberAlreadyExists)
}

// IsProjectNotFound checks if the given error was due to a non-existing project.
func IsProjectNotFound(err error) bool {
	return internal.HasErrorCode(err, projectNotFound)
}

// IsUIDAlreadyExists checks if the given error was due to a duplicate uid.
func IsUIDAlreadyExists(err error) bool {
	return internal.HasErrorCode(err, uidAlreadyExists)
}

// IsUnknown checks if the given error was due to a unknown server error.
func IsUnknown(err error) bool {
	return internal.HasErrorCode(err, unknown)
}

// IsUserNotFound checks if the given error was due to non-existing user.
func IsUserNotFound(err error) bool {
	return internal.HasErrorCode(err, userNotFound)
}

var serverError = map[string]string{
	"CONFIGURATION_NOT_FOUND": projectNotFound,
	"DUPLICATE_EMAIL":         emailAlredyExists,
	"DUPLICATE_LOCAL_ID":      uidAlreadyExists,
	"EMAIL_EXISTS":            emailAlredyExists,
	"INSUFFICIENT_PERMISSION": insufficientPermission,
	"PHONE_NUMBER_EXISTS":     phoneNumberAlreadyExists,
	"PROJECT_NOT_FOUND":       projectNotFound,
}

func handleServerError(err error) error {
	gerr, ok := err.(*googleapi.Error)
	if !ok {
		// Not a back-end error
		return err
	}
	serverCode := gerr.Message
	clientCode, ok := serverError[serverCode]
	if !ok {
		clientCode = unknown
	}
	return internal.Error(clientCode, err.Error())
}

// Validators.

func validateDisplayName(val interface{}) error {
	if len(val.(string)) == 0 {
		return fmt.Errorf("display name must be a non-empty string")
	}
	return nil
}

func validatePhotoURL(val interface{}) error {
	if len(val.(string)) == 0 {
		return fmt.Errorf("photo url must be a non-empty string")
	}
	return nil
}

func validateEmail(val interface{}) error {
	email := val.(string)
	if email == "" {
		return fmt.Errorf("email must not be empty")
	}
	if parts := strings.Split(email, "@"); len(parts) != 2 || len(parts[0]) == 0 || len(parts[1]) == 0 {
		return fmt.Errorf("malformed email string: %q", email)
	}
	return nil
}

func validatePassword(val interface{}) error {
	if len(val.(string)) < 6 {
		return fmt.Errorf("password must be a string at least 6 characters long")
	}
	return nil
}

func validateUID(val interface{}) error {
	uid := val.(string)
	if uid == "" {
		return fmt.Errorf("uid must not be empty")
	}
	if len(val.(string)) > 128 {
		return fmt.Errorf("uid string must not be longer than 128 characters")
	}
	return nil
}

func validatePhone(val interface{}) error {
	phone := val.(string)
	if phone == "" {
		return fmt.Errorf("phone number must not be empty")
	}
	if !regexp.MustCompile(`\+.*[0-9A-Za-z]`).MatchString(phone) {
		return fmt.Errorf("phone number must be a valid, E.164 compliant identifier")
	}
	return nil
}

func (u *UserToCreate) preparePayload(user *identitytoolkit.IdentitytoolkitRelyingpartySignupNewUserRequest) error {
	params := map[string]interface{}{}
	if u.params == nil {
		return nil
	}

	for k, v := range u.params {
		params[k] = v
	}
	for key, validate := range commonValidators {
		if v, ok := params[key]; ok {
			if err := validate(v); err != nil {
				return err
			}
			reflect.ValueOf(user).Elem().FieldByName(strings.Title(key)).SetString(params[key].(string))
		}
	}
	if params["disabled"] != nil {
		user.Disabled = params["disabled"].(bool)
		if !user.Disabled {
			user.ForceSendFields = append(user.ForceSendFields, "Disabled")
		}
	}
	if params["emailVerified"] != nil {
		user.EmailVerified = params["emailVerified"].(bool)
		if !user.EmailVerified {
			user.ForceSendFields = append(user.ForceSendFields, "EmailVerified")
		}
	}

	return nil
}

func (u *UserToUpdate) preparePayload(user *identitytoolkit.IdentitytoolkitRelyingpartySetAccountInfoRequest) error {
	params := map[string]interface{}{}
	for k, v := range u.params {
		params[k] = v
	}
	processDeletion(params, "displayName", "deleteAttribute", "DISPLAY_NAME")
	processDeletion(params, "photoUrl", "deleteAttribute", "PHOTO_URL")
	processDeletion(params, "phoneNumber", "deleteProvider", "phone")

	if err := processClaims(params); err != nil {
		return err
	}

	if params["customAttributes"] != nil {
		user.CustomAttributes = params["customAttributes"].(string)
	}

	for key, validate := range commonValidators {
		if v, ok := params[key]; ok {
			if err := validate(v); err != nil {
				return err
			}
			f := reflect.ValueOf(user).Elem().FieldByName(strings.Title(key))
			if f.Kind() == reflect.String {
				f.SetString(params[key].(string))
			} else if f.Kind() == reflect.Int64 {
				f.SetInt(params[key].(int64))
			}
		}
	}
	if params["disableUser"] != nil {
		user.DisableUser = params["disableUser"].(bool)
		if !user.DisableUser {
			user.ForceSendFields = append(user.ForceSendFields, "DisableUser")
		}
	}
	if params["emailVerified"] != nil {
		user.EmailVerified = params["emailVerified"].(bool)
		if !user.EmailVerified {
			user.ForceSendFields = append(user.ForceSendFields, "EmailVerified")
		}
	}
	if params["deleteAttribute"] != nil {
		user.DeleteAttribute = params["deleteAttribute"].([]string)
	}
	if params["deleteProvider"] != nil {
		user.DeleteProvider = params["deleteProvider"].([]string)
	}

	return nil
}

// End of validators

// Helper functions for retrieval and HTTP calls.

func (c *Client) createUser(ctx context.Context, user *UserToCreate) (string, error) {
	if user == nil {
		user = &UserToCreate{}
	}

	request := &identitytoolkit.IdentitytoolkitRelyingpartySignupNewUserRequest{}

	if err := user.preparePayload(request); err != nil {
		return "", err
	}

	call := c.is.Relyingparty.SignupNewUser(request)
	c.setHeader(call)
	resp, err := call.Context(ctx).Do()
	if err != nil {
		return "", handleServerError(err)
	}

	return resp.LocalId, nil
}

func (c *Client) updateUser(ctx context.Context, uid string, user *UserToUpdate) error {
	if err := validateUID(uid); err != nil {
		return err
	}
	if user == nil || user.params == nil {
		return fmt.Errorf("update parameters must not be nil or empty")
	}
	request := &identitytoolkit.IdentitytoolkitRelyingpartySetAccountInfoRequest{
		LocalId: uid,
	}

	if err := user.preparePayload(request); err != nil {
		return err
	}

	call := c.is.Relyingparty.SetAccountInfo(request)
	c.setHeader(call)
	if _, err := call.Context(ctx).Do(); err != nil {
		return handleServerError(err)
	}
	return nil
}

func (c *Client) getUser(ctx context.Context, request *identitytoolkit.IdentitytoolkitRelyingpartyGetAccountInfoRequest) (*UserRecord, error) {
	call := c.is.Relyingparty.GetAccountInfo(request)
	c.setHeader(call)
	resp, err := call.Context(ctx).Do()
	if err != nil {
		return nil, handleServerError(err)
	}
	if len(resp.Users) == 0 {
		var msg string
		if len(request.LocalId) == 1 {
			msg = fmt.Sprintf("cannot find user from uid: %q", request.LocalId[0])
		} else if len(request.Email) == 1 {
			msg = fmt.Sprintf("cannot find user from email: %q", request.Email[0])
		} else {
			msg = fmt.Sprintf("cannot find user from phone number: %q", request.PhoneNumber[0])
		}
		return nil, internal.Error(userNotFound, msg)
	}

	eu, err := makeExportedUser(resp.Users[0])
	if err != nil {
		return nil, err
	}
	return eu.UserRecord, nil
}

func makeExportedUser(r *identitytoolkit.UserInfo) (*ExportedUserRecord, error) {
	var cc map[string]interface{}
	if r.CustomAttributes != "" {
		if err := json.Unmarshal([]byte(r.CustomAttributes), &cc); err != nil {
			return nil, err
		}
		if len(cc) == 0 {
			cc = nil
		}
	}

	var providerUserInfo []*UserInfo
	for _, u := range r.ProviderUserInfo {
		info := &UserInfo{
			DisplayName: u.DisplayName,
			Email:       u.Email,
			PhoneNumber: u.PhoneNumber,
			PhotoURL:    u.PhotoUrl,
			ProviderID:  u.ProviderId,
			UID:         u.RawId,
		}
		providerUserInfo = append(providerUserInfo, info)
	}

	resp := &ExportedUserRecord{
		UserRecord: &UserRecord{
			UserInfo: &UserInfo{
				DisplayName: r.DisplayName,
				Email:       r.Email,
				PhoneNumber: r.PhoneNumber,
				PhotoURL:    r.PhotoUrl,
				ProviderID:  defaultProviderID,
				UID:         r.LocalId,
			},
			CustomClaims:           cc,
			Disabled:               r.Disabled,
			EmailVerified:          r.EmailVerified,
			ProviderUserInfo:       providerUserInfo,
			TokensValidAfterMillis: r.ValidSince * 1000,
			UserMetadata: &UserMetadata{
				LastLogInTimestamp: r.LastLoginAt,
				CreationTimestamp:  r.CreatedAt,
			},
		},
		PasswordHash: r.PasswordHash,
		PasswordSalt: r.Salt,
	}
	return resp, nil
}
