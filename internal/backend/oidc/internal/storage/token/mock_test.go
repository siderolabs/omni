// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package token_test

import (
	"time"

	"github.com/zitadel/oidc/pkg/oidc"
	"golang.org/x/text/language"

	"github.com/siderolabs/omni/internal/backend/oidc/external"
)

type mockTokenRequest struct{}

func (mockTokenRequest) GetAudience() []string {
	return []string{"test"}
}

func (mockTokenRequest) GetScopes() []string {
	return []string{
		oidc.ScopeOpenID,
		external.ScopeClusterPrefix + "cluster1",
	}
}

func (mockTokenRequest) GetSubject() string {
	return "some@example.com"
}

type mockUserInfoSetter struct {
	claims  map[string]any
	subject string
}

func (m *mockUserInfoSetter) GetSubject() string {
	return ""
}

func (m *mockUserInfoSetter) GetName() string {
	return ""
}

func (m *mockUserInfoSetter) GetGivenName() string {
	return ""
}

func (m *mockUserInfoSetter) GetFamilyName() string {
	return ""
}

func (m *mockUserInfoSetter) GetMiddleName() string {
	return ""
}

func (m *mockUserInfoSetter) GetNickname() string {
	return ""
}

func (m *mockUserInfoSetter) GetProfile() string {
	return ""
}

func (m *mockUserInfoSetter) GetPicture() string {
	return ""
}

func (m *mockUserInfoSetter) GetWebsite() string {
	return ""
}

func (m *mockUserInfoSetter) GetGender() oidc.Gender {
	return ""
}

func (m *mockUserInfoSetter) GetBirthdate() string {
	return ""
}

func (m *mockUserInfoSetter) GetZoneinfo() string {
	return ""
}

func (m *mockUserInfoSetter) GetLocale() language.Tag {
	return language.Und
}

func (m *mockUserInfoSetter) GetPreferredUsername() string {
	return ""
}

func (m *mockUserInfoSetter) GetEmail() string {
	return ""
}

func (m *mockUserInfoSetter) IsEmailVerified() bool {
	return false
}

func (m *mockUserInfoSetter) GetPhoneNumber() string {
	return ""
}

func (m *mockUserInfoSetter) IsPhoneNumberVerified() bool {
	return false
}

func (m *mockUserInfoSetter) GetAddress() oidc.UserInfoAddress {
	return nil
}

func (m *mockUserInfoSetter) GetClaim(string) any {
	return nil
}

func (m *mockUserInfoSetter) GetClaims() map[string]any {
	return nil
}

func (m *mockUserInfoSetter) SetSubject(sub string) {
	m.subject = sub
}

func (m *mockUserInfoSetter) AppendClaims(key string, values any) {
	if m.claims == nil {
		m.claims = map[string]any{}
	}

	m.claims[key] = values
}

func (m *mockUserInfoSetter) SetName(string)                  {}
func (m *mockUserInfoSetter) SetGivenName(string)             {}
func (m *mockUserInfoSetter) SetFamilyName(string)            {}
func (m *mockUserInfoSetter) SetMiddleName(string)            {}
func (m *mockUserInfoSetter) SetNickname(string)              {}
func (m *mockUserInfoSetter) SetUpdatedAt(time.Time)          {}
func (m *mockUserInfoSetter) SetProfile(string)               {}
func (m *mockUserInfoSetter) SetPicture(string)               {}
func (m *mockUserInfoSetter) SetWebsite(string)               {}
func (m *mockUserInfoSetter) SetGender(oidc.Gender)           {}
func (m *mockUserInfoSetter) SetBirthdate(string)             {}
func (m *mockUserInfoSetter) SetZoneinfo(string)              {}
func (m *mockUserInfoSetter) SetLocale(language.Tag)          {}
func (m *mockUserInfoSetter) SetPreferredUsername(string)     {}
func (m *mockUserInfoSetter) SetEmail(string, bool)           {}
func (m *mockUserInfoSetter) SetPhone(string, bool)           {}
func (m *mockUserInfoSetter) SetAddress(oidc.UserInfoAddress) {}
