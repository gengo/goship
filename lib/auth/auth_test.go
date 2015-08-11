package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/sessions"
)

func TestCurrentUser(t *testing.T) {
	anonymous := User{Name: "T-600", Avatar: "http://avatar.example/600"}
	Initialize(anonymous, []byte("12345"))

	enabled = true

	req, err := http.NewRequest("GET", "http://host.example", nil)
	if err != nil {
		t.Fatalf("http.NewRequest(%q, %q, nil) failed with %v; want success", "GET", "http://host.example", nil)
	}
	w := httptest.NewRecorder()

	session, err := store.Get(req, sessionName)
	if err != nil {
		t.Errorf("Can't get a session store")
	}
	session.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
	}

	session.Values["userName"] = "T-800"
	session.Values["avatarURL"] = "http://avatar.example/1234"
	session.Save(req, w)

	user, err := CurrentUser(req)
	if err != nil {
		t.Errorf("Failed to get User from GetUser [%s]", err)
	}
	if got, want := user.Name, session.Values["userName"]; got != want {
		t.Errorf("user.Name = %q; want %q", got, want)
	}
	if got, want := user.Avatar, session.Values["avatarURL"]; got != want {
		t.Errorf("user.Avatar = %q; want %q", got, want)
	}
}
