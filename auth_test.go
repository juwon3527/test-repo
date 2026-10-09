package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func login(t *testing.T, server http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	return response
}

func sessionFor(t *testing.T, server http.Handler, cookies []*http.Cookie) sessionResponse {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	for _, c := range cookies {
		request.AddCookie(c)
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	var got sessionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	return got
}

func TestLoginLogoutFlow(t *testing.T) {
	server := newServer()

	response := login(t, server, `{"email":"FAN@thebridge.test","password":"bluesince1905"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d: %s", response.Code, http.StatusOK, response.Body)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != sessionCookie || !cookies[0].HttpOnly {
		t.Fatalf("unexpected cookies: %+v", cookies)
	}
	if got := sessionFor(t, server, cookies); !got.Authenticated || got.Email != "fan@thebridge.test" {
		t.Fatalf("session after login = %+v", got)
	}

	logout := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	logout.AddCookie(cookies[0])
	logoutResponse := httptest.NewRecorder()
	server.ServeHTTP(logoutResponse, logout)
	if logoutResponse.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d", logoutResponse.Code)
	}
	if got := sessionFor(t, server, cookies); got.Authenticated {
		t.Fatal("session still valid after logout")
	}
}

func TestLoginRejectsBadCredentials(t *testing.T) {
	server := newServer()
	cases := map[string]int{
		`{"email":"fan@thebridge.test","password":"wrong"}`:         http.StatusUnauthorized,
		`{"email":"nobody@example.com","password":"bluesince1905"}`: http.StatusUnauthorized,
		`{"email":"","password":""}`:                                http.StatusBadRequest,
		`not json`:                                                  http.StatusBadRequest,
	}
	for body, want := range cases {
		response := login(t, server, body)
		if response.Code != want {
			t.Errorf("login(%s) status = %d, want %d", body, response.Code, want)
		}
		if len(response.Result().Cookies()) != 0 {
			t.Errorf("login(%s) set a cookie", body)
		}
	}
}

func TestLoginPageIsServed(t *testing.T) {
	response := httptest.NewRecorder()
	newServer().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/login.html", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `id="login-form"`) {
		t.Fatalf("login page status = %d", response.Code)
	}
}
