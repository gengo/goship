package deploypage

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"

	"github.com/gengo/goship/lib/auth"
	helpers "github.com/gengo/goship/lib/view-helpers"
)

// New return an http handler which renders deploy page.
// "pushAddr" is an absolute URL to the websocket endpoint of push notification.
//
// pushAddr is defined for backward compatibility
// TODO(yugui) is it really a right way to solve that?  Is it safe for reverse-proxy or some bind addresses?
func New(assets helpers.Assets, pushAddr string) (http.Handler, error) {
	addr, err := url.Parse(pushAddr)
	if err != nil {
		return nil, err
	}
	if !addr.IsAbs() {
		return nil, fmt.Errorf("not an absolute URL: %s", pushAddr)
	}
	if addr.Scheme != "ws" {
		return nil, fmt.Errorf("not a websocket URL: %s", pushAddr)
	}
	return deployPage{assets: assets, pushAddr: addr}, nil
}

type deployPage struct {
	assets   helpers.Assets
	pushAddr *url.URL
}

func (h deployPage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user, err := auth.CurrentUser(r)
	if err != nil {
		log.Println("Failed to Get User")
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	p := r.FormValue("project")
	env := r.FormValue("environment")
	fromRevision := r.FormValue("from_revision")
	toRevision := r.FormValue("to_revision")
	repoOwner := r.FormValue("repo_owner")
	repoName := r.FormValue("repo_name")
	t, err := template.New("deploy.html").ParseFiles("templates/deploy.html", "templates/base.html")
	if err != nil {
		log.Println("ERROR: ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	js, css := h.assets.Templates()
	t.ExecuteTemplate(w, "base", map[string]interface{}{"Javascript": js, "Stylesheet": css, "Project": p, "Env": env, "User": user, "PushAddresss": h.pushAddr, "RepoOwner": repoOwner, "RepoName": repoName, "ToRevision": toRevision, "FromRevision": fromRevision})
}
