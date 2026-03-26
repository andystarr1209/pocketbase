package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/pocketbase/pocketbase/tools/types"
	"golang.org/x/oauth2"
)

func init() {
	Providers[NameVK] = wrapFactory(NewVKProvider)
}

var _ Provider = (*VK)(nil)

// NameVK is the unique name of the VK provider.
const NameVK string = "vk"

// @todo mark as deprecated
//
// VK allows authentication via VK OAuth2.
type VK struct {
	BaseProvider
}

// NewVKProvider creates new VK provider instance with some defaults.
//
// Docs: https://id.vk.ru/about/business/go/docs/ru/vkid/latest/vk-id/connection/auth-code-flow-user
func NewVKProvider() *VK {
	return &VK{BaseProvider{
		ctx:         context.Background(),
		order:       15,
		logo:        `<svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" fill="none"><path fill="#07f" d="M0 23C0 12.2 0 6.7 3.4 3.4 6.7 0 12.2 0 23 0h2c10.8 0 16.3 0 19.6 3.4C48 6.7 48 12.2 48 23v2c0 10.8 0 16.3-3.4 19.6C41.3 48 35.8 48 25 48h-2c-10.8 0-16.3 0-19.6-3.4C0 41.3 0 35.8 0 25z"/><path fill="#fff" d="M25.5 34.6c-10.9 0-17.1-7.5-17.4-20h5.5c.2 9.2 4.2 13 7.4 13.8V14.6h5.2v7.9c3.1-.3 6.4-4 7.6-7.9h5.1a15 15 0 0 1-7 10c2.6 1.2 6.7 4.3 8.2 10h-5.7a10 10 0 0 0-8.2-7.2v7.2z"/></svg>`,
		displayName: "ВКонтакте",
		pkce:        true,
		scopes:      []string{"vkid.personal_info", "email"},
		authURL:     "https://id.vk.ru/authorize",
		tokenURL:    "https://id.vk.ru/oauth2/auth",
		userInfoURL: "https://id.vk.ru/oauth2/user_info",
	}}
}

// FetchRawUserInfo implements Provider.FetchRawUserInfo interface method.
//
// VK ID expects a POST request to /oauth2/user_info with client_id in the body
// and Authorization: Bearer <access_token> header.
func (p *VK) FetchRawUserInfo(token *oauth2.Token) ([]byte, error) {
	body := url.Values{"client_id": {p.clientId}}
	req, err := http.NewRequestWithContext(p.ctx, http.MethodPost, p.userInfoURL, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return p.sendRawUserInfoRequest(req, token)
}

// FetchAuthUser returns an AuthUser instance based on VK's user api.
//
// API reference: https://id.vk.ru/about/business/go/docs/ru/vkid/latest/vk-id/connection/api
func (p *VK) FetchAuthUser(token *oauth2.Token) (*AuthUser, error) {
	data, err := p.FetchRawUserInfo(token)
	if err != nil {
		return nil, err
	}

	rawUser := map[string]any{}
	if err := json.Unmarshal(data, &rawUser); err != nil {
		return nil, err
	}

	// VK ID wraps user fields under a "user" key; fall back to the flat map
	// in case the response structure changes.
	userMap, ok := rawUser["user"].(map[string]any)
	if !ok {
		userMap = rawUser
	}

	// Extract user ID — VK ID docs use "user.id"; some API versions use "user_id".
	userId := vkExtractStringField(userMap, "user_id", "id")
	if userId == "" {
		return nil, fmt.Errorf("missing user entry in VK response: %s", string(data))
	}

	user := &AuthUser{
		Id:           userId,
		Name:         strings.TrimSpace(vkStr(userMap, "first_name") + " " + vkStr(userMap, "last_name")),
		AvatarURL:    vkStr(userMap, "avatar"),
		RawUser:      rawUser,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
	}

	user.Expiry, _ = types.ParseDateTime(token.Expiry)

	if email := vkStr(userMap, "email"); email != "" {
		user.Email = email
	} else if email, ok := token.Extra("email").(string); ok {
		user.Email = email
	}

	return user, nil
}

// vkStr returns the string value of key from m, converting numeric JSON values as needed.
func vkStr(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return strconv.FormatInt(int64(val), 10)
	case json.Number:
		return val.String()
	default:
		return fmt.Sprintf("%v", val)
	}
}

// vkExtractStringField returns the first non-empty string found for any of the given keys.
func vkExtractStringField(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := vkStr(m, k); s != "" && s != "0" {
			return s
		}
	}
	return ""
}
