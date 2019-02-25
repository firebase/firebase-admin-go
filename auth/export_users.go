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
	"encoding/json"

	identitytoolkit "google.golang.org/api/identitytoolkit/v3"
	"google.golang.org/api/iterator"
)

const maxReturnedResults = 1000

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

// ExportedUserRecord is the returned user value used when listing all the users.
type ExportedUserRecord struct {
	*UserRecord
	PasswordHash string
	PasswordSalt string
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
