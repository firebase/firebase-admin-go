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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	// "io/ioutil" // Not needed for the remaining tests
	// "net/http" // Not needed for the remaining tests
	// "net/http/httptest" // Not needed for the remaining tests
	"reflect"
	"sort"
	// "strconv" // Not needed for the remaining tests
	"strings"
	"testing"
	"time"

	"firebase.google.com/go/v4/internal"
	// "google.golang.org/api/iterator" // Not needed for the remaining tests
	// "firebase.google.com/go/v4/app" // Not directly used by the remaining tests
)

var testUser = &UserRecord{ // Still used by TestMakeExportedUser
	UserInfo: &UserInfo{
		UID:         "testuser",
		Email:       "testuser@example.com",
		PhoneNumber: "+1234567890",
		DisplayName: "Test User",
		PhotoURL:    "http://www.example.com/testuser/photo.png",
		ProviderID:  defaultProviderID,
	},
	Disabled:      false,
	EmailVerified: true,
	ProviderUserInfo: []*UserInfo{
		{
			ProviderID:  "password",
			DisplayName: "Test User",
			PhotoURL:    "http://www.example.com/testuser/photo.png",
			Email:       "testuser@example.com",
			UID:         "testuid",
		}, {
			ProviderID:  "phone",
			PhoneNumber: "+1234567890",
			UID:         "testuid",
		},
	},
	TokensValidAfterMillis: 1494364393000,
	UserMetadata: &UserMetadata{
		CreationTimestamp:  1234567890000,
		LastLogInTimestamp: 1233211232000,
	},
	CustomClaims: map[string]interface{}{"admin": true, "package": "gold"},
	TenantID:     "testTenant",
	MultiFactor: &MultiFactorSettings{
		EnrolledFactors: []*MultiFactorInfo{
			{
				UID:                 "enrolledPhoneFactor",
				FactorID:            "phone",
				EnrollmentTimestamp: 1614776780000,
				Phone: &PhoneMultiFactorInfo{
					PhoneNumber: "+1234567890",
				},
				PhoneNumber: "+1234567890",
				DisplayName: "My MFA Phone",
			},
			{
				UID:                 "enrolledTOTPFactor",
				FactorID:            "totp",
				EnrollmentTimestamp: 1614776780000,
				TOTP:                &TOTPMultiFactorInfo{},
				DisplayName:         "My MFA TOTP",
			},
		},
	},
}

var testUserWithoutMFA = &UserRecord{ // Still used by TestMakeExportedUser
	UserInfo: &UserInfo{
		UID:         "testusernomfa",
		Email:       "testusernomfa@example.com",
		PhoneNumber: "+1234567890",
		DisplayName: "Test User Without MFA",
		PhotoURL:    "http://www.example.com/testusernomfa/photo.png",
		ProviderID:  defaultProviderID,
	},
	Disabled:      false,
	EmailVerified: true,
	ProviderUserInfo: []*UserInfo{
		{
			ProviderID:  "password",
			DisplayName: "Test User Without MFA",
			PhotoURL:    "http://www.example.com/testusernomfa/photo.png",
			Email:       "testusernomfa@example.com",
			UID:         "testuid",
		}, {
			ProviderID:  "phone",
			PhoneNumber: "+1234567890",
			UID:         "testuid",
		},
	},
	TokensValidAfterMillis: 1494364393000,
	UserMetadata: &UserMetadata{
		CreationTimestamp:  1234567890000,
		LastLogInTimestamp: 1233211232000,
	},
	CustomClaims: map[string]interface{}{"admin": true, "package": "gold"},
	TenantID:     "testTenant",
	MultiFactor:  &MultiFactorSettings{},
}


func TestInvalidGetUser(t *testing.T) {
	client := &Client{
		baseClient: &baseClient{},
	}

	user, err := client.GetUser(context.Background(), "")
	if user != nil || err == nil {
		t.Errorf("GetUser('') = (%v, %v); want = (nil, error)", user, err)
	}

	user, err = client.GetUserByEmail(context.Background(), "")
	if user != nil || err == nil {
		t.Errorf("GetUserByEmail('') = (%v, %v); want = (nil, error)", user, err)
	}

	user, err = client.GetUserByPhoneNumber(context.Background(), "")
	if user != nil || err == nil {
		t.Errorf("GetUserPhoneNumber('') = (%v, %v); want = (nil, error)", user, err)
	}

	userRecord, err := client.GetUserByProviderUID(context.Background(), "", "google_uid1")
	want := "providerID must be a non-empty string"
	if userRecord != nil || err == nil || err.Error() != want {
		t.Errorf("GetUserByProviderUID() = (%v, %q); want = (nil, %q)", userRecord, err, want)
	}

	userRecord, err = client.GetUserByProviderUID(context.Background(), "google.com", "")
	want = "providerUID must be a non-empty string"
	if userRecord != nil || err == nil || err.Error() != want {
		t.Errorf("GetUserByProviderUID() = (%v, %q); want = (nil, %q)", userRecord, err, want)
	}
}

func sameUsers(users [](*UserRecord), uids []string) bool {
	if len(users) != len(uids) {
		return false
	}

	sort.Slice(users, func(i, j int) bool {
		return users[i].UID < users[j].UID
	})
	sort.Slice(uids, func(i, j int) bool {
		return uids[i] < uids[j]
	})

	for i := range users {
		if users[i].UID != uids[i] {
			return false
		}
	}

	return true
}

func TestGetUsersExceeds100(t *testing.T) {
	client := &Client{
		baseClient: &baseClient{},
	}

	var identifiers [101]UserIdentifier
	for i := 0; i < 101; i++ {
		identifiers[i] = &UIDIdentifier{UID: fmt.Sprintf("id%d", i)}
	}

	getUsersResult, err := client.GetUsers(context.Background(), identifiers[:])
	want := "`identifiers` parameter must have <= 100 entries"
	if getUsersResult != nil || err == nil || err.Error() != want {
		t.Errorf(
			"GetUsers() = (%v, %q); want = (nil, %q)",
			getUsersResult, err, want)
	}
}

func TestGetUsersEmpty(t *testing.T) {
	client := &Client{
		baseClient: &baseClient{},
	}

	getUsersResult, err := client.GetUsers(context.Background(), [](UserIdentifier){})
	if getUsersResult == nil || err != nil {
		t.Fatalf("GetUsers([]) error = %q; want = nil", err)
	}

	if len(getUsersResult.Users) != 0 {
		t.Errorf("len(GetUsers([]).Users) = %d; want 0", len(getUsersResult.Users))
	}
	if len(getUsersResult.NotFound) != 0 {
		t.Errorf("len(GetUsers([]).NotFound) = %d; want 0", len(getUsersResult.NotFound))
	}
}


func TestGetUsersInvalidUid(t *testing.T) {
	client := &Client{ baseClient: &baseClient{} }

	getUsersResult, err := client.GetUsers(
		context.Background(),
		[]UserIdentifier{&UIDIdentifier{"too long " + strings.Repeat(".", 128)}})
	want := "uid string must not be longer than 128 characters"
	if getUsersResult != nil || err == nil || err.Error() != want {
		t.Errorf("GetUsers() = (%v, %q); want = (nil, %q)", getUsersResult, err, want)
	}
}

func TestGetUsersInvalidEmail(t *testing.T) {
	client := &Client{ baseClient: &baseClient{} }

	getUsersResult, err := client.GetUsers(
		context.Background(),
		[]UserIdentifier{EmailIdentifier{"invalid email addr"}})
	want := `malformed email string: "invalid email addr"`
	if getUsersResult != nil || err == nil || err.Error() != want {
		t.Errorf("GetUsers() = (%v, %q); want = (nil, %q)", getUsersResult, err, want)
	}
}

func TestGetUsersInvalidPhoneNumber(t *testing.T) {
	client := &Client{ baseClient: &baseClient{} }

	getUsersResult, err := client.GetUsers(context.Background(), []UserIdentifier{
		PhoneIdentifier{"invalid phone number"},
	})
	want := "phone number must be a valid, E.164 compliant identifier"
	if getUsersResult != nil || err == nil || err.Error() != want {
		t.Errorf("GetUsers() = (%v, %q); want = (nil, %q)", getUsersResult, err, want)
	}
}

func TestGetUsersInvalidProvider(t *testing.T) {
	client := &Client{ baseClient: &baseClient{} }

	getUsersResult, err := client.GetUsers(context.Background(), []UserIdentifier{
		ProviderIdentifier{ProviderID: "", ProviderUID: ""},
	})
	want := "providerID must be a non-empty string"
	if getUsersResult != nil || err == nil || err.Error() != want {
		t.Errorf("GetUsers() = (%v, %q); want = (nil, %q)", getUsersResult, err, want)
	}
}

func TestGetUsersSingleBadIdentifier(t *testing.T) {
	client := &Client{ baseClient: &baseClient{} }

	identifiers := []UserIdentifier{
		UIDIdentifier{"valid_id1"},
		UIDIdentifier{"valid_id2"},
		UIDIdentifier{"invalid id; too long. " + strings.Repeat(".", 128)},
		UIDIdentifier{"valid_id3"},
		UIDIdentifier{"valid_id4"},
	}

	getUsersResult, err := client.GetUsers(context.Background(), identifiers)
	want := "uid string must not be longer than 128 characters"
	if getUsersResult != nil || err == nil || err.Error() != want {
		t.Errorf("GetUsers() = (%v, %q); want = (nil, %q)", getUsersResult, err, want)
	}
}

func TestInvalidCreateUser(t *testing.T) {
	cases := []struct {
		params *UserToCreate
		want   string
	}{
		{
			(&UserToCreate{}).Password("short"),
			"password must be a string at least 6 characters long",
		}, {
			(&UserToCreate{}).PhoneNumber(""),
			"phone number must be a non-empty string",
		}, {
			(&UserToCreate{}).PhoneNumber("1234"),
			"phone number must be a valid, E.164 compliant identifier",
		}, {
			(&UserToCreate{}).PhoneNumber("+_!@#$"),
			"phone number must be a valid, E.164 compliant identifier",
		}, {
			(&UserToCreate{}).UID(""),
			"uid must be a non-empty string",
		}, {
			(&UserToCreate{}).UID(strings.Repeat("a", 129)),
			"uid string must not be longer than 128 characters",
		}, {
			(&UserToCreate{}).DisplayName(""),
			"display name must be a non-empty string",
		}, {
			(&UserToCreate{}).PhotoURL(""),
			"photo url must be a non-empty string",
		}, {
			(&UserToCreate{}).Email(""),
			"email must be a non-empty string",
		}, {
			(&UserToCreate{}).Email("a"),
			`malformed email string: "a"`,
		}, {
			(&UserToCreate{}).Email("a@"),
			`malformed email string: "a@"`,
		}, {
			(&UserToCreate{}).Email("@a"),
			`malformed email string: "@a"`,
		}, {
			(&UserToCreate{}).Email("a@a@a"),
			`malformed email string: "a@a@a"`,
		}, {
			(&UserToCreate{}).MFASettings(MultiFactorSettings{
				EnrolledFactors: []*MultiFactorInfo{
					{
						UID: "EnrollmentID",
						Phone: &PhoneMultiFactorInfo{
							PhoneNumber: "+11234567890",
						},
						DisplayName: "Spouse's phone number",
						FactorID:    "phone",
					},
				},
			}),
			`"uid" is not supported when adding second factors via "createUser()"`,
		}, {
			(&UserToCreate{}).MFASettings(MultiFactorSettings{
				EnrolledFactors: []*MultiFactorInfo{
					{
						Phone: &PhoneMultiFactorInfo{
							PhoneNumber: "invalid",
						},
						DisplayName: "Spouse's phone number",
						FactorID:    "phone",
					},
				},
			}),
			`the second factor "phoneNumber" for "invalid" must be a non-empty E.164 standard compliant identifier string`,
		}, {
			(&UserToCreate{}).MFASettings(MultiFactorSettings{
				EnrolledFactors: []*MultiFactorInfo{
					{
						Phone: &PhoneMultiFactorInfo{
							PhoneNumber: "+11234567890",
						},
						DisplayName:         "Spouse's phone number",
						FactorID:            "phone",
						EnrollmentTimestamp: time.Now().UTC().Unix(),
					},
				},
			}),
			`"EnrollmentTimeStamp" is not supported when adding second factors via "createUser()"`,
		}, {
			(&UserToCreate{}).MFASettings(MultiFactorSettings{
				EnrolledFactors: []*MultiFactorInfo{
					{
						Phone: &PhoneMultiFactorInfo{
							PhoneNumber: "+11234567890",
						},
						DisplayName: "Spouse's phone number",
						FactorID:    "",
					},
				},
			}),
			`no factor id specified`,
		}, {
			(&UserToCreate{}).MFASettings(MultiFactorSettings{
				EnrolledFactors: []*MultiFactorInfo{
					{
						Phone: &PhoneMultiFactorInfo{
							PhoneNumber: "+11234567890",
						},
						FactorID: "phone",
					},
				},
			}),
			`the second factor "displayName" for "" must be a valid non-empty string`,
		},
	}
	client := &Client{
		baseClient: &baseClient{},
	}
	for i, tc := range cases {
		user, err := client.CreateUser(context.Background(), tc.params)
		if user != nil || err == nil {
			t.Errorf("[%d] CreateUser() = (%v, %v); want = (nil, error)", i, user, err)
		}
		if err.Error() != tc.want {
			t.Errorf("[%d] CreateUser() = %v; want = %v", i, err.Error(), tc.want)
		}
	}
}

var createUserCases = []struct {
	params *UserToCreate
	req    map[string]interface{}
}{
	{
		nil,
		map[string]interface{}{},
	},
	{
		&UserToCreate{},
		map[string]interface{}{},
	},
	{
		(&UserToCreate{}).Password("123456"),
		map[string]interface{}{"password": "123456"},
	},
	{
		(&UserToCreate{}).UID("1"),
		map[string]interface{}{"localId": "1"},
	},
	{
		(&UserToCreate{}).UID(strings.Repeat("a", 128)),
		map[string]interface{}{"localId": strings.Repeat("a", 128)},
	},
	{
		(&UserToCreate{}).PhoneNumber("+1"),
		map[string]interface{}{"phoneNumber": "+1"},
	},
	{
		(&UserToCreate{}).DisplayName("a"),
		map[string]interface{}{"displayName": "a"},
	},
	{
		(&UserToCreate{}).Email("a@a"),
		map[string]interface{}{"email": "a@a"},
	},
	{
		(&UserToCreate{}).Disabled(true),
		map[string]interface{}{"disabled": true},
	},
	{
		(&UserToCreate{}).Disabled(false),
		map[string]interface{}{"disabled": false},
	},
	{
		(&UserToCreate{}).EmailVerified(true),
		map[string]interface{}{"emailVerified": true},
	},
	{
		(&UserToCreate{}).EmailVerified(false),
		map[string]interface{}{"emailVerified": false},
	},
	{
		(&UserToCreate{}).PhotoURL("http://some.url"),
		map[string]interface{}{"photoUrl": "http://some.url"},
	}, {
		(&UserToCreate{}).MFASettings(MultiFactorSettings{
			EnrolledFactors: []*MultiFactorInfo{
				{
					Phone: &PhoneMultiFactorInfo{
						PhoneNumber: "+11234567890",
					},
					DisplayName: "Phone Number active",
					FactorID:    "phone",
				},
				{
					PhoneNumber: "+11234567890",
					DisplayName: "Phone Number deprecated",
					FactorID:    "phone",
				},
			},
		}),
		map[string]interface{}{"mfaInfo": []*multiFactorInfoResponse{
			{
				PhoneInfo:   "+11234567890",
				DisplayName: "Phone Number active",
			},
			{
				PhoneInfo:   "+11234567890",
				DisplayName: "Phone Number deprecated",
			},
		},
		},
	}, {
		(&UserToCreate{}).MFASettings(MultiFactorSettings{
			EnrolledFactors: []*MultiFactorInfo{
				{
					Phone: &PhoneMultiFactorInfo{
						PhoneNumber: "+11234567890",
					},
					DisplayName: "number1",
					FactorID:    "phone",
				},
				{
					Phone: &PhoneMultiFactorInfo{
						PhoneNumber: "+11234567890",
					},
					DisplayName: "number2",
					FactorID:    "phone",
				},
			},
		}),
		map[string]interface{}{"mfaInfo": []*multiFactorInfoResponse{
			{
				PhoneInfo:   "+11234567890",
				DisplayName: "number1",
			},
			{
				PhoneInfo:   "+11234567890",
				DisplayName: "number2",
			},
		},
		},
	},
}

func TestInvalidUpdateUser(t *testing.T) {
	cases := []struct {
		params *UserToUpdate
		want   string
	}{
		{
			nil,
			"update parameters must not be nil or empty",
		}, {
			&UserToUpdate{},
			"update parameters must not be nil or empty",
		}, {
			(&UserToUpdate{}).Email(""),
			"email must be a non-empty string",
		}, {
			(&UserToUpdate{}).Email("invalid"),
			`malformed email string: "invalid"`,
		}, {
			(&UserToUpdate{}).PhoneNumber("1"),
			"phone number must be a valid, E.164 compliant identifier",
		}, {
			(&UserToUpdate{}).CustomClaims(map[string]interface{}{"a": strings.Repeat("a", 993)}),
			"serialized custom claims must not exceed 1000 characters",
		}, {
			(&UserToUpdate{}).Password("short"),
			"password must be a string at least 6 characters long",
		}, {
			(&UserToUpdate{}).MFASettings(MultiFactorSettings{
				EnrolledFactors: []*MultiFactorInfo{
					{
						UID: "enrolledSecondFactor1",
						Phone: &PhoneMultiFactorInfo{
							PhoneNumber: "+11234567890",
						},
						FactorID: "phone",
					},
				},
			}),
			`the second factor "displayName" for "" must be a valid non-empty string`,
		}, {
			(&UserToUpdate{}).MFASettings(MultiFactorSettings{
				EnrolledFactors: []*MultiFactorInfo{
					{
						UID: "enrolledSecondFactor1",
						Phone: &PhoneMultiFactorInfo{
							PhoneNumber: "invalid",
						},
						DisplayName: "Spouse's phone number",
						FactorID:    "phone",
					},
				},
			}),
			`the second factor "phoneNumber" for "invalid" must be a non-empty E.164 standard compliant identifier string`,
		}, {
			(&UserToUpdate{}).ProviderToLink(&UserProvider{UID: "google_uid"}),
			"user provider must specify a provider ID",
		}, {
			(&UserToUpdate{}).ProviderToLink(&UserProvider{ProviderID: "google.com"}),
			"user provider must specify a uid",
		}, {
			(&UserToUpdate{}).ProviderToLink(&UserProvider{ProviderID: "google.com", UID: ""}),
			"user provider must specify a uid",
		}, {
			(&UserToUpdate{}).ProviderToLink(&UserProvider{ProviderID: "", UID: "google_uid"}),
			"user provider must specify a provider ID",
		}, {
			(&UserToUpdate{}).ProvidersToDelete([]string{""}),
			"providersToDelete must not include empty strings",
		}, {
			(&UserToUpdate{}).
				Email("user@example.com").
				ProviderToLink(&UserProvider{
					ProviderID: "email",
					UID:        "user@example.com",
				}),
			"both UserToUpdate.Email and UserToUpdate.ProviderToLink.ProviderID='email' " +
				"were set; to link to the email/password provider, only specify the " +
				"UserToUpdate.Email field",
		}, {
			(&UserToUpdate{}).
				PhoneNumber("+15555550001").
				ProviderToLink(&UserProvider{
					ProviderID: "phone",
					UID:        "+15555550001",
				}),
			"both UserToUpdate.PhoneNumber and UserToUpdate.ProviderToLink.ProviderID='phone' " +
				"were set; to link to the phone provider, only specify the " +
				"UserToUpdate.PhoneNumber field",
		}, {
			(&UserToUpdate{}).
				PhoneNumber("").
				ProvidersToDelete([]string{"phone"}),
			"both UserToUpdate.PhoneNumber='' and " +
				"UserToUpdate.ProvidersToDelete=['phone'] were set; to unlink from a " +
				"phone provider, only specify the UserToUpdate.PhoneNumber='' field",
		},
	}

	for _, claim := range reservedClaims {
		s := struct {
			params *UserToUpdate
			want   string
		}{
			(&UserToUpdate{}).CustomClaims(map[string]interface{}{claim: true}),
			fmt.Sprintf("claim %q is reserved and must not be set", claim),
		}
		cases = append(cases, s)
	}

	client := &Client{
		baseClient: &baseClient{},
	}
	for i, tc := range cases {
		user, err := client.UpdateUser(context.Background(), "uid", tc.params)
		if user != nil || err == nil {
			t.Errorf("[%d] UpdateUser() = (%v, %v); want = (nil, error)", i, user, err)
		}
		if err.Error() != tc.want {
			t.Errorf("[%d] UpdateUser() = %v; want = %v", i, err.Error(), tc.want)
		}
	}
}

func TestUpdateUserEmptyUID(t *testing.T) {
	params := (&UserToUpdate{}).DisplayName("test")
	client := &Client{
		baseClient: &baseClient{},
	}
	user, err := client.UpdateUser(context.Background(), "", params)
	if user != nil || err == nil {
		t.Errorf("UpdateUser('') = (%v, %v); want = (nil, error)", user, err)
	}
}

var updateUserCases = []struct {
	params *UserToUpdate
	req    map[string]interface{}
}{
	{
		(&UserToUpdate{}).Password("123456"),
		map[string]interface{}{"password": "123456"},
	},
	{
		(&UserToUpdate{}).PhoneNumber("+1"),
		map[string]interface{}{"phoneNumber": "+1"},
	},
	{
		(&UserToUpdate{}).DisplayName("a"),
		map[string]interface{}{"displayName": "a"},
	},
	{
		(&UserToUpdate{}).Email("a@a"),
		map[string]interface{}{"email": "a@a"},
	},
	{
		(&UserToUpdate{}).Disabled(true),
		map[string]interface{}{"disableUser": true},
	},
	{
		(&UserToUpdate{}).Disabled(false),
		map[string]interface{}{"disableUser": false},
	},
	{
		(&UserToUpdate{}).EmailVerified(true),
		map[string]interface{}{"emailVerified": true},
	},
	{
		(&UserToUpdate{}).EmailVerified(false),
		map[string]interface{}{"emailVerified": false},
	},
	{
		(&UserToUpdate{}).PhotoURL("http://some.url"),
		map[string]interface{}{"photoUrl": "http://some.url"},
	},
	{
		(&UserToUpdate{}).DisplayName(""),
		map[string]interface{}{"deleteAttribute": []string{"DISPLAY_NAME"}},
	},
	{
		(&UserToUpdate{}).PhoneNumber(""),
		map[string]interface{}{"deleteProvider": []string{"phone"}},
	},
	{
		(&UserToUpdate{}).PhotoURL(""),
		map[string]interface{}{"deleteAttribute": []string{"PHOTO_URL"}},
	},
	{
		(&UserToUpdate{}).PhotoURL("").PhoneNumber("").DisplayName(""),
		map[string]interface{}{
			"deleteAttribute": []string{"DISPLAY_NAME", "PHOTO_URL"},
			"deleteProvider":  []string{"phone"},
		},
	},
	{
		(&UserToUpdate{}).MFASettings(MultiFactorSettings{
			EnrolledFactors: []*MultiFactorInfo{
				{
					UID: "enrolledSecondFactor1",
					Phone: &PhoneMultiFactorInfo{
						PhoneNumber: "+11234567890",
					},
					DisplayName:         "Spouse's phone number",
					FactorID:            "phone",
					EnrollmentTimestamp: time.Now().Unix(),
				}, {
					UID: "enrolledSecondFactor2",
					Phone: &PhoneMultiFactorInfo{
						PhoneNumber: "+11234567890",
					},
					PhoneNumber: "+11234567890",
					DisplayName: "Spouse's phone number",
					FactorID:    "phone",
				}, {
					Phone: &PhoneMultiFactorInfo{
						PhoneNumber: "+11234567890",
					},
					PhoneNumber: "+11234567890",
					DisplayName: "Spouse's phone number",
					FactorID:    "phone",
				},
			},
		}),
		map[string]interface{}{"mfa": multiFactorEnrollments{Enrollments: []*multiFactorInfoResponse{
			{
				MFAEnrollmentID: "enrolledSecondFactor1",
				PhoneInfo:       "+11234567890",
				DisplayName:     "Spouse's phone number",
				EnrolledAt:      time.Now().Format("2006-01-02T15:04:05Z07:00Z"),
			},
			{
				MFAEnrollmentID: "enrolledSecondFactor2",
				DisplayName:     "Spouse's phone number",
				PhoneInfo:       "+11234567890",
			},
			{
				DisplayName: "Spouse's phone number",
				PhoneInfo:   "+11234567890",
			},
		}},
		},
	},
	{
		(&UserToUpdate{}).MFASettings(MultiFactorSettings{}),
		map[string]interface{}{"mfa": multiFactorEnrollments{Enrollments: nil}},
	},
	{
		(&UserToUpdate{}).ProviderToLink(&UserProvider{
			ProviderID: "google.com",
			UID:        "google_uid",
		}),
		map[string]interface{}{
			"linkProviderUserInfo": &UserProvider{
				ProviderID: "google.com",
				UID:        "google_uid",
			}},
	},
	{
		(&UserToUpdate{}).PhoneNumber("").ProvidersToDelete([]string{"google.com"}),
		map[string]interface{}{
			"deleteProvider": []string{"phone", "google.com"},
		},
	},
	{
		(&UserToUpdate{}).ProvidersToDelete([]string{"email", "phone"}),
		map[string]interface{}{
			"deleteProvider": []string{"email", "phone"},
		},
	},
	{
		(&UserToUpdate{}).ProviderToLink(&UserProvider{
			ProviderID: "email",
			UID:        "user@example.com",
		}),
		map[string]interface{}{"email": "user@example.com"},
	},
	{
		(&UserToUpdate{}).ProviderToLink(&UserProvider{
			ProviderID: "phone",
			UID:        "+15555550001",
		}),
		map[string]interface{}{"phoneNumber": "+15555550001"},
	},
	{
		(&UserToUpdate{}).CustomClaims(map[string]interface{}{"a": strings.Repeat("a", 992)}),
		map[string]interface{}{"customAttributes": fmt.Sprintf(`{"a":%q}`, strings.Repeat("a", 992))},
	},
	{
		(&UserToUpdate{}).CustomClaims(map[string]interface{}{}),
		map[string]interface{}{"customAttributes": "{}"},
	},
	{
		(&UserToUpdate{}).CustomClaims(nil),
		map[string]interface{}{"customAttributes": "{}"},
	},
}

func TestUserProvider(t *testing.T) {
	cases := []struct {
		provider *UserProvider
		want     map[string]interface{}
	}{
		{
			provider: &UserProvider{UID: "test", ProviderID: "google.com"},
			want:     map[string]interface{}{"rawId": "test", "providerId": "google.com"},
		},
		{
			provider: &UserProvider{
				UID:         "test",
				ProviderID:  "google.com",
				DisplayName: "Test User",
			},
			want: map[string]interface{}{
				"rawId":       "test",
				"providerId":  "google.com",
				"displayName": "Test User",
			},
		},
		{
			provider: &UserProvider{
				UID:         "test",
				ProviderID:  "google.com",
				DisplayName: "Test User",
				Email:       "test@example.com",
				PhotoURL:    "https://test.com/user.png",
			},
			want: map[string]interface{}{
				"rawId":       "test",
				"providerId":  "google.com",
				"displayName": "Test User",
				"email":       "test@example.com",
				"photoUrl":    "https://test.com/user.png",
			},
		},
	}

	for idx, tc := range cases {
		b, err := json.Marshal(tc.provider)
		if err != nil {
			t.Fatal(err)
		}

		var got map[string]interface{}
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("[%d] UserProvider = %#v; want = %#v", idx, got, tc.want)
		}
	}
}

func TestUserToImport(t *testing.T) {
	cases := []struct {
		user *UserToImport
		want map[string]interface{}
	}{
		{
			user: (&UserToImport{}).UID("test"),
			want: map[string]interface{}{
				"localId": "test",
			},
		},
		{
			user: (&UserToImport{}).UID("test").DisplayName("name"),
			want: map[string]interface{}{
				"localId":     "test",
				"displayName": "name",
			},
		},
		{
			user: (&UserToImport{}).UID("test").Email("test@example.com"),
			want: map[string]interface{}{
				"localId": "test",
				"email":   "test@example.com",
			},
		},
		{
			user: (&UserToImport{}).UID("test").PhotoURL("https://test.com/user.png"),
			want: map[string]interface{}{
				"localId":  "test",
				"photoUrl": "https://test.com/user.png",
			},
		},
		{
			user: (&UserToImport{}).UID("test").PhoneNumber("+1234567890"),
			want: map[string]interface{}{
				"localId":     "test",
				"phoneNumber": "+1234567890",
			},
		},
		{
			user: (&UserToImport{}).UID("test").Metadata(&UserMetadata{
				CreationTimestamp:  int64(100),
				LastLogInTimestamp: int64(150),
			}),
			want: map[string]interface{}{
				"localId":     "test",
				"createdAt":   int64(100),
				"lastLoginAt": int64(150),
			},
		},
		{
			user: (&UserToImport{}).UID("test").Metadata(&UserMetadata{
				CreationTimestamp: int64(100),
			}),
			want: map[string]interface{}{
				"localId":   "test",
				"createdAt": int64(100),
			},
		},
		{
			user: (&UserToImport{}).UID("test").Metadata(&UserMetadata{
				LastLogInTimestamp: int64(150),
			}),
			want: map[string]interface{}{
				"localId":     "test",
				"lastLoginAt": int64(150),
			},
		},
		{
			user: (&UserToImport{}).UID("test").PasswordHash([]byte("password")),
			want: map[string]interface{}{
				"localId":      "test",
				"passwordHash": base64.RawURLEncoding.EncodeToString([]byte("password")),
			},
		},
		{
			user: (&UserToImport{}).UID("test").PasswordSalt([]byte("nacl")),
			want: map[string]interface{}{
				"localId": "test",
				"salt":    base64.RawURLEncoding.EncodeToString([]byte("nacl")),
			},
		},
		{
			user: (&UserToImport{}).UID("test").CustomClaims(map[string]interface{}{"admin": true}),
			want: map[string]interface{}{
				"localId":          "test",
				"customAttributes": `{"admin":true}`,
			},
		},
		{
			user: (&UserToImport{}).UID("test").CustomClaims(map[string]interface{}{}),
			want: map[string]interface{}{
				"localId": "test",
			},
		},
		{
			user: (&UserToImport{}).UID("test").ProviderData([]*UserProvider{
				{
					ProviderID: "google.com",
					UID:        "test",
				},
			}),
			want: map[string]interface{}{
				"localId": "test",
				"providerUserInfo": []*UserProvider{
					{
						ProviderID: "google.com",
						UID:        "test",
					},
				},
			},
		},
		{
			user: (&UserToImport{}).UID("test").EmailVerified(true),
			want: map[string]interface{}{
				"localId":       "test",
				"emailVerified": true,
			},
		},
		{
			user: (&UserToImport{}).UID("test").EmailVerified(false),
			want: map[string]interface{}{
				"localId":       "test",
				"emailVerified": false,
			},
		},
		{
			user: (&UserToImport{}).UID("test").Disabled(true),
			want: map[string]interface{}{
				"localId":  "test",
				"disabled": true,
			},
		},
		{
			user: (&UserToImport{}).UID("test").Disabled(false),
			want: map[string]interface{}{
				"localId":  "test",
				"disabled": false,
			},
		},
	}

	for idx, tc := range cases {
		got, err := tc.user.validatedUserInfo()
		if err != nil {
			t.Errorf("[%d] invalid user: %v", idx, err)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("[%d] UserToImport = %#v; want = %#v", idx, got, tc.want)
		}
	}
}

func TestUserToImportError(t *testing.T) {
	cases := []struct {
		user *UserToImport
		want string
	}{
		{
			&UserToImport{},
			"no parameters are set on the user to import",
		},
		{
			(&UserToImport{}).UID(""),
			"uid must be a non-empty string",
		},
		{
			(&UserToImport{}).UID(strings.Repeat("a", 129)),
			"uid string must not be longer than 128 characters",
		},
		{
			(&UserToImport{}).UID("test").Email("not-an-email"),
			`malformed email string: "not-an-email"`,
		},
		{
			(&UserToImport{}).UID("test").PhoneNumber("not-a-phone"),
			"phone number must be a valid, E.164 compliant identifier",
		},
		{
			(&UserToImport{}).UID("test").CustomClaims(map[string]interface{}{"key": strings.Repeat("a", 1000)}),
			"serialized custom claims must not exceed 1000 characters",
		},
		{
			(&UserToImport{}).UID("test").ProviderData([]*UserProvider{
				{
					UID: "test",
				},
			}),
			"user provider must specify a provider ID",
		},
		{
			(&UserToImport{}).UID("test").ProviderData([]*UserProvider{
				{
					ProviderID: "google.com",
				},
			}),
			"user provider must specify a uid",
		},
	}

	ctx := context.Background()
	appInstance := newTestApp(ctx, "mock-project-id", "", appOptsWithTokenSource...)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatalf("NewClient() err = %v", err)
	}


	for idx, tc := range cases {
		_, err := client.ImportUsers(context.Background(), []*UserToImport{tc.user})
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("[%d] UserToImport = %v; want error containing %q", idx, err, tc.want)
		}
	}
}

func TestInvalidImportUsers(t *testing.T) {
	ctx := context.Background()
	appInstance := newTestApp(ctx, "mock-project-id", "", appOptsWithTokenSource...)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatalf("NewClient() err = %v", err)
	}

	result, err := client.ImportUsers(context.Background(), nil)
	if result != nil || err == nil {
		t.Errorf("ImportUsers(nil) = (%v, %v); want = (nil, error)", result, err)
	}

	result, err = client.ImportUsers(context.Background(), []*UserToImport{})
	if result != nil || err == nil {
		t.Errorf("ImportUsers([]) = (%v, %v); want = (nil, error)", result, err)
	}

	var users []*UserToImport
	for i := 0; i < 1001; i++ {
		users = append(users, (&UserToImport{}).UID(fmt.Sprintf("user%d", i)))
	}
	result, err = client.ImportUsers(context.Background(), users)
	if result != nil || err == nil {
		t.Errorf("ImportUsers(len > 1000) = (%v, %v); want = (nil, error)", result, err)
	}
}


type mockHash struct {
	key, saltSep       string
	rounds, memoryCost int64
}

func (h mockHash) Config() (internal.HashConfig, error) {
	return internal.HashConfig{
		"hashAlgorithm": "MOCKHASH",
		"signerKey":     h.key,
		"saltSeparator": h.saltSep,
		"rounds":        h.rounds,
		"memoryCost":    h.memoryCost,
	}, nil
}

func TestInvalidDeleteUser(t *testing.T) {
	client := &Client{
		baseClient: &baseClient{},
	}
	if err := client.DeleteUser(context.Background(), ""); err == nil {
		t.Errorf("DeleteUser('') = nil; want error")
	}
}

func TestDeleteUsers(t *testing.T) {
	client := &Client{
		baseClient: &baseClient{},
	}

	t.Run("should succeed given an empty list", func(t *testing.T) {
		result, err := client.DeleteUsers(context.Background(), []string{})

		if err != nil {
			t.Fatalf("DeleteUsers([]) error %v; want = nil", err)
		}

		if result.SuccessCount != 0 {
			t.Errorf("DeleteUsers([]).SuccessCount = %d; want = 0", result.SuccessCount)
		}
		if result.FailureCount != 0 {
			t.Errorf("DeleteUsers([]).FailureCount = %d; want = 0", result.FailureCount)
		}
		if len(result.Errors) != 0 {
			t.Errorf("len(DeleteUsers([]).Errors) = %d; want = 0", len(result.Errors))
		}
	})

	t.Run("should be rejected when given more than 1000 identifiers", func(t *testing.T) {
		uids := []string{}
		for i := 0; i < 1001; i++ {
			uids = append(uids, fmt.Sprintf("id%d", i))
		}

		_, err := client.DeleteUsers(context.Background(), uids)
		if err == nil {
			t.Fatalf("DeleteUsers([too_many_uids]) error nil; want not nil")
		}

		if err.Error() != "`uids` parameter must have <= 1000 entries" {
			t.Errorf(
				"DeleteUsers([too_many_uids]) returned an error of '%s'; "+
					"expected '`uids` parameter must have <= 1000 entries'",
				err.Error())
		}
	})

	t.Run("should immediately fail given an invalid id", func(t *testing.T) {
		tooLongUID := "too long " + strings.Repeat(".", 128)
		_, err := client.DeleteUsers(context.Background(), []string{tooLongUID})

		if err == nil {
			t.Fatalf("DeleteUsers([too_long_uid]) error nil; want not nil")
		}

		if err.Error() != "uid string must not be longer than 128 characters" {
			t.Errorf(
				"DeleteUsers([too_long_uid]) returned an error of '%s'; "+
					"expected 'uid string must not be longer than 128 characters'",
				err.Error())
		}
	})

	t.Run("should index errors correctly in result", func(t *testing.T) {
		resp := `{
      "errors": [{
        "index": 0,
        "localId": "uid1",
        "message": "Error Message 1"
      }, {
        "index": 2,
        "localId": "uid3",
        "message": "Error Message 2"
      }]
    }`
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(resp))
		}))
		defer server.Close()

		ctx := context.Background()
		appInstance := newTestApp(ctx, "mock-project-id", "", appOptsWithTokenSource...)
		client, err := NewClient(ctx, appInstance)
		if err != nil {
			t.Fatalf("NewClient() err = %v", err)
		}
		client.baseClient.userManagementEndpoint = server.URL


		result, err := client.DeleteUsers(context.Background(), []string{"uid1", "uid2", "uid3", "uid4"})

		if err != nil {
			t.Fatalf("DeleteUsers([...]) error %v; want = nil", err)
		}

		if result.SuccessCount != 2 {
			t.Errorf("DeleteUsers([...]).SuccessCount = %d; want 2", result.SuccessCount)
		}
		if result.FailureCount != 2 {
			t.Errorf("DeleteUsers([...]).FailureCount = %d; want 2", result.FailureCount)
		}
		if len(result.Errors) != 2 {
			t.Errorf("len(DeleteUsers([...]).Errors) = %d; want 2", len(result.Errors))
		} else {
			if result.Errors[0].Index != 0 {
				t.Errorf("DeleteUsers([...]).Errors[0].Index = %d; want 0", result.Errors[0].Index)
			}
			if result.Errors[0].Reason != "Error Message 1" {
				t.Errorf("DeleteUsers([...]).Errors[0].Reason = %s; want Error Message 1", result.Errors[0].Reason)
			}
			if result.Errors[1].Index != 2 {
				t.Errorf("DeleteUsers([...]).Errors[1].Index = %d; want 2", result.Errors[1].Index)
			}
			if result.Errors[1].Reason != "Error Message 2" {
				t.Errorf("DeleteUsers([...]).Errors[1].Reason = %s; want Error Message 2", result.Errors[1].Reason)
			}
		}
	})
}

func TestMakeExportedUser(t *testing.T) {
	queryResponse := &userQueryResponse{
		UID:                "testuser",
		Email:              "testuser@example.com",
		PhoneNumber:        "+1234567890",
		EmailVerified:      true,
		DisplayName:        "Test User",
		PasswordSalt:       "salt",
		PhotoURL:           "http://www.example.com/testuser/photo.png",
		PasswordHash:       "passwordhash",
		ValidSinceSeconds:  1494364393,
		Disabled:           false,
		CreationTimestamp:  1234567890000,
		LastLogInTimestamp: 1233211232000,
		CustomAttributes:   `{"admin": true, "package": "gold"}`,
		TenantID:           "testTenant",
		ProviderUserInfo: []*UserInfo{
			{
				ProviderID:  "password",
				DisplayName: "Test User",
				PhotoURL:    "http://www.example.com/testuser/photo.png",
				Email:       "testuser@example.com",
				UID:         "testuid",
			}, {
				ProviderID:  "phone",
				PhoneNumber: "+1234567890",
				UID:         "testuid",
			}},
		MFAInfo: []*multiFactorInfoResponse{
			{
				PhoneInfo:       "+1234567890",
				MFAEnrollmentID: "enrolledPhoneFactor",
				DisplayName:     "My MFA Phone",
				EnrolledAt:      "2021-03-03T13:06:20.542896Z",
			},
			{
				TOTPInfo:        &TOTPInfo{},
				MFAEnrollmentID: "enrolledTOTPFactor",
				DisplayName:     "My MFA TOTP",
				EnrolledAt:      "2021-03-03T13:06:20.542896Z",
			},
		},
	}

	want := &ExportedUserRecord{
		UserRecord:   testUser,
		PasswordHash: "passwordhash",
		PasswordSalt: "salt",
	}
	exported, err := queryResponse.makeExportedUserRecord()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(exported.UserRecord, want.UserRecord) {
		t.Errorf("makeExportedUser() = %#v; want: %#v \n(%#v)\n(%#v)", exported.UserRecord, want.UserRecord,
			exported.UserMetadata, want.UserMetadata)
	}
	if exported.PasswordHash != want.PasswordHash {
		t.Errorf("PasswordHash = %q; want = %q", exported.PasswordHash, want.PasswordHash)
	}
	if exported.PasswordSalt != want.PasswordSalt {
		t.Errorf("PasswordSalt = %q; want = %q", exported.PasswordSalt, want.PasswordSalt)
	}
}

func TestUnsupportedAuthFactor(t *testing.T) {
	queryResponse := &userQueryResponse{
		UID: "uid1",
		MFAInfo: []*multiFactorInfoResponse{
			{
				MFAEnrollmentID: "enrollementId",
			},
		},
	}

	exported, err := queryResponse.makeExportedUserRecord()
	if exported != nil || err == nil {
		t.Errorf("makeExportedUserRecord() = (%v, %v); want = (nil, error)", exported, err)
	}
}

func TestExportedUserRecordShouldClearRedacted(t *testing.T) {
	queryResponse := &userQueryResponse{
		UID:          "uid1",
		PasswordHash: base64.StdEncoding.EncodeToString([]byte("REDACTED")),
	}

	exported, err := queryResponse.makeExportedUserRecord()
	if err != nil {
		t.Fatal(err)
	}
	if exported.PasswordHash != "" {
		t.Errorf("PasswordHash = %q; want = ''", exported.PasswordHash)
	}
}

var createSessionCookieCases = []struct {
	expiresIn time.Duration
	want      float64
}{
	{
		expiresIn: 10 * time.Minute,
		want:      600.0,
	},
	{
		expiresIn: 300500 * time.Millisecond,
		want:      300.0,
	},
}

func TestInvalidSetCustomClaims(t *testing.T) {
	cases := []struct {
		cc   map[string]interface{}
		want string
	}{
		{
			map[string]interface{}{"a": strings.Repeat("a", 993)},
			"serialized custom claims must not exceed 1000 characters",
		},
		{
			map[string]interface{}{"a": func() {}},
			"custom claims marshaling error: json: unsupported type: func()",
		},
	}

	for _, res := range reservedClaims {
		s := struct {
			cc   map[string]interface{}
			want string
		}{
			map[string]interface{}{res: true},
			fmt.Sprintf("claim %q is reserved and must not be set", res),
		}
		cases = append(cases, s)
	}

	client := &Client{
		baseClient: &baseClient{}, // Minimal client for input validation
	}
	for _, tc := range cases {
		err := client.SetCustomUserClaims(context.Background(), "uid", tc.cc)
		if err == nil {
			t.Errorf("SetCustomUserClaims() = nil; want error: %s", tc.want)
		} else if err.Error() != tc.want {
			t.Errorf("SetCustomUserClaims() = %q; want = %q", err.Error(), tc.want)
		}
	}
}

// Removed duplicated logFatal function. It should be defined in auth_test.go or a shared test utility.
