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

	"golang.org/x/net/context"
)

const MaxResults = 1000

// UID option ("localId" in the REST), Create only
func WithUID(uid string) withUID { return withUID(uid) }

type withUID string

func (uid withUID) applyForCreateUser(uf *UserFields) { uf.payload["localId"] = uid }

// DisplayName option
func WithDisplayName(dn string) withDisplayName { return withDisplayName(dn) }

type withDisplayName string

func (dn withDisplayName) applyForCreateUser(uf *UserFields) { uf.payload["displayName"] = dn }
func (dn withDisplayName) applyForUpdateUser(uf *UserFields) { uf.payload["displayName"] = dn }

// Email option
func WithEmail(em string) withEmail { return withEmail(em) }

type withEmail string

func (em withEmail) applyForCreateUser(uf *UserFields) { uf.payload["email"] = em }
func (em withEmail) applyForUpdateUser(uf *UserFields) { uf.payload["email"] = em }

// EmailVerified option
func WithEmailVerified(ev bool) withEmailVerified { return withEmailVerified(ev) }

type withEmailVerified bool

func (ev withEmailVerified) applyForCreateUser(uf *UserFields) { uf.payload["emailVerified"] = ev }
func (ev withEmailVerified) applyForUpdateUser(uf *UserFields) { uf.payload["emailVerified"] = ev }

// PhoneNumber option
func WithPhoneNumber(pn string) withPhoneNumber { return withPhoneNumber(pn) }

type withPhoneNumber string

func (pn withPhoneNumber) applyForCreateUser(uf *UserFields) { uf.payload["phoneNumber"] = pn }
func (pn withPhoneNumber) applyForUpdateUser(uf *UserFields) { uf.payload["phoneNumber"] = pn }

// PhotoUTL option
func WithPhotoURL(pu string) withPhotoURL { return withPhotoURL(pu) }

type withPhotoURL string

func (pu withPhotoURL) applyForCreateUser(uf *UserFields) { uf.payload["photoURL"] = pu }
func (pu withPhotoURL) applyForUpdateUser(uf *UserFields) { uf.payload["photoURL"] = pu }

//Password option
func WithPassword(pw string) withPassword { return withPassword(pw) }

type withPassword string

func (pw withPassword) applyForCreateUser(uf *UserFields) { uf.payload["password"] = pw }
func (pw withPassword) applyForUpdateUser(uf *UserFields) { uf.payload["password"] = pw }

// Disabled option
func WithDisabled(da bool) withDisabled { return withDisabled(da) }

type withDisabled bool

func (da withDisabled) applyForCreateUser(uf *UserFields) { uf.payload["disabled"] = da }
func (da withDisabled) applyForUpdateUser(uf *UserFields) { uf.payload["disabled"] = da }

// Remove Options (Update Only)

// Remove Display Name
func WithRemoveDisplayName() withRemoveDisplayName { return withRemoveDisplayName{} }

type withRemoveDisplayName struct{}

func (dn withRemoveDisplayName) applyForUpdateUser(uf *UserFields) {
	if _, ok := uf.payload["deleteAttribute"]; ok {
		uf.payload["deleteAttribute"] = append(uf.payload["deleteAttribute"].([]string), "displayName")
	} else {
		uf.payload["deleteAttribute"] = []string{"displayName"}
	}
}

// Remove Phone Number
func WithRemovePhoneNumber() withRemovePhoneNumber { return withRemovePhoneNumber{} }

type withRemovePhoneNumber struct{}

func (dn withRemovePhoneNumber) applyForUpdateUser(uf *UserFields) {
	if _, ok := uf.payload["deleteProvider"]; ok {
		uf.payload["deleteProvider"] = append(uf.payload["deleteProvider"].([]string), "phoneNumber")
	} else {
		uf.payload["deleteProvider"] = []string{"phoneNumber"}
	}
}

// Remove Photo Url
func WithRemovePhotoURL() withRemovePhotoURL { return withRemovePhotoURL{} }

type withRemovePhotoURL struct{}

func (dn withRemovePhotoURL) applyForUpdateUser(uf *UserFields) {
	if _, ok := uf.payload["deleteAttribute"]; ok {
		uf.payload["deleteAttribute"] = append(uf.payload["deleteAttribute"].([]string), "photoURL")
	} else {
		uf.payload["deleteAttribute"] = []string{"photoURL"}
	}
}

// -------- drafts.

type UserFields struct{ payload map[string]interface{} }

func (uf UserFields) ExportPayload() ([]byte, error) {
	req, err := json.Marshal(&uf.payload)
	//	fmt.Println(string(req), uf, uf.payload, err)
	if err != nil {
		return nil, err
	}
	return req, nil

}

func (uf *UserFields) Validate() (bool, error) {
	for _, deleteList := range []string{"deleteAttribute", "deleteProvider"} {
		if delete, exists := uf.payload[deleteList]; exists {
			for _, deleteAtt := range delete.([]string) {
				if _, found := uf.payload[deleteAtt]; found {
					return false, fmt.Errorf("trying to delete and set %s", deleteAtt)
				}
			}
		}
	}
	return true, nil
}
func NewUserFields() UserFields {
	return UserFields{payload: make(map[string]interface{})}
}

type CreateUserOption interface {
	applyForCreateUser(*UserFields)
}

func (c *Client) CreateUser(ctx context.Context, opts ...CreateUserOption) (ur *UserRecord, err error) {
	f := NewUserFields()
	for _, opt := range opts {
		opt.applyForCreateUser(&f)
	}

	if ok, err := f.Validate(); !ok || err != nil {
		return nil, err
	}

	ans, err := c.makeUserRequest(ctx, "signupNewUser", f.payload)

	if err != nil {
		return nil, fmt.Errorf("bad request %s, %s", string(ans), err)
	}

	jsonMap, err := parseResponse(ans)
	if err != nil {
		return nil, fmt.Errorf("bad json %s, %s", string(ans), err)
	}
	uid := jsonMap["localId"].(string)

	user, err := c.GetUser(ctx, uid)
	if err != nil {
		return nil, err
	}
	return user.UserRecord, nil
}

func (c *Client) DeleteUser(ctx context.Context, uid string) error {

	_, err := c.makeUserRequest(ctx, "deleteAccount",
		map[string]interface{}{"localId": []string{uid}})
	return err
}

type UpdateUserOption interface {
	applyForUpdateUser(*UserFields)
}

///----
type GetUserResponse struct {
	RequestType string               `json:"kind,omitempty"`
	Users       []ResponseUserRecord `json:"users,omitempty"`
}

type ResponseUserRecord struct {
	UID                string            `json:"localId,omitempty"`
	DisplayName        string            `json:"displayName,omitempty"`
	Email              string            `json:"email,omitempty"`
	PhoneNumber        string            `json:"phoneNumber,omitempty"`
	PhotoURL           string            `json:"photoURL,omitempty"`
	CreationTimestamp  int64             `json:"createdAt,string,omitempty"`
	LastLogInTimestamp int64             `json:"lastLoginAt,string,omitempty"`
	ProviderID         string            `json:"providerID,omitempty"`
	CustomClaims       map[string]string `json:"customClaims,omitempty"`
	Disabled           bool              `json:"disabled,omitempty"`
	EmailVerified      bool              `json:"emailVerified,omitempty"`
	ProviderUserInfo   []*UserInfo       `json:"providerMata,omitempty"`
	PasswordHash       string            `json:"passwordHash,omitempty"`
	PasswordSalt       string            `json:"passwordSalt,omitempty"`
	ValidSince         int64             `json:"validSince,string,omitempty"`
}

type ListUsersResponse struct {
	RequestType string               `json:"kind,omitempty"`
	Users       []ResponseUserRecord `json:"users,omitempty"`
	NextPage    string               `json:"nextPageToken,omitempty"`
}

func (c *Client) GetUser(ctx context.Context, uid string) (*ExportedUserRecord, error) {
	return c.getUser(ctx, map[string]interface{}{"localId": []string{uid}})
}
func makeExportedUser(rur ResponseUserRecord) *ExportedUserRecord {
	ans := &ExportedUserRecord{
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
	//	fmt.Printf("rur %+v\nrur %#v\n-------------\nans %+v\nans %#v\n\n========\n", rur, rur, ans, ans)
	return ans
}

func (c *Client) getUser(ctx context.Context, m map[string]interface{}) (*ExportedUserRecord, error) {
	resp, err := c.makeUserRequest(ctx, "getAccountInfo", m)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	var gur GetUserResponse
	err = json.Unmarshal(resp, &gur)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return makeExportedUser(gur.Users[0]), nil
}

func (c *Client) ListUsers(ctx context.Context, pageToken string) (*ListUsersPage, error) {
	return c.ListUsersWithMaxResults(ctx, pageToken, MaxResults)
}

func (c *Client) ListUsersWithMaxResults(ctx context.Context, pageToken string, numResults int) (*ListUsersPage, error) {
	payload := map[string]interface{}{"maxResults": numResults}
	if len(pageToken) > 0 {
		payload["nextPageToken"] = pageToken
	}
	ans, err := c.makeUserRequest(
		ctx,
		"downloadAccount",
		payload)
	if err != nil {
		return nil, err
	}

	var lur ListUsersResponse
	err2 := json.Unmarshal(ans, &lur)
	if err2 != nil {
		return nil, err2
	}
	usersList := make([]*ExportedUserRecord, 0)
	for _, u := range lur.Users {
		usersList = append(usersList, makeExportedUser(u))
	}
	return &ListUsersPage{
		Users:      usersList,
		PageToken:  lur.NextPage,
		maxResults: numResults,
		client:     c,
	}, nil
}

func (lup *ListUsersPage) HasNext() bool {
	return lup.PageToken != ""
}

func (lup *ListUsersPage) Next(ctx context.Context) (*ListUsersPage, error) {
	if lup.PageToken == "" {
		return nil, nil
	}
	return lup.client.ListUsersWithMaxResults(ctx, lup.PageToken, lup.maxResults)
}

type UserItem struct {
	user *ExportedUserRecord
	err  error
}

func (ui *UserItem) Value() (*ExportedUserRecord, error) {
	return ui.user, ui.err
}
func (lup *ListUsersPage) IterateAll(ctx context.Context) chan *UserItem {

	ch := make(chan *UserItem, lup.maxResults)
	go func() {
		var err error
		for lup != nil {
			for _, u := range lup.Users {
				ch <- &UserItem{
					user: u,
					err:  nil,
				}
			}

			lup, err = lup.Next(ctx)
			if err != nil {
				ch <- &UserItem{
					user: nil,
					err:  err,
				}
			}

		}
		close(ch)
	}()
	return ch
}
