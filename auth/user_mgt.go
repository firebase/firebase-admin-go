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
	"strings"
	"time"

	"firebase.google.com/go/internal"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/identitytoolkit/v3"
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

// UserToCreate is the parameter struct for the CreateUser function.
type UserToCreate struct {
	createReq   *identitytoolkit.IdentitytoolkitRelyingpartySignupNewUserRequest
	uid         bool
	displayName bool
	email       bool
	photoURL    bool
	phoneNumber bool
}

func (u *UserToCreate) request() *identitytoolkit.IdentitytoolkitRelyingpartySignupNewUserRequest {
	if u.createReq == nil {
		u.createReq = &identitytoolkit.IdentitytoolkitRelyingpartySignupNewUserRequest{}
	}
	return u.createReq
}

func (u *UserToCreate) validatedRequest() (*identitytoolkit.IdentitytoolkitRelyingpartySignupNewUserRequest, error) {
	req := u.request() // creating a user without any parameters is allowed
	if u.uid {
		if err := validateUID(req.LocalId); err != nil {
			return nil, err
		}
	}
	if u.displayName {
		if err := validateDisplayName(req.DisplayName); err != nil {
			return nil, err
		}
	}
	if u.email {
		if err := validateEmail(req.Email); err != nil {
			return nil, err
		}
	}
	if u.phoneNumber {
		if err := validatePhone(req.PhoneNumber); err != nil {
			return nil, err
		}
	}
	if u.photoURL {
		if err := validatePhotoURL(req.PhotoUrl); err != nil {
			return nil, err
		}
	}
	if req.Password != "" {
		if err := validatePassword(req.Password); err != nil {
			return nil, err
		}
	}
	return req, nil
}

// Disabled setter.
func (u *UserToCreate) Disabled(disabled bool) *UserToCreate {
	req := u.request()
	req.Disabled = disabled
	if !disabled {
		req.ForceSendFields = append(req.ForceSendFields, "Disabled")
	}
	return u
}

// DisplayName setter.
func (u *UserToCreate) DisplayName(name string) *UserToCreate {
	u.request().DisplayName = name
	u.displayName = true
	return u
}

// Email setter.
func (u *UserToCreate) Email(email string) *UserToCreate {
	u.request().Email = email
	u.email = true
	return u
}

// EmailVerified setter.
func (u *UserToCreate) EmailVerified(verified bool) *UserToCreate {
	req := u.request()
	req.EmailVerified = verified
	if !verified {
		req.ForceSendFields = append(req.ForceSendFields, "EmailVerified")
	}
	return u
}

// Password setter.
func (u *UserToCreate) Password(pw string) *UserToCreate {
	u.request().Password = pw
	return u
}

// PhoneNumber setter.
func (u *UserToCreate) PhoneNumber(phone string) *UserToCreate {
	u.request().PhoneNumber = phone
	u.phoneNumber = true
	return u
}

// PhotoURL setter.
func (u *UserToCreate) PhotoURL(url string) *UserToCreate {
	u.request().PhotoUrl = url
	u.photoURL = true
	return u
}

// UID setter.
func (u *UserToCreate) UID(uid string) *UserToCreate {
	u.request().LocalId = uid
	u.uid = true
	return u
}

// UserToUpdate is the parameter struct for the UpdateUser function.
type UserToUpdate struct {
	updateReq    *identitytoolkit.IdentitytoolkitRelyingpartySetAccountInfoRequest
	claims       map[string]interface{}
	displayName  bool
	email        bool
	phoneNumber  bool
	photoURL     bool
	customClaims bool
}

func (u *UserToUpdate) request() *identitytoolkit.IdentitytoolkitRelyingpartySetAccountInfoRequest {
	if u.updateReq == nil {
		u.updateReq = &identitytoolkit.IdentitytoolkitRelyingpartySetAccountInfoRequest{}
	}
	return u.updateReq
}

func (u *UserToUpdate) validatedRequest() (*identitytoolkit.IdentitytoolkitRelyingpartySetAccountInfoRequest, error) {
	if u.updateReq == nil {
		// update without any parameters is never allowed
		return nil, fmt.Errorf("update parameters must not be nil or empty")
	}
	req := u.updateReq
	if u.email {
		if err := validateEmail(req.Email); err != nil {
			return nil, err
		}
	}
	if u.displayName && req.DisplayName == "" {
		req.DeleteAttribute = append(req.DeleteAttribute, "DISPLAY_NAME")
	}
	if u.photoURL && req.PhotoUrl == "" {
		req.DeleteAttribute = append(req.DeleteAttribute, "PHOTO_URL")
	}
	if u.phoneNumber {
		if req.PhoneNumber == "" {
			req.DeleteProvider = append(req.DeleteProvider, "phone")
		} else if err := validatePhone(req.PhoneNumber); err != nil {
			return nil, err
		}
	}
	if u.customClaims {
		cc, err := marshalCustomClaims(u.claims)
		if err != nil {
			return nil, err
		}
		req.CustomAttributes = cc
	}
	if req.Password != "" {
		if err := validatePassword(req.Password); err != nil {
			return nil, err
		}
	}
	return req, nil
}

// CustomClaims setter.
func (u *UserToUpdate) CustomClaims(claims map[string]interface{}) *UserToUpdate {
	u.request() // force initialization of the request for later use
	u.claims = claims
	u.customClaims = true
	return u
}

// Disabled setter.
func (u *UserToUpdate) Disabled(disabled bool) *UserToUpdate {
	req := u.request()
	req.DisableUser = disabled
	if !disabled {
		req.ForceSendFields = append(req.ForceSendFields, "DisableUser")
	}
	return u
}

// DisplayName setter.
func (u *UserToUpdate) DisplayName(name string) *UserToUpdate {
	u.request().DisplayName = name
	u.displayName = true
	return u
}

// Email setter.
func (u *UserToUpdate) Email(email string) *UserToUpdate {
	u.request().Email = email
	u.email = true
	return u
}

// EmailVerified setter.
func (u *UserToUpdate) EmailVerified(verified bool) *UserToUpdate {
	req := u.request()
	req.EmailVerified = verified
	if !verified {
		req.ForceSendFields = append(req.ForceSendFields, "EmailVerified")
	}
	return u
}

// Password setter.
func (u *UserToUpdate) Password(pw string) *UserToUpdate {
	u.request().Password = pw
	return u
}

// PhoneNumber setter.
func (u *UserToUpdate) PhoneNumber(phone string) *UserToUpdate {
	u.request().PhoneNumber = phone
	u.phoneNumber = true
	return u
}

// PhotoURL setter.
func (u *UserToUpdate) PhotoURL(url string) *UserToUpdate {
	u.request().PhotoUrl = url
	u.photoURL = true
	return u
}

// revokeRefreshTokens revokes all refresh tokens for a user by setting the validSince property
// to the present in epoch seconds.
func (u *UserToUpdate) revokeRefreshTokens() *UserToUpdate {
	u.request().ValidSince = time.Now().Unix()
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
	phoneNumberAlreadyExists = "phone-number-already-exists"
	projectNotFound          = "project-not-found"
	sessionCookieRevoked     = "session-cookie-revoked"
	uidAlreadyExists         = "uid-already-exists"
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
	"DUPLICATE_EMAIL":         emailAlreadyExists,
	"DUPLICATE_LOCAL_ID":      uidAlreadyExists,
	"EMAIL_EXISTS":            emailAlreadyExists,
	"INSUFFICIENT_PERMISSION": insufficientPermission,
	"PERMISSION_DENIED":       insufficientPermission,
	"PHONE_NUMBER_EXISTS":     phoneNumberAlreadyExists,
	"PROJECT_NOT_FOUND":       projectNotFound,
	"USER_NOT_FOUND":          userNotFound,
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

// Helper functions for retrieval and HTTP calls.

func (c *Client) createUser(ctx context.Context, user *UserToCreate) (string, error) {
	if user == nil {
		user = &UserToCreate{}
	}

	request, err := user.validatedRequest()
	if err != nil {
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
	if user == nil {
		return fmt.Errorf("update parameters must not be nil or empty")
	}
	request, err := user.validatedRequest()
	if err != nil {
		return err
	}
	request.LocalId = uid
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

const idToolkitEndpoint = "https://identitytoolkit.googleapis.com/v1/projects"

// userManagementClient is a helper for interacting with the Identity Toolkit REST API.
//
// Currently it is only used for some user management operations (e.g. session cookie
// generation). We will gradually migrate all the user management functionality to this
// type, and remove our dependency on the google.golang.org/api/identitytoolkit/v3
// package.
type userManagementClient struct {
	baseURL    string
	projectID  string
	version    string
	httpClient *internal.HTTPClient
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
