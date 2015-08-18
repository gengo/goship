package auth

import (
	"fmt"
	"net/http"
	"os"

	"github.com/golang/glog"
	"github.com/gorilla/sessions"
	"github.com/stretchr/gomniauth"
	"github.com/stretchr/objx"
)

const (
	providerName = "github"
)

// Authenticate decorates "h" with github OAuth authentication.
func Authenticate(h http.Handler) http.Handler {
	callback := fmt.Sprintf("%s/auth/github/login", githubCallbackBase)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := CurrentUser(r)
		if err != nil {
			glog.Warningf("Failed to fetch the current user: %v", err)
			http.Redirect(w, r, callback, http.StatusSeeOther)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func AuthenticateFunc(h http.HandlerFunc) http.Handler {
	return Authenticate(h)
}

// LoginHandler begins github OAuth2 authentication
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if !enabled {
		return
	}

	provider, err := gomniauth.Provider(providerName)
	if err != nil {
		glog.Errorf("failed to get authentication provider %s: %v", providerName, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	state := gomniauth.NewState("after", "success")

	authURL, err := provider.GetBeginAuthURL(state, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, authURL, http.StatusFound)
}

// CallbackHandler receives callback from github OAuth provider
func CallbackHandler(w http.ResponseWriter, r *http.Request) {
	if !enabled {
		http.Error(w, "authenticatin disabled", http.StatusBadRequest)
		return
	}

	provider, err := gomniauth.Provider(providerName)
	if err != nil {
		glog.Errorf("failed to get authentication provider %s: %v", providerName, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	omap, err := objx.FromURLQuery(r.URL.RawQuery)
	if err != nil {
		glog.Errorf("Failed to parse querystring: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	creds, err := provider.CompleteAuth(omap)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	user, userErr := provider.GetUser(creds)
	if userErr != nil {
		glog.Errorf("Failed to get user from Github %s", user)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	session, err := store.Get(r, sessionName)
	if err != nil {
		glog.Errorf("Failed to fetch current session: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	session.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
	}

	session.Values["userName"] = user.Nickname()
	session.Values["avatarURL"] = user.AvatarURL()
	session.Save(r, w)

	http.Redirect(w, r, os.Getenv("GITHUB_CALLBACK_URL"), http.StatusFound)
}
