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
	"encoding/base64"
	"errors"
	"fmt"

	"firebase.google.com/go/internal"
	identitytoolkit "google.golang.org/api/identitytoolkit/v3"
)

const maxImportUsers = 1000

// UserToImport represents a user account that can be bulk imported into Firebase Auth.
type UserToImport struct {
	info   *identitytoolkit.UserInfo
	claims map[string]interface{}
}

// UserImportOption is an option for the ImportUsers() function.
type UserImportOption interface {
	applyTo(req *identitytoolkit.IdentitytoolkitRelyingpartyUploadAccountRequest) error
}

// UserImportResult represents the result of an ImportUsers() call.
type UserImportResult struct {
	SuccessCount int
	FailureCount int
	Errors       []*ErrorInfo
}

// ErrorInfo represents an error encountered while importing a single user account.
//
// The Index field corresponds to the index of the failed user in the users array that was passed
// to ImportUsers().
type ErrorInfo struct {
	Index  int
	Reason string
}

// ImportUsers imports an array of users to Firebase Auth.
//
// No more than 1000 users can be imported in a single call. If at least one user specifies a
// password, a UserImportHash must be specified as an option.
func (c *Client) ImportUsers(ctx context.Context, users []*UserToImport, opts ...UserImportOption) (*UserImportResult, error) {
	if len(users) == 0 {
		return nil, errors.New("users list must not be empty")
	}
	if len(users) > maxImportUsers {
		return nil, fmt.Errorf("users list must not contain more than %d elements", maxImportUsers)
	}

	req := &identitytoolkit.IdentitytoolkitRelyingpartyUploadAccountRequest{}
	hashRequired := false
	for _, u := range users {
		vu, err := u.validatedUserInfo()
		if err != nil {
			return nil, err
		}
		if vu.PasswordHash != "" {
			hashRequired = true
		}
		req.Users = append(req.Users, vu)
	}

	for _, opt := range opts {
		if err := opt.applyTo(req); err != nil {
			return nil, err
		}
	}
	if hashRequired && req.HashAlgorithm == "" {
		return nil, errors.New("hash algorithm option is required to import users with passwords")
	}

	call := c.is.Relyingparty.UploadAccount(req)
	c.setHeader(call)
	resp, err := call.Context(ctx).Do()
	if err != nil {
		return nil, handleServerError(err)
	}
	result := &UserImportResult{
		SuccessCount: len(users) - len(resp.Error),
		FailureCount: len(resp.Error),
	}
	for _, e := range resp.Error {
		result.Errors = append(result.Errors, &ErrorInfo{
			Index:  int(e.Index),
			Reason: e.Message,
		})
	}
	return result, nil
}

// UID setter. This field is required.
func (u *UserToImport) UID(uid string) *UserToImport {
	u.userInfo().LocalId = uid
	return u
}

// Email setter.
func (u *UserToImport) Email(email string) *UserToImport {
	u.userInfo().Email = email
	return u
}

// DisplayName setter.
func (u *UserToImport) DisplayName(displayName string) *UserToImport {
	u.userInfo().DisplayName = displayName
	return u
}

// PhotoURL setter.
func (u *UserToImport) PhotoURL(url string) *UserToImport {
	u.userInfo().PhotoUrl = url
	return u
}

// PhoneNumber setter.
func (u *UserToImport) PhoneNumber(phoneNumber string) *UserToImport {
	u.userInfo().PhoneNumber = phoneNumber
	return u
}

// Metadata setter.
func (u *UserToImport) Metadata(metadata *UserMetadata) *UserToImport {
	info := u.userInfo()
	info.CreatedAt = metadata.CreationTimestamp
	info.LastLoginAt = metadata.LastLogInTimestamp
	return u
}

// UserProvider represents a user identity provider.
//
// One or more user providers can be specified for each user when importing in bulk.
// See UserToImport type.
type UserProvider struct {
	UID         string
	ProviderID  string
	Email       string
	DisplayName string
	PhotoURL    string
}

// ProviderData setter.
func (u *UserToImport) ProviderData(providers []*UserProvider) *UserToImport {
	var providerUserInfo []*identitytoolkit.UserInfoProviderUserInfo
	for _, p := range providers {
		providerUserInfo = append(providerUserInfo, &identitytoolkit.UserInfoProviderUserInfo{
			ProviderId:  p.ProviderID,
			RawId:       p.UID,
			Email:       p.Email,
			DisplayName: p.DisplayName,
			PhotoUrl:    p.PhotoURL,
		})
	}
	u.userInfo().ProviderUserInfo = providerUserInfo
	return u
}

// CustomClaims setter.
func (u *UserToImport) CustomClaims(claims map[string]interface{}) *UserToImport {
	u.claims = claims
	return u
}

// Disabled setter.
func (u *UserToImport) Disabled(disabled bool) *UserToImport {
	info := u.userInfo()
	info.Disabled = disabled
	if !disabled {
		info.ForceSendFields = append(info.ForceSendFields, "Disabled")
	}
	return u
}

// EmailVerified setter.
func (u *UserToImport) EmailVerified(emailVerified bool) *UserToImport {
	info := u.userInfo()
	info.EmailVerified = emailVerified
	if !emailVerified {
		info.ForceSendFields = append(info.ForceSendFields, "EmailVerified")
	}
	return u
}

// PasswordHash setter. When set a UserImportHash must be specified as an option to call
// ImportUsers().
func (u *UserToImport) PasswordHash(password []byte) *UserToImport {
	u.userInfo().PasswordHash = base64.RawURLEncoding.EncodeToString(password)
	return u
}

// PasswordSalt setter.
func (u *UserToImport) PasswordSalt(salt []byte) *UserToImport {
	u.userInfo().Salt = base64.RawURLEncoding.EncodeToString(salt)
	return u
}

func (u *UserToImport) userInfo() *identitytoolkit.UserInfo {
	if u.info == nil {
		u.info = &identitytoolkit.UserInfo{}
	}
	return u.info
}

func (u *UserToImport) validatedUserInfo() (*identitytoolkit.UserInfo, error) {
	if u.info == nil {
		return nil, fmt.Errorf("no parameters are set on the user to import")
	}
	info := u.info
	if err := validateUID(info.LocalId); err != nil {
		return nil, err
	}
	if info.Email != "" {
		if err := validateEmail(info.Email); err != nil {
			return nil, err
		}
	}
	if info.PhoneNumber != "" {
		if err := validatePhone(info.PhoneNumber); err != nil {
			return nil, err
		}
	}
	if len(u.claims) > 0 {
		cc, err := marshalCustomClaims(u.claims)
		if err != nil {
			return nil, err
		}
		info.CustomAttributes = cc
	}

	for _, p := range info.ProviderUserInfo {
		if p.RawId == "" {
			return nil, fmt.Errorf("user provdier must specify a uid")
		}
		if p.ProviderId == "" {
			return nil, fmt.Errorf("user provider must specify a provider ID")
		}
	}
	return info, nil
}

// WithHash returns a UserImportOption that specifies a hash configuration.
func WithHash(hash UserImportHash) UserImportOption {
	return withHash{hash}
}

// UserImportHash represents a hash algorithm and the associated configuration that can be used to
// hash user passwords.
//
// A UserImportHash must be specified in the form of a UserImportOption when importing users with
// passwords. See ImportUsers() and WithHash() functions.
type UserImportHash interface {
	Config() (*internal.HashConfig, error)
}

type withHash struct {
	hash UserImportHash
}

func (w withHash) applyTo(req *identitytoolkit.IdentitytoolkitRelyingpartyUploadAccountRequest) error {
	conf, err := w.hash.Config()
	if err != nil {
		return err
	}
	req.HashAlgorithm = conf.HashAlgorithm
	req.SignerKey = conf.SignerKey
	req.SaltSeparator = conf.SaltSeparator
	req.Rounds = conf.Rounds
	req.MemoryCost = conf.MemoryCost
	req.DkLen = conf.DerivedKeyLength
	req.Parallelization = conf.Parallelization
	req.BlockSize = conf.BlockSize
	req.ForceSendFields = conf.ForceSendFields
	return nil
}
