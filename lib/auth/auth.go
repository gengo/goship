package auth

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/sessions"
	"github.com/stretchr/gomniauth"
	githubOauth "github.com/stretchr/gomniauth/providers/github"
)

const (
	sessionName = "goship"
)

var (
	// enabled is true iff client authentication is enabled.
	enabled bool
	// defaultUser is the value which CurrentUser returns if client authentication is disabled.
	defaultUser User

	githubCallbackBase string

	store *sessions.CookieStore
)

// Initialize collects server-side credential from environment variables and prepare for authentication with Github OAuth.
//
// Client authentication is disabled and CurrentUser always returns "anonymous" if any of the environment variables are missing.
func Initialize(anynomous User, cookieSecret []byte) {
	store = sessions.NewCookieStore(cookieSecret)
	githubCallbackBase = os.Getenv("GITHUB_CALLBACK_URL")
	cred := struct {
		githubRandomHashKey string
		githubOmniauthID    string
		githubOmniauthKey   string
	}{
		os.Getenv("GITHUB_RANDOM_HASH_KEY"),
		os.Getenv("GITHUB_OMNI_AUTH_ID"),
		os.Getenv("GITHUB_OMNI_AUTH_KEY"),
	}
	defaultUser = anynomous

	if cred.githubRandomHashKey == "" || cred.githubOmniauthID == "" || cred.githubOmniauthKey == "" || githubCallbackBase == "" {
		log.Printf(
			"Missing one or more Gomniauth Environment Variables: Running with with limited functionality! \n GITHUB_RANDOM_HASH_KEY [%s] \n GITHUB_OMNI_AUTH_ID [%s] \n GITHUB_OMNI_AUTH_KEY [%s] \n GITHUB_CALLBACK_URL [%s]",
			cred.githubRandomHashKey,
			cred.githubOmniauthID,
			cred.githubOmniauthKey,
			githubCallbackBase,
		)
		enabled = false
		return
	}
	url := fmt.Sprintf("%s/auth/github/callback", githubCallbackBase)

	gomniauth.SetSecurityKey(cred.githubRandomHashKey)
	gomniauth.WithProviders(
		githubOauth.New(cred.githubOmniauthID, cred.githubOmniauthKey, url),
	)
	enabled = true
}

func Enabled() bool {
	return enabled
}

// User is the user who the current request is on behalf of.
type User struct {
	// Name is the name of the user
	Name string
	// Avatar is the URL to the avatar of the user
	Avatar string
}

// CurrentUser returns the current login user of the request.
// It returns the default user if client authentication is disabled in the current context.
func CurrentUser(r *http.Request) (User, error) {
	if !enabled {
		return defaultUser, nil
	}
	session, err := store.Get(r, sessionName)
	if err != nil {
		return User{}, errors.New("cannot fetch current session")
	}
	name, ok := session.Values["userName"].(string)
	if !ok {
		return User{}, errors.New("no username")
	}
	avatar, ok := session.Values["avatarURL"].(string)
	if !ok {
		return User{}, errors.New("no avatar")
	}
	return User{Name: name, Avatar: avatar}, nil
}
