package main

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type fakeImages struct {
	mu      sync.Mutex
	objects map[string]string // key -> content type
}

func (f *fakeImages) Put(_ context.Context, key, contentType string, _ []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[key] = contentType
	return nil
}

func (f *fakeImages) URL(_ context.Context, key string) (string, error) {
	return "https://bucket.example/" + key, nil
}

func newTestApp() *App {
	return &App{store: newMemoryStore(), images: &fakeImages{objects: map[string]string{}}}
}

type testClient struct {
	t      *testing.T
	server *httptest.Server
	http   *http.Client
}

func newTestClient(t *testing.T, app *App) *testClient {
	t.Helper()
	server := httptest.NewServer(newServer(app))
	t.Cleanup(server.Close)
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return &testClient{t: t, server: server, http: client}
}

func (c *testClient) do(request *http.Request) (*http.Response, map[string]any) {
	c.t.Helper()
	response, err := c.http.Do(request)
	if err != nil {
		c.t.Fatal(err)
	}
	defer response.Body.Close()
	var body map[string]any
	raw, _ := io.ReadAll(response.Body)
	json.Unmarshal(raw, &body)
	return response, body
}

func (c *testClient) postJSON(path, body string) (*http.Response, map[string]any) {
	c.t.Helper()
	request, _ := http.NewRequest(http.MethodPost, c.server.URL+path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	return c.do(request)
}

func (c *testClient) get(path string) (*http.Response, map[string]any) {
	c.t.Helper()
	request, _ := http.NewRequest(http.MethodGet, c.server.URL+path, nil)
	return c.do(request)
}

func (c *testClient) upload(path string, image []byte, caption string) (*http.Response, map[string]any) {
	c.t.Helper()
	var buf bytes.Buffer
	form := multipart.NewWriter(&buf)
	if caption != "" {
		form.WriteField("caption", caption)
	}
	file, _ := form.CreateFormFile("image", "photo.png")
	file.Write(image)
	form.Close()
	request, _ := http.NewRequest(http.MethodPost, c.server.URL+path, &buf)
	request.Header.Set("Content-Type", form.FormDataContentType())
	return c.do(request)
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

const signupBody = `{"name":"Blue Fan","email":"Fan@Example.com","password":"bluesince1905"}`

func TestSignupLoginLogout(t *testing.T) {
	c := newTestClient(t, newTestApp())

	response, body := c.postJSON("/api/signup", signupBody)
	if response.StatusCode != http.StatusCreated || body["email"] != "fan@example.com" {
		t.Fatalf("signup = %d %v", response.StatusCode, body)
	}
	if _, body := c.get("/api/session"); body["authenticated"] != true || body["name"] != "Blue Fan" {
		t.Fatalf("session after signup = %v", body)
	}

	if response, _ := c.postJSON("/api/logout", ""); response.StatusCode != http.StatusNoContent {
		t.Fatalf("logout = %d", response.StatusCode)
	}
	if _, body := c.get("/api/session"); body["authenticated"] != false {
		t.Fatalf("session after logout = %v", body)
	}

	response, _ = c.postJSON("/api/login", `{"email":"FAN@example.com","password":"bluesince1905"}`)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("login = %d", response.StatusCode)
	}
	cookie := response.Cookies()[0]
	if cookie.Name != sessionCookie || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("unexpected session cookie: %+v", cookie)
	}
	if _, body := c.get("/api/session"); body["authenticated"] != true {
		t.Fatalf("session after login = %v", body)
	}
}

func TestSignupRejectsInvalidAndDuplicate(t *testing.T) {
	c := newTestClient(t, newTestApp())
	if response, _ := c.postJSON("/api/signup", signupBody); response.StatusCode != http.StatusCreated {
		t.Fatalf("first signup = %d", response.StatusCode)
	}
	cases := map[string]int{
		signupBody: http.StatusConflict,
		`{"name":"","email":"a@example.com","password":"longenough"}`:                       http.StatusBadRequest,
		`{"name":"A","email":"not-an-email","password":"longenough"}`:                       http.StatusBadRequest,
		`{"name":"A","email":"Bob <b@example.com>","password":"longenough"}`:                http.StatusBadRequest,
		`{"name":"A","email":"a@example.com","password":"short"}`:                           http.StatusBadRequest,
		`{"name":"A","email":"a@example.com","password":"` + strings.Repeat("x", 73) + `"}`: http.StatusBadRequest,
		`not json`: http.StatusBadRequest,
	}
	for body, want := range cases {
		if response, _ := c.postJSON("/api/signup", body); response.StatusCode != want {
			t.Errorf("signup(%.60s) = %d, want %d", body, response.StatusCode, want)
		}
	}
}

func TestLoginRejectsBadCredentials(t *testing.T) {
	c := newTestClient(t, newTestApp())
	c.postJSON("/api/signup", signupBody)
	c.postJSON("/api/logout", "")
	cases := map[string]int{
		`{"email":"fan@example.com","password":"wrong-password"}`:   http.StatusUnauthorized,
		`{"email":"nobody@example.com","password":"bluesince1905"}`: http.StatusUnauthorized,
		`{"email":"","password":""}`:                                http.StatusBadRequest,
	}
	for body, want := range cases {
		response, _ := c.postJSON("/api/login", body)
		if response.StatusCode != want || len(response.Cookies()) != 0 {
			t.Errorf("login(%s) = %d with %d cookies, want %d and none", body, response.StatusCode, len(response.Cookies()), want)
		}
	}
}

func TestCrossOriginPostIsBlocked(t *testing.T) {
	c := newTestClient(t, newTestApp())
	request, _ := http.NewRequest(http.MethodPost, c.server.URL+"/api/signup", strings.NewReader(signupBody))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://evil.example")
	if response, _ := c.do(request); response.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-origin signup = %d, want 403", response.StatusCode)
	}
}

func TestAvatarUpload(t *testing.T) {
	app := newTestApp()
	c := newTestClient(t, app)

	if response, _ := c.upload("/api/account/avatar", pngBytes(t), ""); response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous avatar upload = %d, want 401", response.StatusCode)
	}
	c.postJSON("/api/signup", signupBody)
	response, body := c.upload("/api/account/avatar", pngBytes(t), "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("avatar upload = %d %v", response.StatusCode, body)
	}
	avatarURL, _ := body["avatarUrl"].(string)
	key := strings.TrimPrefix(avatarURL, "/media/")
	if !strings.HasPrefix(key, "avatars/") || !strings.HasSuffix(key, ".png") {
		t.Fatalf("avatarUrl = %q", avatarURL)
	}
	if got := app.images.(*fakeImages).objects[key]; got != "image/png" {
		t.Fatalf("stored content type = %q", got)
	}
	if _, body := c.get("/api/session"); body["avatarUrl"] != avatarURL {
		t.Fatalf("session avatarUrl = %v, want %s", body["avatarUrl"], avatarURL)
	}

	response, _ = c.get(avatarURL)
	if response.StatusCode != http.StatusFound || response.Header.Get("Location") != "https://bucket.example/"+key {
		t.Fatalf("media = %d -> %s", response.StatusCode, response.Header.Get("Location"))
	}
}

func TestGalleryUpload(t *testing.T) {
	c := newTestClient(t, newTestApp())
	c.postJSON("/api/signup", signupBody)

	response, body := c.upload("/api/gallery", pngBytes(t), "  Shed End, full voice  ")
	if response.StatusCode != http.StatusCreated || body["caption"] != "Shed End, full voice" || body["author"] != "Blue Fan" {
		t.Fatalf("gallery upload = %d %v", response.StatusCode, body)
	}
	_, list := c.get("/api/gallery")
	photos, _ := list["photos"].([]any)
	if list["uploadsEnabled"] != true || len(photos) != 1 {
		t.Fatalf("gallery = %v", list)
	}
}

func TestUploadValidation(t *testing.T) {
	c := newTestClient(t, newTestApp())
	c.postJSON("/api/signup", signupBody)
	cases := []struct {
		name    string
		image   []byte
		caption string
		want    int
	}{
		{"not an image", []byte("<html><script>alert(1)</script></html>"), "", http.StatusUnsupportedMediaType},
		{"empty", nil, "", http.StatusBadRequest},
		{"too large", append(pngBytes(t), make([]byte, maxImageBytes)...), "", http.StatusRequestEntityTooLarge},
		{"long caption", pngBytes(t), strings.Repeat("a", 141), http.StatusBadRequest},
	}
	for _, tc := range cases {
		if response, body := c.upload("/api/gallery", tc.image, tc.caption); response.StatusCode != tc.want {
			t.Errorf("%s: status = %d %v, want %d", tc.name, response.StatusCode, body, tc.want)
		}
	}
}

func TestUploadsDisabledWithoutS3(t *testing.T) {
	c := newTestClient(t, &App{store: newMemoryStore()})
	c.postJSON("/api/signup", signupBody)
	if response, _ := c.upload("/api/gallery", pngBytes(t), ""); response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("upload without S3 = %d, want 503", response.StatusCode)
	}
	if _, list := c.get("/api/gallery"); list["uploadsEnabled"] != false {
		t.Fatalf("gallery = %v", list)
	}
}

func TestMediaRejectsUnknownKeys(t *testing.T) {
	c := newTestClient(t, newTestApp())
	for _, path := range []string{"/media/secrets.txt", "/media/avatars/../x.png", "/media/gallery/abc.png"} {
		if response, _ := c.get(path); response.StatusCode == http.StatusFound {
			t.Errorf("GET %s redirected", path)
		}
	}
}

func TestPagesAreServed(t *testing.T) {
	c := newTestClient(t, newTestApp())
	for path, marker := range map[string]string{
		"/login.html":   `id="login-form"`,
		"/account.html": `id="avatar-form"`,
		"/":             `id="gallery-grid"`,
	} {
		response, err := c.http.Get(c.server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if response.StatusCode != http.StatusOK || !strings.Contains(string(raw), marker) {
			t.Errorf("GET %s = %d, missing %s", path, response.StatusCode, marker)
		}
	}
}
