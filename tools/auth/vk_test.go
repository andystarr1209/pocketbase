package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestNewVKProviderDefaults(t *testing.T) {
	t.Parallel()

	p := NewVKProvider()

	if !p.PKCE() {
		t.Fatal("Expected PKCE to be enabled")
	}

	if p.AuthURL() != "https://id.vk.ru/authorize" {
		t.Fatalf("Expected VK ID authURL, got %q", p.AuthURL())
	}

	if p.TokenURL() != "https://id.vk.ru/oauth2/auth" {
		t.Fatalf("Expected VK ID tokenURL, got %q", p.TokenURL())
	}

	if p.UserInfoURL() != "https://id.vk.ru/oauth2/user_info" {
		t.Fatalf("Expected VK ID userInfoURL, got %q", p.UserInfoURL())
	}

	scopes := p.Scopes()
	if len(scopes) != 1 || scopes[0] != "vkid.personal_info" {
		t.Fatalf("Expected vkid.personal_info scope, got %#v", scopes)
	}
}

func TestVKFetchRawUserInfoUsesPost(t *testing.T) {
	t.Parallel()

	token := &oauth2.Token{AccessToken: "test_access_token"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("Expected %s request, got %s", http.MethodPost, r.Method)
		}

		if authHeader := r.Header.Get("Authorization"); authHeader != "Bearer test_access_token" {
			t.Fatalf("Expected Authorization header to be set, got %q", authHeader)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user":{"id":1}}`))
	}))
	defer srv.Close()

	p := NewVKProvider()
	p.SetContext(context.Background())
	p.SetUserInfoURL(srv.URL)

	data, err := p.FetchRawUserInfo(token)
	if err != nil {
		t.Fatalf("Expected nil, got error %v", err)
	}

	if string(data) != `{"user":{"id":1}}` {
		t.Fatalf("Unexpected response body %s", string(data))
	}
}

func TestVKFetchAuthUser(t *testing.T) {
	t.Parallel()

	token := &oauth2.Token{
		AccessToken:  "access_tk",
		RefreshToken: "refresh_tk",
		Expiry:       time.Date(2026, 3, 24, 10, 0, 0, 0, time.UTC),
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"user":{"id":123,"first_name":"Ivan","last_name":"Petrov","avatar":"https://cdn.example/avatar.png","email":"ivan@example.com"}}`))
	}))
	defer srv.Close()

	p := NewVKProvider()
	p.SetContext(context.Background())
	p.SetUserInfoURL(srv.URL)

	u, err := p.FetchAuthUser(token)
	if err != nil {
		t.Fatalf("Expected nil, got error %v", err)
	}

	if u.Id != "123" {
		t.Fatalf("Expected user id 123, got %q", u.Id)
	}

	if u.Name != "Ivan Petrov" {
		t.Fatalf("Expected user name Ivan Petrov, got %q", u.Name)
	}

	if u.AvatarURL != "https://cdn.example/avatar.png" {
		t.Fatalf("Expected avatar url to be mapped, got %q", u.AvatarURL)
	}

	if u.Email != "ivan@example.com" {
		t.Fatalf("Expected email to be mapped, got %q", u.Email)
	}

	if u.AccessToken != "access_tk" || u.RefreshToken != "refresh_tk" {
		t.Fatalf("Expected tokens to be mapped, got access=%q refresh=%q", u.AccessToken, u.RefreshToken)
	}
}

func TestVKFetchAuthUserMissingUserEntry(t *testing.T) {
	t.Parallel()

	token := &oauth2.Token{AccessToken: "test_access_token"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"user":{"id":0}}`))
	}))
	defer srv.Close()

	p := NewVKProvider()
	p.SetContext(context.Background())
	p.SetUserInfoURL(srv.URL)

	_, err := p.FetchAuthUser(token)
	if err == nil {
		t.Fatal("Expected error for missing user id")
	}
}
