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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"firebase.google.com/go/internal"
	"google.golang.org/api/googleapi"
)

const (
	maxLenPayloadCC   = 1000
	defaultProviderID = "firebase"
)

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
	DisplayName string `json:"displayName,omitempty"`
	Email       string `json:"email,omitempty"`
	PhoneNumber string `json:"phoneNumber,omitempty"`
	PhotoURL    string `json:"photoUrl,omitempty"`
	// In the ProviderUserInfo[] ProviderID can be a short domain name (e.g. google.com),
	// or the identity of an OpenID identity provider.
	// In UserRecord.UserInfo it will return the constant string "firebase".
	ProviderID string `json:"providerId,omitempty"`
	UID        string `json:"rawId,omitempty"`
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

// UserToCreate is the parameter struct for the CreateUser function.
type UserToCreate struct {
	params map[string]interface{}
}

// Disabled setter.
func (u *UserToCreate) Disabled(disabled bool) *UserToCreate {
	return u.set("disabled", disabled)
}

// DisplayName setter.
func (u *UserToCreate) DisplayName(name string) *UserToCreate {
	return u.set("displayName", name)
}

// Email setter.
func (u *UserToCreate) Email(email string) *UserToCreate {
	return u.set("email", email)
}

// EmailVerified setter.
func (u *UserToCreate) EmailVerified(verified bool) *UserToCreate {
	return u.set("emailVerified", verified)
}

// Password setter.
func (u *UserToCreate) Password(pw string) *UserToCreate {
	return u.set("password", pw)
}

// PhoneNumber setter.
func (u *UserToCreate) PhoneNumber(phone string) *UserToCreate {
	return u.set("phoneNumber", phone)
}

// PhotoURL setter.
func (u *UserToCreate) PhotoURL(url string) *UserToCreate {
	return u.set("photoUrl", url)
}

// UID setter.
func (u *UserToCreate) UID(uid string) *UserToCreate {
	return u.set("localId", uid)
}

func (u *UserToCreate) set(key string, value interface{}) *UserToCreate {
	if u.params == nil {
		u.params = make(map[string]interface{})
	}
	u.params[key] = value
	return u
}

func (u *UserToCreate) validatedRequest() (map[string]interface{}, error) {
	req := make(map[string]interface{})
	for k, v := range u.params {
		req[k] = v
	}

	if uid, ok := req["localId"]; ok {
		if err := validateUID(uid.(string)); err != nil {
			return nil, err
		}
	}
	if name, ok := req["displayName"]; ok {
		if err := validateDisplayName(name.(string)); err != nil {
			return nil, err
		}
	}
	if email, ok := req["email"]; ok {
		if err := validateEmail(email.(string)); err != nil {
			return nil, err
		}
	}
	if phone, ok := req["phoneNumber"]; ok {
		if err := validatePhone(phone.(string)); err != nil {
			return nil, err
		}
	}
	if url, ok := req["photoUrl"]; ok {
		if err := validatePhotoURL(url.(string)); err != nil {
			return nil, err
		}
	}
	if pw, ok := req["password"]; ok {
		if err := validatePassword(pw.(string)); err != nil {
			return nil, err
		}
	}

	return req, nil
}

// UserToUpdate is the parameter struct for the UpdateUser function.
type UserToUpdate struct {
	params map[string]interface{}
}

// CustomClaims setter.
func (u *UserToUpdate) CustomClaims(claims map[string]interface{}) *UserToUpdate {
	return u.set("customClaims", claims)
}

// Disabled setter.
func (u *UserToUpdate) Disabled(disabled bool) *UserToUpdate {
	return u.set("disableUser", disabled)
}

// DisplayName setter. Set to empty string to remove the display name from the user account.
func (u *UserToUpdate) DisplayName(name string) *UserToUpdate {
	return u.set("displayName", name)
}

// Email setter.
func (u *UserToUpdate) Email(email string) *UserToUpdate {
	return u.set("email", email)
}

// EmailVerified setter.
func (u *UserToUpdate) EmailVerified(verified bool) *UserToUpdate {
	return u.set("emailVerified", verified)
}

// Password setter.
func (u *UserToUpdate) Password(pw string) *UserToUpdate {
	return u.set("password", pw)
}

// PhoneNumber setter. Set to empty string to remove the phone number and the corresponding auth provider
// from the user account.
func (u *UserToUpdate) PhoneNumber(phone string) *UserToUpdate {
	return u.set("phoneNumber", phone)
}

// PhotoURL setter. Set to empty string to remove the photo URL from the user account.
func (u *UserToUpdate) PhotoURL(url string) *UserToUpdate {
	return u.set("photoUrl", url)
}

// revokeRefreshTokens revokes all refresh tokens for a user by setting the validSince property
// to the present in epoch seconds.
func (u *UserToUpdate) revokeRefreshTokens() *UserToUpdate {
	return u.set("validSince", strconv.FormatInt(time.Now().Unix(), 10))
}

func (u *UserToUpdate) set(key string, value interface{}) *UserToUpdate {
	if u.params == nil {
		u.params = make(map[string]interface{})
	}
	u.params[key] = value
	return u
}

func (u *UserToUpdate) validatedRequest() (map[string]interface{}, error) {
	if len(u.params) == 0 {
		// update without any parameters is never allowed
		return nil, fmt.Errorf("update parameters must not be nil or empty")
	}

	req := make(map[string]interface{})
	for k, v := range u.params {
		req[k] = v
	}

	if email, ok := req["email"]; ok {
		if err := validateEmail(email.(string)); err != nil {
			return nil, err
		}
	}

	handleDeletion := func(key, deleteKey, deleteVal string) {
		var deleteList []string
		list, ok := req[deleteKey]
		if ok {
			deleteList = list.([]string)
		}
		req[deleteKey] = append(deleteList, deleteVal)
		delete(req, key)
	}

	if name, ok := req["displayName"]; ok {
		if name == "" {
			handleDeletion("displayName", "deleteAttribute", "DISPLAY_NAME")
		} else if err := validateDisplayName(name.(string)); err != nil {
			return nil, err
		}
	}

	if url, ok := req["photoUrl"]; ok {
		if url == "" {
			handleDeletion("photoUrl", "deleteAttribute", "PHOTO_URL")
		} else if err := validatePhotoURL(url.(string)); err != nil {
			return nil, err
		}
	}

	if phone, ok := req["phoneNumber"]; ok {
		if phone == "" {
			handleDeletion("phoneNumber", "deleteProvider", "phone")
		} else if err := validatePhone(phone.(string)); err != nil {
			return nil, err
		}
	}

	if claims, ok := req["customClaims"]; ok {
		cc, err := marshalCustomClaims(claims.(map[string]interface{}))
		if err != nil {
			return nil, err
		}
		req["customAttributes"] = cc
		delete(req, "customClaims")
	}

	if pw, ok := req["password"]; ok {
		if err := validatePassword(pw.(string)); err != nil {
			return nil, err
		}
	}
	return req, nil
}

// RevokeRefreshTokens revokes all refresh tokens issued to a user.
//
// RevokeRefreshTokens updates the user's TokensValidAfterMillis to the current UTC second.
// It is important that the server on which this is called has its clock set correctly and synchronized.
//
// While this revokes all sessions for a specified user and disables any new ID tokens for existing sessions
// from getting minted, existing ID tokens may remain active until their natural expiration (one hour).
// To verify that ID tokens are revoked, use `verifyIdTokenAndCheckRevoked(ctx, idToken)`.
func (c *Client) RevokeRefreshTokens(ctx context.Context, uid string) error {
	return c.updateUser(ctx, uid, (&UserToUpdate{}).revokeRefreshTokens())
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

func marshalCustomClaims(claims map[string]interface{}) (string, error) {
	for _, key := range reservedClaims {
		if _, ok := claims[key]; ok {
			return "", fmt.Errorf("claim %q is reserved and must not be set", key)
		}
	}

	b, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("custom claims marshaling error: %v", err)
	}
	s := string(b)
	if s == "null" {
		s = "{}" // claims map has been explicitly set to nil for deletion.
	}
	if len(s) > maxLenPayloadCC {
		return "", fmt.Errorf("serialized custom claims must not exceed %d characters", maxLenPayloadCC)
	}
	return s, nil
}

// Error handlers.

const (
	emailAlreadyExists       = "email-already-exists"
	idTokenRevoked           = "id-token-revoked"
	insufficientPermission   = "insufficient-permission"
	invalidDynamicLinkDomain = "invalid-dynamic-link-domain"
	phoneNumberAlreadyExists = "phone-number-already-exists"
	projectNotFound          = "project-not-found"
	sessionCookieRevoked     = "session-cookie-revoked"
	uidAlreadyExists         = "uid-already-exists"
	unauthorizedContinueURI  = "unauthorized-continue-uri"
	unknown                  = "unknown-error"
	userNotFound             = "user-not-found"
)

// IsEmailAlreadyExists checks if the given error was due to a duplicate email.
func IsEmailAlreadyExists(err error) bool {
	return internal.HasErrorCode(err, emailAlreadyExists)
}

// IsIDTokenRevoked checks if the given error was due to a revoked ID token.
func IsIDTokenRevoked(err error) bool {
	return internal.HasErrorCode(err, idTokenRevoked)
}

// IsInsufficientPermission checks if the given error was due to insufficient permissions.
func IsInsufficientPermission(err error) bool {
	return internal.HasErrorCode(err, insufficientPermission)
}

// IsInvalidDynamicLinkDomain checks if the given error was due to an invalid dynamic link domain.
func IsInvalidDynamicLinkDomain(err error) bool {
	return internal.HasErrorCode(err, invalidDynamicLinkDomain)
}

// IsPhoneNumberAlreadyExists checks if the given error was due to a duplicate phone number.
func IsPhoneNumberAlreadyExists(err error) bool {
	return internal.HasErrorCode(err, phoneNumberAlreadyExists)
}

// IsProjectNotFound checks if the given error was due to a non-existing project.
func IsProjectNotFound(err error) bool {
	return internal.HasErrorCode(err, projectNotFound)
}

// IsSessionCookieRevoked checks if the given error was due to a revoked session cookie.
func IsSessionCookieRevoked(err error) bool {
	return internal.HasErrorCode(err, sessionCookieRevoked)
}

// IsUIDAlreadyExists checks if the given error was due to a duplicate uid.
func IsUIDAlreadyExists(err error) bool {
	return internal.HasErrorCode(err, uidAlreadyExists)
}

// IsUnauthorizedContinueURI checks if the given error was due to an unauthorized continue URI domain.
func IsUnauthorizedContinueURI(err error) bool {
	return internal.HasErrorCode(err, unauthorizedContinueURI)
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
	"CONFIGURATION_NOT_FOUND":     projectNotFound,
	"DUPLICATE_EMAIL":             emailAlreadyExists,
	"DUPLICATE_LOCAL_ID":          uidAlreadyExists,
	"EMAIL_EXISTS":                emailAlreadyExists,
	"INSUFFICIENT_PERMISSION":     insufficientPermission,
	"INVALID_DYNAMIC_LINK_DOMAIN": invalidDynamicLinkDomain,
	"PERMISSION_DENIED":           insufficientPermission,
	"PHONE_NUMBER_EXISTS":         phoneNumberAlreadyExists,
	"PROJECT_NOT_FOUND":           projectNotFound,
	"UNAUTHORIZED_DOMAIN":         unauthorizedContinueURI,
	"USER_NOT_FOUND":              userNotFound,
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

func validateDisplayName(val string) error {
	if val == "" {
		return fmt.Errorf("display name must be a non-empty string")
	}
	return nil
}

func validatePhotoURL(val string) error {
	if val == "" {
		return fmt.Errorf("photo url must be a non-empty string")
	}
	return nil
}

func validateEmail(email string) error {
	if email == "" {
		return fmt.Errorf("email must be a non-empty string")
	}
	if parts := strings.Split(email, "@"); len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("malformed email string: %q", email)
	}
	return nil
}

func validatePassword(val string) error {
	if len(val) < 6 {
		return fmt.Errorf("password must be a string at least 6 characters long")
	}
	return nil
}

func validateUID(uid string) error {
	if uid == "" {
		return fmt.Errorf("uid must be a non-empty string")
	}
	if len(uid) > 128 {
		return fmt.Errorf("uid string must not be longer than 128 characters")
	}
	return nil
}

func validatePhone(phone string) error {
	if phone == "" {
		return fmt.Errorf("phone number must be a non-empty string")
	}
	if !regexp.MustCompile(`\+.*[0-9A-Za-z]`).MatchString(phone) {
		return fmt.Errorf("phone number must be a valid, E.164 compliant identifier")
	}
	return nil
}

// End of validators

const idToolkitEndpoint = "https://identitytoolkit.googleapis.com/v1/projects"

// userManagementClient is a helper for interacting with the Identity Toolkit REST API.
type userManagementClient struct {
	baseURL    string
	projectID  string
	version    string
	httpClient *internal.HTTPClient
}

// GetUser gets the user data corresponding to the specified user ID.
func (c *userManagementClient) GetUser(ctx context.Context, uid string) (*UserRecord, error) {
	return c.getUser(ctx, &userQuery{
		field: "localId",
		value: uid,
		label: "uid",
	})
}

// GetUserByEmail gets the user data corresponding to the specified email.
func (c *userManagementClient) GetUserByEmail(ctx context.Context, email string) (*UserRecord, error) {
	if err := validateEmail(email); err != nil {
		return nil, err
	}
	return c.getUser(ctx, &userQuery{
		field: "email",
		value: email,
	})
}

// GetUserByPhoneNumber gets the user data corresponding to the specified user phone number.
func (c *userManagementClient) GetUserByPhoneNumber(ctx context.Context, phone string) (*UserRecord, error) {
	if err := validatePhone(phone); err != nil {
		return nil, err
	}
	return c.getUser(ctx, &userQuery{
		field: "phoneNumber",
		value: phone,
		label: "phone number",
	})
}

type userQuery struct {
	field string
	value string
	label string
}

func (q *userQuery) description() string {
	label := q.label
	if label == "" {
		label = q.field
	}
	return fmt.Sprintf("%s: %q", label, q.value)
}

func (q *userQuery) build() map[string]interface{} {
	return map[string]interface{}{
		q.field: []string{q.value},
	}
}

func (c *userManagementClient) getUser(ctx context.Context, query *userQuery) (*UserRecord, error) {
	resp, err := c.post(ctx, "/accounts:lookup", query.build())
	if err != nil {
		return nil, err
	}

	if resp.Status != http.StatusOK {
		return nil, handleHTTPError(resp)
	}

	var parsed struct {
		Users []*userQueryResponse `json:"users"`
	}
	if err := json.Unmarshal(resp.Body, &parsed); err != nil {
		return nil, err
	}

	if len(parsed.Users) == 0 {
		return nil, internal.Errorf(userNotFound, "cannot find user from %s", query.description())
	}

	return parsed.Users[0].makeUserRecord()
}

type userQueryResponse struct {
	UID                string      `json:"localId,omitempty"`
	DisplayName        string      `json:"displayName,omitempty"`
	Email              string      `json:"email,omitempty"`
	PhoneNumber        string      `json:"phoneNumber,omitempty"`
	PhotoURL           string      `json:"photoUrl,omitempty"`
	CreationTimestamp  int64       `json:"createdAt,string,omitempty"`
	LastLogInTimestamp int64       `json:"lastLoginAt,string,omitempty"`
	ProviderID         string      `json:"providerId,omitempty"`
	CustomAttributes   string      `json:"customAttributes,omitempty"`
	Disabled           bool        `json:"disabled,omitempty"`
	EmailVerified      bool        `json:"emailVerified,omitempty"`
	ProviderUserInfo   []*UserInfo `json:"providerUserInfo,omitempty"`
	PasswordHash       string      `json:"passwordHash,omitempty"`
	PasswordSalt       string      `json:"salt,omitempty"`
	ValidSinceSeconds  int64       `json:"validSince,string,omitempty"`
}

func (r *userQueryResponse) makeUserRecord() (*UserRecord, error) {
	exported, err := r.makeExportedUserRecord()
	if err != nil {
		return nil, err
	}

	return exported.UserRecord, nil
}

func (r *userQueryResponse) makeExportedUserRecord() (*ExportedUserRecord, error) {
	var customClaims map[string]interface{}
	if r.CustomAttributes != "" {
		err := json.Unmarshal([]byte(r.CustomAttributes), &customClaims)
		if err != nil {
			return nil, err
		}
		if len(customClaims) == 0 {
			customClaims = nil
		}
	}

	return &ExportedUserRecord{
		UserRecord: &UserRecord{
			UserInfo: &UserInfo{
				DisplayName: r.DisplayName,
				Email:       r.Email,
				PhoneNumber: r.PhoneNumber,
				PhotoURL:    r.PhotoURL,
				UID:         r.UID,
				ProviderID:  defaultProviderID,
			},
			CustomClaims:           customClaims,
			Disabled:               r.Disabled,
			EmailVerified:          r.EmailVerified,
			ProviderUserInfo:       r.ProviderUserInfo,
			TokensValidAfterMillis: r.ValidSinceSeconds * 1000,
			UserMetadata: &UserMetadata{
				LastLogInTimestamp: r.LastLogInTimestamp,
				CreationTimestamp:  r.CreationTimestamp,
			},
		},
		PasswordHash: r.PasswordHash,
		PasswordSalt: r.PasswordSalt,
	}, nil
}

// CreateUser creates a new user with the specified properties.
func (c *userManagementClient) CreateUser(ctx context.Context, user *UserToCreate) (*UserRecord, error) {
	uid, err := c.createUser(ctx, user)
	if err != nil {
		return nil, err
	}
	return c.GetUser(ctx, uid)
}

func (c *userManagementClient) createUser(ctx context.Context, user *UserToCreate) (string, error) {
	if user == nil {
		user = &UserToCreate{}
	}

	request, err := user.validatedRequest()
	if err != nil {
		return "", err
	}

	resp, err := c.post(ctx, "/accounts", request)
	if err != nil {
		return "", err
	}

	if resp.Status != http.StatusOK {
		return "", handleHTTPError(resp)
	}

	var result struct {
		UID string `json:"localId"`
	}
	err = json.Unmarshal(resp.Body, &result)
	return result.UID, err
}

// UpdateUser updates an existing user account with the specified properties.
func (c *userManagementClient) UpdateUser(
	ctx context.Context, uid string, user *UserToUpdate) (ur *UserRecord, err error) {
	if err := c.updateUser(ctx, uid, user); err != nil {
		return nil, err
	}
	return c.GetUser(ctx, uid)
}

func (c *userManagementClient) updateUser(ctx context.Context, uid string, user *UserToUpdate) error {
	if err := validateUID(uid); err != nil {
		return err
	}
	if user == nil {
		return fmt.Errorf("update parameters must not be nil or empty")
	}

	request, err := user.validatedRequest()
	if err != nil {
		return err
	}
	request["localId"] = uid

	resp, err := c.post(ctx, "/accounts:update", request)
	if err != nil {
		return err
	}

	if resp.Status != http.StatusOK {
		return handleHTTPError(resp)
	}
	return nil
}

// DeleteUser deletes the user by the given UID.
func (c *userManagementClient) DeleteUser(ctx context.Context, uid string) error {
	if err := validateUID(uid); err != nil {
		return err
	}

	payload := map[string]interface{}{
		"localId": uid,
	}
	resp, err := c.post(ctx, "/accounts:delete", payload)
	if err != nil {
		return err
	}

	if resp.Status != http.StatusOK {
		return handleHTTPError(resp)
	}
	return nil
}

// SessionCookie creates a new Firebase session cookie from the given ID token and expiry
// duration. The returned JWT can be set as a server-side session cookie with a custom cookie
// policy. Expiry duration must be at least 5 minutes but may not exceed 14 days.
func (c *userManagementClient) SessionCookie(
	ctx context.Context,
	idToken string,
	expiresIn time.Duration,
) (string, error) {

	if idToken == "" {
		return "", errors.New("id token must not be empty")
	}

	if expiresIn < 5*time.Minute || expiresIn > 14*24*time.Hour {
		return "", errors.New("expiry duration must be between 5 minutes and 14 days")
	}

	payload := map[string]interface{}{
		"idToken":       idToken,
		"validDuration": int64(expiresIn.Seconds()),
	}
	resp, err := c.post(ctx, ":createSessionCookie", payload)
	if err != nil {
		return "", err
	}

	if resp.Status != http.StatusOK {
		return "", handleHTTPError(resp)
	}

	var result struct {
		SessionCookie string `json:"sessionCookie"`
	}
	err = json.Unmarshal(resp.Body, &result)
	return result.SessionCookie, err
}

func (c *userManagementClient) post(
	ctx context.Context,
	path string,
	payload interface{},
) (*internal.Response, error) {

	req, err := c.newRequest(http.MethodPost, path)
	if err != nil {
		return nil, err
	}
	req.Body = internal.NewJSONEntity(payload)
	return c.httpClient.Do(ctx, req)
}

func (c *userManagementClient) newRequest(method, path string) (*internal.Request, error) {
	if c.projectID == "" {
		return nil, errors.New("project id not available")
	}

	versionHeader := internal.WithHeader("X-Client-Version", c.version)
	return &internal.Request{
		Method: method,
		URL:    fmt.Sprintf("%s/%s%s", c.baseURL, c.projectID, path),
		Opts:   []internal.HTTPOption{versionHeader},
	}, nil
}

func handleHTTPError(resp *internal.Response) error {
	var httpErr struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	json.Unmarshal(resp.Body, &httpErr) // ignore any json parse errors at this level
	serverCode := httpErr.Error.Message
	clientCode, ok := serverError[serverCode]
	if !ok {
		clientCode = unknown
	}
	return internal.Errorf(
		clientCode,
		"http error status: %d; body: %s",
		resp.Status,
		string(resp.Body))
}
